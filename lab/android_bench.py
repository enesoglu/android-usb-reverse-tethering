"""Measure framed PC-to-Android traffic without changing the phone's VPN."""
import argparse
import json
from pathlib import Path
import socket
import struct
import subprocess
import time

PACKAGE = "dev.usbreverse.lab"
READERS = {"simple": 0, "bulk": 1, "direct": 2}
MAGIC = 0x55525431
READY = 0x52454144


def read_exact(sock: socket.socket, length: int) -> bytes:
    chunks = bytearray()
    while len(chunks) < length:
        chunk = sock.recv(length - len(chunks))
        if not chunk:
            raise ConnectionError(f"unexpected EOF after {len(chunks)} of {length} bytes")
        chunks.extend(chunk)
    return bytes(chunks)


def frame_batches(payload_size: int, frames: int):
    payload = bytes([0xA5]) * payload_size
    batch = bytearray()
    for sequence in range(frames):
        batch.extend(struct.pack("!I", sequence))
        batch.extend(payload)
        if sequence % 64 == 63:
            yield bytes(batch)
            batch.clear()
    if batch:
        yield bytes(batch)


def measure(adb: str, serial: str, payload_size: int, mib: int, reader: str = "bulk", receive_buffer: int = 0, precompute: bool = False, nodelay: bool = False) -> dict:
    def command(*args):
        return subprocess.run([adb, "-s", serial, *args], check=True, capture_output=True, text=True, timeout=20).stdout
    with socket.socket() as listener:
        listener.bind(("127.0.0.1", 0))
        listener.listen(1)
        listener.settimeout(20)
        port = listener.getsockname()[1]
        remote = f"tcp:{port}"
        if remote in command("reverse", "--list").split():
            raise RuntimeError("refusing to overwrite an existing ADB mapping")
        command("reverse", remote, remote)
        try:
            command("shell", "am", "force-stop", PACKAGE)
            command("shell", "am", "start", "-W", "-n", PACKAGE + "/.BenchActivity", "--ei", "port", str(port), "--ei", "receive_buffer", str(receive_buffer), "--ei", "reader_mode", str(READERS[reader]))
            peer, _ = listener.accept()
            with peer:
                peer.settimeout(30)
                peer.setsockopt(socket.IPPROTO_TCP, socket.TCP_NODELAY, int(nodelay))
                frames = mib * 1024 * 1024 // payload_size
                preparation_started = time.perf_counter()
                batches = list(frame_batches(payload_size, frames)) if precompute else frame_batches(payload_size, frames)
                preparation_seconds = time.perf_counter() - preparation_started
                peer.sendall(struct.pack("!IIII", MAGIC, payload_size, frames, READERS[reader]))
                if struct.unpack("!I", read_exact(peer, 4))[0] != READY:
                    raise RuntimeError("Android receiver did not acknowledge protocol")
                started = time.perf_counter()
                for batch in batches:
                    peer.sendall(batch)
                result_length = struct.unpack("!I", read_exact(peer, 4))[0]
                if not 0 < result_length <= 4096:
                    raise RuntimeError("invalid result size")
                android = json.loads(read_exact(peer, result_length))
                elapsed = time.perf_counter() - started
                expected = frames * payload_size
                passed = (android["payload_bytes"] == expected and android["frames_received"] == frames
                          and android["sequence_errors"] == 0 and android["payload_sample_errors"] == 0
                          and android["elapsed_ns"] > 0)
                return {"scenario": "android-framed-adb-reverse", "payload_size": payload_size, "reader": reader, "requested_receive_buffer": receive_buffer, "precompute": precompute, "pc_tcp_nodelay": nodelay,
                        "preparation_seconds": round(preparation_seconds, 4),
                        "payload_bytes": expected, "frames": frames, "passed": passed,
                        "pc_seconds": round(elapsed, 4),
                        "pc_payload_mbps": round(expected * 8 / elapsed / 1e6, 2),
                        "android_payload_mbps": round(expected * 8000 / android["elapsed_ns"], 2),
                        "android": android,
                        "scope": "lossless TCP framing; no VPN, UDP socket or Internet throughput measured"}
        finally:
            command("reverse", "--remove", remote)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--adb", default="adb")
    parser.add_argument("--serial", required=True)
    parser.add_argument("--mib", type=int, default=128)
    parser.add_argument("--repeat", type=int, default=3)
    parser.add_argument("--sizes", type=int, nargs="+", default=[1200, 16384])
    parser.add_argument("--nodelay", action="store_true")
    parser.add_argument("--precompute", action="store_true")
    parser.add_argument("--receive-buffer", type=int, default=0)
    parser.add_argument("--reader", choices=tuple(READERS), default="bulk")
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    if not 1 <= args.mib <= 1024 or not 1 <= args.repeat <= 10 or any(not 64 <= size <= 65507 for size in args.sizes):
        parser.error("invalid test limits")
    runs = []
    try:
        for size in args.sizes:
            for _ in range(args.repeat):
                result = measure(args.adb, args.serial, size, args.mib, args.reader, args.receive_buffer, args.precompute, args.nodelay)
                runs.append(result)
                print(json.dumps(result), flush=True)
    finally:
        if args.output:
            args.output.parent.mkdir(parents=True, exist_ok=True)
            args.output.write_text(json.dumps({"schema_version": 1, "runs": runs,
                "planned_runs": len(args.sizes) * args.repeat,
                "passed": len(runs) == len(args.sizes) * args.repeat and all(r["passed"] for r in runs)}, indent=2) + "\n", encoding="utf-8")
    return 0 if all(r["passed"] for r in runs) else 1


if __name__ == "__main__":
    raise SystemExit(main())
