package dev.usbreverse.lab;

import java.nio.ByteBuffer;

/** Parses fixed-size synthetic frames across arbitrary transport read boundaries. */
public final class FrameDecoder {
    private final int payloadSize;
    private int headerBytes;
    private int sequence;
    private int payloadRemaining;
    public long frames;
    public long sequenceErrors;
    public long sampleErrors;

    public FrameDecoder(int payloadSize) {
        if (payloadSize < 1) throw new IllegalArgumentException("payload must be positive");
        this.payloadSize = payloadSize;
    }

    public void accept(byte[] buffer, int length) {
        if (length < 0 || length > buffer.length) throw new IllegalArgumentException("invalid read length");
        accept(ByteBuffer.wrap(buffer, 0, length));
    }

    public void accept(ByteBuffer buffer) {
        while (buffer.hasRemaining()) {
            if (headerBytes < 4) {
                sequence = (sequence << 8) | (buffer.get() & 0xFF);
                if (++headerBytes == 4) {
                    if (sequence != frames) sequenceErrors++;
                    payloadRemaining = payloadSize;
                }
            } else {
                int take = Math.min(payloadRemaining, buffer.remaining());
                int offset = buffer.position();
                if (payloadRemaining == payloadSize && buffer.get(offset) != (byte) 0xA5) sampleErrors++;
                if (take == payloadRemaining && buffer.get(offset + take - 1) != (byte) 0xA5) sampleErrors++;
                buffer.position(offset + take);
                payloadRemaining -= take;
                if (payloadRemaining == 0) {
                    frames++;
                    headerBytes = 0;
                    sequence = 0;
                }
            }
        }
    }

    public boolean atBoundary() { return headerBytes == 0 && payloadRemaining == 0; }
}
