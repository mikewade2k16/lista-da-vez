package customerdata

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) ResolveClientScope(
	ctx context.Context,
	accountID, requestedClientID, userID string,
	platformAdmin bool,
) (Scope, error) {
	if r == nil || r.pool == nil || accountID == "" || userID == "" {
		return Scope{}, ErrNotFound
	}
	var isAgency bool
	var organizationID *string
	err := r.pool.QueryRow(ctx, `
		select is_agency, organization_id::text
		from core.accounts
		where id = $1::uuid and is_active = true
	`, accountID).Scan(&isAgency, &organizationID)
	if err != nil {
		return Scope{}, mapDBError(err)
	}
	clientID := strings.TrimSpace(requestedClientID)
	if !isAgency {
		if clientID == "" {
			clientID = accountID
		}
		if clientID != accountID {
			return Scope{}, ErrNotFound
		}
		return Scope{AccountID: accountID, ClientAccountID: clientID, ActorUserID: userID}, nil
	}
	if clientID == "" {
		return Scope{}, invalid("clientAccountId", "required_for_agency")
	}
	if organizationID == nil {
		return Scope{}, ErrNotFound
	}
	var allowed bool
	err = r.pool.QueryRow(ctx, `
		select exists (
			select 1
			from core.accounts c
			where c.id = $1::uuid
			  and c.is_active = true
			  and c.is_agency = false
			  and c.organization_id = $2::uuid
			  and (
				$3::boolean
				or exists (
					select 1 from core.account_users au
					where au.account_id = c.id
					  and au.user_id = $4::uuid
					  and au.is_active = true
				)
				or exists (
					select 1 from core.organization_users ou
					where ou.organization_id = c.organization_id
					  and ou.user_id = $4::uuid
				)
			  )
		)
	`, clientID, *organizationID, platformAdmin, userID).Scan(&allowed)
	if err != nil {
		return Scope{}, err
	}
	if !allowed {
		return Scope{}, ErrNotFound
	}
	return Scope{AccountID: accountID, ClientAccountID: clientID, ActorUserID: userID}, nil
}

func (r *PostgresRepository) ResolveServiceScope(
	ctx context.Context,
	accountID, clientAccountID string,
) (Scope, error) {
	if r == nil || r.pool == nil || accountID == "" || clientAccountID == "" {
		return Scope{}, ErrNotFound
	}
	var allowed bool
	err := r.pool.QueryRow(ctx, `
		select exists (
			select 1
			from core.accounts owner
			join core.accounts client on client.id = $2::uuid
			where owner.id = $1::uuid
			  and owner.is_active = true
			  and client.is_active = true
			  and (
				(owner.is_agency = false and client.id = owner.id)
				or (
					owner.is_agency = true
					and client.is_agency = false
					and client.organization_id = owner.organization_id
				)
			  )
		)
	`, accountID, clientAccountID).Scan(&allowed)
	if err != nil {
		return Scope{}, err
	}
	if !allowed {
		return Scope{}, ErrNotFound
	}
	return Scope{AccountID: accountID, ClientAccountID: clientAccountID}, nil
}

func (r *PostgresRepository) FindResourceClient(
	ctx context.Context,
	accountID, resourceKind, resourceID string,
) (string, error) {
	if r == nil || r.pool == nil || accountID == "" || resourceID == "" {
		return "", ErrNotFound
	}
	queries := map[string]string{
		ResourceRelationship: `
			select client_account_id::text from customer_data.relationships
			where account_id = $1::uuid and id = $2::uuid`,
		ResourceIdentity: `
			select client_account_id::text from customer_data.subject_identities
			where account_id = $1::uuid and id = $2::uuid`,
		ResourceNote: `
			select client_account_id::text from customer_data.relationship_notes
			where account_id = $1::uuid and id = $2::uuid`,
		ResourceOffline: `
			select client_account_id::text from customer_data.offline_interactions
			where account_id = $1::uuid and id = $2::uuid`,
		ResourceCandidate: `
			select client_account_id::text from customer_data.match_candidates
			where account_id = $1::uuid and id = $2::uuid`,
		ResourceMerge: `
			select client_account_id::text from customer_data.merge_events
			where account_id = $1::uuid and id = $2::uuid`,
		ResourceSegment: `
			select client_account_id::text from customer_data.segments
			where account_id = $1::uuid and id = $2::uuid`,
		ResourceSegmentVersion: `
			select client_account_id::text from customer_data.segment_versions
			where account_id = $1::uuid and id = $2::uuid`,
		ResourceEvaluationRun: `
			select client_account_id::text from customer_data.segment_evaluation_runs
			where account_id = $1::uuid and id = $2::uuid`,
		ResourceMaterialization: `
			select client_account_id::text from customer_data.segment_materializations
			where account_id = $1::uuid and id = $2::uuid`,
	}
	query, ok := queries[resourceKind]
	if !ok {
		return "", ErrNotFound
	}
	var clientID string
	if err := r.pool.QueryRow(ctx, query, accountID, resourceID).Scan(&clientID); err != nil {
		return "", mapDBError(err)
	}
	return clientID, nil
}

func (r *PostgresRepository) CapabilityMode(ctx context.Context, scope Scope, capability string) (CapabilityMode, error) {
	if r == nil || r.pool == nil {
		return CapabilityOff, nil
	}
	var mode string
	err := r.pool.QueryRow(ctx, `
		select coalesce((
			select mode
			from customer_data.capability_states
			where account_id = $1::uuid
			  and client_account_id = $2::uuid
			  and capability_key = $3
		), 'off')
	`, scope.AccountID, scope.ClientAccountID, capability).Scan(&mode)
	return CapabilityMode(mode), err
}

