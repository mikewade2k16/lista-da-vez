package knowledge

import (
	"context"
	"encoding/json"
	"strings"
)

func (s *Store) searchPublic(ctx context.Context, terms []string) ([]Evidence, error) {
	out := []Evidence{}
	concepts := publicConcepts(terms)
	if len(concepts) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx, `select p.id::text,p.version,p.title,p.excerpt,p.source_ref,p.metadata,coalesce((p.metadata->>'page')::int,0),coalesce(p.metadata->>'heading',''),
 c.id is not null,`+publicReviewSQL+`
 from knowledge.public_sources p left join knowledge.public_collections c on c.id=p.collection_id
 cross join lateral (select count(*) as matched from unnest($1::text[]) q where p.search_vector@@to_tsquery('simple',q)) m
 where p.status='published' and p.reviewed_at is not null and p.search_vector@@to_tsquery('simple',$2) and m.matched>=$3
 and not exists(select 1 from knowledge.public_sources n where n.source_key=p.source_key and n.status='published' and n.version>p.version)
 order by m.matched desc,ts_rank_cd(p.search_vector,to_tsquery('simple',$2),1) desc,p.source_key limit 3`, concepts, strings.Join(concepts, " | "), publicMinCoverage(len(concepts)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var v Evidence
		var raw []byte
		var page int
		var heading string
		var collected bool
		var review PublicReviewSummary
		if err = rows.Scan(&v.ChunkID, &v.Version, &v.Title, &v.Excerpt, &v.SourceRef, &raw, &page, &heading, &collected, &review.Owner, &review.AI, &review.AIConcerns, &review.Accountant); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &v.Metadata); err != nil {
			return nil, err
		}
		v.DocumentID = v.ChunkID
		v.ApplicabilityReason = "public_informational_not_personal_tax_advice"
		v.Location = json.RawMessage(`{"section":"trecho público revisado"}`)
		if collected {
			v.Review = &review
			v.Location = mustJSON(map[string]any{"section": heading, "page": page})
		}
		out = append(out, v)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
func (s *Store) publicReferenceCount(ctx context.Context, ids []string) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `select count(*) from knowledge.public_sources where id=any($1::uuid[]) and status='published' and reviewed_at is not null`, ids).Scan(&count)
	return count, err
}
