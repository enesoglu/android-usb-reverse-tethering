using System;
using System.IO;
using System.Globalization;
using System.Linq;
using System.Security.Principal;
using System.Text.Json;
using System.Threading;
using System.Threading.Tasks;
using System.Windows;
using Forms = System.Windows.Forms;
using Drawing = System.Drawing;
namespace UsbRelayDesktop;

public partial class App : System.Windows.Application
{
    private Mutex? mutex; private EventWaitHandle? show, disconnectEvent, exitEvent; private RegisteredWaitHandle? registration, disconnectRegistration, exitRegistration; private Forms.NotifyIcon? tray;
    private Drawing.Icon? trayImage; private Backend? backend; private MainWindow? window; private bool exiting, trayTip;
    protected override async void OnStartup(StartupEventArgs e)
    {
        CultureInfo.DefaultThreadCurrentCulture = CultureInfo.GetCultureInfo("en-US");
        CultureInfo.DefaultThreadCurrentUICulture = CultureInfo.GetCultureInfo("en-US");
        base.OnStartup(e);
        if (e.Args.Contains("--self-test"))
        {
            await SelfTest(e.Args);
            return;
        }
        string scope = "Local\\UsbReverseTether." + WindowsIdentity.GetCurrent().User!.Value;
        mutex = new Mutex(false, scope);
        bool owns;
        try
        {
            owns = mutex.WaitOne(0);
        }
        catch (AbandonedMutexException) { owns = true; }
        if (!owns)
        {
            string action = e.Args.Contains("--disconnect") ? ".Disconnect" : e.Args.Contains("--exit") ? ".Exit" : ".Show";
            try
            {
                using var wake = EventWaitHandle.OpenExisting(scope + action);
                wake.Set();
                if (action == ".Exit" && e.Args.Contains("--wait"))
                {
                    try
                    {
                        if (mutex.WaitOne(TimeSpan.FromSeconds(20)))
                            mutex.ReleaseMutex();
                    }
                    catch (AbandonedMutexException) { mutex.ReleaseMutex(); }
                }
            }
            catch (Exception) { }
            Shutdown();
            return;
        }
        if (e.Args.Contains("--disconnect") || e.Args.Contains("--exit"))
        {
            mutex.ReleaseMutex();
            Shutdown();
            return;
        }
        show = new EventWaitHandle(false, EventResetMode.AutoReset, scope + ".Show");
        registration = ThreadPool.RegisterWaitForSingleObject(show, (s, t) => Dispatcher.InvokeAsync(() => window?.ShowWindow()), null, Timeout.Infinite, false);
        disconnectEvent = new EventWaitHandle(false, EventResetMode.AutoReset, scope + ".Disconnect");
        exitEvent = new EventWaitHandle(false, EventResetMode.AutoReset, scope + ".Exit");
        disconnectRegistration = ThreadPool.RegisterWaitForSingleObject(disconnectEvent, (s, t) => Dispatcher.InvokeAsync(async () => { if (window != null) await window.DisconnectAsync(); }), null, Timeout.Infinite, false);
        exitRegistration = ThreadPool.RegisterWaitForSingleObject(exitEvent, (s, t) => Dispatcher.InvokeAsync(async () => await ExitAsync()), null, Timeout.Infinite, false);
        backend = new Backend();
        window = new MainWindow(backend);
        MainWindow = window;
        ConfigureTrayAndWindow();
        var initialSnapshot = new TaskCompletionSource<Snapshot>(TaskCreationOptions.RunContinuationsAsynchronously);
        void OnInitialSnapshot(Snapshot snapshot) => initialSnapshot.TrySetResult(snapshot);
        backend.Changed += OnInitialSnapshot;
        try
        {
            backend.Start();
            var snapshot = await initialSnapshot.Task.WaitAsync(TimeSpan.FromSeconds(12));
            PresentStartup(snapshot.Settings, e.Args.Contains("--background"));
            if (window.IsVisible && !PlatformTools.SetupCompleted)
                new SetupWindow { Owner = window }.ShowDialog();
        }
        catch (Exception error) { window.ShowWindow(); MessageBox.Show(window, error.Message, "USB Relay could not start", MessageBoxButton.OK, MessageBoxImage.Error); }
        finally { backend.Changed -= OnInitialSnapshot; }
    }
    private void PresentStartup(Settings settings, bool background)
    {
        if (!background && !settings.StartMinimized)
            window?.ShowWindow();
    }
    private void ConfigureTrayAndWindow()
    {
        var view = window ?? throw new InvalidOperationException("Window is not initialized");
        using (var iconData = GetResourceStream(new Uri("pack://application:,,,/Assets/usb-relay.ico")).Stream)
        using (var source = new Drawing.Icon(iconData))
        {
            trayImage = (Drawing.Icon)source.Clone();
        }
        tray = new Forms.NotifyIcon { Text = "USB Relay · Private USB Internet", Icon = trayImage, Visible = true };
        var menu = new Forms.ContextMenuStrip();
        menu.Items.Add("Open USB Relay", null, (s, a) => view.ShowWindow());
        menu.Items.Add("Preferences", null, (s, a) => { view.ShowWindow(); view.Page("Settings"); });
        menu.Items.Add("Disconnect", null, async (s, a) => await view.DisconnectAsync());
        menu.Items.Add(new Forms.ToolStripSeparator());
        menu.Items.Add("Exit", null, async (s, a) => await ExitAsync());
        tray.ContextMenuStrip = menu;
        tray.DoubleClick += (s, a) => view.ShowWindow();
        view.Closing += (s, a) => { if (!exiting && view.CurrentSettings.CloseToTray) { a.Cancel = true; HideToTray(); } else if (!exiting) { a.Cancel = true; _ = ExitAsync(); } };
        view.StateChanged += (s, a) => { if (view.WindowState == WindowState.Minimized && view.CurrentSettings.MinimizeToTray) HideToTray(); };
        view.SnapshotApplied += snapshot => { if (tray != null) { string label = "USB Relay · " + snapshot.State.Replace('_', ' '); tray.Text = label[..Math.Min(63, label.Length)]; } };
    }
    private void HideToTray()
    {
        window?.Hide();
        if (!trayTip && tray != null)
        {
            trayTip = true;
            tray.ShowBalloonTip(2500, "USB Relay is still running", "Open the tray icon to reconnect, change preferences or exit.", Forms.ToolTipIcon.Info);
        }
    }
    private async Task ExitAsync()
    {
        if (exiting)
            return;
        exiting = true;
        if (tray != null)
        {
            tray.Visible = false;
            tray.Dispose();
            trayImage?.Dispose();
            trayImage = null;
            tray = null;
        }
        if (backend != null)
            await backend.DisposeAsync();
        registration?.Unregister(null);
        disconnectRegistration?.Unregister(null);
        exitRegistration?.Unregister(null);
        show?.Dispose();
        disconnectEvent?.Dispose();
        exitEvent?.Dispose();
        try
        {
            mutex?.ReleaseMutex();
        }
        catch (ApplicationException) { }
        mutex?.Dispose();
        window?.Close();
        Shutdown();
    }
    private static bool TamperedAssetsAreRejected()
    {
        string folder = Path.Combine(Path.GetTempPath(), "UsbRelayAssetTest-" + Guid.NewGuid().ToString("N"));
        Directory.CreateDirectory(folder);
        try
        {
            foreach (string name in new[] { "usb-relayd.exe", "usb-transfer-lab.apk" })
                File.Copy(Path.Combine(AppContext.BaseDirectory, name), Path.Combine(folder, name));
            using (var file = new FileStream(Path.Combine(folder, "usb-relayd.exe"), FileMode.Open, FileAccess.ReadWrite))
            {
                int first = file.ReadByte();
                file.Position = 0;
                file.WriteByte((byte)(first ^ 1));
            }
            try
            {
                Backend.VerifyAssets(folder);
                return false;
            }
            catch (IOException) { return true; }
        }
        finally
        {
            File.Delete(Path.Combine(folder, "usb-relayd.exe"));
            File.Delete(Path.Combine(folder, "usb-transfer-lab.apk"));
            Directory.Delete(folder);
        }
    }
    private async Task SelfTest(string[] args)
    {
        string output = args.SkipWhile(v => v != "--output").Skip(1).FirstOrDefault() ?? Path.Combine(Path.GetTempPath(), "usb-relay-preview.png");
        var result = new System.Collections.Generic.Dictionary<string, bool>();
        try
        {
            string? officialArchive = args.SkipWhile(v => v != "--platform-tools-archive").Skip(1).FirstOrDefault();
            foreach (var check in PlatformTools.SelfTest(officialArchive))
                result[check.Key] = check.Value;
            result["startup_registry_roundtrip"] = StartupRegistration.SelfTest();
            Backend.VerifyAssets(AppContext.BaseDirectory);
            result["asset_hashes_verified"] = true;
            result["tampered_assets_rejected"] = TamperedAssetsAreRejected();
            var view = new MainWindow(null);
            window = view;
            MainWindow = view;
            trayTip = true;
            ConfigureTrayAndWindow();
            view.Title = "USB Relay · Synthetic UI preview";
            PresentStartup(new Settings { StartMinimized = true, MinimizeToTray = false }, false);
            result["start_minimized_without_window"] = !view.IsVisible && tray!.Visible;
            ((Forms.ToolStripMenuItem)tray!.ContextMenuStrip!.Items[0]).PerformClick();
            result["restore_after_minimized_start"] = view.IsVisible && view.WindowState == WindowState.Normal;
            view.Hide();
            PresentStartup(new Settings(), true);
            result["explicit_background_without_window"] = !view.IsVisible && tray.Visible;
            PresentStartup(new Settings(), false);
            result["normal_start_shows_window"] = view.IsVisible;
            for (int i = 0; i < 30; i++)
                view.Apply(new Snapshot { State = "connected", Message = "Your phone is using this computer's Internet", Serial = "synthetic-device", HostPin = new string('a', 64), Devices = new() { new Device { Serial = "synthetic-device", Model = "Demo Android phone", State = "device", Trusted = true } }, DownloadMbps = 120 + Math.Sin(i * .35) * 60, UploadMbps = 8 + Math.Cos(i * .4) * 3, DownloadBytes = 536870912, UploadBytes = 16777216, UptimeSeconds = 125, ActiveFlows = 12 });
            await Task.Delay(300);
            view.RenderPreview(output);
            view.WindowState = WindowState.Minimized;
            await Task.Delay(100);
            result["minimize_to_tray"] = !view.IsVisible && tray!.Visible;
            ((Forms.ToolStripMenuItem)tray!.ContextMenuStrip!.Items[0]).PerformClick();
            result["restore_window"] = view.IsVisible && view.WindowState == WindowState.Normal;
            result["preview_created"] = File.Exists(output) && new FileInfo(output).Length > 10000;
            exiting = true;
            tray!.Visible = false;
            tray.Dispose();
            trayImage?.Dispose();
            trayImage = null;
            tray = null;
            view.Close();
            File.WriteAllText(Path.ChangeExtension(output, "json"), JsonSerializer.Serialize(result));
            Shutdown(result.Values.All(v => v) ? 0 : 1);
        }
        catch (Exception error) { tray?.Dispose(); File.WriteAllText(Path.ChangeExtension(output, "error.txt"), error.ToString()); Shutdown(1); }
    }
}
