package tasks

import (
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

func (handler *HTTPHandler) expandRelations(w http.ResponseWriter, r *http.Request, ctx taskHTTPContext) {
	relations, err := handler.service.ExpandRelations(r.Context(), ctx.Access, r.PathValue("taskId"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}

	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"relations": relations})
}
