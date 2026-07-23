package socialpublishing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *Store) Overview(ctx context.Context, accountID string, now time.Time) (Overview, error) {
	overview := Overview{
		Counts:   map[string]int64{},
		Upcoming: []Post{},
	}
	connection, err := s.GetConnection(ctx, accountID)
	if err == nil {
		view := connection.Connection
		overview.Connection = &view
	} else if !errors.Is(err, ErrNotConnected) {
		return Overview{}, err
	}

	const countsQuery = `
		select status, count(*)
		from social_publishing.posts
		where account_id = $1::uuid
		group by status`
	rows, err := s.pool.Query(ctx, countsQuery, accountID)
	if err != nil {
		return Overview{}, fmt.Errorf("social publishing: contar posts: %w", err)
	}
	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			rows.Close()
			return Overview{}, fmt.Errorf("social publishing: ler contagem: %w", err)
		}
		overview.Counts[status] = count
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Overview{}, fmt.Errorf("social publishing: iterar contagem: %w", err)
	}
	rows.Close()

	const upcomingQuery = `
		select ` + postSelectColumns + `
		from social_publishing.posts p
		where p.account_id = $1::uuid
		  and p.status = 'scheduled'
		  and p.scheduled_for >= $2
		order by p.scheduled_for, p.id
		limit 10`
	upcomingRows, err := s.pool.Query(ctx, upcomingQuery, accountID, now)
	if err != nil {
		return Overview{}, fmt.Errorf("social publishing: listar proximos: %w", err)
	}
	defer upcomingRows.Close()
	for upcomingRows.Next() {
		post, scanErr := scanPost(upcomingRows)
		if scanErr != nil {
			return Overview{}, fmt.Errorf("social publishing: ler proximo: %w", scanErr)
		}
		overview.Upcoming = append(overview.Upcoming, post)
	}
	if err := upcomingRows.Err(); err != nil {
		return Overview{}, fmt.Errorf("social publishing: iterar proximos: %w", err)
	}

	const analyticsQuery = `
		select coalesce(sum(views), 0), coalesce(sum(reach), 0),
		       coalesce(sum(likes), 0), coalesce(sum(comments), 0),
		       coalesce(sum(saved), 0), coalesce(sum(shares), 0),
		       coalesce(sum(total_interactions), 0), max(captured_at)
		from social_publishing.post_analytics
		where account_id = $1::uuid`
	var capturedAt *time.Time
	err = s.pool.QueryRow(ctx, analyticsQuery, accountID).Scan(
		&overview.Analytics.Views,
		&overview.Analytics.Reach,
		&overview.Analytics.Likes,
		&overview.Analytics.Comments,
		&overview.Analytics.Saved,
		&overview.Analytics.Shares,
		&overview.Analytics.TotalInteractions,
		&capturedAt,
	)
	if err != nil {
		return Overview{}, fmt.Errorf("social publishing: agregar analytics: %w", err)
	}
	if capturedAt != nil {
		overview.Analytics.CapturedAt = capturedAt.UTC()
	}
	return overview, nil
}

