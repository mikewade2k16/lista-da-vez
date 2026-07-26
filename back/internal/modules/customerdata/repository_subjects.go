package customerdata

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

func (r *PostgresRepository) ListSubjects(
	ctx context.Context,
	scope Scope,
	filter SubjectFilter,
	includeIdentities bool,
) (SubjectPage, error) {
	cursor, err := decodeStableCursor(filter.Cursor)
	if err != nil {
		return SubjectPage{}, err
	}
	args := []any{scope.AccountID, scope.ClientAccountID}
	var query strings.Builder
	query.WriteString(`
		select ` + subjectColumns("s") + `, ` + relationshipColumns("rel") + `
		from customer_data.relationships rel
		join customer_data.subjects s
		  on s.account_id = rel.account_id and s.id = rel.subject_id
		where rel.account_id = $1::uuid
		  and rel.client_account_id = $2::uuid
	`)
	if filter.Query != "" {
		appendClause(&query, &args, " and lower(rel.display_name) like lower($%d)", "%"+strings.TrimSpace(filter.Query)+"%")
	}
	if filter.SubjectType != "" {
		appendClause(&query, &args, " and s.subject_type = $%d", filter.SubjectType)
	}
	if filter.LifecycleStatus != "" {
		appendClause(&query, &args, " and rel.lifecycle_status = $%d", filter.LifecycleStatus)
	}
	if filter.Tag != "" {
		appendClause(&query, &args, " and rel.tags ? $%d", strings.ToLower(strings.TrimSpace(filter.Tag)))
	}
	if filter.OwnerUserID != "" {
		appendClause(&query, &args, " and rel.owner_user_id = $%d::uuid", filter.OwnerUserID)
	}
	if filter.Archived != nil {
		if *filter.Archived {
			query.WriteString(" and rel.archived_at is not null")
		} else {
			query.WriteString(" and rel.archived_at is null")
		}
	}
	if filter.UpdatedAfter != nil {
		appendClause(&query, &args, " and rel.updated_at > $%d", filter.UpdatedAfter.UTC())
	}
	if !cursor.At.IsZero() {
		args = append(args, cursor.At, cursor.ID)
		_, _ = fmt.Fprintf(
			&query,
			" and (rel.updated_at, rel.id) < ($%d, $%d::uuid)",
			len(args)-1,
			len(args),
		)
	}
	args = append(args, filter.Limit+1)
	_, _ = fmt.Fprintf(&query, " order by rel.updated_at desc, rel.id desc limit $%d", len(args))

	rows, err := r.pool.Query(ctx, query.String(), args...)
	if err != nil {
		return SubjectPage{}, err
	}
	defer rows.Close()
	items := make([]SubjectListItem, 0, filter.Limit)
	var next string
	for rows.Next() {
		var subject Subject
		var relationship Relationship
		var tagsRaw, customRaw []byte
		if err := rows.Scan(
			&subject.ID, &subject.SubjectType, &subject.Status, &subject.MergedIntoSubjectID,
			&subject.Revision, &subject.CreatedAt, &subject.UpdatedAt,
			&relationship.ID, &relationship.ClientAccountID, &relationship.SubjectID,
			&relationship.DisplayName, &relationship.PreferredName, &relationship.LifecycleStatus,
			&relationship.ClassificationSource, &relationship.ClassificationConfidence,
			&relationship.OwnerUserID, &tagsRaw, &customRaw, &relationship.FirstSeenAt,
			&relationship.LastSeenAt, &relationship.LastQualifiedAt, &relationship.ArchivedAt,
			&relationship.Revision, &relationship.CreatedAt, &relationship.UpdatedAt,
		); err != nil {
			return SubjectPage{}, err
		}
		_ = json.Unmarshal(tagsRaw, &relationship.Tags)
		relationship.CustomFields = customRaw
		if len(items) == filter.Limit {
			next = encodeStableCursor(items[len(items)-1].Relationship.UpdatedAt, items[len(items)-1].Relationship.ID)
			break
		}
		item := SubjectListItem{
			SubjectID: subject.ID, SubjectType: subject.SubjectType, Relationship: relationship,
		}
		if includeIdentities {
			identities, err := r.ListIdentities(ctx, scope, relationship.ID)
			if err != nil {
				return SubjectPage{}, err
			}
			item.PrimaryIdentities = identities
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return SubjectPage{}, err
	}
	return SubjectPage{Items: items, PageInfo: PageInfo{NextCursor: next}}, nil
}

func (r *PostgresRepository) CreateSubject(
	ctx context.Context,
	scope Scope,
	input CreateSubjectInput,
	identities []ProtectedIdentity,
) (CreateSubjectResult, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return CreateSubjectResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	result, found, err := r.findCreatedSubject(ctx, tx, scope, input.IdempotencyKey)
	if err != nil {
		return CreateSubjectResult{}, err
	}
	if found {
		result.Replayed = true
		return result, nil
	}
	var subject Subject
	err = tx.QueryRow(ctx, `
		insert into customer_data.subjects (
			account_id, subject_type, status, idempotency_key,
			created_by_user_id, updated_by_user_id
		) values ($1::uuid, $2, 'active', $3, nullif($4, '')::uuid, nullif($4, '')::uuid)
		returning `+subjectColumns("")+`
	`, scope.AccountID, input.SubjectType, input.IdempotencyKey, scope.ActorUserID).Scan(
		&subject.ID, &subject.SubjectType, &subject.Status, &subject.MergedIntoSubjectID,
		&subject.Revision, &subject.CreatedAt, &subject.UpdatedAt,
	)
	if err != nil {
		return CreateSubjectResult{}, mapDBError(err)
	}
	if input.SubjectType == "person" {
		_, err = tx.Exec(ctx, `
			insert into customer_data.subject_person_profiles (
				account_id, subject_id, legal_name, preferred_name, locale, timezone
			) values ($1::uuid, $2::uuid, $3, $4, $5, $6)
		`, scope.AccountID, subject.ID, input.Profile.LegalName, input.Profile.PreferredName,
			input.Profile.Locale, input.Profile.Timezone)
		if err != nil {
			return CreateSubjectResult{}, err
		}
	}
	tags := stringJSON(input.Relationship.Tags)
	custom := nullableJSON(input.Relationship.CustomFields, "{}")
	lifecycle := input.Relationship.LifecycleStatus
	if lifecycle == "" {
		lifecycle = "lead"
	}
	var relationship Relationship
	row := tx.QueryRow(ctx, `
		insert into customer_data.relationships (
			account_id, client_account_id, subject_id, display_name, preferred_name,
			lifecycle_status, classification_source, owner_user_id, tags, custom_fields,
			created_by_user_id, updated_by_user_id
		) values (
			$1::uuid, $2::uuid, $3::uuid, $4, $5, $6, 'manual',
			nullif($7, '')::uuid, $8::jsonb, $9::jsonb,
			nullif($10, '')::uuid, nullif($10, '')::uuid
		)
		returning `+relationshipColumns("")+`
	`, scope.AccountID, scope.ClientAccountID, subject.ID,
		strings.TrimSpace(input.Relationship.DisplayName), input.Relationship.PreferredName,
		lifecycle, stringValue(input.Relationship.OwnerUserID), tags, custom, scope.ActorUserID)
	relationship, err = scanRelationship(row)
	if err != nil {
		return CreateSubjectResult{}, err
	}
	identityViews := make([]IdentityView, 0, len(identities))
	for _, identity := range identities {
		view, err := insertIdentityTx(ctx, tx, scope, relationship, identity)
		if err != nil {
			return CreateSubjectResult{}, err
		}
		identityViews = append(identityViews, view)
	}
	if err := insertOutbox(ctx, tx, scope, "subject", subject.ID, "customer_data.subject.resolved", "subject.create:"+input.IdempotencyKey); err != nil {
		return CreateSubjectResult{}, err
	}
	if err := insertAudit(ctx, tx, scope, subject.ID, relationship.ID, "create", "subject", subject.ID, "manual_create"); err != nil {
		return CreateSubjectResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return CreateSubjectResult{}, err
	}
	return CreateSubjectResult{Subject: subject, Relationship: relationship, Identities: identityViews}, nil
}

func (r *PostgresRepository) findCreatedSubject(
	ctx context.Context,
	tx pgx.Tx,
	scope Scope,
	idempotencyKey string,
) (CreateSubjectResult, bool, error) {
	var subject Subject
	var relationship Relationship
	var tagsRaw, customRaw []byte
	err := tx.QueryRow(ctx, `
		select `+subjectColumns("s")+`, `+relationshipColumns("rel")+`
		from customer_data.subjects s
		join customer_data.relationships rel
		  on rel.account_id = s.account_id and rel.subject_id = s.id
		where s.account_id = $1::uuid
		  and s.idempotency_key = $2
		  and rel.client_account_id = $3::uuid
	`, scope.AccountID, idempotencyKey, scope.ClientAccountID).Scan(
		&subject.ID, &subject.SubjectType, &subject.Status, &subject.MergedIntoSubjectID,
		&subject.Revision, &subject.CreatedAt, &subject.UpdatedAt,
		&relationship.ID, &relationship.ClientAccountID, &relationship.SubjectID,
		&relationship.DisplayName, &relationship.PreferredName, &relationship.LifecycleStatus,
		&relationship.ClassificationSource, &relationship.ClassificationConfidence,
		&relationship.OwnerUserID, &tagsRaw, &customRaw, &relationship.FirstSeenAt,
		&relationship.LastSeenAt, &relationship.LastQualifiedAt, &relationship.ArchivedAt,
		&relationship.Revision, &relationship.CreatedAt, &relationship.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return CreateSubjectResult{}, false, nil
		}
		return CreateSubjectResult{}, false, err
	}
	_ = json.Unmarshal(tagsRaw, &relationship.Tags)
	relationship.CustomFields = customRaw
	return CreateSubjectResult{Subject: subject, Relationship: relationship}, true, nil
}

