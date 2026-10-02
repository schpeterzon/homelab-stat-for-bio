package render

import (
	"fmt"
	"strings"

	"github.com/schpeterzon/homelab-stat-for-bio/internal/profile"
)

// Stack draws the technology stack as labelled rows of chips.
func Stack(p profile.Profile, t Theme) []byte {
	const w, rowH, top = 860, 40.0, 58.0
	h := int(top + rowH*float64(len(p.Stack)) + 14)
	accents := []string{t.CPU, t.Memory, t.Storage, t.Tunnel, t.OK}
	var b strings.Builder
	b.WriteString(card(0, 0, w, float64(h), t))
	b.WriteString(`<text x="24" y="34" class="lbl">STACK</text>`)
	n := 0
	for r, layer := range p.Stack {
		y := top + float64(r)*rowH
		accent := accents[r%len(accents)]
		fmt.Fprintf(&b, `<rect x="24" y="%.1f" width="3" height="16" rx="1.5" fill="%s"/>`, y+6, accent)
		fmt.Fprintf(&b, `<text x="36" y="%.1f" class="s mu" font-size="10.5" font-weight="600" letter-spacing=".1em">%s</text>`, y+18, esc(strings.ToUpper(layer.Label)))
		if r > 0 {
			fmt.Fprintf(&b, `<path d="M24 %.1f H836" stroke="%s"/>`, y-6, t.Grid)
		}
		x := 172.0
		for _, item := range layer.Items {
			cw := textWidth(item, 12, sansAdvance) + 26
			if x+cw > 836 {
				break
			}
			fmt.Fprintf(&b, `<g class="in" style="animation-delay:%.2fs"><rect x="%.1f" y="%.1f" width="%.1f" height="26" rx="13" fill="%s" stroke="%s"/>`, 0.04*float64(n), x+.5, y+.5, cw, t.Bg, t.Border)
			fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="2.5" fill="%s"/>`, x+12, y+13.5, accent)
			fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" class="s tx" font-size="12">%s</text></g>`, x+20, y+17.5, esc(item))
			x += cw + 8
			n++
		}
	}
	var parts []string
	for _, l := range p.Stack {
		parts = append(parts, l.Label+": "+strings.Join(l.Items, ", "))
	}
	return document(w, h, "Technology stack", strings.Join(parts, ". "), t, "", "", b.String())
}
