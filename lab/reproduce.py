"""Exercise the real gnirehtet UDP overflow path with an intentionally stalled peer."""
import argparse
from collections import Counter
import json
from pathlib import Path
import socket
import struct
import subprocess
import threading
import time

from lab.packets import udp_packet


def reproduce(executable: str, seconds: float = 3.0) -> dict:
    counts = Counter()
    lock = threading.Lock()
    with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as source:
        source.bind(("127.0.0.1", 0))
        source.settimeout(5)
        with socket.socket() as reservation:
            reservation.bind(("127.0.0.1", 0))
            port = reservation.getsockname()[1]
        relay = subprocess.Popen([executable, "relay", "-p", str(port)],
                                 stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
        def collect():
            for raw in relay.stdout:
                line = raw.decode(errors="replace")
                with lock:
                    if "Client buffer full" in line:
                        counts["buffer_full_warnings"] += 1
                    if "Cannot send to client, drop packet" in line:
                        counts["udp_drop_warnings"] += 1
                    if "ERROR" in line:
                        counts["relay_errors"] += 1
        reader = threading.Thread(target=collect, daemon=True)
        reader.start()
        try:
            deadline = time.monotonic() + 5
            while True:
                if relay.poll() is not None:
                    raise RuntimeError("relay exited before accepting a client")
                try:
                    peer = socket.create_connection(("127.0.0.1", port), timeout=.2)
                    break
                except OSError:
                    if time.monotonic() >= deadline:
                        raise RuntimeError("relay did not become ready")
                    time.sleep(.025)
            with peer:
                peer.setsockopt(socket.SOL_SOCKET, socket.SO_RCVBUF, 4096)
                peer.settimeout(5)
                handshake = b""
                while len(handshake) < 4:
                    chunk = peer.recv(4 - len(handshake))
                    if not chunk:
                        raise RuntimeError("relay handshake ended early")
                    handshake += chunk
                peer.sendall(udp_packet(b"synthetic-probe", source.getsockname()[1]))
                probe, relay_address = source.recvfrom(2048)
                if probe != b"synthetic-probe":
                    raise RuntimeError("UDP mapping validation failed")
                # This is the controlled failure: the client stops reading, while
                # an upstream UDP producer keeps sending. Nothing touches Android.
                payload = bytearray(1200)
                sent = 0
                start = time.perf_counter()
                deadline = time.monotonic() + seconds
                while time.monotonic() < deadline:
                    struct.pack_into("!Q", payload, 0, sent)
                    source.sendto(payload, relay_address)
                    sent += 1
                elapsed = time.perf_counter() - start
                time.sleep(.15)
                with lock:
                    snapshot = dict(counts)
                return {"scenario": "gnirehtet-stalled-consumer", "payload_bytes": 1200,
                        "datagrams_sent": sent, "elapsed_seconds": round(elapsed, 3),
                        "offered_mbps": round(sent * 1200 * 8 / elapsed / 1e6, 2),
                        "buffer_full_warnings": snapshot.get("buffer_full_warnings", 0),
                        "udp_drop_warnings": snapshot.get("udp_drop_warnings", 0),
                        "relay_errors": snapshot.get("relay_errors", 0),
                        "scope": "PC loopback; synthetic stalled client; not a phone diagnosis"}
        finally:
            if relay.poll() is None:
                relay.terminate()
            try:
                relay.wait(timeout=5)
            except subprocess.TimeoutExpired:
                relay.kill()
                relay.wait(timeout=5)
            reader.join(timeout=5)
            relay.stdout.close()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--relay", required=True)
    parser.add_argument("--seconds", type=float, default=3)
    parser.add_argument("--repeat", type=int, default=3)
    parser.add_argument("--expect", choices=("clean", "drop"), default="clean")
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    if not 0 < args.seconds <= 30 or not 1 <= args.repeat <= 10:
        parser.error("seconds must be in (0, 30], repeat in [1, 10]")
    results = [reproduce(args.relay, args.seconds) for _ in range(args.repeat)]
    for result in results:
        print(json.dumps(result), flush=True)
    reproduced = all(r["buffer_full_warnings"] > 0 and r["udp_drop_warnings"] > 0 for r in results)
    clean = all(r["buffer_full_warnings"] == 0 and r["udp_drop_warnings"] == 0 for r in results)
    passed = (reproduced if args.expect == "drop" else clean) and not any(r["relay_errors"] for r in results)
    report = {"schema_version": 1, "expected": args.expect, "passed": passed, "runs": results}
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
    print("PASS" if passed else "FAIL", flush=True)
    return 0 if passed else 1


if __name__ == "__main__":
    raise SystemExit(main())
