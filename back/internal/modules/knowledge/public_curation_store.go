package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

const publicTimeFormat = `'YYYY-MM-DD"T"HH24:MI:SS"Z"'`

// publicReviewSQL resume as revisões de uma coleta (alias c): última decisão do dono,
// contagem de IAs e última decisão do contador.
const publicReviewSQL = `coalesce((select r.verdict from knowledge.public_reviews r where r.collection_id=c.id and r.reviewer_kind='owner' order by r.created_at desc,r.id desc limit 1),'pending'),
 (select count(*) from knowledge.public_reviews r where r.collection_id=c.id and r.reviewer_kind='ai' and r.verdict='approved'),
 (select count(*) from knowledge.public_reviews r where r.collection_id=c.id and r.reviewer_kind='ai' and r.verdict<>'approved'),
 coalesce((select r.verdict from knowledge.public_reviews r where r.collection_id=c.id and r.reviewer_kind='accountant' order by r.created_at desc,r.id desc limit 1),'pending')`

const publicCollectionColumns = `c.id::text,c.catalog_key,c.version,c.title,c.source_ref,c.content_type,c.sha256,c.byte_size,c.pages,c.chunk_count,
 (select count(*) from knowledge.public_sources p where p.collection_id=c.id and p.status='withdrawn' and p.reviewed_at is null),
 c.status,to_char(c.collected_at at time zone 'UTC',` + publicTimeFormat + `),to_char(c.published_at at time zone 'UTC',` + publicTimeFormat + `),` + publicReviewSQL

func scanPublicCollection(row pgx.Row) (PublicCollection, error) {
	var c PublicCollection
	err := row.Scan(&c.ID, &c.CatalogKey, &c.Version, &c.Title, &c.SourceRef, &c.ContentType, &c.SHA256, &c.ByteSize, &c.Pages, &c.ChunkCount, &c.Excluded,
		&c.Status, &c.CollectedAt, &c.PublishedAt, &c.Review.Owner, &c.Review.AI, &c.Review.AIConcerns, &c.Review.Accountant)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, fail("not_found", 404)
	}
	return c, err
}

func (s *Store) recordPublicAttempt(ctx context.Context, key, outcome, code, collection, actor string) (PublicAttempt, error) {
	var a PublicAttempt
	err := s.pool.QueryRow(ctx, `insert into knowledge.public_collection_attempts(catalog_key,outcome,error_code,collection_id,actor_id)
 values($1,$2,nullif($3,''),nullif($4,'')::uuid,$5::uuid) returning outcome,coalesce(error_code,''),coalesce(collection_id::text,''),to_char(created_at at time zone 'UTC',`+publicTimeFormat+`)`,
		key, outcome, code, collection, actor).Scan(&a.Outcome, &a.ErrorCode, &a.CollectionID, &a.CreatedAt)
	return a, err
}

func (s *Store) latestPublicByHash(ctx context.Context, key, sha string) (string, bool, error) {
	var id string
	err := s.pool.QueryRow(ctx, `select id::text from knowledge.public_collections where catalog_key=$1 and sha256=$2 and status in ('collected','published') order by version desc limit 1`, key, sha).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	return id, err == nil, err
}

func (s *Store) insertPublicCollection(ctx context.Context, entry PublicCatalogEntry, actor, sha string, size int64, pages *int, chunks []publicChunk, now time.Time) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `select pg_advisory_xact_lock(hashtextextended('knowledge:public:'||$1,0))`, entry.Key); err != nil {
		return "", err
	}
	var id string
	var version int
	if err = tx.QueryRow(ctx, `insert into knowledge.public_collections(catalog_key,version,title,source_ref,content_type,sha256,byte_size,pages,chunk_count,collected_by,metadata)
 select $1,coalesce(max(version),0)+1,$2,$3,$4,$5,$6,$7,$8,$9::uuid,$10::jsonb from knowledge.public_collections where catalog_key=$1 returning id::text,version`,
		entry.Key, entry.Title, entry.URL, entry.ContentType, sha, size, pages, len(chunks), actor, mustJSON(map[string]string{"authority": entry.Authority, "notes": entry.Notes})).Scan(&id, &version); err != nil {
		return "", err
	}
	batch := &pgx.Batch{}
	for _, chunk := range chunks {
		title := entry.Title
		if chunk.Heading != "" {
			title += " — " + chunk.Heading
		}
		if chunk.Page > 0 {
			title += fmt.Sprintf(" (p. %d)", chunk.Page)
		}
		metadata := map[string]any{"authority": entry.Authority, "jurisdiction": entry.Jurisdiction, "regime": entry.Regime, "classification": "official_document_excerpt",
			"publishedOn": nil, "validFrom": nil, "validTo": nil, "validitySource": "", "collectionId": id, "chunkIndex": chunk.Index, "page": chunk.Page,
			"heading": chunk.Heading, "fetchedAt": now.Format(time.RFC3339), "sha256": sha}
		batch.Queue(`insert into knowledge.public_sources(source_key,version,title,source_ref,excerpt,metadata,status,collection_id,chunk_index) values($1,$2,$3,$4,$5,$6::jsonb,'draft',$7::uuid,$8)`,
			fmt.Sprintf("%s:%04d", entry.Key, chunk.Index), version, truncateRunes(title, 500), entry.URL, chunk.Text, mustJSON(metadata), id, chunk.Index)
	}
	if err = tx.SendBatch(ctx, batch).Close(); err != nil {
		return "", err
	}
	return id, tx.Commit(ctx)
}

