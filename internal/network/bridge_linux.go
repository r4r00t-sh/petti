//go:build linux

package network

import (
	"errors"
	"fmt"
	"net"
	"os"

	"github.com/vishvananda/netlink"
)

// EnsureBridge creates petti0 with 10.88.0.1/16 if it does not exist.
func EnsureBridge() error {
	link, err := netlink.LinkByName(BridgeName)
	if err != nil {
		var notFound netlink.LinkNotFoundError
		if !errors.As(err, &notFound) {
			return fmt.Errorf("lookup bridge %s: %w", BridgeName, err)
		}
		br := &netlink.Bridge{LinkAttrs: netlink.LinkAttrs{Name: BridgeName}}
		if err := netlink.LinkAdd(br); err != nil {
			return fmt.Errorf("create bridge %s: %w", BridgeName, err)
		}
		link, err = netlink.LinkByName(BridgeName)
		if err != nil {
			return fmt.Errorf("bridge %s after create: %w", BridgeName, err)
		}
	}

	addr, err := netlink.ParseAddr(BridgeIP + "/16")
	if err != nil {
		return err
	}
	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return err
	}
	have := false
	for _, a := range addrs {
		if a.IP.Equal(addr.IP) {
			have = true
			break
		}
	}
	if !have {
		if err := netlink.AddrAdd(link, addr); err != nil {
			return fmt.Errorf("addr add %s on %s: %w", BridgeIP, BridgeName, err)
		}
	}
	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("up %s: %w", BridgeName, err)
	}
	return nil
}

// EnableIPForward sets net.ipv4.ip_forward=1.
func EnableIPForward() error {
	return os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1\n"), 0o644)
}

// bridgeGatewayIP returns the parsed gateway address.
func bridgeGatewayIP() net.IP {
	return net.ParseIP(BridgeIP).To4()
}
