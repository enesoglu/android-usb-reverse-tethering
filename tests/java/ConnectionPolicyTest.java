package dev.usbreverse.lab;

public final class ConnectionPolicyTest {
    public static void main(String[] args) {
        if (!ConnectionPolicy.isStop("stop_remote") || ConnectionPolicy.isUserStop("stop_remote"))
            throw new AssertionError("Controller cleanup must allow same-attachment recovery");
        if (!ConnectionPolicy.isStop("stop") || !ConnectionPolicy.isUserStop("stop"))
            throw new AssertionError("Explicit user stop must suppress automatic recovery");
        if (ConnectionPolicy.isStop(null) || ConnectionPolicy.isStop("connect"))
            throw new AssertionError("Only stop actions may disconnect");
        for (boolean background : new boolean[]{false,true}) {
            for (boolean stopped : new boolean[]{false,true}) {
                for (String next : new String[]{"old","new"}) {
                    boolean expected = background && stopped && next.equals("old");
                    if (ConnectionPolicy.suppressBackground(background,stopped,"old",next) != expected)
                        throw new AssertionError("Incorrect attachment recovery policy");
                }
            }
        }
        System.out.println("PASS: disconnect intent and attachment recovery policy");
    }
}
