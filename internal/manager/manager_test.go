package manager

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/enesoglu/android-usb-reverse-tethering/internal/adb"
	"github.com/enesoglu/android-usb-reverse-tethering/internal/identity"
	"io"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"
)

type fakeADB struct {
	mu             sync.Mutex
	devices        []adb.Device
	phone          identity.Identity
	calls, cleaned int
	client         *tls.Conn
}

func (f *fakeADB) List(context.Context) ([]adb.Device, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]adb.Device{}, f.devices...), nil
}
func (f *fakeADB) Stop(string) {}
func (f *fakeADB) Prepare(ctx context.Context, d adb.Device, p adb.Provision, automatic bool) (func(), error) {
	f.mu.Lock()
	f.calls++
	phone := f.phone
	f.mu.Unlock()
	go func() {
		config := &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, Certificates: []tls.Certificate{phone.Certificate}, NextProtos: []string{"usb-relay/2"}, InsecureSkipVerify: true}
		config.VerifyPeerCertificate = func(raw [][]byte, _ [][]*x509.Certificate) error {
			cert, err := x509.ParseCertificate(raw[0])
			if err != nil || identity.Fingerprint(cert) != p.Pin {
				return errors.New("wrong host pin")
			}
			return nil
		}
		conn, err := tls.DialWithDialer(&net.Dialer{Timeout: time.Second}, "tcp", "127.0.0.1:"+strconv.Itoa(p.Port), config)
		if err != nil {
			return
		}
		defer conn.Close()
		nonce, _ := hex.DecodeString(p.Nonce)
		conn.SetDeadline(time.Now().Add(2 * time.Second))
		if _, err := conn.Write(append([]byte("URV2"), nonce...)); err != nil {
			return
		}
		var accepted [4]byte
		if _, err := io.ReadFull(conn, accepted[:]); err != nil {
			return
		}
		conn.SetDeadline(time.Time{})
		f.mu.Lock()
		f.client = conn
		f.mu.Unlock()
		<-ctx.Done()
	}()
	return func() { f.mu.Lock(); f.cleaned++; f.mu.Unlock() }, nil
}
func (f *fakeADB) set(devices ...adb.Device) { f.mu.Lock(); f.devices = devices; f.mu.Unlock() }
func (f *fakeADB) count() int                { f.mu.Lock(); defer f.mu.Unlock(); return f.calls }
func (f *fakeADB) userStop() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.client != nil {
		f.client.Write([]byte{0, 0, 1})
	}
}
func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	end := time.Now().Add(4 * time.Second)
	for time.Now().Before(end) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition did not become true")
}
func fixture(t *testing.T, known bool) (*Manager, *fakeADB, context.Context) {
	t.Helper()
	host, _ := identity.New()
	phone, _ := identity.New()
	backend := &fakeADB{phone: phone, devices: []adb.Device{{Serial: "synthetic-phone", Model: "Synthetic Android", State: "device", Transport: "1"}}}
	config := DefaultConfig()
	if known {
		config.Trusted = []Trusted{{Serial: "synthetic-phone", Pin: phone.Pin, Model: "Synthetic Android"}}
	}
	m := New(backend, host, config, func(Config) error { return nil })
	ctx, cancel := context.WithCancel(context.Background())
	m.ctx = ctx
	t.Cleanup(func() { cancel(); m.shutdown.Wait() })
	return m, backend, ctx
}
func TestUnknownPhoneRequiresManualPairing(t *testing.T) {
	m, f, ctx := fixture(t, false)
	m.poll(ctx)
	if f.count() != 0 {
		t.Fatal("unknown phone auto-connected")
	}
	if err := m.Connect("synthetic-phone", true); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return m.Snapshot().State == "connected" })
	snapshot := m.Snapshot()
	if len(snapshot.Trusted) != 1 || snapshot.Trusted[0].Pin != f.phone.Pin {
		t.Fatal("authenticated phone was not remembered")
	}
}
func TestManualStopStaysStoppedUntilReattach(t *testing.T) {
	m, f, ctx := fixture(t, true)
	m.poll(ctx)
	waitFor(t, func() bool { return m.Snapshot().State == "connected" })
	m.Disconnect()
	waitFor(t, func() bool { return m.Snapshot().Serial == "" })
	m.poll(ctx)
	if f.count() != 1 {
		t.Fatal("manual disconnect immediately reconnected")
	}
	f.set()
	m.poll(ctx)
	f.set(adb.Device{Serial: "synthetic-phone", Model: "Synthetic", State: "device", Transport: "2"})
	m.poll(ctx)
	waitFor(t, func() bool { return f.count() == 2 && m.Snapshot().State == "connected" })
}
func TestPhoneDisconnectSuppressesReconnect(t *testing.T) {
	m, f, ctx := fixture(t, true)
	m.poll(ctx)
	waitFor(t, func() bool { return m.Snapshot().State == "connected" })
	f.userStop()
	waitFor(t, func() bool { return m.Snapshot().Serial == "" })
	m.poll(ctx)
	if f.count() != 1 {
		t.Fatal("phone stop was ignored")
	}
}
func TestChangedIdentityFailsClosed(t *testing.T) {
	m, f, ctx := fixture(t, true)
	other, _ := identity.New()
	f.phone = other
	m.poll(ctx)
	waitFor(t, func() bool { return m.Snapshot().State == "security_error" })
	m.poll(ctx)
	if f.count() != 1 {
		t.Fatal("changed identity was retried automatically")
	}
	if m.Snapshot().Trusted[0].Pin == other.Pin {
		t.Fatal("changed identity silently trusted")
	}
}
func TestFailedSettingsSavePreservesConfiguration(t *testing.T) {
	m, _, _ := fixture(t, false)
	before := m.Snapshot().Settings
	m.save = func(Config) error { return errors.New("synthetic write failure") }
	next := before
	next.DNS = "9.9.9.9"
	next.StartMinimized = true
	if m.SetSettings(next) == nil {
		t.Fatal("save failure ignored")
	}
	if m.Snapshot().Settings != before {
		t.Fatal("failed write changed settings")
	}
}
func TestForgetRevokesActivePhone(t *testing.T) {
	m, f, ctx := fixture(t, true)
	m.poll(ctx)
	waitFor(t, func() bool { return m.Snapshot().State == "connected" })
	if err := m.Forget("synthetic-phone"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return m.Snapshot().Serial == "" })
	m.poll(ctx)
	if f.count() != 1 || len(m.Snapshot().Trusted) != 0 {
		t.Fatal("forgotten phone reconnected")
	}
}
func TestUnapprovedDebuggingNeverAutoconnects(t *testing.T) {
	m, f, ctx := fixture(t, true)
	f.set(adb.Device{Serial: "synthetic-phone", State: "unauthorized"})
	m.poll(ctx)
	if f.count() != 0 {
		t.Fatal("unauthorized phone connected")
	}
}

