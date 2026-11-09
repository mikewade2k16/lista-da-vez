package accounting

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
)

func readDiagnosis(ctx context.Context, tx pgx.Tx, s Scope, id string) (json.RawMessage, error) {
	if !validID(id) {
		return nil, failure("not_found", 404)
	}
	var out json.RawMessage
	e := repositoryReadDiagnosisQueryRowDiagnoses(ctx, tx, s.AccountID, s.IssuerID, id, &out)
	return out, dbError(e)
}
func (s *Service) Read(ctx context.Context, sc Scope, kind, id, connector, environment string, page, limit int) (any, error) {
	if page < 1 || page > 100000 || limit < 1 || limit > 100 {
		return nil, failure("invalid_page", 400)
	}
	perm := "accounting.documents.view"
	if kind == "issuer" || kind == "diagnosis" || kind == "diagnoses" {
		perm = "accounting.issuers.view"
	}
	if e := s.check(ctx, sc, perm); e != nil {
		return nil, e
	}
	tx, e := s.store.begin(ctx)
	if e != nil {
		return nil, e
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, e = issuerLock(ctx, tx, sc, "view"); e != nil {
		return nil, e
	}
	switch kind {
	case "issuer":
		p, pv, e := profile(ctx, tx, sc)
		if e != nil {
			return nil, e
		}
		var v int64
		e = repositoryReadQueryRowIssuers(ctx, tx, sc.AccountID, sc.IssuerID, &v)
		return map[string]any{"id": sc.IssuerID, "version": v, "profileVersion": pv, "profile": p, "emissionAvailable": false}, e
	case "diagnosis":
		return readDiagnosis(ctx, tx, sc, id)
	case "document":
		return loadDocument(ctx, tx, sc, id)
	case "review":
		d, e := loadDocument(ctx, tx, sc, id)
		if e != nil {
			return nil, e
		}
		return s.review(ctx, tx, sc, d, connector, environment)
	case "execution":
		x, e := loadExecution(ctx, tx, sc, id)
		if e != nil {
			return nil, e
		}
		d, e := loadDocument(ctx, tx, sc, x.DocumentID)
		if e != nil {
			return nil, e
		}
		x.DocumentState = d.State
		x.Artifacts, e = artifactViews(ctx, tx, sc, id)
		return x, e
	case "history":
		if _, e = loadExecution(ctx, tx, sc, id); e != nil {
			return nil, e
		}
		rows, e := repositoryReadQueryExecutionEvents(ctx, tx, sc.AccountID, sc.IssuerID, id, limit, (page-1)*limit)
		if e != nil {
			return nil, e
		}
		defer rows.Close()
		out := []json.RawMessage{}
		for rows.Next() {
			var b json.RawMessage
			if e = rows.Scan(&b); e != nil {
				return nil, e
			}
			out = append(out, b)
		}
		return out, rows.Err()
	case "documents", "diagnoses":
		return listRecords(ctx, tx, sc, kind, page, limit)
	}
	return nil, failure("not_found", 404)
}
func (s *Service) ListIssuers(ctx context.Context, sc Scope, page, limit int) (any, error) {
	if page < 1 || page > 100000 || limit < 1 || limit > 100 {
		return nil, failure("invalid_page", 400)
	}
	if e := s.check(ctx, sc, "accounting.issuers.view"); e != nil {
		return nil, e
	}
	rows, e := s.store.repositoryListIssuersQueryIssuers(ctx, sc.AccountID, sc.ActorID, limit, (page-1)*limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id string
		var v int64
		if e = rows.Scan(&id, &v); e != nil {
			return nil, e
		}
		out = append(out, map[string]any{"id": id, "version": v})
	}
	return out, rows.Err()
}
