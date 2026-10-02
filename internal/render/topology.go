package render

import (
	"fmt"
	"sort"
	"strings"

	"github.com/schpeterzon/homelab-stat-for-bio/internal/models"
	"github.com/schpeterzon/homelab-stat-for-bio/internal/profile"
)

type box struct {
	node       profile.Node
	x, y, w, h float64
	col        int
}

// column geometry: x, width. Gutters between columns carry the link routing.
var columns = [4][2]float64{{20, 132}, {204, 170}, {426, 186}, {664, 176}}

const (
	topoTop    = 112.0
	topoBottom = 432.0
	tunnelLane = 82.0 // horizontal channel above every box, used by links that skip a column
	topoW      = 860
	topoH      = 452
)

// Topology draws the infrastructure map. Boxes bound to a collector show live
// values with a pulsing indicator; static boxes show a hollow indicator.
func Topology(s models.Status, p profile.Profile, t Theme) []byte {
	tp := p.Topology
	boxes := map[string]*box{}
	var order []*box
	for c, nodes := range [][]profile.Node{tp.Edge, tp.Hosts, tp.Platforms, tp.Workloads} {
		if len(nodes) == 0 {
			continue
		}
		const gap = 14.0
		h := (topoBottom - topoTop - gap*float64(len(nodes)-1)) / float64(len(nodes))
		if c == 0 && h > 64 {
			h = 64 // edge nodes are small; keep them compact at the top
		}
		for i, n := range nodes {
			bx := &box{node: n, x: columns[c][0], y: topoTop + float64(i)*(h+gap), w: columns[c][1], h: h, col: c}
			if c == 0 {
				bx.y = topoTop + 30 + float64(i)*(h+56)
			}
			boxes[n.ID] = bx
			order = append(order, bx)
		}
	}

	var b strings.Builder
	b.WriteString(card(0, 0, topoW, topoH, t))
	b.WriteString(`<text x="24" y="34" class="lbl">INFRASTRUCTURE MAP</text>`)
	fmt.Fprintf(&b, `<text x="24" y="54" class="m mu" font-size="11">%d hypervisors, %d platforms, %d workload groups</text>`, len(tp.Hosts), len(tp.Platforms), len(tp.Workloads))
	b.WriteString(legend(t))
	for c, title := range []string{"EDGE", "HYPERVISORS", "PLATFORMS", "WORKLOADS"} {
		fmt.Fprintf(&b, `<text x="%.0f" y="%.0f" class="s fa" font-size="9.5" font-weight="600" letter-spacing=".14em">%s</text>`, columns[c][0], topoTop-10, title)
	}
	b.WriteString(links(tp.Links, boxes, t))
	for i, bx := range order {
		b.WriteString(drawBox(bx, s, t, i))
	}
	desc := fmt.Sprintf("Infrastructure map: %s.", describe(tp))
	return document(topoW, topoH, "Infrastructure map", desc, t, `@keyframes flow{to{stroke-dashoffset:-18}}.flow{animation:flow 1.1s linear infinite}`, arrow(t), b.String())
}

func legend(t Theme) string {
	var b strings.Builder
	x := 560.0
	fmt.Fprintf(&b, `<path d="M%.0f 32 h22" stroke="%s" stroke-width="1.6" stroke-dasharray="5 4" class="flow"/><text x="%.0f" y="36" class="m mu" font-size="10">tunnel</text>`, x, t.Tunnel, x+28)
	x += 78
	fmt.Fprintf(&b, `<path d="M%.0f 32 h22" stroke="%s" stroke-width="1.4"/><text x="%.0f" y="36" class="m mu" font-size="10">link</text>`, x, t.Faint, x+28)
	x += 64
	b.WriteString(statusDot(x+4, 32, t.OK, true, 0))
	fmt.Fprintf(&b, `<text x="%.0f" y="36" class="m mu" font-size="10">live</text>`, x+14)
	x += 52
	b.WriteString(statusDot(x+4, 32, t.Idle, false, 0))
	fmt.Fprintf(&b, `<text x="%.0f" y="36" class="m mu" font-size="10">no telemetry</text>`, x+14)
	return b.String()
}

