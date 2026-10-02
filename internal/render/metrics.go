package render

import (
	"fmt"
	"math"
	"strings"

	"github.com/schpeterzon/homelab-stat-for-bio/internal/models"
)

// Metrics draws a facts strip followed by one gauge panel per resource.
// Sparklines use a fixed 0-100 % scale so panels are directly comparable.
func Metrics(s models.Status, t Theme) []byte {
	const w, h = 860, 300
	var b strings.Builder
	b.WriteString(facts(s, t))

	storagePct := 0.0
	if s.System.Storage.Total > 0 {
		storagePct = s.System.Storage.Used / s.System.Storage.Total * 100
	}
	_, cLo, cAvg, cHi := summary(s.History.CPU)
	_, mLo, mAvg, mHi := summary(s.History.Memory)
	st := s.System.Storage
	panels := []struct {
		label, color string
		value        float64
		history      []float64
		rows         [3][2]string
	}{
		{scoped(s, "CPU"), t.CPU, s.System.CPU, s.History.CPU, [3][2]string{{"AVG", pct(cAvg)}, {"MIN", pct(cLo)}, {"MAX", pct(cHi)}}},
		{scoped(s, "MEMORY"), t.Memory, s.System.Memory, s.History.Memory, [3][2]string{{"AVG", pct(mAvg)}, {"MIN", pct(mLo)}, {"MAX", pct(mHi)}}},
		{scoped(s, "STORAGE"), t.Storage, storagePct, s.History.Storage, [3][2]string{{"USED", tb(st.Used)}, {"FREE", tb(st.Total - st.Used)}, {"SIZE", tb(st.Total)}}},
	}
	const pw, gap, top = 276.0, 16.0, 84.0
	for i, p := range panels {
		x := float64(i) * (pw + gap)
		fmt.Fprintf(&b, `<g class="in" style="animation-delay:%.2fs">`, 0.1+float64(i)*0.12)
		b.WriteString(card(x, top, pw, h-top, t))
		fmt.Fprintf(&b, `<text x="%.0f" y="%.0f" class="lbl">%s</text>`, x+20, top+30, p.label)
		state, color := level(p.value, t)
		fmt.Fprintf(&b, `<text x="%.0f" y="%.0f" text-anchor="end" class="s" font-size="10" font-weight="700" letter-spacing=".1em" fill="%s">%s</text>`, x+pw-20, top+30, color, state)
		b.WriteString(ring(x+62, top+92, p.value, p.color, t, i))
		for r, row := range p.rows {
			y := top + 74 + float64(r)*22
			fmt.Fprintf(&b, `<text x="%.0f" y="%.0f" class="s mu" font-size="10.5" letter-spacing=".08em">%s</text>`, x+128, y, row[0])
			fmt.Fprintf(&b, `<text x="%.0f" y="%.0f" text-anchor="end" class="m tx" font-size="12.5">%s</text>`, x+pw-20, y, row[1])
			if r < 2 {
				fmt.Fprintf(&b, `<path d="M%.0f %.1f H%.0f" stroke="%s"/>`, x+128, y+8.5, x+pw-20, t.Grid)
			}
		}
		b.WriteString(sparkline(x+20, top+150, pw-40, 46, p.history, p.color, t, i))
		b.WriteString(`</g>`)
	}
	desc := fmt.Sprintf("CPU %s, memory %s, storage %s of %s used.", pct(s.System.CPU), pct(s.System.Memory), tb(st.Used), tb(st.Total))
	return document(w, h, "Live resource usage", desc, t, "", gradients(t), b.String())
}

func facts(s models.Status, t Theme) string {
	k8s, k8sLive := "no telemetry", s.Kubernetes.Nodes > 0
	if k8sLive {
		k8s = fmt.Sprintf("%d nodes / %d pods", s.Kubernetes.Ready, s.Kubernetes.Running)
	}
	docker := fmt.Sprintf("%d running", s.Docker.Containers)
	if len(s.Docker.Hosts) > 1 {
		docker = fmt.Sprintf("%d on %d hosts", s.Docker.Containers, len(s.Docker.Hosts))
	}
	type cell struct {
		label, value string
		dim          bool
	}
	cells := []cell{
		{"HOST", orDash(s.System.Hostname), false},
		{"UPTIME", Uptime(s.System.Uptime), false},
		{"KERNEL", orDash(strings.SplitN(s.System.Kernel, "+", 2)[0]), false},
		{"CONTAINERS", docker, false},
		{"KUBERNETES", k8s, !k8sLive},
	}
	if s.Scope == "cluster" {
		px := s.Proxmox
		cells[0] = cell{"HYPERVISORS", fmt.Sprintf("%d/%d online", px.Online, px.Nodes), px.Online < px.Nodes}
		cells[1] = cell{"CAPACITY", fmt.Sprintf("%d cores / %.0f GB", px.Cores, px.MemTB*1024), false}
		cells[2] = cell{"GUESTS", fmt.Sprintf("%d of %d running", px.Running, px.Guests), false}
	}
	var b strings.Builder
	b.WriteString(`<g class="in">`)
	b.WriteString(card(0, 0, 860, 68, t))
	cw := 860.0 / float64(len(cells))
	for i, c := range cells {
		x := float64(i)*cw + 20
		if i > 0 {
			fmt.Fprintf(&b, `<path d="M%.1f 16 V52" stroke="%s"/>`, float64(i)*cw, t.Border)
		}
		fmt.Fprintf(&b, `<text x="%.1f" y="28" class="lbl">%s</text>`, x, c.label)
		class := "tx"
		if c.dim {
			class = "fa"
		}
		fmt.Fprintf(&b, `<text x="%.1f" y="50" class="m %s" font-size="13.5" font-weight="600">%s</text>`, x, class, esc(c.value))
	}
	b.WriteString(`</g>`)
	return b.String()
}

