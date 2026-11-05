#!/usr/bin/env bash
# Petti micro-benchmarks. Requires a built ./bin/petti and (for run) root.
# Usage: sudo ./scripts/bench.sh
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PETTI="${PETTI:-$ROOT/bin/petti}"
RUNS="${RUNS:-20}"
IMAGE="${IMAGE:-alpine}"
NETWORK_MODE="${NETWORK_MODE:-none}"

die() { echo "error: $*" >&2; exit 1; }

need_petti() {
  [[ -x "$PETTI" ]] || die "petti binary not found at $PETTI (run: make build)"
}

ms_now() {
  # monotonic-ish wall clock in milliseconds
  date +%s%3N
}

median_p95() {
  # reads integers (ms) from stdin, one per line; prints "median p95"
  local -a sorted
  mapfile -t sorted < <(sort -n)
  local n=${#sorted[@]}
  (( n > 0 )) || die "no samples"
  local mid=$((n / 2))
  local med
  if (( n % 2 == 1 )); then
    med=${sorted[$mid]}
  else
    med=$(( (sorted[mid-1] + sorted[mid]) / 2 ))
  fi
  # nearest-rank p95
  local idx=$(( (95 * n + 99) / 100 - 1 ))
  (( idx < 0 )) && idx=0
  (( idx >= n )) && idx=$((n - 1))
  echo "$med ${sorted[$idx]}"
}

time_cmd_ms() {
  local start end
  start=$(ms_now)
  "$@" >/dev/null 2>&1
  end=$(ms_now)
  echo $((end - start))
}

run_samples() {
  local label=$1
  shift
  local -a samples=()
  local i ms
  for ((i = 1; i <= RUNS; i++)); do
    ms=$(time_cmd_ms "$@")
    samples+=("$ms")
    printf '  %s run %2d/%d: %sms\n' "$label" "$i" "$RUNS" "$ms" >&2
  done
  printf '%s\n' "${samples[@]}" | median_p95
}

print_env() {
  echo "## Environment"
  echo
  echo "| Field | Value |"
  echo "|-------|-------|"
  local cpu cores ram kernel gover petver today
  cpu="$(grep -m1 'model name' /proc/cpuinfo 2>/dev/null | cut -d: -f2- | sed 's/^ //' || echo unknown)"
  cores="$(nproc 2>/dev/null || echo unknown)"
  ram="$(awk '/MemTotal/ {printf "%.1f GiB", $2/1024/1024}' /proc/meminfo 2>/dev/null || echo unknown)"
  kernel="$(uname -r)"
  gover="$(go version 2>/dev/null || echo 'go not on PATH')"
  petver="$("$PETTI" -v 2>/dev/null || echo unknown)"
  today="$(date -u +%Y-%m-%dT%H:%MZ)"
  echo "| Date (UTC) | $today |"
  echo "| CPU | $cpu |"
  echo "| Cores | $cores |"
  echo "| RAM | $ram |"
  echo "| Kernel | $kernel |"
  echo "| Go | $gover |"
  echo "| Petti | $petver |"
  echo "| Runs per timing | $RUNS |"
  echo "| Image | $IMAGE |"
  echo "| Petti network | $NETWORK_MODE |"
  echo
}

main() {
  need_petti
  if [[ "$(id -u)" -ne 0 ]]; then
    die "run as root (sudo ./scripts/bench.sh) so petti can create namespaces/cgroups"
  fi

  print_env

  echo "## Ensuring image is present"
  echo
  "$PETTI" pull "$IMAGE" >/dev/null
  echo "pulled/cached: $IMAGE"
  echo

  echo "## Petti cold start (\`petti run --rm --network $NETWORK_MODE $IMAGE true\`)"
  echo
  # Drop page cache best-effort so the first loop is closer to cold I/O.
  sync || true
  if [[ -w /proc/sys/vm/drop_caches ]]; then
    echo 3 >/proc/sys/vm/drop_caches || true
  fi
  read -r cold_med cold_p95 < <(run_samples cold "$PETTI" run --rm --network "$NETWORK_MODE" "$IMAGE" true)

  echo "## Petti warm start (same command, no cache drop)"
  echo
  read -r warm_med warm_p95 < <(run_samples warm "$PETTI" run --rm --network "$NETWORK_MODE" "$IMAGE" true)

  echo "## Petti pull (image removed first)"
  echo
  "$PETTI" rmi "$IMAGE" >/dev/null 2>&1 || true
  # single timed pull
  pull_ms=$(time_cmd_ms "$PETTI" pull "$IMAGE")
  echo "pull: ${pull_ms}ms"
  echo

  local docker_note="skipped (docker not installed)"
  local docker_med="—" docker_p95="—"
  if command -v docker >/dev/null 2>&1; then
    echo "## Docker cold start (\`docker run --rm $IMAGE true\`)"
    echo
    docker pull "$IMAGE" >/dev/null
    sync || true
    if [[ -w /proc/sys/vm/drop_caches ]]; then
      echo 3 >/proc/sys/vm/drop_caches || true
    fi
    read -r docker_med docker_p95 < <(run_samples docker docker run --rm "$IMAGE" true)
    docker_note="measured"
  else
    echo "## Docker cold start"
    echo
    echo "$docker_note"
    echo
  fi

  echo "## Results"
  echo
  echo "| Workload | median (ms) | p95 (ms) |"
  echo "|----------|-------------|----------|"
  echo "| petti cold start | $cold_med | $cold_p95 |"
  echo "| petti warm start | $warm_med | $warm_p95 |"
  echo "| petti pull (uncached) | $pull_ms | — |"
  echo "| docker cold start | $docker_med | $docker_p95 |"
  echo
  echo "Docker status: $docker_note"
}

main "$@"
