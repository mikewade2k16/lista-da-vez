package site

import (
	"context"
	"errors"
	"math"
	"strings"
)

// trackingConversionLabels mapeia event_names conhecidos para rotulos amigaveis.
// Mantem o painel generico: nomes desconhecidos caem no humanize automatico.
var trackingConversionLabels = map[string]string{
	"whatsapp":        "WhatsApp",
	"whatsapp_click":  "WhatsApp",
	"wpp_click":       "WhatsApp",
	"maps_click":      "Mapa clicado",
	"map_click":       "Mapa clicado",
	"maps":            "Mapa clicado",
	"cookie_accept":   "Cookie aceito",
	"cookie_accepted": "Cookie aceito",
	"phone_click":     "Telefone",
	"tel_click":       "Telefone",
	"email_click":     "E-mail",
	"form_submit":     "Formulario enviado",
	"lead_submit":     "Lead enviado",
	"product_click":   "Produto clicado",
	"cta_click":       "CTA clicado",
}

// GetTrackingAnalytics agrega os eventos de tracking e enriquece as conversoes
// com rotulos amigaveis e percentual sobre visitantes unicos da janela.
func (s *Service) GetTrackingAnalytics(ctx context.Context, filter TrackingAnalyticsFilter) (TrackingAnalyticsView, error) {
	if s.tracking == nil {
		return TrackingAnalyticsView{}, errors.New("tracking repository is not configured")
	}

	view, err := s.tracking.Analytics(ctx, filter)
	if err != nil {
		return TrackingAnalyticsView{}, err
	}

	visitors := view.Totals.TotalVisitors
	for i := range view.Conversions {
		conversion := &view.Conversions[i]
		conversion.Label = trackingConversionLabel(conversion.Key)
		if visitors > 0 {
			conversion.PercentOfVisitors = math.Round(float64(conversion.Count)/float64(visitors)*1000) / 10
		}
	}

	return view, nil
}

func trackingConversionLabel(key string) string {
	normalized := strings.ToLower(strings.TrimSpace(key))
	if label, ok := trackingConversionLabels[normalized]; ok {
		return label
	}
	return humanizeEventName(normalized)
}

// humanizeEventName transforma "cookie_accept" / "cookie-accept" em "Cookie accept".
func humanizeEventName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "Evento"
	}
	words := strings.FieldsFunc(value, func(r rune) bool {
		return r == '_' || r == '-' || r == '.' || r == ' '
	})
	for i, word := range words {
		if word == "" {
			continue
		}
		runes := []rune(word)
		runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
}
