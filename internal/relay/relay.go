// Package relay terminates a USB-only packet tunnel in a userspace TCP/IP stack.
package relay

import (
	"bufio"
	"context"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"strconv"
	"sync/atomic"
	"time"

	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
	"gvisor.dev/gvisor/pkg/waiter"
)

// MTU is the largest negotiated packet; v2 clients retain their 1500-byte limit.
const MTU = 9000
const legacyMTU = 1500
const Magic = "URV1"

// ErrPeerDisconnected marks an explicit user stop, which must suppress auto-reconnect.
var ErrPeerDisconnected = errors.New("phone requested disconnect")

// Metrics contains aggregate counts only; no addresses or traffic are logged.
type Metrics struct {
	TCPQueueWaits    atomic.Uint64
	TransportFlushes atomic.Uint64
	UDPQueueDrops    atomic.Uint64
	QueuedPackets    atomic.Int64
	UnsupportedIPv6  atomic.Uint64
	InPackets        atomic.Uint64
	OutPackets       atomic.Uint64
	InBytes          atomic.Uint64
	OutBytes         atomic.Uint64
	QueueRejections  atomic.Uint64
	FlowRejections   atomic.Uint64
	DialFailures     atomic.Uint64
	ActiveFlows      atomic.Int64
}

type endpoint struct {
	ctx context.Context
	*channel.Endpoint
	metrics *Metrics
	queue   *fairQueue
}

// InvalidPacketError exposes header sizes without recording addresses or payload.
type InvalidPacketError struct {
	Version        int
	HeaderLength   int
	DeclaredLength int
	ReceivedLength int
}

func (e *InvalidPacketError) Error() string { return "invalid ipv4 packet" }

// ReadPacket rejects malformed framing and inconsistent IPv4 lengths.
func ReadPacket(reader io.Reader) ([]byte, error) { return readPacket(reader, MTU) }

func readPacket(reader io.Reader, limit int) ([]byte, error) {
	var prefix [2]byte
	if _, err := io.ReadFull(reader, prefix[:]); err != nil {
		return nil, err
	}
	size := int(binary.BigEndian.Uint16(prefix[:]))
	if size == 0 {
		var control [1]byte
		if _, err := io.ReadFull(reader, control[:]); err != nil {
			return nil, err
		}
		if control[0] == 1 {
			return nil, ErrPeerDisconnected
		}
		return nil, errors.New("unsupported control frame")
	}
	if size < 20 || size > limit {
		return nil, errors.New("invalid packet size")
	}
	packet := make([]byte, size)
	if _, err := io.ReadFull(reader, packet); err != nil {
		return nil, err
	}
	version := int(packet[0] >> 4)
	ihl := int(packet[0]&15) * 4
	declared := int(binary.BigEndian.Uint16(packet[2:4]))
	valid := version == 4 && ihl >= 20 && ihl <= size && declared == size
	if version == 6 && size >= 40 {
		ihl = 40
		declared = 40 + int(binary.BigEndian.Uint16(packet[4:6]))
		valid = declared == size
	}
	if !valid {
		return nil, &InvalidPacketError{Version: version, HeaderLength: ihl, DeclaredLength: declared, ReceivedLength: size}
	}

	return packet, nil
}

// PacketWriter reuses one frame buffer and performs one transport write per packet.
// A PacketWriter belongs to a single writer goroutine.
type PacketWriter struct {
	writer io.Writer
	frame  [MTU + 2]byte
}

// NewPacketWriter creates a bounded packet encoder.
func NewPacketWriter(writer io.Writer) *PacketWriter { return &PacketWriter{writer: writer} }

