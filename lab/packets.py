"""Minimal IPv4/UDP framing for synthetic relay tests, not an Internet stack."""
import socket
import struct


def checksum(data: bytes) -> int:
    if len(data) % 2:
        data += b"\0"
    total = sum(struct.unpack(f"!{len(data) // 2}H", data))
    while total >> 16:
        total = (total & 0xFFFF) + (total >> 16)
    return (~total) & 0xFFFF


def udp_packet(payload: bytes, destination_port: int, source_port: int = 49152) -> bytes:
    if len(payload) > 65507:
        raise ValueError("UDP payload exceeds IPv4 limit")
    if not 1 <= source_port <= 65535 or not 1 <= destination_port <= 65535:
        raise ValueError("invalid port")
    # Gnirehtet maps 10.0.2.2 to the PC loopback; no public traffic is generated.
    udp = struct.pack("!HHHH", source_port, destination_port, 8 + len(payload), 0) + payload
    header = struct.pack("!BBHHHBBH4s4s", 0x45, 0, 20 + len(udp), 0, 0, 64, 17, 0,
                         socket.inet_aton("10.0.0.2"), socket.inet_aton("10.0.2.2"))
    return header[:10] + struct.pack("!H", checksum(header)) + header[12:] + udp


class PacketReader:
    """Parse IP packet boundaries from an arbitrary TCP chunk stream."""
    def __init__(self):
        self.pending = bytearray()

    def feed(self, chunk: bytes) -> list[bytes]:
        self.pending.extend(chunk)
        packets = []
        offset = 0
        while len(self.pending) - offset >= 20:
            version = self.pending[offset] >> 4
            ihl = (self.pending[offset] & 15) * 4
            size = struct.unpack_from("!H", self.pending, offset + 2)[0]
            if version != 4 or ihl < 20 or size < ihl:
                raise ValueError("invalid IPv4 framing")
            if len(self.pending) - offset < size:
                break
            packets.append(bytes(self.pending[offset:offset + size]))
            offset += size
        if offset:
            del self.pending[:offset]
        return packets