// links routes every link orthogonally through the gutters. Ports are spread
// along each box side so parallel links never share a segment.
func links(ls []profile.Link, boxes map[string]*box, t Theme) string {
	out := map[string][]int{}
	in := map[string][]int{}
	for i, l := range ls {
		out[l.From] = append(out[l.From], i)
		in[l.To] = append(in[l.To], i)
	}
	port := func(group map[string][]int, id string, i int, other func(profile.Link) *box) float64 {
		idx := append([]int(nil), group[id]...)
		sort.SliceStable(idx, func(a, c int) bool { return other(ls[idx[a]]).y < other(ls[idx[c]]).y })
		bx := boxes[id]
		for k, v := range idx {
			if v == i {
				pad := clamp(bx.h*0.22, 10, 26)
				if len(idx) == 1 {
					return bx.y + bx.h/2
				}
				return bx.y + pad + float64(k)*(bx.h-2*pad)/float64(len(idx)-1)
			}
		}
		return bx.y + bx.h/2
	}
	gutterUse := map[int]int{}
	var b strings.Builder
	for i, l := range ls {
		from, to := boxes[l.From], boxes[l.To]
		color, extra := t.Faint, ` stroke-width="1.4"`
		if l.Kind == "tunnel" {
			color, extra = t.Tunnel, ` stroke-width="1.6" stroke-dasharray="5 4" class="flow"`
		}
		var d string
		switch {
		case from.col == to.col:
			// Vertical link inside a column, e.g. Internet to Cloudflare.
			x := from.x + from.w/2
			d = fmt.Sprintf("M%.1f %.1f V%.1f", x, from.y+from.h, to.y-4)
		case to.col == from.col+1:
			y1 := port(out, l.From, i, func(k profile.Link) *box { return boxes[k.To] })
			y2 := port(in, l.To, i, func(k profile.Link) *box { return boxes[k.From] })
			gx := gutter(from.col, gutterUse)
			d = orth(from.x+from.w, y1, gx, y2, to.x-4)
		default:
			// Skip a column by climbing into the channel above all boxes.
			y1 := port(out, l.From, i, func(k profile.Link) *box { return boxes[k.To] })
			y2 := port(in, l.To, i, func(k profile.Link) *box { return boxes[k.From] })
			g1, g2 := gutter(from.col, gutterUse), gutter(to.col-1, gutterUse)
			lane := tunnelLane - float64(gutterUse[-1])*5
			gutterUse[-1]++
			d = fmt.Sprintf("M%.1f %.1f H%.1f Q%.1f %.1f %.1f %.1f V%.1f Q%.1f %.1f %.1f %.1f H%.1f Q%.1f %.1f %.1f %.1f V%.1f Q%.1f %.1f %.1f %.1f H%.1f",
				from.x+from.w, y1, g1-6, g1, y1, g1, y1-6, lane+6, g1, lane, g1+6, lane, g2-6, g2, lane, g2, lane+6, y2-6, g2, y2, g2+6, y2, to.x-4)
		}
		fmt.Fprintf(&b, `<path d="%s" fill="none" stroke="%s"%s marker-end="url(#arrow-%s)"/>`, d, color, extra, kindOf(l))
		if l.Label != "" && from.col != to.col {
			fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" class="m" font-size="9.5" fill="%s">%s</text>`, (columns[1][0]+columns[1][1]/2)-textWidth(l.Label, 9.5, monoAdvance)/2, tunnelLane-6-float64(gutterUse[-1]-1)*5, color, esc(l.Label))
		}
	}
	return b.String()
}

func kindOf(l profile.Link) string {
	if l.Kind == "tunnel" {
		return "tunnel"
	}
	return "link"
}

// gutter returns a distinct x inside the gutter right of column c.
func gutter(c int, use map[int]int) float64 {
	left := columns[c][0] + columns[c][1]
	right := columns[c+1][0]
	n := use[c]
	use[c]++
	mid := (left + right) / 2
	off := float64((n+1)/2) * 5
	if n%2 == 1 {
		off = -off
	}
	return clamp(mid+off, left+10, right-12)
}

