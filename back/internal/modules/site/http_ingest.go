package site

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// RegisterIngestRoutes monta os endpoints publicos de webhook ingest.
// Sao publicos (sem JWT) — autenticacao por HMAC SHA-256 do body usando
// o secret cadastrado em site.webhook_sources.
//
//	POST /v1/webhooks/leads/{sourceSlug}
//	POST /v1/webhooks/products/{sourceSlug}
//	POST /v1/webhooks/tracking/{sourceSlug}
//
// Header obrigatorio: X-Signature: sha256=<hex>
func RegisterIngestRoutes(mux *http.ServeMux, svc *Service) {
	mux.HandleFunc("POST /v1/webhooks/leads/{sourceSlug}", handleIngestLead(svc))
	mux.HandleFunc("POST /v1/webhooks/products/{sourceSlug}", handleIngestProduct(svc))
	mux.HandleFunc("POST /v1/webhooks/tracking/{sourceSlug}", handleIngestTracking(svc))
}

func handleIngestLead(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		source, fields, raw, ok := authenticateAndParse(w, r, svc, "leads")
		if !ok {
			return
		}
		lead, err := svc.IngestLead(r.Context(), source, fields, raw)
		if err != nil {
			writeSiteError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusCreated, lead)
	}
}

func handleIngestProduct(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		source, fields, raw, ok := authenticateAndParse(w, r, svc, "products")
		if !ok {
			return
		}
		product, err := svc.IngestProduct(r.Context(), source, fields, raw)
		if err != nil {
			writeSiteError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusCreated, product)
	}
}

// authenticateAndParse resolve sourceSlug → source + secretHash, le o body,
// valida HMAC, e parseia o JSON em map[string]any. Retorna (source, fields,
// rawBody, true) em caso de sucesso; em falha responde HTTP e retorna ok=false.
func authenticateAndParse(w http.ResponseWriter, r *http.Request, svc *Service, expectedType string) (WebhookSourceView, map[string]any, string, bool) {
	slug := strings.ToLower(strings.TrimSpace(r.PathValue("sourceSlug")))
	if slug == "" {
		httpapi.WriteError(w, r, http.StatusBadRequest, "missing_slug", "sourceSlug ausente no path.")
		return WebhookSourceView{}, nil, "", false
	}

	source, secret, err := svc.FindSourceBySlug(r.Context(), slug)
	if err != nil {
		writeSiteError(w, r, err)
		return WebhookSourceView{}, nil, "", false
	}
	if source.EntityType != expectedType {
		httpapi.WriteError(w, r, http.StatusBadRequest, "wrong_entity_type",
			"Esta source está configurada para outro tipo de entidade.")
		return WebhookSourceView{}, nil, "", false
	}

	signature := strings.TrimSpace(r.Header.Get("X-Signature"))
	if signature == "" {
		httpapi.WriteError(w, r, http.StatusUnauthorized, "missing_signature", "Header X-Signature obrigatorio.")
		return WebhookSourceView{}, nil, "", false
	}
	signature = strings.TrimPrefix(signature, "sha256=")

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20)) // 1 MB
	if err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "body_too_large", "Payload acima do limite.")
		return WebhookSourceView{}, nil, "", false
	}

	// Cliente assina body com secret cadastrado (padrao GitHub/Stripe):
	// X-Signature = hex(HMAC_SHA256(body, secret)). Comparamos com hmac.Equal
	// para evitar timing attack.
	expected := computeSignature(body, secret)
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		httpapi.WriteError(w, r, http.StatusUnauthorized, "invalid_signature",
			"X-Signature nao bate com HMAC esperado.")
		return WebhookSourceView{}, nil, "", false
	}

	var fields map[string]any
	if err := json.Unmarshal(body, &fields); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload nao e JSON valido.")
		return WebhookSourceView{}, nil, "", false
	}
	return source, fields, string(body), true
}

func computeSignature(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
