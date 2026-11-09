package knowledge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

const MaxBytes = 2 * 1024 * 1024

func validDate(p *string) bool {
	if p == nil {
		return true
	}
	t, e := time.Parse("2006-01-02", *p)
	return e == nil && t.Format("2006-01-02") == *p
}
func validateMetadata(m Metadata) error {
	for _, v := range []string{m.Authority, m.Jurisdiction, m.Regime, m.Classification, m.ValiditySource} {
		if len(v) > 1000 || strings.ContainsRune(v, 0) {
			return fail("invalid_metadata", 400)
		}
	}
	if strings.TrimSpace(m.Authority) == "" || strings.TrimSpace(m.Jurisdiction) == "" || strings.TrimSpace(m.Classification) == "" || !validDate(m.PublishedOn) || !validDate(m.ValidFrom) || !validDate(m.ValidTo) {
		return fail("invalid_metadata", 400)
	}
	if m.ValidFrom != nil && (m.ValiditySource == "" || (m.ValidTo != nil && *m.ValidTo <= *m.ValidFrom)) {
		return fail("invalid_validity", 400)
	}
	return nil
}
func (s *Service) Bases(ctx context.Context, sc Scope) ([]Base, error) {
	if e := s.authorize(ctx, sc, false); e != nil {
		return nil, e
	}
	return s.store.bases(ctx, sc)
}
func (s *Service) CreateBase(ctx context.Context, sc Scope, name string) (Base, error) {
	if e := s.authorize(ctx, sc, true); e != nil {
		return Base{}, e
	}
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 200 {
		return Base{}, fail("invalid_name", 400)
	}
	return s.store.createBase(ctx, sc, name)
}
func (s *Service) SetBase(ctx context.Context, sc Scope, id string, version int64, enabled bool) (Base, error) {
	if e := s.authorize(ctx, sc, true); e != nil {
		return Base{}, e
	}
	if !idPattern.MatchString(id) || version < 1 {
		return Base{}, fail("invalid_input", 400)
	}
	return s.store.setBase(ctx, sc, id, version, enabled)
}
func (s *Service) Documents(ctx context.Context, sc Scope, base string, page, limit int) ([]Document, error) {
	if e := s.authorize(ctx, sc, false); e != nil {
		return nil, e
	}
	if !idPattern.MatchString(base) || page < 1 || page > 10000 || limit < 1 || limit > 100 {
		return nil, fail("invalid_input", 400)
	}
	return s.store.documents(ctx, sc, base, page, limit)
}
func (s *Service) Import(ctx context.Context, sc Scope, in Import) (Document, error) {
	if e := s.authorize(ctx, sc, true); e != nil {
		return Document{}, e
	}
	source, e := url.Parse(in.SourceRef)
	if e != nil || source.User != nil || source.Fragment != "" || source.RawQuery != "" || (source.Scheme != "https" && source.Scheme != "manual") || source.Host == "" || len(in.SourceRef) > 1000 || !idPattern.MatchString(in.BaseID) || strings.TrimSpace(in.Title) == "" || len(in.Title) > 500 || len(in.FileName) > 200 || len(in.Content) == 0 || len(in.Content) > MaxBytes {
		return Document{}, fail("invalid_import", 400)
	}
	if e = validateMetadata(in.Metadata); e != nil {
		return Document{}, e
	}
	if e = s.store.assertBase(ctx, sc, in.BaseID); e != nil {
		return Document{}, e
	}
	hash := sha256.Sum256(in.Content)
	checksum := hex.EncodeToString(hash[:])
	if old, found, e := s.store.duplicate(ctx, sc, in.BaseID, checksum); e != nil || found {
		return old, e
	}
	var text string
	switch in.ContentType {
	case "text/plain":
		if !utf8.Valid(in.Content) || strings.ContainsRune(string(in.Content), 0) {
			return Document{}, fail("invalid_text", 400)
		}
		text = string(in.Content)
	case "application/pdf":
		if len(in.Content) < 5 || string(in.Content[:5]) != "%PDF-" {
			return Document{}, fail("invalid_pdf", 400)
		}
		if s.ports.ExtractPDF == nil {
			return Document{}, fail("extraction_unavailable", 503)
		}
		text, e = s.ports.ExtractPDF(ctx, in.Content)
		if e != nil {
			return Document{}, fail("extraction_failed", 422)
		}
	default:
		return Document{}, fail("unsupported_type", 415)
	}
	// Plain text only. HTML/scripts are rejected rather than executed or fetched.
	if strings.Contains(strings.ToLower(text), "<script") || strings.Contains(strings.ToLower(text), "<html") || strings.TrimSpace(text) == "" || len(text) > 256000 {
		return Document{}, fail("invalid_text", 422)
	}
	if s.ports.PutOriginal == nil {
		return Document{}, fail("storage_unavailable", 503)
	}
	object, e := s.ports.PutOriginal(ctx, sc, in.BaseID+":"+checksum, in)
	if e != nil {
		return Document{}, fail("storage_unavailable", 503)
	}
	if !idPattern.MatchString(object) {
		return Document{}, fail("storage_unavailable", 503)
	}
	if e = s.authorize(ctx, sc, true); e != nil {
		return Document{}, e
	}
	return s.store.ingest(ctx, sc, in, object, checksum, text)
}
func (s *Service) Review(ctx context.Context, sc Scope, id string, in Review) (Document, error) {
	if e := s.authorize(ctx, sc, true); e != nil {
		return Document{}, e
	}
	if !idPattern.MatchString(id) || in.ExpectedVersion < 1 {
		return Document{}, fail("invalid_input", 400)
	}
	if e := validateMetadata(in.Metadata); e != nil {
		return Document{}, e
	}
	return s.store.review(ctx, sc, id, in)
}
func (s *Service) Publish(ctx context.Context, sc Scope, id string, version int64, confirmed bool) (Document, error) {
	if e := s.authorize(ctx, sc, true); e != nil {
		return Document{}, e
	}
	if !confirmed || !idPattern.MatchString(id) || version < 1 {
		return Document{}, fail("review_required", 400)
	}
	return s.store.publish(ctx, sc, id, version)
}
func (s *Service) Deactivate(ctx context.Context, sc Scope, id string, version int64) (Document, error) {
	if e := s.authorize(ctx, sc, true); e != nil {
		return Document{}, e
	}
	if !idPattern.MatchString(id) || version < 1 {
		return Document{}, fail("invalid_input", 400)
	}
	return s.store.deactivate(ctx, sc, id, version)
}
func (s *Service) Search(ctx context.Context, sc Scope, q Query) (Result, error) {
	out := Result{Status: "insufficient_evidence", Evidence: []Evidence{}, Message: "Sem evidência aplicável. Score FTS não é confiança jurídica."}
	if e := s.authorize(ctx, sc, false); e != nil {
		return out, e
	}
	q.Query = strings.TrimSpace(q.Query)
	if q.TopK == 0 {
		q.TopK = 5
	}
	if q.Page == 0 {
		q.Page = 1
	}
	if q.Query == "" || len([]rune(q.Query)) > 500 || q.TopK < 1 || q.TopK > 20 || q.Page < 1 || q.Page > 10000 || len(q.Jurisdiction) > 100 || len(q.Regime) > 100 {
		return out, fail("invalid_query", 400)
	}
	if q.CompetenceDate != "" && !validDate(&q.CompetenceDate) {
		return out, fail("invalid_date", 400)
	}
	evidence, e := s.store.searchFiscal(ctx, sc, q)
	if e != nil {
		return out, e
	}
	if e = s.authorize(ctx, sc, false); e != nil {
		return out, e
	}
	size := 0
	for index, item := range evidence {
		size += len(raw(item))
		if size > 24*1024 {
			evidence = evidence[:index]
			out.Truncated = true
			break
		}
	}
	out.Evidence = evidence
	for _, v := range evidence {
		if v.Applicable {
			out.Status = "completed"
		}
	}
	// Multiple matching sources/versions require review; ranking never resolves legal conflict.
	sources := map[string]bool{}
	for _, v := range evidence {
		sources[v.DocumentID] = true
	}
	out.PotentialConflict = len(sources) > 1
	if len(evidence) > 0 {
		out.Message = "Trechos recuperados como dados informativos. Confira aplicabilidade e possíveis conflitos antes de usar."
	}
	return out, nil
}
func raw(v any) json.RawMessage { b, _ := json.Marshal(v); return b }
