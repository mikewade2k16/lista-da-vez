package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

func fiscalError(e error) error {
	if errors.Is(e, pgx.ErrNoRows) {
		return fail("not_found", 404)
	}
	return e
}
func (s *Store) bases(ctx context.Context, sc Scope) ([]Base, error) {
	rows, e := s.pool.Query(ctx, `select k.id::text,k.name,b.is_enabled and k.is_enabled,b.version from messaging.knowledge_bases k join accounting.knowledge_bindings b on b.account_id=k.account_id and b.knowledge_base_id=k.id where b.account_id=$1::uuid and b.issuer_id=$2::uuid order by k.name,k.id limit 100`, sc.AccountID, sc.ResourceID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Base{}
	for rows.Next() {
		var v Base
		if e = rows.Scan(&v.ID, &v.Name, &v.Enabled, &v.Version); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) createBase(ctx context.Context, sc Scope, name string) (Base, error) {
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return Base{}, e
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var b Base
	e = tx.QueryRow(ctx, `insert into messaging.knowledge_bases(account_id,name,search_config)values($1::uuid,$2,$3::jsonb)returning id::text,name`, sc.AccountID, name, raw(map[string]string{"consumer": "accounting", "issuerId": sc.ResourceID})).Scan(&b.ID, &b.Name)
	if e != nil {
		return b, e
	}
	e = tx.QueryRow(ctx, `insert into accounting.knowledge_bindings(account_id,issuer_id,knowledge_base_id)values($1::uuid,$2::uuid,$3::uuid)returning version`, sc.AccountID, sc.ResourceID, b.ID).Scan(&b.Version)
	if e != nil {
		return b, e
	}
	return b, tx.Commit(ctx)
}
func (s *Store) assertBase(ctx context.Context, sc Scope, id string) error {
	var exists bool
	e := s.pool.QueryRow(ctx, `select exists(select 1 from accounting.knowledge_bindings where account_id=$1::uuid and issuer_id=$2::uuid and knowledge_base_id=$3::uuid)`, sc.AccountID, sc.ResourceID, id).Scan(&exists)
	if e == nil && !exists {
		return fail("not_found", 404)
	}
	return e
}
func (s *Store) setBase(ctx context.Context, sc Scope, id string, version int64, enabled bool) (Base, error) {
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return Base{}, e
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var b Base
	var current int64
	e = tx.QueryRow(ctx, `select b.version,k.name from accounting.knowledge_bindings b join messaging.knowledge_bases k on k.account_id=b.account_id and k.id=b.knowledge_base_id where b.account_id=$1::uuid and b.issuer_id=$2::uuid and b.knowledge_base_id=$3::uuid for update of b,k`, sc.AccountID, sc.ResourceID, id).Scan(&current, &b.Name)
	if e != nil {
		return b, fiscalError(e)
	}
	if current != version {
		return b, fail("version_conflict", 409)
	}
	_, e = tx.Exec(ctx, `update accounting.knowledge_bindings set is_enabled=$4,version=version+1,updated_at=now()where account_id=$1::uuid and issuer_id=$2::uuid and knowledge_base_id=$3::uuid`, sc.AccountID, sc.ResourceID, id, enabled)
	if e != nil {
		return b, e
	}
	_, e = tx.Exec(ctx, `update messaging.knowledge_bases set is_enabled=$3,updated_at=now()where account_id=$1::uuid and id=$2::uuid`, sc.AccountID, id, enabled)
	if e != nil {
		return b, e
	}
	b.ID = id
	b.Enabled = enabled
	b.Version = current + 1
	return b, tx.Commit(ctx)
}

const fiscalDocColumns = `d.id::text,d.knowledge_base_id::text,d.title,d.source_ref,d.checksum,d.status,d.version,a.version_lock,a.metadata,a.reviewed_by::text,(select count(*) from messaging.knowledge_chunks c where c.account_id=d.account_id and c.document_id=d.id)`
const fiscalDocFrom = ` from messaging.knowledge_documents d join messaging.knowledge_document_applicability a on a.account_id=d.account_id and a.document_id=d.id and a.version=d.version join accounting.knowledge_bindings b on b.account_id=d.account_id and b.knowledge_base_id=d.knowledge_base_id where d.account_id=$1::uuid and b.issuer_id=$2::uuid `

func scanFiscal(row rowScanner) (Document, error) {
	var d Document
	var m []byte
	e := row.Scan(&d.ID, &d.BaseID, &d.Title, &d.SourceRef, &d.Checksum, &d.Status, &d.Version, &d.LockVersion, &m, &d.ReviewedBy, &d.ChunkCount)
	if e != nil {
		return d, fiscalError(e)
	}
	e = json.Unmarshal(m, &d.Metadata)
	return d, e
}
func (s *Store) documents(ctx context.Context, sc Scope, base string, page, limit int) ([]Document, error) {
	if e := s.assertBase(ctx, sc, base); e != nil {
		return nil, e
	}
	rows, e := s.pool.Query(ctx, `select `+fiscalDocColumns+fiscalDocFrom+`and d.knowledge_base_id=$3::uuid order by d.created_at desc,d.id limit $4 offset $5`, sc.AccountID, sc.ResourceID, base, limit, (page-1)*limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Document{}
	for rows.Next() {
		d, e := scanFiscal(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
func (s *Store) duplicate(ctx context.Context, sc Scope, base, checksum string) (Document, bool, error) {
	d, e := scanFiscal(s.pool.QueryRow(ctx, `select `+fiscalDocColumns+fiscalDocFrom+`and d.knowledge_base_id=$3::uuid and d.checksum=$4 order by d.version desc limit 1`, sc.AccountID, sc.ResourceID, base, checksum))
	var domain *Error
	if errors.As(e, &domain) && domain.Status == 404 {
		return d, false, nil
	}
	return d, e == nil, e
}
func (s *Store) ingest(ctx context.Context, sc Scope, in Import, object, checksum, text string) (Document, error) {
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return Document{}, e
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var base string
	e = tx.QueryRow(ctx, `select knowledge_base_id::text from accounting.knowledge_bindings where account_id=$1::uuid and issuer_id=$2::uuid and knowledge_base_id=$3::uuid for update`, sc.AccountID, sc.ResourceID, in.BaseID).Scan(&base)
	if e != nil {
		return Document{}, fiscalError(e)
	}
	if in.ingestion != nil {
		var run string
		e = tx.QueryRow(ctx, `select r.id::text from messaging.knowledge_ingestion_runs r join messaging.knowledge_source_subscriptions s on s.account_id=r.account_id and s.id=r.subscription_id join accounting.knowledge_bindings b on b.account_id=s.account_id and b.issuer_id=s.issuer_id and b.knowledge_base_id=s.base_id join messaging.knowledge_bases k on k.account_id=b.account_id and k.id=b.knowledge_base_id where r.account_id=$1::uuid and r.issuer_id=$2::uuid and r.id=$3::uuid and r.fence=$4 and r.state='fetching' and s.version=$5 and s.enabled and b.is_enabled and k.is_enabled for update of s,r`, sc.AccountID, sc.ResourceID, in.ingestion.RunID, in.ingestion.Fence, in.ingestion.SubscriptionVersion).Scan(&run)
		if e != nil {
			return Document{}, fiscalError(e)
		}
	}

	old, e := scanFiscal(tx.QueryRow(ctx, `select `+fiscalDocColumns+fiscalDocFrom+`and d.knowledge_base_id=$3::uuid and d.checksum=$4 order by d.version desc limit 1`, sc.AccountID, sc.ResourceID, in.BaseID, checksum))
	if e == nil {
		return old, tx.Commit(ctx)
	}
	var domain *Error
	if !errors.As(e, &domain) || domain.Status != 404 {
		return Document{}, e
	}
	var version int
	e = tx.QueryRow(ctx, `select coalesce(max(version),0)+1 from messaging.knowledge_documents where account_id=$1::uuid and knowledge_base_id=$2::uuid and source_ref=$3`, sc.AccountID, in.BaseID, in.SourceRef).Scan(&version)
	if e != nil {
		return Document{}, e
	}
	var id string
	e = tx.QueryRow(ctx, `insert into messaging.knowledge_documents(account_id,knowledge_base_id,title,source_ref,checksum,version,metadata)values($1::uuid,$2::uuid,$3,$4,$5,$6,'{}')returning id::text`, sc.AccountID, in.BaseID, in.Title, in.SourceRef, checksum, version).Scan(&id)
	if e != nil {
		return Document{}, e
	}
	_, e = tx.Exec(ctx, `insert into messaging.knowledge_document_applicability(account_id,document_id,version,metadata,original_storage_id,checksum)values($1::uuid,$2::uuid,$3,$4::jsonb,$5::uuid,$6)`, sc.AccountID, id, version, raw(in.Metadata), object, checksum)
	if e != nil {
		return Document{}, e
	}
	runes := []rune(strings.ReplaceAll(text, "\r\n", "\n"))
	ordinal := 0
	for start := 0; start < len(runes); start += 1600 {
		end := min(start+1600, len(runes))
		body := strings.TrimSpace(string(runes[start:end]))
		if body == "" {
			continue
		}
		_, e = tx.Exec(ctx, `insert into messaging.knowledge_chunks(account_id,document_id,ordinal,body_text,token_count,location)values($1::uuid,$2::uuid,$3,$4,$5,$6::jsonb)`, sc.AccountID, id, ordinal, body, len(strings.Fields(body)), raw(map[string]any{"unit": "unicode_codepoint", "start": start, "end": end, "extraction": "plain_text_v1"}))
		if e != nil {
			return Document{}, e
		}
		ordinal++
	}
	out, e := scanFiscal(tx.QueryRow(ctx, `select `+fiscalDocColumns+fiscalDocFrom+`and d.id=$3::uuid`, sc.AccountID, sc.ResourceID, id))
	if e != nil {
		return out, e
	}
	return out, tx.Commit(ctx)
}
