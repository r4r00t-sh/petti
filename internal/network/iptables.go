package network

import (
	"fmt"
	"os/exec"
	"strings"
)

// NAT manages host iptables rules for masquerade and published ports.
// Shelling out to iptables is intentional (educational runtime; isolated here).
type NAT interface {
	EnsureMasquerade(subnetCIDR, bridge string) error
	EnsureForward(bridge string) error
	Publish(ep Endpoint, p PortMapping) error
	Unpublish(ep Endpoint, p PortMapping) error
}

// IPTables is the default NAT implementation using the iptables CLI.
type IPTables struct {
	Bin string // defaults to "iptables"
}

func (t *IPTables) bin() string {
	if t.Bin != "" {
		return t.Bin
	}
	return "iptables"
}

func (t *IPTables) run(args ...string) error {
	cmd := exec.Command(t.bin(), args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("iptables %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (t *IPTables) EnsureMasquerade(subnetCIDR, bridge string) error {
	rule := []string{
		"POSTROUTING",
		"-s", subnetCIDR, "!", "-o", bridge,
		"-j", "MASQUERADE",
		"-m", "comment", "--comment", "petti:masquerade",
	}
	return t.ensureNat(rule...)
}

func (t *IPTables) EnsureForward(bridge string) error {
	if err := t.ensureFilter("FORWARD", "-i", bridge, "-j", "ACCEPT", "-m", "comment", "--comment", "petti:forward-in"); err != nil {
		return err
	}
	return t.ensureFilter("FORWARD", "-o", bridge, "-j", "ACCEPT", "-m", "comment", "--comment", "petti:forward-out")
}

func (t *IPTables) ensureFilter(args ...string) error {
	check := append([]string{"-C"}, args...)
	if exec.Command(t.bin(), check...).Run() == nil {
		return nil
	}
	return t.run(append([]string{"-A"}, args...)...)
}

func (t *IPTables) ensureNat(chainAndRule ...string) error {
	check := append([]string{"-t", "nat", "-C"}, chainAndRule...)
	if exec.Command(t.bin(), check...).Run() == nil {
		return nil
	}
	return t.run(append([]string{"-t", "nat", "-A"}, chainAndRule...)...)
}

func portComment(id string, p PortMapping) string {
	return fmt.Sprintf("petti:port:%s:%d", id, p.HostPort)
}

func dnatRule(ep Endpoint, p PortMapping, chain string) []string {
	proto := p.Protocol
	if proto == "" {
		proto = "tcp"
	}
	dest := fmt.Sprintf("%s:%d", ep.IP, p.ContainerPort)
	rule := []string{chain, "-p", proto}
	if p.HostIP != "" && p.HostIP != "0.0.0.0" {
		rule = append(rule, "-d", p.HostIP)
	}
	rule = append(rule,
		"--dport", fmt.Sprintf("%d", p.HostPort),
		"-j", "DNAT", "--to-destination", dest,
		"-m", "comment", "--comment", portComment(ep.ContainerID, p),
	)
	return rule
}

func (t *IPTables) Publish(ep Endpoint, p PortMapping) error {
	// PREROUTING for external clients; OUTPUT for host-local curls to published ports.
	if err := t.ensureNat(dnatRule(ep, p, "PREROUTING")...); err != nil {
		return err
	}
	return t.ensureNat(dnatRule(ep, p, "OUTPUT")...)
}

func (t *IPTables) Unpublish(ep Endpoint, p PortMapping) error {
	for _, chain := range []string{"PREROUTING", "OUTPUT"} {
		rule := dnatRule(ep, p, chain)
		_ = t.run(append([]string{"-t", "nat", "-D"}, rule...)...)
	}
	return nil
}

// DefaultNAT is the process-wide NAT backend (overridable in tests).
var DefaultNAT NAT = &IPTables{}
