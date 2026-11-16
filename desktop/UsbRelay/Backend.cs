using System;
using System.Collections.Concurrent;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Reflection;
using System.Security.Cryptography;
using System.Text.Json;
using System.Threading;
using System.Threading.Tasks;
namespace UsbRelayDesktop;

public sealed class Backend : IAsyncDisposable
{
    public event Action<Snapshot>? Changed; public event Action<string>? Failed;
    private Process? process; private readonly SemaphoreSlim write = new(1, 1); private readonly ConcurrentDictionary<int, TaskCompletionSource<bool>> pending = new(); private int sequence; private bool stopping;
    private static readonly JsonSerializerOptions Json = new() { PropertyNameCaseInsensitive = true, PropertyNamingPolicy = JsonNamingPolicy.CamelCase };
    public static void VerifyAssets(string root)
    {
        using Stream resource = Assembly.GetExecutingAssembly().GetManifestResourceStream("assets.manifest.json") ?? throw new IOException("The asset manifest is missing");
        var expected = JsonSerializer.Deserialize<System.Collections.Generic.Dictionary<string, string>>(resource) ?? throw new IOException("The asset manifest is invalid");
        foreach (string name in new[] { "usb-relayd.exe", "usb-transfer-lab.apk" })
        {
            if (!expected.TryGetValue(name, out var hash))
                throw new IOException("An asset is missing from the manifest");
            using var file = File.OpenRead(Path.Combine(root, name));
            string actual = Convert.ToHexString(SHA256.HashData(file));
            if (!string.Equals(actual, hash, StringComparison.OrdinalIgnoreCase))
                throw new IOException("Application files changed or are incomplete. Rebuild or reinstall the application.");
        }
    }
    public void Start()
    {
        string root = AppContext.BaseDirectory;
        VerifyAssets(root);
        process = new Process { StartInfo = new ProcessStartInfo(Path.Combine(root, "usb-relayd.exe")) { UseShellExecute = false, CreateNoWindow = true, RedirectStandardInput = true, RedirectStandardOutput = true, RedirectStandardError = true, WorkingDirectory = root } };
        process.StartInfo.ArgumentList.Add("--assets");
        process.StartInfo.ArgumentList.Add(root);
        using (var resource = Assembly.GetExecutingAssembly().GetManifestResourceStream("assets.manifest.json")!)
        {
            var metadata = JsonSerializer.Deserialize<System.Collections.Generic.Dictionary<string, string>>(resource)!;
            string package = metadata.GetValueOrDefault("companionPackage", "dev.usbreverse.lab");
            string state = metadata.GetValueOrDefault("stateDirectory", "UsbReverseTether");
            if ((package != "dev.usbreverse.lab" && package != "io.github.enesoglu.usbrelay") || (state != "UsbReverseTether" && state != "UsbRelay"))
                throw new IOException("Invalid release configuration");
            process.StartInfo.ArgumentList.Add("--companion-package");
            process.StartInfo.ArgumentList.Add(package);
            process.StartInfo.ArgumentList.Add("--data");
            process.StartInfo.ArgumentList.Add(Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), state));
        }
        process.Start();
        _ = ReadOutput();
        _ = DrainErrors();
    }
    private async Task ReadOutput()
    {
        try
        {
            while (process != null && await process.StandardOutput.ReadLineAsync() is string line)
            {
                if (line.Length > 1048576)
                    throw new IOException("Control message exceeded limit");
                using var doc = JsonDocument.Parse(line);
                var root = doc.RootElement;
                string? type = root.GetProperty("type").GetString();
                if (type == "snapshot")
                {
                    Snapshot? value = root.GetProperty("state").Deserialize<Snapshot>(Json);
                    if (value != null)
                        Changed?.Invoke(value);
                }
                else if (type == "fatal")
                {
                    Failed?.Invoke(root.GetProperty("message").GetString() ?? "Private manager could not start");
                }
                else if (type == "result")
                {
                    int id = root.GetProperty("id").GetInt32();
                    if (pending.TryRemove(id, out var completion))
                    {
                        if (root.GetProperty("ok").GetBoolean())
                            completion.TrySetResult(true);
                        else
                            completion.TrySetException(new IOException(root.GetProperty("error").GetString() ?? "Operation failed"));
                    }
                }
            }
        }
        catch (Exception) { if (!stopping) Failed?.Invoke("The private connection manager stopped unexpectedly."); }
        finally { if (!stopping) Failed?.Invoke("The connection manager is unavailable."); foreach (var item in pending.Values) item.TrySetException(new IOException("Manager disconnected")); }
    }
    private async Task DrainErrors()
    {
        try
        {
            while (process != null && await process.StandardError.ReadLineAsync() != null)
            {
            }
        }
        catch (Exception) { }
    }
    public async Task Send(string method, string? serial = null, bool remember = false, Settings? settings = null)
    {
        if (process == null || process.HasExited)
            throw new IOException("Connection manager is not running");
        int id = Interlocked.Increment(ref sequence);
        var completion = new TaskCompletionSource<bool>(TaskCreationOptions.RunContinuationsAsynchronously);
        pending[id] = completion;
        try
        {
            string request = JsonSerializer.Serialize(new
            {
                id,
                method,
                serial,
                remember,
                settings
            }, Json);
            await write.WaitAsync();
            try
            {
                await process.StandardInput.WriteLineAsync(request);
                await process.StandardInput.FlushAsync();
            }
            finally { write.Release(); }
            await completion.Task.WaitAsync(TimeSpan.FromSeconds(12));
        }
        finally { pending.TryRemove(id, out _); }
    }
    public async ValueTask DisposeAsync()
    {
        stopping = true;
        if (process != null)
        {
            try
            {
                await process.StandardInput.WriteLineAsync("{\"method\":\"shutdown\"}");
                process.StandardInput.Close();
                await process.WaitForExitAsync().WaitAsync(TimeSpan.FromSeconds(12));
            }
            catch (Exception) { try { if (!process.HasExited) process.Kill(); } catch (Exception) { } }
            process.Dispose();
            process = null;
        }
        write.Dispose();
    }
}
