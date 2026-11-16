// Package securetunnel authenticates both USB tunnel endpoints with TLS 1.3.
package securetunnel

import (
	"context"
	"crypto/ecdsa"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"github.com/enesoglu/android-usb-reverse-tethering/internal/identity"
	"github.com/enesoglu/android-usb-reverse-tethering/internal/relay"
	"net"
	"time"
)

var ErrIdentityChanged = errors.New("phone identity does not match its saved pairing")

// Serve accepts a pinned client or enrolls one only through an explicitly pending pairing.
// Enrollment is committed after the one-time nonce is authenticated, not at certificate presentation.
func Serve(ctx context.Context, conn net.Conn, host identity.Identity, expected string, nonce []byte, metrics *relay.Metrics, policy relay.Policy, paired func(string) error) error {
	defer conn.Close()
	var peer string
	config := &tls.Config{Certificates: []tls.Certificate{host.Certificate}, MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, ClientAuth: tls.RequireAnyClientCert, NextProtos: []string{"usb-relay/2"}}
	config.VerifyPeerCertificate = func(raw [][]byte, _ [][]*x509.Certificate) error {
		if len(raw) != 1 {
			return errors.New("invalid client certificate chain")
		}
		cert, err := x509.ParseCertificate(raw[0])
		if err != nil {
			return errors.New("invalid phone certificate")
		}
		public, ok := cert.PublicKey.(*ecdsa.PublicKey)
		if !ok || public.Curve.Params().BitSize != 256 {
			return errors.New("phone must use a P-256 identity")
		}
		now := time.Now()
		if now.Before(cert.NotBefore) || now.After(cert.NotAfter) {
			return errors.New("phone certificate is not currently valid")
		}
		peer = identity.Fingerprint(cert)
		if expected != "" && subtle.ConstantTimeCompare([]byte(peer), []byte(expected)) != 1 {
			return ErrIdentityChanged
		}
		return nil
	}
	tunnel := tls.Server(conn, config)
	helloCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	if err := tunnel.HandshakeContext(helloCtx); err != nil {
		return err
	}
	if tunnel.ConnectionState().NegotiatedProtocol != "usb-relay/2" {
		return errors.New("unsupported tunnel protocol")
	}
	return relay.ServeWithPolicy(ctx, tunnel, nonce, metrics, policy, func() error { return paired(peer) })
}
