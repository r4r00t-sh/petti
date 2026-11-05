# Contributing

Thanks for taking an interest in Petti. Keep changes small and honest: this is
an educational runtime, not a Docker replacement.

## Build and unit tests

```bash
make build
go vet ./...
go test ./...
go test -race ./...
make lint   # golangci-lint v2; install with go install ...@latest
```

## Integration tests

Require Linux, root, and a **statically linked** busybox (`busybox-static` on
Debian/Ubuntu):

```bash
sudo make integration
```

That builds `bin/petti`, sets `PETTI_INIT`, and runs `go test -tags=integration`
with `-p 1`. Do not re-exec the `go test` binary as `__init`.

## Commits

Use [Conventional Commits](https://www.conventionalcommits.org/):

- `feat:`, `fix:`, `docs:`, `ci:`, `chore:`, `test:`, `refactor:`
- Imperative subject, ≤72 characters
- Body explains **why** when it is not obvious
- One logical change per commit

## Pull requests

- Describe what you changed and how you tested it (commands + outcomes).
- Do not invent benchmark numbers or claim root-only behavior without running it.
- Prefer linking to `docs/ARCHITECTURE.md` / `docs/SECURITY.md` over restating
  the whole design in the PR body.