func (r *PostgresRepository) GetProfile(
	ctx context.Context,
	scope Scope,
	relationshipID string,
	sections ProfileSections,
) (DeterministicProfile, error) {
	var subject Subject
	var relationship Relationship
	var tagsRaw, customRaw []byte
	err := r.pool.QueryRow(ctx, `
		select `+subjectColumns("s")+`, `+relationshipColumns("rel")+`
		from customer_data.relationships rel
		join customer_data.subjects s
		  on s.account_id = rel.account_id and s.id = rel.subject_id
		where rel.account_id = $1::uuid
		  and rel.client_account_id = $2::uuid
		  and rel.id = $3::uuid
	`, scope.AccountID, scope.ClientAccountID, relationshipID).Scan(
		&subject.ID, &subject.SubjectType, &subject.Status, &subject.MergedIntoSubjectID,
		&subject.Revision, &subject.CreatedAt, &subject.UpdatedAt,
		&relationship.ID, &relationship.ClientAccountID, &relationship.SubjectID,
		&relationship.DisplayName, &relationship.PreferredName, &relationship.LifecycleStatus,
		&relationship.ClassificationSource, &relationship.ClassificationConfidence,
		&relationship.OwnerUserID, &tagsRaw, &customRaw, &relationship.FirstSeenAt,
		&relationship.LastSeenAt, &relationship.LastQualifiedAt, &relationship.ArchivedAt,
		&relationship.Revision, &relationship.CreatedAt, &relationship.UpdatedAt,
	)
	if err != nil {
		return DeterministicProfile{}, mapDBError(err)
	}
	_ = json.Unmarshal(tagsRaw, &relationship.Tags)
	relationship.CustomFields = customRaw
	out := DeterministicProfile{Subject: subject, Relationship: relationship}
	if sections.Identities {
		out.Identities, err = r.ListIdentities(ctx, scope, relationshipID)
	}
	if err == nil && sections.Notes {
		out.Notes, err = r.ListNotes(ctx, scope, relationshipID, 50)
	}
	if err == nil && sections.Interactions {
		out.Interactions, err = r.ListOfflineInteractions(ctx, scope, relationshipID, 50, nil)
	}
	if err == nil && sections.Consents {
		out.Consents, err = r.ListConsents(ctx, scope, relationshipID, 100)
	}
	return out, err
}

