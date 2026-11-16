package relay

import (
	"net/netip"
)

// Policy denies PC-local services and private networks unless explicitly allowed.
// DiagnosticPorts is restricted to locally created test servers, never all loopback ports.
type Policy struct {
	AllowLAN        bool
	DiagnosticPorts map[uint16]bool
	HostAddresses   map[netip.Addr]bool
	LegacyLab       bool
}

func (p Policy) allow(address netip.Addr, port uint16) bool {
	if p.LegacyLab {
		return true
	}
	address = address.Unmap()
	if address == netip.MustParseAddr("10.0.2.2") {
		return p.DiagnosticPorts[port]
	}
	if address.IsLoopback() || address.IsUnspecified() || address.IsMulticast() || address.IsLinkLocalUnicast() || p.HostAddresses[address] || address == netip.MustParseAddr("255.255.255.255") {
		return false
	}
	return !address.IsPrivate() || p.AllowLAN
}
