package render

import (
	"fmt"
	"strings"

	"github.com/schpeterzon/homelab-stat-for-bio/internal/models"
	"github.com/schpeterzon/homelab-stat-for-bio/internal/profile"
)

// Header draws the banner: identity on the left, and on the right a rack-style
// LED matrix whose columns are the recorded CPU history, so the artwork is
// the data.
func Header(s models.Status, p profile.Profile, t Theme) []byte {
	const w, h = 860, 220
	var b strings.Builder
	b.WriteString(card(0, 0, w, h, t))
	fmt.Fprintf(&b, `<rect x="1" y="1" width="%d" height="%d" rx="11" fill="url(#dots)"/>`, w-2, h-2)
	fmt.Fprintf(&b, `<rect x="1" y="1" width="%d" height="%d" rx="11" fill="url(#fadeL)"/>`, w-2, h-2)

	kicker := p.Kicker
	if kicker == "" {
		kicker = "HOMELAB"
	}
	fmt.Fprintf(&b, `<g class="in"><text x="40" y="54" class="lbl">%s</text>`, esc(strings.ToUpper(kicker)))
	fmt.Fprintf(&b, `<text x="38" y="104" class="s tx" font-size="46" font-weight="700" letter-spacing="-.5">%s</text>`, esc(p.Name))
	fmt.Fprintf(&b, `<text x="40" y="132" class="m mu" font-size="13">%s</text></g>`, esc(p.Tagline))

	state, color := healthStyle(s.Health, t)
	stamp := s.Updated.Format("02 Jan 2006, 15:04 UTC")
	pillW := 44 + textWidth(state, 11, 0.82) + textWidth(stamp, 11, monoAdvance)
	fmt.Fprintf(&b, `<g class="in" style="animation-delay:.2s"><rect x="40.5" y="152.5" width="%.0f" height="27" rx="13.5" fill="%s" stroke="%s"/>`, pillW, t.Bg, t.Border)
	b.WriteString(statusDot(57, 166, color, s.Health == "Operational" || s.Health == "Healthy", 0))
	fmt.Fprintf(&b, `<text x="70" y="170" class="s" font-size="11" font-weight="700" letter-spacing=".1em" fill="%s">%s</text>`, color, state)
	fmt.Fprintf(&b, `<text x="%.0f" y="170" class="m mu" font-size="11">%s</text></g>`, 78+textWidth(state, 11, 0.82), esc(stamp))

	meta := []string{}
	if px := s.Proxmox; s.Scope == "cluster" {
		meta = append(meta, fmt.Sprintf("proxmox %d nodes / %d cores / %d guests", px.Online, px.Cores, px.Running))
		if s.Docker.Containers > 0 {
			meta = append(meta, fmt.Sprintf("%d containers", s.Docker.Containers))
		}
		if s.Kubernetes.Nodes > 0 {
			meta = append(meta, fmt.Sprintf("%d pods", s.Kubernetes.Running))
		}
		s.System = models.SystemStatus{}
	}
	if s.System.Hostname != "" {
		meta = append(meta, "host "+s.System.Hostname)
	}
	if s.System.Uptime > 0 {
		meta = append(meta, "up "+Uptime(s.System.Uptime))
	}
	if s.System.Kernel != "" {
		meta = append(meta, "kernel "+s.System.Kernel)
	}
	fmt.Fprintf(&b, `<text x="40" y="202" class="m fa in" style="animation-delay:.35s" font-size="10.5">%s</text>`, esc(strings.Join(meta, "  /  ")))

	b.WriteString(ledMatrix(s.History.CPU, t))

	css := `@keyframes sweep{from{transform:translateX(-80px)}to{transform:translateX(400px)}}
.sweep{animation:sweep 7s linear infinite}
@keyframes blink{0%,100%{opacity:1}50%{opacity:.35}}
.peak{animation:blink 3.2s ease-in-out infinite}`
	defs := fmt.Sprintf(`<pattern id="dots" width="18" height="18" patternUnits="userSpaceOnUse"><circle cx="1.5" cy="1.5" r="1" fill="%[1]s"/></pattern>
<linearGradient id="fadeL" x1="0" x2="1"><stop offset="0" stop-color="%[2]s"/><stop offset=".55" stop-color="%[2]s" stop-opacity=".6"/><stop offset="1" stop-color="%[2]s" stop-opacity="0"/></linearGradient>
<linearGradient id="band" x1="0" x2="1"><stop offset="0" stop-color="%[3]s" stop-opacity="0"/><stop offset=".5" stop-color="%[3]s" stop-opacity=".18"/><stop offset="1" stop-color="%[3]s" stop-opacity="0"/></linearGradient>
<clipPath id="matrix"><rect x="466" y="34" width="360" height="150" rx="6"/></clipPath>`, t.Grid, t.Panel, t.CPU)
	desc := fmt.Sprintf("%s. Status %s, updated %s.", p.Tagline, state, stamp)
	return document(w, h, p.Name+" homelab", desc, t, css, defs, b.String())
}

