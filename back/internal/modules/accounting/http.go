package accounting

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

func decode(w http.ResponseWriter, r *http.Request, out any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	var raw json.RawMessage
	if e := d.Decode(&raw); e != nil {
		return failure("invalid_input", 400)
	}
	if len(bytes.TrimSpace(raw)) == 0 || bytes.TrimSpace(raw)[0] != '{' {
		return failure("invalid_input", 400)
	}
	if _, ok := out.(*Command); ok {
		var fields map[string]json.RawMessage
		if e := json.Unmarshal(raw, &fields); e != nil {
			return failure("invalid_input", 400)
		}
		allowed := commandFields(r)
		for key := range fields {
			if !allowed[key] {
				return failure("invalid_input", 400)
			}
		}
	}
	strict := json.NewDecoder(bytes.NewReader(raw))
	strict.DisallowUnknownFields()
	if e := strict.Decode(out); e != nil {
		return failure("invalid_input", 400)
	}
	if e := d.Decode(new(any)); !errors.Is(e, io.EOF) {
		return failure("invalid_input", 400)
	}
	return nil
}
func commandFields(r *http.Request) map[string]bool {
	fields := "expectedVersion"
	switch {
	case r.Method == http.MethodDelete:
	case strings.Contains(r.URL.Path, "/members/"):
		fields += " status grants confirmed"
	case strings.HasSuffix(r.URL.Path, "/issuers"):
		fields += " profile intent enableIssue"
	case strings.HasSuffix(r.URL.Path, "/validate"):
		fields += " connectorId environment"
	case strings.HasSuffix(r.URL.Path, "/reviews"):
		fields += " expectedProfileVersion snapshotHash confirmed connectorId environment"
	case strings.HasSuffix(r.URL.Path, "/authorizations"):
		fields += " reviewId snapshotHash confirmed connectorId environment"
	case strings.HasSuffix(r.URL.Path, "/execute"):
		fields += " authorizationId businessKey operation"
	case r.Method == http.MethodPatch && strings.Contains(r.URL.Path, "/documents/"):
		fields += " fields source"
	case r.Method == http.MethodPatch:
		fields += " profile"
	}
	out := map[string]bool{}
	for _, name := range strings.Fields(fields) {
		out[name] = true
	}
	return out
}

