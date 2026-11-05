//go:build linux

package network

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

// Setup configures networking for a container whose init is already running
// (blocked on the sync pipe). Must be called before releasing sync.
func Setup(opts SetupOpts) (*Endpoint, error) {
	mode, err := ParseMode(opts.Mode)
	if err != nil {
		return nil, err
	}
	if opts.Pid <= 0 {
		return nil, fmt.Errorf("network setup: invalid pid")
	}
	ep := &Endpoint{
		ContainerID: opts.ContainerID,
		Mode:        mode,
		Ports:       opts.Ports,
	}

	switch mode {
	case ModeHost:
		return ep, nil
	case ModeNone:
		// Loopback is brought up inside __init (already in the netns). Avoids
		// fragile setns-from-parent around a just-cloned child.
		return ep, nil
	case ModeBridge:
		return setupBridge(opts, ep)
	default:
		return nil, fmt.Errorf("unsupported network mode %q", mode)
	}
}

func setupBridge(opts SetupOpts, ep *Endpoint) (*Endpoint, error) {
	if err := EnsureBridge(); err != nil {
		return nil, err
	}
	if err := EnableIPForward(); err != nil {
		return nil, fmt.Errorf("ip_forward: %w", err)
	}
	if err := DefaultNAT.EnsureMasquerade(SubnetCIDR, BridgeName); err != nil {
		return nil, fmt.Errorf("masquerade: %w", err)
	}
	if err := DefaultNAT.EnsureForward(BridgeName); err != nil {
		return nil, fmt.Errorf("forward: %w", err)
	}

	ip, err := AllocateIP(opts.DataRoot, opts.ContainerID)
	if err != nil {
		return nil, err
	}
	ep.IP = ip.String()
	ep.Gateway = BridgeIP

	hostName, peerName := vethNames(opts.ContainerID)
	ep.HostVeth = hostName

	// Clean leftover host veth from a previous crash.
	if old, err := netlink.LinkByName(hostName); err == nil {
		_ = netlink.LinkDel(old)
	}

	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: hostName},
		PeerName:  peerName,
	}
	if err := netlink.LinkAdd(veth); err != nil {
		_ = ReleaseIP(opts.DataRoot, opts.ContainerID)
		return nil, fmt.Errorf("create veth: %w", err)
	}

	hostLink, err := netlink.LinkByName(hostName)
	if err != nil {
		_ = cleanupVeth(hostName)
		_ = ReleaseIP(opts.DataRoot, opts.ContainerID)
		return nil, err
	}
	peerLink, err := netlink.LinkByName(peerName)
	if err != nil {
		_ = cleanupVeth(hostName)
		_ = ReleaseIP(opts.DataRoot, opts.ContainerID)
		return nil, err
	}

	br, err := netlink.LinkByName(BridgeName)
	if err != nil {
		_ = cleanupVeth(hostName)
		_ = ReleaseIP(opts.DataRoot, opts.ContainerID)
		return nil, err
	}
	if err := netlink.LinkSetMaster(hostLink, br); err != nil {
		_ = cleanupVeth(hostName)
		_ = ReleaseIP(opts.DataRoot, opts.ContainerID)
		return nil, fmt.Errorf("attach %s to %s: %w", hostName, BridgeName, err)
	}
	if err := netlink.LinkSetUp(hostLink); err != nil {
		_ = cleanupVeth(hostName)
		_ = ReleaseIP(opts.DataRoot, opts.ContainerID)
		return nil, fmt.Errorf("up %s: %w", hostName, err)
	}

	if err := netlink.LinkSetNsPid(peerLink, opts.Pid); err != nil {
		_ = cleanupVeth(hostName)
		_ = ReleaseIP(opts.DataRoot, opts.ContainerID)
		return nil, fmt.Errorf("move peer to netns: %w", err)
	}

	if err := withNetNS(opts.Pid, func() error {
		return configureContainerIface(peerName, ip, bridgeGatewayIP())
	}); err != nil {
		_ = cleanupVeth(hostName)
		_ = ReleaseIP(opts.DataRoot, opts.ContainerID)
		return nil, err
	}

	for _, p := range opts.Ports {
		if err := DefaultNAT.Publish(*ep, p); err != nil {
			_ = Teardown(opts.DataRoot, *ep)
			return nil, fmt.Errorf("publish %s: %w", p.String(), err)
		}
	}
	return ep, nil
}

