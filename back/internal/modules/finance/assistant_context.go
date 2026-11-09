package finance

// assistantLines bounds model context without changing stored rows or authoritative totals.
func assistantLines(lines []Line) []Line {
	if len(lines) > 50 {
		lines = lines[:50]
	}
	out := append([]Line{}, lines...)
	for i := range out {
		out[i].Description = assistantText(out[i].Description, 500)
		out[i].Details = assistantText(out[i].Details, 1000)
		// The aggregate adjustment remains authoritative; history is bounded.
		if len(out[i].Adjustments) > 10 {
			out[i].Adjustments = out[i].Adjustments[:10]
		}
		out[i].Adjustments = append([]Adjustment{}, out[i].Adjustments...)
		for j := range out[i].Adjustments {
			out[i].Adjustments[j].Note = assistantText(out[i].Adjustments[j].Note, 500)
		}
	}
	return out
}

func assistantText(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "…"
}
