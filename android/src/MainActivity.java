package dev.usbreverse.lab;

import android.app.Activity;
import android.content.Intent;
import android.graphics.Color;
import android.graphics.Typeface;
import android.graphics.drawable.GradientDrawable;
import android.net.VpnService;
import android.os.Bundle;
import android.os.Handler;
import android.os.Looper;
import android.provider.Settings;
import android.view.View;
import android.widget.Button;
import android.widget.LinearLayout;
import android.widget.TextView;

/** Public launcher. Configuration and auto-start extras are intentionally ignored. */
public final class MainActivity extends Activity {
 private final Handler handler=new Handler(Looper.getMainLooper());private TextView state,details;private Button connect;
 private int dp(int value){return Math.round(value*getResources().getDisplayMetrics().density);}
 private TextView text(String value,int size,int color){TextView view=new TextView(this);view.setText(value);view.setTextSize(size);view.setTextColor(color);return view;}
 private final Runnable refresh=new Runnable(){public void run(){
  android.content.SharedPreferences status=getSharedPreferences("status",MODE_PRIVATE);String current=status.getString("state","disconnected");state.setText(current.replace('_',' ').toUpperCase(java.util.Locale.ROOT));
  boolean configured=true;try{ConnectionConfig.load(MainActivity.this);}catch(Exception error){configured=false;}
  connect.setEnabled(configured);connect.setAlpha(configured?1f:.45f);
  details.setText("connected".equals(current)?"TLS 1.3 · Verified computer\nYour Internet traffic is passing through USB.":configured?"Connect from your desktop or tap below.\nA new VPN replaces your current phone VPN.":"Open USB Relay on your computer and choose this phone.");
  String error=status.getString("last_error","");if(!error.isEmpty())details.append("\n"+error);handler.postDelayed(this,1000);
 }};
 @Override public void onCreate(Bundle saved){super.onCreate(saved);
  LinearLayout layout=new LinearLayout(this);layout.setOrientation(LinearLayout.VERTICAL);layout.setBackgroundColor(Color.rgb(13,20,31));layout.setPadding(dp(28),dp(36),dp(28),dp(24));
  layout.setOnApplyWindowInsetsListener((view,insets)->{view.setPadding(dp(28),insets.getSystemWindowInsetTop()+dp(36),dp(28),insets.getSystemWindowInsetBottom()+dp(24));return insets;});
  TextView eyebrow=text("PRIVATE INTERNET · OVER USB",12,Color.rgb(66,211,178));layout.addView(eyebrow);
  TextView title=text("USB Relay",36,Color.WHITE);title.setTypeface(Typeface.DEFAULT,Typeface.BOLD);title.setPadding(0,dp(12),0,dp(36));layout.addView(title);
  state=text("DISCONNECTED",20,Color.rgb(66,211,178));layout.addView(state);
  details=text("",16,Color.rgb(159,176,198));details.setPadding(0,dp(16),0,dp(36));layout.addView(details);
  connect=new Button(this);connect.setText("Connect securely");connect.setTextColor(Color.rgb(8,31,27));GradientDrawable fill=new GradientDrawable();fill.setColor(Color.rgb(66,211,178));fill.setCornerRadius(dp(14));connect.setBackground(fill);layout.addView(connect,new LinearLayout.LayoutParams(-1,dp(56)));
  connect.setOnClickListener(view->{Intent permission=VpnService.prepare(this);if(permission!=null)startActivityForResult(permission,100);else startVpn();});
  Button stop=new Button(this);stop.setText("Disconnect");layout.addView(stop);stop.setOnClickListener(view->startService(new Intent(this,UsbVpnService.class).setAction("stop")));
  Button vpnSettings=new Button(this);vpnSettings.setText("Android VPN settings");layout.addView(vpnSettings);vpnSettings.setOnClickListener(view->startActivity(new Intent(Settings.ACTION_VPN_SETTINGS)));
  Button about=new Button(this);about.setText("About and GPL license");layout.addView(about);about.setOnClickListener(view->AboutDialog.show(this));
  TextView hint=text("For strict protection against Wi-Fi or mobile fallback, enable Always-on VPN and Block connections without VPN in Android settings.",13,Color.rgb(135,153,178));hint.setPadding(0,dp(24),0,0);layout.addView(hint);setContentView(layout);
  getWindow().getDecorView().setSystemUiVisibility(0);
 }
 private void startVpn(){getSharedPreferences("secure_connection",MODE_PRIVATE).edit().putBoolean("manual_stop",false).apply();startForegroundService(new Intent(this,UsbVpnService.class).setAction("connect"));}
 @Override protected void onActivityResult(int request,int result,Intent data){super.onActivityResult(request,result,data);if(request==100&&result==RESULT_OK)startVpn();}
 @Override public void onResume(){super.onResume();handler.post(refresh);}
 @Override public void onPause(){handler.removeCallbacks(refresh);super.onPause();}
}
