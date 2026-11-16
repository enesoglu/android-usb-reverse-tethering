package identity

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIdentityPersistsAndDoesNotResetAfterCorruption(t *testing.T) {
	dir := t.TempDir()
	first, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatal(err)
	}
	again, err := LoadOrCreate(dir)
	if err != nil || first.Pin != again.Pin {
		t.Fatal("identity did not persist")
	}
	path := filepath.Join(dir, "identity.dpapi")
	os.WriteFile(path, []byte("corrupted"), 0600)
	if _, err := LoadOrCreate(dir); err == nil {
		t.Fatal("identity silently reset")
	}
}

func TestMissingIdentityDoesNotReplaceExistingPairingIdentity(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "settings.dpapi"), []byte("existing protected settings"), 0600)
	if _, err := LoadOrCreate(dir); err == nil {
		t.Fatal("missing identity silently regenerated alongside existing pairings")
	}
}
