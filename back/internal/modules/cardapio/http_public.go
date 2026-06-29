package cardapio

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// eventsRateBudget e o orcamento compartilhado do bucket "events" (singular + batch):
// ~600 eventos/min por IP. O batch debita len(events) de uma vez (allowN); o singular
// debita 1 — ambos no MESMO bucket para nao dessincronizar.
const eventsRateBudget = 600

// RegisterPublicRoutes monta as rotas publicas (sem JWT, sem gating). O CORS
// wildcard de /v1/public/* e tratado no middleware da plataforma. Erros no
// formato do contrato {"error":{code,message}} (mensagens pt-BR) via WriteError.
func RegisterPublicRoutes(mux *http.ServeMux, svc *Service, limiter *rateLimiter) {
	mux.Handle("GET /v1/public/resolve", handleResolve(svc))
	mux.Handle("GET /v1/public/restaurants/{slug}", handlePublicMenu(svc))
	mux.Handle("GET /v1/public/restaurants/{slug}/layout", handlePublicLayout(svc))
	mux.Handle("GET /v1/public/restaurants/{slug}/products/{productSlug}", handlePublicProduct(svc))
	mux.Handle("POST /v1/public/restaurants/{slug}/orders", handlePublicOrder(svc, limiter))
	mux.Handle("POST /v1/public/restaurants/{slug}/events", handlePublicEvent(svc, limiter))
	mux.Handle("POST /v1/public/restaurants/{slug}/events/batch", handlePublicEventBatch(svc, limiter))
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
	case errors.Is(err, ErrPaymentInvalid):
		httpapi.WriteError(w, r, http.StatusBadRequest, "payment_invalid", "Forma de pagamento indisponivel neste restaurante.")
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

// handlePublicLayout serve o layout PUBLICADO do site (Opcao B). 404 quando nao
// ha publicado => o site estatico cai no defaultHomeLayout bundlado.
func handlePublicLayout(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		layout, version, err := svc.PublicLayout(r.Context(), r.PathValue("slug"))
		if err != nil {
			writePublicError(w, r, err)
			return
		}
		// no-cache (revalida com ETag a cada carga): publicar no Studio reflete no
		// site num F5 normal, sem esperar TTL. O layout muda quando o dono publica
		// — diferente do menu/produto, que podem ficar 60s em cache.
		w.Header().Set("ETag", "\""+strconv.FormatInt(version, 10)+"\"")
		w.Header().Set("Cache-Control", "no-cache")
		httpapi.WriteJSON(w, http.StatusOK, layout)
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
		// Chave por (tenant, IP): inclui o slug para isolar o orcamento entre
		// restaurantes (um tenant ruidoso nao consome a cota dos vizinhos).
		if !limiter.allow("orders|"+r.PathValue("slug"), clientIP(r), 10, time.Minute) {
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
		// Mesmo bucket "events" do batch (allowN 1 slot) p/ nao dessincronizar o orcamento.
		// Chave por (tenant, IP): inclui o slug para isolar o orcamento entre restaurantes.
		if !limiter.allowN("events|"+r.PathValue("slug"), clientIP(r), 1, eventsRateBudget, time.Minute) {
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

// readBatchJSON le o corpo do lote com decoder dedicado: LimitReader de
// maxBatchBodyBytes (256KB, < 1MB do httpapi.ReadJSON), DisallowUnknownFields e
// rejeita lixo apos o objeto (decoder.More()).
func readBatchJSON(r *http.Request, dst *PublicEventBatchInput) error {
	if r.Body == nil {
		return io.EOF
	}
	defer func() { _ = r.Body.Close() }()
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxBatchBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if decoder.More() {
		return io.ErrUnexpectedEOF
	}
	return nil
}

// handlePublicEventBatch ingere um lote (sendBeacon no unload). 1..maxBatchEvents
// eventos, corpo <= 256KB. Rate limit debita len(events) no bucket compartilhado
// (allowN). Le User-Agent/Referer + hash do IP (CARDAPIO_TELEMETRY_SALT) e delega ao
// service. Resposta 202 {accepted, rejected} (best-effort: nome fora da allowlist ou
// context grande conta em rejected, sem derrubar o lote).
func handlePublicEventBatch(svc *Service, limiter *rateLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in PublicEventBatchInput
		if err := readBatchJSON(r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Lote de eventos invalido.")
			return
		}
		if len(in.Events) < 1 || len(in.Events) > maxBatchEvents {
			httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Lote de eventos invalido.")
			return
		}
		// Mesmo bucket "events|slug" do singular: orcamento por (tenant, IP), isolado por restaurante.
		if !limiter.allowN("events|"+r.PathValue("slug"), clientIP(r), len(in.Events), eventsRateBudget, time.Minute) {
			httpapi.WriteError(w, r, http.StatusTooManyRequests, "rate_limited", "Muitos eventos. Aguarde um instante.")
			return
		}
		userAgent := r.Header.Get("User-Agent")
		referer := r.Header.Get("Referer")
		ipHash := ipHashHex(clientIP(r), svc.cfg.TelemetrySalt)

		accepted, rejected, err := svc.RecordEventBatch(r.Context(), r.PathValue("slug"), in, userAgent, referer, ipHash)
		if err != nil {
			writePublicError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusAccepted, map[string]int{"accepted": accepted, "rejected": rejected})
	}
}
