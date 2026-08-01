package github

import (
	"fmt"
	"github.com/schpeterzon/homelab-stat-for-bio/internal/config"
	"os/exec"
	"path/filepath"
)

// Sync fast-forwards the target checkout before generated files are written.
// This lets a profile README be edited on GitHub between scheduled runs.
func Sync(c config.Config) error {
	cmd := exec.Command("git", "pull", "--ff-only", c.Publish.Remote, c.Publish.Branch)
	cmd.Dir = c.RepositoryPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git pull --ff-only %s %s: %w: %s", c.Publish.Remote, c.Publish.Branch, err, out)
	}
	return nil
}

func Publish(c config.Config) error {
	generated := []string{c.README, c.StatusFile}
	generated = append(generated, filepath.Join(c.AssetsDir, "*.svg"))
	cmd := exec.Command("git", append([]string{"add"}, generated...)...)
	cmd.Dir = c.RepositoryPath
	if err := cmd.Run(); err != nil {
		return err
	}
	cmd = exec.Command("git", "diff", "--cached", "--quiet")
	cmd.Dir = c.RepositoryPath
	if err := cmd.Run(); err == nil {
		return nil
	} else if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
		return err
	}
	for _, args := range [][]string{{"commit", "-m", c.Publish.Message}, {"push", c.Publish.Remote, "HEAD:" + c.Publish.Branch}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = c.RepositoryPath
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git %v: %w: %s", args, err, out)
		}
	}
	return nil
}
