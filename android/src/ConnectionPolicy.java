package dev.usbreverse.lab;

/** Distinguishes user intent from controller cleanup during automatic recovery. */
final class ConnectionPolicy {
    static boolean isStop(String action) {
        return "stop".equals(action) || "stop_remote".equals(action);
    }
    static boolean isUserStop(String action) {
        return "stop".equals(action);
    }
    static boolean suppressBackground(boolean background, boolean manuallyStopped,
                                      String previousAttachment, String attachment) {
        return background && manuallyStopped && attachment.equals(previousAttachment);
    }
}
