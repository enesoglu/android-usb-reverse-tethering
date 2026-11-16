package dev.usbreverse.lab;
import android.content.ContentProvider;
import android.content.ContentValues;
import android.database.Cursor;
import android.database.MatrixCursor;
import android.net.Uri;

/** Read-only, shell-protected health information; never returns nonces or traffic. */
public final class DiagnosticsProvider extends ContentProvider {
 public boolean onCreate(){return true;}
 public Cursor query(Uri uri,String[] projection,String selection,String[] selectionArgs,String sort){
  MatrixCursor cursor=new MatrixCursor(new String[]{"state","phase","error","host_pin","phone_pin","tls_key_signing","tun_drops","up_packets","down_packets"});
  android.content.SharedPreferences status=getContext().getSharedPreferences("status",0);
  String host="",phone="";try{host=ConnectionConfig.load(getContext()).pin;}catch(Exception ignored){}try{phone=TunnelTLS.publicFingerprint();}catch(Exception ignored){}
  cursor.addRow(new Object[]{status.getString("state","disconnected"),status.getString("phase",""),status.getString("last_error",""),host,phone,TunnelTLS.signingSelfTest()?"passed":"failed",status.getLong("tun_drops",0),status.getLong("up_packets",0),status.getLong("down_packets",0)});return cursor;
 }
 public String getType(Uri uri){return "vnd.android.cursor.item/vnd.usbrelay.health";}
 public Uri insert(Uri uri,ContentValues values){throw new UnsupportedOperationException("read only");}
 public int update(Uri uri,ContentValues values,String selection,String[] args){throw new UnsupportedOperationException("read only");}
 public int delete(Uri uri,String selection,String[] args){throw new UnsupportedOperationException("read only");}
}
