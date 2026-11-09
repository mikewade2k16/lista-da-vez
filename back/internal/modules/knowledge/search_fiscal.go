package knowledge

import (
	"context"
	"encoding/json"
)

const fiscalSearchSQL = `with query as(select plainto_tsquery('simple',$3) term)
 select d.id::text,d.version,c.id::text,d.title,left(c.body_text,1600),d.source_ref,ts_rank_cd(c.search_vector,query.term)::float8,c.location,a.metadata,
 ($4<>'' and $5<>'' and $6<>'' and a.metadata->>'validFrom' is not null and a.metadata->>'validitySource'<>'' and a.metadata->>'jurisdiction'=$5 and a.metadata->>'regime'=$6) applicable
 from query join messaging.knowledge_chunks c on c.search_vector@@query.term
 join messaging.knowledge_documents d on d.account_id=c.account_id and d.id=c.document_id
 join messaging.knowledge_bases k on k.account_id=d.account_id and k.id=d.knowledge_base_id
 join accounting.knowledge_bindings b on b.account_id=k.account_id and b.knowledge_base_id=k.id
 join messaging.knowledge_document_applicability a on a.account_id=d.account_id and a.document_id=d.id and a.version=d.version
 where c.account_id=$1::uuid and b.issuer_id=$2::uuid and b.is_enabled and k.is_enabled and d.status='published' and a.status='reviewed'
 and ($5='' or a.metadata->>'jurisdiction'=$5 or a.metadata->>'jurisdiction'='unknown')
 and ($6='' or a.metadata->>'regime'=$6 or coalesce(a.metadata->>'regime','') in ('','unknown'))
 and ($4='' or a.metadata->>'validFrom' is null or a.metadata->>'validFrom'<=$4)
 and ($4='' or a.metadata->>'validTo' is null or a.metadata->>'validTo'>$4)
 order by ts_rank_cd(c.search_vector,query.term) desc,d.version desc,c.ordinal,c.id limit $7 offset $8`

func (s *Store) searchFiscal(ctx context.Context, sc Scope, q Query) ([]Evidence, error) {
	rows, e := s.pool.Query(ctx, fiscalSearchSQL, sc.AccountID, sc.ResourceID, q.Query, q.CompetenceDate, q.Jurisdiction, q.Regime, q.TopK, (q.Page-1)*q.TopK)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Evidence{}
	for rows.Next() {
		var v Evidence
		var metadata []byte
		var applicable *bool
		if e = rows.Scan(&v.DocumentID, &v.Version, &v.ChunkID, &v.Title, &v.Excerpt, &v.SourceRef, &v.Score, &v.Location, &metadata, &applicable); e != nil {
			return nil, e
		}
		if e = json.Unmarshal(metadata, &v.Metadata); e != nil {
			return nil, e
		}
		v.Applicable = q.ContextConfirmed && applicable != nil && *applicable
		v.ApplicabilityReason = "informational_unverified_context_or_validity"
		if v.Applicable {
			v.ApplicabilityReason = "reviewed_metadata_matches_explicit_context"
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Service) Resolve(ctx context.Context, sc Scope, document, chunk string) (Evidence, error) {
	var out Evidence
	if e := s.authorize(ctx, sc, false); e != nil {
		return out, e
	}
	if !idPattern.MatchString(document) || !idPattern.MatchString(chunk) {
		return out, fail("not_found", 404)
	}
	out, e := s.store.resolve(ctx, sc, document, chunk)
	if e != nil {
		return out, e
	}
	if e = s.authorize(ctx, sc, false); e != nil {
		return Evidence{}, e
	}
	return out, nil
}
func (s *Store) resolve(ctx context.Context, sc Scope, document, chunk string) (Evidence, error) {
	var v Evidence
	var metadata []byte
	e := s.pool.QueryRow(ctx, `select d.id::text,d.version,c.id::text,d.title,left(c.body_text,1600),d.source_ref,c.location,a.metadata
 from messaging.knowledge_chunks c join messaging.knowledge_documents d on d.account_id=c.account_id and d.id=c.document_id
 join messaging.knowledge_bases k on k.account_id=d.account_id and k.id=d.knowledge_base_id
 join accounting.knowledge_bindings b on b.account_id=k.account_id and b.knowledge_base_id=k.id
 join messaging.knowledge_document_applicability a on a.account_id=d.account_id and a.document_id=d.id and a.version=d.version
 where c.account_id=$1::uuid and b.issuer_id=$2::uuid and d.id=$3::uuid and c.id=$4::uuid and b.is_enabled and k.is_enabled and d.status='published' and a.status='reviewed'`, sc.AccountID, sc.ResourceID, document, chunk).Scan(&v.DocumentID, &v.Version, &v.ChunkID, &v.Title, &v.Excerpt, &v.SourceRef, &v.Location, &metadata)
	if e != nil {
		return v, fiscalError(e)
	}
	e = json.Unmarshal(metadata, &v.Metadata)
	v.ApplicabilityReason = "reference_only_context_requires_revalidation"
	return v, e
}
