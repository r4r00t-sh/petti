# Benchmarks

How Petti’s micro-benchmarks are collected and what they mean.

## Method

Run from a built tree as root:

```bash
make build
sudo ./scripts/bench.sh
```

The script prints environment metadata (CPU, cores, RAM, kernel, Go, Petti
version, UTC date) and a markdown results table.

| Workload | Command |
|----------|---------|
| Petti cold start | `petti run --rm --network none alpine true`, 20 runs after best-effort `drop_caches` |
| Petti warm start | same command, 20 runs, no cache drop |
| Petti pull | `petti rmi alpine` then one timed `petti pull alpine` |
| Docker cold start | `docker run --rm alpine true`, 20 runs (skipped if `docker` is missing) |

Timing is wall-clock milliseconds via `date +%s%3N` around each command.
Reported statistics are **median** and **nearest-rank p95** over the 20 samples
(pull is a single sample).

Override defaults with env vars: `PETTI`, `RUNS`, `IMAGE`, `NETWORK_MODE`.

## What the numbers do **not** mean

- Petti and Docker are **not** feature-equivalent (different networking defaults,
  security profiles, image stores, and daemon vs single-process models).
- Cold start here means “page cache dropped when permitted,” not a fresh VM or
  empty disk cache guaranteed by the kernel.
- Pull time depends on registry RTT, Hub anonymous rate limits, and whether
  layers were already in a shared content store elsewhere.
- A lower millisecond count does not imply production readiness or security.

## Results

**not yet measured**

Paste the markdown table from `sudo ./scripts/bench.sh` below after you run it
on a Linux host you control.

```text
(no measured results in this tree yet)
```
