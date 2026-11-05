#!/usr/bin/env bash
# Short, captioned demo for asciinema recording (~30s of wall time with pauses).
# Usage: sudo ./scripts/demo.sh
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PETTI="${PETTI:-$ROOT/bin/petti}"
NAME="petti-demo"

need() {
  [[ -x "$PETTI" ]] || { echo "build first: make build" >&2; exit 1; }
  [[ "$(id -u)" -eq 0 ]] || { echo "run as root: sudo $0" >&2; exit 1; }
}

say() {
  echo
  echo "==> $*"
  sleep 1
}

cleanup() {
  "$PETTI" rm -f "$NAME" >/dev/null 2>&1 || true
}
trap cleanup EXIT

need
cleanup

say "Petti version"
"$PETTI" -v
sleep 1

say "Pull alpine (OCI layers into the local store)"
"$PETTI" pull alpine
sleep 1

say "Run: hostname, process tree, os-release"
"$PETTI" run --rm --network none alpine sh -c 'hostname; echo; ps; echo; cat /etc/os-release'
sleep 2

say "Memory limit: expect OOM / exit 137 under --memory 8m"
set +e
"$PETTI" run --rm --network none --memory 8m alpine \
  sh -c 'dd if=/dev/zero of=/dev/shm/big bs=1M count=64 status=none'
oom_rc=$?
set -e
echo "(container exit code: $oom_rc)"
sleep 2

say "Detached container + petti ps"
"$PETTI" run -d --name "$NAME" --network none alpine sleep 120
"$PETTI" ps
sleep 2

say "Stop and remove"
"$PETTI" stop "$NAME"
"$PETTI" rm "$NAME"
trap - EXIT

echo
echo "==> Done. Record this script with asciinema — see docs/RECORDING.md"
