package omnichannel

import (
	"context"
)

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
