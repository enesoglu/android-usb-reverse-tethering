package private

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestProtectedRoundTripAndTamper(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret.dpapi")
	original := map[string]string{"secret": "synthetic-private-value"}
	if err := Save(path, original); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte(original["secret"])) {
		t.Fatal("plaintext secret was persisted")
	}
	var result map[string]string
	if err := Load(path, &result); err != nil || result["secret"] != original["secret"] {
		t.Fatalf("roundtrip failed: %v", err)
	}
	raw[len(raw)/2] ^= 0xFF
	os.WriteFile(path, raw, 0600)
	if Load(path, &result) == nil {
		t.Fatal("tampered store was accepted")
	}
}
