// Package profile holds the hand-maintained description of the homelab:
// identity, technology stack, and the infrastructure topology drawn on the map.
// Live values are overlaid at render time; nothing here is collected.
package profile

import (
	"encoding/json"
	"fmt"
	"os"
)

type Profile struct {
	Name     string   `json:"name"`
	Kicker   string   `json:"kicker"`
	Tagline  string   `json:"tagline"`
	Stack    []Layer  `json:"stack"`
	Topology Topology `json:"topology"`
}

// Layer is one row of the technology stack card.
type Layer struct {
	Label string   `json:"label"`
	Items []string `json:"items"`
}

// Topology is laid out in four columns: edge, hosts, platforms, workloads.
// Links connect node ids across any columns; their kind selects the line style.
type Topology struct {
	Edge      []Node `json:"edge"`
	Hosts     []Node `json:"hosts"`
	Platforms []Node `json:"platforms"`
	Workloads []Node `json:"workloads"`
	Links     []Link `json:"links"`
}

type Node struct {
	ID     string   `json:"id"`
	Label  string   `json:"label"`
	Detail string   `json:"detail"`
	Specs  string   `json:"specs"`
	Items  []string `json:"items"`
	// Telemetry binds a node to a collector ("docker", "kubernetes", "proxmox")
	// so the map can show live state instead of a static description.
	Telemetry string `json:"telemetry"`
}

type Link struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Kind  string `json:"kind"` // "link" (default) or "tunnel"
	Label string `json:"label"`
}

func Load(path string) (Profile, error) {
	var p Profile
	b, err := os.ReadFile(path)
	if err != nil {
		return p, err
	}
	if err := json.Unmarshal(b, &p); err != nil {
		return p, fmt.Errorf("%s: %w", path, err)
	}
	return p, p.validate()
}

func (p Profile) validate() error {
	ids := map[string]bool{}
	for _, column := range [][]Node{p.Topology.Edge, p.Topology.Hosts, p.Topology.Platforms, p.Topology.Workloads} {
		for _, n := range column {
			if n.ID == "" {
				return fmt.Errorf("topology node %q has no id", n.Label)
			}
			if ids[n.ID] {
				return fmt.Errorf("duplicate topology id %q", n.ID)
			}
			ids[n.ID] = true
		}
	}
	for _, l := range p.Topology.Links {
		if !ids[l.From] || !ids[l.To] {
			return fmt.Errorf("link %s -> %s references an unknown node", l.From, l.To)
		}
	}
	return nil
}
