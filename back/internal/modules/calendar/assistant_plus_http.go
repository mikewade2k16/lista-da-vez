package calendar

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// branch handlers receive the authenticated scope from the existing transport.
func writePlusError(w http.ResponseWriter, r *http.Request, e error) {
	status, code := assistantPlusStatus(e)
	httpapi.WriteError(w, r, status, code, "Não foi possível concluir no contexto atual. Confira os dados e permissões.")
}
func (s *Service) handlePlusHistory(w http.ResponseWriter, r *http.Request, id string) {
	if !strings.HasPrefix(r.URL.Path, "/v1/assistant/") {
		writePlusError(w, r, plusChatError("invalid_input", 400))
		return
	}
	account, p, ok := chatAuth(w, r)
	if !ok {
		return
	}
	if s.plus == nil {
		writePlusError(w, r, plusChatError("capability_unavailable", 503))
		return
	}
	scope := AssistantPlusScope{AccountID: account, ActorID: p.UserID, IssuerID: r.URL.Query().Get("issuerId")}
	if r.Method == http.MethodDelete {
		if e := s.plus.Delete(r.Context(), scope, id); e != nil {
			writePlusError(w, r, e)
			return
		}
		w.WriteHeader(204)
		return
	}
	page, limit := 1, 20
	var e error
	if r.URL.Query().Has("page") {
		page, e = strconv.Atoi(r.URL.Query().Get("page"))
		if e != nil {
			writePlusError(w, r, plusChatError("invalid_page", 400))
			return
		}
	}
	if r.URL.Query().Has("limit") {
		limit, e = strconv.Atoi(r.URL.Query().Get("limit"))
		if e != nil {
			writePlusError(w, r, plusChatError("invalid_page", 400))
			return
		}
	}
	out, e := s.plus.History(r.Context(), scope, id, page, limit)
	if e != nil {
		writePlusError(w, r, e)
		return
	}
	httpapi.WriteJSON(w, 200, out)
}
