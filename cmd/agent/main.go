package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/schp/homelab-stat-for-bio/internal/collectors"
	"github.com/schp/homelab-stat-for-bio/internal/config"
	gh "github.com/schp/homelab-stat-for-bio/internal/github"
	"github.com/schp/homelab-stat-for-bio/internal/models"
	"github.com/schp/homelab-stat-for-bio/internal/readme"
	"github.com/schp/homelab-stat-for-bio/internal/svg"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to configuration file")
	publish := flag.Bool("publish", false, "commit and push generated files")
	dryRun := flag.Bool("dry-run", false, "collect and print status without writing files")
	flag.Parse()
	c, err := config.Load(*configPath)
	fatal(err)
	previous := readStatus(filepath.Join(c.RepositoryPath, c.StatusFile))
	s := collectors.Collect(c)
	if !previous.Updated.IsZero() && time.Since(previous.Updated) > 48*time.Hour {
		s.Health = "Stale"
	}
	s.AssetVersion = strconv.FormatInt(s.Updated.Unix(), 10)
	s.History = appendHistory(previous, s)
	if *dryRun {
		fatal(json.NewEncoder(os.Stdout).Encode(s))
		return
	}
	fatal(os.MkdirAll(filepath.Join(c.RepositoryPath, c.AssetsDir), 0755))
	fatal(writeJSON(filepath.Join(c.RepositoryPath, c.StatusFile), s))
	fatal(svg.Write(filepath.Join(c.RepositoryPath, c.AssetsDir), s.AssetVersion, s.History.CPU, s.History.Memory, s.History.Storage))
	fatal(readme.Render(c.Template, filepath.Join(c.RepositoryPath, c.README), s))
	if *publish || c.Publish.Enabled {
		fatal(gh.Publish(c))
	}
	fmt.Printf("updated %s (%s)\n", c.README, s.Updated.Format("2006-01-02 15:04 UTC"))
}
func readStatus(path string) models.Status {
	var s models.Status
	b, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(b, &s)
	}
	return s
}
func appendHistory(old models.Status, next models.Status) models.History {
	return models.History{CPU: appendPoint(old.History.CPU, next.System.CPU), Memory: appendPoint(old.History.Memory, next.System.Memory), Storage: appendPoint(old.History.Storage, percent(next.System.Storage.Used, next.System.Storage.Total))}
}
func appendPoint(values []float64, next float64) []float64 {
	values = append(values, next)
	if len(values) > 30 {
		return values[len(values)-30:]
	}
	return values
}
func percent(used, total float64) float64 {
	if total == 0 {
		return 0
	}
	return used / total * 100
}
func writeJSON(path string, s models.Status) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0644)
}
func fatal(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "homelab-agent:", err)
		os.Exit(1)
	}
}
