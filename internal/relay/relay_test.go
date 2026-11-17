package relay

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
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
)

func testClientVersion(t *testing.T, negotiate bool) (*stack.Stack, *Metrics) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	host, phone := net.Pipe()
	token := bytes.Repeat([]byte{42}, 32)
	metrics := &Metrics{}
	ended := make(chan error, 1)
	go func() {
		if negotiate {
			ended <- ServeWithPolicy(ctx, host, token, metrics, Policy{LegacyLab: true}, func() error { return nil })
		} else {
			ended <- Serve(ctx, host, token, metrics)
		}
	}()
	phone.SetDeadline(time.Now().Add(5 * time.Second))
	magic := Magic
	hello := append([]byte(magic), token...)
	if negotiate {
		magic = "URV3"
		hello = append([]byte(magic), token...)
		hello = binary.BigEndian.AppendUint16(hello, uint16(MTU))
	}
	if _, err := phone.Write(hello); err != nil {
		t.Fatal(err)
	}
	response := make([]byte, 4)
	if negotiate {
		response = make([]byte, 6)
	}
	if _, err := io.ReadFull(phone, response); err != nil || string(response[:4]) != magic {
		t.Fatalf("handshake: %v", err)
	}
	if negotiate && int(binary.BigEndian.Uint16(response[4:])) != MTU {
		t.Fatal("negotiated packet limit differs")
	}
	phone.SetDeadline(time.Time{})
	// Regression: an Android IPv6 control packet must not close the IPv4 tunnel.
	control := make([]byte, 76)
	control[0] = 0x60
	binary.BigEndian.PutUint16(control[4:6], 36)
	if err := WritePacket(phone, control); err != nil {
		t.Fatal(err)
	}

	link := channel.New(1024, MTU, "")
	s := stack.New(stack.Options{NetworkProtocols: []stack.NetworkProtocolFactory{ipv4.NewProtocol}, TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol, udp.NewProtocol}})
	if err := s.CreateNIC(1, link); err != nil {
		t.Fatal(err)
	}
	address := tcpip.ProtocolAddress{Protocol: ipv4.ProtocolNumber, AddressWithPrefix: tcpip.AddressWithPrefix{Address: tcpip.AddrFrom4([4]byte{10, 100, 0, 2}), PrefixLen: 32}}
	if err := s.AddProtocolAddress(1, address, stack.AddressProperties{}); err != nil {
		t.Fatal(err)
	}
	s.SetRouteTable([]tcpip.Route{{Destination: header.IPv4EmptySubnet, NIC: 1}})
	go func() {
		for {
			pkt := link.ReadContext(ctx)
			if pkt == nil {
				return
			}
			view := pkt.ToView()
			err := WritePacket(phone, view.ToSlice())
			view.Release()
			pkt.DecRef()
			if err != nil {
				return
			}
		}
	}()
	go func() {
		for {
			data, err := ReadPacket(phone)
			if err != nil {
				return
			}
			pkt := stack.NewPacketBuffer(stack.PacketBufferOptions{Payload: buffer.MakeWithData(data)})
			link.InjectInbound(ipv4.ProtocolNumber, pkt)
			pkt.DecRef()
		}
	}()
	t.Cleanup(func() {
		cancel()
		phone.Close()
		host.Close()
		s.Close()
		link.Close()
		select {
		case <-ended:
		case <-time.After(3 * time.Second):
			t.Error("relay did not stop")
		}
		deadline := time.Now().Add(3 * time.Second)
		for metrics.ActiveFlows.Load() != 0 && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		if metrics.ActiveFlows.Load() != 0 {
			t.Error("relay leaked active flows")
		}
	})
	return s, metrics
}

