package cardapio

import (
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// Handlers das zonas de entrega (WS-A). Mesmo padrao scopedAccountID das demais
// rotas do painel: accountId de query/header validado contra o Principal; 404
// uniforme fora do escopo. PATCH usa UpdateDeliveryZoneInput (pointer-based).

func handleListZones(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, _, err := scopedAccountID(r, false)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		items, err := svc.ListDeliveryZones(r.Context(), accountID, r.PathValue("id"))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"deliveryZones": items})
	}
}

func handleCreateZone(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, _, err := scopedAccountID(r, false)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		var in DeliveryZoneInput
		if err := httpapi.ReadJSON(r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := svc.CreateDeliveryZone(r.Context(), accountID, r.PathValue("id"), in)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusCreated, view)
	}
}

func handleUpdateZone(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, _, err := scopedAccountID(r, false)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		var in UpdateDeliveryZoneInput
		if err := httpapi.ReadJSON(r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := svc.UpdateDeliveryZone(r.Context(), accountID, r.PathValue("id"), in)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleDeleteZone(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, _, err := scopedAccountID(r, false)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		if err := svc.DeleteDeliveryZone(r.Context(), accountID, r.PathValue("id")); err != nil {
			writeServiceError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