func (r *PostgresRepository) UpdateSubject(
	ctx context.Context,
	scope Scope,
	subjectID string,
	patch SubjectPatch,
) (Subject, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Subject{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var subject Subject
	var subjectType string
	err = tx.QueryRow(ctx, `
		update customer_data.subjects s
		set revision = revision + 1,
		    updated_by_user_id = nullif($5, '')::uuid,
		    updated_at = now()
		where s.account_id = $1::uuid
		  and s.id = $2::uuid
		  and s.revision = $3
		  and exists (
			select 1 from customer_data.relationships rel
			where rel.account_id = s.account_id
			  and rel.subject_id = s.id
			  and rel.client_account_id = $4::uuid
		  )
		returning `+subjectColumns("")+`, subject_type
	`, scope.AccountID, subjectID, patch.ExpectedRevision, scope.ClientAccountID, scope.ActorUserID).Scan(
		&subject.ID, &subject.SubjectType, &subject.Status, &subject.MergedIntoSubjectID,
		&subject.Revision, &subject.CreatedAt, &subject.UpdatedAt, &subjectType,
	)
	if err != nil {
		return Subject{}, mapMutationError(err)
	}
	if subjectType != "person" {
		return Subject{}, ErrConflict
	}
	_, err = tx.Exec(ctx, `
		insert into customer_data.subject_person_profiles (
			account_id, subject_id, legal_name, preferred_name, locale, timezone
		) values ($1::uuid, $2::uuid, $3, $4, $5, $6)
		on conflict (account_id, subject_id) do update
		set legal_name = coalesce(excluded.legal_name, customer_data.subject_person_profiles.legal_name),
		    preferred_name = coalesce(excluded.preferred_name, customer_data.subject_person_profiles.preferred_name),
		    locale = coalesce(excluded.locale, customer_data.subject_person_profiles.locale),
		    timezone = coalesce(excluded.timezone, customer_data.subject_person_profiles.timezone),
		    revision = customer_data.subject_person_profiles.revision + 1,
		    updated_at = now()
	`, scope.AccountID, subjectID, patch.LegalName, patch.PreferredName, patch.Locale, patch.Timezone)
	if err != nil {
		return Subject{}, err
	}
	if err := insertAudit(ctx, tx, scope, subjectID, "", "update", "subject", subjectID, "manual_patch"); err != nil {
		return Subject{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Subject{}, err
	}
	return subject, nil
}

func (r *PostgresRepository) UpdateRelationship(
	ctx context.Context,
	scope Scope,
	relationshipID string,
	patch RelationshipPatch,
) (Relationship, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Relationship{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tags := []byte("[]")
	if patch.Tags != nil {
		tags = stringJSON(*patch.Tags)
	}
	custom := []byte("{}")
	if patch.CustomFields != nil {
		custom = *patch.CustomFields
	}
	archive := false
	if patch.Archive != nil {
		archive = *patch.Archive
	}
	row := tx.QueryRow(ctx, `
		update customer_data.relationships
		set display_name = case when $4::boolean then $5 else display_name end,
		    preferred_name = case when $6::boolean then $7 else preferred_name end,
		    lifecycle_status = case when $8::boolean then $9 else lifecycle_status end,
		    owner_user_id = case when $10::boolean then nullif($11, '')::uuid else owner_user_id end,
		    tags = case when $12::boolean then $13::jsonb else tags end,
		    custom_fields = case when $14::boolean then $15::jsonb else custom_fields end,
		    archived_at = case when $16::boolean then (case when $17 then now() else null end) else archived_at end,
		    classification_source = 'manual',
		    revision = revision + 1,
		    updated_by_user_id = nullif($18, '')::uuid,
		    updated_at = now()
		where account_id = $1::uuid
		  and client_account_id = $2::uuid
		  and id = $3::uuid
		  and revision = $19
		returning `+relationshipColumns("")+`
	`, scope.AccountID, scope.ClientAccountID, relationshipID,
		patch.DisplayName != nil, pointerString(patch.DisplayName),
		patch.PreferredName != nil, pointerString(patch.PreferredName),
		patch.LifecycleStatus != nil, pointerString(patch.LifecycleStatus),
		patch.OwnerUserID != nil, pointerString(patch.OwnerUserID),
		patch.Tags != nil, tags, patch.CustomFields != nil, custom,
		patch.Archive != nil, archive, scope.ActorUserID, patch.ExpectedRevision)
	item, err := scanRelationship(row)
	if err != nil {
		return Relationship{}, mapMutationError(err)
	}
	if err := insertAudit(
		ctx,
		tx,
		scope,
		item.SubjectID,
		item.ID,
		"update",
		"relationship",
		item.ID,
		"manual_patch",
	); err != nil {
		return Relationship{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Relationship{}, err
	}
	return item, nil
}

func mapMutationError(err error) error {
	if err == pgx.ErrNoRows {
		return ErrConflict
	}
	return mapDBError(err)
}

func pointerString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
