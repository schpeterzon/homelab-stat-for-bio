        ┌──────────────────────────────┐
        │ homelab-agent                │
        │                              │
        │ Data Collectors              │
        │ • Kubernetes                 │
        │ • Docker                     │
        │ • Proxmox                    │
        │ • Linux                      │
        │ • SMART                      │
        │                              │
        │ Generated files              │
        │ • status.json                │
        │ • README.md                  │
        │ • cpu.svg                    │
        │ • ram.svg                    │
        │ • storage.svg                │
        └──────────────┬───────────────┘
                       │
                  git push
                       │
                       ▼
              GitHub Profile Repo Update


homelab-agent/
│
├── cmd/
│   └── agent/
│       └── main.go
│
├── internal/
│   ├── collectors/
│   │      kubernetes.go
│   │      docker.go
│   │      system.go
│   │      proxmox.go
│   │      storage.go
│   │
│   ├── github/
│   │      commit.go
│   │
│   ├── readme/
│   │      template.go
│   │
│   ├── svg/
│   │      cpu.go
│   │      ram.go
│   │      storage.go
│   │
│   └── models/
│          status.go
│
├── templates/
│      README.md.tmpl
│
├── output/
│
├── go.mod
└── config.yaml