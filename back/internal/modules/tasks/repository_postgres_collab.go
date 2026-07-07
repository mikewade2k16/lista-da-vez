package tasks

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

func (repository *PostgresRepository) AddComment(ctx context.Context, accountID string, input AddCommentInput, authorUserID string) (Comment, error) {
	sql, args := repository.scopedQuery(accountID, `
		insert into tasks.task_comments (task_id, author_user_id, body_html)
		select t.id, $3::uuid, $4
		from tasks.tasks t
		where t.account_id = $1::uuid and t.id = $2::uuid and t.archived = false
		returning id::text, task_id::text, author_user_id::text, body_html, created_at, updated_at, deleted_at
	`, input.TaskID, authorUserID, input.BodyHTML)
	comment, err := scanComment(repository.pool.QueryRow(ctx, sql, args...).Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return Comment{}, ErrTaskNotFound
	}
	return comment, err
}

func (repository *PostgresRepository) AddCommentMentions(ctx context.Context, accountID, taskID, commentID string, mentionedUserIDs []string) ([]string, error) {
	mentionedUserIDs = uniqueUserIDs(mentionedUserIDs...)
	if len(mentionedUserIDs) == 0 {
		return nil, nil
	}

	rows, err := repository.pool.Query(ctx, `
		with eligible_users as (
			select distinct au.user_id
			from core.account_users au
			join unnest($3::text[]) input(user_id_text) on au.user_id = input.user_id_text::uuid
			where au.account_id = $1::uuid and au.is_active = true
		)
		insert into tasks.task_mentions (task_id, comment_id, mentioned_user_id)
		select t.id, $2::uuid, eu.user_id
		from tasks.tasks t
		join eligible_users eu on true
		where t.account_id = $1::uuid and t.id = $4::uuid and t.archived = false
		returning mentioned_user_id::text
	`, accountID, commentID, mentionedUserIDs, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	inserted := make([]string, 0)
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		inserted = append(inserted, userID)
	}
	return inserted, rows.Err()
}

func (repository *PostgresRepository) ListComments(ctx context.Context, access AccessContext, taskID string) ([]Comment, error) {
	sql, args := repository.scopedQuery(access.AccountID, `
		select c.id::text, c.task_id::text, c.author_user_id::text, c.body_html,
		       c.created_at, c.updated_at, c.deleted_at
		from tasks.task_comments c
		join tasks.tasks t on t.id = c.task_id
		where t.account_id = $1::uuid and t.id = $2::uuid and c.deleted_at is null
		order by c.created_at asc
	`, taskID)
	rows, err := repository.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	comments := make([]Comment, 0)
	for rows.Next() {
		comment, err := scanComment(rows.Scan)
		if err != nil {
			return nil, err
		}
		comments = append(comments, comment)
	}
	return comments, rows.Err()
}

func (repository *PostgresRepository) UpsertSubscribers(ctx context.Context, accountID, taskID string, userIDs []string) error {
	userIDs = uniqueUserIDs(userIDs...)
	if len(userIDs) == 0 {
		return nil
	}
	_, err := repository.pool.Exec(ctx, `
		with eligible_users as (
			select distinct au.user_id
			from core.account_users au
			join unnest($3::text[]) input(user_id_text) on au.user_id = input.user_id_text::uuid
			where au.account_id = $1::uuid and au.is_active = true
		)
		insert into tasks.task_subscribers (task_id, user_id)
		select t.id, eu.user_id
		from tasks.tasks t
		join eligible_users eu on true
		where t.account_id = $1::uuid and t.id = $2::uuid and t.archived = false
		on conflict (task_id, user_id) do nothing
	`, accountID, taskID, userIDs)
	return err
}

func (repository *PostgresRepository) ListSubscriberUserIDs(ctx context.Context, accountID, taskID string) ([]string, error) {
	rows, err := repository.pool.Query(ctx, `
		select s.user_id::text
		from tasks.task_subscribers s
		join tasks.tasks t on t.id = s.task_id
		where t.account_id = $1::uuid and t.id = $2::uuid and t.archived = false
		order by s.user_id::text asc
	`, accountID, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	userIDs := make([]string, 0)
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		userIDs = append(userIDs, userID)
	}
	return userIDs, rows.Err()
}

