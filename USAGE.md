# homelab-agent

A single Go binary that concurrently collects local and homelab health metrics, writes a GitHub-profile-ready README, generates local SVG sparklines, and can commit and push only when asked. Collector snapshots are isolated before the coordinator merges them, keeping each source reusable for a future API or dashboard.

## Quick start

1. Install Go 1.22 or newer.
2. Copy `config.example.yaml` to `config.yaml` and set the collectors you use.
3. Run `go run ./cmd/agent` from this repository. Use `--dry-run` first to inspect the snapshot.
4. Set `publish.enabled: true`, or pass `--publish`, only after confirming the generated changes.

The binary uses the locally configured `kubectl` and `docker` CLIs, so it naturally observes the same cluster/context and Docker daemon as the host. Proxmox is optional; its API token secret should be supplied through `PROXMOX_TOKEN_SECRET` rather than committed configuration.

Generated files are `README.md`, `status.json`, and `assets/{cpu,ram,storage}.svg`. The snapshot separates `system`, `kubernetes`, `docker`, and `proxmox` data; system metadata includes hostname, uptime, and kernel. Each run retains the latest 30 observations in `status.json` for sparklines, which show current, minimum, average, and maximum values.

## Automation

Build with `go build -o homelab-agent ./cmd/agent`, then schedule `homelab-agent --config /path/to/config.yaml` from systemd or cron. The default configuration never executes Git operations. A publishing run stages only the configured README, JSON snapshot, and assets before committing and pushing the configured branch.

## GitHub profile publishing

`config.profile.yaml` targets the adjacent `../schpeterzon` profile repository and writes its README, `status.json`, and root-level `cpu.svg`, `ram.svg`, and `storage.svg`. For the supplied `/opt` systemd deployment, use `deploy/server/config.yaml`; it expects the profile repository at `/opt/schpeterzon`. Build the binary, install the supplied user units, and enable the four-hour timer:

```bash
go build -o homelab-agent ./cmd/agent
mkdir -p ~/.config/systemd/user
cp deploy/systemd/homelab-profile.{service,timer} ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable --now homelab-profile.timer
```

The service invokes `--publish`, so ensure the `schpeterzon` repository can push to `origin` without an interactive prompt.
