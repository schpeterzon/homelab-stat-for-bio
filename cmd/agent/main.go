package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/schpeterzon/homelab-stat-for-bio/internal/collectors"
	"github.com/schpeterzon/homelab-stat-for-bio/internal/config"
	gh "github.com/schpeterzon/homelab-stat-for-bio/internal/github"
	"github.com/schpeterzon/homelab-stat-for-bio/internal/models"
	"github.com/schpeterzon/homelab-stat-for-bio/internal/profile"
	"github.com/schpeterzon/homelab-stat-for-bio/internal/readme"
	"github.com/schpeterzon/homelab-stat-for-bio/internal/render"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to configuration file")
	publish := flag.Bool("publish", false, "commit and push generated files")
	dryRun := flag.Bool("dry-run", false, "collect and print status without writing files")
	renderOnly := flag.Bool("render-only", false, "re-render cards and README from the existing status.json without collecting")
	flag.Parse()
	c, err := config.Load(*configPath)
	fatal(err)
	p, err := profile.Load(c.Profile)
	fatal(err)
	if *renderOnly {
		s := readStatus(filepath.Join(c.RepositoryPath, c.StatusFile))
		s.History.Time = alignTimes(s.History.Time, len(s.History.CPU), s.Updated, time.Duration(c.IntervalHours*float64(time.Hour)))
		fatal(render.WriteAll(filepath.Join(c.RepositoryPath, c.AssetsDir), s, p))
		fatal(readme.Render(c.Template, filepath.Join(c.RepositoryPath, c.README), c.AssetsDir, s))
		return
	}
	if !*dryRun && (*publish || c.Publish.Enabled) {
		fatal(gh.Sync(c))
	}
	previous := readStatus(filepath.Join(c.RepositoryPath, c.StatusFile))
	s := collectors.Collect(c)
	s.AssetVersion = strconv.FormatInt(s.Updated.Unix(), 10)
	s.History = appendHistory(previous, s, c)
	if *dryRun {
		fatal(json.NewEncoder(os.Stdout).Encode(s))
		return
	}
	fatal(writeJSON(filepath.Join(c.RepositoryPath, c.StatusFile), s))
	fatal(render.WriteAll(filepath.Join(c.RepositoryPath, c.AssetsDir), s, p))
	fatal(readme.Render(c.Template, filepath.Join(c.RepositoryPath, c.README), c.AssetsDir, s))
	if *publish || c.Publish.Enabled {
		fatal(gh.Publish(c))
	}
	fmt.Printf("updated %s (%s, %s)\n", c.README, s.Health, s.Updated.Format("2006-01-02 15:04 UTC"))
}

func readStatus(path string) models.Status {
	var s models.Status
	b, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(b, &s)
	}
	return s
}

func appendHistory(old, next models.Status, c config.Config) models.History {
	h := old.History
	h.Time = alignTimes(h.Time, len(h.CPU), old.Updated, time.Duration(c.IntervalHours*float64(time.Hour)))
	h.Time = append(h.Time, next.Updated)
	h.CPU = append(h.CPU, next.System.CPU)
	h.Memory = append(h.Memory, next.System.Memory)
	h.Storage = append(h.Storage, percent(next.System.Storage.Used, next.System.Storage.Total))
	n := c.HistoryPoints
	return models.History{Time: tail(h.Time, n), CPU: tail(h.CPU, n), Memory: tail(h.Memory, n), Storage: tail(h.Storage, n)}
}

// alignTimes back-fills timestamps for snapshots written before history
// carried them, assuming one sample per schedule interval ending at last.
func alignTimes(times []time.Time, n int, last time.Time, interval time.Duration) []time.Time {
	if len(times) == n {
		return times
	}
	out := make([]time.Time, n)
	for i := range out {
		out[i] = last.Add(-time.Duration(n-1-i) * interval)
	}
	return out
}

func tail[T any](values []T, n int) []T {
	if len(values) > n {
		return values[len(values)-n:]
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
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
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
