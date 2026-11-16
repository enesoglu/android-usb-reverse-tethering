package securetunnel

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"github.com/enesoglu/android-usb-reverse-tethering/internal/identity"
	"github.com/enesoglu/android-usb-reverse-tethering/internal/relay"
	"io"
	"net"
	"testing"
	"time"
)

func ipChecksum(header []byte) uint16 {
	sum := uint32(0)
	for i := 0; i < len(header); i += 2 {
		sum += uint32(binary.BigEndian.Uint16(header[i : i+2]))
	}
	for sum>>16 != 0 {
		sum = (sum & 65535) + (sum >> 16)
	}
	return ^uint16(sum)
}
func TestEncryptedDatagramTraversesRealStack(t *testing.T) {
	echo, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer echo.Close()
	echo.SetDeadline(time.Now().Add(5 * time.Second))
	go func() {
		data := make([]byte, 2048)
		n, address, err := echo.ReadFrom(data)
		if err == nil {
			echo.WriteTo(data[:n], address)
		}
	}()
	port := uint16(echo.LocalAddr().(*net.UDPAddr).Port)
	host, _ := identity.New()
	phone, _ := identity.New()
	nonce := bytes.Repeat([]byte{7}, 32)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ended := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			ended <- err
			return
		}
		ended <- Serve(ctx, conn, host, phone.Pin, nonce, &relay.Metrics{}, relay.Policy{DiagnosticPorts: map[uint16]bool{port: true}}, func(string) error { return nil })
	}()
	roots := x509.NewCertPool()
	roots.AddCert(host.Certificate.Leaf)
	client, err := tls.Dial("tcp", listener.Addr().String(), &tls.Config{RootCAs: roots, ServerName: "usb-relay.local", Certificates: []tls.Certificate{phone.Certificate}, MinVersion: tls.VersionTLS13, NextProtos: []string{"usb-relay/2"}})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	client.SetDeadline(time.Now().Add(5 * time.Second))
	client.Write(append([]byte("URV2"), nonce...))
	var reply [4]byte
	if _, err := io.ReadFull(client, reply[:]); err != nil || string(reply[:]) != "URV2" {
		t.Fatal("nonce handshake failed")
	}
	payload := []byte("synthetic encrypted UDP payload")
	packet := make([]byte, 28+len(payload))
	packet[0] = 0x45
	binary.BigEndian.PutUint16(packet[2:4], uint16(len(packet)))
	packet[8] = 64
	packet[9] = 17
	copy(packet[12:16], []byte{10, 100, 0, 2})
	copy(packet[16:20], []byte{10, 0, 2, 2})
	binary.BigEndian.PutUint16(packet[10:12], ipChecksum(packet[:20]))
	binary.BigEndian.PutUint16(packet[20:22], 40001)
	binary.BigEndian.PutUint16(packet[22:24], port)
	binary.BigEndian.PutUint16(packet[24:26], uint16(8+len(payload)))
	copy(packet[28:], payload)
	if err := relay.WritePacket(client, packet); err != nil {
		t.Fatal(err)
	}
	received, err := relay.ReadPacket(client)
	if err != nil {
		t.Fatal(err)
	}
	offset := int(received[0]&15)*4 + 8
	if offset > len(received) || !bytes.Equal(received[offset:], payload) {
		t.Fatal("encrypted datagram payload changed")
	}
	client.Write([]byte{0, 0, 1})
	select {
	case <-ended:
	case <-time.After(3 * time.Second):
		t.Fatal("encrypted tunnel did not stop")
	}
}
