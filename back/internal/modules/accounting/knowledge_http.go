package accounting

import (
	"encoding/json"
	"errors"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/knowledge"
	"io"
	"net/http"
)

func knowledgeResult(w http.ResponseWriter, r *http.Request, v any, e error) {
	var domain *knowledge.Error
	if errors.As(e, &domain) {
		e = failure(domain.Code, domain.Status)
	}
	result(w, r, 200, v, e)
}
func knowledgeDecode(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 3*1024*1024)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return failure("invalid_input", 400)
	}
	if e := d.Decode(new(any)); !errors.Is(e, io.EOF) {
		return failure("invalid_input", 400)
	}
	return nil
}
func RegisterKnowledgeRoutes(mux *http.ServeMux, s *knowledge.Service, middleware *auth.Middleware) {
	registerKnowledgeIngestionRoutes(mux, s, middleware)
	const prefix = "/v1/accounting/issuers/{issuer}/knowledge"
	handle := func(pattern string, fn http.HandlerFunc) { mux.Handle(pattern, middleware.RequireAuthWithAccount(fn)) }
	context := func(r *http.Request) knowledge.Scope {
		sc := scope(r)
		return knowledge.Scope{AccountID: sc.AccountID, ActorID: sc.ActorID, ResourceID: sc.IssuerID}
	}
	handle("GET "+prefix+"/bases", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.Bases(r.Context(), context(r))
		knowledgeResult(w, r, v, e)
	})
	handle("POST "+prefix+"/bases", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Name string `json:"name"`
		}
		if e := knowledgeDecode(w, r, &in); e != nil {
			knowledgeResult(w, r, nil, e)
			return
		}
		v, e := s.CreateBase(r.Context(), context(r), in.Name)
		knowledgeResult(w, r, v, e)
	})
	handle("PATCH "+prefix+"/bases/{base}", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			ExpectedVersion int64 `json:"expectedVersion"`
			Enabled         bool  `json:"enabled"`
		}
		if e := knowledgeDecode(w, r, &in); e != nil {
			knowledgeResult(w, r, nil, e)
			return
		}
		v, e := s.SetBase(r.Context(), context(r), r.PathValue("base"), in.ExpectedVersion, in.Enabled)
		knowledgeResult(w, r, v, e)
	})
	handle("GET "+prefix+"/bases/{base}/documents", func(w http.ResponseWriter, r *http.Request) {
		page, limit, e := pageLimit(r)
		if e != nil {
			knowledgeResult(w, r, nil, e)
			return
		}
		v, e := s.Documents(r.Context(), context(r), r.PathValue("base"), page, limit)
		knowledgeResult(w, r, v, e)
	})
	handle("POST "+prefix+"/imports", func(w http.ResponseWriter, r *http.Request) {
		var in knowledge.Import
		if e := knowledgeDecode(w, r, &in); e != nil {
			knowledgeResult(w, r, nil, e)
			return
		}
		v, e := s.Import(r.Context(), context(r), in)
		knowledgeResult(w, r, v, e)
	})
	handle("PUT "+prefix+"/documents/{document}/review", func(w http.ResponseWriter, r *http.Request) {
		var in knowledge.Review
		if e := knowledgeDecode(w, r, &in); e != nil {
			knowledgeResult(w, r, nil, e)
			return
		}
		v, e := s.Review(r.Context(), context(r), r.PathValue("document"), in)
		knowledgeResult(w, r, v, e)
	})
	for _, op := range []string{"publish", "deactivate"} {
		handle("POST "+prefix+"/documents/{document}/"+op, func(w http.ResponseWriter, r *http.Request) {
			var in struct {
				ExpectedVersion int64 `json:"expectedVersion"`
				Confirmed       bool  `json:"confirmed"`
			}
			if e := knowledgeDecode(w, r, &in); e != nil {
				knowledgeResult(w, r, nil, e)
				return
			}
			var v knowledge.Document
			var e error
			if op == "publish" {
				v, e = s.Publish(r.Context(), context(r), r.PathValue("document"), in.ExpectedVersion, in.Confirmed)
			} else {
				v, e = s.Deactivate(r.Context(), context(r), r.PathValue("document"), in.ExpectedVersion)
			}
			knowledgeResult(w, r, v, e)
		})
	}
	handle("GET "+prefix+"/evidence/{document}/{chunk}", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.Resolve(r.Context(), context(r), r.PathValue("document"), r.PathValue("chunk"))
		knowledgeResult(w, r, v, e)
	})
	handle("POST "+prefix+"/search", func(w http.ResponseWriter, r *http.Request) {
		var in knowledge.Query
		if e := knowledgeDecode(w, r, &in); e != nil {
			knowledgeResult(w, r, nil, e)
			return
		}
		v, e := s.Search(r.Context(), context(r), in)
		knowledgeResult(w, r, v, e)
	})
}
