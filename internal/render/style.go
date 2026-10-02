// Package render draws the profile cards as self-contained SVG documents.
//
// GitHub serves README images through a sanitiser that strips scripts,
// foreignObject, external fonts, and SMIL, so every card uses system font
// stacks and CSS keyframes only. Each animation declares only its "from"
// state: with prefers-reduced-motion the animation is removed and the element
// rests in its final, fully drawn state.
package render

import (
	"fmt"
	"math"
	"strings"
)

type Theme struct {
	Name                         string
	Bg, Panel, Border, Grid      string
	Text, Muted, Faint           string
	CPU, Memory, Storage         string
	OK, Warn, Crit, Idle, Tunnel string
}

var Dark = Theme{
	Name: "dark",
	Bg:   "#0d1117", Panel: "#161b22", Border: "#30363d", Grid: "#21262d",
	Text: "#e6edf3", Muted: "#8b949e", Faint: "#6e7681",
	CPU: "#39c5bb", Memory: "#a58bfa", Storage: "#e3b341",
	OK: "#3fb950", Warn: "#d29922", Crit: "#f85149", Idle: "#6e7681", Tunnel: "#f0883e",
}

var Light = Theme{
	Name: "light",
	Bg:   "#ffffff", Panel: "#f6f8fa", Border: "#d0d7de", Grid: "#e6eaef",
	Text: "#1f2328", Muted: "#59636e", Faint: "#818b98",
	CPU: "#0e8a82", Memory: "#6e40c9", Storage: "#9a6700",
	OK: "#1a7f37", Warn: "#9a6700", Crit: "#cf222e", Idle: "#818b98", Tunnel: "#bc4c00",
}

const (
	sans = `-apple-system,BlinkMacSystemFont,'Segoe UI','Noto Sans',Helvetica,Arial,sans-serif`
	mono = `ui-monospace,SFMono-Regular,'SF Mono',Menlo,Consolas,'Liberation Mono',monospace`
	// Approximate advance widths used to size pills and chips without font metrics.
	sansAdvance = 0.56
	monoAdvance = 0.61
)

func baseCSS(t Theme) string {
	return fmt.Sprintf(`.s{font-family:%[1]s}.m{font-family:%[2]s}
.tx{fill:%[3]s}.mu{fill:%[4]s}.fa{fill:%[5]s}
.lbl{font-family:%[1]s;font-size:10.5px;font-weight:600;letter-spacing:.14em;fill:%[4]s}
.in{animation:fade .8s ease-out both}
.draw{stroke-dasharray:1;animation:draw 1.6s cubic-bezier(.3,.7,.2,1) both}
.pulse{opacity:0;transform-box:fill-box;transform-origin:center;animation:pulse 2.4s ease-out infinite}
@keyframes fade{from{opacity:0}}
@keyframes draw{from{stroke-dashoffset:1}}
@keyframes pulse{0%%{opacity:.6;transform:scale(1)}100%%{opacity:0;transform:scale(3.2)}}
@media (prefers-reduced-motion:reduce){*{animation:none!important}}`, sans, mono, t.Text, t.Muted, t.Faint)
}

// document wraps a card body with accessible metadata and the shared stylesheet.
func document(w, h int, title, desc string, t Theme, css, defs, body string) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" role="img" aria-labelledby="title desc">`, w, h, w, h)
	fmt.Fprintf(&b, `<title id="title">%s</title><desc id="desc">%s</desc>`, esc(title), esc(desc))
	fmt.Fprintf(&b, `<style>%s%s</style>`, baseCSS(t), css)
	if defs != "" {
		fmt.Fprintf(&b, `<defs>%s</defs>`, defs)
	}
	b.WriteString(body)
	b.WriteString(`</svg>`)
	return []byte(b.String())
}

func card(x, y, w, h float64, t Theme) string {
	return fmt.Sprintf(`<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="12" fill="%s" stroke="%s"/>`, x+.5, y+.5, w-1, h-1, t.Panel, t.Border)
}

// statusDot is a filled dot with an expanding halo when live, or a hollow ring
// when the source does not report telemetry.
func statusDot(cx, cy float64, color string, live bool, delay float64) string {
	if !live {
		return fmt.Sprintf(`<circle cx="%.1f" cy="%.1f" r="3.5" fill="none" stroke="%s" stroke-width="1.5"/>`, cx, cy, color)
	}
	return fmt.Sprintf(`<circle class="pulse" style="animation-delay:%.2fs" cx="%.1f" cy="%.1f" r="4" fill="%s"/><circle cx="%.1f" cy="%.1f" r="4" fill="%s"/>`, delay, cx, cy, color, cx, cy, color)
}

func level(v float64, t Theme) (string, string) {
	switch {
	case v >= 90:
		return "CRITICAL", t.Crit
	case v >= 75:
		return "ELEVATED", t.Warn
	default:
		return "NOMINAL", t.OK
	}
}

func textWidth(s string, size, advance float64) float64 {
	return float64(len([]rune(s))) * size * advance
}

func esc(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;").Replace(s)
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

func pct(v float64) string { return fmt.Sprintf("%.1f%%", v) }

// summary returns current, minimum, mean, and maximum of a series.
func summary(data []float64) (cur, lo, mean, hi float64) {
	if len(data) == 0 {
		return
	}
	lo, hi = math.Inf(1), math.Inf(-1)
	for _, v := range data {
		lo, hi, mean = math.Min(lo, v), math.Max(hi, v), mean+v
	}
	return data[len(data)-1], lo, mean / float64(len(data)), hi
}
