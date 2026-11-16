package dev.usbreverse.lab;

import android.app.Activity;
import android.app.AlertDialog;
import android.content.Intent;
import android.net.Uri;
import android.widget.ScrollView;
import android.widget.TextView;
import java.io.ByteArrayOutputStream;
import java.io.InputStream;
import java.nio.charset.StandardCharsets;

/** Makes the GPL notices and corresponding-source location visible in the app. */
final class AboutDialog {
    static void show(Activity activity) {
        new AlertDialog.Builder(activity).setTitle("About USB Relay")
            .setMessage("Copyright 2026 USB Relay contributors.\n\nFree software under GNU GPL version 3 only. You may redistribute and modify it under that license. This program comes with no warranty.")
            .setPositiveButton("View license",(dialog,which)->license(activity))
            .setNeutralButton("Source code",(dialog,which)->activity.startActivity(new Intent(Intent.ACTION_VIEW,Uri.parse("https://github.com/enesoglu/android-usb-reverse-tethering"))))
            .setNegativeButton("Close",null).show();
    }
    private static void license(Activity activity) {
        try(InputStream stream=activity.getAssets().open("LICENSE")) {
            ByteArrayOutputStream bytes=new ByteArrayOutputStream();byte[] block=new byte[4096];int count;
            while((count=stream.read(block))>=0)bytes.write(block,0,count);
            TextView text=new TextView(activity);text.setText(new String(bytes.toByteArray(),StandardCharsets.UTF_8));text.setTextIsSelectable(true);text.setPadding(24,16,24,16);
            ScrollView scroll=new ScrollView(activity);scroll.addView(text);
            new AlertDialog.Builder(activity).setTitle("GNU GPL version 3").setView(scroll).setPositiveButton("Close",null).show();
        }catch(Exception error){new AlertDialog.Builder(activity).setMessage("The GPL license is also included in the Windows package and source repository.").setPositiveButton("Close",null).show();}
    }
}
