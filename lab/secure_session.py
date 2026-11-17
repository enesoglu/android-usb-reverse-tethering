"""Own an isolated secure daemon for explicit real-phone diagnostics."""
from contextlib import contextmanager
from lab.devices import application_id
import json
import ctypes
from ctypes import wintypes
from pathlib import Path
import queue
import os
import subprocess
import threading
import time

@contextmanager
def diagnostic_session(serial,ports,enabled):
    if not enabled:
        yield
        return
    desktop=Path(os.environ.get('USB_RELAY_DESKTOP_ASSETS','build/desktop')).resolve()
    if not (desktop/'usb-relayd.exe').is_file():raise RuntimeError('Build the desktop bundle first')
    check=subprocess.run(['powershell','-NoProfile','-Command',"if(Get-Process -Name UsbRelay -ErrorAction SilentlyContinue){exit 1}"],capture_output=True,creationflags=subprocess.CREATE_NO_WINDOW)
    if check.returncode:raise RuntimeError('Exit USB Relay from the tray before running isolated phone diagnostics')
    data=Path('.local/diagnostic-state').resolve();data.mkdir(parents=True,exist_ok=True)
    arguments=[str(desktop/'usb-relayd.exe'),'--assets',str(desktop),'--data',str(data),'--diagnostic-ports',','.join(map(str,ports)),'--companion-package',application_id()]
    if os.environ.get('USB_RELAY_CPU_PROFILE'):arguments+=['--cpu-profile',str(Path(os.environ['USB_RELAY_CPU_PROFILE']).resolve())]
    process=subprocess.Popen(arguments,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,text=True,creationflags=subprocess.CREATE_NO_WINDOW)
    events=queue.Queue()
    measurements={'tcp_queue_waits':0,'queue_rejections':0,'peak_download_mbps':0.0,'max_active_flows':0,'relay_cpu_core_percent':0.0,'relay_download_bytes':0,'relay_upload_bytes':0,'relay_uptime_seconds':0,'transport_packets':0,'transport_flushes':0}
    times=ctypes.WinDLL('kernel32',use_last_error=True).GetProcessTimes
    times.argtypes=[wintypes.HANDLE]+[ctypes.POINTER(wintypes.FILETIME)]*4
    times.restype=wintypes.BOOL
    def cpu_seconds():
        values=[wintypes.FILETIME() for _ in range(4)]
        if not times(wintypes.HANDLE(int(process._handle)),*[ctypes.byref(v) for v in values]):return 0.0
        return sum((v.dwHighDateTime<<32)|v.dwLowDateTime for v in values[2:])/10000000.0
    cpu_start=cpu_seconds();clock_start=time.monotonic()


    def collect():
        for line in process.stdout:
            try:
                event=json.loads(line)
                if event.get('type')=='snapshot':
                    state=event['state']
                    measurements['tcp_queue_waits']=max(measurements['tcp_queue_waits'],state.get('tcpQueueWaits',0))
                    measurements['transport_packets']=max(measurements['transport_packets'],state.get('transportPackets',0))
                    measurements['transport_flushes']=max(measurements['transport_flushes'],state.get('transportFlushes',0))
                    measurements['relay_download_bytes']=max(measurements['relay_download_bytes'],state.get('downloadBytes',0))
                    measurements['relay_upload_bytes']=max(measurements['relay_upload_bytes'],state.get('uploadBytes',0))
                    measurements['relay_uptime_seconds']=max(measurements['relay_uptime_seconds'],state.get('uptimeSeconds',0))
                    measurements['queue_rejections']=max(measurements['queue_rejections'],state.get('queueRejections',0))
                    measurements['peak_download_mbps']=max(measurements['peak_download_mbps'],state.get('downloadMbps',0))
                    measurements['max_active_flows']=max(measurements['max_active_flows'],state.get('activeFlows',0))
                    measurements['relay_cpu_core_percent']=100*(cpu_seconds()-cpu_start)/max(.001,time.monotonic()-clock_start)
                events.put(event)
            except ValueError:pass
    reader=threading.Thread(target=collect,daemon=True);reader.start()
    try:
        deadline=time.monotonic()+20
        while True:
            try:event=events.get(timeout=max(.1,deadline-time.monotonic()))
            except queue.Empty:raise RuntimeError('Phone did not finish the secure handshake; inspect protected phone diagnostics and VPN consent') from None
            if event.get('type')=='fatal':raise RuntimeError(event.get('message','Secure daemon failed'))
            if event.get('type')=='snapshot' and any(d.get('serial')==serial and d.get('state')=='device' for d in event['state'].get('devices',[])):break
            if time.monotonic()>deadline:raise RuntimeError('Phone was not discovered')
        process.stdin.write(json.dumps({'id':1,'method':'connect','serial':serial,'remember':False})+'\n');process.stdin.flush()
        deadline=time.monotonic()+60
        while True:
            try:event=events.get(timeout=max(.1,deadline-time.monotonic()))
            except queue.Empty:raise RuntimeError('Phone did not finish the secure handshake; inspect protected phone diagnostics and VPN consent') from None
            if event.get('type')=='result' and not event.get('ok'):raise RuntimeError(event.get('error','Connect failed'))
            if event.get('type')=='snapshot':
                state=event['state']
                if state.get('state')=='connected':break
                if state.get('state') in ('error','security_error','adb_unavailable'):raise RuntimeError(state.get('message','Secure connection failed'))
            if time.monotonic()>deadline:raise RuntimeError('Approve the VPN on the phone, then retry')
        yield measurements
    finally:
        if process.poll() is None:
            try:
                process.stdin.write('{"method":"shutdown"}\n');process.stdin.flush();process.stdin.close();process.wait(timeout=15)
            except (BrokenPipeError,OSError,subprocess.TimeoutExpired):
                if process.poll() is None:process.kill();process.wait(timeout=5)
        reader.join(timeout=2);process.stdout.close()
