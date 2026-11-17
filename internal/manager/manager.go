// Package manager owns pairing, reconnect policy and a single active USB session.
package manager

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"github.com/enesoglu/android-usb-reverse-tethering/internal/adb"
	"github.com/enesoglu/android-usb-reverse-tethering/internal/identity"
	"github.com/enesoglu/android-usb-reverse-tethering/internal/relay"
	"github.com/enesoglu/android-usb-reverse-tethering/internal/securetunnel"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Backend is the tested seam between device policy and ADB side effects.
type Backend interface {
	List(context.Context) ([]adb.Device, error)
	Prepare(context.Context, adb.Device, adb.Provision, bool) (func(), error)
	Stop(string)
}
type Settings struct {
	ADBPath          string `json:"adbPath"`
	AutoConnect      bool   `json:"autoConnect"`
	StartWithWindows bool   `json:"startWithWindows"`
	StartMinimized   bool   `json:"startMinimized"`
	MinimizeToTray   bool   `json:"minimizeToTray"`
	CloseToTray      bool   `json:"closeToTray"`
	AllowLAN         bool   `json:"allowLAN"`
	DNS              string `json:"dns"`
}
type Trusted struct {
	Serial   string    `json:"serial"`
	Model    string    `json:"model"`
	Pin      string    `json:"pin"`
	PairedAt time.Time `json:"pairedAt"`
}
type Config struct {
	Settings Settings  `json:"settings"`
	Trusted  []Trusted `json:"trusted"`
}

func DefaultConfig() Config {
	return Config{Settings: Settings{AutoConnect: true, MinimizeToTray: true, CloseToTray: true, DNS: "1.1.1.1"}}
}

type Device struct {
	adb.Device
	Trusted bool   `json:"trusted"`
	Pin     string `json:"pin,omitempty"`
}
type Snapshot struct {
	TCPQueueWaits    uint64    `json:"tcpQueueWaits"`
	TransportPackets uint64    `json:"transportPackets"`
	TransportFlushes uint64    `json:"transportFlushes"`
	State            string    `json:"state"`
	Message          string    `json:"message"`
	Devices          []Device  `json:"devices"`
	Trusted          []Trusted `json:"trusted"`
	Settings         Settings  `json:"settings"`
	Serial           string    `json:"serial"`
	HostPin          string    `json:"hostPin"`
	DownloadMbps     float64   `json:"downloadMbps"`
	UploadMbps       float64   `json:"uploadMbps"`
	DownloadBytes    uint64    `json:"downloadBytes"`
	UploadBytes      uint64    `json:"uploadBytes"`
	QueueRejections  uint64    `json:"queueRejections"`
	ActiveFlows      int64     `json:"activeFlows"`
	UptimeSeconds    int64     `json:"uptimeSeconds"`
}
type session struct {
	device           adb.Device
	cancel           context.CancelFunc
	metrics          relay.Metrics
	started          time.Time
	connected        bool
	lastIn, lastOut  uint64
	lastSample       time.Time
	downRate, upRate float64
}
type Manager struct {
	mu              sync.Mutex
	backend         Backend
	host            identity.Identity
	config          Config
	save            func(Config) error
	ctx             context.Context
	devices         []adb.Device
	active          *session
	state, message  string
	suppressed      map[string]bool
	attempts        map[string]int
	retry           map[string]time.Time
	shutdown        sync.WaitGroup
	bootID          string
	closed          bool
	diagnosticPorts map[uint16]bool
}