// Write encodes one packet and handles partial transport writes.
func (writer *PacketWriter) Write(packet []byte) error {
	if len(packet) < 20 || len(packet) > MTU {
		return errors.New("invalid packet size")
	}
	binary.BigEndian.PutUint16(writer.frame[:2], uint16(len(packet)))
	copy(writer.frame[2:], packet)
	data := writer.frame[:len(packet)+2]
	for len(data) > 0 {
		n, err := writer.writer.Write(data)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}

// WritePacket encodes one packet for callers without a persistent writer.
func WritePacket(writer io.Writer, packet []byte) error { return NewPacketWriter(writer).Write(packet) }

func destination(id stack.TransportEndpointID) string {
	address := id.LocalAddress.String()
	if address == "10.0.2.2" {
		address = "127.0.0.1"
	}
	return net.JoinHostPort(address, strconv.Itoa(int(id.LocalPort)))
}

// Serve handles one authenticated tunnel and releases all flows when it ends.
func Serve(parent context.Context, conn net.Conn, token []byte, metrics *Metrics) error {
	return serve(parent, conn, token, metrics, Policy{LegacyLab: true}, nil, Magic)
}

// ServeWithPolicy accepts authenticated v2/v3 framing; MTU negotiation stays inside TLS.
func ServeWithPolicy(parent context.Context, conn net.Conn, nonce []byte, metrics *Metrics, policy Policy, authenticated func() error) error {
	return serve(parent, conn, nonce, metrics, policy, authenticated, "URV2")
}
func serve(parent context.Context, conn net.Conn, token []byte, metrics *Metrics, policy Policy, authenticated func() error, magic string) error {
	if len(token) != 32 {
		return errors.New("token must contain 32 bytes")
	}
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	defer conn.Close()
	stopClose := context.AfterFunc(ctx, func() { conn.Close() })
	defer stopClose()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	var hello [36]byte
	if _, err := io.ReadFull(conn, hello[:]); err != nil {
		return fmt.Errorf("read handshake: %w", err)
	}
	negotiated := magic == "URV2" && string(hello[:4]) == "URV3"
	if (!negotiated && string(hello[:4]) != magic) || subtle.ConstantTimeCompare(hello[4:], token) != 1 {
		return errors.New("unauthorized tunnel")
	}
	mtu := legacyMTU
	if magic == Magic {
		mtu = MTU
	} // isolated legacy-lab protocol
	if negotiated {
		var requested [2]byte
		if _, err := io.ReadFull(conn, requested[:]); err != nil {
			return err
		}
		mtu = int(binary.BigEndian.Uint16(requested[:]))
		if mtu < legacyMTU || mtu > MTU {
			return errors.New("invalid negotiated MTU")
		}
		magic = "URV3"
	}
	if authenticated != nil {
		if err := authenticated(); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(conn, magic); err != nil {
		return err
	}
	if negotiated {
		var agreed [2]byte
		binary.BigEndian.PutUint16(agreed[:], uint16(mtu))
		if _, err := conn.Write(agreed[:]); err != nil {
			return err
		}
	}
	conn.SetDeadline(time.Time{})
	link := &endpoint{ctx: ctx, Endpoint: channel.New(1, uint32(mtu), ""), metrics: metrics, queue: newFairQueue(metrics)}
	defer link.queue.close()
	ipstack := stack.New(stack.Options{
		NetworkProtocols:   []stack.NetworkProtocolFactory{ipv4.NewProtocol},
		TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol, udp.NewProtocol},
	})
	defer ipstack.Close()
	defer link.Close()
	if err := ipstack.CreateNIC(1, link); err != nil {
		return errors.New(err.String())
	}
	if err := ipstack.SetPromiscuousMode(1, true); err != nil {
		return errors.New(err.String())
	}
	if err := ipstack.SetSpoofing(1, true); err != nil {
		return errors.New(err.String())
	}
	ipstack.SetRouteTable([]tcpip.Route{{Destination: header.IPv4EmptySubnet, NIC: 1}})
	slots := make(chan struct{}, 256)
	acquire := func() bool {
		select {
		case slots <- struct{}{}:
			metrics.ActiveFlows.Add(1)
			return true
		default:
			metrics.FlowRejections.Add(1)
			return false
		}
	}
	release := func() { <-slots; metrics.ActiveFlows.Add(-1) }
	dialer := net.Dialer{Timeout: 8 * time.Second}
	tcpForwarder := tcp.NewForwarder(ipstack, 1024*1024, 128, func(request *tcp.ForwarderRequest) {
		if !acquire() {
			request.Complete(true)
			return
		}
		defer release()
		id := request.ID()
		address, parseErr := netip.ParseAddr(id.LocalAddress.String())
		if parseErr != nil || !policy.allow(address, id.LocalPort) {
			metrics.FlowRejections.Add(1)
			request.Complete(true)
			return
		}
		upstream, err := dialer.DialContext(ctx, "tcp4", destination(id))
		if err != nil {
			metrics.DialFailures.Add(1)
			request.Complete(true)
			return
		}
		defer upstream.Close()
		var queue waiter.Queue
		ep, stackErr := request.CreateEndpoint(&queue)
		if stackErr != nil {
			request.Complete(true)
			return
		}
		// This TCP leg spans USB, not the upstream Internet RTT. Keep its
		// send window bounded so autotuning does not create a large backlog
		// and expensive outstanding-segment scans during sustained downloads.
		ep.SocketOptions().SetSendBufferSize(512*1024, true)
		request.Complete(false)
		downstream := gonet.NewTCPConn(&queue, ep)
		defer downstream.Close()
		bridge(ctx, downstream, upstream, false)
	})
	ipstack.SetTransportProtocolHandler(tcp.ProtocolNumber, tcpForwarder.HandlePacket)
	udpForwarder := udp.NewForwarder(ipstack, func(request *udp.ForwarderRequest) bool {
		id := request.ID()
		address, parseErr := netip.ParseAddr(id.LocalAddress.String())
		if parseErr != nil || !policy.allow(address, id.LocalPort) {
			metrics.FlowRejections.Add(1)
			return false
		}
		if !acquire() {
			return false
		}
		var queue waiter.Queue
		ep, stackErr := request.CreateEndpoint(&queue)
		if stackErr != nil {
			release()
			return true
		}
		downstream := gonet.NewUDPConn(&queue, ep)
		go func() {
			defer release()
			defer downstream.Close()
			upstream, err := dialer.DialContext(ctx, "udp4", destination(request.ID()))
			if err != nil {
				metrics.DialFailures.Add(1)
				return
			}
			defer upstream.Close()
			bridge(ctx, downstream, upstream, true)
		}()
		return true
	})
	ipstack.SetTransportProtocolHandler(udp.ProtocolNumber, udpForwarder.HandlePacket)
	writerDone := make(chan error, 1)
	go func() {
		buffered := bufio.NewWriterSize(conn, 32768)
		packetWriter := NewPacketWriter(buffered)
		for {
			packet := link.queue.read(ctx)
			if packet == nil {
				writerDone <- ctx.Err()
				return
			}
			for count := 0; ; count++ {
				view := packet.ToView()
				data := view.ToSlice()
				size := len(data)
				conn.SetWriteDeadline(time.Now().Add(15 * time.Second))
				err := packetWriter.Write(data)
				view.Release()
				packet.DecRef()
				if err != nil {
					writerDone <- err
					cancel()
					return
				}
				metrics.OutPackets.Add(1)
				metrics.OutBytes.Add(uint64(size))
				if count >= 20 || buffered.Available() < mtu+2 {
					break
				}
				packet = link.queue.tryRead()
				if packet == nil {
					break
				}
			}
			// Never wait for another packet while an incomplete TLS batch is buffered.
			metrics.TransportFlushes.Add(1)
			if err := buffered.Flush(); err != nil {
				writerDone <- err
				cancel()
				return
			}
		}
	}()
	defer func() { cancel(); <-writerDone }()
	reader := bufio.NewReaderSize(conn, 65536)
	for {
		data, err := readPacket(reader, mtu)
		if err != nil {
			return err
		}
		if data[0]>>4 == 6 {
			// Android emits IPv6 control frames even on an IPv4-only VPN.
			// Count and discard them without terminating the IPv4 session.
			metrics.UnsupportedIPv6.Add(1)
			continue
		}
		metrics.InPackets.Add(1)
		metrics.InBytes.Add(uint64(len(data)))
		packet := stack.NewPacketBuffer(stack.PacketBufferOptions{Payload: buffer.MakeWithData(data)})
		link.InjectInbound(ipv4.ProtocolNumber, packet)
		packet.DecRef()
	}
}

func bridge(parent context.Context, left, right net.Conn, datagram bool) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	stop := context.AfterFunc(ctx, func() { left.Close(); right.Close() })
	defer stop()
	origin := time.Now()
	var last atomic.Int64
	last.Store(0)
	done := make(chan error, 2)
	copyTo := func(dst, src net.Conn) {
		size := 32 * 1024
		if datagram {
			size = 65535
		}
		data := make([]byte, size)
		for {
			n, err := src.Read(data)
			if n > 0 || (n == 0 && err == nil && datagram) {
				last.Store(time.Since(origin).Nanoseconds())
				written, writeErr := dst.Write(data[:n])
				if writeErr != nil {
					done <- writeErr
					return
				}
				if written != n {
					done <- io.ErrShortWrite
					return
				}
			}
			if err != nil {
				if err == io.EOF && !datagram {
					if half, ok := dst.(interface{ CloseWrite() error }); ok {
						half.CloseWrite()
					}
				}
				done <- err
				return
			}
		}
	}
	go copyTo(left, right)
	go copyTo(right, left)
	timer := time.NewTicker(10 * time.Second)
	defer timer.Stop()
	for completed := 0; completed < 2; {
		select {
		case err := <-done:
			completed++
			if datagram || err != io.EOF {
				cancel()
			}
		case <-timer.C:
			idleLimit := 10 * time.Minute
			if datagram {
				idleLimit = 90 * time.Second
			}
			if time.Since(origin)-time.Duration(last.Load()) > idleLimit {
				cancel()
			}
		case <-ctx.Done():
			// The close callback wakes both blocked readers; drain their results.
			for ; completed < 2; completed++ {
				<-done
			}
		}
	}
}
