"""Measure SHA-256-verified VPN throughput and UDP latency under load."""
import argparse
from lab.secure_session import diagnostic_session
from lab.devices import resolve_device, component, diagnostics_uri
import http.server
import hashlib
import json
import re
from pathlib import Path
import secrets
import socket
import subprocess
import threading
import time


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--serial")
    parser.add_argument("--secure", action="store_true", help="own an isolated mTLS daemon; exit desktop app first")
    parser.add_argument("--mib",type=int,default=128)
    parser.add_argument("--output",type=Path)
    args=parser.parse_args()
    if not args.serial: args.serial=resolve_device()
    if not 1<=args.mib<=512: parser.error("mib must be in [1,512]")
    expected=args.mib*1024*1024
    block=bytes([0xA5])*65536
    digest=hashlib.sha256()
    for _ in range(expected//len(block)): digest.update(block)
    expected_hash=digest.hexdigest()
    def adb(*parts):
        result=subprocess.run(["adb","-s",args.serial,*parts],capture_output=True,text=True,timeout=20)
        if result.returncode: raise RuntimeError("ADB probe setup failed")
        return result.stdout
    class Health(http.server.BaseHTTPRequestHandler):
        def do_GET(self):
            self.send_response(200); self.send_header("Content-Length",str(expected)); self.end_headers()
            for _ in range(expected//len(block)): self.wfile.write(block)
        def log_message(self,*parts): pass
    health_server=http.server.HTTPServer(("127.0.0.1",0),Health)
    threading.Thread(target=health_server.serve_forever,daemon=True).start()
    with socket.socket(socket.AF_INET,socket.SOCK_DGRAM) as udp, socket.socket() as reports:
        udp.bind(("127.0.0.1",0)); udp.settimeout(1)
        stop=threading.Event()
        def echo():
            while not stop.is_set():
                try: data,address=udp.recvfrom(2048); udp.sendto(data,address)
                except socket.timeout: continue
                except OSError: return
        echo_thread=threading.Thread(target=echo,daemon=True);echo_thread.start()
        reports.bind(("127.0.0.1",0)); reports.listen(1); reports.settimeout(60)
        port=reports.getsockname()[1]; mapping=f"tcp:{port}"
        if mapping in adb("reverse","--list").split(): raise RuntimeError("report mapping already exists")
        adb("reverse",mapping,mapping)
        nonce=secrets.token_hex(16)
        try:
            with diagnostic_session(args.serial,[health_server.server_port,udp.getsockname()[1],port],args.secure) as session_metrics:
                adb("shell","am","start","-W","-n",component("SpeedActivity"),"--ei","tcp_port",str(health_server.server_port),
                    "--ei","udp_port",str(udp.getsockname()[1]),"--ei","report_port",str(port),"--es","nonce",nonce,"--ei","mib",str(args.mib))
                deadline=time.monotonic()+60
                while time.monotonic()<deadline:
                    reports.settimeout(max(.1,deadline-time.monotonic()))
                    conn,_=reports.accept()
                    with conn:
                        conn.settimeout(5)
                        try:
                            with conn.makefile("rb") as report:
                                line=report.readline(8193)
                            if not line or len(line)>8192: continue
                            result=json.loads(line)
                            if result.pop("nonce",None)!=nonce: continue
                        except (ValueError,OSError):
                            continue
                        conn.sendall(b"OK")
                        result["passed"]=(result.get("passed") is True and result.get("received_bytes")==expected and result.get("sha256")==expected_hash)
                        result["relay_metrics"]=dict(session_metrics or {})
                        health=adb("shell","content","query","--uri",diagnostics_uri())
                        for field in ("tun_drops","up_packets","down_packets"):
                            match=re.search(r"\b"+field+r"=(\d+)",health)
                            if match:result["relay_metrics"]["sampled_phone_"+field]=int(match.group(1))
                        result["expected_bytes"]=expected
                        result["sha256_verified"]=result.get("sha256")==expected_hash
                        result["scope"]="local synthetic HTTP over actual Android VPN; not public Internet speed"
                        print(json.dumps(result),flush=True)
                        if args.output:
                            args.output.parent.mkdir(parents=True,exist_ok=True)
                            args.output.write_text(json.dumps(result,indent=2)+"\n",encoding="utf-8")
                        return 0 if result["passed"] else 1
                raise RuntimeError("phone did not deliver a valid probe report")
        finally:
            adb("reverse","--remove",mapping)
            stop.set();echo_thread.join(timeout=2);health_server.shutdown();health_server.server_close()


if __name__=="__main__": raise SystemExit(main())