func TestTCPInternetRelay(t *testing.T) {
	for _, negotiated := range []bool{false, true} {
		t.Run(strconv.FormatBool(negotiated), func(t *testing.T) { testTCPInternetRelay(t, negotiated) })
	}
}
func testTCPInternetRelay(t *testing.T, negotiated bool) {
	expected := "synthetic response through raw IP tunnel"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, expected) }))
	defer server.Close()
	_, portText, _ := net.SplitHostPort(server.Listener.Addr().String())
	port, _ := strconv.Atoi(portText)
	s, metrics := testClientVersion(t, negotiated)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := gonet.DialContextTCP(ctx, s, tcpip.FullAddress{NIC: 1, Addr: tcpip.AddrFrom4([4]byte{10, 0, 2, 2}), Port: uint16(port)}, ipv4.ProtocolNumber)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	io.WriteString(conn, "GET / HTTP/1.1\r\nHost: synthetic.test\r\nConnection: close\r\n\r\n")
	response, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil || string(body) != expected || response.StatusCode != 200 {
		t.Fatalf("unexpected response: %q %v", body, err)
	}
	if metrics.UnsupportedIPv6.Load() != 1 {
		t.Error("IPv6 control frame was not counted")
	}
	if metrics.InPackets.Load() == 0 || metrics.OutPackets.Load() == 0 {
		t.Error("missing packet counters")
	}
}

func TestUDPEchoThroughRelay(t *testing.T) {
	for _, negotiated := range []bool{false, true} {
		t.Run(strconv.FormatBool(negotiated), func(t *testing.T) { testUDPEchoThroughRelay(t, negotiated) })
	}
}
func testUDPEchoThroughRelay(t *testing.T, negotiated bool) {
	server, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	go func() {
		data := make([]byte, 65535)
		for {
			n, addr, err := server.ReadFrom(data)
			if err != nil {
				return
			}
			server.WriteTo(data[:n], addr)
		}
	}()
	port := server.LocalAddr().(*net.UDPAddr).Port
	s, _ := testClientVersion(t, negotiated)
	conn, err := gonet.DialUDP(s, nil, &tcpip.FullAddress{NIC: 1, Addr: tcpip.AddrFrom4([4]byte{10, 0, 2, 2}), Port: uint16(port)}, ipv4.ProtocolNumber)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	for _, data := range [][]byte{[]byte("synthetic UDP"), bytes.Repeat([]byte{7}, 1200), bytes.Repeat([]byte{8}, 8000), bytes.Repeat([]byte{9}, 16000), bytes.Repeat([]byte{10}, 65507)} {
		if _, err := conn.Write(data); err != nil {
			t.Fatal(err)
		}
		got := make([]byte, 65535)
		n, err := conn.Read(got)
		if err != nil || !bytes.Equal(got[:n], data) {
			t.Fatalf("datagram changed: n=%d err=%v", n, err)
		}
	}
}

func TestWrongTokenIsRejectedBeforeForwarding(t *testing.T) {
	host, phone := net.Pipe()
	defer phone.Close()
	metrics := &Metrics{}
	ended := make(chan error, 1)
	go func() { ended <- Serve(context.Background(), host, bytes.Repeat([]byte{1}, 32), metrics) }()
	phone.SetDeadline(time.Now().Add(time.Second))
	phone.Write(append([]byte(Magic), bytes.Repeat([]byte{2}, 32)...))
	data := make([]byte, 4)
	if _, err := phone.Read(data); err == nil {
		t.Error("unauthorized client received acceptance")
	}
	if err := <-ended; err == nil {
		t.Error("wrong token accepted")
	}
	if metrics.InPackets.Load() != 0 || metrics.ActiveFlows.Load() != 0 {
		t.Error("unauthorized traffic reached stack")
	}
}

func TestMalformedPacketLengthsAreRejected(t *testing.T) {
	for _, size := range []int{0, 19, MTU + 1} {
		var header [2]byte
		binary.BigEndian.PutUint16(header[:], uint16(size))
		if _, err := ReadPacket(bytes.NewReader(append(header[:], byte(0)))); err == nil {
			t.Errorf("accepted invalid length %d", size)
		}
	}
	packet := make([]byte, 20)
	packet[0] = 0x45
	binary.BigEndian.PutUint16(packet[2:4], 21)
	var wire bytes.Buffer
	WritePacket(&wire, packet)
	if _, err := ReadPacket(&wire); err == nil {
		t.Error("accepted inconsistent IPv4 length")
	}
}

