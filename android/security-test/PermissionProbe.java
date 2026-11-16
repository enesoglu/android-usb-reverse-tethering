package dev.usbreverse.securitytest;
import android.app.Instrumentation;
import android.content.ComponentName;
import android.content.Intent;
import android.net.Uri;
import android.os.Bundle;

/** Exercises exported entry points as an ordinary app UID, without shell identity. */
public final class PermissionProbe extends Instrumentation {
 @Override public void onCreate(Bundle args){super.onCreate(args);start();}
 @Override public void onStart(){Bundle result=new Bundle();boolean provision=false,diagnostics=false,benchmark=false,stop=false;
  try{getTargetContext().startActivity(new Intent().setComponent(new ComponentName("dev.usbreverse.lab","dev.usbreverse.lab.ProvisionActivity")).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK).putExtra("config","e30="));}catch(SecurityException expected){provision=true;}catch(Exception unexpected){}
  try{getTargetContext().getContentResolver().query(Uri.parse("content://dev.usbreverse.lab.diagnostics/status"),null,null,null,null);}catch(SecurityException expected){diagnostics=true;}catch(Exception unexpected){}
  try{getTargetContext().startActivity(new Intent().setComponent(new ComponentName("dev.usbreverse.lab","dev.usbreverse.lab.SpeedActivity")).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK));}catch(SecurityException expected){benchmark=true;}catch(Exception unexpected){}
  try{getTargetContext().startActivity(new Intent().setComponent(new ComponentName("dev.usbreverse.lab","dev.usbreverse.lab.ProvisionActivity")).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK).putExtra("stop",true));}catch(SecurityException expected){stop=true;}catch(Exception unexpected){}
  result.putBoolean("untrusted_stop_denied",stop);
  result.putBoolean("untrusted_provision_denied",provision);result.putBoolean("untrusted_diagnostics_denied",diagnostics);result.putBoolean("untrusted_benchmark_denied",benchmark);result.putBoolean("passed",provision&&diagnostics&&benchmark&&stop);finish(provision&&diagnostics&&benchmark&&stop?0:1,result);
 }
}
