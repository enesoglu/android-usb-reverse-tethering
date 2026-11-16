package dev.usbreverse.lab;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.content.Intent;
import android.net.VpnService;
import android.os.Handler;
import android.os.Looper;
import android.os.ParcelFileDescriptor;
import android.system.ErrnoException;
import android.system.Os;
import android.system.OsConstants;
import android.system.StructPollfd;
import java.io.BufferedInputStream;
import java.io.BufferedOutputStream;
import java.io.Closeable;
import java.io.DataInputStream;
import java.io.DataOutputStream;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.IOException;
import java.net.InetSocketAddress;
import java.net.Socket;
import java.util.concurrent.atomic.AtomicLong;

/** Owns the user-approved VPN and a protected, authenticated USB loopback tunnel. */
public final class UsbVpnService extends VpnService {
    private static final int NOTICE = 42;
    private final Handler handler = new Handler(Looper.getMainLooper());
    private Session current;
    private final Runnable refresh = new Runnable() {
        @Override public void run() {
            Session session = current;
            if (session != null) {
                String text = "Up " + session.up.get() + " packets / Down " + session.down.get()
                    + " packets / TUN drops " + session.drops.get();
                getSharedPreferences("status",MODE_PRIVATE).edit().putLong("tun_drops",session.drops.get()).putLong("up_packets",session.up.get()).putLong("down_packets",session.down.get()).apply();
                ((NotificationManager) getSystemService(NOTIFICATION_SERVICE)).notify(NOTICE, notification(text));
                handler.postDelayed(this, 2000);
            }
        }
    };

    @Override public int onStartCommand(Intent intent, int flags, int startId) {
        if(intent==null){stopSelf();return START_NOT_STICKY;}
        if(ConnectionPolicy.isStop(intent.getAction())){
            if(ConnectionPolicy.isUserStop(intent.getAction()))getSharedPreferences("secure_connection",MODE_PRIVATE).edit().putBoolean("manual_stop",true).apply();
            Session previous=current;
            if(previous!=null){new Thread(()->{previous.userStop();previous.close();handler.post(()->{if(current==previous)stopSelf();});},"usb-user-stop").start();handler.postDelayed(()->{previous.close();if(current==previous)stopSelf();},750);}else stopSelf();
            return START_NOT_STICKY;
        }
        ConnectionConfig config;
        try{config=ConnectionConfig.load(this);}catch(Exception error){stopSelf();return START_NOT_STICKY;}
        if(current!=null)current.close();
        NotificationManager manager = (NotificationManager) getSystemService(NOTIFICATION_SERVICE);
        manager.createNotificationChannel(new NotificationChannel("usb", "USB connection", NotificationManager.IMPORTANCE_LOW));
        startForeground(NOTICE, notification("Connecting over USB"));
        getSharedPreferences("status", MODE_PRIVATE).edit().putString("state", "connecting").remove("last_error").apply();
        Session session = new Session(config);
        current = session;
        handler.post(refresh);
        new Thread(() -> runSession(session, startId), "usb-vpn-download").start();
        return START_NOT_STICKY;
    }

    private Notification notification(String text) {
        Intent stop = new Intent(this, UsbVpnService.class).setAction("stop");
        PendingIntent action = PendingIntent.getService(this, 0, stop, PendingIntent.FLAG_IMMUTABLE | PendingIntent.FLAG_UPDATE_CURRENT);
        return new Notification.Builder(this, "usb").setContentTitle("USB Reverse Tether")
            .setContentText(text).setSmallIcon(android.R.drawable.stat_sys_upload_done)
            .setOngoing(true).addAction(new Notification.Action.Builder(null, "Disconnect", action).build()).build();
    }

