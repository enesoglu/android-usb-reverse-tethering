using System;
using System.IO;
using Microsoft.Win32;
namespace UsbRelayDesktop;

public static class StartupRegistration
{
    private const string RunPath = @"Software\Microsoft\Windows\CurrentVersion\Run";
    public static void Set(bool enabled, string executable, string valueName = "UsbReverseTether")
    {
        executable = Path.GetFullPath(executable);
        if (executable.Contains('"'))
            throw new ArgumentException("Invalid executable path");
        using RegistryKey key = Registry.CurrentUser.CreateSubKey(RunPath, true);
        if (enabled)
        {
            string command = $"\"{executable}\" --startup";
            if (command.Length > 260)
                throw new IOException("Move the app to a shorter path before enabling startup");
            key.SetValue(valueName, command, RegistryValueKind.String);
        }
        else
            key.DeleteValue(valueName, false);
    }
    public static bool IsEnabled(string valueName = "UsbReverseTether")
    {
        using RegistryKey? key = Registry.CurrentUser.OpenSubKey(RunPath);
        return key?.GetValue(valueName) is string;
    }
    public static string? Capture()
    {
        using RegistryKey? key = Registry.CurrentUser.OpenSubKey(RunPath);
        return key?.GetValue("UsbReverseTether") as string;
    }
    public static void Restore(string? command)
    {
        using RegistryKey key = Registry.CurrentUser.CreateSubKey(RunPath, true);
        if (command == null)
            key.DeleteValue("UsbReverseTether", false);
        else
            key.SetValue("UsbReverseTether", command, RegistryValueKind.String);
    }
    public static bool SelfTest()
    {
        string name = "UsbReverseTether.Test." + Guid.NewGuid().ToString("N");
        try
        {
            Set(true, Environment.ProcessPath!, name);
            using var key = Registry.CurrentUser.OpenSubKey(RunPath);
            if (key?.GetValue(name) as string != $"\"{Environment.ProcessPath}\" --startup")
                return false;
            if (!IsEnabled(name))
                return false;
            Set(false, Environment.ProcessPath!, name);
            return !IsEnabled(name);
        }
        finally { Set(false, Environment.ProcessPath!, name); }
    }
}
