# Design decisions

Short log of tradeoffs. Newest first.

## Milestone 9

### Petfile subset, not full Dockerfile

Enough for demos (`FROM`/`RUN`/`COPY`/config). No multi-stage, ARG, or
build-secrets — keeps the builder understandable.

### `RUN` requires root

Needs namespaces + overlay commit of the upperdir. Documented; COPY-only
builds can still run after a local base image exists.

### `gopkg.in/yaml.v3` for compose-lite

Only non-x/sys dependency beyond netlink. Chosen over a hand-rolled YAML subset
so `petti.yaml` stays familiar.

### Skip OCI runtime-spec conformance

Would need a runc-compatible `config.json` path; educational value is lower
than build/save/compose for this portfolio slice.

## Milestone 8

### Hand-rolled BPF seccomp (no libseccomp)

Keeps the dependency story “stdlib + x/sys (+ netlink)”. The profile is a
**denylist** of obviously dangerous calls — weaker than Docker’s default
allowlist, documented honestly in `docs/SECURITY.md`.

### Cap resolve order: defaults → drop → add

Matches Docker’s mental model (`--cap-drop ALL --cap-add NET_BIND_SERVICE`).

### Mask paths before pivot

`/dev/null` binds and ro remounts happen on the still-host-visible rootfs tree
so they survive `pivot_root`.

## Milestone 7

### Auto rootless when non-root

No separate binary. `geteuid() != 0` selects user namespaces; `--rootless`
forces the same path as root for testing.

### `newuidmap` when available, single-uid `/proc` map otherwise

Full subordinate ranges need the setuid helpers. Without `uidmap` installed we
still map the calling uid→0 so basic rootless works.

### Overlay `userxattr` then copy-merge

Unprivileged overlay mounts often fail (and whiteout `mknod` needs caps).
Copying layers is slower but correct and matches the milestone fallback.

### Default `--network host` rootless

Creating `petti0`/NAT needs host net admin. User-mode networking is future work.

### AppArmor unprivileged userns

Ubuntu’s `apparmor_restrict_unprivileged_userns` yields `EACCES` on re-exec into
a new userns. We surface an explicit hint and ship a sample profile under
`docs/apparmor/`.

## Milestone 6

### `github.com/vishvananda/netlink` for links/addrs/routes

Beats hand-rolled netlink for an educational runtime; iptables stays behind a
small `NAT` interface so the shell-out is isolated and mockable.

### Pre-allocate IP + bind-mount DNS files

Avoids mutating a shared `--rootfs`. Hosts/resolv live under the container
bundle and are bind-mounted like volumes before `pivot_root`.

### Leave bridge + MASQUERADE after last container

Idempotent first-use setup; per-container veth / DNAT / IPAM entries are what
we tear down on stop/rm. Destroying `petti0` automatically would surprise
operators mid-session.

### DNAT in both PREROUTING and OUTPUT

OUTPUT covers `curl localhost:published` on the host; PREROUTING covers
external clients.

## Milestone 5

### `/proc/self/fd` lowerdir paths

Avoids the PAGE_SIZE mount-option limit when many long layer paths are joined.

### Convert OCI whiteouts at mount time (as root)

`mknod` for overlay whiteouts needs privileges; conversion is cached per layer
via `.petti-overlay-whiteouts`.

### No `Pdeathsig` when detached

Otherwise `petti run -d` exiting would SIGKILL the container.

### Detached logs use raw `*os.File`

`os/exec` only dups real files; a custom writer would spawn a copy goroutine
that dies when the parent exits.

### `-t` is best-effort

Flags are accepted; full host PTY raw-mode / SIGWINCH wiring is still thin.
`exec` uses stdio passthrough via `__exec` + `setns`.

## Milestone 4

### Copy-merge layers then overlay

M4 used copy-merge so `run alpine` worked before overlay landed in M5.

### Verify compressed digest and diffID

Blobs are checked against the manifest digest on download; after gunzip we also
verify the config `rootfs.diff_ids` entry so unpack corruption is caught.

### Preserve `.wh.*` in layer dirs

Whiteouts are applied at merge time (and later by overlay conversion), not
discarded during the initial untar.

## Milestone 3

### cgroup v2 only

No v1 dual-stack. Detect via `cgroup.controllers` and fail clearly on legacy setups.

### `memory.swap.max=0` when `--memory` is set

Makes the memory ceiling hard and OOM tests predictable. (Docker’s swap defaults
are more nuanced; we document the difference.)

### Attach cgroup before sync release

Limits must cover the user process from its first instruction after `exec`.
The child blocks on the sync pipe until the parent writes `cgroup.procs`.
`pids.max` is deferred until `__init` is about to `pivot_root` so Go's
multithreaded re-exec is not SIGABRT'd under a tight `--pids-limit`.

## Milestone 2

### `pivot_root` over `chroot`

Old root is unmounted from the mount namespace, closing classic chroot escapes.
See the comment in `internal/runtime/pivot_linux.go`.

### Docker-like default capability set

Keep a small allow-list (including `NET_BIND_SERVICE`, `MKNOD`, …) rather than
dropping everything. Matches what people expect from `docker run` defaults and
is documented in `docs/SECURITY.md`. `--cap-add` / `--cap-drop` come in M8.

### `no_new_privs` before exec

Prevents the container from regaining privileges through setuid binaries even
if such binaries exist in the rootfs.

### Numeric `--user` only

Username resolution against rootfs `/etc/passwd` is deferred; M2 accepts `uid[:gid]`.

## Milestone 1

### Re-exec instead of in-process `unshare`

Go is multithreaded; `unshare` / `setns` affect only the calling thread and
interact badly with the Go scheduler. Re-execing `/proc/self/exe` with
`Cloneflags` gives a single-threaded start into the new namespaces.

### No `CLONE_NEWNET` yet

Networking is Milestone 6. Isolating net too early would break DNS/`ping`
tests without a veth/bridge story.

### Sync pipe even when the parent has nothing to do

Keeps the handshake identical for later uid_map / cgroup / veth setup so we
do not redesign the init protocol each milestone.

## Milestone 0

### Hand-rolled CLI instead of Cobra

Keeps the dependency surface small (“stdlib first”) and makes the top-level `-v` (version) vs later `run -v` (volume) split explicit in our own parser.

### Module path `github.com/r4r00t-sh/petti`

Matches the public GitHub repository so `go install github.com/r4r00t-sh/petti/cmd/petti@latest` resolves correctly.

### Exit code 125 for internal errors

Mirrors Docker’s convention for runtime-daemon / client internal failures, separate from container exit codes and usage errors.

### Integration job in CI

`.github/workflows/ci.yml` runs `sudo make integration` on Ubuntu with
`busybox-static` installed. Unit/race/lint jobs stay separate so a kernel or
root flake does not hide package-level failures.

## Future ideas

- User-mode networking for rootless bridge-like connectivity
- OCI runtime-spec `config.json` export + conformance subset
- Richer Dockerfile parity (multi-stage, ARG)
