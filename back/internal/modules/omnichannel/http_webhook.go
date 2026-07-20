package omnichannel

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// Webhook inbound PUBLICO (sem JWT), provider-agnostico. Rota:
//
//	POST /v1/webhooks/omnichannel/{provider}/{accountSlug}
//
// FORA do gate de modulo por design (nao esta em moduleGatingRules — precedente
// /v1/public/*, /s/{slug}, site/http_ingest.go). Como nao ha middleware, o SERVICE faz o
// equivalente: slug inexistente / conta inativa / modulo desabilitado -> 404 (nunca 403).
//
// Protecoes na ORDEM (spec C3). Deviacao documentada: a resolucao da conta precede a
// verificacao de assinatura, porque a credencial e por instancia (D-A) — sem a conta nao
// ha o que comparar. Mesmo padrao do site/http_ingest.go.

// webhookMaxBody limita o corpo do webhook (protecao 413). Payloads de WhatsApp sao
// pequenos; midia entra por URL/download, nao no corpo do webhook.
const webhookMaxBody = 1 << 20 // 1 MiB

const (
	webhookRateLimit  = 600
	webhookRateWindow = time.Minute
)

// registerWebhookRoutes monta a rota publica do webhook. Chamada de dentro do modulo
// (handle.RegisterRoutes) — precedente cardapio/module.go.
func registerWebhookRoutes(mux *http.ServeMux, svc *InboundService, limiter *rateLimiter) {
	mux.HandleFunc("POST /v1/webhooks/omnichannel/{provider}/{accountSlug}", handleWebhook(svc, limiter))
}

func handleWebhook(svc *InboundService, limiter *rateLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		provider := strings.TrimSpace(r.PathValue("provider"))
		slug := strings.TrimSpace(r.PathValue("accountSlug"))

		// (1) Rate-limit por provider:slug:ip -> 429. Escopo inclui o provider e o slug para
		// nao deixar um par ruidoso derrubar os demais.
		scope := provider + ":" + slug
		if !limiter.allow(scope, clientIP(r), webhookRateLimit, webhookRateWindow) {
			httpapi.WriteError(w, r, http.StatusTooManyRequests, "rate_limited",
				"Muitas requisicoes. Tente novamente em instantes.")
			return
		}

		// (3) Content-Type allowlist -> 415 (checado antes de ler o corpo).
		if !isJSONContentType(r.Header.Get("Content-Type")) {
			httpapi.WriteError(w, r, http.StatusUnsupportedMediaType, "unsupported_media_type",
				"Content-Type deve ser application/json.")
			return
		}

		// (4) Corpo limitado -> 413. MaxBytesReader antes de ler (modelo site/http_ingest.go).
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, webhookMaxBody))
		if err != nil {
			httpapi.WriteError(w, r, http.StatusRequestEntityTooLarge, "payload_too_large",
				"Payload acima do limite.")
			return
		}

		// (5) Conta/slug/modulo -> 404 (nunca 403; nao revelar existencia).
		accountID, err := svc.ResolveAccount(r.Context(), provider, slug)
		if err != nil {
			writeWebhookError(w, r, err)
			return
		}

		// (2) Autenticidade -> 401 (constant-time no adapter; ordem real depois da conta).
		if err := svc.Verify(r.Context(), accountID, provider, r.Header, body); err != nil {
			writeWebhookError(w, r, err)
			return
		}

		// (6) Traduz + deduplica + persiste. Sempre 202 com o status (accepted|duplicate|ignored).
		status, err := svc.Ingest(r.Context(), accountID, provider, r.Header, body)
		if err != nil {
			writeWebhookError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusAccepted, map[string]string{"status": string(status)})
	}
}

// isJSONContentType aceita application/json (com ou sem parametros como charset).
func isJSONContentType(ct string) bool {
	ct = strings.ToLower(strings.TrimSpace(ct))
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	return ct == "application/json"
}

// writeWebhookError mapeia o erro do dominio do webhook para HTTP, SEM ecoar o payload
// (canonico §10). 401 so para autenticidade; 404 para tudo que e "fora de escopo".
func writeWebhookError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrUnauthorized):
		httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized",
			"Assinatura/token do webhook invalido.")
	case errors.Is(err, ErrNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Recurso nao encontrado.")
	case errors.Is(err, ErrInvalidBody):
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Payload invalido.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error",
			"Falha ao processar o webhook.")
	}
}
