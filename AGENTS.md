# AGENTS.md

Guidance for AI coding agents working in this repository. User-facing documentation lives in [README.md](README.md).

## Project

`hcloud-ddns-fw` is a small Go service that detects changes to the host's public IPv4/IPv6 address and then:

- updates **existing** A/AAAA records in Hetzner Cloud DNS (it never creates records), and
- **replaces the full rule set** of configured Hetzner Cloud Firewalls with rules pointing at the new IP.

It is shipped as a Docker image (`ghcr.io/fliprocks/hcloud-ddns-fw`) and is meant to run long-lived with a tiny footprint (< 10 MB RAM, `mem_limit: 32M`).

## Layout

| Path | Purpose |
| --- | --- |
| [main.go](main.go) | Entry point: env validation, `list-firewalls` subcommand, selects the trigger |
| [internal/trigger.go](internal/trigger.go) | Triggers: `RunInterval` (polling loop) and `UpdateURLServer` (HTTP DDNS update URL) |
| [internal/updater.go](internal/updater.go) | `Updater.Update()` – iterates DNS zones/records and firewalls, calls hcloud |
| [internal/pubip/ipchecker.go](internal/pubip/ipchecker.go) | Fetches public IP(s) or accepts them from the update URL, persists last known IP to `IP_STATE_DIR` (default `/tmp/hcloud-ddns-fw`), detects changes |
| [internal/pubip/ipv6.go](internal/pubip/ipv6.go) | Builds an IPv6 address from prefix + prefix ID + interface ID |
| [internal/hcloud_functions/](internal/hcloud_functions/) | Thin wrappers around `hcloud-go/v2` for DNS (`dns.go`) and firewalls (`fw.go`) |
| [internal/config/](internal/config/) | YAML config model (`model.go`) and `slog` logger setup (`logger.go`) |
| [config.yaml](config.yaml) | Example/production config (DNS zones + firewall rules) |
| [compose.yaml](compose.yaml), [examples/](examples/) | Deployment examples |
| [.github/workflows/build_dockerimage.yml](.github/workflows/build_dockerimage.yml) | Multi-arch image build, triggered **only on tag push** |

## Configuration model

- **Secrets and runtime behaviour come from environment variables** (`HCLOUD_TOKEN`, `TRIGGER`, `IP_ADDRESS_SRC`, `IP_MODE`, `UPDATE_INTERVAL`, `UPDATE_URL_*`, `IPv6_*`, `HTTP_TIMEOUT`, `LOGLEVEL`, `IPv4_API_URL`, `IPv6_API_URL`, `IP_STATE_DIR`, `DEVMODE`, …). They are read via `os.Getenv` directly where needed; there is no central env struct.
- **What to update comes from `config.yaml`** (`dns-zones`, `firewalls`). With `DEVMODE=true` the service reads `config-dev.yaml` instead (gitignored).
- When you add or change an env var or config key, update the matching section in [configuration.md](configuration.md) and, if relevant, [compose.yaml](compose.yaml) / [examples/](examples/).

## Triggers

- `TRIGGER=interval` (default): `RunInterval` polls the IP API every `UPDATE_INTERVAL` seconds.
- `TRIGGER=updateUrl`: `UpdateURLServer` listens on `UPDATE_URL_LISTEN` (default `:8080`) and runs an update per authenticated GET request to `/?user=…&pass=…&ip4=…&prefix=…`. With `IP_ADDRESS_SRC=updateUrl` IPv4 comes from the `ip4` query parameter and IPv6 is built from `prefix` + `IPv6_PREFIX_LENGTH` (overrides the length in `prefix`) + optional `IPv6_PREFIX_ID` + `IPv6_INTERFACE_ID`. Other parameters (e.g. `domain`, `ip6`) are deliberately ignored. Responses are DynDNS status words (`good`, `nochg`, `badauth`, `badparam`, `911`). `InitialUpdate` runs one update at startup (with persisted IPs from `IP_STATE_DIR`, `/app/state` in the Docker image, or fetched via API for `IP_ADDRESS_SRC=apiUrl`) so config changes apply without waiting for the router.
- Never log the request URL/query in the update URL server, it contains the password.

Feature ideas are collected in `ideas.md` (gitignored, local only).

## Build and check

Commands assume the dev container (Go module/build caches live under `.cache/` in the repo).

```bash
go build ./...            # compile
go vet ./...              # static checks (currently clean)
go test ./...             # unit tests (offline)
gofmt -l .                # formatting
docker build -t hcloud-ddns-fw:dev .
```

Running locally needs a real `HCLOUD_TOKEN` and **talks to the live Hetzner API**. It modifies real DNS records and overwrites real firewalls. Do not run the service (or `go run .`) against a token without the user's explicit OK. `go run . list-firewalls` is read-only and safe.

Tests must stay offline: never call the hcloud API or ipify. Use `httptest`, point `IPv4_API_URL`/`IPv6_API_URL` and `IP_STATE_DIR` (`t.TempDir()`) at test values, and build the `Updater` with an empty `config.Config` so `Update()` makes no API calls (see [internal/trigger_test.go](internal/trigger_test.go)).

## Conventions

- Go version follows `go.mod` (currently 1.27); the Dockerfile and dev container image must use the same major/minor.
- Logging: use the injected `*slog.Logger` with structured key/value pairs (`logger.Error("msg", "err", err)`). No `fmt.Println` except for CLI output such as `list-firewalls`.
- Errors: return `error` from helpers. Fatal handling (`os.Exit`) currently happens in `main.go` and `updater.go`. Note that config errors detected during updates exit with code `0` so Docker's `restart: always` doesn't hot-loop on a broken config. Keep that behaviour unless asked to change it.
- hcloud calls: create the client with `hcloud.WithToken`, wrap each call in `context.WithTimeout(HTTP_TIMEOUT)`, and wait for returned actions with `client.Action.WaitFor`.
- Existing files are **not** fully `gofmt`-clean (mixed tabs/spaces). Format files you touch with `gofmt -w <file>`, but don't reformat unrelated files in the same change.
- Keep dependencies minimal (currently only `hcloud-go` and `goccy/go-yaml`). When adding one, also update [THIRD_PARTY_LICENSES](THIRD_PARTY_LICENSES).
- Markdown/YAML are formatted with Prettier (2-space indent, see [.prettierrc](.prettierrc)).

## Safety

- Never commit real tokens, `config-dev.yaml`, `compose-dev.yaml`, `.devcontainer/devcontainer.json` or the IP state files (`ipv4`, `ipv6`). They are gitignored; keep it that way.
- Firewall updates replace **all** rules on the target firewall. Treat changes to `fw.go`/`updater.go` firewall logic as high-risk: a bug can lock users out of their servers.
- Pushing a tag publishes a Docker image to GHCR. Don't create or push tags unless explicitly asked.
