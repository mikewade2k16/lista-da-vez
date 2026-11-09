package accounting

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct{ pool *pgxpool.Pool }

func NewStore(p *pgxpool.Pool) *Store { return &Store{pool: p} }
func dbError(e error) error {
	if errors.Is(e, pgx.ErrNoRows) {
		return failure("not_found", 404)
	}
	return e
}
func (s *Store) begin(ctx context.Context) (pgx.Tx, error) { return s.pool.Begin(ctx) }
func issuerLock(ctx context.Context, tx pgx.Tx, s Scope, grant string) (int64, error) {
	if !validID(s.IssuerID) {
		return 0, failure("not_found", 404)
	}
	var access int64
	var grants []string
	e := tx.QueryRow(ctx, `select i.access_version,m.grants from accounting.issuers i join accounting.issuer_members m on m.account_id=i.account_id and m.issuer_id=i.id
 where i.account_id=$1::uuid and i.id=$2::uuid and m.user_id=$3::uuid and i.status='active' and m.status='active' for update of i,m`, s.AccountID, s.IssuerID, s.ActorID).Scan(&access, &grants)
	if e != nil {
		return 0, dbError(e)
	}
	for _, g := range grants {
		if g == grant {
			return access, nil
		}
	}
	return 0, failure("permission_denied", 403)
}
func loadDocument(ctx context.Context, tx pgx.Tx, s Scope, id string) (Document, error) {
	var d Document
	var fields []byte
	if !validID(id) {
		return d, failure("not_found", 404)
	}
	e := tx.QueryRow(ctx, `select id::text,issuer_id::text,version,profile_version,diagnosis_id::text,diagnosis_version,kind,state,payload,source_ref,validated_connector,validated_environment,content_version from accounting.documents
 where deleted_at is null and account_id=$1::uuid and issuer_id=$2::uuid and id=$3::uuid for update`, s.AccountID, s.IssuerID, id).Scan(&d.ID, &d.IssuerID, &d.Version, &d.ProfileVersion, &d.DiagnosisID, &d.DiagnosisVersion, &d.Kind, &d.State, &fields, &d.Source, &d.ValidatedConnector, &d.ValidatedEnvironment, &d.ContentVersion)
	if e != nil {
		return d, dbError(e)
	}
	e = json.Unmarshal(fields, &d.Fields)
	return d, e
}
func profile(ctx context.Context, tx pgx.Tx, s Scope) (Profile, int64, error) {
	var p Profile
	var version int64
	var data []byte
	e := tx.QueryRow(ctx, `select version,data from accounting.issuer_profiles where account_id=$1::uuid and issuer_id=$2::uuid order by version desc limit 1`, s.AccountID, s.IssuerID).Scan(&version, &data)
	if e != nil {
		return p, 0, dbError(e)
	}
	e = json.Unmarshal(data, &p)
	return p, version, e
}
func (s *Store) command(ctx context.Context, sc Scope, op, key string, input any, grant string, run func(pgx.Tx) (any, error)) (json.RawMessage, error) {
	if len(key) < 8 || len(key) > 200 {
		return nil, failure("invalid_idempotency_key", 400)
	}
	for _, r := range key {
		if r < 33 || r > 126 {
			return nil, failure("invalid_idempotency_key", 400)
		}
	}
	tx, e := s.begin(ctx)
	if e != nil {
		return nil, e
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// Serializes identical create keys as well as concurrent writes in the issuer.
	_, e = tx.Exec(ctx, `select pg_advisory_xact_lock(hashtextextended($1,0))`, sc.AccountID+sc.ActorID+sc.IssuerID+op+key)
	if e != nil {
		return nil, e
	}
	if sc.IssuerID != "" {
		if _, e = issuerLock(ctx, tx, sc, grant); e != nil {
			return nil, e
		}
	}
	h := hash(input)
	var previous string
	var result json.RawMessage
	e = tx.QueryRow(ctx, `select request_hash,result from accounting.request_receipts where account_id=$1::uuid and actor_id=$2::uuid and issuer_id is not distinct from nullif($3,'')::uuid and operation=$4 and idempotency_key=$5`, sc.AccountID, sc.ActorID, sc.IssuerID, op, key).Scan(&previous, &result)
	if e == nil {
		if h != previous {
			return nil, failure("idempotency_conflict", 409)
		}
		return result, tx.Commit(ctx)
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return nil, e
	}
	value, e := run(tx)
	if e != nil {
		return nil, e
	}
	result, e = json.Marshal(value)
	if e != nil {
		return nil, e
	}
	_, e = tx.Exec(ctx, `insert into accounting.request_receipts(account_id,actor_id,issuer_id,operation,idempotency_key,request_hash,result) values($1::uuid,$2::uuid,nullif($3,'')::uuid,$4,$5,$6,$7)`, sc.AccountID, sc.ActorID, sc.IssuerID, op, key, h, result)
	if e != nil {
		return nil, e
	}
	return result, tx.Commit(ctx)
}
func invalidate(ctx context.Context, tx pgx.Tx, s Scope, id string) error {
	_, e := tx.Exec(ctx, `update accounting.document_reviews set revoked_at=coalesce(revoked_at,now()) where account_id=$1::uuid and issuer_id=$2::uuid and document_id=$3::uuid`, s.AccountID, s.IssuerID, id)
	if e != nil {
		return e
	}
	_, e = tx.Exec(ctx, `update accounting.submission_authorizations set revoked_at=coalesce(revoked_at,now()),version=version+1 where account_id=$1::uuid and issuer_id=$2::uuid and document_id=$3::uuid and consumed_at is null and revoked_at is null`, s.AccountID, s.IssuerID, id)
	return e
}
