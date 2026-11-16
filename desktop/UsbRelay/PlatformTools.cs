using System;
using System.Collections.Generic;
using System.IO;
using System.IO.Compression;
using System.Linq;
using System.Net.Http;
using System.Reflection;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
using System.Threading;
using System.Threading.Tasks;

namespace UsbRelayDesktop;

internal static class PlatformTools
{
    internal sealed class Manifest
    {
        public string Version { get; set; } = "";
        public string Url { get; set; } = "";
        public string Sha256 { get; set; } = "";
        public string TermsUrl { get; set; } = "";
        public string DownloadPage { get; set; } = "";
    }
    internal static Manifest Config
    {
        get
        {
            using Stream stream = Assembly.GetExecutingAssembly().GetManifestResourceStream("platform-tools.json") ?? throw new IOException("Platform-tools metadata is missing");
            return JsonSerializer.Deserialize<Manifest>(stream, new JsonSerializerOptions { PropertyNameCaseInsensitive = true }) ?? throw new IOException("Invalid platform-tools metadata");
        }
    }
    internal static string DataRoot => Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "UsbRelay");
    internal static string InstallRoot => Path.Combine(DataRoot, "tools", "platform-tools-" + Config.Version);
    internal static bool SetupCompleted => File.Exists(Path.Combine(DataRoot, "setup-completed"));
    internal static void CompleteSetup()
    {
        Directory.CreateDirectory(DataRoot);
        File.WriteAllText(Path.Combine(DataRoot, "setup-completed"), "1");
    }
    internal static string FindExisting()
    {
        var paths = new List<string> {
            Path.Combine(InstallRoot,"platform-tools","adb.exe"),
            Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),"Android","Sdk","platform-tools","adb.exe")
        };
        string? sdk = Environment.GetEnvironmentVariable("ANDROID_HOME");
        if (!string.IsNullOrWhiteSpace(sdk) && Path.IsPathFullyQualified(sdk))
            paths.Add(Path.Combine(sdk, "platform-tools", "adb.exe"));
        foreach (string folder in (Environment.GetEnvironmentVariable("PATH") ?? "").Split(Path.PathSeparator))
        {
            string candidate = folder.Trim().Trim('"');
            if (Path.IsPathFullyQualified(candidate))
                paths.Add(Path.Combine(candidate, "adb.exe"));
        }
        return paths.FirstOrDefault(path => Path.IsPathFullyQualified(path) && File.Exists(path)) ?? "";
    }
    internal static async Task DownloadAsync(IProgress<double> progress, CancellationToken cancel)
    {
        var config = Config;
        var uri = new Uri(config.Url);
        if (uri.Scheme != "https" || uri.Host != "dl.google.com")
            throw new IOException("Unexpected download source");
        if (Directory.Exists(InstallRoot))
            throw new IOException("This platform-tools version is already installed. Choose the existing adb.exe in Preferences.");
        string parent = Path.GetDirectoryName(InstallRoot)!;
        Directory.CreateDirectory(parent);
        string archive = Path.Combine(parent, "download-" + Guid.NewGuid().ToString("N") + ".zip");
        try
        {
            using var client = new HttpClient { Timeout = TimeSpan.FromMinutes(3) };
            using var response = await client.GetAsync(uri, HttpCompletionOption.ResponseHeadersRead, cancel);
            response.EnsureSuccessStatusCode();
            long? length = response.Content.Headers.ContentLength;
            if (length > 64 * 1024 * 1024)
                throw new IOException("Download exceeds size limit");
            using (var input = await response.Content.ReadAsStreamAsync(cancel))
            using (var output = new FileStream(archive, FileMode.CreateNew, FileAccess.Write, FileShare.None))
            {
                byte[] buffer = new byte[65536];
                long total = 0;
                int count;
                while ((count = await input.ReadAsync(buffer, cancel)) > 0)
                {
                    total += count;
                    if (total > 64 * 1024 * 1024)
                        throw new IOException("Download exceeds size limit");
                    await output.WriteAsync(buffer.AsMemory(0, count), cancel);
                    if (length > 0)
                        progress.Report(total * 100.0 / length.Value);
                }
            }
            cancel.ThrowIfCancellationRequested();
            ExtractVerified(archive, config.Sha256, InstallRoot);
            progress.Report(100);
        }
        finally { File.Delete(archive); }
    }
    internal static void ExtractVerified(string archive, string expectedHash, string destination)
    {
        using (var input = File.OpenRead(archive))
        {
            string actual = Convert.ToHexString(SHA256.HashData(input));
            if (!string.Equals(actual, expectedHash, StringComparison.OrdinalIgnoreCase))
                throw new IOException("Google platform-tools checksum mismatch. Nothing was installed.");
        }
        destination = Path.GetFullPath(destination);
        if (Directory.Exists(destination) || File.Exists(destination))
            throw new IOException("Refusing to overwrite an existing tools directory");
        string parent = Path.GetDirectoryName(destination)!;
        Directory.CreateDirectory(parent);
        string staging = Path.Combine(parent, ".pending-" + Guid.NewGuid().ToString("N"));
        Directory.CreateDirectory(staging);
        try
        {
            using var zip = ZipFile.OpenRead(archive);
            if (zip.Entries.Count > 128)
                throw new InvalidDataException("Too many archive entries");
            long total = 0;
            foreach (var entry in zip.Entries)
            {
                total = checked(total + entry.Length);
                if (total > 128 * 1024 * 1024)
                    throw new InvalidDataException("Extracted tools exceed size limit");
                string name = entry.FullName;
                if (!name.StartsWith("platform-tools/", StringComparison.Ordinal) || name.Contains('\\') || name.Contains(':') || name.Split('/').Any(p => p == ".." || p == "."))
                    throw new InvalidDataException("Unsafe archive path");
                string path = Path.GetFullPath(Path.Combine(staging, name));
                if (!path.StartsWith(staging + Path.DirectorySeparatorChar, StringComparison.OrdinalIgnoreCase))
                    throw new InvalidDataException("Archive path escapes destination");
                if (name.EndsWith('/'))
                {
                    Directory.CreateDirectory(path);
                    continue;
                }
                Directory.CreateDirectory(Path.GetDirectoryName(path)!);
                entry.ExtractToFile(path, false);
            }
            foreach (string name in new[] { "adb.exe", "AdbWinApi.dll", "AdbWinUsbApi.dll" })
                if (!File.Exists(Path.Combine(staging, "platform-tools", name)))
                    throw new InvalidDataException("Required platform-tools files are missing");
            Directory.Move(staging, destination);
        }
        finally { if (Directory.Exists(staging)) RemoveOwnedTestDirectory(staging, parent); }
    }
    private static void RemoveOwnedTestDirectory(string path, string parent)
    {
        path = Path.GetFullPath(path);
        parent = Path.GetFullPath(parent).TrimEnd(Path.DirectorySeparatorChar, Path.AltDirectorySeparatorChar) + Path.DirectorySeparatorChar;
        if (!path.StartsWith(parent, StringComparison.OrdinalIgnoreCase) || (File.GetAttributes(path) & FileAttributes.ReparsePoint) != 0)
            throw new IOException("Unsafe cleanup path");
        Directory.Delete(path, true);
    }
    internal static Dictionary<string, bool> SelfTest(string? officialArchive = null)
    {
        string parent = Path.GetTempPath();
        string root = Path.Combine(parent, "UsbRelayToolsTest-" + Guid.NewGuid().ToString("N"));
        Directory.CreateDirectory(root);
        var result = new Dictionary<string, bool>();
        try
        {
            string archive = Path.Combine(root, "fixture.zip");
            using (var zip = ZipFile.Open(archive, ZipArchiveMode.Create))
                foreach (string name in new[] { "adb.exe", "AdbWinApi.dll", "AdbWinUsbApi.dll" })
                {
                    using var writer = new StreamWriter(zip.CreateEntry("platform-tools/" + name).Open());
                    writer.Write("synthetic fixture");
                }
            string hash;
            using (var stream = File.OpenRead(archive))
                hash = Convert.ToHexString(SHA256.HashData(stream));
            try
            {
                ExtractVerified(archive, new string('0', 64), Path.Combine(root, "bad-hash"));
                result["tools_bad_hash_rejected"] = false;
            }
            catch (IOException) { result["tools_bad_hash_rejected"] = true; }
            ExtractVerified(archive, hash, Path.Combine(root, "valid"));
            result["tools_extraction"] = File.Exists(Path.Combine(root, "valid", "platform-tools", "adb.exe"));
            try
            {
                ExtractVerified(archive, hash, Path.Combine(root, "valid"));
                result["tools_overwrite_rejected"] = false;
            }
            catch (IOException) { result["tools_overwrite_rejected"] = true; }
            string malicious = Path.Combine(root, "unsafe.zip");
            using (var zip = ZipFile.Open(malicious, ZipArchiveMode.Create))
            {
                using var writer = new StreamWriter(zip.CreateEntry("platform-tools/../escape.exe").Open());
                writer.Write("fixture");
            }
            using (var stream = File.OpenRead(malicious))
                hash = Convert.ToHexString(SHA256.HashData(stream));
            try
            {
                ExtractVerified(malicious, hash, Path.Combine(root, "unsafe"));
                result["tools_zip_traversal_rejected"] = false;
            }
            catch (InvalidDataException) { result["tools_zip_traversal_rejected"] = true; }
            if (officialArchive != null)
            {
                ExtractVerified(officialArchive, Config.Sha256, Path.Combine(root, "official"));
                result["official_tools_extraction"] = File.Exists(Path.Combine(root, "official", "platform-tools", "adb.exe"));
            }
            return result;
        }
        finally { RemoveOwnedTestDirectory(root, parent); }
    }
}
