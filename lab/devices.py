"""Choose a single authorized ADB device without logging device identifiers."""
import subprocess


def choose_device(output: str) -> str:
    devices=[]
    for line in output.splitlines():
        parts=line.split()
        if len(parts)>=2 and parts[1]=="device": devices.append(parts[0])
    if not devices: raise RuntimeError("No authorized Android device; connect USB and approve USB debugging")
    if len(devices)!=1: raise RuntimeError("Multiple Android devices; select one with --serial")
    return devices[0]


def resolve_device() -> str:
    result=subprocess.run(["adb","devices"],capture_output=True,text=True,timeout=15)
    if result.returncode: raise RuntimeError("Cannot query ADB devices")
    return choose_device(result.stdout)

# Explicit lab selection supports the separately installed public companion.
def application_id():
    import os
    package=os.environ.get("USB_RELAY_PACKAGE","dev.usbreverse.lab")
    if package not in {"dev.usbreverse.lab","io.github.enesoglu.usbrelay"}:
        raise ValueError("Unsupported companion package")
    return package

def component(activity):
    import re
    if not re.fullmatch("[A-Za-z]+",activity):raise ValueError("Invalid activity name")
    return application_id()+"/dev.usbreverse.lab."+activity

def diagnostics_uri():
    return "content://"+application_id()+".diagnostics/status"
