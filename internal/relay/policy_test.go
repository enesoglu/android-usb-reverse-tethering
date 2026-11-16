package relay

import (
	"net/netip"
	"testing"
)

func TestDestinationPolicy(t *testing.T) {
	for _, entry := range []struct {
		ip   string
		lan  bool
		want bool
	}{{"1.1.1.1", false, true}, {"127.0.0.1", true, false}, {"169.254.169.254", true, false}, {"192.168.1.10", false, false}, {"192.168.1.10", true, true}, {"224.0.0.1", true, false}, {"10.0.2.2", true, false}, {"0.0.0.0", true, false}} {
		p := Policy{AllowLAN: entry.lan}
		if got := p.allow(netip.MustParseAddr(entry.ip), 443); got != entry.want {
			t.Errorf("%s lan=%v: %v", entry.ip, entry.lan, got)
		}
	}
	p := Policy{DiagnosticPorts: map[uint16]bool{12345: true}}
	if !p.allow(netip.MustParseAddr("10.0.2.2"), 12345) || p.allow(netip.MustParseAddr("10.0.2.2"), 12346) {
		t.Error("diagnostic access escaped port allowlist")
	}
}
