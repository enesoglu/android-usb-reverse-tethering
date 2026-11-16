# Query Windows USB hub descriptors without changing any device state.
$ErrorActionPreference = 'Stop'
if (-not ('UsbLinkProbe' -as [type])) {
Add-Type -TypeDefinition @"
using System;
using System.Collections.Generic;
using System.Runtime.InteropServices;
using Microsoft.Win32.SafeHandles;

public static class UsbLinkProbe {
    [StructLayout(LayoutKind.Sequential)]
    struct DeviceInterface {
        public int Size;
        public Guid ClassGuid;
        public uint Flags;
        public IntPtr Reserved;
    }
    [DllImport("setupapi.dll", SetLastError=true)]
    static extern IntPtr SetupDiGetClassDevs(ref Guid guid, IntPtr enumerator, IntPtr parent, uint flags);
    [DllImport("setupapi.dll", SetLastError=true)]
    static extern bool SetupDiEnumDeviceInterfaces(IntPtr info, IntPtr device, ref Guid guid, uint index, ref DeviceInterface data);
    [DllImport("setupapi.dll", CharSet=CharSet.Unicode, SetLastError=true)]
    static extern bool SetupDiGetDeviceInterfaceDetail(IntPtr info, ref DeviceInterface data, IntPtr detail, int length, out int required, IntPtr device);
    [DllImport("setupapi.dll")]
    static extern bool SetupDiDestroyDeviceInfoList(IntPtr info);
    [DllImport("kernel32.dll", CharSet=CharSet.Unicode, SetLastError=true)]
    static extern SafeFileHandle CreateFile(string path, uint access, uint share, IntPtr security, uint creation, uint flags, IntPtr template);
    [DllImport("kernel32.dll", SetLastError=true)]
    static extern bool DeviceIoControl(SafeFileHandle file, uint code, byte[] input, int inputLength, byte[] output, int outputLength, out int returned, IntPtr overlapped);

    public sealed class Link {
        public string Vendor {get;set;}
        public string Product {get;set;}
        public string DeviceUsbVersion {get;set;}
        public int SpeedCode {get;set;}
        public string Speed {get;set;}
        public int? SuperSpeedFlags {get;set;}
        public bool? OperatingAtSuperSpeed {get;set;}
    }
    public static Link[] Read() {
        var links = new List<Link>();
        Guid guid = new Guid("f18a0e88-c30c-11d0-8815-00a0c906bed8");
        IntPtr devices = SetupDiGetClassDevs(ref guid, IntPtr.Zero, IntPtr.Zero, 0x12);
        if (devices == new IntPtr(-1)) throw new System.ComponentModel.Win32Exception();
        try {
            for (uint index=0; ; index++) {
                var data = new DeviceInterface();
                data.Size = Marshal.SizeOf(typeof(DeviceInterface));
                if (!SetupDiEnumDeviceInterfaces(devices, IntPtr.Zero, ref guid, index, ref data)) {
                    int error = Marshal.GetLastWin32Error();
                    if (error != 259) throw new System.ComponentModel.Win32Exception(error);
                    break;
                }
                int required;
                SetupDiGetDeviceInterfaceDetail(devices, ref data, IntPtr.Zero, 0, out required, IntPtr.Zero);
                IntPtr detail = Marshal.AllocHGlobal(required);
                try {
                    Marshal.WriteInt32(detail, IntPtr.Size == 8 ? 8 : 6);
                    if (!SetupDiGetDeviceInterfaceDetail(devices, ref data, detail, required, out required, IntPtr.Zero))
                        throw new System.ComponentModel.Win32Exception();
                    string path = Marshal.PtrToStringUni(IntPtr.Add(detail, 4));
                    using (var hub = CreateFile(path, 0x40000000, 3, IntPtr.Zero, 3, 0, IntPtr.Zero)) {
                        if (hub.IsInvalid) { Console.Error.WriteLine("hub open error: " + Marshal.GetLastWin32Error()); continue; }
                        byte[] node = new byte[4096];
                        int returned;
                        if (!DeviceIoControl(hub, 0x220408, node, node.Length, node, node.Length, out returned, IntPtr.Zero)) { Console.Error.WriteLine("USB query error: " + Marshal.GetLastWin32Error()); continue; }
                        int ports = node[6];
                        for (int port=1; port<=ports; port++) {
                            byte[] connection = new byte[4096];
                            Array.Copy(BitConverter.GetBytes(port), connection, 4);
                            if (!DeviceIoControl(hub, 0x220448, connection, connection.Length, connection, connection.Length, out returned, IntPtr.Zero)) { Console.Error.WriteLine("USB query error: " + Marshal.GetLastWin32Error()); continue; }
                            // USBIOCTL structures are packed: connection status starts at byte 31.
                            if (BitConverter.ToInt32(connection, 31) != 1) continue;
                            ushort vendor = BitConverter.ToUInt16(connection, 12);
                            if (vendor != 0x18D1 && vendor != 0x04E8) continue;
                            int speed = connection[23];
                            byte[] v2 = new byte[16];
                            Array.Copy(BitConverter.GetBytes(port), 0, v2, 0, 4);
                            Array.Copy(BitConverter.GetBytes(16), 0, v2, 4, 4);
                            Array.Copy(BitConverter.GetBytes(4), 0, v2, 8, 4);
                            int? superFlags = null;
                            if (DeviceIoControl(hub, 0x22045C, v2, v2.Length, v2, v2.Length, out returned, IntPtr.Zero))
                                superFlags = BitConverter.ToInt32(v2, 12);
                            // Microsoft recommends V2 flags over the legacy speed byte.
                            if (superFlags.HasValue && (superFlags.Value & 5) != 0) speed = Math.Max(speed, 3);
                            string[] names = {"Low (1.5 Mbps)", "Full (12 Mbps)", "High (480 Mbps)", "Super (5 Gbps or above)"};
                            links.Add(new Link {
                                Vendor = vendor.ToString("X4"),
                                Product = BitConverter.ToUInt16(connection, 14).ToString("X4"),
                                DeviceUsbVersion = BitConverter.ToUInt16(connection, 6).ToString("X4"),
                                SpeedCode = speed,
                                Speed = speed < names.Length ? names[speed] : "Unknown",
                                SuperSpeedFlags = superFlags,
                                OperatingAtSuperSpeed = superFlags.HasValue ? (bool?) ((superFlags.Value & 1) != 0) : null
                            });
                        }
                    }
                } finally { Marshal.FreeHGlobal(detail); }
            }
        } finally { SetupDiDestroyDeviceInfoList(devices); }
        return links.ToArray();
    }
}
"@
}
$links = @([UsbLinkProbe]::Read())
if ($links.Count -eq 0) { throw "No matching Samsung/Android USB device found" }
ConvertTo-Json -InputObject $links -Depth 3
