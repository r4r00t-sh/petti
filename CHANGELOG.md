# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Benchmark harness (`scripts/bench.sh`) and methodology docs
- GitHub Release workflow for `v*` tags and `make release-local`

### Changed

- Go module path is `github.com/r4r00t-sh/petti`
- README cleanup (badges, limitations, how-it-works, AppArmor warning)

## [0.1.0] - 2026-11-04

Initial public snapshot covering milestones 0–9:

### Added

- CLI skeleton, version flags, and CI (milestone 0)
- Namespace re-exec `__init`, mounts, pivot_root, capabilities, env/user (1–2)
- cgroups v2 memory / CPU / PIDs limits (3)
- OCI registry pull, content-addressed image store (4)
- overlayfs rootfs, container lifecycle (`run`/`ps`/`stop`/`rm`/`logs`/`exec`) (5)
- Bridge networking (`petti0`), port publish, host/none modes (6)
- Rootless user namespaces and overlay fallbacks (7)
- Seccomp denylist, masked proc/sys paths, `--read-only` (8)
- Petfile `build`, `save`/`load`, compose-lite, `system prune` (9)

[Unreleased]: https://github.com/r4r00t-sh/petti/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/r4r00t-sh/petti/releases/tag/v0.1.0
