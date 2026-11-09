package knowledge

import (
	"context"
	"strings"
)

func (s *Store) SearchOmnichannel(ctx context.Context, accountID, agentID, query string, topK int, minScore float64) ([]KnowledgeSearchResult, error) {
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
		where c.account_id=$1::uuid and b.agent_id=$2::uuid and b.is_enabled and k.is_enabled and d.status='published' and k.search_config->>'consumer' is distinct from 'accounting'
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
