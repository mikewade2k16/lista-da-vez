package customerdata

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/jackc/pgx/v5"
)

func identityColumns(alias string) string {
	if alias != "" {
		alias += "."
	}
	return strings.Join([]string{
		alias + "id::text", alias + "identity_kind", alias + "issuer",
		alias + "masked_value", alias + "verification_status", alias + "verification_method",
		alias + "first_seen_at", alias + "last_seen_at", alias + "verified_at",
		alias + "revoked_at", alias + "revision",
	}, ", ")
}

func scanIdentity(row rowScanner) (IdentityView, error) {
	var item IdentityView
	err := row.Scan(
		&item.ID, &item.Kind, &item.Issuer, &item.MaskedValue,
		&item.VerificationStatus, &item.VerificationMethod, &item.FirstSeenAt,
		&item.LastSeenAt, &item.VerifiedAt, &item.RevokedAt, &item.Revision,
	)
	return item, mapDBError(err)
}

func (r *PostgresRepository) ListIdentities(
	ctx context.Context,
	scope Scope,
	relationshipID string,
) ([]IdentityView, error) {
	rows, err := r.pool.Query(ctx, `
		select `+identityColumns("")+`
		from customer_data.subject_identities
		where account_id = $1::uuid
		  and client_account_id = $2::uuid
		  and relationship_id = $3::uuid
		order by
		  case verification_status when 'verified' then 0 when 'unverified' then 1 else 2 end,
		  updated_at desc, id
	`, scope.AccountID, scope.ClientAccountID, relationshipID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]IdentityView, 0)
	for rows.Next() {
		item, err := scanIdentity(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) AddIdentity(
	ctx context.Context,
	scope Scope,
	relationshipID string,
	identity ProtectedIdentity,
) (IdentityView, bool, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return IdentityView{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	existing, found, err := findIdentityByIdempotency(ctx, tx, scope, identity.IdempotencyKey)
	if err != nil {
		return IdentityView{}, false, err
	}
	if found {
		return existing, true, nil
	}
	var relationship Relationship
	row := tx.QueryRow(ctx, `
		select `+relationshipColumns("")+`
		from customer_data.relationships
		where account_id = $1::uuid
		  and client_account_id = $2::uuid
		  and id = $3::uuid
		  and archived_at is null
	`, scope.AccountID, scope.ClientAccountID, relationshipID)
	relationship, err = scanRelationship(row)
	if err != nil {
		return IdentityView{}, false, err
	}
	item, err := insertIdentityTx(ctx, tx, scope, relationship, identity)
	if err != nil {
		if err == ErrConflict {
			matched, matchErr := findIdentityByFingerprint(ctx, tx, scope, identity)
			if matchErr == nil && matched.ID != "" {
				if matchedRelationshipID, relErr := identityRelationshipID(ctx, tx, scope, matched.ID); relErr == nil &&
					matchedRelationshipID == relationshipID {
					return matched, true, nil
				}
			}
		}
		return IdentityView{}, false, err
	}
	if err := insertOutbox(ctx, tx, scope, "identity", item.ID, "customer_data.identity.changed", "identity.create:"+identity.IdempotencyKey); err != nil {
		return IdentityView{}, false, err
	}
	if err := insertAudit(ctx, tx, scope, relationship.SubjectID, relationshipID, "create", "identity", item.ID, "identity_added"); err != nil {
		return IdentityView{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return IdentityView{}, false, err
	}
	return item, false, nil
}

func insertIdentityTx(
	ctx context.Context,
	tx pgx.Tx,
	scope Scope,
	relationship Relationship,
	identity ProtectedIdentity,
) (IdentityView, error) {
	metadata := nullableJSON(identity.Metadata, "{}")
	verifiedAt := any(nil)
	if identity.VerificationStatus == "verified" {
		verifiedAt = identity.OccurredAt
	}
	row := tx.QueryRow(ctx, `
		insert into customer_data.subject_identities (
			account_id, client_account_id, relationship_id, subject_id,
			identity_kind, issuer, value_ciphertext, value_fingerprint,
			key_version, masked_value, verification_status, verification_method,
			source_ref_type, source_ref_id, metadata, first_seen_at, last_seen_at,
			verified_at, idempotency_key, created_by_user_id, updated_by_user_id
		) values (
			$1::uuid, $2::uuid, $3::uuid, $4::uuid,
			$5, $6, $7, $8, $9, $10, $11, nullif($12, ''),
			nullif($13, ''), nullif($14, ''), $15::jsonb, $16, $16,
			$17, $18, nullif($19, '')::uuid, nullif($19, '')::uuid
		)
		returning `+identityColumns("")+`
	`, scope.AccountID, scope.ClientAccountID, relationship.ID, relationship.SubjectID,
		identity.Kind, identity.Issuer, identity.Ciphertext, identity.Fingerprint,
		identity.KeyVersion, identity.MaskedValue, identity.VerificationStatus,
		identity.VerificationMethod, identity.SourceRefType, identity.SourceRefID,
		metadata, identity.OccurredAt, verifiedAt, identity.IdempotencyKey, scope.ActorUserID)
	item, err := scanIdentity(row)
	if err != nil {
		return IdentityView{}, mapDBError(err)
	}
	return item, nil
}

func findIdentityByIdempotency(
	ctx context.Context,
	tx pgx.Tx,
	scope Scope,
	key string,
) (IdentityView, bool, error) {
	item, err := scanIdentity(tx.QueryRow(ctx, `
		select `+identityColumns("")+`
		from customer_data.subject_identities
		where account_id = $1::uuid
		  and client_account_id = $2::uuid
		  and idempotency_key = $3
	`, scope.AccountID, scope.ClientAccountID, key))
	if err == ErrNotFound {
		return IdentityView{}, false, nil
	}
	return item, err == nil, err
}

func findIdentityByFingerprint(
	ctx context.Context,
	tx pgx.Tx,
	scope Scope,
	identity ProtectedIdentity,
) (IdentityView, error) {
	return scanIdentity(tx.QueryRow(ctx, `
		select `+identityColumns("")+`
		from customer_data.subject_identities
		where account_id = $1::uuid
		  and client_account_id = $2::uuid
		  and identity_kind = $3
		  and issuer = $4
		  and value_fingerprint = $5
		  and verification_status <> 'revoked'
		limit 1
	`, scope.AccountID, scope.ClientAccountID, identity.Kind, identity.Issuer, identity.Fingerprint))
}

func identityRelationshipID(
	ctx context.Context,
	tx pgx.Tx,
	scope Scope,
	identityID string,
) (string, error) {
	var relationshipID string
	err := tx.QueryRow(ctx, `
		select relationship_id::text
		from customer_data.subject_identities
		where account_id = $1::uuid and client_account_id = $2::uuid and id = $3::uuid
	`, scope.AccountID, scope.ClientAccountID, identityID).Scan(&relationshipID)
	return relationshipID, mapDBError(err)
}

func (r *PostgresRepository) SetIdentityState(
	ctx context.Context,
	scope Scope,
	identityID, state string,
	input IdentityStateInput,
) (IdentityView, bool, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return IdentityView{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var replay bool
	err = tx.QueryRow(ctx, `
		select exists (
			select 1 from customer_data.outbox_events
			where account_id = $1::uuid
			  and client_account_id = $2::uuid
			  and idempotency_key = $3
			)
	`, scope.AccountID, scope.ClientAccountID, "identity.state:"+input.IdempotencyKey).Scan(&replay)
	if err != nil {
		return IdentityView{}, false, err
	}
	if replay {
		item, err := scanIdentity(tx.QueryRow(ctx, `
			select `+identityColumns("")+`
			from customer_data.subject_identities
			where account_id = $1::uuid and client_account_id = $2::uuid and id = $3::uuid
		`, scope.AccountID, scope.ClientAccountID, identityID))
		return item, true, err
	}
	item, err := scanIdentity(tx.QueryRow(ctx, `
		update customer_data.subject_identities
		set verification_status = $4,
		    verification_method = case when $4 = 'verified' then $5 else verification_method end,
		    verified_at = case when $4 = 'verified' then now() else verified_at end,
		    revoked_at = case when $4 = 'revoked' then now() else null end,
		    revision = revision + 1,
		    updated_by_user_id = nullif($6, '')::uuid,
		    updated_at = now()
		where account_id = $1::uuid
		  and client_account_id = $2::uuid
		  and id = $3::uuid
		  and revision = $7
		  and verification_status <> 'revoked'
		returning `+identityColumns("")+`
	`, scope.AccountID, scope.ClientAccountID, identityID, state,
		input.VerificationMethod, scope.ActorUserID, input.ExpectedRevision))
	if err != nil {
		return IdentityView{}, false, mapMutationError(err)
	}
	if err := insertOutbox(ctx, tx, scope, "identity", identityID, "customer_data.identity.changed", "identity.state:"+input.IdempotencyKey); err != nil {
		return IdentityView{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return IdentityView{}, false, err
	}
	return item, false, nil
}

func (r *PostgresRepository) ResolveSubject(
	ctx context.Context,
	scope Scope,
	request ResolveSubjectRequest,
	identities []ProtectedIdentity,
) (ResolveSubjectResult, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return ResolveSubjectResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var linkedSubjectID, linkedRelationshipID string
	err = tx.QueryRow(ctx, `
		select subject_id::text, relationship_id::text
		from customer_data.subject_source_links
		where account_id = $1::uuid
		  and client_account_id = $2::uuid
		  and source_module = $3
		  and source_entity_type = $4
		  and source_entity_id = $5
		  and status = 'active'
	`, scope.AccountID, scope.ClientAccountID, request.Source.SourceModule,
		request.Source.SourceEntityType, request.Source.SourceEntityID).Scan(&linkedSubjectID, &linkedRelationshipID)
	if err == nil {
		if err := refreshRuleDisplayNameTx(
			ctx,
			tx,
			scope,
			linkedSubjectID,
			linkedRelationshipID,
			request.DisplayName,
		); err != nil {
			return ResolveSubjectResult{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return ResolveSubjectResult{}, err
		}
		return ResolveSubjectResult{
			Status: "resolved", SubjectID: linkedSubjectID, RelationshipID: linkedRelationshipID,
			MatchMethod: "source_link", MatchConfidence: 1, Replayed: true, ReasonCodes: []string{},
		}, nil
	}
	if err != pgx.ErrNoRows {
		return ResolveSubjectResult{}, err
	}

	type match struct {
		subjectID, relationshipID string
	}
	matches := map[string]match{}
	for _, identity := range identities {
		var item match
		err := tx.QueryRow(ctx, `
			select subject_id::text, relationship_id::text
			from customer_data.subject_identities
			where account_id = $1::uuid
			  and client_account_id = $2::uuid
			  and identity_kind = $3
			  and issuer = $4
			  and value_fingerprint = $5
			  and verification_status = 'verified'
			limit 1
		`, scope.AccountID, scope.ClientAccountID, identity.Kind, identity.Issuer, identity.Fingerprint).
			Scan(&item.subjectID, &item.relationshipID)
		if err == nil {
			matches[item.subjectID] = item
		} else if err != pgx.ErrNoRows {
			return ResolveSubjectResult{}, err
		}
	}
	if len(matches) == 1 {
		var resolved match
		for _, item := range matches {
			resolved = item
		}
		if err := insertSourceLinkTx(ctx, tx, scope, request, resolved.subjectID, resolved.relationshipID, "verified_exact", 1, "active"); err != nil {
			return ResolveSubjectResult{}, err
		}
		if err := refreshRuleDisplayNameTx(
			ctx,
			tx,
			scope,
			resolved.subjectID,
			resolved.relationshipID,
			request.DisplayName,
		); err != nil {
			return ResolveSubjectResult{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return ResolveSubjectResult{}, err
		}
		return ResolveSubjectResult{
			Status: "resolved", SubjectID: resolved.subjectID, RelationshipID: resolved.relationshipID,
			MatchMethod: "verified_exact", MatchConfidence: 1, ReasonCodes: []string{},
		}, nil
	}
	if len(matches) > 1 {
		candidateID, err := insertResolveCandidateTx(ctx, tx, scope, request, nil, nil, "ambiguous_strong_identity", 1,
			[]string{"mixed_history"})
		if err != nil {
			return ResolveSubjectResult{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return ResolveSubjectResult{}, err
		}
		return ResolveSubjectResult{
			Status: "quarantined", CandidateID: candidateID,
			ReasonCodes: []string{"ambiguous_strong_identity"},
		}, nil
	}

	var crossSubjectID string
	for _, identity := range identities {
		err := tx.QueryRow(ctx, `
			select subject_id::text
			from customer_data.subject_identities
			where account_id = $1::uuid
			  and client_account_id <> $2::uuid
			  and identity_kind = $3
			  and issuer = $4
			  and value_fingerprint = $5
			  and verification_status = 'verified'
			order by created_at
			limit 1
		`, scope.AccountID, scope.ClientAccountID, identity.Kind, identity.Issuer, identity.Fingerprint).
			Scan(&crossSubjectID)
		if err == nil {
			break
		}
		if err != pgx.ErrNoRows {
			return ResolveSubjectResult{}, err
		}
	}
	if crossSubjectID != "" {
		candidateID, err := insertResolveCandidateTx(ctx, tx, scope, request, &crossSubjectID, nil, "verified_exact", 1,
			[]string{"cross_client"})
		if err != nil {
			return ResolveSubjectResult{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return ResolveSubjectResult{}, err
		}
		return ResolveSubjectResult{
			Status: "candidate", CandidateID: candidateID, MatchMethod: "verified_exact",
			MatchConfidence: 1, ReasonCodes: []string{"cross_client"},
		}, nil
	}

	canCreate := request.AllowCreate && hasVerifiedProviderIdentity(identities)
	if !canCreate {
		if err := tx.Commit(ctx); err != nil {
			return ResolveSubjectResult{}, err
		}
		return ResolveSubjectResult{Status: "not_found", ReasonCodes: []string{"creation_not_allowed"}}, nil
	}
	createInput := CreateSubjectInput{
		ClientAccountID: scope.ClientAccountID,
		SubjectType:     "person",
		Relationship: RelationshipInput{
			DisplayName:     fallbackDisplayName(request.DisplayName),
			LifecycleStatus: "lead", Tags: []string{}, CustomFields: []byte("{}"),
		},
		IdempotencyKey: request.RequestID,
	}
	result, err := createSubjectWithinTx(ctx, tx, scope, createInput, identities)
	if err != nil {
		return ResolveSubjectResult{}, err
	}
	if err := insertSourceLinkTx(ctx, tx, scope, request, result.Subject.ID, result.Relationship.ID, "verified_exact", 1, "active"); err != nil {
		return ResolveSubjectResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ResolveSubjectResult{}, err
	}
	return ResolveSubjectResult{
		Status: "created", SubjectID: result.Subject.ID, RelationshipID: result.Relationship.ID,
		MatchMethod: "verified_exact", MatchConfidence: 1, ReasonCodes: []string{},
	}, nil
}

func createSubjectWithinTx(
	ctx context.Context,
	tx pgx.Tx,
	scope Scope,
	input CreateSubjectInput,
	identities []ProtectedIdentity,
) (CreateSubjectResult, error) {
	var subject Subject
	err := tx.QueryRow(ctx, `
		insert into customer_data.subjects (
			account_id, subject_type, status, idempotency_key,
			created_by_user_id, updated_by_user_id
		) values ($1::uuid, 'person', 'active', $2, nullif($3, '')::uuid, nullif($3, '')::uuid)
		returning `+subjectColumns("")+`
	`, scope.AccountID, input.IdempotencyKey, scope.ActorUserID).Scan(
		&subject.ID, &subject.SubjectType, &subject.Status, &subject.MergedIntoSubjectID,
		&subject.Revision, &subject.CreatedAt, &subject.UpdatedAt,
	)
	if err != nil {
		return CreateSubjectResult{}, mapDBError(err)
	}
	relationship, err := scanRelationship(tx.QueryRow(ctx, `
		insert into customer_data.relationships (
			account_id, client_account_id, subject_id, display_name,
			lifecycle_status, classification_source, tags, custom_fields
		) values ($1::uuid, $2::uuid, $3::uuid, $4, 'lead', 'rule', '[]'::jsonb, '{}'::jsonb)
		returning `+relationshipColumns("")+`
	`, scope.AccountID, scope.ClientAccountID, subject.ID, input.Relationship.DisplayName))
	if err != nil {
		return CreateSubjectResult{}, err
	}
	for _, identity := range identities {
		if _, err := insertIdentityTx(ctx, tx, scope, relationship, identity); err != nil {
			return CreateSubjectResult{}, err
		}
	}
	return CreateSubjectResult{Subject: subject, Relationship: relationship}, nil
}

func insertSourceLinkTx(
	ctx context.Context,
	tx pgx.Tx,
	scope Scope,
	request ResolveSubjectRequest,
	subjectID, relationshipID, method string,
	confidence float64,
	status string,
) error {
	_, err := tx.Exec(ctx, `
		insert into customer_data.subject_source_links (
			account_id, client_account_id, subject_id, relationship_id,
			source_module, source_key, source_entity_type, source_entity_id,
			source_version, source_hash, link_method, match_confidence, status,
			idempotency_key
		) values (
			$1::uuid, $2::uuid, $3::uuid, $4::uuid,
			$5, $6, $7, $8, nullif($9, ''), nullif($10, ''),
			$11, $12, $13, $14
		)
		on conflict (account_id, client_account_id, source_module, source_entity_type, source_entity_id)
		do nothing
	`, scope.AccountID, scope.ClientAccountID, subjectID, relationshipID,
		request.Source.SourceModule, request.Source.SourceKey, request.Source.SourceEntityType,
		request.Source.SourceEntityID, request.Source.SourceVersion, request.Source.SourceHash,
		method, confidence, status, request.RequestID+":source")
	return mapDBError(err)
}

// refreshRuleDisplayNameTx keeps the provider name current only while the
// relationship is still rule-managed. Any manual relationship edit changes
// classification_source to manual and permanently wins over this enrichment.
func refreshRuleDisplayNameTx(
	ctx context.Context,
	tx pgx.Tx,
	scope Scope,
	subjectID string,
	relationshipID string,
	displayName string,
) error {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" || len(displayName) > 200 {
		return nil
	}
	tag, err := tx.Exec(ctx, `
		update customer_data.relationships
		set display_name = $4,
		    revision = revision + 1,
		    updated_at = now()
		where account_id = $1::uuid
		  and client_account_id = $2::uuid
		  and id = $3::uuid
		  and classification_source = 'rule'
		  and display_name is distinct from $4
	`, scope.AccountID, scope.ClientAccountID, relationshipID, displayName)
	if err != nil {
		return mapDBError(err)
	}
	if tag.RowsAffected() == 0 {
		return nil
	}
	return insertAudit(
		ctx,
		tx,
		scope,
		subjectID,
		relationshipID,
		"relationship.display_name.refreshed",
		"relationship",
		relationshipID,
		"provider_supplied_name",
	)
}

func insertResolveCandidateTx(
	ctx context.Context,
	tx pgx.Tx,
	scope Scope,
	request ResolveSubjectRequest,
	subjectID, relationshipID *string,
	method string,
	confidence float64,
	riskFlags []string,
) (string, error) {
	evidence, _ := json.Marshal([]string{
		request.Source.SourceModule + ":" + request.Source.SourceEntityType + ":" + request.Source.SourceEntityID,
	})
	flags, _ := json.Marshal(riskFlags)
	var id string
	err := tx.QueryRow(ctx, `
		insert into customer_data.match_candidates (
			account_id, client_account_id, incoming_source_key, incoming_source_type,
			incoming_source_id, incoming_source_version, candidate_subject_id,
			candidate_relationship_id, match_method, match_confidence,
			evidence_refs, risk_flags, idempotency_key
		) values (
			$1::uuid, $2::uuid, $3, $4, $5, nullif($6, ''),
			nullif($7, '')::uuid, nullif($8, '')::uuid, $9, $10,
			$11::jsonb, $12::jsonb, $13
		)
		on conflict (account_id, client_account_id, idempotency_key) do update
		set updated_at = customer_data.match_candidates.updated_at
		returning id::text
	`, scope.AccountID, scope.ClientAccountID, request.Source.SourceKey,
		request.Source.SourceEntityType, request.Source.SourceEntityID,
		request.Source.SourceVersion, pointerString(subjectID), pointerString(relationshipID),
		method, confidence, evidence, flags, request.RequestID+":candidate").Scan(&id)
	return id, mapDBError(err)
}

func hasVerifiedProviderIdentity(identities []ProtectedIdentity) bool {
	for _, item := range identities {
		if item.VerificationStatus != "verified" {
			continue
		}
		switch item.Kind {
		case "whatsapp", "instagram", "erp_customer", "phone", "email":
			return true
		}
	}
	return false
}

func fallbackDisplayName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 200 {
		return "Contato"
	}
	return value
}
