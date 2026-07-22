package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

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
		where account_id=$1::uuid order by lower(name), id`, accountID)
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
		where account_id=$1::uuid and id=$2::uuid returning `+knowledgeBaseColumns,
		accountID, id, patch.Name, patch.IsEnabled, nullableRaw(patch.SearchConfig)))
}

func (s *Store) GetKnowledgeBase(ctx context.Context, accountID, id string) (KnowledgeBaseView, error) {
	item, err := scanKnowledgeBase(s.pool.QueryRow(ctx, `select `+knowledgeBaseColumns+`
		from messaging.knowledge_bases where account_id=$1::uuid and id=$2::uuid`, accountID, id))
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
	item, err := scanKnowledgeDocument(s.pool.QueryRow(ctx, `select `+knowledgeDocumentColumns+` from messaging.knowledge_documents d
		where d.account_id=$1::uuid and d.knowledge_base_id=$2::uuid and d.id=$3::uuid`, accountID, baseID, documentID))
	if errors.Is(err, pgx.ErrNoRows) {
		return KnowledgeDocumentView{}, ErrNotFound
	}
	return item, err
}

func (s *Store) CreateKnowledgeDocument(ctx context.Context, accountID, baseID string, in KnowledgeDocumentInput) (KnowledgeDocumentView, error) {
	return scanKnowledgeDocument(s.pool.QueryRow(ctx, `insert into messaging.knowledge_documents
		(account_id,knowledge_base_id,source_ref,title,checksum,version,metadata)
		values ($1::uuid,$2::uuid,$3,$4,$5,$6,$7::jsonb) returning `+knowledgeDocumentColumns,
		accountID, baseID, in.SourceRef, in.Title, in.Checksum, in.Version, in.Metadata))
}

func (s *Store) UpdateKnowledgeDocument(ctx context.Context, accountID, baseID, id string, patch KnowledgeDocumentPatch) (KnowledgeDocumentView, error) {
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

const knowledgeBindingColumns = `b.id::text, b.agent_id::text, b.knowledge_base_id::text, k.name,
	b.is_enabled, b.top_k, b.min_score::float8, b.created_at, b.updated_at`

func scanKnowledgeBinding(row rowScanner) (AIKnowledgeBindingView, error) {
	var out AIKnowledgeBindingView
	if err := row.Scan(&out.ID, &out.AgentID, &out.KnowledgeBaseID, &out.BaseName, &out.IsEnabled,
		&out.TopK, &out.MinScore, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return AIKnowledgeBindingView{}, err
	}
	return out, nil
}

func (s *Store) ListAIKnowledgeBindings(ctx context.Context, accountID, agentID string) ([]AIKnowledgeBindingView, error) {
	rows, err := s.pool.Query(ctx, `select `+knowledgeBindingColumns+` from messaging.ai_knowledge_bindings b
		join messaging.knowledge_bases k on k.account_id=b.account_id and k.id=b.knowledge_base_id
		where b.account_id=$1::uuid and b.agent_id=$2::uuid order by lower(k.name), b.id`, accountID, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AIKnowledgeBindingView, 0)
	for rows.Next() {
		item, err := scanKnowledgeBinding(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) CreateAIKnowledgeBinding(ctx context.Context, accountID, agentID string, in AIKnowledgeBindingInput, enabled bool) (AIKnowledgeBindingView, error) {
	return scanKnowledgeBinding(s.pool.QueryRow(ctx, `insert into messaging.ai_knowledge_bindings
		(account_id,agent_id,knowledge_base_id,is_enabled,top_k,min_score)
		values ($1::uuid,$2::uuid,$3::uuid,$4,$5,$6) returning `+knowledgeBindingColumns,
		accountID, agentID, in.KnowledgeBaseID, enabled, in.TopK, in.MinScore))
}

func (s *Store) UpdateAIKnowledgeBinding(ctx context.Context, accountID, agentID, id string, patch AIKnowledgeBindingPatch) (AIKnowledgeBindingView, error) {
	return scanKnowledgeBinding(s.pool.QueryRow(ctx, `update messaging.ai_knowledge_bindings b set
		is_enabled=coalesce($4,is_enabled), top_k=coalesce($5,top_k), min_score=coalesce($6,min_score), updated_at=now()
		where b.account_id=$1::uuid and b.agent_id=$2::uuid and b.id=$3::uuid returning `+knowledgeBindingColumns,
		accountID, agentID, id, patch.IsEnabled, patch.TopK, patch.MinScore))
}

func (s *Store) DisableAIKnowledgeBinding(ctx context.Context, accountID, agentID, id string) error {
	tag, err := s.pool.Exec(ctx, `update messaging.ai_knowledge_bindings set is_enabled=false, updated_at=now()
		where account_id=$1::uuid and agent_id=$2::uuid and id=$3::uuid`, accountID, agentID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SearchKnowledge(ctx context.Context, accountID, agentID, query string, topK int, minScore float64) ([]KnowledgeSearchResult, error) {
	rows, err := s.pool.Query(ctx, `with q as (select plainto_tsquery('simple',$3) query), candidates as (
		select d.id::text document_id, d.title, c.id::text chunk_id,
			left(regexp_replace(c.body_text, '[[:space:]]+', ' ', 'g'), 2000) excerpt,
			ts_rank_cd(c.search_vector,q.query)::float8 score, d.source_ref, d.version,
			d.knowledge_base_id, b.top_k, b.min_score::float8 binding_min_score,
			row_number() over (partition by d.knowledge_base_id order by ts_rank_cd(c.search_vector,q.query) desc, d.version desc, c.ordinal) position
		from q
		join messaging.knowledge_chunks c on c.search_vector @@ q.query
		join messaging.knowledge_documents d on d.account_id=c.account_id and d.id=c.document_id
		join messaging.knowledge_bases k on k.account_id=d.account_id and k.id=d.knowledge_base_id
		join messaging.ai_knowledge_bindings b on b.account_id=k.account_id and b.knowledge_base_id=k.id
		where c.account_id=$1::uuid and b.agent_id=$2::uuid and b.is_enabled and k.is_enabled and d.status='published'
	)
	select document_id, title, chunk_id, excerpt, score, source_ref, version
	from candidates
	where score >= greatest($4, binding_min_score) and position <= least($5, top_k)
	order by score desc, version desc, chunk_id
	limit $5`, accountID, agentID, strings.TrimSpace(query), minScore, topK)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]KnowledgeSearchResult, 0)
	for rows.Next() {
		var item KnowledgeSearchResult
		if err := rows.Scan(&item.DocumentID, &item.Title, &item.ChunkID, &item.Excerpt, &item.Score, &item.SourceRef, &item.Version); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
