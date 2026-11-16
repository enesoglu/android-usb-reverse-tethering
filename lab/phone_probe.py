"""Verify TCP, UDP and public HTTPS from Android while the USB VPN is active."""
import argparse
from lab.secure_session import diagnostic_session
from lab.devices import resolve_device, component, diagnostics_uri
import http.server
import json
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
    parser.add_argument("--output",type=Path)
    args=parser.parse_args()
    if not args.serial: args.serial=resolve_device()
    def adb(*parts):
        result=subprocess.run(["adb","-s",args.serial,*parts],capture_output=True,text=True,timeout=20)
        if result.returncode: raise RuntimeError("ADB probe setup failed")
        return result.stdout
    class Health(http.server.BaseHTTPRequestHandler):
        def do_GET(self):
            data=b"usb-relay-health"
            self.send_response(200); self.send_header("Content-Length",str(len(data))); self.end_headers(); self.wfile.write(data)
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
            with diagnostic_session(args.serial,[health_server.server_port,udp.getsockname()[1],port],args.secure):
                adb("shell","am","start","-W","-n",component("ProbeActivity"),"--ei","tcp_port",str(health_server.server_port),
                    "--ei","udp_port",str(udp.getsockname()[1]),"--ei","report_port",str(port),"--es","nonce",nonce)
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
                        result["passed"]=all(result.get(key) is True for key in ("tcp","udp","https"))
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