func mustJSON(v any) []byte {
	raw, _ := json.Marshal(v)
	return raw
}

func (s *Store) publicCollection(ctx context.Context, id string) (PublicCollection, error) {
	if !idPattern.MatchString(id) {
		return PublicCollection{}, fail("not_found", 404)
	}
	return scanPublicCollection(s.pool.QueryRow(ctx, `select `+publicCollectionColumns+` from knowledge.public_collections c where c.id=$1::uuid`, id))
}

func (s *Store) publicCatalogStatus(ctx context.Context, entries []PublicCatalogEntry) ([]PublicCatalogStatus, error) {
	out := make([]PublicCatalogStatus, 0, len(entries))
	keys := make([]string, 0, len(entries))
	index := map[string]int{}
	for i, entry := range entries {
		out = append(out, PublicCatalogStatus{Entry: entry})
		keys = append(keys, entry.Key)
		index[entry.Key] = i
	}
	rows, err := s.pool.Query(ctx, `select distinct on (catalog_key) catalog_key,outcome,coalesce(error_code,''),coalesce(collection_id::text,''),to_char(created_at at time zone 'UTC',`+publicTimeFormat+`)
 from knowledge.public_collection_attempts where catalog_key=any($1) order by catalog_key,created_at desc,id desc`, keys)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var key string
		var a PublicAttempt
		if err = rows.Scan(&key, &a.Outcome, &a.ErrorCode, &a.CollectionID, &a.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		out[index[key]].LastAttempt = &a
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	rows, err = s.pool.Query(ctx, `select `+publicCollectionColumns+`,(c.status='published') from knowledge.public_collections c
 where c.catalog_key=any($1) and (c.status='published' or c.version=(select max(x.version) from knowledge.public_collections x where x.catalog_key=c.catalog_key))`, keys)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var c PublicCollection
		var published bool
		if err = rows.Scan(&c.ID, &c.CatalogKey, &c.Version, &c.Title, &c.SourceRef, &c.ContentType, &c.SHA256, &c.ByteSize, &c.Pages, &c.ChunkCount, &c.Excluded,
			&c.Status, &c.CollectedAt, &c.PublishedAt, &c.Review.Owner, &c.Review.AI, &c.Review.AIConcerns, &c.Review.Accountant, &published); err != nil {
			return nil, err
		}
		item := &out[index[c.CatalogKey]]
		copied := c
		if published {
			item.Published = &copied
		}
		if item.Latest == nil || c.Version > item.Latest.Version {
			latest := c
			item.Latest = &latest
		}
	}
	return out, rows.Err()
}

func (s *Store) publicChunks(ctx context.Context, collection string, page, limit int) ([]PublicChunk, error) {
	rows, err := s.pool.Query(ctx, `select p.id::text,p.chunk_index,coalesce((p.metadata->>'page')::int,0),coalesce(p.metadata->>'heading',''),p.title,p.excerpt,p.status
 from knowledge.public_sources p where p.collection_id=$1::uuid order by p.chunk_index limit $2 offset $3`, collection, limit, (page-1)*limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (PublicChunk, error) {
		var c PublicChunk
		return c, r.Scan(&c.ID, &c.Index, &c.Page, &c.Heading, &c.Title, &c.Text, &c.Status)
	})
}

func (s *Store) excludePublicChunk(ctx context.Context, collection, chunk string) error {
	tag, err := s.pool.Exec(ctx, `update knowledge.public_sources p set status='withdrawn' where p.id=$2::uuid and p.collection_id=$1::uuid and p.status='draft'
 and exists(select 1 from knowledge.public_collections c where c.id=$1::uuid and c.status='collected')`, collection, chunk)
	if err == nil && tag.RowsAffected() == 0 {
		return fail("not_found", 404)
	}
	return err
}

