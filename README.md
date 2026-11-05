# Petti

[![CI](https://github.com/r4r00t-sh/petti/actions/workflows/ci.yml/badge.svg)](https://github.com/r4r00t-sh/petti/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**Petti** (പെട്ടി) means “box” in Malayalam. It is a Docker-like container runtime written in Go from scratch. Use it to learn how Linux containers work by reading a small, end-to-end implementation — not as a production substitute for Docker, runc, or containerd.

> **Security honesty:** Petti is a learning runtime. It is **not** safe for untrusted workloads. See [docs/SECURITY.md](docs/SECURITY.md).

<!-- DEMO_GIF: record with scripts/demo.sh (see docs/RECORDING.md) and save as docs/demo.gif
![Petti demo](docs/demo.gif)
-->

## How it works

`petti pull` fetches OCI layers into a content-addressed store. `petti run` mounts an overlay, re-execs itself as `__init` with `CLONE_NEW*` flags, attaches cgroup v2 limits, optionally sets up a veth on `petti0`, then signals a sync pipe so init can `pivot_root`, drop capabilities, apply seccomp, and `exec` the container command. Rootless mode adds a user namespace and defaults to host networking. Details: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Status

Milestones **0–9** are complete (including build / save-load / compose-lite).

| Milestone | Feature | Done |
|-----------|---------|------|
| 0 | Skeleton, version, CI | yes |
| 1 | Namespaces + re-exec init | yes |
| 2 | pivot_root, caps, env/user | yes |
| 3 | cgroups v2 | yes |
| 4 | Registry + image store | yes |
| 5 | Overlay + lifecycle | yes |
| 6 | Networking | yes |
| 7 | Rootless | yes |
| 8 | Hardening / seccomp | yes |
| 9 | Build / save-load / compose | yes |

## Requirements

- Linux (kernel with cgroups v2; Ubuntu 22.04+ recommended)
- Go (latest stable)
- Root for privileged `run` (bridge networking, overlay, host cgroups); `pull` and rootless `run` work without root

## Build

```bash
make build
./bin/petti -v
```

Version is also available as `petti --v`, `petti --version`, and `petti version` (identical output).

```bash
make test
make vet
make lint   # requires golangci-lint v2
```

```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
```


Install from source (Linux):

```bash
go install github.com/r4r00t-sh/petti/cmd/petti@latest
```

Or build from a clone with `make build` (writes `./bin/petti`).

## Quick start

```bash
make build
./bin/petti pull alpine
./bin/petti images
sudo ./bin/petti run alpine cat /etc/os-release

sudo ./bin/petti run -d --name demo -p 8080:80 alpine sleep 3600
sudo ./bin/petti ps
sudo ./bin/petti exec demo ip addr show eth0
sudo ./bin/petti stop demo
sudo ./bin/petti rm demo
```

`pull` works unprivileged. Privileged `run` needs **root** (bridge/cgroup/overlay).
**Rootless** `run` works as a normal user (`--network host` by default); on Ubuntu
23.10+ you may need `uidmap` and to relax AppArmor userns restrictions — see
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

Legacy `--rootfs` still works for busybox fixtures used in earlier milestones.

### Rootless quick start

```bash
make build
./bin/petti pull alpine
# if exec fails with EACCES on Ubuntu:
#   sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0
#   sudo apt install uidmap
./bin/petti run --rm alpine echo hello
```

> **Warning:** Setting `kernel.apparmor_restrict_unprivileged_userns=0` weakens
> host isolation for all unprivileged user namespaces. Only do this on a
> development machine you control; prefer the sample profile under
> [docs/apparmor/](docs/apparmor/) when you can load it instead.

### Build / save / compose

```bash
./bin/petti pull alpine
# COPY-only builds work without RUN; RUN needs sudo:
sudo ./bin/petti build -t hello -f examples/Petfile examples
sudo ./bin/petti run --rm localhost/hello:latest
./bin/petti save -o /tmp/hello.tar.gz localhost/hello:latest
./bin/petti load /tmp/hello.tar.gz
sudo ./bin/petti compose up -f examples/petti.yaml
sudo ./bin/petti compose down -f examples/petti.yaml
./bin/petti system prune
```

### Integration tests

```bash
sudo make integration
```

## Benchmarks

Results are **not yet measured** in this tree. Method and caveats:
[docs/BENCHMARKS.md](docs/BENCHMARKS.md).

```bash
make build
sudo ./scripts/bench.sh
```

Paste the script’s results table into `docs/BENCHMARKS.md` (and a short summary
here) after a real run — do not invent numbers.

## Why this exists

Containers are Linux namespaces, cgroups, mount tricks, and careful process management. Petti implements those pieces in one codebase so you can follow a pull, an overlay mount, a re-exec init, and a bridge setup without jumping across many repos.

## Limitations

Honest gaps versus Docker / runc (see also [docs/SECURITY.md](docs/SECURITY.md)):

- **Not for untrusted workloads** — shared kernel; seccomp is a small denylist, not Docker’s default allowlist.
- **No AppArmor / SELinux** integration or policy generation.
- **Rootless networking** defaults to `--network host`; there is no user-mode network stack, and `--network bridge` is unsupported without host `CAP_NET_ADMIN`.
- **Registry pulls are anonymous** (Bearer token without credentials). Private registries and Docker Hub authenticated pulls are unsupported; Hub anonymous rate limits still apply.
- **Petfile build** supports a subset (`FROM`, `RUN`, `COPY`, `WORKDIR`, `ENV`, `CMD`/`ENTRYPOINT` and similar config). No multi-stage builds, `ARG`, or build secrets. `RUN` needs root.
- **Compose-lite** (`petti.yaml`) starts/stops a flat service map (`image`, `command`, ports, volumes, env, caps). No networks/depends_on/healthchecks; `restart` is ignored.
- **Linux + cgroups v2 only**; non-Linux builds are stubs.

## Docs

- [Architecture](docs/ARCHITECTURE.md)
- [CLI](docs/CLI.md)
- [Security / threat model](docs/SECURITY.md)
- [Design decisions](docs/DESIGN-DECISIONS.md)

## License

[MIT](LICENSE)
