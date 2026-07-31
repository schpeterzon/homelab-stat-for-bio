# homelab-agent

A single Go binary that collects local and homelab health metrics, writes a GitHub-profile-ready README, generates local SVG sparklines, and can commit and push only when asked.

## Quick start

1. Install Go 1.22 or newer.
2. Copy `config.example.yaml` to `config.yaml` and set the collectors you use.
3. Run `go run ./cmd/agent` from this repository. Use `--dry-run` first to inspect the snapshot.
4. Set `publish.enabled: true`, or pass `--publish`, only after confirming the generated changes.

The binary uses the locally configured `kubectl` and `docker` CLIs, so it naturally observes the same cluster/context and Docker daemon as the host. Proxmox is optional; its API token secret should be supplied through `PROXMOX_TOKEN_SECRET` rather than committed configuration.

Generated files are `README.md`, `status.json`, and `assets/{cpu,ram,storage}.svg`. Each run retains the latest 30 observations in `status.json` for the sparklines.

## Automation

Build with `go build -o homelab-agent ./cmd/agent`, then schedule `homelab-agent --config /path/to/config.yaml` from systemd or cron. The default configuration never executes Git operations. A publishing run stages only the configured README, JSON snapshot, and assets before committing and pushing the configured branch.
