package site

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

const trackingSignatureTolerance = 5 * time.Minute

type trackingWebhookPayload struct {
	Source   string           `json:"source"`
	EventKey string           `json:"event_key"`
	SentAt   string           `json:"sent_at"`
	BatchID  string           `json:"batch_id"`
	Events   []map[string]any `json:"events"`
}

func handleIngestTracking(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		source, payload, ok := authenticateAndParseTracking(w, r, svc)
		if !ok {
			return
		}

		resp, err := svc.IngestTracking(
			r.Context(),
			source,
			payload.Source,
			payload.SentAt,
			payload.BatchID,
			payload.Events,
		)
		if err != nil {
			writeSiteError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusCreated, resp)
	}
}

func authenticateAndParseTracking(w http.ResponseWriter, r *http.Request, svc *Service) (WebhookSourceView, trackingWebhookPayload, bool) {
	slug := strings.ToLower(strings.TrimSpace(r.PathValue("sourceSlug")))
	if slug == "" {
		httpapi.WriteError(w, r, http.StatusBadRequest, "missing_slug", "sourceSlug ausente no path.")
		return WebhookSourceView{}, trackingWebhookPayload{}, false
	}

	source, secret, err := svc.FindSourceBySlug(r.Context(), slug)
	if err != nil {
		writeSiteError(w, r, err)
		return WebhookSourceView{}, trackingWebhookPayload{}, false
	}
	if source.EntityType != "tracking" {
		httpapi.WriteError(w, r, http.StatusBadRequest, "wrong_entity_type",
			"Esta source esta configurada para outro tipo de entidade.")
		return WebhookSourceView{}, trackingWebhookPayload{}, false
	}

	timestamp := strings.TrimSpace(r.Header.Get("X-Omni-Timestamp"))
	signature := strings.TrimSpace(r.Header.Get("X-Omni-Signature"))
	headerSource := strings.TrimSpace(r.Header.Get("X-Omni-Source"))
	if timestamp == "" || signature == "" || headerSource == "" {
		httpapi.WriteError(w, r, http.StatusUnauthorized, "missing_signature_headers",
			"Headers X-Omni-Timestamp, X-Omni-Signature e X-Omni-Source sao obrigatorios.")
		return WebhookSourceView{}, trackingWebhookPayload{}, false
	}
	if !trackingTimestampIsFresh(timestamp, time.Now()) {
		httpapi.WriteError(w, r, http.StatusUnauthorized, "expired_timestamp",
			"Timestamp do webhook fora da janela permitida.")
		return WebhookSourceView{}, trackingWebhookPayload{}, false
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "body_too_large", "Payload acima do limite.")
		return WebhookSourceView{}, trackingWebhookPayload{}, false
	}

	expected := "sha256=" + computeTimestampedSignature(timestamp, body, secret)
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		httpapi.WriteError(w, r, http.StatusUnauthorized, "invalid_signature",
			"X-Omni-Signature nao bate com HMAC esperado.")
		return WebhookSourceView{}, trackingWebhookPayload{}, false
	}

	var payload trackingWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload nao e JSON valido.")
		return WebhookSourceView{}, trackingWebhookPayload{}, false
	}
	if payload.EventKey != "site_tracking" || len(payload.Events) == 0 {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_payload",
			"Payload deve conter event_key=site_tracking e events[].")
		return WebhookSourceView{}, trackingWebhookPayload{}, false
	}
	if strings.TrimSpace(payload.Source) == "" || strings.TrimSpace(payload.Source) != headerSource {
		httpapi.WriteError(w, r, http.StatusUnauthorized, "invalid_source",
			"X-Omni-Source deve bater com source do payload.")
		return WebhookSourceView{}, trackingWebhookPayload{}, false
	}

	return source, payload, true
}

func computeTimestampedSignature(timestamp string, body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func trackingTimestampIsFresh(value string, now time.Time) bool {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return false
	}
	delta := now.Sub(parsed)
	if delta < 0 {
		delta = -delta
	}
	return delta <= trackingSignatureTolerance
}
