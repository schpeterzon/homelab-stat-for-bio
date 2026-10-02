package render

import (
	"fmt"
	"strings"
	"time"

	"github.com/schpeterzon/homelab-stat-for-bio/internal/models"
)

const (
	mapDays  = 7
	mapSlots = 6 // four-hour buckets per day
)

// LoadMap draws CPU and memory as calendar heatmaps: one column per day,
// one row per four-hour slot. Buckets without a sample are outlined, so gaps
// in collection stay visible instead of being smoothed over.
func LoadMap(s models.Status, t Theme) []byte {
	const w, h = 860, 262
	var b strings.Builder
	b.WriteString(card(0, 0, w, h, t))
	b.WriteString(`<text x="24" y="34" class="lbl">LOAD MAP</text>`)
	b.WriteString(`<text x="24" y="54" class="m mu" font-size="11">Four-hour buckets over the last seven days, UTC</text>`)

	// Legend: five steps from idle to saturated.
	lx := 836.0 - 5*16 - 34
	fmt.Fprintf(&b, `<text x="%.0f" y="38" text-anchor="end" class="m fa" font-size="10">0%%</text>`, lx-6)
	for i := 0; i < 5; i++ {
		fmt.Fprintf(&b, `<rect x="%.0f" y="28" width="12" height="12" rx="2.5" fill="%s" fill-opacity="%.2f"/>`, lx+float64(i)*16, t.CPU, step(float64(i)*20+10))
	}
	fmt.Fprintf(&b, `<text x="%.0f" y="38" class="m fa" font-size="10">100%%</text>`, lx+5*16+2)

	end := s.Updated.UTC()
	if end.IsZero() {
		end = time.Now().UTC()
	}
	first := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -(mapDays - 1))
	b.WriteString(heatmap(24, "CPU", t.CPU, buckets(s.History.Time, s.History.CPU, first), first, t, 0))
	b.WriteString(heatmap(448, "MEMORY", t.Memory, buckets(s.History.Time, s.History.Memory, first), first, t, 1))
	return document(w, h, "Seven-day load map", "Heatmap of CPU and memory utilisation in four-hour buckets for the last seven days.", t, "", "", b.String())
}

// buckets keeps the latest sample per (day, slot). -1 marks an empty bucket.
func buckets(times []time.Time, values []float64, first time.Time) [mapDays][mapSlots]float64 {
	var grid [mapDays][mapSlots]float64
	for d := range grid {
		for s := range grid[d] {
			grid[d][s] = -1
		}
	}
	for i, at := range times {
		if i >= len(values) {
			break
		}
		at = at.UTC()
		day := int(at.Sub(first).Hours() / 24)
		if at.Before(first) || day >= mapDays {
			continue
		}
		grid[day][at.Hour()/4] = values[i]
	}
	return grid
}

func heatmap(x0 float64, label, color string, grid [mapDays][mapSlots]float64, first time.Time, t Theme, n int) string {
	const cw, ch, gx, gy, top = 44.0, 18.0, 4.0, 4.0, 96.0
	gridX := x0 + 34
	var b strings.Builder
	fmt.Fprintf(&b, `<rect x="%.0f" y="72" width="8" height="8" rx="2" fill="%s"/><text x="%.0f" y="80" class="lbl">%s</text>`, x0, color, x0+14, label)
	for s := 0; s < mapSlots; s++ {
		fmt.Fprintf(&b, `<text x="%.0f" y="%.1f" class="m fa" font-size="9.5">%02d</text>`, x0, top+float64(s)*(ch+gy)+12.5, s*4)
	}
	for d := 0; d < mapDays; d++ {
		x := gridX + float64(d)*(cw+gx)
		day := first.AddDate(0, 0, d)
		fmt.Fprintf(&b, `<text x="%.1f" y="%.0f" text-anchor="middle" class="m fa" font-size="9.5">%s</text>`, x+cw/2, top+mapSlots*(ch+gy)+12, strings.ToUpper(day.Format("Mon 02")))
		for s := 0; s < mapSlots; s++ {
			y := top + float64(s)*(ch+gy)
			v := grid[d][s]
			if v < 0 {
				fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="%.0f" height="%.0f" rx="3" fill="none" stroke="%s" stroke-dasharray="2 2"/>`, x+.5, y+.5, cw-1, ch-1, t.Border)
				continue
			}
			fmt.Fprintf(&b, `<rect class="in" style="animation-delay:%.2fs" x="%.1f" y="%.1f" width="%.0f" height="%.0f" rx="3" fill="%s" fill-opacity="%.2f"><title>%s %02d:00 UTC: %s</title></rect>`,
				float64(n)*0.2+float64(d)*0.07+float64(s)*0.02, x, y, cw, ch, color, step(v), day.Format("02 Jan"), s*4, pct(v))
		}
	}
	return b.String()
}

func step(v float64) float64 {
	switch {
	case v >= 80:
		return 1
	case v >= 60:
		return .78
	case v >= 40:
		return .56
	case v >= 20:
		return .36
	default:
		return .18
	}
}