func (s *Store) ListAnalytics(
	ctx context.Context,
	accountID string,
	limit int,
) ([]Analytics, error) {
	const query = `
		select post_id::text, views, reach, likes, comments, saved, shares,
		       total_interactions, captured_at
		from social_publishing.post_analytics
		where account_id = $1::uuid
		order by captured_at desc, post_id
		limit $2`
	rows, err := s.pool.Query(ctx, query, accountID, limit)
	if err != nil {
		return nil, fmt.Errorf("social publishing: listar analytics: %w", err)
	}
	defer rows.Close()
	analytics := make([]Analytics, 0)
	for rows.Next() {
		var item Analytics
		if err := rows.Scan(
			&item.PostID,
			&item.Views,
			&item.Reach,
			&item.Likes,
			&item.Comments,
			&item.Saved,
			&item.Shares,
			&item.TotalInteractions,
			&item.CapturedAt,
		); err != nil {
			return nil, fmt.Errorf("social publishing: ler analytics: %w", err)
		}
		analytics = append(analytics, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("social publishing: iterar analytics: %w", err)
	}
	return analytics, nil
}

func (s *Store) RuntimeContext(
	ctx context.Context,
	accountID string,
	now time.Time,
) (RuntimeContext, error) {
	overview, err := s.Overview(ctx, accountID, now)
	if err != nil {
		return RuntimeContext{}, err
	}
	return RuntimeContext{
		AccountID:   accountID,
		GeneratedAt: now.UTC(),
		Counts:      overview.Counts,
		Upcoming:    overview.Upcoming,
		Analytics:   overview.Analytics,
	}, nil
}

func (s *Store) AnalyticsTarget(
	ctx context.Context,
	accountID, postID string,
) (analyticsTarget, error) {
	const query = `
		select p.id::text, p.account_id::text, p.external_media_id,
		       active_connection.access_token_ciphertext
		from social_publishing.posts p
		join social_publishing.connections target_connection
		  on target_connection.account_id = p.account_id
		 and target_connection.id = p.connection_id
		join social_publishing.connections active_connection
		  on active_connection.account_id = p.account_id
		 and active_connection.ig_user_id = target_connection.ig_user_id
		 and active_connection.status = 'connected'
		where p.account_id = $1::uuid
		  and p.id = $2::uuid
		  and p.status = 'published'
		  and p.external_media_id <> ''`
	var target analyticsTarget
	err := s.pool.QueryRow(ctx, query, accountID, postID).Scan(
		&target.PostID,
		&target.AccountID,
		&target.ExternalMediaID,
		&target.TokenCiphertext,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return analyticsTarget{}, ErrNotFound
	}
	if err != nil {
		return analyticsTarget{}, fmt.Errorf("social publishing: alvo analytics: %w", err)
	}
	return target, nil
}

func (s *Store) SaveAnalytics(
	ctx context.Context,
	accountID, postID string,
	analytics Analytics,
) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("social publishing: iniciar analytics: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const upsert = `
		insert into social_publishing.post_analytics (
			account_id, post_id, views, reach, likes, comments, saved, shares,
			total_interactions, captured_at
		)
		values ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10)
		on conflict (account_id, post_id) do update set
			views = excluded.views,
			reach = excluded.reach,
			likes = excluded.likes,
			comments = excluded.comments,
			saved = excluded.saved,
			shares = excluded.shares,
			total_interactions = excluded.total_interactions,
			captured_at = excluded.captured_at,
			updated_at = now()`
	if _, err := tx.Exec(
		ctx,
		upsert,
		accountID,
		postID,
		analytics.Views,
		analytics.Reach,
		analytics.Likes,
		analytics.Comments,
		analytics.Saved,
		analytics.Shares,
		analytics.TotalInteractions,
		analytics.CapturedAt,
	); err != nil {
		return fmt.Errorf("social publishing: salvar analytics atual: %w", err)
	}
	const snapshot = `
		insert into social_publishing.analytics_snapshots (
			account_id, post_id, source, views, reach, likes, comments, saved,
			shares, total_interactions, captured_at
		)
		values (
			$1::uuid, $2::uuid, 'instagram', $3, $4, $5, $6, $7, $8, $9, $10
		)
		on conflict (account_id, post_id, captured_at) do nothing`
	if _, err := tx.Exec(
		ctx,
		snapshot,
		accountID,
		postID,
		analytics.Views,
		analytics.Reach,
		analytics.Likes,
		analytics.Comments,
		analytics.Saved,
		analytics.Shares,
		analytics.TotalInteractions,
		analytics.CapturedAt,
	); err != nil {
		return fmt.Errorf("social publishing: salvar snapshot analytics: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("social publishing: confirmar analytics: %w", err)
	}
	return nil
}

func (s *Store) SavePermalink(
	ctx context.Context,
	accountID, postID, mediaID, permalink string,
) error {
	const query = `
		update social_publishing.posts
		set permalink = $4, updated_at = now()
		where account_id = $1::uuid
		  and id = $2::uuid
		  and external_media_id = $3`
	if _, err := s.pool.Exec(ctx, query, accountID, postID, mediaID, permalink); err != nil {
		return fmt.Errorf("social publishing: salvar permalink: %w", err)
	}
	return nil
}
