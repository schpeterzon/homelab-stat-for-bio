package render

import (
	"os"
	"path/filepath"

	"github.com/schpeterzon/homelab-stat-for-bio/internal/models"
	"github.com/schpeterzon/homelab-stat-for-bio/internal/profile"
)

// Cards lists every generated card. Each is written as <name>-dark.svg and
// <name>-light.svg so the README can switch with the viewer's GitHub theme.
var Cards = []string{"header", "metrics", "loadmap", "topology", "stack"}

func WriteAll(dir string, s models.Status, p profile.Profile) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	for _, t := range []Theme{Dark, Light} {
		docs := map[string][]byte{
			"header":   Header(s, p, t),
			"metrics":  Metrics(s, t),
			"loadmap":  LoadMap(s, t),
			"topology": Topology(s, p, t),
			"stack":    Stack(p, t),
		}
		for _, name := range Cards {
			if err := os.WriteFile(filepath.Join(dir, name+"-"+t.Name+".svg"), docs[name], 0644); err != nil {
				return err
			}
		}
	}
	return nil
}
