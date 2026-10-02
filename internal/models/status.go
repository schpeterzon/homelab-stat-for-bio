package models

import "time"

// Status is the portable snapshot committed to the profile repository.
// Each source has its own section so it can later be exposed and cached independently.
type Status struct {
	Health       string           `json:"health"`
	Scope        string           `json:"scope"` // "cluster" or "host": what the headline metrics describe
	System       SystemStatus     `json:"system"`
	Kubernetes   KubernetesStatus `json:"kubernetes"`
	Docker       DockerStatus     `json:"docker"`
	Proxmox      ProxmoxStatus    `json:"proxmox"`
	Updated      time.Time        `json:"updated"`
	AssetVersion string           `json:"asset_version"`
	Errors       []string         `json:"errors,omitempty"`
	History      History          `json:"history"`
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
	Nodes       int     `json:"nodes"`
	Ready       int     `json:"ready"`
	Pods        int     `json:"pods"`
	Running     int     `json:"running"`
	Deployments int     `json:"deployments"`
	CPU         float64 `json:"cpu,omitempty"`    // cluster CPU %, from metrics-server
	Memory      float64 `json:"memory,omitempty"` // cluster memory %, from metrics-server
}

// DockerStatus aggregates every configured Docker endpoint.
type DockerStatus struct {
	Containers int          `json:"containers"`
	Hosts      []DockerHost `json:"hosts,omitempty"`
}
type DockerHost struct {
	Name       string `json:"name"`
	Containers int    `json:"containers"`
}

// ProxmoxStatus is cluster-wide: CPU and memory are weighted across online
// nodes, storage counts each shared pool once.
type ProxmoxStatus struct {
	Nodes   int           `json:"nodes"`
	Online  int           `json:"online"`
	Guests  int           `json:"guests"`
	Running int           `json:"running"`
	CPU     float64       `json:"cpu"`
	Cores   int           `json:"cores"`
	Memory  float64       `json:"memory"`
	MemTB   float64       `json:"memory_total_tb"`
	Storage Storage       `json:"storage"`
	Hosts   []ProxmoxNode `json:"hosts,omitempty"`
}
type ProxmoxNode struct {
	Name   string  `json:"name"`
	Online bool    `json:"online"`
	CPU    float64 `json:"cpu"`
	Memory float64 `json:"memory"`
	Guests int     `json:"guests"`
	Uptime int64   `json:"uptime_seconds"`
}

// History holds parallel series. Time was added after the first releases, so
// older snapshots may carry values without timestamps; see main.alignTimes.
type History struct {
	Time    []time.Time `json:"time,omitempty"`
	CPU     []float64   `json:"cpu"`
	Memory  []float64   `json:"memory"`
	Storage []float64   `json:"storage"`
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
