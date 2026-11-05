//go:build linux

package runtime

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// Default capability set kept inside the container (root). Mirrors the common
// Docker/OCI default allow-list: enough for package managers and servers,
// without CAP_SYS_ADMIN / CAP_NET_ADMIN / CAP_SYS_MODULE and friends.
//
// Documented also in docs/SECURITY.md and docs/CLI.md.
var defaultCaps = []uintptr{
	unix.CAP_CHOWN,
	unix.CAP_DAC_OVERRIDE,
	unix.CAP_FOWNER,
	unix.CAP_FSETID,
	unix.CAP_KILL,
	unix.CAP_SETGID,
	unix.CAP_SETUID,
	unix.CAP_SETPCAP,
	unix.CAP_NET_BIND_SERVICE,
	unix.CAP_NET_RAW,
	unix.CAP_SYS_CHROOT,
	unix.CAP_MKNOD,
	unix.CAP_AUDIT_WRITE,
	unix.CAP_SETFCAP,
}

// knownCaps maps normalized names (no CAP_ prefix) to values.
var knownCaps = map[string]uintptr{
	"CHOWN":            unix.CAP_CHOWN,
	"DAC_OVERRIDE":     unix.CAP_DAC_OVERRIDE,
	"DAC_READ_SEARCH":  unix.CAP_DAC_READ_SEARCH,
	"FOWNER":           unix.CAP_FOWNER,
	"FSETID":           unix.CAP_FSETID,
	"KILL":             unix.CAP_KILL,
	"SETGID":           unix.CAP_SETGID,
	"SETUID":           unix.CAP_SETUID,
	"SETPCAP":          unix.CAP_SETPCAP,
	"LINUX_IMMUTABLE":  unix.CAP_LINUX_IMMUTABLE,
	"NET_BIND_SERVICE": unix.CAP_NET_BIND_SERVICE,
	"NET_BROADCAST":    unix.CAP_NET_BROADCAST,
	"NET_ADMIN":        unix.CAP_NET_ADMIN,
	"NET_RAW":          unix.CAP_NET_RAW,
	"IPC_LOCK":         unix.CAP_IPC_LOCK,
	"IPC_OWNER":        unix.CAP_IPC_OWNER,
	"SYS_MODULE":       unix.CAP_SYS_MODULE,
	"SYS_RAWIO":        unix.CAP_SYS_RAWIO,
	"SYS_CHROOT":       unix.CAP_SYS_CHROOT,
	"SYS_PTRACE":       unix.CAP_SYS_PTRACE,
	"SYS_PACCT":        unix.CAP_SYS_PACCT,
	"SYS_ADMIN":        unix.CAP_SYS_ADMIN,
	"SYS_BOOT":         unix.CAP_SYS_BOOT,
	"SYS_NICE":         unix.CAP_SYS_NICE,
	"SYS_RESOURCE":     unix.CAP_SYS_RESOURCE,
	"SYS_TIME":         unix.CAP_SYS_TIME,
	"SYS_TTY_CONFIG":   unix.CAP_SYS_TTY_CONFIG,
	"MKNOD":            unix.CAP_MKNOD,
	"LEASE":            unix.CAP_LEASE,
	"AUDIT_WRITE":      unix.CAP_AUDIT_WRITE,
	"AUDIT_CONTROL":    unix.CAP_AUDIT_CONTROL,
	"SETFCAP":          unix.CAP_SETFCAP,
	"MAC_OVERRIDE":     unix.CAP_MAC_OVERRIDE,
	"MAC_ADMIN":        unix.CAP_MAC_ADMIN,
	"SYSLOG":           unix.CAP_SYSLOG,
	"WAKE_ALARM":       unix.CAP_WAKE_ALARM,
	"BLOCK_SUSPEND":    unix.CAP_BLOCK_SUSPEND,
	"AUDIT_READ":       unix.CAP_AUDIT_READ,
}

// DefaultCapNames is the human-readable form of defaultCaps for docs/tests.
func DefaultCapNames() []string {
	return []string{
		"CHOWN", "DAC_OVERRIDE", "FOWNER", "FSETID", "KILL",
		"SETGID", "SETUID", "SETPCAP", "NET_BIND_SERVICE", "NET_RAW",
		"SYS_CHROOT", "MKNOD", "AUDIT_WRITE", "SETFCAP",
	}
}

