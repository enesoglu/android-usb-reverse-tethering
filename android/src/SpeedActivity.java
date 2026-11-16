package dev.usbreverse.lab;

import android.app.Activity;
import android.os.Bundle;
import android.widget.TextView;
import java.io.BufferedInputStream;
import java.io.InputStream;
import java.net.DatagramPacket;
import java.net.DatagramSocket;
import java.net.InetAddress;
import java.net.InetSocketAddress;
import java.net.Socket;
import java.nio.ByteBuffer;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collections;
import java.util.concurrent.atomic.AtomicBoolean;
import org.json.JSONObject;

/** Measures receiver-verified VPN goodput and synthetic UDP RTT under load. */
public final class SpeedActivity extends Activity {
    private static final class Pings {
        final ArrayList<Double> ms=new ArrayList<>(); int sent; int lost;
        JSONObject json() throws Exception {
            ArrayList<Double> sorted=new ArrayList<>(ms); Collections.sort(sorted);
            JSONObject out=new JSONObject(); out.put("sent",sent);out.put("lost",lost);out.put("received",ms.size());
            if(!sorted.isEmpty()) {
                out.put("median_ms",sorted.get(sorted.size()/2));
                out.put("p95_ms",sorted.get(Math.min(sorted.size()-1,(int)Math.ceil(sorted.size()*.95)-1)));
                out.put("max_ms",sorted.get(sorted.size()-1));
            }
            return out;
        }
    }
    @Override public void onCreate(Bundle state) {
        super.onCreate(state);
        TextView status=new TextView(this);status.setTextSize(20);status.setPadding(24,80,24,24);
        status.setText("USB VPN speed test\nVerifying every downloaded byte with SHA-256...");setContentView(status);
        int tcp=getIntent().getIntExtra("tcp_port",0),udp=getIntent().getIntExtra("udp_port",0),report=getIntent().getIntExtra("report_port",0);
        int mib=getIntent().getIntExtra("mib",0);String nonce=getIntent().getStringExtra("nonce");
        if(tcp<1024||tcp>65535||udp<1024||udp>65535||report<1024||report>65535||mib<1||mib>512||nonce==null||!nonce.matches("[0-9a-f]{32}")) {status.setText("Invalid speed test configuration");return;}
        new Thread(() -> runTest(tcp,udp,report,mib,nonce,status),"usb-vpn-speed").start();
    }
    private static void ping(DatagramSocket socket,InetAddress address,int port,int sequence,Pings stats) {
        stats.sent++;
        try {
            byte[] payload=new byte[64];Arrays.fill(payload,(byte)0xA5);ByteBuffer.wrap(payload).putInt(sequence);
            long start=System.nanoTime();socket.send(new DatagramPacket(payload,payload.length,address,port));
            byte[] echoed=new byte[64];DatagramPacket reply=new DatagramPacket(echoed,echoed.length);socket.receive(reply);
            if(reply.getLength()==payload.length&&Arrays.equals(payload,echoed)) stats.ms.add((System.nanoTime()-start)/1000000.0);
            else stats.lost++;
        } catch(Exception error) {stats.lost++;}
    }
    private void runTest(int tcp,int udp,int reportPort,int mib,String nonce,TextView status) {
        AtomicBoolean stop=new AtomicBoolean(false);Thread pingThread=null;
        Pings idle=new Pings(),loaded=new Pings();JSONObject result=new JSONObject();
        try(DatagramSocket pinger=new DatagramSocket()) {
            result.put("nonce",nonce);result.put("passed",false);
            pinger.setSoTimeout(500);InetAddress address=InetAddress.getByName("10.0.2.2");
            for(int index=0;index<20;index++) {ping(pinger,address,udp,index,idle);Thread.sleep(50);}
            final InetAddress pingAddress=address;
            pingThread=new Thread(() -> {int sequence=20;while(!stop.get()&&sequence<2020){ping(pinger,pingAddress,udp,sequence++,loaded);try{Thread.sleep(50);}catch(InterruptedException error){return;}}},"usb-latency");
            pingThread.start();
            long expected=(long)mib*1024*1024,count=0;MessageDigest digest=MessageDigest.getInstance("SHA-256");
            long start;
            try(Socket socket=new Socket()) {
                socket.connect(new InetSocketAddress(address,tcp),5000);socket.setSoTimeout(15000);
                socket.getOutputStream().write("GET / HTTP/1.0\r\nHost: synthetic.test\r\n\r\n".getBytes(StandardCharsets.US_ASCII));
                InputStream input=new BufferedInputStream(socket.getInputStream(),65536);
                int tail=0,headerBytes=0;
                while(tail!=0x0D0A0D0A) {int value=input.read();if(value<0||++headerBytes>8192)throw new IllegalStateException("invalid HTTP header");tail=(tail<<8)|value;}
                byte[] data=new byte[65536];start=System.nanoTime();
                while(count<expected) {int n=input.read(data,0,(int)Math.min(data.length,expected-count));if(n<0)throw new IllegalStateException("truncated download");digest.update(data,0,n);count+=n;}
            }
            long elapsed=System.nanoTime()-start;
            stop.set(true);pingThread.join(2000);
            byte[] hash=digest.digest();StringBuilder hex=new StringBuilder();for(byte value:hash)hex.append(String.format(java.util.Locale.ROOT,"%02x",value&255));
            result.put("received_bytes",count);result.put("elapsed_ns",elapsed);result.put("payload_mbps",count*8000.0/elapsed);
            result.put("sha256",hex.toString());result.put("idle_rtt",idle.json());result.put("loaded_rtt",loaded.json());result.put("passed",true);
        } catch(Exception error) {
            try {result.put("error",error.getClass().getSimpleName());}catch(Exception ignored){}
        } finally {stop.set(true);if(pingThread!=null){try{pingThread.join(2000);}catch(InterruptedException ignored){}}}
        try {
            getSharedPreferences("status",MODE_PRIVATE).edit().putString("speed_result",result.toString()).apply();
            try(Socket report=new Socket()) {
                report.connect(new InetSocketAddress("127.0.0.1",reportPort),5000);report.setSoTimeout(10000);
                report.getOutputStream().write((result.toString()+"\n").getBytes(StandardCharsets.UTF_8));
                if(report.getInputStream().read()!=79||report.getInputStream().read()!=75)throw new IllegalStateException("report not acknowledged");
            }
            final String text="USB VPN speed test completed\n"+result.optDouble("payload_mbps",0)+" Mbps\nSee PC report for SHA-256 and RTT results.";
            runOnUiThread(() -> status.setText(text));
        } catch(Exception error){runOnUiThread(() -> status.setText("Speed report failed: "+error.getClass().getSimpleName()));}
    }
}
