//go:build linux

package seccomp

import (
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/sys/unix"
)

// BlockedNames documents the default denylist for docs/tests.
func BlockedNames() []string {
	return []string{
		"acct", "bpf", "delete_module", "finit_module", "init_module",
		"kexec_file_load", "kexec_load", "keyctl", "mount", "mount_setattr",
		"move_mount", "open_tree", "perf_event_open", "pivot_root", "ptrace",
		"reboot", "setns", "swapon", "umount2", "unshare", "userfaultfd",
		"fsopen", "fsconfig", "fsmount", "fspick",
	}
}

// defaultBlocked returns syscall numbers to deny on this GOARCH build.
func defaultBlocked() []uint32 {
	return []uint32{
		unix.SYS_ACCT,
		unix.SYS_BPF,
		unix.SYS_DELETE_MODULE,
		unix.SYS_FINIT_MODULE,
		unix.SYS_INIT_MODULE,
		unix.SYS_KEXEC_FILE_LOAD,
		unix.SYS_KEXEC_LOAD,
		unix.SYS_KEYCTL,
		unix.SYS_MOUNT,
		unix.SYS_MOUNT_SETATTR,
		unix.SYS_MOVE_MOUNT,
		unix.SYS_OPEN_TREE,
		unix.SYS_PERF_EVENT_OPEN,
		unix.SYS_PIVOT_ROOT,
		unix.SYS_PTRACE,
		unix.SYS_REBOOT,
		unix.SYS_SETNS,
		unix.SYS_SWAPON,
		unix.SYS_UMOUNT2,
		unix.SYS_UNSHARE,
		unix.SYS_USERFAULTFD,
		unix.SYS_FSOPEN,
		unix.SYS_FSCONFIG,
		unix.SYS_FSMOUNT,
		unix.SYS_FSPICK,
	}
}

func nativeArch() (uint32, error) {
	switch runtime.GOARCH {
	case "amd64":
		return unix.AUDIT_ARCH_X86_64, nil
	case "arm64":
		return unix.AUDIT_ARCH_AARCH64, nil
	default:
		return 0, fmt.Errorf("seccomp: unsupported GOARCH %q", runtime.GOARCH)
	}
}

// Apply installs the default denylist (or no-ops for ModeUnconfined).
// Caller must have already set PR_SET_NO_NEW_PRIVS.
func Apply(mode Mode) error {
	if mode == "" {
		mode = ModeDefault
	}
	if mode == ModeUnconfined {
		return nil
	}
	if mode != ModeDefault {
		return fmt.Errorf("seccomp: unknown mode %q", mode)
	}

	arch, err := nativeArch()
	if err != nil {
		return err
	}
	blocked := defaultBlocked()
	filter := buildFilter(arch, blocked)

	prog := unix.SockFprog{
		Len:    uint16(len(filter)),
		Filter: &filter[0],
	}
	_, _, errno := unix.Syscall(unix.SYS_SECCOMP, uintptr(unix.SECCOMP_SET_MODE_FILTER), 0, uintptr(unsafe.Pointer(&prog)))
	if errno != 0 {
		return fmt.Errorf("seccomp SET_MODE_FILTER: %w", errno)
	}
	runtime.KeepAlive(filter)
	return nil
}

func buildFilter(arch uint32, blocked []uint32) []unix.SockFilter {
	// Classic BPF seccomp filter:
	//  1) validate arch
	//  2) load syscall nr
	//  3) for each blocked nr → RET_ERRNO|EPERM
	//  4) RET_ALLOW
	const (
		bpfLD  = unix.BPF_LD
		bpfW   = unix.BPF_W
		bpfABS = unix.BPF_ABS
		bpfJMP = unix.BPF_JMP
		bpfJEQ = unix.BPF_JEQ
		bpfK   = unix.BPF_K
		bpfRET = unix.BPF_RET
		bpfIMM = unix.BPF_IMM
	)
	// Offsets into struct seccomp_data { int nr; __u32 arch; ... }
	const (
		offNR   = 0
		offArch = 4
	)

	stmt := func(code uint16, k uint32) unix.SockFilter {
		return unix.SockFilter{Code: code, K: k}
	}
	jump := func(code uint16, jt, jf uint8, k uint32) unix.SockFilter {
		return unix.SockFilter{Code: code, Jt: jt, Jf: jf, K: k}
	}

	out := make([]unix.SockFilter, 0, 4+2*len(blocked))
	// Load arch.
	out = append(out, stmt(bpfLD|bpfW|bpfABS, offArch))
	// if arch == expected → skip kill; else kill
	out = append(out, jump(bpfJMP|bpfJEQ|bpfK, 1, 0, arch))
	out = append(out, stmt(bpfRET|bpfK, unix.SECCOMP_RET_KILL_THREAD))
	// Load nr.
	out = append(out, stmt(bpfLD|bpfW|bpfABS, offNR))
	for _, nr := range blocked {
		// if nr == blocked → fall through to errno; else skip errno insn
		out = append(out, jump(bpfJMP|bpfJEQ|bpfK, 0, 1, nr))
		out = append(out, stmt(bpfRET|bpfK, unix.SECCOMP_RET_ERRNO|uint32(unix.EPERM)))
	}
	out = append(out, stmt(bpfRET|bpfK, unix.SECCOMP_RET_ALLOW))
	return out
}
