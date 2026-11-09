package finance

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DocumentStore struct{ pool *pgxpool.Pool }

func scanDocument(row pgx.Row) (DocumentDraft, error) {
	var d DocumentDraft
	var data []byte
	var updated time.Time
	err := row.Scan(&d.ID, &d.SourceName, &d.ExtractedText, &data, &updated)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, ErrFiscalNotFound
	}
	if err != nil {
		return d, err
	}
	if err = json.Unmarshal(data, &d.Fields); err != nil {
		return d, err
	}
	d.UpdatedAt = updated.UTC().Format(time.RFC3339Nano)
	return documentView(d), nil
}

const documentColumns = `id::text,source_name,extracted_text,data,updated_at`

func (s *DocumentStore) Get(ctx context.Context, account, user, id string) (DocumentDraft, error) {
	return scanDocument(s.pool.QueryRow(ctx, `select `+documentColumns+` from finance.document_drafts where account_id=$1::uuid and user_id=$2::uuid and id=$3::uuid`, account, user, id))
}
func (s *DocumentStore) ByHash(ctx context.Context, account, user, hash string) (DocumentDraft, error) {
	return scanDocument(s.pool.QueryRow(ctx, `select `+documentColumns+` from finance.document_drafts where account_id=$1::uuid and user_id=$2::uuid and source_hash=$3`, account, user, hash))
}
func (s *DocumentStore) List(ctx context.Context, account, user string) ([]DocumentDraft, error) {
	rows, err := s.pool.Query(ctx, `select `+documentColumns+` from finance.document_drafts where account_id=$1::uuid and user_id=$2::uuid order by updated_at desc,id desc limit 10`, account, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DocumentDraft{}
	for rows.Next() {
		d, e := scanDocument(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
func (s *DocumentStore) Create(ctx context.Context, account, user, hash, name, text string, fields DocumentFields) (DocumentDraft, error) {
	data, err := json.Marshal(fields)
	if err != nil {
		return DocumentDraft{}, err
	}
	// A repeated image must not overwrite manual corrections or create duplicate drafts.
	d, err := scanDocument(s.pool.QueryRow(ctx, `insert into finance.document_drafts(account_id,user_id,source_hash,source_name,extracted_text,data) values($1::uuid,$2::uuid,$3,$4,$5,$6::jsonb) on conflict(account_id,user_id,source_hash) do nothing returning `+documentColumns, account, user, hash, name, text, data))
	if errors.Is(err, ErrFiscalNotFound) {
		return s.ByHash(ctx, account, user, hash)
	}
	return d, err
}
func (s *DocumentStore) Save(ctx context.Context, account, user, id string, in DocumentDraftInput) (DocumentDraft, error) {
	return s.saveExtraction(ctx, account, user, id, in, nil)
}

func (s *DocumentStore) saveExtraction(ctx context.Context, account, user, id string, in DocumentDraftInput, text *string) (DocumentDraft, error) {
	data, err := json.Marshal(in.Fields)
	if err != nil {
		return DocumentDraft{}, err
	}
	d, err := scanDocument(s.pool.QueryRow(ctx, `update finance.document_drafts set data=$4::jsonb,extracted_text=coalesce($6,extracted_text),updated_at=clock_timestamp() where account_id=$1::uuid and user_id=$2::uuid and id=$3::uuid and updated_at=$5::timestamptz returning `+documentColumns, account, user, id, data, in.ExpectedUpdatedAt, text))
	if errors.Is(err, ErrFiscalNotFound) {
		if _, e := s.Get(ctx, account, user, id); e != nil {
			return d, e
		}
		return d, ErrFiscalConflict
	}
	return d, err
}
