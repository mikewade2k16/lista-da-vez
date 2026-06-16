package metaads

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// handleSync dispara o sync Graph->cache de uma conta de anuncio. adAccountID vem
// do body {"adAccountId":"..."} ou do query ?adAccountId=.
func handleSync(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		var body struct {
			AdAccountID string `json:"adAccountId"`
		}
		// Body e opcional (pode vir vazio quando o id vem no query).
		_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes)).Decode(&body)

		adAccountID := strings.TrimSpace(body.AdAccountID)
		if adAccountID == "" {
			adAccountID = strings.TrimSpace(r.URL.Query().Get("adAccountId"))
		}
		if adAccountID == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "missing_ad_account", "Informe a conta de anuncio (adAccountId).")
			return
		}

		result, err := svc.Sync(r.Context(), accountID, adAccountID)
		if err != nil {
			writeServiceError(w, r, err, "Falha ao sincronizar com a Meta.")
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, result)
	}
}

func handleCampaignsList(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		adAccountID := strings.TrimSpace(r.URL.Query().Get("adAccountId"))
		if adAccountID == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "missing_ad_account", "Informe a conta de anuncio (adAccountId).")
			return
		}
		views, err := svc.ListCampaigns(r.Context(), accountID, adAccountID)
		if err != nil {
			writeServiceError(w, r, err, "Falha ao listar campanhas.")
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, views)
	}
}

func handleInsights(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		q := r.URL.Query()
		adAccountID := strings.TrimSpace(q.Get("adAccountId"))
		if adAccountID == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "missing_ad_account", "Informe a conta de anuncio (adAccountId).")
			return
		}
		rangeKey := strings.TrimSpace(q.Get("range"))
		level := strings.TrimSpace(q.Get("level"))
		points, err := svc.Insights(r.Context(), accountID, adAccountID, rangeKey, level)
		if err != nil {
			writeServiceError(w, r, err, "Falha ao carregar metricas.")
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, points)
	}
}
