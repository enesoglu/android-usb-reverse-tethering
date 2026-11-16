"""Explicit actual-phone pairing and reconnect verification using the desktop state."""
import argparse
import json
import os
from pathlib import Path
import queue
import subprocess
import threading
import time
from lab.devices import resolve_device, application_id


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--physical', action='store_true')
    parser.add_argument('--output', type=Path, default=Path('results/phone-lifecycle-v02.json'))
    args = parser.parse_args()
    serial = resolve_device()
    assets = Path(os.environ.get('USB_RELAY_DESKTOP_ASSETS','build/desktop')).resolve()
    metadata=json.loads((assets/'assets.manifest.json').read_text(encoding='utf-8-sig'))
    state=metadata.get('stateDirectory','UsbReverseTether')
    if state not in {'UsbRelay','UsbReverseTether'}:raise RuntimeError('Invalid state directory')
    data=Path(os.environ['LOCALAPPDATA'])/state
    daemon = assets / 'usb-relayd.exe'
    check = subprocess.run(['powershell','-NoProfile','-Command', 'if(Get-Process -Name UsbRelay,usb-relayd -ErrorAction SilentlyContinue){exit 1}'],capture_output=True,creationflags=subprocess.CREATE_NO_WINDOW)
    if check.returncode:
        raise RuntimeError('Exit the desktop and other diagnostics first')
    results = {}

    def start():
        proc = subprocess.Popen([str(daemon),'--assets',str(assets),'--data',str(data),'--companion-package',application_id()],stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,text=True,creationflags=subprocess.CREATE_NO_WINDOW)
        events = queue.Queue()
        def collect():
            for line in proc.stdout:
                try: events.put(json.loads(line))
                except ValueError: pass
        threading.Thread(target=collect,daemon=True).start()
        return proc, events

    def send(proc,method):
        proc.stdin.write(json.dumps({'id':1,'method':method,'serial':serial,'remember':True})+'\n')
        proc.stdin.flush()

    def wait(events,predicate,seconds=60):
        deadline = time.monotonic()+seconds
        while time.monotonic()<deadline:
            try: event=events.get(timeout=min(1,max(.1,deadline-time.monotonic())))
            except queue.Empty: continue
            if event.get('type')=='fatal': raise RuntimeError(event.get('message','Manager failed'))
            if event.get('type')=='snapshot':
                state=event['state']
                if state.get('state') in ('security_error','error','adb_unavailable'): raise RuntimeError(state.get('message','Connection failed'))
                if predicate(state): return state
        raise RuntimeError('Actual-phone lifecycle condition timed out')

    def stop(proc):
        if proc.poll() is None:
            try:
                send(proc,'shutdown');proc.stdin.close();proc.wait(timeout=20)
            except (OSError,subprocess.TimeoutExpired):
                if proc.poll() is None:proc.kill();proc.wait(timeout=5)
        proc.stdout.close()

    proc,events=start()
    try:
        initial=wait(events,lambda s:any(d['serial']==serial for d in s['devices']))
        if initial['state']!='connected':
            send(proc,'connect')
        connected=wait(events,lambda s:s['state']=='connected')
        saved=next(t for t in connected['trusted'] if t['serial']==serial)
        results['authenticated_pairing_saved']=len(saved['pin'])==64
        print('PASS: authenticated phone pairing saved',flush=True)
        send(proc,'disconnect')
        wait(events,lambda s:s['state']=='disconnected' and not s.get('serial'))
        until=time.monotonic()+3
        while time.monotonic()<until:
            state=wait(events,lambda s:True,5)
            if state.get('serial'):raise RuntimeError('Manual stop unexpectedly reconnected')
        results['manual_stop_suppressed']=True
        print('PASS: manual stop remains stopped',flush=True)
    finally:stop(proc)

    proc,events=start()
    try:
        resumed=wait(events,lambda s:s['state']=='connected')
        results['saved_phone_auto_connected_after_restart']=any(t['serial']==serial and t['pin']==saved['pin'] for t in resumed['trusted'])
        print('READY: saved phone auto-connected; physical cable test can begin',flush=True)
        if args.physical:
            wait(events,lambda s:not any(d['serial']==serial for d in s['devices']),120)
            results['physical_usb_detach_detected']=True
            print('DETACHED: waiting for the same phone to return',flush=True)
            resumed=wait(events,lambda s:s['state']=='connected',120)
            results['physical_usb_reattach_auto_connected']=any(t['serial']==serial and t['pin']==saved['pin'] for t in resumed['trusted'])
        results['passed']=all(results.values())
        args.output.parent.mkdir(parents=True,exist_ok=True)
        args.output.write_text(json.dumps(results,indent=2)+'\n',encoding='utf-8')
        print(json.dumps(results),flush=True)
        return 0 if results['passed'] else 1
    finally:stop(proc)

if __name__=='__main__':raise SystemExit(main())
