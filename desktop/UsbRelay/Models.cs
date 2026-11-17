using System;
using System.Collections.Generic;
namespace UsbRelayDesktop;

public sealed class Settings
{
    public string ADBPath { get; set; } = ""; public bool AutoConnect { get; set; } = true; public bool StartWithWindows
    {
        get; set;
    }
    public bool StartMinimized
    {
        get; set;
    }
    public bool MinimizeToTray { get; set; } = true; public bool CloseToTray { get; set; } = true; public bool AllowLAN
    {
        get; set;
    }
    public string DNS { get; set; } = "1.1.1.1";
}
public sealed class Device
{
    public string Serial { get; set; } = ""; public string Model { get; set; } = "Android phone"; public string State { get; set; } = ""; public bool Trusted
    {
        get; set;
    }
    public string Pin { get; set; } = ""; public string Display => $"{Model}   ·   {(State == "device" ? (Trusted ? "Paired" : "Not paired") : State)}";
}
public sealed class Trusted
{
    public string Serial { get; set; } = ""; public string Model { get; set; } = ""; public string Pin { get; set; } = ""; public DateTime PairedAt
    {
        get; set;
    }
    public string ShortPin => Pin.Length >= 16 ? $"Key  {Pin[..8]} … {Pin[^8..]}" : "Key unavailable"; public string PairedLabel => $"Paired {PairedAt.ToLocalTime():d}";
}
public sealed class Snapshot
{
    public string State { get; set; } = "starting"; public string Message { get; set; } = "Starting the private connection manager"; public List<Device> Devices { get; set; } = new(); public List<Trusted> Trusted { get; set; } = new(); public Settings Settings { get; set; } = new(); public string Serial { get; set; } = ""; public string HostPin { get; set; } = ""; public double DownloadMbps
    {
        get; set;
    }
    public double UploadMbps
    {
        get; set;
    }
    public ulong DownloadBytes
    {
        get; set;
    }
    public ulong UploadBytes
    {
        get; set;
    }
    public ulong QueueRejections
    {
        get; set;
    }
    public long ActiveFlows
    {
        get; set;
    }
    public long UptimeSeconds
    {
        get; set;
    }
}
