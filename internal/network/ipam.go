package network

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/r4r00t-sh/petti/internal/state"
)

// ipamFile is the on-disk allocation map: containerID → IP string.
type ipamFile struct {
	Subnet string            `json:"subnet"`
	Alloc  map[string]string `json:"alloc"` // id -> IP
}

func ipamPath(dataRoot string) string {
	return filepath.Join(dataRoot, "network", "ipam.json")
}

func ipamLock(dataRoot string) string {
	return ipamPath(dataRoot) + ".lock"
}

// AllocateIP reserves the next free address in 10.88.0.0/16 for id.
// 10.88.0.0 (network) and 10.88.0.1 (bridge) are never allocated.
func AllocateIP(dataRoot, id string) (net.IP, error) {
	if dataRoot == "" {
		return nil, fmt.Errorf("data root required for IPAM")
	}
	if id == "" {
		return nil, fmt.Errorf("container id required for IPAM")
	}
	_, cidr, err := net.ParseCIDR(SubnetCIDR)
	if err != nil {
		return nil, err
	}

	var out net.IP
	err = state.WithLock(ipamLock(dataRoot), func() error {
		f, err := loadIPAM(dataRoot)
		if err != nil {
			return err
		}
		if existing, ok := f.Alloc[id]; ok {
			ip := net.ParseIP(existing)
			if ip == nil {
				return fmt.Errorf("corrupt IPAM entry for %s: %q", id, existing)
			}
			out = ip.To4()
			return nil
		}
		used := map[string]struct{}{BridgeIP: {}}
		for _, ip := range f.Alloc {
			used[ip] = struct{}{}
		}
		ip := nextIP(cidr, used)
		if ip == nil {
			return fmt.Errorf("IPAM pool exhausted (%s)", SubnetCIDR)
		}
		f.Alloc[id] = ip.String()
		f.Subnet = SubnetCIDR
		if err := saveIPAM(dataRoot, f); err != nil {
			return err
		}
		out = ip
		return nil
	})
	return out, err
}

// ReleaseIP frees the address held by id (idempotent).
func ReleaseIP(dataRoot, id string) error {
	if dataRoot == "" || id == "" {
		return nil
	}
	return state.WithLock(ipamLock(dataRoot), func() error {
		f, err := loadIPAM(dataRoot)
		if err != nil {
			return err
		}
		if _, ok := f.Alloc[id]; !ok {
			return nil
		}
		delete(f.Alloc, id)
		return saveIPAM(dataRoot, f)
	})
}

func loadIPAM(dataRoot string) (*ipamFile, error) {
	path := ipamPath(dataRoot)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ipamFile{Subnet: SubnetCIDR, Alloc: map[string]string{}}, nil
		}
		return nil, err
	}
	var f ipamFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("ipam: %w", err)
	}
	if f.Alloc == nil {
		f.Alloc = map[string]string{}
	}
	return &f, nil
}

func saveIPAM(dataRoot string, f *ipamFile) error {
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return state.WriteFileAtomic(ipamPath(dataRoot), data, 0o644)
}

// nextIP walks the /16 from .2 upward and returns the first free IPv4.
func nextIP(cidr *net.IPNet, used map[string]struct{}) net.IP {
	ip := make(net.IP, len(cidr.IP.To4()))
	copy(ip, cidr.IP.To4())
	// start at .2
	ip[3] = 2
	for {
		if !cidr.Contains(ip) {
			return nil
		}
		s := ip.String()
		if _, ok := used[s]; !ok {
			out := make(net.IP, 4)
			copy(out, ip)
			return out
		}
		// increment
		for i := 3; i >= 0; i-- {
			ip[i]++
			if ip[i] != 0 {
				break
			}
		}
	}
}
