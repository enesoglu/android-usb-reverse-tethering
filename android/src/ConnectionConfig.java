package dev.usbreverse.lab;

import android.content.Context;
import org.json.JSONObject;

/** Configuration may only be enrolled by the shell-protected provisioning activity. */
final class ConnectionConfig {
 final int port;final String pin,nonce,dns,attachId;final boolean background;
 ConnectionConfig(String json)throws Exception {
  JSONObject data=new JSONObject(json);port=data.getInt("port");pin=data.getString("pin");nonce=data.getString("nonce");dns=data.getString("dns");attachId=data.getString("attachId");background=data.optBoolean("background",false);
  if(port<1024||port>65535)throw new InvalidField("port");
  if(!pin.matches("[0-9a-f]{64}"))throw new InvalidField("pin");
  if(!nonce.matches("[0-9a-f]{64}"))throw new InvalidField("nonce");
  if(!dns.matches("[0-9]{1,3}(\\.[0-9]{1,3}){3}"))throw new InvalidField("dns");
  if(!attachId.matches("[A-Za-z0-9:._-]{1,128}"))throw new InvalidField("attachment");
  for(String part:dns.split("\\."))if(Integer.parseInt(part)>255)throw new InvalidField("dns");
 }
 static final class InvalidField extends IllegalArgumentException {
  final String field; InvalidField(String field){super("Invalid configuration field");this.field=field;}
 }
 static ConnectionConfig load(Context context)throws Exception{return new ConnectionConfig(context.getSharedPreferences("secure_connection",Context.MODE_PRIVATE).getString("config",""));}
}
