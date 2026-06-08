package settings

import "strings"

func normalizeEnum(value string, allowed []string, fallback string) string {
	trimmed := strings.TrimSpace(value)
	for _, candidate := range allowed {
		if candidate == trimmed {
			return trimmed
		}
	}

	return fallback
}

func fallbackString(value string, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}

	return trimmed
}

func fallbackCategory(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "Sem categoria"
	}

	return trimmed
}

func maxFloat(value float64, minimum float64) float64 {
	if value < minimum {
		return minimum
	}

	return value
}

func maxInt(value int, minimum int) int {
	if value < minimum {
		return minimum
	}

	return value
}