func ledMatrix(history []float64, t Theme) string {
	const cols, rows = 30, 10
	const x0, y0, pitchX, pitchY, led = 470.0, 40.0, 11.8, 13.0, 8.0
	// Right-align the history so the newest sample is the rightmost column.
	values := make([]float64, cols)
	filled := make([]bool, cols)
	if len(history) > cols {
		history = history[len(history)-cols:]
	}
	for i, v := range history {
		values[cols-len(history)+i] = v
		filled[cols-len(history)+i] = true
	}
	var b strings.Builder
	b.WriteString(`<g clip-path="url(#matrix)">`)
	for c := 0; c < cols; c++ {
		lit := 0
		if filled[c] {
			lit = int(clamp(values[c]/100*rows+0.5, 1, rows))
		}
		for r := 0; r < rows; r++ {
			x, y := x0+float64(c)*pitchX, y0+float64(rows-1-r)*pitchY
			if r >= lit {
				fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="%.0f" height="%.0f" rx="1.5" fill="%s"/>`, x, y, led, led, t.Grid)
				continue
			}
			color := t.CPU
			if r >= 9 {
				color = t.Crit
			} else if r >= 7 {
				color = t.Warn
			}
			opacity := 0.35 + 0.65*float64(r+1)/rows
			class, style := "in", fmt.Sprintf("animation-delay:%.2fs", float64(c)*0.03+float64(r)*0.02)
			if r == lit-1 {
				class, style = "peak", fmt.Sprintf("animation-delay:%.2fs", float64(c)*0.11)
			}
			fmt.Fprintf(&b, `<rect class="%s" style="%s" x="%.1f" y="%.1f" width="%.0f" height="%.0f" rx="1.5" fill="%s" fill-opacity="%.2f"/>`, class, style, x, y, led, led, color, opacity)
		}
	}
	b.WriteString(`<rect class="sweep" x="466" y="34" width="80" height="150" fill="url(#band)"/></g>`)
	fmt.Fprintf(&b, `<text x="822" y="190" text-anchor="end" class="m fa" font-size="10">CPU / LAST %d SAMPLES</text>`, len(history))
	return b.String()
}

func healthStyle(health string, t Theme) (string, string) {
	switch health {
	case "Operational", "Healthy": // "Healthy" is written by releases before Degraded existed
		return "OPERATIONAL", t.OK
	case "Degraded":
		return "DEGRADED", t.Warn
	case "":
		return "UNKNOWN", t.Idle
	default:
		return strings.ToUpper(health), t.Crit
	}
}

// Uptime formats seconds as the two most significant units, e.g. "29d 4h".
func Uptime(seconds int64) string {
	d, h, m := seconds/86400, seconds%86400/3600, seconds%3600/60
	switch {
	case d > 0:
		return fmt.Sprintf("%dd %dh", d, h)
	case h > 0:
		return fmt.Sprintf("%dh %dm", h, m)
	default:
		return fmt.Sprintf("%dm", m)
	}
}