func TestAutoConnectCanBeDisabled(t *testing.T) {
	m, f, ctx := fixture(t, true)
	m.config.Settings.AutoConnect = false
	m.poll(ctx)
	if f.count() != 0 {
		t.Fatal("auto-connect disabled setting ignored")
	}
}
func TestPairingIsNotSavedBeforeSuccessfulStoreWrite(t *testing.T) {
	m, f, ctx := fixture(t, false)
	m.poll(ctx)
	m.save = func(Config) error { return errors.New("synthetic storage failure") }
	if err := m.Connect("synthetic-phone", true); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return m.Snapshot().State == "error" })
	if len(m.Snapshot().Trusted) != 0 {
		t.Fatal("failed pairing was trusted")
	}
	if f.count() != 1 {
		t.Fatal("unexpected retries")
	}
}
func TestClosedManagerRejectsNewConnection(t *testing.T) {
	m, _, _ := fixture(t, true)
	m.closed = true
	if m.Connect("synthetic-phone", true) == nil {
		t.Fatal("shutdown manager accepted new work")
	}
}

func TestStartMinimizedPersistsAcrossManagerRestart(t *testing.T) {
	m, _, _ := fixture(t, false)
	var saved []byte
	m.save = func(config Config) (err error) {
		saved, err = json.Marshal(config)
		return err
	}
	for _, enabled := range []bool{true, false} {
		next := m.Snapshot().Settings
		next.StartMinimized = enabled
		if err := m.SetSettings(next); err != nil {
			t.Fatal(err)
		}
		restored := DefaultConfig()
		if err := json.Unmarshal(saved, &restored); err != nil {
			t.Fatal(err)
		}
		restarted := New(m.backend, m.host, restored, m.save)
		if restarted.Snapshot().Settings.StartMinimized != enabled {
			t.Fatal("startup preference lost after restart")
		}
	}
	legacy := DefaultConfig()
	if err := json.Unmarshal([]byte(`{"settings":{"startWithWindows":true}}`), &legacy); err != nil {
		t.Fatal(err)
	}
	if legacy.Settings.StartMinimized {
		t.Fatal("legacy config should show the window by default")
	}
}
