import socket
import struct
import unittest

from lab.android_bench import frame_batches, read_exact
from lab.packets import PacketReader, checksum, udp_packet


class PacketTests(unittest.TestCase):
    def test_udp_header_and_checksum(self):
        packet = udp_packet(b"hello", 12345)
        self.assertEqual(checksum(packet[:20]), 0)
        self.assertEqual(struct.unpack_from("!H", packet, 2)[0], len(packet))
        self.assertEqual(packet[12:16], socket.inet_aton("10.0.0.2"))
        self.assertEqual(packet[16:20], socket.inet_aton("10.0.2.2"))
        self.assertEqual(struct.unpack_from("!HHHH", packet, 20), (49152, 12345, 13, 0))
        self.assertEqual(packet[28:], b"hello")

    def test_invalid_udp_payload_and_ports(self):
        for payload, port in [(bytes(65508), 80), (b"", 0), (b"", 65536)]:
            with self.subTest(size=len(payload), port=port), self.assertRaises(ValueError):
                udp_packet(payload, port)

    def test_every_possible_tcp_split_preserves_packet_boundaries(self):
        packets = [udp_packet(b"first", 10000), udp_packet(b"second", 10001)]
        wire = b"".join(packets)
        for offset in range(len(wire) + 1):
            reader = PacketReader()
            self.assertEqual(reader.feed(wire[:offset]) + reader.feed(wire[offset:]), packets)
            self.assertEqual(reader.pending, b"")

    def test_corrupt_ip_lengths_are_rejected(self):
        for header, size in [(0x65, 20), (0x44, 20), (0x45, 10)]:
            packet = bytearray(udp_packet(b"", 10000))
            packet[0] = header
            struct.pack_into("!H", packet, 2, size)
            with self.subTest(header=header, size=size), self.assertRaises(ValueError):
                PacketReader().feed(packet)

    def test_frames_preserve_sequences_across_batch_boundary(self):
        batches = list(frame_batches(64, 65))
        self.assertEqual(len(batches), 2)
        wire = b"".join(batches)
        self.assertEqual(len(wire), 65 * 68)
        for sequence in range(65):
            self.assertEqual(struct.unpack_from("!I", wire, sequence * 68)[0], sequence)
            self.assertEqual(wire[sequence * 68 + 4: (sequence + 1) * 68], bytes([0xA5]) * 64)

    def test_read_exact_handles_fragmentation_and_eof(self):
        class Fragmented:
            def __init__(self):
                self.parts = iter([b"a", b"bc", b""])
            def recv(self, count):
                return next(self.parts)
        self.assertEqual(read_exact(Fragmented(), 3), b"abc")
        with self.assertRaises(ConnectionError):
            read_exact(Fragmented(), 4)


class DeliveryValidationTests(unittest.TestCase):
    def test_sender_success_cannot_substitute_for_delivery(self):
        from lab.native_control import validated_rate
        self.assertIsNone(validated_rate(134217728, 0, 0.8))
        self.assertIsNone(validated_rate(134217728, 134217727, 0.8))
        self.assertIsNone(validated_rate(0, 0, 0.8))
        self.assertIsNone(validated_rate(134217728, 134217728, 0))
        self.assertEqual(validated_rate(1000000, 1000000, 1), 8.0)


if __name__ == "__main__":
    unittest.main()