type shortWriter struct{ bytes.Buffer }

func (w *shortWriter) Write(data []byte) (int, error) {
	if len(data) > 1 {
		data = data[:1]
	}
	return w.Buffer.Write(data)
}
func TestPacketFramingHandlesShortWrites(t *testing.T) {
	packet := make([]byte, 20)
	packet[0] = 0x45
	binary.BigEndian.PutUint16(packet[2:4], 20)
	writer := &shortWriter{}
	if err := WritePacket(writer, packet); err != nil {
		t.Fatal(err)
	}
	result, err := ReadPacket(bytes.NewReader(writer.Bytes()))
	if err != nil || !bytes.Equal(packet, result) {
		t.Fatalf("framing changed: %v", err)
	}
}

// Exercise the authenticated wire negotiation, including the old v2 client.
func TestNegotiatedPacketLimit(t *testing.T) {
	for _, tc := range []struct {
		name, magic   string
		request, want int
	}{
		{"legacy-v2", "URV2", 0, 1500},
		{"v3-small", "URV3", 1500, 1500},
		{"v3-large", "URV3", 9000, 9000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			host, phone := net.Pipe()
			defer phone.Close()
			phone.SetDeadline(time.Now().Add(3 * time.Second))
			nonce := bytes.Repeat([]byte{9}, 32)
			metrics := &Metrics{}
			ended := make(chan error, 1)
			go func() {
				ended <- ServeWithPolicy(context.Background(), host, nonce, metrics, Policy{LegacyLab: true}, func() error { return nil })
			}()
			hello := append([]byte(tc.magic), nonce...)
			if tc.request != 0 {
				hello = binary.BigEndian.AppendUint16(hello, uint16(tc.request))
			}
			if _, err := phone.Write(hello); err != nil {
				t.Fatal(err)
			}
			reply := make([]byte, 4)
			if tc.request != 0 {
				reply = make([]byte, 6)
			}
			if _, err := io.ReadFull(phone, reply); err != nil {
				t.Fatal(err)
			}
			if string(reply[:4]) != tc.magic {
				t.Fatalf("wrong reply %q", reply[:4])
			}
			if tc.request != 0 && int(binary.BigEndian.Uint16(reply[4:])) != tc.want {
				t.Fatal("MTU agreement differs")
			}
			// A valid large IPv6 frame is counted/discarded; it must not kill IPv4 service.
			packet := make([]byte, tc.want)
			packet[0] = 0x60
			binary.BigEndian.PutUint16(packet[4:6], uint16(len(packet)-40))
			var wire bytes.Buffer
			binary.Write(&wire, binary.BigEndian, uint16(len(packet)))
			wire.Write(packet)
			if _, err := phone.Write(wire.Bytes()); err != nil {
				t.Fatal(err)
			}
			// The session limit must be enforced before reading an oversized payload.
			var oversized [2]byte
			binary.BigEndian.PutUint16(oversized[:], uint16(tc.want+1))
			phone.Write(oversized[:])
			if err := <-ended; err == nil {
				t.Fatal("oversized frame accepted")
			}
			if metrics.UnsupportedIPv6.Load() != 1 {
				t.Fatal("valid frame did not traverse negotiated decoder")
			}
		})
	}
}

func TestInvalidNegotiatedMTU(t *testing.T) {
	for _, mtu := range []uint16{0, 1499, 9001, 65535} {
		host, phone := net.Pipe()
		phone.SetDeadline(time.Now().Add(time.Second))
		nonce := bytes.Repeat([]byte{9}, 32)
		done := make(chan error, 1)
		go func() {
			done <- ServeWithPolicy(context.Background(), host, nonce, &Metrics{}, Policy{}, func() error { return nil })
		}()
		hello := append([]byte("URV3"), nonce...)
		hello = binary.BigEndian.AppendUint16(hello, mtu)
		phone.Write(hello)
		if err := <-done; err == nil {
			t.Errorf("accepted MTU %d", mtu)
		}
		phone.Close()
	}
}
