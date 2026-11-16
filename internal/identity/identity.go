// Package identity owns the desktop TLS identity and pin format.
package identity

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"github.com/enesoglu/android-usb-reverse-tethering/internal/private"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

type record struct {
	Certificate []byte `json:"certificate"`
	Key         []byte `json:"key"`
}

// Identity contains the in-memory TLS certificate and public-key fingerprint.
type Identity struct {
	Certificate tls.Certificate
	Pin         string
}

// Fingerprint returns the SHA-256 public-key pin, not the certificate's expiry-sensitive hash.
func Fingerprint(cert *x509.Certificate) string {
	sum := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
	return hex.EncodeToString(sum[:])
}

// New generates a P-256 identity. Private key material is never logged.
func New() (Identity, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return Identity{}, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return Identity{}, err
	}
	now := time.Now()
	template := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "USB Relay"}, DNSNames: []string{"usb-relay.local"}, NotBefore: now.Add(-time.Hour), NotAfter: now.AddDate(10, 0, 0), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return Identity{}, err
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		return Identity{}, err
	}
	return Identity{Certificate: tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: parsed}, Pin: Fingerprint(parsed)}, nil
}

// LoadOrCreate persists a new identity only when no record exists; damaged records never reset trust silently.
func LoadOrCreate(dir string) (Identity, error) {
	path := filepath.Join(dir, "identity.dpapi")
	var saved record
	err := private.Load(path, &saved)
	if errors.Is(err, os.ErrNotExist) {
		if _, settingsErr := os.Stat(filepath.Join(dir, "settings.dpapi")); settingsErr == nil {
			return Identity{}, errors.New("desktop identity is missing while saved settings remain; explicit recovery is required")
		}

		value, err := New()
		if err != nil {
			return Identity{}, err
		}
		key, err := x509.MarshalPKCS8PrivateKey(value.Certificate.PrivateKey)
		if err != nil {
			return Identity{}, err
		}
		saved = record{Certificate: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: value.Certificate.Certificate[0]}), Key: pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key})}
		if err = private.Save(path, saved); err != nil {
			return Identity{}, err
		}
		return value, nil
	}
	if err != nil {
		return Identity{}, err
	}
	cert, err := tls.X509KeyPair(saved.Certificate, saved.Key)
	if err != nil {
		return Identity{}, errors.New("invalid desktop identity")
	}
	cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return Identity{}, err
	}
	if time.Now().Before(cert.Leaf.NotBefore) || time.Now().After(cert.Leaf.NotAfter) {
		return Identity{}, errors.New("desktop identity has expired; explicit identity reset required")
	}
	return Identity{Certificate: cert, Pin: Fingerprint(cert.Leaf)}, nil
}
