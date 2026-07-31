package svg

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// Write renders a self-contained status card for every metric history.
func Write(dir string, cpu, memory, storage []float64) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	for _, metric := range []struct {
		name, label, color string
		data               []float64
	}{{"cpu", "CPU", "58a6ff", cpu}, {"ram", "Memory", "a371f7", memory}, {"storage", "Storage", "3fb950", storage}} {
		if err := os.WriteFile(filepath.Join(dir, metric.name+".svg"), card(metric.label, metric.color, metric.data), 0644); err != nil {
			return err
		}
	}
	return nil
}
func card(label, color string, data []float64) []byte {
	current, min, average, max := summary(data)
	points := make([]string, len(data))
	currentY := 76 - current*.34
	if currentY < 42 {
		currentY = 42
	}
	if currentY > 76 {
		currentY = 76
	}
	for i, value := range data {
		x := 12.0
		if len(data) > 1 {
			x += float64(i) * 220 / float64(len(data)-1)
		}
		y := 76 - value*.34
		if y < 42 {
			y = 42
		}
		if y > 76 {
			y = 76
		}
		points[i] = fmt.Sprintf("%.1f,%.1f", x, y)
	}
	chart := fmt.Sprintf(`<polyline points="%s" fill="none" stroke="#%s" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"/>`, strings.Join(points, " "), color)
	if len(data) == 1 {
		chart = fmt.Sprintf(`<path d="M102 %.1f H142" stroke="#%s" stroke-width="3" stroke-linecap="round"/><circle cx="122" cy="%.1f" r="5" fill="#%s"/>`, currentY, color, currentY, color)
	}
	return []byte(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="244" height="112" viewBox="0 0 244 112" role="img" aria-label="%s %.1f percent; average %.1f, minimum %.1f, maximum %.1f"><rect width="244" height="112" rx="10" fill="#161b22"/><text x="12" y="22" fill="#8b949e" font-family="system-ui,sans-serif" font-size="12">%s</text><text x="232" y="22" text-anchor="end" fill="#f0f6fc" font-family="system-ui,sans-serif" font-size="18" font-weight="700">%.1f%%</text><path d="M12 76 H232" stroke="#30363d"/>%s<text x="12" y="100" fill="#8b949e" font-family="system-ui,sans-serif" font-size="10">min %.1f%%</text><text x="122" y="100" text-anchor="middle" fill="#8b949e" font-family="system-ui,sans-serif" font-size="10">avg %.1f%%</text><text x="232" y="100" text-anchor="end" fill="#8b949e" font-family="system-ui,sans-serif" font-size="10">max %.1f%%</text></svg>`, label, current, average, min, max, label, current, chart, min, average, max))
}
func summary(data []float64) (current, min, average, max float64) {
	if len(data) == 0 {
		return
	}
	min = math.MaxFloat64
	for _, value := range data {
		current = value
		if value < min {
			min = value
		}
		if value > max {
			max = value
		}
		average += value
	}
	average /= float64(len(data))
	return
}