    private void runSession(Session session, int startId) {
        try {
            Socket socket = new Socket();
            session.socket = socket;
            if (session.closed) return;
            session.phase = "protect";
            socket.bind(new InetSocketAddress(0));
            if (!protect(socket)) throw new IOException("cannot protect USB transport");
            session.phase = "connect";
            socket.connect(new InetSocketAddress("127.0.0.1", session.config.port), 10000);
            socket.setTcpNoDelay(true);
            socket.setSoTimeout(10000);
            session.phase="tls";
            socket=TunnelTLS.connect(socket,session.config);session.socket=socket;
            DataOutputStream networkOutput = new DataOutputStream(new BufferedOutputStream(socket.getOutputStream(), 65536));
            session.networkOutput=networkOutput;
            DataInputStream networkInput = new DataInputStream(new BufferedInputStream(socket.getInputStream(), 65536));
            session.phase = "handshake";
            networkOutput.writeInt(0x55525632);
            for (int offset=0; offset<64; offset+=2) networkOutput.writeByte(Integer.parseInt(session.config.nonce.substring(offset, offset+2), 16));
            networkOutput.flush();
            if (networkInput.readInt() != 0x55525632) throw new IOException("relay rejected tunnel");
            socket.setSoTimeout(0);
            session.phase = "establish";
            ParcelFileDescriptor tun = new Builder().setSession("USB Reverse Tether").setMtu(1500)
                .addAddress("10.100.0.2", 32).addRoute("0.0.0.0", 0).addDnsServer(session.config.dns)
                .setBlocking(false).establish();
            if (tun == null) throw new IOException("VPN permission unavailable");
            session.tun = tun;
            session.input = new ParcelFileDescriptor.AutoCloseInputStream(ParcelFileDescriptor.dup(tun.getFileDescriptor()));
            session.output = new ParcelFileDescriptor.AutoCloseOutputStream(ParcelFileDescriptor.dup(tun.getFileDescriptor()));
            if (session.closed) return;
            Thread upload = new Thread(() -> upload(session, networkOutput), "usb-vpn-upload");
            session.upload = upload;
            upload.start();
            session.phase = "transfer";
            getSharedPreferences("status", MODE_PRIVATE).edit().putString("state", "connected").putString("phase", "transfer").apply();
            byte[] packet = new byte[1500];
            while (!session.closed) {
                int size = networkInput.readUnsignedShort();
                if (size < 20 || size > packet.length) throw new IOException("invalid relay frame size");
                networkInput.readFully(packet, 0, size);
                if ((packet[0] >>> 4 & 15) != 4) throw new IOException("invalid relay IP version");
                try {
                    session.output.write(packet, 0, size);
                    session.down.incrementAndGet();
                } catch (IOException error) {
                    if (error.getCause() instanceof ErrnoException && ((ErrnoException) error.getCause()).errno == OsConstants.EAGAIN) {
                        session.drops.incrementAndGet();
                    } else { throw error; }
                }
            }
        } catch (Exception error) {
            // Persist only the exception class, never destination addresses or user traffic.
            if(!session.closed)getSharedPreferences("status",MODE_PRIVATE).edit().putString("last_error",error.getClass().getSimpleName()).putString("phase",session.phase).putString("state","failed").apply();
        } finally {
            session.close();
            handler.post(() -> {
                if (current == session) { current = null; handler.removeCallbacks(refresh); stopForeground(STOP_FOREGROUND_REMOVE); stopSelfResult(startId); }
            });
        }
    }

    private void upload(Session session, DataOutputStream output) {
        try {
            byte[] packet = new byte[1500];
            StructPollfd poll = new StructPollfd();
            poll.fd = session.input.getFD();
            poll.events = (short) OsConstants.POLLIN;
            StructPollfd[] polls = new StructPollfd[]{poll};
            while (!session.closed) {
                if (Os.poll(polls, 1000) <= 0) continue;
                if ((poll.revents & OsConstants.POLLIN) == 0) continue;
                int size = session.input.read(packet);
                if (size < 0) break;
                if (size == 0) continue;
                synchronized(session.writeLock){output.writeShort(size);output.write(packet,0,size);output.flush();}
                session.up.incrementAndGet();
            }
        } catch (Exception error) {
            if (!session.closed) getSharedPreferences("status", MODE_PRIVATE).edit().putString("last_error", error.getClass().getSimpleName()).putString("phase", session.phase).putString("state", "failed").apply();
        } finally { session.close(); }
    }

    @Override public void onRevoke() { Session session=current;if(session!=null)new Thread(()->session.userStop(),"usb-revoked").start();stopSelf(); }
    @Override public void onDestroy() {
        Session session = current;
        current = null;
        if (session != null) session.close();
        getSharedPreferences("status", MODE_PRIVATE).edit().putString("state", "disconnected").apply();
        handler.removeCallbacks(refresh);
        stopForeground(STOP_FOREGROUND_REMOVE);
        super.onDestroy();
    }

    private static final class Session {
        final ConnectionConfig config;
        final Object writeLock=new Object();
        volatile DataOutputStream networkOutput;
        volatile boolean closed;
        volatile String phase = "starting";
        volatile Socket socket;
        volatile ParcelFileDescriptor tun;
        volatile FileInputStream input;
        volatile FileOutputStream output;
        volatile Thread upload;
        final AtomicLong up = new AtomicLong();
        final AtomicLong down = new AtomicLong();
        final AtomicLong drops = new AtomicLong();
        Session(ConnectionConfig config){this.config=config;}
        void userStop(){try{synchronized(writeLock){if(networkOutput!=null){networkOutput.writeShort(0);networkOutput.writeByte(1);networkOutput.flush();}}}catch(IOException ignored){}}
        synchronized void close() {
            closed = true;
            quiet(socket); quiet(input); quiet(output); quiet(tun);
        }
        private void quiet(Closeable resource) { if (resource != null) { try { resource.close(); } catch (IOException ignored) { } } }
    }
}
