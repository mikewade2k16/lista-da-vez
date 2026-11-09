package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
)

func (s *Service) ingestionSource(id string) (IngestionSource, error) {
	for _, source := range s.ports.IngestionSources {
		if source.ID == id {
			if e := validateIngestionSource(source); e != nil {
				return source, e
			}
			return source, nil
		}
	}
	return IngestionSource{}, fail("source_not_allowed", 422)
}
func (s *Service) IngestionCatalog(ctx context.Context, sc Scope) ([]IngestionSource, error) {
	if e := s.authorize(ctx, sc, false); e != nil {
		return nil, e
	}
	out := []IngestionSource{}
	for _, v := range s.ports.IngestionSources {
		if validateIngestionSource(v) == nil {
			out = append(out, v)
		}
	}
	return out, nil
}
func (s *Service) SaveSourceSubscription(ctx context.Context, sc Scope, key string, in SubscriptionInput) (json.RawMessage, error) {
	if e := s.authorize(ctx, sc, true); e != nil {
		return nil, e
	}
	if !idPattern.MatchString(in.BaseID) || in.ExpectedVersion < 0 || in.IntervalHours < 6 || in.IntervalHours > 168 {
		return nil, fail("invalid_input", 400)
	}
	source, e := s.ingestionSource(in.SourceID)
	if e != nil {
		return nil, e
	}
	if s.ingestionQueue == nil {
		return nil, fail("ingestion_unavailable", 503)
	}
	if in.Enabled {
		if e = s.store.ingestionBase(ctx, sc, in.BaseID); e != nil {
			return nil, e
		}
	} else {
		if e = s.store.assertBase(ctx, sc, in.BaseID); e != nil {
			return nil, e
		}
	}
	return s.store.ingestionCommand(ctx, sc, key, in, func(tx pgx.Tx) (any, error) {
		if _, e := tx.Exec(ctx, `select pg_advisory_xact_lock(hashtextextended($1,0))`, sc.AccountID+sc.ResourceID+"knowledge-sources"); e != nil {
			return nil, e
		}
		current, e := scanSubscription(tx.QueryRow(ctx, `select `+subscriptionColumns+` from messaging.knowledge_source_subscriptions s where s.account_id=$1::uuid and s.issuer_id=$2::uuid and s.base_id=$3::uuid and s.source_id=$4 for update`, sc.AccountID, sc.ResourceID, in.BaseID, in.SourceID))
		if e != nil {
			var domain *Error
			if !errors.As(e, &domain) || domain.Status != 404 {
				return nil, e
			}
		}
		if current.Version != in.ExpectedVersion {
			return nil, fail("version_conflict", 409)
		}
		if current.ID == "" {
			var count int
			if e = tx.QueryRow(ctx, `select count(*) from messaging.knowledge_source_subscriptions where account_id=$1::uuid and issuer_id=$2::uuid`, sc.AccountID, sc.ResourceID).Scan(&count); e != nil {
				return nil, e
			}
			if count >= 20 {
				return nil, fail("source_limit", 409)
			}
		}
		sub, e := scanSubscription(tx.QueryRow(ctx, `insert into messaging.knowledge_source_subscriptions as s(account_id,issuer_id,actor_id,base_id,source_id,source_revision,enabled,interval_hours)values($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5,$6,$7,$8)on conflict(account_id,issuer_id,base_id,source_id)do update set actor_id=excluded.actor_id,source_revision=excluded.source_revision,enabled=excluded.enabled,interval_hours=excluded.interval_hours,version=s.version+1,next_check_at=now(),updated_at=now()returning `+subscriptionColumns, sc.AccountID, sc.ResourceID, sc.ActorID, in.BaseID, in.SourceID, source.Revision, in.Enabled, in.IntervalHours))
		if e != nil {
			return nil, e
		}
		_, e = tx.Exec(ctx, `update messaging.knowledge_ingestion_runs set state='cancelled',fence=fence+1,lease_until=null,error_code='subscription_changed',updated_at=now() where account_id=$1::uuid and subscription_id=$2::uuid and subscription_version<>$3 and state in ('queued','fetching','retry_wait')`, sc.AccountID, sub.ID, sub.Version)
		if e != nil {
			return nil, e
		}
		if e = s.authorize(ctx, sc, true); e != nil {
			return nil, e
		}
		return sub, nil
	})
}
func (s *Service) SourceSubscriptions(ctx context.Context, sc Scope) ([]SourceSubscription, error) {
	if e := s.authorize(ctx, sc, false); e != nil {
		return nil, e
	}
	rows, e := s.store.pool.Query(ctx, `select `+subscriptionColumns+` from messaging.knowledge_source_subscriptions s where s.account_id=$1::uuid and s.issuer_id=$2::uuid order by s.created_at,s.id limit 20`, sc.AccountID, sc.ResourceID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []SourceSubscription{}
	for rows.Next() {
		v, e := scanSubscription(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Service) IngestionRuns(ctx context.Context, sc Scope, page, limit int) (any, error) {
	if e := s.authorize(ctx, sc, false); e != nil {
		return nil, e
	}
	if page < 1 || page > 10000 || limit < 1 || limit > 100 {
		return nil, fail("invalid_page", 400)
	}
	rows, e := s.store.pool.Query(ctx, `select `+ingestionRunColumns+` from messaging.knowledge_ingestion_runs r where r.account_id=$1::uuid and r.issuer_id=$2::uuid order by r.created_at desc,r.id limit $3 offset $4`, sc.AccountID, sc.ResourceID, limit, (page-1)*limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []IngestionRun{}
	for rows.Next() {
		r, e := scanIngestionRun(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, r)
	}
	return map[string]any{"items": out, "page": page, "limit": limit, "publicationAutomatic": false, "retrieval": "fts_simple"}, rows.Err()
}