func New(backend Backend, host identity.Identity, config Config, save func(Config) error) *Manager {
	boot := make([]byte, 16)
	if _, err := rand.Read(boot); err != nil {
		panic("secure random source unavailable")
	}
	return &Manager{bootID: hex.EncodeToString(boot), backend: backend, host: host, config: config, save: save, state: "disconnected", message: "Connect a phone over USB to begin", suppressed: map[string]bool{}, attempts: map[string]int{}, retry: map[string]time.Time{}}
}
func (m *Manager) Run(ctx context.Context) {
	m.mu.Lock()
	m.ctx = ctx
	m.mu.Unlock()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		m.poll(ctx)
		select {
		case <-ctx.Done():
			m.mu.Lock()
			m.closed = true
			if m.active != nil {
				m.active.cancel()
			}
			m.mu.Unlock()
			m.shutdown.Wait()
			return
		case <-ticker.C:
		}
	}
}
func (m *Manager) poll(ctx context.Context) {
	devices, err := m.backend.List(ctx)
	m.mu.Lock()
	defer m.mu.Unlock()
	if err != nil {
		if m.active == nil {
			m.state = "adb_unavailable"
			m.message = err.Error()
		}
		return
	}
	previous := map[string]string{}
	for _, d := range m.devices {
		previous[d.Serial] = d.Transport
	}
	for _, d := range devices {
		if prior, ok := previous[d.Serial]; ok && prior != d.Transport {
			delete(m.suppressed, d.Serial)
			delete(m.retry, d.Serial)
			if m.active != nil && m.active.device.Serial == d.Serial {
				m.active.cancel()
			}
		}
	}
	m.devices = devices
	present := map[string]bool{}
	ready := map[string]bool{}
	for _, d := range devices {
		present[d.Serial] = true
		ready[d.Serial] = d.State == "device"
	}
	for id := range m.suppressed {
		if !present[id] {
			delete(m.suppressed, id)
			delete(m.retry, id)
			delete(m.attempts, id)
		}
	}
	if m.active != nil {
		if !ready[m.active.device.Serial] {
			m.active.cancel()
			m.state = "reconnecting"
			m.message = "USB disconnected; waiting for the trusted phone"
		}
		return
	}
	if m.config.Settings.AutoConnect {
		for _, d := range devices {
			if d.State == "device" && !m.suppressed[d.Serial] && m.trust(d.Serial).Pin != "" && !time.Now().Before(m.retry[d.Serial]) {
				m.start(d, true, true)
				return
			}
		}
	}
	if m.state == "adb_unavailable" || m.state == "reconnecting" {
		m.state = "disconnected"
		m.message = "Waiting for a trusted USB phone"
	}
}
func (m *Manager) trust(serial string) Trusted {
	for _, t := range m.config.Trusted {
		if t.Serial == serial {
			return t
		}
	}
	return Trusted{}
}
func (m *Manager) Connect(serial string, remember bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ctx == nil || m.closed || m.ctx.Err() != nil {
		return errors.New("device manager is starting")
	}
	if m.active != nil {
		return errors.New("disconnect the active phone first")
	}
	for _, d := range m.devices {
		if d.Serial == serial {
			if d.State != "device" {
				return errors.New("approve USB debugging on the phone first")
			}
			delete(m.suppressed, serial)
			m.start(d, remember, false)
			return nil
		}
	}
	return errors.New("phone is no longer connected")
}
func (m *Manager) start(device adb.Device, remember, automatic bool) {
	ctx, cancel := context.WithCancel(m.ctx)
	s := &session{device: device, cancel: cancel, lastSample: time.Now()}
	m.active = s
	m.state = "connecting"
	m.message = "Preparing the encrypted USB connection"
	expected := m.trust(device.Serial).Pin
	settings := m.config.Settings
	m.shutdown.Add(1)
	go func() {
		defer m.shutdown.Done()
		err := m.runSession(ctx, s, expected, remember, automatic, settings)
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.active != s {
			return
		}
		m.active = nil
		if errors.Is(err, relay.ErrPeerDisconnected) {
			m.suppressed[device.Serial] = true
			m.state = "disconnected"
			m.message = "Disconnected from the phone"
			return
		}
		if errors.Is(err, securetunnel.ErrIdentityChanged) {
			m.suppressed[device.Serial] = true
			m.state = "security_error"
			m.message = "Phone identity changed. Verify the phone, then forget and pair it again."
			return
		}
		if m.suppressed[device.Serial] || m.ctx.Err() != nil {
			m.state = "disconnected"
			m.message = "Disconnected"
			return
		}
		if err != nil {
			m.attempts[device.Serial]++
			seconds := 1 << min(m.attempts[device.Serial]-1, 4)
			m.retry[device.Serial] = time.Now().Add(time.Duration(seconds) * time.Second)
			m.state = "reconnecting"
			m.message = "Connection interrupted. Retrying the trusted phone."
			if expected == "" {
				m.state = "error"
				m.message = err.Error()
			}
			if automatic && err.Error() == "phone app was force-stopped; open it or connect manually" {
				m.suppressed[device.Serial] = true
				m.state = "approval_required"
				m.message = err.Error()
			}
		}
	}()
}
func (m *Manager) runSession(ctx context.Context, s *session, expected string, remember, automatic bool, settings Settings) error {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return errors.New("could not open a local USB endpoint")
	}
	defer listener.Close()
	stop := context.AfterFunc(ctx, func() { listener.Close() })
	defer stop()
	nonce := make([]byte, 32)
	if _, err = rand.Read(nonce); err != nil {
		return errors.New("could not generate a secure session")
	}
	provision := adb.Provision{MTU: relay.MTU, Port: listener.Addr().(*net.TCPAddr).Port, Pin: m.host.Pin, Nonce: hex.EncodeToString(nonce), DNS: settings.DNS, Background: automatic, AttachID: m.bootID + ":" + s.device.Transport}
	cleanup, err := m.backend.Prepare(ctx, s.device, provision, automatic)
	if err != nil {
		return err
	}
	defer cleanup()
	defer m.backend.Stop(s.device.Serial)
	m.mu.Lock()
	if m.active == s {
		m.state = "waiting_for_phone"
		m.message = "Approve the VPN on your phone if prompted"
	}
	m.mu.Unlock()
	hosts := map[netip.Addr]bool{}
	if addresses, e := net.InterfaceAddrs(); e == nil {
		for _, raw := range addresses {
			if prefix, e := netip.ParsePrefix(raw.String()); e == nil {
				hosts[prefix.Addr().Unmap()] = true
			}
		}
	}
	policy := relay.Policy{AllowLAN: settings.AllowLAN, HostAddresses: hosts, DiagnosticPorts: m.diagnosticPorts}
	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}
		authenticated := false
		attemptCompleted := false
		err = securetunnel.Serve(ctx, conn, m.host, expected, nonce, &s.metrics, policy, func(pin string) error {
			m.mu.Lock()
			defer m.mu.Unlock()
			if ctx.Err() != nil || m.active != s {
				return context.Canceled
			}
			attemptCompleted = true
			if expected == "" && remember {
				copyConfig := m.config
				copyConfig.Trusted = append(append([]Trusted{}, m.config.Trusted...), Trusted{Serial: s.device.Serial, Model: s.device.Model, Pin: pin, PairedAt: time.Now().UTC()})
				if err := m.save(copyConfig); err != nil {
					return errors.New("could not save this pairing securely")
				}
				m.config = copyConfig
			}
			authenticated = true
			s.connected = true
			s.started = time.Now()
			m.attempts[s.device.Serial] = 0
			m.state = "connected"
			m.message = "Your phone is using this computer's Internet"
			return nil
		})
		if authenticated || attemptCompleted || errors.Is(err, securetunnel.ErrIdentityChanged) {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
}
func (m *Manager) Disconnect() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.active != nil {
		m.suppressed[m.active.device.Serial] = true
		m.active.cancel()
	}
	m.state = "disconnected"
	m.message = "Disconnected"
}
func (m *Manager) Forget(serial string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	next := m.config
	next.Trusted = nil
	for _, t := range m.config.Trusted {
		if t.Serial != serial {
			next.Trusted = append(next.Trusted, t)
		}
	}
	if err := m.save(next); err != nil {
		return errors.New("could not update the protected pairing store")
	}
	m.config = next
	m.suppressed[serial] = true
	if m.active != nil && m.active.device.Serial == serial {
		m.active.cancel()
	}
	return nil
}
func (m *Manager) SetSettings(settings Settings) error {
	if settings.ADBPath != "" {
		if !filepath.IsAbs(settings.ADBPath) || !strings.EqualFold(filepath.Base(settings.ADBPath), "adb.exe") {
			return errors.New("choose an absolute path to adb.exe")
		}
		if info, err := os.Stat(settings.ADBPath); err != nil || info.IsDir() {
			return errors.New("the selected adb.exe was not found")
		}
	}

	address, err := netip.ParseAddr(settings.DNS)
	if err != nil || !address.Is4() || address.IsLoopback() || address.IsUnspecified() || address.IsMulticast() || address.IsLinkLocalUnicast() || (!settings.AllowLAN && address.IsPrivate()) {
		return errors.New("enter an allowed IPv4 DNS server")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	next := m.config
	next.Settings = settings
	if err := m.save(next); err != nil {
		return errors.New("could not save settings securely")
	}
	m.config = next
	return nil
}
func (m *Manager) Snapshot() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := Snapshot{State: m.state, Message: m.message, Settings: m.config.Settings, Trusted: append([]Trusted{}, m.config.Trusted...), HostPin: m.host.Pin, Devices: []Device{}}
	for _, d := range m.devices {
		t := m.trust(d.Serial)
		out.Devices = append(out.Devices, Device{Device: d, Trusted: t.Pin != "", Pin: t.Pin})
	}
	sort.Slice(out.Devices, func(i, j int) bool { return out.Devices[i].Serial < out.Devices[j].Serial })
	if s := m.active; s != nil {
		out.Serial = s.device.Serial
		out.DownloadBytes = s.metrics.OutBytes.Load()
		out.UploadBytes = s.metrics.InBytes.Load()
		out.TCPQueueWaits = s.metrics.TCPQueueWaits.Load()
		out.TransportPackets = s.metrics.OutPackets.Load()
		out.TransportFlushes = s.metrics.TransportFlushes.Load()
		out.QueueRejections = s.metrics.QueueRejections.Load()
		out.ActiveFlows = s.metrics.ActiveFlows.Load()
		if s.connected {
			out.UptimeSeconds = int64(time.Since(s.started).Seconds())
		}
		now := time.Now()
		if elapsed := now.Sub(s.lastSample).Seconds(); elapsed >= .5 {
			s.downRate = float64(out.DownloadBytes-s.lastOut) * 8 / elapsed / 1e6
			s.upRate = float64(out.UploadBytes-s.lastIn) * 8 / elapsed / 1e6
			s.lastOut = out.DownloadBytes
			s.lastIn = out.UploadBytes
			s.lastSample = now
		}
		out.DownloadMbps = s.downRate
		out.UploadMbps = s.upRate
	}
	return out
}

// SetDiagnosticPorts enables only explicit local test servers before Run starts.
func (m *Manager) SetDiagnosticPorts(ports []uint16) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ctx != nil {
		return
	}
	m.diagnosticPorts = map[uint16]bool{}
	for _, port := range ports {
		m.diagnosticPorts[port] = true
	}
}