func result(w http.ResponseWriter, r *http.Request, status int, data any, e error) {
	if e != nil {
		code, httpStatus := "accounting_error", 500
		var domain *Error
		if errors.As(e, &domain) {
			code, httpStatus = domain.Code, domain.Status
		}
		httpapi.WriteJSON(w, httpStatus, map[string]any{"contractVersion": "1.0", "status": "error", "requestId": httpapi.RequestIDFromContext(r.Context()), "error": map[string]any{"code": code, "message": "Não foi possível concluir a operação. Confira os dados e permissões.", "fields": []string{}}})
		return
	}
	httpapi.WriteJSON(w, status, map[string]any{"contractVersion": "1.0", "status": "success", "requestId": httpapi.RequestIDFromContext(r.Context()), "data": data})
}
func scope(r *http.Request) Scope {
	p, _ := auth.PrincipalFromContext(r.Context())
	return Scope{AccountID: p.AccountID, ActorID: p.UserID, IssuerID: r.PathValue("issuer"), CorrelationID: httpapi.RequestIDFromContext(r.Context())}
}
func pageLimit(r *http.Request) (int, int, error) {
	page, limit := 1, 20
	var e error
	if r.URL.Query().Has("page") {
		page, e = strconv.Atoi(r.URL.Query().Get("page"))
		if e != nil {
			return 0, 0, failure("invalid_page", 400)
		}
	}
	if r.URL.Query().Has("limit") {
		limit, e = strconv.Atoi(r.URL.Query().Get("limit"))
		if e != nil {
			return 0, 0, failure("invalid_page", 400)
		}
	}
	if page < 1 || page > 100000 || limit < 1 || limit > 100 {
		return 0, 0, failure("invalid_page", 400)
	}
	return page, limit, nil
}
func RegisterRoutes(mux *http.ServeMux, s *Service, m *auth.Middleware) {
	RegisterArtifactRoute(mux, s, m)
	RegisterObligationRoutes(mux, s, m)
	register := func(pattern string, h http.HandlerFunc) { mux.Handle(pattern, m.RequireAuthWithAccount(h)) }
	register("POST /v1/accounting/issuers", func(w http.ResponseWriter, r *http.Request) {
		var in Command
		if e := decode(w, r, &in); e != nil {
			result(w, r, 0, nil, e)
			return
		}
		out, e := s.CreateIssuer(r.Context(), scope(r), r.Header.Get("Idempotency-Key"), in)
		result(w, r, 201, out, e)
	})
	for _, pattern := range []string{"DELETE /v1/accounting/issuers/{issuer}", "DELETE /v1/accounting/issuers/{issuer}/documents/{id}"} {
		register(pattern, func(w http.ResponseWriter, r *http.Request) {
			var in Command
			if e := decode(w, r, &in); e != nil {
				result(w, r, 0, nil, e)
				return
			}
			out, e := s.Delete(r.Context(), scope(r), r.PathValue("id"), r.Header.Get("Idempotency-Key"), in)
			result(w, r, 200, out, e)
		})
	}
	register("GET /v1/accounting/issuers", func(w http.ResponseWriter, r *http.Request) {
		p, l, e := pageLimit(r)
		if e != nil {
			result(w, r, 0, nil, e)
			return
		}
		out, e := s.ListIssuers(r.Context(), scope(r), p, l)
		result(w, r, 200, out, e)
	})
	register("PATCH /v1/accounting/issuers/{issuer}", func(w http.ResponseWriter, r *http.Request) {
		var in Command
		if e := decode(w, r, &in); e != nil {
			result(w, r, 0, nil, e)
			return
		}
		out, e := s.UpdateIssuer(r.Context(), scope(r), r.Header.Get("Idempotency-Key"), in)
		result(w, r, 200, out, e)
	})
	register("PUT /v1/accounting/issuers/{issuer}/members/{id}", func(w http.ResponseWriter, r *http.Request) {
		var in Command
		if e := decode(w, r, &in); e != nil {
			result(w, r, 0, nil, e)
			return
		}
		out, e := s.SaveMember(r.Context(), scope(r), r.PathValue("id"), r.Header.Get("Idempotency-Key"), in)
		result(w, r, 200, out, e)
	})
	register("PUT /v1/accounting/issuers/{issuer}/diagnosis", func(w http.ResponseWriter, r *http.Request) {
		var in DiagnosisInput
		if e := decode(w, r, &in); e != nil {
			result(w, r, 0, nil, e)
			return
		}
		out, e := s.SaveDiagnosis(r.Context(), scope(r), r.Header.Get("Idempotency-Key"), in)
		result(w, r, 200, out, e)
	})
	register("POST /v1/accounting/issuers/{issuer}/documents/prepare", func(w http.ResponseWriter, r *http.Request) {
		var in PrepareInput
		if e := decode(w, r, &in); e != nil {
			result(w, r, 0, nil, e)
			return
		}
		out, e := s.Prepare(r.Context(), scope(r), r.Header.Get("Idempotency-Key"), in)
		result(w, r, 201, out, e)
	})
	reads := map[string]string{
		"GET /v1/accounting/issuers/{issuer}":                         "issuer",
		"GET /v1/accounting/issuers/{issuer}/documents":               "documents",
		"GET /v1/accounting/issuers/{issuer}/documents/{id}":          "document",
		"GET /v1/accounting/issuers/{issuer}/documents/{id}/review":   "review",
		"GET /v1/accounting/issuers/{issuer}/executions/{id}":         "execution",
		"GET /v1/accounting/issuers/{issuer}/executions/{id}/history": "history",
	}
	for pattern, kind := range reads {
		register(pattern, func(w http.ResponseWriter, r *http.Request) {
			p, l, e := pageLimit(r)
			if e != nil {
				result(w, r, 0, nil, e)
				return
			}
			out, e := s.Read(r.Context(), scope(r), kind, r.PathValue("id"), r.URL.Query().Get("connectorId"), r.URL.Query().Get("environment"), p, l)
			result(w, r, 200, out, e)
		})
	}
	register("GET /v1/accounting/issuers/{issuer}/diagnosis", func(w http.ResponseWriter, r *http.Request) {
		p, l, e := pageLimit(r)
		if e != nil {
			result(w, r, 0, nil, e)
			return
		}
		id := r.URL.Query().Get("diagnosisId")
		kind := "diagnoses"
		if id != "" {
			kind = "diagnosis"
		}
		out, e := s.Read(r.Context(), scope(r), kind, id, "", "", p, l)
		result(w, r, 200, out, e)
	})
	commands := map[string]string{
		"PATCH /v1/accounting/issuers/{issuer}/documents/{id}":                   "edit",
		"POST /v1/accounting/issuers/{issuer}/documents/{id}/validate":           "validate",
		"POST /v1/accounting/issuers/{issuer}/documents/{id}/reviews":            "review",
		"DELETE /v1/accounting/issuers/{issuer}/documents/{id}/reviews/{review}": "revoke-review",
		"POST /v1/accounting/issuers/{issuer}/documents/{id}/authorizations":     "authorize",
		"POST /v1/accounting/issuers/{issuer}/documents/{id}/execute":            "execute",
	}
	for pattern, op := range commands {
		register(pattern, func(w http.ResponseWriter, r *http.Request) {
			var in Command
			if e := decode(w, r, &in); e != nil {
				result(w, r, 0, nil, e)
				return
			}
			if op == "revoke-review" {
				in.ReviewID = r.PathValue("review")
			}
			out, e := s.DocumentCommand(r.Context(), scope(r), r.PathValue("id"), op, r.Header.Get("Idempotency-Key"), in)
			status := 200
			if op == "execute" {
				status = 202
			}
			if op == "review" || op == "authorize" {
				status = 201
			}
			result(w, r, status, out, e)
		})
	}
	register("POST /v1/accounting/issuers/{issuer}/authorizations/{id}/revoke", func(w http.ResponseWriter, r *http.Request) {
		var in Command
		if e := decode(w, r, &in); e != nil {
			result(w, r, 0, nil, e)
			return
		}
		out, e := s.RevokeAuthorization(r.Context(), scope(r), r.PathValue("id"), r.Header.Get("Idempotency-Key"), in)
		result(w, r, 200, out, e)
	})
	register("POST /v1/accounting/issuers/{issuer}/executions/{id}/recover-artifacts", func(w http.ResponseWriter, r *http.Request) {
		var in Command
		if e := decode(w, r, &in); e != nil {
			result(w, r, 0, nil, e)
			return
		}
		out, e := s.Recover(r.Context(), scope(r), r.PathValue("id"), r.Header.Get("Idempotency-Key"), in)
		result(w, r, 202, out, e)
	})
}
