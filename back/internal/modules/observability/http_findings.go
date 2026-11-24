package observability

import (
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// updateFinding: PATCH /findings/{id} — status, responsável, prazo e nota do achado.
func (h *httpHandler) updateFinding(w http.ResponseWriter, r *http.Request) {
	var patch findingPatch
	if err := httpapi.ReadJSON(r, &patch); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_input", "Corpo inválido.")
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	actor := alertActor{UserID: principal.UserID, Name: principal.DisplayName}
	finding, err := h.service.UpdateFinding(r.Context(), r.PathValue("id"), patch, actor)
	if invalid, ok := isInvalidFinding(err); ok {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_finding", invalid.Message)
		return
	}
	respond(w, r, finding, err)
}