func (r *PostgresRepository) WriterMode(ctx context.Context, scope Scope, entity string) (WriterMode, error) {
	if r == nil || r.pool == nil {
		return WriterLegacy, nil
	}
	var mode string
	err := r.pool.QueryRow(ctx, `
		select coalesce((
			select mode
			from customer_data.writer_states
			where account_id = $1::uuid
			  and client_account_id = $2::uuid
			  and entity_key = $3
		), 'legacy')
	`, scope.AccountID, scope.ClientAccountID, entity).Scan(&mode)
	return WriterMode(mode), err
}

func mapDBError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505", "23514", "23503":
			return ErrConflict
		case "22P02":
			return ErrNotFound
		}
	}
	return err
}

type stableCursor struct {
	At time.Time `json:"at"`
	ID string    `json:"id"`
}

func encodeStableCursor(at time.Time, id string) string {
	raw, _ := json.Marshal(stableCursor{At: at.UTC(), ID: id})
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeStableCursor(value string) (stableCursor, error) {
	if strings.TrimSpace(value) == "" {
		return stableCursor{}, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return stableCursor{}, invalid("cursor", "invalid")
	}
	var cursor stableCursor
	if json.Unmarshal(raw, &cursor) != nil || cursor.At.IsZero() || cursor.ID == "" {
		return stableCursor{}, invalid("cursor", "invalid")
	}
	return cursor, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRelationship(row rowScanner) (Relationship, error) {
	var item Relationship
	var tagsRaw, customRaw []byte
	err := row.Scan(
		&item.ID, &item.ClientAccountID, &item.SubjectID, &item.DisplayName,
		&item.PreferredName, &item.LifecycleStatus, &item.ClassificationSource,
		&item.ClassificationConfidence, &item.OwnerUserID, &tagsRaw, &customRaw,
		&item.FirstSeenAt, &item.LastSeenAt, &item.LastQualifiedAt, &item.ArchivedAt,
		&item.Revision, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		return Relationship{}, mapDBError(err)
	}
	if err := json.Unmarshal(tagsRaw, &item.Tags); err != nil {
		return Relationship{}, err
	}
	item.CustomFields = append(json.RawMessage(nil), customRaw...)
	return item, nil
}

func relationshipColumns(alias string) string {
	if alias != "" {
		alias += "."
	}
	return strings.Join([]string{
		alias + "id::text", alias + "client_account_id::text", alias + "subject_id::text",
		alias + "display_name", alias + "preferred_name", alias + "lifecycle_status",
		alias + "classification_source", alias + "classification_confidence",
		alias + "owner_user_id::text", alias + "tags", alias + "custom_fields",
		alias + "first_seen_at", alias + "last_seen_at", alias + "last_qualified_at",
		alias + "archived_at", alias + "revision", alias + "created_at", alias + "updated_at",
	}, ", ")
}

func subjectColumns(alias string) string {
	if alias != "" {
		alias += "."
	}
	return strings.Join([]string{
		alias + "id::text", alias + "subject_type", alias + "status",
		alias + "merged_into_subject_id::text", alias + "revision",
		alias + "created_at", alias + "updated_at",
	}, ", ")
}

func insertOutbox(
	ctx context.Context,
	tx pgx.Tx,
	scope Scope,
	aggregateType, aggregateID, topic, idempotency string,
) error {
	payload, _ := json.Marshal(map[string]string{
		"aggregateId":     aggregateID,
		"clientAccountId": scope.ClientAccountID,
	})
	_, err := tx.Exec(ctx, `
		insert into customer_data.outbox_events (
			account_id, client_account_id, aggregate_type, aggregate_id,
			topic, schema_version, payload, idempotency_key
		) values ($1::uuid, $2::uuid, $3, $4::uuid, $5, 'v1', $6::jsonb, $7)
		on conflict (account_id, client_account_id, idempotency_key) do nothing
	`, scope.AccountID, scope.ClientAccountID, aggregateType, aggregateID, topic, payload, idempotency)
	return err
}

func insertAudit(
	ctx context.Context,
	tx pgx.Tx,
	scope Scope,
	subjectID, relationshipID, action, entityType, entityID, reason string,
) error {
	var actorID any
	if scope.ActorUserID != "" {
		actorID = scope.ActorUserID
	}
	_, err := tx.Exec(ctx, `
		insert into customer_data.audit_events (
			account_id, client_account_id, subject_id, relationship_id,
			actor_type, actor_id, action, entity_type, entity_id, reason
		) values (
			$1::uuid, $2::uuid, nullif($3, '')::uuid, nullif($4, '')::uuid,
			$5, $6, $7, $8, $9, nullif($10, '')
		)
	`, scope.AccountID, scope.ClientAccountID, subjectID, relationshipID,
		map[bool]string{true: "user", false: "service"}[scope.ActorUserID != ""],
		actorID, action, entityType, entityID, reason)
	return err
}

func requireRowsAffected(tag pgconn.CommandTag) error {
	if tag.RowsAffected() == 0 {
		return ErrConflict
	}
	return nil
}

func stringJSON(values []string) []byte {
	raw, _ := json.Marshal(values)
	return raw
}

func nullableJSON(raw json.RawMessage, fallback string) []byte {
	if len(raw) == 0 {
		return []byte(fallback)
	}
	return raw
}

func appendClause(query *strings.Builder, args *[]any, clause string, value any) {
	*args = append(*args, value)
	_, _ = fmt.Fprintf(query, clause, len(*args))
}
