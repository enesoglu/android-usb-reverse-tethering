using System;
using System.Diagnostics;
using System.IO;
using System.Threading;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Media;

namespace UsbRelayDesktop;

internal sealed class SetupWindow : Window
{
    private readonly CancellationTokenSource cancel = new();
    private readonly TextBlock status;
    private readonly Button download;
    private readonly CheckBox consent;
    private readonly ProgressBar progress;
    internal SetupWindow()
    {
        Title = "USB Relay · Setup";
        Width = 720;
        Height = Math.Min(790, SystemParameters.WorkArea.Height - 40);
        MinHeight = 500;
        WindowStartupLocation = WindowStartupLocation.CenterOwner;
        Background = new SolidColorBrush(Color.FromRgb(13, 22, 35));
        Foreground = Brushes.White;
        var content = new StackPanel { Margin = new Thickness(30) };
        Content = new ScrollViewer { Content = content, VerticalScrollBarVisibility = ScrollBarVisibility.Auto };
        AddText(content, "Get connected in a few steps", 28, true);
        AddText(content, "Share this Windows PC's internet with your Android phone over USB. No root or Wi-Fi hotspot is needed.", 14);
        AddText(content, "1 · Prepare this computer", 20, true);
        string found = PlatformTools.FindExisting();
        status = AddText(content, found != "" ? "Google platform tools detected. You can continue to the phone steps." : "ADB is needed to reach your phone. Download the verified Google tools below, or choose your own adb.exe in Preferences.", 14);
        consent = new CheckBox { Content = "I have read and accept the Android SDK terms for this download.", Margin = new Thickness(0, 12, 0, 8), Foreground = Brushes.White };
        content.Children.Add(consent);
        var terms = new Button { Content = "Read Google's SDK terms", HorizontalAlignment = HorizontalAlignment.Left, Margin = new Thickness(0, 0, 0, 8) };
        terms.Click += (s, e) => Open(PlatformTools.Config.TermsUrl);
        content.Children.Add(terms);
        download = new Button { Content = "Download Google platform tools", HorizontalAlignment = HorizontalAlignment.Left, IsEnabled = found == "" };
        download.Click += Download;
        content.Children.Add(download);
        progress = new ProgressBar { Minimum = 0, Maximum = 100, Height = 7, Margin = new Thickness(0, 12, 0, 6) };
        content.Children.Add(progress);
        AddText(content, "Downloaded directly from Google, verified against a pinned SHA-256, and installed for your Windows account. Not included in this project's download.", 12);
        AddText(content, "2 · Prepare your Android phone", 20, true);
        AddText(content, "Open Settings → About phone → Build number and tap it seven times. On Samsung, Build number is under Software information. Enter your phone PIN if asked. Then enable USB debugging in Developer options.", 14);
        AddText(content, "3 · Connect and authorize", 20, true);
        AddText(content, "Connect a data-capable USB cable, unlock the phone and approve its Allow USB debugging prompt. Only authorize a computer you trust. Choose the phone in USB Relay and click Trust & connect.", 14);
        AddText(content, "4 · Approve Android's VPN", 20, true);
        AddText(content, "On the phone, open USB Relay, tap Connect securely and approve Android's VPN request. This replaces another active VPN. These security prompts cannot be bypassed automatically.", 14);
        var guide = new Button { Content = "Open the detailed offline guide", HorizontalAlignment = HorizontalAlignment.Left, Margin = new Thickness(0, 16, 0, 8) };
        guide.Click += (s, e) => Open(Path.Combine(AppContext.BaseDirectory, "START-HERE.html"));
        content.Children.Add(guide);
        var done = new Button { Content = "Done · show my devices", HorizontalAlignment = HorizontalAlignment.Right, Margin = new Thickness(0, 8, 0, 0) };
        done.Click += (s, e) => { PlatformTools.CompleteSetup(); Close(); };
        content.Children.Add(done);
        Closed += (s, e) => cancel.Cancel();
    }
    private static TextBlock AddText(Panel panel, string text, int size, bool bold = false)
    {
        var block = new TextBlock { Text = text, FontSize = size, FontWeight = bold ? FontWeights.SemiBold : FontWeights.Normal, TextWrapping = TextWrapping.Wrap, Margin = new Thickness(0, bold ? 18 : 8, 0, 4), Foreground = bold ? Brushes.White : new SolidColorBrush(Color.FromRgb(167, 186, 209)) };
        panel.Children.Add(block);
        return block;
    }
    private async void Download(object sender, RoutedEventArgs e)
    {
        if (consent.IsChecked != true)
        {
            status.Text = "Read the SDK terms and tick the consent box before downloading. You can also use your own adb.exe.";
            return;
        }
        download.IsEnabled = false;
        try
        {
            status.Text = "Downloading and verifying Google's platform tools…";
            await PlatformTools.DownloadAsync(new Progress<double>(value => progress.Value = value), cancel.Token);
            status.Text = "Platform tools are ready. The app will detect your authorized USB phone automatically.";
        }
        catch (OperationCanceledException) { status.Text = "Download cancelled. No incomplete tools were installed."; download.IsEnabled = true; }
        catch (Exception error) { status.Text = error.Message; download.IsEnabled = true; }
    }
    internal static void Open(string target)
    {
        try
        {
            Process.Start(new ProcessStartInfo(target) { UseShellExecute = true });
        }
        catch (Exception) { MessageBox.Show("Could not open the guide or website. See the documents included with the app.", "USB Relay", MessageBoxButton.OK, MessageBoxImage.Information); }
    }
}
