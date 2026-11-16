"""Start the loopback relay and configure the user-approved Android USB VPN."""
import argparse
from lab.devices import resolve_device
import json
import os
from pathlib import Path
import queue
import secrets
import subprocess
import threading

PACKAGE = "dev.usbreverse.lab"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--serial")
    parser.add_argument("--port", type=int, default=31417)
    parser.add_argument("--relay", type=Path, default=Path("build/usb-relay.exe"))
    args = parser.parse_args()
    if not args.serial: args.serial=resolve_device()
    if not 1024 <= args.port <= 65535:
        parser.error("port must be in [1024, 65535]")
    if not args.relay.is_file():
        parser.error("build the relay with task build first")
    def adb(*parts):
        result = subprocess.run(["adb", "-s", args.serial, *parts], capture_output=True, text=True, timeout=20)
        if result.returncode:
            # Arguments may contain the ephemeral session key. Never print them.
            raise RuntimeError("ADB command failed; check the device connection")
        return result.stdout
    mapping = f"tcp:{args.port}"
    if mapping in adb("reverse", "--list").split():
        raise RuntimeError("refusing to overwrite an existing ADB mapping")
    token = secrets.token_hex(32)
    environment = os.environ.copy()
    environment["URT_TOKEN"] = token
    relay = subprocess.Popen([str(args.relay.resolve()), "--port", str(args.port)], env=environment,
                             stdin=subprocess.DEVNULL, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
    ready = queue.Queue()
    disconnected = threading.Event()
    local=Path(".local")
    local.mkdir(exist_ok=True)
    stop_file=local / f"stop-{os.getpid()}"
    stop_file.unlink(missing_ok=True)
    metadata=local / "controller.json"
    metadata.write_text(json.dumps({"pid":os.getpid()}),encoding="utf-8")
    def collect():
        for line in relay.stdout:
            print(line.rstrip(), flush=True)
            if "USB relay listening on loopback" in line:
                ready.put(True)
            if line.strip() == "USB tunnel disconnected":
                disconnected.set()
    reader = threading.Thread(target=collect, daemon=True)
    reader.start()
    installed_mapping = False
    launched = False
    try:
        try:
            ready.get(timeout=10)
        except queue.Empty:
            raise RuntimeError("relay did not become ready") from None
        if relay.poll() is not None:
            raise RuntimeError("relay exited before device setup")
        adb("reverse", mapping, mapping)
        installed_mapping = True
        adb("shell", "am", "force-stop", PACKAGE)
        adb("shell", "am", "start", "-W", "-n", PACKAGE + "/.MainActivity", "--ez", "vpn", "true",
            "--ei", "port", str(args.port), "--es", "token", token, "--ez", "auto_connect", "true")
        launched = True
        print("Ready. Tap Connect USB VPN on the phone and accept Android's VPN prompt.", flush=True)
        print("Starting this VPN replaces the phone's current VPN. Ctrl+C disconnects and cleans up.", flush=True)
        while relay.poll() is None and not stop_file.exists():
            if disconnected.wait(.5): break
    except KeyboardInterrupt:
        print("Disconnecting USB VPN", flush=True)
    finally:
        if launched:
            try:
                adb("shell", "am", "force-stop", PACKAGE)
            except (RuntimeError, subprocess.TimeoutExpired):
                print("Phone unavailable during cleanup; the VPN should stop when USB disconnects.", flush=True)
        if installed_mapping:
            try:
                adb("reverse", "--remove", mapping)
            except (RuntimeError, subprocess.TimeoutExpired):
                print("Phone unavailable; temporary mapping cleanup will complete on device disconnect.", flush=True)
        if relay.poll() is None:
            relay.terminate()
            try:
                relay.wait(timeout=5)
            except subprocess.TimeoutExpired:
                relay.kill()
                relay.wait(timeout=5)
        reader.join(timeout=5)
        relay.stdout.close()
        stop_file.unlink(missing_ok=True)
        try:
            if json.loads(metadata.read_text(encoding="utf-8")).get("pid")==os.getpid(): metadata.unlink()
        except (FileNotFoundError,ValueError): pass


if __name__ == "__main__":
    main()
