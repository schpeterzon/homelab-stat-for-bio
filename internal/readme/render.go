package readme

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"text/template"

	"github.com/schpeterzon/homelab-stat-for-bio/internal/models"
)

func Render(templatePath, outputPath string, status models.Status) error {
	b, err := os.ReadFile(templatePath)
	if err != nil {
		return err
	}
	t, err := template.New(filepath.Base(templatePath)).Funcs(template.FuncMap{"percent": func(v float64) string { return trim(v) }}).Parse(string(b))
	if err != nil {
		return err
	}
	var out bytes.Buffer
	if err := t.Execute(&out, status); err != nil {
		return err
	}
	return os.WriteFile(outputPath, out.Bytes(), 0644)
}
func trim(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) }
