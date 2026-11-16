import dev.usbreverse.lab.FrameDecoder;
import java.io.ByteArrayOutputStream;
import java.io.DataOutputStream;
import java.nio.ByteBuffer;
import java.util.Arrays;
import java.util.Random;

public final class FrameDecoderTest {
    private static void check(boolean condition, String message) {
        if (!condition) throw new AssertionError(message);
    }
    private static byte[] fixture(int payloadSize, int frames) throws Exception {
        ByteArrayOutputStream bytes = new ByteArrayOutputStream();
        DataOutputStream output = new DataOutputStream(bytes);
        byte[] payload = new byte[payloadSize];
        Arrays.fill(payload, (byte) 0xA5);
        for (int sequence=0; sequence<frames; sequence++) {
            output.writeInt(sequence);
            output.write(payload);
        }
        return bytes.toByteArray();
    }
    public static void main(String[] args) throws Exception {
        int cases = 0;
        for (int payloadSize : new int[]{1, 64, 1200, 16384}) {
            byte[] wire = fixture(payloadSize, 5);
            for (int split=0; split<=Math.min(wire.length, 2000); split++) {
                FrameDecoder decoder = new FrameDecoder(payloadSize);
                decoder.accept(Arrays.copyOfRange(wire, 0, split), split);
                decoder.accept(Arrays.copyOfRange(wire, split, wire.length), wire.length - split);
                check(decoder.frames == 5 && decoder.atBoundary(), "fragmented frames incomplete");
                check(decoder.sequenceErrors == 0 && decoder.sampleErrors == 0, "fragmentation corrupted frames");
                cases++;
            }
            FrameDecoder directDecoder = new FrameDecoder(payloadSize);
            Random random = new Random(481516);
            int offset = 0;
            while (offset < wire.length) {
                int length = Math.min(1 + random.nextInt(257), wire.length - offset);
                ByteBuffer direct = ByteBuffer.allocateDirect(length);
                direct.put(wire, offset, length).flip();
                directDecoder.accept(direct);
                offset += length;
            }
            check(directDecoder.frames == 5 && directDecoder.atBoundary(), "direct frames incomplete");
            check(directDecoder.sequenceErrors == 0 && directDecoder.sampleErrors == 0, "direct fragmentation corrupted frames");
            cases++;
        }
        byte[] badSequence = fixture(64, 2);
        badSequence[3] = 1;
        FrameDecoder reordered = new FrameDecoder(64);
        reordered.accept(badSequence, badSequence.length);
        check(reordered.sequenceErrors == 1, "sequence corruption was not detected");
        byte[] corruptPayload = fixture(64, 2);
        corruptPayload[4] = 0;
        FrameDecoder corrupt = new FrameDecoder(64);
        corrupt.accept(corruptPayload, corruptPayload.length);
        check(corrupt.sampleErrors > 0, "sample corruption was not detected");
        FrameDecoder truncated = new FrameDecoder(64);
        truncated.accept(corruptPayload, corruptPayload.length - 1);
        check(truncated.frames == 1 && !truncated.atBoundary(), "truncation was not detected");
        System.out.println("PASS: " + (cases + 3) + " decoder cases");
    }
}
