package securetunnel

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"github.com/enesoglu/android-usb-reverse-tethering/internal/identity"
	"github.com/enesoglu/android-usb-reverse-tethering/internal/relay"
	"io"
	"net"
	"testing"
	"time"
)

func TestMutualTLSAndNonce(t *testing.T) {
	for _, scenario := range []string{"valid", "changed_phone", "wrong_nonce", "tls12"} {
		t.Run(scenario, func(t *testing.T) {
			host, _ := identity.New()
			phone, _ := identity.New()
			other, _ := identity.New()
			expected := phone.Pin
			if scenario == "changed_phone" {
				expected = other.Pin
			}
			nonce := bytes.Repeat([]byte{42}, 32)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			listener, err := net.Listen("tcp4", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			ended := make(chan error, 1)
			paired := make(chan string, 1)
			metrics := &relay.Metrics{}
			go func() {
				conn, err := listener.Accept()
				if err != nil {
					ended <- err
					return
				}
				ended <- Serve(ctx, conn, host, expected, nonce, metrics, relay.Policy{}, func(pin string) error { paired <- pin; return nil })
			}()
			roots := x509.NewCertPool()
			roots.AddCert(host.Certificate.Leaf)
			version := uint16(tls.VersionTLS13)
			if scenario == "tls12" {
				version = tls.VersionTLS12
			}
			config := &tls.Config{RootCAs: roots, ServerName: "usb-relay.local", Certificates: []tls.Certificate{phone.Certificate}, MinVersion: version, MaxVersion: version, NextProtos: []string{"usb-relay/2"}}
			conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 2 * time.Second}, "tcp", listener.Addr().String(), config)
			accepted := false
			if err == nil {
				defer conn.Close()
				conn.SetDeadline(time.Now().Add(2 * time.Second))
				sent := append([]byte(nil), nonce...)
				if scenario == "wrong_nonce" {
					sent[0] ^= 1
				}
				conn.Write(append([]byte("URV2"), sent...))
				var reply [4]byte
				_, err = io.ReadFull(conn, reply[:])
				accepted = err == nil && string(reply[:]) == "URV2"
				if accepted {
					conn.Write([]byte{0, 0, 1})
				}
			}
			if scenario == "valid" {
				if !accepted {
					t.Fatalf("valid client rejected: %v", err)
				}
				select {
				case pin := <-paired:
					if pin != phone.Pin {
						t.Error("wrong identity paired")
					}
				case <-time.After(2 * time.Second):
					t.Fatal("no pairing callback")
				}
			} else {
				if accepted {
					t.Fatal("invalid client accepted")
				}
				select {
				case <-paired:
					t.Fatal("invalid client changed trust")
				default:
				}
			}
			select {
			case err := <-ended:
				if scenario == "changed_phone" && !errors.Is(err, ErrIdentityChanged) {
					t.Fatalf("wrong mismatch error: %v", err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("server did not stop")
			}
			if metrics.InPackets.Load() != 0 {
				t.Fatal("unexpected application traffic")
			}
		})
	}
}
