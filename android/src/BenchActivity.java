package dev.usbreverse.lab;

import android.app.Activity;
import android.content.Intent;
import android.net.VpnService;
import android.widget.Button;
import android.widget.LinearLayout;
import android.os.Bundle;
import android.view.WindowManager;
import android.widget.TextView;
import java.io.BufferedInputStream;
import java.io.DataInputStream;
import java.io.DataOutputStream;
import java.net.InetSocketAddress;
import java.net.Socket;
import java.nio.ByteBuffer;
import java.nio.channels.SocketChannel;
import java.nio.charset.StandardCharsets;

/** Provides USB VPN controls and an optional synthetic transfer benchmark. */
public final class BenchActivity extends Activity {
    private TextView status;
    private Intent vpnConfiguration;
    private final android.os.Handler statusHandler = new android.os.Handler(android.os.Looper.getMainLooper());
    private volatile Socket activeSocket;
    private volatile boolean destroyed;

    @Override public void onCreate(Bundle state) {
        super.onCreate(state);
        status = new TextView(this);
        status.setTextSize(20);
        status.setPadding(24, 24, 24, 24);
        if (getIntent().getBooleanExtra("vpn", false) || !getIntent().hasExtra("port")) { showVpn(); return; }
        getWindow().addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON);
        setContentView(status);
        final int port = getIntent().getIntExtra("port", 0);
        if (port < 1024 || port > 65535) {
            status.setText("USB Transfer Lab\nRun the PC benchmark to begin.\nNo VPN or network settings are changed.");
            return;
        }
        status.setText("USB Transfer Lab\nReceiving synthetic data over USB...");
        new Thread(() -> receive(port), "usb-benchmark").start();
    }

    private void showVpn() {
        LinearLayout layout = new LinearLayout(this);
        layout.setOrientation(LinearLayout.VERTICAL);
        layout.setOnApplyWindowInsetsListener((view, insets) -> {
            view.setPadding(24 + insets.getSystemWindowInsetLeft(), 24 + insets.getSystemWindowInsetTop(),
                24 + insets.getSystemWindowInsetRight(), 24 + insets.getSystemWindowInsetBottom());
            return insets;
        });
        status.setText("USB Reverse Tether\nExperimental IPv4 USB connection.\nKeep the PC relay running.\nA new VPN replaces your current phone VPN.");
        layout.addView(status);
        Button connect = new Button(this);
        connect.setText("Connect USB VPN");
        layout.addView(connect);
        Button disconnect = new Button(this);
        disconnect.setText("Disconnect");
        layout.addView(disconnect);
        setContentView(layout);
        android.content.SharedPreferences connection = getSharedPreferences("connection",MODE_PRIVATE);
        String key = getIntent().getStringExtra("token");
        int port = getIntent().getIntExtra("port", connection.getInt("port",0));
        if (key != null && key.matches("[0-9a-fA-F]{64}")) connection.edit().putString("token",key).putInt("port",port).apply();
        else key = connection.getString("token",null);
        final boolean configured = key != null && port >= 1024 && port <= 65535;
        connect.setEnabled(configured);
        if (!configured) status.setText("USB Reverse Tether\nStart the PC relay to configure this phone.");
        vpnConfiguration = new Intent(this, UsbVpnService.class).putExtra("port",port).putExtra("token",key);
        connect.setOnClickListener(view -> {
            Intent permission = VpnService.prepare(this);
            if (permission != null) startActivityForResult(permission, 100);
            else startUsbVpn();
        });
        disconnect.setOnClickListener(view -> { stopService(new Intent(this, UsbVpnService.class)); status.setText("USB VPN disconnected"); });
        statusHandler.postDelayed(new Runnable() {
            @Override public void run() {
                if (destroyed) return;
                android.content.SharedPreferences state = getSharedPreferences("status", MODE_PRIVATE);
                String text = "USB VPN: " + state.getString("state", "ready");
                String error = state.getString("last_error", "");
                if (!error.isEmpty()) text += "\nLast error: " + error + " (" + state.getString("phase", "unknown") + ")";
                status.setText(text);
                statusHandler.postDelayed(this, 1000);
            }
        }, 1000);
        if (configured && getIntent().getBooleanExtra("auto_connect", false) && VpnService.prepare(this) == null) startUsbVpn();
    }

    private void startUsbVpn() {
        startForegroundService(vpnConfiguration);
        status.setText("USB VPN starting. See the persistent notification for packet counters.");
    }

    @Override protected void onActivityResult(int request, int result, Intent data) {
        super.onActivityResult(request, result, data);
        if (request == 100 && result == RESULT_OK) startUsbVpn();
        else if (request == 100) status.setText("VPN permission was not granted. Tap Connect to retry.");
    }

    private void receive(int port) {
        final int requestedMode = getIntent().getIntExtra("reader_mode", 1);
        try (SocketChannel channel = requestedMode == 2 ? SocketChannel.open() : null;
             Socket socket = channel == null ? new Socket() : channel.socket()) {
            activeSocket = socket;
            if (destroyed) return;
            int requestedBuffer = getIntent().getIntExtra("receive_buffer", 0);
            if (requestedBuffer > 0) socket.setReceiveBufferSize(requestedBuffer);
            final int effectiveBuffer = socket.getReceiveBufferSize();
            socket.connect(new InetSocketAddress("127.0.0.1", port), 10000);
            socket.setSoTimeout(20000);
            socket.setTcpNoDelay(true);
            DataInputStream input = new DataInputStream(new BufferedInputStream(socket.getInputStream(), 262144));
            DataOutputStream output = new DataOutputStream(socket.getOutputStream());
            if (input.readInt() != 0x55525431) throw new IllegalArgumentException("invalid protocol magic");
            int payloadSize = input.readInt();
            int frames = input.readInt();
            int mode = input.readInt();
            if (payloadSize < 64 || payloadSize > 65507 || frames < 1 || frames > 10000000)
                throw new IllegalArgumentException("invalid test limits");
            byte[] payload = new byte[payloadSize];
            output.writeInt(0x52454144);
            output.flush();
            long start = System.nanoTime();
            long ioNs = 0;
            long parseNs = 0;
            long readCalls = 0;
            long cpuStart = android.os.Debug.threadCpuTimeNanos();
            long sequenceErrors = 0;
            long sampleErrors = 0;
            if (mode == 0) {
                for (int frame = 0; frame < frames; frame++) {
                    if (input.readInt() != frame) sequenceErrors++;
                    input.readFully(payload);
                    if (payload[0] != (byte) 0xA5 || payload[payloadSize - 1] != (byte) 0xA5) sampleErrors++;
                }
            } else if (mode == 1) {
                FrameDecoder decoder = new FrameDecoder(payloadSize);
                byte[] bulk = new byte[262144];
                long remaining = (long) frames * (payloadSize + 4);
                while (remaining > 0) {
                    int count = input.read(bulk, 0, (int) Math.min(remaining, bulk.length));
                    if (count < 0) throw new java.io.EOFException("truncated frames");
                    decoder.accept(bulk, count);
                    remaining -= count;
                }
                if (decoder.frames != frames || !decoder.atBoundary()) throw new IllegalStateException("incomplete frames");
                sequenceErrors = decoder.sequenceErrors;
                sampleErrors = decoder.sampleErrors;
            } else if (mode == 2 && channel != null) {
                FrameDecoder decoder = new FrameDecoder(payloadSize);
                ByteBuffer direct = ByteBuffer.allocateDirect(262144);
                long remaining = (long) frames * (payloadSize + 4);
                while (remaining > 0) {
                    direct.clear();
                    direct.limit((int) Math.min(remaining, direct.capacity()));
                    long beforeRead = System.nanoTime();
                    int count = channel.read(direct);
                    long afterRead = System.nanoTime();
                    ioNs += afterRead - beforeRead;
                    readCalls++;
                    if (count < 0) throw new java.io.EOFException("truncated direct frames");
                    direct.flip();
                    decoder.accept(direct);
                    parseNs += System.nanoTime() - afterRead;
                    remaining -= count;
                }
                if (decoder.frames != frames || !decoder.atBoundary()) throw new IllegalStateException("incomplete direct frames");
                sequenceErrors = decoder.sequenceErrors;
                sampleErrors = decoder.sampleErrors;
            } else {
                throw new IllegalArgumentException("invalid reader mode");
            }
            long elapsed = System.nanoTime() - start;
            long bytes = (long) frames * payloadSize;
            String result = "{\"io_ns\":" + ioNs + ",\"parse_ns\":" + parseNs + ",\"read_calls\":" + readCalls + ",\"cpu_ns\":" + (android.os.Debug.threadCpuTimeNanos() - cpuStart) + ",\"receive_buffer\":" + effectiveBuffer + ",\"payload_bytes\":" + bytes + ",\"frames_received\":" + frames
                + ",\"elapsed_ns\":" + elapsed + ",\"sequence_errors\":" + sequenceErrors
                + ",\"payload_sample_errors\":" + sampleErrors + "}";
            byte[] encoded = result.getBytes(StandardCharsets.UTF_8);
            output.writeInt(encoded.length);
            output.write(encoded);
            output.flush();
            final String display = "USB Transfer Lab\nCompleted\nPayload: " + bytes / 1048576.0
                + " MiB\nRate: " + Math.round(bytes * 8000.0 / elapsed) + " Mbps\nSequence errors: "
                + sequenceErrors + "\nPayload sample errors: " + sampleErrors
                + "\n\nThis is a framed USB test, not Internet or VPN throughput.";
            runOnUiThread(() -> { if (!destroyed) status.setText(display); });
        } catch (Exception error) {
            final String message = "USB Transfer Lab\nTest failed: " + error.getClass().getSimpleName()
                + ": " + error.getMessage();
            runOnUiThread(() -> { if (!destroyed) status.setText(message); });
        } finally {
            activeSocket = null;
        }
    }

    @Override public void onDestroy() {
        destroyed = true;
        statusHandler.removeCallbacksAndMessages(null);
        Socket socket = activeSocket;
        if (socket != null) { try { socket.close(); } catch (Exception ignored) { } }
        super.onDestroy();
    }
}
