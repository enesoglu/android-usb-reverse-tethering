package dev.usbreverse.lab;

import android.app.Activity;
import android.os.Bundle;
import android.widget.TextView;
import java.io.ByteArrayOutputStream;
import java.io.InputStream;
import java.net.DatagramPacket;
import java.net.DatagramSocket;
import java.net.InetAddress;
import java.net.InetSocketAddress;
import java.net.Socket;
import java.net.URL;
import java.nio.charset.StandardCharsets;
import java.util.Arrays;
import javax.net.ssl.HttpsURLConnection;
import org.json.JSONObject;

/** Runs synthetic transport checks and a public HTTPS connectivity probe. */
public final class ProbeActivity extends Activity {
    @Override public void onCreate(Bundle state) {
        super.onCreate(state);
        TextView status = new TextView(this);
        status.setTextSize(20);
        status.setPadding(24,24,24,24);
        status.setText("USB VPN connectivity check running...");
        setContentView(status);
        final int tcpPort=getIntent().getIntExtra("tcp_port",0);
        final int udpPort=getIntent().getIntExtra("udp_port",0);
        final int reportPort=getIntent().getIntExtra("report_port",0);
        final String nonce=getIntent().getStringExtra("nonce");
        if (tcpPort<1024 || tcpPort>65535 || udpPort<1024 || udpPort>65535 || reportPort<1024 || reportPort>65535
                || nonce==null || !nonce.matches("[0-9a-f]{32}")) { status.setText("Invalid probe configuration"); return; }
        new Thread(() -> {
            JSONObject result=new JSONObject();
            try {
                result.put("nonce",nonce);
                result.put("vpn_state", getSharedPreferences("status",MODE_PRIVATE).getString("state","unknown"));
                result.put("vpn_error", getSharedPreferences("status",MODE_PRIVATE).getString("last_error",""));
                result.put("vpn_phase", getSharedPreferences("status",MODE_PRIVATE).getString("phase",""));
                result.put("tcp",false); result.put("udp",false); result.put("https",false);
                try (Socket socket=new Socket()) {
                    socket.connect(new InetSocketAddress("10.0.2.2",tcpPort),5000);
                    socket.setSoTimeout(5000);
                    socket.getOutputStream().write("GET / HTTP/1.0\r\nHost: synthetic.test\r\n\r\n".getBytes(StandardCharsets.US_ASCII));
                    String response=new String(readBounded(socket.getInputStream()),StandardCharsets.US_ASCII);
                    result.put("tcp",response.startsWith("HTTP/1.0 200") && response.endsWith("usb-relay-health"));
                } catch(Exception error) { result.put("tcp_error",error.getClass().getSimpleName()); }
                try (DatagramSocket socket=new DatagramSocket()) {
                    socket.setSoTimeout(5000);
                    byte[] payload=new byte[1200]; Arrays.fill(payload,(byte)0xA5);
                    DatagramPacket outbound=new DatagramPacket(payload,payload.length,InetAddress.getByName("10.0.2.2"),udpPort);
                    int valid=0;
                    for(int sequence=0;sequence<100;sequence++) {
                        payload[0]=(byte)sequence;
                        socket.send(outbound);
                        byte[] received=new byte[1200];
                        DatagramPacket inbound=new DatagramPacket(received,received.length);
                        socket.receive(inbound);
                        if(inbound.getLength()!=payload.length || !Arrays.equals(payload,received)) break;
                        valid++;
                    }
                    result.put("udp_datagrams_verified",valid); result.put("udp",valid==100);
                } catch(Exception error) { result.put("udp_error",error.getClass().getSimpleName()); }
                HttpsURLConnection https=null;
                try {
                    https=(HttpsURLConnection)new URL("https://example.com/").openConnection();
                    https.setConnectTimeout(10000); https.setReadTimeout(10000);
                    int code=https.getResponseCode();
                    byte[] body=readBounded(https.getInputStream());
                    result.put("https",code==200 && body.length>0); result.put("https_status",code); result.put("https_bytes",body.length);
                } catch(Exception error) { result.put("https_error",error.getClass().getSimpleName()); }
                finally { if(https!=null) https.disconnect(); }
                getSharedPreferences("status",MODE_PRIVATE).edit().putString("probe_result",result.toString()).apply();
                try(Socket report=new Socket()) {
                    report.connect(new InetSocketAddress("127.0.0.1",reportPort),5000);
                    report.setSoTimeout(10000);
                    report.getOutputStream().write((result.toString()+"\n").getBytes(StandardCharsets.UTF_8));
                    if (report.getInputStream().read() != 79 || report.getInputStream().read() != 75) throw new IllegalStateException("report not acknowledged");
                }
                final String summary="TCP: "+result.getBoolean("tcp")+"\nUDP: "+result.getBoolean("udp")+"\nHTTPS: "+result.getBoolean("https");
                runOnUiThread(() -> status.setText("USB VPN probe completed\n"+summary));
            } catch(Exception error) {
                runOnUiThread(() -> status.setText("Probe failed: "+error.getClass().getSimpleName()));
            }
        },"usb-vpn-probe").start();
    }
    private static byte[] readBounded(InputStream input) throws Exception {
        ByteArrayOutputStream output=new ByteArrayOutputStream();
        byte[] data=new byte[8192]; int count;
        while((count=input.read(data))!=-1) {
            if(output.size()+count>131072) throw new IllegalStateException("probe response exceeds limit");
            output.write(data,0,count);
        }
        return output.toByteArray();
    }
}
