# Recording the demo GIF

Record `scripts/demo.sh` with [asciinema](https://asciinema.org/), then convert
to a GIF with [agg](https://github.com/asciinema/agg).

## Prerequisites

- Built binary: `make build`
- Root (demo uses namespaces / cgroups / overlay)
- Network for `petti pull alpine`
- Tools: `asciinema`, `agg` (or `docker run --rm -v "$PWD":/data asciinema/agg`)

```bash
# examples
cargo install --git https://github.com/asciinema/agg
# or: pipx / package manager install of asciinema
```

## Record

Use a tall enough terminal (~80×24 or larger). From the repo root:

```bash
make build
asciinema rec -c 'sudo ./scripts/demo.sh' docs/demo.cast
```

Replay locally:

```bash
asciinema play docs/demo.cast
```

## Convert to GIF

```bash
agg --font-size 14 docs/demo.cast docs/demo.gif
```

Keep `docs/demo.gif` under ~5–8 MB when possible (shorter cast / larger font
reduces size). Commit the GIF when you are happy with it; the cast file is
optional.

## README embed

Uncomment (or replace) the placeholder near the top of `README.md`:

```markdown
![Petti demo](docs/demo.gif)
```

The current placeholder looks like:

```html
<!-- DEMO_GIF: record with scripts/demo.sh (see docs/RECORDING.md) and save as docs/demo.gif
![Petti demo](docs/demo.gif)
-->
```

## Tips

- Run once unrecorded so layers are cached if you want a snappier pull line,
  or record a cold pull for honesty.
- If the OOM step is flaky on your kernel, still leave it in — the exit code
  caption is the teaching moment.
- Do not fabricate a GIF; only commit footage from a real recording.
