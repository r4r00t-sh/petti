# Security / threat model

Petti is an **educational** container runtime. It is **not** safe for untrusted
workloads. This document states what it isolates, what it deliberately does
not, and how that compares to runc/Docker.

## Trust boundaries

| Layer | Petti (root mode) | Petti (rootless) | runc / Docker (typical) |
|-------|-------------------|------------------|-------------------------|
| Namespaces (pid/mnt/uts/ipc) | yes | yes | yes |
| Network namespace | yes (`bridge`/`none`) | default `host` | yes |
| User namespace | opt-in (`--rootless`) | yes | often yes (userns-remap / rootless) |
| `pivot_root` | yes | yes | yes |
| Capabilities (default drop) | yes | yes (scoped to userns) | yes |
| `no_new_privs` | yes | yes | yes |
| Seccomp | denylist default | denylist default | large allowlist (Docker default profile) |
| cgroup v2 limits | yes | best-effort (user delegation) | yes |
| Masked `/proc`/`/sys` paths | yes | yes | yes |
| AppArmor / SELinux | **no** | **no** | often yes |
| Shared kernel | **yes — always** | **yes — always** | **yes — always** |

A kernel bug in any of namespaces, cgroups, overlayfs, netfilter, or seccomp
can become a host compromise. Petti does not claim otherwise.

## What Petti isolates

1. **Process tree** — `CLONE_NEWPID`; container init is PID 1 inside.
2. **Mounts / filesystem** — private mount ns, `pivot_root`, old root unmounted;
   optional `--read-only` root.
3. **Hostname / IPC** — UTS and IPC namespaces.
4. **Network** — netns + `petti0` bridge (root) or host net (rootless default).
5. **Capabilities** — Docker-like default allow-list; `--cap-add` / `--cap-drop`.
6. **Privilege escalation via setuid** — `PR_SET_NO_NEW_PRIVS`.
7. **Dangerous syscalls** — seccomp denylist (see below); disable with
   `--security-opt seccomp=unconfined`.
8. **Sensitive proc/sys nodes** — `/proc/kcore`, `/proc/sysrq-trigger`,
   `/proc/timer_list`, `/sys/firmware` masked; `/proc/sys`, `/proc/irq`,
   `/proc/bus` remounted read-only.
9. **Resources** — optional cgroup v2 memory/CPU/PIDs.

### Default capabilities (kept)

`CHOWN`, `DAC_OVERRIDE`, `FOWNER`, `FSETID`, `KILL`, `SETGID`, `SETUID`,
`SETPCAP`, `NET_BIND_SERVICE`, `NET_RAW`, `SYS_CHROOT`, `MKNOD`,
`AUDIT_WRITE`, `SETFCAP`

### Default seccomp denylist (blocked → `EPERM`)

`acct`, `bpf`, `delete_module`, `finit_module`, `init_module`,
`kexec_load`, `kexec_file_load`, `keyctl`, `mount`, `umount2`, `pivot_root`,
`mount_setattr`, `move_mount`, `open_tree`, `fsopen`, `fsconfig`, `fsmount`,
`fspick`, `ptrace`, `reboot`, `swapon`, `setns`, `unshare`, `userfaultfd`,
`perf_event_open`

This is a **denylist**, not Docker’s default **allowlist**. New or obscure
syscalls are still permitted. That is an intentional educational tradeoff
(small hand-rolled BPF filter, no libseccomp) and a real security gap versus
runc.

## What Petti does **not** claim

- Safety against a malicious image or a kernel exploit
- AppArmor / SELinux policy generation or enforcement
- Seccomp allowlist parity with Docker/runc
- User-namespace remapping **by default** in root mode (use `--rootless`)
- Protection of the host’s netfilter rules, bridge, or cgroup controllers from
  a root-mode container that somehow regained `CAP_NET_ADMIN` / `CAP_SYS_ADMIN`
- Multi-tenant isolation suitable for a public PaaS

## Root mode vs rootless

In **root mode**, the container shares the host user namespace unless
`--rootless` is set. Container “root” is host root for UIDs. Capabilities are
dropped, but a missed cap or a kernel bug is host-root impact.

In **rootless** mode, UIDs are mapped (host user → container root; subordinate
range via `newuidmap` when available). Capabilities are namespaced. Bridge
networking is unavailable (no host `CAP_NET_ADMIN`).

## Comparison to runc (honest)

| Topic | Petti | runc |
|-------|-------|------|
| Purpose | Learn / portfolio | Production OCI runtime |
| Seccomp | Small denylist | OCI profile / Docker default allowlist |
| LSMs | None | Optional AppArmor/SELinux hooks |
| Conformance | Not claimed | OCI runtime-spec |
| Attack surface review | Informal | Years of production hardening |

If you need to run untrusted code, use a production runtime (or a VM), not Petti.

## Reporting

Treat this project as experimental. Do not run untrusted images with Petti.
