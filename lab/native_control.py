"""Count bytes on Android before accepting a native USB throughput result."""
import argparse
import json
from pathlib import Path
import socket
import subprocess
import time

from lab.android_bench import frame_batches


def validated_rate(expected: int, received: int, elapsed: float) -> float | None:
    if expected <= 0 or received != expected or elapsed <= 0:
        return None
    return round(received * 8 / elapsed / 1e6, 2)


def measure(serial: str, stdin_mode: str = "open", mib: int = 128) -> dict:
    def command(*args):
        return subprocess.run(["adb", "-s", serial, *args], capture_output=True, text=True, check=True, timeout=20).stdout
    with socket.socket() as listener:
        listener.bind(("127.0.0.1", 0))
        listener.listen(1)
        listener.settimeout(15)
        port = listener.getsockname()[1]
        mapping = f"tcp:{port}"
        if mapping in command("reverse", "--list").split():
            raise RuntimeError("refusing to overwrite an existing ADB mapping")
        command("reverse", mapping, mapping)
        process = None
        try:
            # Open stdin explicitly. Toybox nc may exit without receiving when
            # stdin is already EOF; PC sendall success does not prove delivery.
            process = subprocess.Popen(["adb", "-s", serial, "shell", f"nc 127.0.0.1 {port} | wc -c"],
                                       stdin=subprocess.PIPE if stdin_mode == "open" else subprocess.DEVNULL,
                                       stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            peer, _ = listener.accept()
            with peer:
                peer.settimeout(20)
                batches = list(frame_batches(1200, mib * 1024 * 1024 // 1200))
                count = 0
                started = time.perf_counter()
                send_error = None
                try:
                    for batch in batches:
                        peer.sendall(batch)
                        count += len(batch)
                    peer.shutdown(socket.SHUT_WR)
                except OSError as error:
                    send_error = type(error).__name__
                process.wait(timeout=30)
                output = process.stdout.read().decode().strip()
                received = int(output)
                elapsed = time.perf_counter() - started
                expected = sum(len(batch) for batch in batches)
                rate = validated_rate(expected, received, elapsed)
                passed = rate is not None and process.returncode == 0 and send_error is None
                return {"scenario": "native-counted-control", "stdin_mode": stdin_mode,
                        "expected_bytes": expected, "pc_sent_bytes": count, "android_received_bytes": received,
                        "seconds": round(elapsed, 4), "validated_wire_mbps": rate, "passed": passed,
                        "send_error": send_error, "scope": "received byte count checked; no content validation or VPN"}
        finally:
            command("reverse", "--remove", mapping)
            if process is not None:
                if process.poll() is None:
                    process.terminate()
                    process.wait(timeout=5)
                if process.stdin is not None:
                    process.stdin.close()
                process.stdout.close()
                process.stderr.close()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--serial", required=True)
    parser.add_argument("--stdin-mode", choices=("open", "eof"), default="open")
    parser.add_argument("--mib", type=int, default=128)
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    if not 1 <= args.mib <= 1024:
        parser.error("mib must be in [1, 1024]")
    result = measure(args.serial, args.stdin_mode, args.mib)
    print(json.dumps(result), flush=True)
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(json.dumps(result, indent=2) + "\n", encoding="utf-8")
    return 0 if result["passed"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
