package svg

import ("fmt"; "os"; "path/filepath"; "strings")

func Write(dir string, cpu, memory, storage []float64) error {
	if err := os.MkdirAll(dir, 0755); err != nil { return err }
	for _, metric := range []struct { name, label, color string; data []float64 }{{"cpu", "CPU", "58a6ff", cpu}, {"ram", "Memory", "a371f7", memory}, {"storage", "Storage", "3fb950", storage}} { if err := os.WriteFile(filepath.Join(dir, metric.name+".svg"), card(metric.label, metric.color, metric.data), 0644); err != nil { return err } }
	return nil
}
func card(label, color string, data []float64) []byte { last:=0.0;if len(data)>0{last=data[len(data)-1]}; points:=make([]string,len(data)); for i,v:=range data{x:=12.0;if len(data)>1{x+=float64(i)*220/float64(len(data)-1)};y:=74-v*.42;if y<32{y=32};if y>74{y=74};points[i]=fmt.Sprintf("%.1f,%.1f",x,y)};return []byte(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="244" height="96" viewBox="0 0 244 96" role="img" aria-label="%s %.1f percent"><rect width="244" height="96" rx="10" fill="#161b22"/><text x="12" y="24" fill="#8b949e" font-family="system-ui,sans-serif" font-size="12">%s</text><text x="232" y="24" text-anchor="end" fill="#f0f6fc" font-family="system-ui,sans-serif" font-size="18" font-weight="700">%.1f%%</text><path d="M12 74 H232" stroke="#30363d"/><polyline points="%s" fill="none" stroke="#%s" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"/></svg>`,label,last,label,last,strings.Join(points," "),color)) }
