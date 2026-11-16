package dev.usbreverse.lab;

import android.app.Activity;
import android.content.Intent;
import android.net.VpnService;
import android.os.Bundle;
import android.util.Base64;
import java.nio.charset.StandardCharsets;

/** Exported only with android.permission.DUMP: ordinary apps cannot supply tunnel configuration. */
public final class ProvisionActivity extends Activity {
 @Override public void onCreate(Bundle state){super.onCreate(state);provision();}
 @Override protected void onNewIntent(Intent intent){super.onNewIntent(intent);setIntent(intent);provision();}
 private void provision(){
  String phase="parse";
  try{
   if(getIntent().getBooleanExtra("stop",false)){startService(new Intent(this,UsbVpnService.class).setAction("stop_remote"));finish();return;}

   String encoded=getIntent().getStringExtra("config");
   // Android may restore this transient activity after a package update.
   if(encoded==null){finish();return;}
   if(encoded.length()>4096)throw new IllegalArgumentException();
   phase="decode";String json=new String(Base64.decode(encoded,Base64.DEFAULT),StandardCharsets.UTF_8);phase="validate";ConnectionConfig config=new ConnectionConfig(json);
   android.content.SharedPreferences preferences=getSharedPreferences("secure_connection",MODE_PRIVATE);
   boolean suppressed=ConnectionPolicy.suppressBackground(config.background,preferences.getBoolean("manual_stop",false),preferences.getString("attach_id",""),config.attachId);
   phase="store";
   if(!preferences.edit().putString("config",json).putString("attach_id",config.attachId).putBoolean("manual_stop",suppressed).commit())throw new IllegalStateException();
   phase="start_service";
   if(!suppressed&&VpnService.prepare(this)==null){startForegroundService(new Intent(this,UsbVpnService.class).setAction("connect"));}
   phase="show_controls";
   if(!config.background||VpnService.prepare(this)!=null)startActivity(new Intent(this,MainActivity.class).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK));
  }catch(Exception error){getSharedPreferences("status",MODE_PRIVATE).edit().putString("state","configuration_error").putString("last_error",(error instanceof ConnectionConfig.InvalidField)?"Invalid field: "+((ConnectionConfig.InvalidField)error).field:error.getClass().getSimpleName()).putString("phase",phase).apply();}
  finish();
 }
}
