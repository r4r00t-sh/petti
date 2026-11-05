package network

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// Network modes for `petti run --network`.
const (
	ModeBridge = "bridge"
	ModeHost   = "host"
	ModeNone   = "none"
)

const (
	// BridgeName is the Linux bridge used for ModeBridge.
	BridgeName = "petti0"
	// SubnetCIDR is the container address pool.
	SubnetCIDR = "10.88.0.0/16"
	// BridgeIP is the gateway address on petti0.
	BridgeIP = "10.88.0.1"
)

// Mode validates and normalizes a network mode string.
func ParseMode(s string) (string, error) {
	if s == "" {
		return ModeBridge, nil
	}
	switch strings.ToLower(s) {
	case ModeBridge, ModeHost, ModeNone:
		return strings.ToLower(s), nil
	default:
		return "", fmt.Errorf("invalid --network %q (want bridge|host|none)", s)
	}
}

// PortMapping is a published port (-p).
type PortMapping struct {
	HostIP        string `json:"hostIP,omitempty"` // empty = 0.0.0.0
	HostPort      int    `json:"hostPort"`
	ContainerPort int    `json:"containerPort"`
	Protocol      string `json:"protocol"` // tcp|udp
}

// String renders the mapping in Docker-like form.
func (p PortMapping) String() string {
	proto := p.Protocol
	if proto == "" {
		proto = "tcp"
	}
	if p.HostIP != "" {
		return fmt.Sprintf("%s:%d:%d/%s", p.HostIP, p.HostPort, p.ContainerPort, proto)
	}
	return fmt.Sprintf("%d:%d/%s", p.HostPort, p.ContainerPort, proto)
}

// ParsePort parses -p hostPort:containerPort[/proto] or [ip:]hostPort:containerPort[/proto].
func ParsePort(s string) (PortMapping, error) {
	proto := "tcp"
	body := s
	if i := strings.LastIndex(s, "/"); i >= 0 {
		proto = strings.ToLower(s[i+1:])
		body = s[:i]
		if proto != "tcp" && proto != "udp" {
			return PortMapping{}, fmt.Errorf("invalid port protocol %q (want tcp|udp)", proto)
		}
	}

	parts := strings.Split(body, ":")
	var m PortMapping
	m.Protocol = proto

	switch len(parts) {
	case 1:
		p, err := atoiPort(parts[0])
		if err != nil {
			return PortMapping{}, err
		}
		m.HostPort = p
		m.ContainerPort = p
	case 2:
		// host:container OR ip:port (ambiguous) — treat as hostPort:containerPort
		hp, err := atoiPort(parts[0])
		if err != nil {
			return PortMapping{}, fmt.Errorf("invalid -p %q: %w", s, err)
		}
		cp, err := atoiPort(parts[1])
		if err != nil {
			return PortMapping{}, fmt.Errorf("invalid -p %q: %w", s, err)
		}
		m.HostPort = hp
		m.ContainerPort = cp
	case 3:
		if net.ParseIP(parts[0]) == nil {
			return PortMapping{}, fmt.Errorf("invalid -p %q: bad host IP", s)
		}
		hp, err := atoiPort(parts[1])
		if err != nil {
			return PortMapping{}, fmt.Errorf("invalid -p %q: %w", s, err)
		}
		cp, err := atoiPort(parts[2])
		if err != nil {
			return PortMapping{}, fmt.Errorf("invalid -p %q: %w", s, err)
		}
		m.HostIP = parts[0]
		m.HostPort = hp
		m.ContainerPort = cp
	default:
		return PortMapping{}, fmt.Errorf("invalid -p %q (want [ip:]hostPort:containerPort[/proto])", s)
	}
	return m, nil
}

func atoiPort(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > 65535 {
		return 0, fmt.Errorf("invalid port %q", s)
	}
	return n, nil
}

// Endpoint is the live network state for one container (persisted for teardown).
type Endpoint struct {
	ContainerID string        `json:"containerID"`
	Mode        string        `json:"mode"`
	IP          string        `json:"ip,omitempty"`
	Gateway     string        `json:"gateway,omitempty"`
	HostVeth    string        `json:"hostVeth,omitempty"`
	Ports       []PortMapping `json:"ports,omitempty"`
}

// SetupOpts configures network for a started container init (before sync).
type SetupOpts struct {
	ContainerID string
	Pid         int
	Hostname    string
	Mode        string
	Ports       []PortMapping
	DataRoot    string // Petti data root for IPAM
}
