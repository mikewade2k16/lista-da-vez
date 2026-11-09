package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jackc/pgx/v5"
)

const knowledgeBaseColumns = `id::text, name, is_enabled, search_config, created_at, updated_at`

func scanKnowledgeBase(row rowScanner) (KnowledgeBaseView, error) {
	var out KnowledgeBaseView
	if err := row.Scan(&out.ID, &out.Name, &out.IsEnabled, &out.SearchConfig, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return KnowledgeBaseView{}, err
	}
	out.SearchConfig = jsonOrEmpty(out.SearchConfig)
	return out, nil
}

func (s *Store) ListKnowledgeBases(ctx context.Context, accountID string) ([]KnowledgeBaseView, error) {
	rows, err := s.pool.Query(ctx, `select `+knowledgeBaseColumns+` from messaging.knowledge_bases
		where account_id=$1::uuid and search_config->>'consumer' is distinct from 'accounting' order by lower(name), id`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]KnowledgeBaseView, 0)
	for rows.Next() {
		item, err := scanKnowledgeBase(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) CreateKnowledgeBase(ctx context.Context, accountID, name string, enabled bool, config json.RawMessage) (KnowledgeBaseView, error) {
	return scanKnowledgeBase(s.pool.QueryRow(ctx, `insert into messaging.knowledge_bases
		(account_id,name,is_enabled,search_config) values ($1::uuid,$2,$3,$4::jsonb)
		returning `+knowledgeBaseColumns, accountID, name, enabled, config))
}

func (s *Store) UpdateKnowledgeBase(ctx context.Context, accountID, id string, patch KnowledgeBasePatch) (KnowledgeBaseView, error) {
	return scanKnowledgeBase(s.pool.QueryRow(ctx, `update messaging.knowledge_bases set
		name=coalesce($3,name), is_enabled=coalesce($4,is_enabled),
		search_config=coalesce($5::jsonb,search_config), updated_at=now()
		where account_id=$1::uuid and id=$2::uuid and search_config->>'consumer' is distinct from 'accounting' returning `+knowledgeBaseColumns,
		accountID, id, patch.Name, patch.IsEnabled, nullableRaw(patch.SearchConfig)))
}

func (s *Store) GetKnowledgeBase(ctx context.Context, accountID, id string) (KnowledgeBaseView, error) {
	item, err := scanKnowledgeBase(s.pool.QueryRow(ctx, `select `+knowledgeBaseColumns+`
		from messaging.knowledge_bases where account_id=$1::uuid and id=$2::uuid and search_config->>'consumer' is distinct from 'accounting'`, accountID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return KnowledgeBaseView{}, ErrNotFound
	}
	return item, err
}

const knowledgeDocumentColumns = `d.id::text, d.knowledge_base_id::text, d.source_ref, d.title, d.checksum,
	d.status, d.version, (select count(*) from messaging.knowledge_chunks c where c.account_id=d.account_id and c.document_id=d.id),
	d.metadata, d.error, d.created_at, d.updated_at`

func scanKnowledgeDocument(row rowScanner) (KnowledgeDocumentView, error) {
	var out KnowledgeDocumentView
	if err := row.Scan(&out.ID, &out.KnowledgeBaseID, &out.SourceRef, &out.Title, &out.Checksum, &out.Status,
		&out.Version, &out.ChunkCount, &out.Metadata, &out.Error, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return KnowledgeDocumentView{}, err
	}
	out.Metadata = jsonOrEmpty(out.Metadata)
	return out, nil
}

func (s *Store) ListKnowledgeDocuments(ctx context.Context, accountID, baseID string) ([]KnowledgeDocumentView, error) {
	if _, err := s.GetKnowledgeBase(ctx, accountID, baseID); err != nil {
		return nil, err
	}

	rows, err := s.pool.Query(ctx, `select `+knowledgeDocumentColumns+` from messaging.knowledge_documents d
		where d.account_id=$1::uuid and d.knowledge_base_id=$2::uuid order by d.version desc, d.created_at desc, d.id`, accountID, baseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]KnowledgeDocumentView, 0)
	for rows.Next() {
		item, err := scanKnowledgeDocument(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) GetKnowledgeDocument(ctx context.Context, accountID, baseID, documentID string) (KnowledgeDocumentView, error) {
	if _, err := s.GetKnowledgeBase(ctx, accountID, baseID); err != nil {
		return KnowledgeDocumentView{}, err
	}

	item, err := scanKnowledgeDocument(s.pool.QueryRow(ctx, `select `+knowledgeDocumentColumns+` from messaging.knowledge_documents d
		where d.account_id=$1::uuid and d.knowledge_base_id=$2::uuid and d.id=$3::uuid`, accountID, baseID, documentID))
	if errors.Is(err, pgx.ErrNoRows) {
		return KnowledgeDocumentView{}, ErrNotFound
	}
	return item, err
}

func (s *Store) CreateKnowledgeDocument(ctx context.Context, accountID, baseID string, in KnowledgeDocumentInput) (KnowledgeDocumentView, error) {
	if _, err := s.GetKnowledgeBase(ctx, accountID, baseID); err != nil {
		return KnowledgeDocumentView{}, err
	}

	return scanKnowledgeDocument(s.pool.QueryRow(ctx, `insert into messaging.knowledge_documents as d
		(account_id,knowledge_base_id,source_ref,title,checksum,version,metadata)
		values ($1::uuid,$2::uuid,$3,$4,$5,$6,$7::jsonb) returning `+knowledgeDocumentColumns,
		accountID, baseID, in.SourceRef, in.Title, in.Checksum, in.Version, in.Metadata))
}

func (s *Store) UpdateKnowledgeDocument(ctx context.Context, accountID, baseID, id string, patch KnowledgeDocumentPatch) (KnowledgeDocumentView, error) {
	current, e := s.GetKnowledgeDocument(ctx, accountID, baseID, id)
	if e != nil {
		return KnowledgeDocumentView{}, e
	}
	if current.Status == "published" || current.Status == "archived" {
		if patch.Title != nil || patch.Metadata != nil || patch.Error != nil || (patch.Status != nil && *patch.Status != "archived" && *patch.Status != current.Status) {
			return KnowledgeDocumentView{}, ErrConflict
		}
	}

	if _, err := s.GetKnowledgeBase(ctx, accountID, baseID); err != nil {
		return KnowledgeDocumentView{}, err
	}

	return scanKnowledgeDocument(s.pool.QueryRow(ctx, `update messaging.knowledge_documents d set
		title=coalesce($4,title), status=coalesce($5,status), metadata=coalesce($6::jsonb,metadata),
		error=coalesce($7,error), updated_at=now()
		where d.account_id=$1::uuid and d.knowledge_base_id=$2::uuid and d.id=$3::uuid returning `+knowledgeDocumentColumns,
		accountID, baseID, id, patch.Title, patch.Status, nullableRaw(patch.Metadata), patch.Error))
}

func (s *Store) HasKnowledgeChunks(ctx context.Context, accountID, documentID string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `select exists(select 1 from messaging.knowledge_chunks
		where account_id=$1::uuid and document_id=$2::uuid)`, accountID, documentID).Scan(&exists)
	return exists, err
}

func (s *Store) ReplaceKnowledgeChunks(ctx context.Context, accountID, baseID, documentID string, chunks []KnowledgeChunkInput) error {
	if _, err := s.GetKnowledgeBase(ctx, accountID, baseID); err != nil {
		return err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var status string
	err = tx.QueryRow(ctx, `select status from messaging.knowledge_documents
		where account_id=$1::uuid and knowledge_base_id=$2::uuid and id=$3::uuid for update`, accountID, baseID, documentID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if status == "published" || status == "archived" {
		return ErrConflict
	}
	if _, err := tx.Exec(ctx, `delete from messaging.knowledge_chunks where account_id=$1::uuid and document_id=$2::uuid`, accountID, documentID); err != nil {
		return err
	}
	for _, chunk := range chunks {
		if _, err := tx.Exec(ctx, `insert into messaging.knowledge_chunks
			(account_id,document_id,ordinal,body_text,token_count) values ($1::uuid,$2::uuid,$3,$4,$5)`,
			accountID, documentID, chunk.Ordinal, chunk.BodyText, chunk.TokenCount); err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `update messaging.knowledge_documents set status='processing', error='', updated_at=now()
		where account_id=$1::uuid and knowledge_base_id=$2::uuid and id=$3::uuid`, accountID, baseID, documentID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type Store struct{ pool *pgxpool.Pool }

func NewStore(p *pgxpool.Pool) *Store { return &Store{pool: p} }

type rowScanner interface{ Scan(...any) error }

var ErrNotFound = errors.New("knowledge not found")
var ErrConflict = errors.New("knowledge conflict")

func jsonOrEmpty(v json.RawMessage) json.RawMessage {
	if len(v) == 0 {
		return json.RawMessage(`{}`)
	}
	return v
}
func nullableRaw(v *json.RawMessage) any {
	if v == nil {
		return nil
	}
	return *v
}