func configureContainerIface(peerName string, ip, gw net.IP) error {
	if err := setLoopbackUp(); err != nil {
		return err
	}
	var link netlink.Link
	var err error
	// LinkSetNsPid can race briefly with netns visibility.
	for i := 0; i < 20; i++ {
		link, err = netlink.LinkByName(peerName)
		if err == nil {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if err != nil {
		return fmt.Errorf("peer %s in netns: %w", peerName, err)
	}
	// If a previous crashed container left eth0, delete it first.
	if old, err := netlink.LinkByName("eth0"); err == nil {
		_ = netlink.LinkDel(old)
	}
	if err := netlink.LinkSetName(link, "eth0"); err != nil {
		return fmt.Errorf("rename to eth0: %w", err)
	}
	link, err = netlink.LinkByName("eth0")
	if err != nil {
		return err
	}
	addr, err := netlink.ParseAddr(ip.String() + "/16")
	if err != nil {
		return err
	}
	if err := netlink.AddrAdd(link, addr); err != nil {
		return fmt.Errorf("addr add: %w", err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("up eth0: %w", err)
	}
	route := &netlink.Route{
		LinkIndex: link.Attrs().Index,
		Gw:        gw,
	}
	if err := netlink.RouteAdd(route); err != nil {
		return fmt.Errorf("default route via %s: %w", gw, err)
	}
	return nil
}

func setLoopbackUp() error {
	lo, err := netlink.LinkByName("lo")
	if err != nil {
		return fmt.Errorf("lo: %w", err)
	}
	if err := netlink.LinkSetUp(lo); err != nil {
		return fmt.Errorf("up lo: %w", err)
	}
	return nil
}

func withNetNS(pid int, fn func() error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	origns, err := currentNetNS()
	if err != nil {
		return fmt.Errorf("get current netns: %w", err)
	}
	defer origns.Close()

	var ns netns.NsHandle
	for i := 0; i < 20; i++ {
		ns, err = netns.GetFromPid(pid)
		if err == nil {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if err != nil {
		return fmt.Errorf("get netns of pid %d: %w", pid, err)
	}
	defer ns.Close()

	if err := netns.Set(ns); err != nil {
		return fmt.Errorf("setns: %w", err)
	}
	defer func() { _ = netns.Set(origns) }()

	return fn()
}

func currentNetNS() (netns.NsHandle, error) {
	ns, err := netns.Get()
	if err == nil {
		return ns, nil
	}
	// Fallback when netns.Get cannot resolve the path (some CI /proc layouts).
	f, err2 := os.Open("/proc/self/ns/net")
	if err2 != nil {
		return 0, err
	}
	fd, err2 := unix.Dup(int(f.Fd()))
	_ = f.Close()
	if err2 != nil {
		return 0, err
	}
	return netns.NsHandle(fd), nil
}

// SetLoopbackUp brings up lo in the current network namespace (used by __init).
func SetLoopbackUp() error {
	return setLoopbackUp()
}

func vethNames(id string) (host, peer string) {
	suf := id
	if len(suf) > 8 {
		suf = suf[:8]
	}
	return "veth" + suf, "vpeer" + suf
}

func cleanupVeth(hostName string) error {
	link, err := netlink.LinkByName(hostName)
	if err != nil {
		return nil
	}
	return netlink.LinkDel(link)
}

// Teardown removes host veth, DNAT rules, and frees the IP (idempotent).
func Teardown(dataRoot string, ep Endpoint) error {
	if ep.Mode == ModeHost || ep.Mode == "" {
		return nil
	}
	var first error
	if ep.Mode == ModeBridge {
		for _, p := range ep.Ports {
			if err := DefaultNAT.Unpublish(ep, p); err != nil && first == nil {
				first = err
			}
		}
		if ep.HostVeth != "" {
			if err := cleanupVeth(ep.HostVeth); err != nil && first == nil {
				first = err
			}
		}
		if err := ReleaseIP(dataRoot, ep.ContainerID); err != nil && first == nil {
			first = err
		}
	}
	return first
}
