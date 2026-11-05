package network

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestAllocateReleaseIP(t *testing.T) {
	root := t.TempDir()
	ip1, err := AllocateIP(root, "c1")
	if err != nil {
		t.Fatal(err)
	}
	if ip1.String() != "10.88.0.2" {
		t.Fatalf("first alloc: %s", ip1)
	}
	ip2, err := AllocateIP(root, "c2")
	if err != nil {
		t.Fatal(err)
	}
	if ip2.String() != "10.88.0.3" {
		t.Fatalf("second alloc: %s", ip2)
	}
	again, err := AllocateIP(root, "c1")
	if err != nil {
		t.Fatal(err)
	}
	if again.String() != ip1.String() {
		t.Fatalf("re-alloc: %s vs %s", again, ip1)
	}
	if err := ReleaseIP(root, "c1"); err != nil {
		t.Fatal(err)
	}
	ip3, err := AllocateIP(root, "c3")
	if err != nil {
		t.Fatal(err)
	}
	if ip3.String() != "10.88.0.2" {
		t.Fatalf("reuse after release: %s", ip3)
	}
	if _, err := os.Stat(filepath.Join(root, "network", "ipam.json")); err != nil {
		t.Fatal(err)
	}
}

func TestNextIPSkipsBridge(t *testing.T) {
	_, cidr, err := net.ParseCIDR(SubnetCIDR)
	if err != nil {
		t.Fatal(err)
	}
	used := map[string]struct{}{BridgeIP: {}}
	ip := nextIP(cidr, used)
	if ip.String() != "10.88.0.2" {
		t.Fatalf("got %s", ip)
	}
}