func (s *Store) insertPublicReview(ctx context.Context, actor, collection string, in PublicReviewInput) (PublicReview, error) {
	var r PublicReview
	err := s.pool.QueryRow(ctx, `insert into knowledge.public_reviews(collection_id,reviewer_kind,reviewer_name,verdict,notes,actor_id)
 select c.id,$2,$3,$4,$5,$6::uuid from knowledge.public_collections c where c.id=$1::uuid and c.status in ('collected','published')
 returning id::text,reviewer_kind,reviewer_name,verdict,notes,actor_id::text,to_char(created_at at time zone 'UTC',`+publicTimeFormat+`)`,
		collection, in.ReviewerKind, in.ReviewerName, in.Verdict, in.Notes, actor).Scan(&r.ID, &r.ReviewerKind, &r.ReviewerName, &r.Verdict, &r.Notes, &r.ActorID, &r.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, fail("not_found", 404)
	}
	return r, err
}

func (s *Store) publicReviews(ctx context.Context, collection string) ([]PublicReview, error) {
	rows, err := s.pool.Query(ctx, `select id::text,reviewer_kind,reviewer_name,verdict,notes,actor_id::text,to_char(created_at at time zone 'UTC',`+publicTimeFormat+`)
 from knowledge.public_reviews where collection_id=$1::uuid order by created_at desc,id desc limit 200`, collection)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (PublicReview, error) {
		var v PublicReview
		return v, r.Scan(&v.ID, &v.ReviewerKind, &v.ReviewerName, &v.Verdict, &v.Notes, &v.ActorID, &v.CreatedAt)
	})
}

func (s *Store) publishPublicCollection(ctx context.Context, actor, collection string) (PublicCollection, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PublicCollection{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var key, status string
	if err = tx.QueryRow(ctx, `select catalog_key,status from knowledge.public_collections where id=$1::uuid for update`, collection).Scan(&key, &status); errors.Is(err, pgx.ErrNoRows) {
		return PublicCollection{}, fail("not_found", 404)
	} else if err != nil {
		return PublicCollection{}, err
	}
	if _, err = tx.Exec(ctx, `select pg_advisory_xact_lock(hashtextextended('knowledge:public:'||$1,0))`, key); err != nil {
		return PublicCollection{}, err
	}
	if status != "collected" {
		return PublicCollection{}, fail("collection_not_publishable", 409)
	}
	var owner string
	var drafts int
	if err = tx.QueryRow(ctx, `select coalesce((select verdict from knowledge.public_reviews where collection_id=$1::uuid and reviewer_kind='owner' order by created_at desc,id desc limit 1),'pending'),
 (select count(*) from knowledge.public_sources where collection_id=$1::uuid and status='draft')`, collection).Scan(&owner, &drafts); err != nil {
		return PublicCollection{}, err
	}
	if owner != "approved" {
		return PublicCollection{}, fail("owner_approval_required", 409)
	}
	if drafts == 0 {
		return PublicCollection{}, fail("collection_empty", 409)
	}
	steps := []struct {
		sql  string
		args []any
	}{
		{`update knowledge.public_sources set status='withdrawn' where status='published' and collection_id in (select id from knowledge.public_collections where catalog_key=$1 and status='published' and id<>$2::uuid)`, []any{key, collection}},
		{`update knowledge.public_collections set status='superseded' where catalog_key=$1 and status='published' and id<>$2::uuid`, []any{key, collection}},
		{`update knowledge.public_sources set status='published',reviewed_at=clock_timestamp() where collection_id=$1::uuid and status='draft'`, []any{collection}},
		{`update knowledge.public_collections set status='published',published_by=$2::uuid,published_at=clock_timestamp() where id=$1::uuid`, []any{collection, actor}},
	}
	for _, step := range steps {
		if _, err = tx.Exec(ctx, step.sql, step.args...); err != nil {
			return PublicCollection{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return PublicCollection{}, err
	}
	return s.publicCollection(ctx, collection)
}

func (s *Store) discardPublicCollection(ctx context.Context, collection string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tag, err := tx.Exec(ctx, `update knowledge.public_collections set status='discarded' where id=$1::uuid and status='collected'`, collection)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fail("collection_not_discardable", 409)
	}
	if _, err = tx.Exec(ctx, `update knowledge.public_sources set status='withdrawn' where collection_id=$1::uuid and status='draft'`, collection); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
