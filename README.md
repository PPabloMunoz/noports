# noports

A lightweight reverse proxy CLI that maps local development ports to clean, persistent `.localhost` domains with automatic HTTPS.

A Go clone of [Portless](https://github.com/kallegan/portless). Single self-contained binary. Standard library plus [Cobra](https://github.com/spf13/cobra) for CLI commands only.

Instead of juggling ports (`:3000`, `:8080`, `:5173`), run:

```sh
noports run --name api npm start
```

and open `https://api.localhost`.

## Features

- Named `.localhost` routes instead of raw ports
- Automatic HTTPS with a locally generated CA and per-route leaf certs
- HTTP (`:80`) to HTTPS redirect, HTTPS reverse proxy on `:443`
- Background daemon with Unix socket control plane
- Persisted routes in `~/.noports/routes.json`
- Web dashboard at `https://localhost`
- Auto-starts daemon and CA on first `run` / `alias` / `list` / `get`
- `run` sets `PORT` (plus `NOPORTS_PORT`, `NOPORTS_NAME`, `NOPORTS_URL`) in the child process environment, and appends `--port <port>` to the child args only with `--port-arg` (needed by some frameworks, e.g. `astro dev`)

## Requirements

- Go 1.24+ (to build from source)
- macOS or Linux
- Permission to bind `:80`/`:443` and to install a local CA (one-time `trust`, may prompt for sudo)

## Install

```sh
go install github.com/ppablomunoz/noports@latest
```

Or build locally:

```sh
git clone https://github.com/ppablomunoz/noports.git
cd noports
go build -o bin/noports .
./bin/noports --help
```

Dependencies: only `github.com/spf13/cobra` (plus its `pflag`/`mousetrap` transitive deps). Everything else is Go stdlib. See `go.mod`.

## Quickstart

Trust the local CA once:

```sh
noports trust
```

Run an app (name defaults to current directory basename, port defaults to a free OS-assigned loopback port):

```sh
noports run --name web python3 -m http.server
# https://web.localhost
```

With an explicit port:

```sh
noports run --name api --port 3000 npm start
```

For frameworks that need a `--port` CLI flag instead of the `PORT` env var (e.g. Astro):

```sh
noports run --name web --port-arg astro dev
# https://web.localhost
```

`noports` flags must come before the command; everything after it is passed to the child verbatim. `--` works as an explicit separator but is optional.

Map an already-running service:

```sh
noports alias api 3000
# https://api.localhost
```

List and inspect routes:

```sh
noports list
noports list --json
noports get api
noports get api --json
```

Check daemon health and logs:

```sh
noports status
noports logs
noports logs -n 100 -f
```

Open the dashboard:

```text
https://localhost
```

## Commands

| Command | Usage |
| --- | --- |
| `run` | `noports run [--name <name>] [--port <port>] [--port-arg] [--wait <duration>] [--] <cmd> [args...]` — prints `Serving at https://<name>.localhost` once registered (after the backend accepts TCP when `--wait` is set) |
| `alias` | `noports alias <name> <port>` — prints `Alias added: https://<name>.localhost` |
| `alias --remove` | `noports alias --remove <name>` / `noports alias -r <name>` |
| `get` | `noports get <name> [--json]` — prints `https://<name>.localhost` |
| `list` | `noports list [--json]` — prints `NAME / HOST / PORT / PID` table, or a "no routes" hint when empty |
| `status` | `noports status` — daemon state (pid, route count, socket, log path) |
| `logs` | `noports logs [-n <lines>] [-f]` — tail/follow `~/.noports/daemon.log` |
| `proxy` | `noports proxy start` / `noports proxy stop` |
| `daemon` | `noports daemon` — normally auto-started, runs redirect + proxy + socket |
| `trust` | `noports trust` — install local CA into system trust store |
| `clean` | `noports clean` — stop daemon, uninstall CA, delete certs and routes |

Notes:

- `run` requires at least one arg (the command to execute). It sets `PORT`, `NOPORTS_PORT`, `NOPORTS_NAME`, and `NOPORTS_URL` (`https://<name>.localhost`) in the child environment, replacing any existing values. It appends `--port <port>` to the child args only when `--port-arg` is passed. The route is removed automatically when the child exits or on `Ctrl-C`.
- `run --wait <duration>` (e.g. `--wait 10s`) polls until the backend accepts TCP on its port before reporting ready; on timeout the child is interrupted and the route removed. Without it, the URL prints as soon as the route is registered even if the backend is still starting (first request may 502 with a hint).
- `get <name>` accepts the bare name (`api`), the daemon resolves it as `api.localhost`.
- `list --json` prints the routes array (`[]` when empty, for scripting); `get --json` prints the route object.
- Most commands call `EnsureProxy` first, so the daemon and CA are created automatically if missing.
- `clean` prompts for confirmation and removes the CA, `~/.noports/certs`, and `~/.noports/routes.json`.

Shell completion is available via Cobra:

```sh
noports completion bash|zsh|fish|powershell
```

## How it works

```text
CLI (run/alias/list/get)
  -> Unix socket (/tmp/noports.sock)
    -> daemon
      -> registry (in-memory + routes.json)
      -> HTTPS reverse proxy (:443, SNI cert selection)
      -> HTTP redirect (:80 -> https://)
      -> dashboard handler (https://localhost)
```

- `internal/app`: daemon lifecycle, PID file, log file, servers, socket accept loop.
- `internal/proxy`: `:80` redirect server and `:443` reverse proxy (`httputil.NewSingleHostReverseProxy`). Dashboard serves `localhost` and loopback IPs; unreachable backends get a `502` naming the expected `localhost:<port>`.
- `internal/registry`: route table (`hostname -> port, PID`), persisted as JSON.
- `internal/pki`: local ECDSA P-256 CA + per-hostname leaf certs, OS trust store integration. 3-month leafs auto-renew under 30 days of expiry; the 10-year CA rotates under 90 days (stale leafs are dropped and re-issued).
- `internal/ipc`: JSON `Request`/`Response` protocol over the Unix socket (`alias_add`, `alias_remove`, `get`, `list`).
- `internal/client`: CLI-side socket dialing, daemon auto-start with readiness pipe, free-port selection, TCP readiness wait, cleanup.
- `web/index.html.tmpl`: dashboard template, embedded via `go:embed` in `main.go`.
- `cmd/`: one file per Cobra command.

## Files and configuration

| Path | Purpose |
| --- | --- |
| `/tmp/noports.sock` | Daemon control socket (`$NOPORTS_SOCKET` overrides) |
| `~/.noports/routes.json` | Persisted routes |
| `~/.noports/daemon.pid` | Daemon PID file |
| `~/.noports/daemon.log` | Daemon log |
| `~/.noports/certs/` | Local CA + leaf certs |

## Development

```sh
go build ./...
go vet ./...
go build -o bin/noports .
```

`vendor/` is checked in via `go mod vendor`. `bin/` is gitignored.

Project layout:

```text
main.go          # embeds web/index.html.tmpl, calls cmd.Execute()
cmd/             # cobra commands: run, alias, get, list, status, logs, proxy, daemon, trust, clean
internal/app/    # daemon Run()
internal/proxy/  # redirect + reverse proxy servers
internal/registry/ # route store + persistence
internal/pki/    # CA, leaf certs, trust store (darwin/linux/other)
internal/ipc/    # socket listener, handler, protocol
internal/client/ # CLI helpers: ensure/start/stop daemon, socket I/O
internal/paths/  # socket path, ~/.noports paths
web/             # dashboard HTML template
```

## Uninstall

```sh
noports clean
rm -rf ~/.noports
rm "$(which noports)"
```

## License

MIT — see `LICENSE`.
