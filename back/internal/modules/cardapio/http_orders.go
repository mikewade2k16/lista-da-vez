package cardapio

import (
	"net/http"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// RegisterOrderRoutes monta os endpoints de pedidos e eventos do painel.
func RegisterOrderRoutes(mux *http.ServeMux, svc *Service, middleware *auth.Middleware) {
	wrap := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuth(h)
	}

	mux.Handle("GET /v1/cardapio/restaurants/{id}/orders", wrap(handleListOrders(svc)))
	mux.Handle("PATCH /v1/cardapio/orders/{id}", wrap(handleUpdateOrderStatus(svc)))
	mux.Handle("GET /v1/cardapio/restaurants/{id}/events", wrap(handleListEvents(svc)))
}

func handleListOrders(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, _, err := scopedAccountID(r, false)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		page, perPage := parsePage(r)
		status := strings.TrimSpace(r.URL.Query().Get("status"))
		items, total, err := svc.ListOrders(r.Context(), accountID, r.PathValue("id"), status, page, perPage)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{
			"orders": items,
			"total":  total,
		})
	}
}

func handleUpdateOrderStatus(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, _, err := scopedAccountID(r, false)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		var body struct {
			Status string `json:"status"`
		}
		if err := httpapi.ReadJSON(r, &body); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := svc.UpdateOrderStatus(r.Context(), accountID, r.PathValue("id"), strings.TrimSpace(body.Status))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleListEvents(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, _, err := scopedAccountID(r, false)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		page, perPage := parsePage(r)
		items, total, err := svc.ListEvents(r.Context(), accountID, r.PathValue("id"), page, perPage)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{
			"events": items,
			"total":  total,
		})
	}
}
