# homelab-stat-for-bio

A single Go binary (standard library only) that collects homelab metrics and renders a GitHub profile README with animated, theme-aware SVG cards:

| Card | Content |
|---|---|
| `header` | Identity, health, and an LED matrix drawn from CPU history |
| `metrics` | Cluster facts, CPU / memory / storage gauges with sparklines |
| `loadmap` | Seven-day heatmap in four-hour buckets |
| `topology` | Infrastructure map with live per-node telemetry |
| `stack` | Technology stack |

Each card is written as `assets/<card>-dark.svg` and `assets/<card>-light.svg` and embedded with `<picture>`, so it follows the viewer's GitHub theme. Animations are CSS-only (GitHub strips scripts and SMIL) and stop under `prefers-reduced-motion`.

## Data sources

| Collector | Source | Provides |
|---|---|---|
| host | `/proc` on the machine running the agent | fallback metrics, uptime, kernel |
| proxmox | `/api2/json/cluster/resources` | cluster CPU (core-weighted), memory, storage (shared pools counted once), guests, per-node state |
| kubernetes | `kubectl` with an explicit kubeconfig | nodes ready, running pods, deployments, usage via metrics-server |
| docker | `docker ps -q`, locally or over SSH | running containers per endpoint |

With `metrics_source: auto` (default) the headline metrics are cluster-wide whenever Proxmox reports; otherwise they describe the local host. A failing enabled collector marks the status **Degraded** and is listed under collector notices.

The agent does not need to run on a hypervisor: every source is reached over the network.

## Setup

### Proxmox (read-only token)

```bash
pveum user add stats@pve
pveum acl modify / --users stats@pve --roles PVEAuditor
pveum user token add stats@pve profile --privsep 0
```

Set `proxmox.url`, `proxmox.token_id: stats@pve!profile`, and export the secret as `PROXMOX_TOKEN_SECRET` (for systemd: an `EnvironmentFile=` readable only by the service user). Use `verify_tls: false` only if the Proxmox certificate is self-signed and you accept that.

### Kubernetes (read-only kubeconfig)

The built-in `view` role cannot list nodes, so bind an extra ClusterRole:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata: { name: profile-stats, namespace: kube-system }
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata: { name: profile-stats }
rules:
  - apiGroups: [""]
    resources: [nodes, pods]
    verbs: [get, list]
  - apiGroups: [apps]
    resources: [deployments]
    verbs: [list]
  - apiGroups: [metrics.k8s.io]
    resources: [nodes]
    verbs: [get, list]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata: { name: profile-stats }
roleRef: { apiGroup: rbac.authorization.k8s.io, kind: ClusterRole, name: profile-stats }
subjects: [{ kind: ServiceAccount, name: profile-stats, namespace: kube-system }]
```

Create a token (`kubectl -n kube-system create token profile-stats --duration=8760h`), build a kubeconfig with it, copy it to the agent host, and set `kubernetes.kubeconfig`. Do not copy the k3s admin kubeconfig. The token expires; renew it before the duration ends.

### Docker endpoints

```yaml
docker:
  hosts: "local, ssh:stats@docker-ct"
```

For SSH endpoints, restrict the key on the remote host so it can only list containers, in `~/.ssh/authorized_keys`:

```
restrict,command="docker ps -q" ssh-ed25519 AAAA... homelab-agent
```

docker-socket-proxy is not recommended here: CVE-2026-78122 allows reading container filesystems through it when `CONTAINERS=1`.

### Profile content

`profile.json` holds the name, tagline, stack, and topology (nodes, links, and `telemetry` bindings: `proxmox`, `docker`, `kubernetes`). Host nodes match Proxmox nodes by `id` or `label`. Keep private addresses out of it; the output is public.

## Usage

```bash
go build -o homelab-agent ./cmd/agent
./homelab-agent --config config.yaml --dry-run      # collect and print JSON
./homelab-agent --config config.yaml                # write README, status.json, cards
./homelab-agent --config config.yaml --render-only  # re-render from existing status.json
./homelab-agent --config config.yaml --publish      # also commit and push
```

`config.example.yaml` documents all options; `config.profile.yaml` targets the adjacent `../schpeterzon` profile repository. History keeps `history_points` samples (default 42, seven days at four-hour intervals).

### Scheduling

```bash
mkdir -p ~/.config/systemd/user
cp deploy/systemd/homelab-profile.{service,timer} ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable --now homelab-profile.timer
```

The service runs with `--publish`, so the profile repository must push to `origin` non-interactively.

## Known limitations

- A stopped agent cannot mark its own output stale; the README keeps the last snapshot.
- If one ZFS pool appears as two Proxmox storages (for example `local` and `local-zfs`), its capacity is counted twice. Set `storage.total_tb` to override.
