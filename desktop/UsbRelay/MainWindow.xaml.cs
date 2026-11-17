using System;
using System.Collections.Generic;
using System.Globalization;
using System.IO;
using System.Linq;
using System.Runtime.InteropServices;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Interop;
using System.Windows.Media;
using System.Windows.Media.Imaging;
namespace UsbRelayDesktop;

public partial class MainWindow : Window
{
    private readonly Backend? backend; private Snapshot current = new(); private bool editing, applying; private readonly Queue<(double down, double up)> history = new(); public event Action<Snapshot>? SnapshotApplied;
    [DllImport("dwmapi.dll")] private static extern int DwmSetWindowAttribute(IntPtr handle, int attribute, ref int value, int size);
    public MainWindow(Backend? backend)
    {
        InitializeComponent();
        Height = Math.Max(MinHeight, Math.Min(Height, SystemParameters.WorkArea.Height - 40));
        this.backend = backend;
        SourceInitialized += (s, e) => { int dark = 1; DwmSetWindowAttribute(new WindowInteropHelper(this).Handle, 20, ref dark, 4); };
        if (backend != null)
        {
            backend.Changed += snapshot => Dispatcher.InvokeAsync(() => Apply(snapshot));
            backend.Failed += message => Dispatcher.InvokeAsync(() => { HeroMessage.Text = message; HeroTitle.Text = "Connection manager needs attention"; StatusText.Text = "UNAVAILABLE"; ShowWindow(); });
        }
    }
    public Settings CurrentSettings => current.Settings;
    public void ShowWindow()
    {
        if (!IsVisible)
            Show();
        WindowState = WindowState.Normal;
        Activate();
    }
    public void Apply(Snapshot snapshot)
    {
        current = snapshot;
        HeroMessage.Text = snapshot.Message;
        StatusText.Text = snapshot.State.Replace('_', ' ').ToUpperInvariant();
        bool connected = snapshot.State == "connected";
        StatusBadge.Background = new SolidColorBrush(connected ? Color.FromRgb(23, 68, 57) : Color.FromRgb(37, 55, 75));
        StatusText.Foreground = new SolidColorBrush(connected ? Color.FromRgb(89, 226, 187) : Color.FromRgb(165, 184, 209));
        HeroTitle.Text = connected ? "Connected. Privately." : snapshot.State == "reconnecting" ? "We'll bring it back." : snapshot.State == "waiting_for_phone" ? "One quick check on your phone" : "Ready when you are";
        string? selected = (DeviceSelector.SelectedItem as Device)?.Serial;
        DeviceSelector.ItemsSource = snapshot.Devices;
        DeviceSelector.IsEnabled = snapshot.Devices.Count > 0;
        DeviceSelector.SelectedItem = snapshot.Devices.FirstOrDefault(d => d.Serial == selected) ?? snapshot.Devices.FirstOrDefault(d => d.Serial == snapshot.Serial) ?? snapshot.Devices.FirstOrDefault();
        UpdateButtons();
        DisconnectButton.IsEnabled = snapshot.Serial != "";
        DownloadText.Text = snapshot.DownloadMbps.ToString("F1", CultureInfo.CurrentCulture);
        UploadText.Text = snapshot.UploadMbps.ToString("F1", CultureInfo.CurrentCulture);
        DownloadTotal.Text = Size(snapshot.DownloadBytes) + " received";
        UploadTotal.Text = Size(snapshot.UploadBytes) + " sent";
        UptimeText.Text = connected ? TimeSpan.FromSeconds(snapshot.UptimeSeconds).ToString(@"hh\:mm\:ss") : "—";
        EncryptionText.Text = connected ? "TLS 1.3 · Paired phone" : "TLS 1.3 when connected";
        PressureText.Text = "Queue rejections: " + snapshot.QueueRejections.ToString("N0");
        HostPinText.Text = snapshot.HostPin;
        TrustedList.ItemsSource = snapshot.Trusted;
        NoTrustedText.Visibility = snapshot.Trusted.Count == 0 ? Visibility.Visible : Visibility.Collapsed;
        history.Enqueue((snapshot.DownloadMbps, snapshot.UploadMbps));
        while (history.Count > 30)
            history.Dequeue();
        DrawChart();
        if (!editing)
        {
            applying = true;
            StartupCheck.IsChecked = snapshot.Settings.StartWithWindows;
            StartMinimizedCheck.IsChecked = snapshot.Settings.StartMinimized;
            AutoConnectCheck.IsChecked = snapshot.Settings.AutoConnect;
            MinimizeCheck.IsChecked = snapshot.Settings.MinimizeToTray;
            CloseTrayCheck.IsChecked = snapshot.Settings.CloseToTray;
            LANCheck.IsChecked = snapshot.Settings.AllowLAN;
            DnsText.Text = snapshot.Settings.DNS;
            AdbPathText.Text = snapshot.Settings.ADBPath;
            applying = false;
        }
        SnapshotApplied?.Invoke(snapshot);
    }
    private static string Size(ulong value)
    {
        if (value >= 1073741824)
            return (value / 1073741824.0).ToString("F2") + " GiB";
        if (value >= 1048576)
            return (value / 1048576.0).ToString("F1") + " MiB";
        if (value >= 1024)
            return (value / 1024.0).ToString("F1") + " KiB";
        return value + " B";
    }
    private void UpdateButtons()
    {
        var d = DeviceSelector.SelectedItem as Device;
        ConnectButton.IsEnabled = d?.State == "device" && string.IsNullOrEmpty(current.Serial);
        ConnectButton.Content = d?.Trusted == true ? "Connect securely" : "Trust & connect";
    }
    private void DeviceChanged(object sender, SelectionChangedEventArgs e) => UpdateButtons();
    private void Navigate(object sender, RoutedEventArgs e)
    {
        if (sender is Button b)
            Page(b.Tag?.ToString() ?? "Connection");
    }
    public void Page(string page)
    {
        ConnectionPage.Visibility = page == "Connection" ? Visibility.Visible : Visibility.Collapsed;
        DevicesPage.Visibility = page == "Devices" ? Visibility.Visible : Visibility.Collapsed;
        SettingsPage.Visibility = page == "Settings" ? Visibility.Visible : Visibility.Collapsed;
        SecurityPage.Visibility = page == "Security" ? Visibility.Visible : Visibility.Collapsed;
    }
    private async Task Invoke(string method, string? serial = null, bool remember = false, Settings? settings = null)
    {
        if (backend == null)
            return;
        try
        {
            await backend.Send(method, serial, remember, settings);
        }
        catch (Exception error) { MessageBox.Show(this, error.Message, "USB Relay", MessageBoxButton.OK, MessageBoxImage.Information); }
    }
    private async void Connect(object sender, RoutedEventArgs e)
    {
        if (DeviceSelector.SelectedItem is not Device d)
            return;
        if (!d.Trusted && MessageBox.Show(this, $"Trust {d.Model} and allow it to reconnect automatically?\n\nOnly continue if this is the phone you connected. Its cryptographic identity will be saved after verification.", "Pair this phone", MessageBoxButton.YesNo, MessageBoxImage.Question) != MessageBoxResult.Yes)
            return;
        await Invoke("connect", d.Serial, true);
    }
    public async Task DisconnectAsync() => await Invoke("disconnect"); private async void Disconnect(object sender, RoutedEventArgs e) => await DisconnectAsync();
    private async void Forget(object sender, RoutedEventArgs e)
    {
        if (sender is Button b && b.Tag is string serial && MessageBox.Show(this, "Forget this phone and stop any active connection?", "Forget phone", MessageBoxButton.YesNo, MessageBoxImage.Question) == MessageBoxResult.Yes)
            await Invoke("forget", serial);
    }
    private void EditSettings(object sender, RoutedEventArgs e)
    {
        if (!applying)
        {
            editing = true;
            SavedText.Text = "Unsaved changes";
        }
    }
    private void DnsEdited(object sender, TextChangedEventArgs e)
    {
        if (IsLoaded && !applying)
        {
            editing = true;
            SavedText.Text = "Unsaved changes";
        }
    }
    private async void SaveSettings(object sender, RoutedEventArgs e)
    {
        if (backend == null)
            return;
        var next = new Settings { StartWithWindows = StartupCheck.IsChecked == true, StartMinimized = StartMinimizedCheck.IsChecked == true, AutoConnect = AutoConnectCheck.IsChecked == true, MinimizeToTray = MinimizeCheck.IsChecked == true, CloseToTray = CloseTrayCheck.IsChecked == true, AllowLAN = LANCheck.IsChecked == true, DNS = DnsText.Text.Trim(), ADBPath = AdbPathText.Text.Trim() };
        string? previous = StartupRegistration.Capture();
        try
        {
            StartupRegistration.Set(next.StartWithWindows, Environment.ProcessPath!);
            await backend.Send("settings", settings: next);
            editing = false;
            SavedText.Text = "Preferences saved";
        }
        catch (Exception error) { try { StartupRegistration.Restore(previous); } catch (Exception) { } MessageBox.Show(this, error.Message, "Preferences were not saved", MessageBoxButton.OK, MessageBoxImage.Information); }
    }
    private void BrowseAdb(object sender, RoutedEventArgs e)
    {
        var dialog = new Microsoft.Win32.OpenFileDialog { Filter = "Android Debug Bridge|adb.exe", Title = "Choose adb.exe" };
        if (dialog.ShowDialog(this) == true)
            AdbPathText.Text = dialog.FileName;
    }
    private void CopyPin(object sender, RoutedEventArgs e)
    {
        if (current.HostPin != "")
            Clipboard.SetText(current.HostPin);
    }
    private void CopyDiagnostics(object sender, RoutedEventArgs e) => Clipboard.SetText($"USB Relay 0.3.0-alpha.1\nState: {current.State}\nEncryption: TLS 1.3 mutual authentication\nActive flows: {current.ActiveFlows}\nQueue rejections: {current.QueueRejections}\nPhones connected: {current.Devices.Count}\nTraffic and device identifiers omitted.");
    private void Help(object sender, RoutedEventArgs e) => new SetupWindow { Owner = this }.ShowDialog();
    private void OpenLicense(object sender, RoutedEventArgs e) => SetupWindow.Open(Path.Combine(AppContext.BaseDirectory, "LICENSE.txt"));
    private void OpenSource(object sender, RoutedEventArgs e) => SetupWindow.Open("https://github.com/enesoglu/android-usb-reverse-tethering");
    private void ChartSizeChanged(object sender, SizeChangedEventArgs e) => DrawChart();
    private void DrawChart()
    {
        if (TrafficCanvas == null || history.Count == 0)
            return;
        double width = Math.Max(1, TrafficCanvas.ActualWidth), height = TrafficCanvas.Height - 8, max = Math.Max(1, history.Max(p => Math.Max(p.down, p.up)));
        var down = new PointCollection();
        var up = new PointCollection();
        int index = 30 - history.Count;
        foreach (var value in history)
        {
            double x = index++ * width / 29;
            down.Add(new Point(x, height - value.down / max * (height - 5)));
            up.Add(new Point(x, height - value.up / max * (height - 5)));
        }
        DownloadLine.Points = down;
        UploadLine.Points = up;
    }
    public void RenderPreview(string path)
    {
        UpdateLayout();
        var visual = (FrameworkElement)Content;
        var bitmap = new RenderTargetBitmap((int)visual.ActualWidth, (int)visual.ActualHeight, 96, 96, PixelFormats.Pbgra32);
        bitmap.Render(visual);
        var encoder = new PngBitmapEncoder();
        encoder.Frames.Add(BitmapFrame.Create(bitmap));
        Directory.CreateDirectory(Path.GetDirectoryName(Path.GetFullPath(path))!);
        using var output = File.Create(path);
        encoder.Save(output);
    }
}