func ring(cx, cy, value float64, color string, t Theme, i int) string {
	const r = 36.0
	c := 2 * math.Pi * r
	offset := c * (1 - clamp(value, 0, 100)/100)
	whole, frac := math.Modf(clamp(value, 0, 999))
	return fmt.Sprintf(`<circle cx="%.1f" cy="%.1f" r="%.0f" fill="none" stroke="%s" stroke-width="8"/>`+
		`<circle cx="%.1f" cy="%.1f" r="%.0f" fill="none" stroke="%s" stroke-width="8" stroke-linecap="round" stroke-dasharray="%.2f" stroke-dashoffset="%.2f" transform="rotate(-90 %.1f %.1f)" style="animation:ring%d 1.4s %.2fs cubic-bezier(.3,.7,.2,1) both"/>`+
		`<style>@keyframes ring%d{from{stroke-dashoffset:%.2f}}</style>`+
		`<text x="%.1f" y="%.1f" text-anchor="middle" class="s tx" font-size="19" font-weight="700">%.0f<tspan font-size="12" class="mu">.%d%%</tspan></text>`,
		cx, cy, r, t.Grid,
		cx, cy, r, color, c, offset, cx, cy, i, 0.2+float64(i)*0.12,
		i, c,
		cx, cy+6.5, whole, int(math.Round(frac*10))%10)
}

func sparkline(x, y, w, h float64, data []float64, color string, t Theme, i int) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<path d="M%.1f %.1f H%.1f" stroke="%s" stroke-dasharray="2 4"/>`, x, y+h/2, x+w, t.Grid)
	fmt.Fprintf(&b, `<path d="M%.1f %.1f H%.1f" stroke="%s"/>`, x, y+h, x+w, t.Border)
	if len(data) == 0 {
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" class="m fa" font-size="10">collecting history</text>`, x, y+h/2-4)
		return b.String()
	}
	pts := make([]string, len(data))
	var lx, ly float64
	for j, v := range data {
		lx = x + w
		if len(data) > 1 {
			lx = x + float64(j)*w/float64(len(data)-1)
		}
		ly = y + h - clamp(v, 0, 100)/100*h
		pts[j] = fmt.Sprintf("%.1f %.1f", lx, ly)
	}
	line := "M" + strings.Join(pts, " L")
	fmt.Fprintf(&b, `<path d="%s L%.1f %.1f L%.1f %.1f Z" fill="url(#area%d)" class="in" style="animation-delay:.9s"/>`, line, x+w, y+h, x, y+h, i)
	fmt.Fprintf(&b, `<path d="%s" pathLength="1" fill="none" stroke="%s" stroke-width="2" stroke-linejoin="round" stroke-linecap="round" class="draw" style="animation-delay:%.2fs"/>`, line, color, 0.3+float64(i)*0.15)
	b.WriteString(statusDot(lx, ly, color, true, float64(i)*0.4))
	fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" class="m fa" font-size="9.5">%d samples</text><text x="%.1f" y="%.1f" text-anchor="end" class="m fa" font-size="9.5">now</text>`, x, y+h+14, len(data), x+w, y+h+14)
	return b.String()
}

func gradients(t Theme) string {
	var b strings.Builder
	for i, c := range []string{t.CPU, t.Memory, t.Storage} {
		fmt.Fprintf(&b, `<linearGradient id="area%d" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="%s" stop-opacity=".28"/><stop offset="1" stop-color="%s" stop-opacity="0"/></linearGradient>`, i, c, c)
	}
	return b.String()
}

func tb(v float64) string {
	if v < 1 {
		return fmt.Sprintf("%.0f GB", v*1024)
	}
	return fmt.Sprintf("%.2f TB", v)
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func scoped(s models.Status, label string) string {
	if s.Scope == "cluster" {
		return "CLUSTER " + label
	}
	return label
}