// orth draws a horizontal-vertical-horizontal path with rounded corners.
func orth(x1, y1, gx, y2, x2 float64) string {
	if abs(y2-y1) < 1 {
		return fmt.Sprintf("M%.1f %.1f H%.1f", x1, y1, x2)
	}
	r := clamp(abs(y2-y1)/2, 0, 6)
	dir := 1.0
	if y2 < y1 {
		dir = -1
	}
	return fmt.Sprintf("M%.1f %.1f H%.1f Q%.1f %.1f %.1f %.1f V%.1f Q%.1f %.1f %.1f %.1f H%.1f",
		x1, y1, gx-r, gx, y1, gx, y1+dir*r, y2-dir*r, gx, y2, gx+r, y2, x2)
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func arrow(t Theme) string {
	return fmt.Sprintf(`<marker id="arrow-link" viewBox="0 0 8 8" refX="1" refY="4" markerWidth="7" markerHeight="7" orient="auto"><path d="M0 0 L8 4 L0 8 Z" fill="%s"/></marker><marker id="arrow-tunnel" viewBox="0 0 8 8" refX="1" refY="4" markerWidth="7" markerHeight="7" orient="auto"><path d="M0 0 L8 4 L0 8 Z" fill="%s"/></marker>`, t.Faint, t.Tunnel)
}

func drawBox(bx *box, s models.Status, t Theme, i int) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<g class="in" style="animation-delay:%.2fs">`, 0.05*float64(i))
	fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="8" fill="%s" stroke="%s"/>`, bx.x+.5, bx.y+.5, bx.w-1, bx.h-1, t.Bg, t.Border)
	live, value, color := telemetry(bx.node, s, t)
	x, y := bx.x+14, bx.y+22
	if bx.node.Telemetry != "" {
		b.WriteString(statusDot(bx.x+bx.w-14, bx.y+17, color, live, float64(i)*0.3))
	}
	fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" class="s tx" font-size="12.5" font-weight="600">%s</text>`, x, y, esc(bx.node.Label))
	lines := []string{}
	if bx.node.Detail != "" {
		lines = append(lines, bx.node.Detail)
	}
	if bx.node.Specs != "" {
		lines = append(lines, bx.node.Specs)
	}
	if len(bx.node.Items) > 0 {
		lines = append(lines, wrap(bx.node.Items, bx.w-28, 10)...)
	}
	maxLines := int((bx.h - 34 - boolf(value != "")*22) / 14)
	if len(lines) > maxLines {
		lines = lines[:max(maxLines, 0)]
	}
	for k, line := range lines {
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" class="m mu" font-size="10">%s</text>`, x, y+18+float64(k)*14, esc(line))
	}
	if value != "" {
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" class="m" font-size="11" font-weight="600" fill="%s">%s</text>`, x, bx.y+bx.h-14, color, esc(value))
	}
	b.WriteString(`</g>`)
	return b.String()
}

func telemetry(n profile.Node, s models.Status, t Theme) (bool, string, string) {
	switch n.Telemetry {
	case "docker":
		if s.Docker.Containers > 0 {
			if len(s.Docker.Hosts) > 1 {
				return true, fmt.Sprintf("%d containers / %d hosts", s.Docker.Containers, len(s.Docker.Hosts)), t.OK
			}
			return true, fmt.Sprintf("%d containers running", s.Docker.Containers), t.OK
		}
		return false, "no telemetry", t.Idle
	case "kubernetes":
		k := s.Kubernetes
		if k.Nodes > 0 {
			color := t.OK
			if k.Ready < k.Nodes {
				color = t.Warn
			}
			return true, fmt.Sprintf("%d/%d ready / %d pods", k.Ready, k.Nodes, k.Running), color
		}
		return false, "no telemetry", t.Idle
	case "proxmox":
		// Host boxes match their Proxmox node by id or label.
		for _, h := range s.Proxmox.Hosts {
			if h.Name != n.ID && h.Name != n.Label {
				continue
			}
			if !h.Online {
				return false, "offline", t.Crit
			}
			return true, fmt.Sprintf("cpu %.0f%% / mem %.0f%%", h.CPU, h.Memory), t.OK
		}
		return false, "no telemetry", t.Idle
	}
	return false, "", t.Idle
}

// wrap joins items with separators into lines that fit width at a mono size.
func wrap(items []string, width, size float64) []string {
	limit := int(width / (size * monoAdvance))
	var lines []string
	cur := ""
	for _, it := range items {
		next := it
		if cur != "" {
			next = cur + " / " + it
		}
		if len(next) > limit && cur != "" {
			lines = append(lines, cur)
			next = it
		}
		cur = next
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

func boolf(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func describe(tp profile.Topology) string {
	var parts []string
	for _, l := range tp.Links {
		parts = append(parts, l.From+" to "+l.To)
	}
	return strings.Join(parts, ", ")
}
