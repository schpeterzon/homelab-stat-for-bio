package readme

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"text/template"

	"github.com/schpeterzon/homelab-stat-for-bio/internal/models"
	"github.com/schpeterzon/homelab-stat-for-bio/internal/render"
)

// Render executes the README template. assetsDir is the card directory
// relative to the README, used by the picture helper.
func Render(templatePath, outputPath, assetsDir string, status models.Status) error {
	b, err := os.ReadFile(templatePath)
	if err != nil {
		return err
	}
	funcs := template.FuncMap{
		"percent": func(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) },
		"uptime":  render.Uptime,
		// picture emits a theme-aware image: GitHub resolves prefers-color-scheme
		// against the viewer's GitHub theme, not only the operating system.
		"picture": func(name, alt string) string {
			src := func(theme string) string { return path.Join(filepath.ToSlash(assetsDir), name+"-"+theme+".svg") }
			return fmt.Sprintf("<picture>\n  <source media=\"(prefers-color-scheme: dark)\" srcset=\"%s\">\n  <source media=\"(prefers-color-scheme: light)\" srcset=\"%s\">\n  <img alt=\"%s\" src=\"%s\" width=\"100%%\">\n</picture>", src("dark"), src("light"), template.HTMLEscapeString(alt), src("dark"))
		},
	}
	t, err := template.New(filepath.Base(templatePath)).Funcs(funcs).Parse(string(b))
	if err != nil {
		return err
	}
	var out bytes.Buffer
	if err := t.Execute(&out, status); err != nil {
		return err
	}
	return os.WriteFile(outputPath, out.Bytes(), 0644)
}
