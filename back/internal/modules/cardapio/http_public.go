package cardapio

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// RegisterPublicRoutes monta as rotas publicas (sem JWT, sem gating). O CORS
// wildcard de /v1/public/* e tratado no middleware da plataforma. Erros no
// formato do contrato {"error":{code,message}} (mensagens pt-BR) via WriteError.
func RegisterPublicRoutes(mux *http.ServeMux, svc *Service, limiter *rateLimiter) {
	mux.Handle("GET /v1/public/resolve", handleResolve(svc))
	mux.Handle("GET /v1/public/restaurants/{slug}", handlePublicMenu(svc))
	mux.Handle("GET /v1/public/restaurants/{slug}/products/{productSlug}", handlePublicProduct(svc))
	mux.Handle("POST /v1/public/restaurants/{slug}/orders", handlePublicOrder(svc, limiter))
	mux.Handle("POST /v1/public/restaurants/{slug}/events", handlePublicEvent(svc, limiter))
}

// writePublicError traduz erros de dominio para o formato do contrato (pt-BR).
func writePublicError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Restaurante nao encontrado.")
	case errors.Is(err, ErrTypeUnavailable):
		httpapi.WriteError(w, r, http.StatusBadRequest, "type_unavailable", "Forma de recebimento indisponivel neste restaurante.")
	case errors.Is(err, ErrNameRequired):
		httpapi.WriteError(w, r, http.StatusBadRequest, "name_required", "Informe seu nome.")
	case errors.Is(err, ErrPhoneRequired):
		httpapi.WriteError(w, r, http.StatusBadRequest, "phone_required", "Informe o telefone para entrega.")
	case errors.Is(err, ErrEmptyCart):
		httpapi.WriteError(w, r, http.StatusBadRequest, "empty_cart", "Sua sacola esta vazia.")
	case errors.Is(err, ErrMinOrder):
		httpapi.WriteError(w, r, http.StatusBadRequest, "below_min_order", "O valor do pedido esta abaixo do pedido minimo.")
	case errors.Is(err, ErrItemUnavailable):
		httpapi.WriteError(w, r, http.StatusBadRequest, "item_unavailable", "Um item da sacola nao esta mais disponivel. Atualize a pagina e tente de novo.")
	case errors.Is(err, ErrOptionInvalid):
		httpapi.WriteError(w, r, http.StatusBadRequest, "option_invalid", "Uma opcao (tamanho ou adicional) do item e invalida. Limpe a sacola e adicione de novo.")
	case errors.Is(err, ErrValidation):
		httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Pedido invalido. Confira os dados e tente novamente.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro interno. Tente novamente.")
	}
}

func setPublicCache(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "public, max-age=60")
}

func handleResolve(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host := strings.TrimSpace(r.URL.Query().Get("host"))
		if host == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Parametro host obrigatorio.")
			return
		}
		slug, err := svc.Resolve(r.Context(), host)
		if err != nil {
			writePublicError(w, r, err)
			return
		}
		setPublicCache(w)
		httpapi.WriteJSON(w, http.StatusOK, map[string]string{"slug": slug})
	}
}

func handlePublicMenu(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		menu, err := svc.PublicMenu(r.Context(), r.PathValue("slug"))
		if err != nil {
			writePublicError(w, r, err)
			return
		}
		setPublicCache(w)
		httpapi.WriteJSON(w, http.StatusOK, menu)
	}
}

func handlePublicProduct(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		view, err := svc.PublicProduct(r.Context(), r.PathValue("slug"), r.PathValue("productSlug"))
		if err != nil {
			writePublicError(w, r, err)
			return
		}
		setPublicCache(w)
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handlePublicOrder(svc *Service, limiter *rateLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !limiter.allow("orders", clientIP(r), 10, time.Minute) {
			httpapi.WriteError(w, r, http.StatusTooManyRequests, "rate_limited", "Muitos pedidos. Aguarde um instante.")
			return
		}
		var in PublicOrderInput
		if err := httpapi.ReadJSON(r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Pedido invalido.")
			return
		}
		order, err := svc.PlaceOrder(r.Context(), r.PathValue("slug"), in)
		if err != nil {
			writePublicError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusCreated, map[string]any{"order": order})
	}
}

func handlePublicEvent(svc *Service, limiter *rateLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !limiter.allow("events", clientIP(r), 60, time.Minute) {
			httpapi.WriteError(w, r, http.StatusTooManyRequests, "rate_limited", "Muitos eventos. Aguarde um instante.")
			return
		}
		var in PublicEventInput
		if err := httpapi.ReadJSON(r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Evento invalido.")
			return
		}
		if err := svc.RecordEvent(r.Context(), r.PathValue("slug"), in); err != nil {
			writePublicError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusAccepted, map[string]string{"status": "ok"})
	}
}