// ParseCapName accepts CAP_SYS_ADMIN, SYS_ADMIN, or sys_admin.
func ParseCapName(s string) (uintptr, error) {
	n := strings.ToUpper(strings.TrimSpace(s))
	n = strings.TrimPrefix(n, "CAP_")
	if n == "ALL" {
		return 0, fmt.Errorf("ALL is not a single capability")
	}
	c, ok := knownCaps[n]
	if !ok {
		return 0, fmt.Errorf("unknown capability %q", s)
	}
	return c, nil
}

// ResolveCaps applies Docker-like --cap-add / --cap-drop on top of the default set.
// Order: start from defaults → apply drops (ALL clears) → apply adds (ALL = everything).
func ResolveCaps(add, drop []string) ([]uintptr, error) {
	keep := map[uintptr]struct{}{}
	for _, c := range defaultCaps {
		keep[c] = struct{}{}
	}

	for _, d := range drop {
		if strings.EqualFold(strings.TrimPrefix(strings.TrimSpace(d), "CAP_"), "ALL") {
			keep = map[uintptr]struct{}{}
			continue
		}
		c, err := ParseCapName(d)
		if err != nil {
			return nil, fmt.Errorf("--cap-drop: %w", err)
		}
		delete(keep, c)
	}
	for _, a := range add {
		if strings.EqualFold(strings.TrimPrefix(strings.TrimSpace(a), "CAP_"), "ALL") {
			last := lastCapability()
			for c := uintptr(0); c <= last; c++ {
				keep[c] = struct{}{}
			}
			continue
		}
		c, err := ParseCapName(a)
		if err != nil {
			return nil, fmt.Errorf("--cap-add: %w", err)
		}
		keep[c] = struct{}{}
	}

	out := make([]uintptr, 0, len(keep))
	for c := range keep {
		out = append(out, c)
	}
	return out, nil
}

func lastCapability() uintptr {
	b, err := os.ReadFile("/proc/sys/kernel/cap_last_cap")
	if err != nil {
		return unix.CAP_LAST_CAP
	}
	n, err := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 32)
	if err != nil {
		return unix.CAP_LAST_CAP
	}
	return uintptr(n)
}

// dropCapabilities reduces the process to keep and clears anything else from
// the bounding set so exec'd children cannot regain them.
func dropCapabilities(keepList []uintptr) error {
	if keepList == nil {
		keepList = append([]uintptr(nil), defaultCaps...)
	}
	keep := make(map[uintptr]struct{}, len(keepList))
	for _, c := range keepList {
		keep[c] = struct{}{}
	}
	last := lastCapability()
	for cap := uintptr(0); cap <= last; cap++ {
		if _, ok := keep[cap]; ok {
			continue
		}
		if err := unix.Prctl(unix.PR_CAPBSET_DROP, cap, 0, 0, 0); err != nil {
			if err != unix.EINVAL {
				return fmt.Errorf("PR_CAPBSET_DROP %d: %w", cap, err)
			}
		}
	}

	var mask [2]uint32
	for c := range keep {
		if c < 32 {
			mask[0] |= 1 << c
		} else if c < 64 {
			mask[1] |= 1 << (c - 32)
		}
	}

	hdr := unix.CapUserHeader{
		Version: unix.LINUX_CAPABILITY_VERSION_3,
		Pid:     0,
	}
	data := [2]unix.CapUserData{
		{Effective: mask[0], Permitted: mask[0], Inheritable: 0},
		{Effective: mask[1], Permitted: mask[1], Inheritable: 0},
	}
	if err := unix.Capset(&hdr, &data[0]); err != nil {
		return fmt.Errorf("capset: %w", err)
	}
	return nil
}

// setNoNewPrivs blocks privilege gains via setuid binaries / file caps after this point.
func setNoNewPrivs() error {
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return fmt.Errorf("PR_SET_NO_NEW_PRIVS: %w", err)
	}
	return nil
}
