//go:build !wasip1 && !windows

package wnetruntime

import (
	"net/netip"

	"github.com/egdaemon/wasinet/wasinet/internal/langx"
	"golang.org/x/sys/unix"
)

// Firewall enforces allow-then-block CIDR-range restriction shared by every
// Socket implementation in this package: an address matching Allow is always
// permitted; otherwise an address matching Block is rejected.
type Firewall struct {
	Allow []netip.Prefix
	Block []netip.Prefix
}

// FirewallOption configures a Firewall via NewFirewall.
type FirewallOption func(*Firewall)

// FirewallOptionAllow appends cidrs to the firewall's allow list.
func FirewallOptionAllow(cidrs ...netip.Prefix) FirewallOption {
	return func(f *Firewall) { f.Allow = append(f.Allow, cidrs...) }
}

// FirewallOptionBlock appends cidrs to the firewall's block list.
func FirewallOptionBlock(cidrs ...netip.Prefix) FirewallOption {
	return func(f *Firewall) { f.Block = append(f.Block, cidrs...) }
}

// NewFirewall builds a Firewall from opts, applied in order. With no opts
// the result is unrestricted: every address is permitted.
func NewFirewall(opts ...FirewallOption) Firewall {
	return langx.Clone(Firewall{}, opts...)
}

// UnrestrictedFirewall permits every address unless narrowed via opts.
func UnrestrictedFirewall(opts ...FirewallOption) Firewall {
	return NewFirewall(opts...)
}

// PublicFirewall restricts to public IP address space, blocking private,
// loopback, link-local, and multicast ranges by default. opts are applied
// afterwards and can widen or narrow the defaults.
func PublicFirewall(opts ...FirewallOption) Firewall {
	return NewFirewall(append([]FirewallOption{FirewallOptionBlock(PrivatePrefixes()...)}, opts...)...)
}

func (t Firewall) restricted(sa unix.Sockaddr) error {
	addr, ok := sockaddrAddr(sa)
	if !ok {
		return nil
	}

	for _, p := range t.Allow {
		if p.Contains(addr) {
			return nil
		}
	}

	for _, p := range t.Block {
		if p.Contains(addr) {
			return unix.EACCES
		}
	}

	return nil
}
