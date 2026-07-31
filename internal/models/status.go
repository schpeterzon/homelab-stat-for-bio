package models

import "time"

// Status is the portable snapshot committed to the profile repository.
// Each source has its own section so it can later be exposed and cached independently.
type Status struct {
	Health     string           `json:"health"`
	System     SystemStatus     `json:"system"`
	Kubernetes KubernetesStatus `json:"kubernetes"`
	Docker     DockerStatus     `json:"docker"`
	Proxmox    ProxmoxStatus    `json:"proxmox"`
	Updated    time.Time        `json:"updated"`
	Errors     []string         `json:"errors,omitempty"`
	History    History          `json:"history"`
}

type SystemStatus struct {
	CPU      float64 `json:"cpu"`
	Memory   float64 `json:"memory"`
	Storage  Storage `json:"storage"`
	Hostname string  `json:"hostname"`
	Uptime   int64   `json:"uptime_seconds"`
	Kernel   string  `json:"kernel"`
}

type Storage struct {
	Used       float64 `json:"used_tb"`
	Total      float64 `json:"total_tb"`
	Mountpoint string  `json:"mountpoint,omitempty"`
}
type KubernetesStatus struct {
	Nodes int `json:"nodes"`
	Pods  int `json:"pods"`
}
type DockerStatus struct {
	Containers int `json:"containers"`
}
type ProxmoxStatus struct {
	Nodes   int     `json:"nodes"`
	Memory  float64 `json:"memory"`
	Storage Storage `json:"storage"`
}
type History struct {
	CPU     []float64 `json:"cpu"`
	Memory  []float64 `json:"memory"`
	Storage []float64 `json:"storage"`
}

// Result is an isolated collector result. Applying it is deliberately centralised,
// keeping collectors reusable and free of shared Status mutations.
type Result struct {
	System     *SystemStatus
	Kubernetes *KubernetesStatus
	Docker     *DockerStatus
	Proxmox    *ProxmoxStatus
}

func (r Result) Apply(s *Status) {
	if r.System != nil {
		s.System = *r.System
	}
	if r.Kubernetes != nil {
		s.Kubernetes = *r.Kubernetes
	}
	if r.Docker != nil {
		s.Docker = *r.Docker
	}
	if r.Proxmox != nil {
		s.Proxmox = *r.Proxmox
	}
}