func (repository *PostgresRepository) AddShare(ctx context.Context, accountID string, input AddShareInput, sharedByUserID string) (Share, error) {
	sql, args := repository.scopedQuery(accountID, `
		insert into tasks.task_shares (task_id, client_account_id, permission, shared_by_user_id)
		select t.id, $3::uuid, $4, $5::uuid
		from tasks.tasks t
		where t.account_id = $1::uuid and t.id = $2::uuid and t.archived = false
		on conflict (task_id, client_account_id) where revoked_at is null
		do update set permission = excluded.permission
		returning id::text, task_id::text, client_account_id::text, permission,
		          shared_by_user_id::text, created_at, revoked_at
	`, input.TaskID, input.ClientAccountID, input.Permission, sharedByUserID)
	share, err := scanShare(repository.pool.QueryRow(ctx, sql, args...).Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return Share{}, ErrTaskNotFound
	}
	return share, err
}

func (repository *PostgresRepository) ListRelations(ctx context.Context, access AccessContext, taskID string) ([]Relation, error) {
	sql, args := repository.scopedQuery(access.AccountID, `
		select r.id::text, r.task_id::text, r.module, r.resource_type, r.resource_id,
		       r.label_cache, r.metadata_cache, r.refreshed_at
		from tasks.task_relations r
		join tasks.tasks t on t.id = r.task_id
		where t.account_id = $1::uuid and t.id = $2::uuid
		order by r.module asc, r.resource_type asc, r.label_cache asc
	`, taskID)
	rows, err := repository.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	relations := make([]Relation, 0)
	for rows.Next() {
		relation, err := scanRelation(rows.Scan)
		if err != nil {
			return nil, err
		}
		relations = append(relations, relation)
	}
	return relations, rows.Err()
}

func (repository *PostgresRepository) AddRelation(ctx context.Context, accountID string, input AddRelationInput) (Relation, error) {
	metadataJSON, err := json.Marshal(normalizeMap(input.MetadataCache))
	if err != nil {
		return Relation{}, err
	}
	sql, args := repository.scopedQuery(accountID, `
		insert into tasks.task_relations (
			task_id, module, resource_type, resource_id, label_cache, metadata_cache
		)
		select t.id, $3, $4, $5, $6, $7::jsonb
		from tasks.tasks t
		where t.account_id = $1::uuid and t.id = $2::uuid and t.archived = false
		on conflict (task_id, module, resource_type, resource_id)
		do update set label_cache = excluded.label_cache,
		              metadata_cache = excluded.metadata_cache,
		              refreshed_at = now()
		returning id::text, task_id::text, module, resource_type, resource_id,
		          label_cache, metadata_cache, refreshed_at
	`, input.TaskID, input.Module, input.ResourceType, input.ResourceID, input.LabelCache, metadataJSON)
	relation, err := scanRelation(repository.pool.QueryRow(ctx, sql, args...).Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return Relation{}, ErrTaskNotFound
	}
	return relation, err
}

func (repository *PostgresRepository) RemoveRelation(ctx context.Context, accountID, taskID, module, resourceType, resourceID string) (bool, error) {
	sql, args := repository.scopedQuery(accountID, `
		delete from tasks.task_relations r
		using tasks.tasks t
		where r.task_id = t.id
		  and t.account_id = $1::uuid
		  and r.task_id = $2::uuid
		  and r.module = $3
		  and r.resource_type = $4
		  and r.resource_id = $5
	`, taskID, module, resourceType, resourceID)
	tag, err := repository.pool.Exec(ctx, sql, args...)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func scanComment(scan func(...any) error) (Comment, error) {
	var comment Comment
	err := scan(&comment.ID, &comment.TaskID, &comment.AuthorUserID, &comment.BodyHTML, &comment.CreatedAt, &comment.UpdatedAt, &comment.DeletedAt)
	return comment, err
}

func scanShare(scan func(...any) error) (Share, error) {
	var share Share
	err := scan(&share.ID, &share.TaskID, &share.ClientAccountID, &share.Permission, &share.SharedByUserID, &share.CreatedAt, &share.RevokedAt)
	return share, err
}

func scanRelation(scan func(...any) error) (Relation, error) {
	var relation Relation
	var metadataRaw []byte
	err := scan(&relation.ID, &relation.TaskID, &relation.Module, &relation.ResourceType, &relation.ResourceID, &relation.LabelCache, &metadataRaw, &relation.RefreshedAt)
	if err != nil {
		return Relation{}, err
	}
	relation.MetadataCache = decodeMap(metadataRaw)
	return relation, nil
}
