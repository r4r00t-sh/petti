# CLI reference

## Global

| Invocation | Behavior |
|------------|----------|
| `petti -v` / `--v` / `--version` / `version` | **Version** |
| `petti run -v host:dest` | **Volume** bind-mount |

Do not confuse top-level `-v` (version) with `run -v` (volume).

## Image commands

- `petti pull [--platform os/arch] <image>`
- `petti images`
- `petti rmi <image>`
- `petti build [-f Petfile] [-t name:tag] [--no-cache] <context>`
- `petti save -o file.tar.gz <image…>`
- `petti load <file.tar.gz>`

## Compose / system

- `petti compose up [-f petti.yaml]`
- `petti compose down [-f petti.yaml]`
- `petti system prune`
- Global `--debug` for `log/slog` on stderr

## `run`

```text
petti run [flags] <image> [cmd...]
petti run [flags] --rootfs <dir> <cmd...>
```

| Flag | Meaning |
|------|---------|
| `-d`, `--detach` | Background; print container id |
| `-i` / `-t` / `-it` | Keep STDIN / request TTY (TTY is best-effort) |
| `--name` | Name |
| `--rm` | Remove bundle when foreground container exits |
| `-v`, `--volume host:dest[:ro]` | Bind mount |
| `-e` / `-w` / `-u` / `--hostname` / `--entrypoint` | As before |
| `--memory` / `--cpus` / `--pids-limit` | cgroup v2 |
| `--network bridge\|host\|none` | Default `bridge` as root; default `host` when rootless |
| `-p`, `--publish host:container[/proto]` | DNAT publish (bridge only; repeatable) |
| `--rootless` | Force user-namespace mode (automatic when non-root) |
| `--cap-add` / `--cap-drop` | Adjust capabilities (`ALL` supported) |
| `--security-opt seccomp=default\|unconfined` | Seccomp denylist (default) or off |
| `--read-only` | Remount container `/` read-only after pivot |

Image runs use **overlayfs** (shared lower layers + per-container upper).

## Lifecycle

| Command | Meaning |
|---------|---------|
| `petti ps [-a]` | List running (or all) containers |
| `petti stop [-t sec] <id\|name>` | SIGTERM then SIGKILL |
| `petti kill <id\|name>` | SIGKILL |
| `petti rm [-f] <id\|name>` | Remove (force if running) |
| `petti logs [-f] <id\|name>` | Show / follow detached logs |
| `petti inspect <id\|name>` | JSON metadata |
| `petti exec [-it] <id\|name> <cmd…>` | `setns` into container and exec |

### Example

```bash
petti pull alpine
sudo petti run -d --name web -p 8080:80 alpine sleep 3600
sudo petti ps
sudo petti exec web ip addr show eth0
sudo petti exec web wget -qO- https://example.com
sudo petti stop web
sudo petti rm web
```
