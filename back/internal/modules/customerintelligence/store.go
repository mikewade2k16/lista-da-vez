package customerintelligence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

func repositoryError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, pgx.ErrNoRows):
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505", "40001":
			return ErrConflict
		case "23503", "23514", "22P02":
			return fmt.Errorf("%w: constraint=%s", ErrInvalidInput, pgErr.ConstraintName)
		}
	}
	return err
}

func normalizedJSON(raw json.RawMessage, fallback string) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(fallback)
	}
	return raw
}

func hashBytes(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func decodeStrings(raw []byte) []string {
	var values []string
	_ = json.Unmarshal(raw, &values)
	if values == nil {
		return []string{}
	}
	return values
}

func (s *PostgresRepository) GetCapability(
	ctx context.Context,
	scope Scope,
	key, scopeKey string,
) (Capability, error) {
	var item Capability
	var raw []byte
	err := s.pool.QueryRow(ctx, `
		select id, account_id, client_account_id, capability_key, scope_key,
		       mode, config, revision, updated_at
		from intelligence.capabilities
		where account_id = $1 and client_account_id = $2
		  and capability_key = $3 and scope_key = $4`,
		scope.AccountID, scope.ClientAccountID, key, scopeKey,
	).Scan(
		&item.ID, &item.AccountID, &item.ClientAccountID, &item.Key, &item.ScopeKey,
		&item.Mode, &raw, &item.Revision, &item.UpdatedAt,
	)
	item.Config = normalizedJSON(raw, `{}`)
	return item, repositoryError(err)
}

func (s *PostgresRepository) ListCapabilities(
	ctx context.Context,
	scope Scope,
) ([]Capability, error) {
	rows, err := s.pool.Query(ctx, `
		select id, account_id, client_account_id, capability_key, scope_key,
		       mode, config, revision, updated_at
		from intelligence.capabilities
		where account_id = $1 and client_account_id = $2
		order by capability_key, scope_key`,
		scope.AccountID, scope.ClientAccountID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Capability, 0)
	for rows.Next() {
		var item Capability
		var config []byte
		if err := rows.Scan(
			&item.ID, &item.AccountID, &item.ClientAccountID, &item.Key,
			&item.ScopeKey, &item.Mode, &config, &item.Revision, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		item.Config = normalizedJSON(config, `{}`)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresRepository) UpsertCapability(
	ctx context.Context,
	accountID, actorID string,
	input CapabilityInput,
) (Capability, error) {
	var item Capability
	var raw []byte
	err := s.pool.QueryRow(ctx, `
		insert into intelligence.capabilities (
		    account_id, client_account_id, capability_key, scope_key, mode,
		    config, revision, updated_by_user_id
		)
		values ($1, $2, $3, $4, $5, $6, 1, nullif($7, '')::uuid)
		on conflict (account_id, client_account_id, capability_key, scope_key)
		do update set
		    mode = excluded.mode,
		    config = excluded.config,
		    revision = intelligence.capabilities.revision + 1,
		    updated_by_user_id = excluded.updated_by_user_id,
		    updated_at = now()
		where $8 > 0 and intelligence.capabilities.revision = $8
		returning id, account_id, client_account_id, capability_key, scope_key,
		          mode, config, revision, updated_at`,
		accountID, input.ClientAccountID, input.Key, input.ScopeKey, input.Mode,
		normalizedJSON(input.Config, `{}`), actorID, input.ExpectedRevision,
	).Scan(
		&item.ID, &item.AccountID, &item.ClientAccountID, &item.Key, &item.ScopeKey,
		&item.Mode, &raw, &item.Revision, &item.UpdatedAt,
	)
	item.Config = normalizedJSON(raw, `{}`)
	if errors.Is(err, pgx.ErrNoRows) {
		return Capability{}, ErrConflict
	}
	return item, repositoryError(err)
}

func (s *PostgresRepository) ListSourceConfigs(
	ctx context.Context,
	scope Scope,
) ([]SourceConfig, error) {
	rows, err := s.pool.Query(ctx, `
		select source.id, source.account_id, source.client_account_id,
		       source.source_key, source.connection_key, source.status, source.mode,
		       source.purpose_key, source.field_allowlist, source.freshness_seconds,
		       source.retention_policy_key, source.retention_policy_version_id,
		       policy.version, policy.snapshot_ttl_seconds, policy.on_expiry,
		       source.config, source.revision, source.last_health_status, source.updated_at
		from intelligence.source_configs source
		join intelligence.retention_policy_versions policy
		  on policy.account_id = source.account_id
		 and policy.id = source.retention_policy_version_id
		where source.account_id = $1 and source.client_account_id = $2
		order by source.source_key, source.connection_key`,
		scope.AccountID, scope.ClientAccountID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]SourceConfig, 0)
	for rows.Next() {
		var item SourceConfig
		var fields, config []byte
		if err := rows.Scan(
			&item.ID, &item.AccountID, &item.ClientAccountID, &item.SourceKey,
			&item.ConnectionKey, &item.Status, &item.Mode, &item.PurposeKey,
			&fields, &item.FreshnessSeconds, &item.RetentionPolicyKey,
			&item.RetentionPolicyVersionID, &item.RetentionPolicyVersion,
			&item.SnapshotTTLSeconds, &item.OnExpiry, &config, &item.Revision,
			&item.LastHealthStatus, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		item.FieldAllowlist = decodeStrings(fields)
		item.Config = normalizedJSON(config, `{}`)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresRepository) GetSourceConfig(
	ctx context.Context,
	scope Scope,
	id string,
) (SourceConfig, error) {
	var item SourceConfig
	var fields, config []byte
	err := s.pool.QueryRow(ctx, `
		select source.id, source.account_id, source.client_account_id,
		       source.source_key, source.connection_key, source.status, source.mode,
		       source.purpose_key, source.field_allowlist, source.freshness_seconds,
		       source.retention_policy_key, source.retention_policy_version_id,
		       policy.version, policy.snapshot_ttl_seconds, policy.on_expiry,
		       source.config, source.revision, source.last_health_status, source.updated_at
		from intelligence.source_configs source
		join intelligence.retention_policy_versions policy
		  on policy.account_id = source.account_id
		 and policy.id = source.retention_policy_version_id
		where source.account_id = $1 and source.client_account_id = $2
		  and source.id = $3`,
		scope.AccountID, scope.ClientAccountID, id,
	).Scan(
		&item.ID, &item.AccountID, &item.ClientAccountID, &item.SourceKey,
		&item.ConnectionKey, &item.Status, &item.Mode, &item.PurposeKey,
		&fields, &item.FreshnessSeconds, &item.RetentionPolicyKey,
		&item.RetentionPolicyVersionID, &item.RetentionPolicyVersion,
		&item.SnapshotTTLSeconds, &item.OnExpiry, &config, &item.Revision,
		&item.LastHealthStatus, &item.UpdatedAt,
	)
	item.FieldAllowlist = decodeStrings(fields)
	item.Config = normalizedJSON(config, `{}`)
	return item, repositoryError(err)
}

func (s *PostgresRepository) UpsertSourceConfig(
	ctx context.Context,
	accountID, actorID string,
	input SourceConfigInput,
) (SourceConfig, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return SourceConfig{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var current SourceConfig
	var currentFields, currentConfig []byte
	err = tx.QueryRow(ctx, `
		select source.id, source.account_id, source.client_account_id,
		       source.source_key, source.connection_key, source.status, source.mode,
		       source.purpose_key, source.field_allowlist, source.freshness_seconds,
		       source.retention_policy_key, source.retention_policy_version_id,
		       policy.version, policy.snapshot_ttl_seconds, policy.on_expiry,
		       source.config, source.revision, source.last_health_status, source.updated_at
		from intelligence.source_configs source
		join intelligence.retention_policy_versions policy
		  on policy.account_id = source.account_id
		 and policy.id = source.retention_policy_version_id
		where source.account_id = $1
		  and source.client_account_id = $2
		  and source.source_key = $3
		  and source.connection_key = $4
		for update of source`,
		accountID, input.ClientAccountID, input.SourceKey, input.ConnectionKey,
	).Scan(
		&current.ID, &current.AccountID, &current.ClientAccountID, &current.SourceKey,
		&current.ConnectionKey, &current.Status, &current.Mode, &current.PurposeKey,
		&currentFields, &current.FreshnessSeconds, &current.RetentionPolicyKey,
		&current.RetentionPolicyVersionID, &current.RetentionPolicyVersion,
		&current.SnapshotTTLSeconds, &current.OnExpiry, &currentConfig,
		&current.Revision, &current.LastHealthStatus, &current.UpdatedAt,
	)
	switch {
	case err == nil:
		if input.ExpectedRevision <= 0 || current.Revision != input.ExpectedRevision {
			return SourceConfig{}, ErrConflict
		}
	case errors.Is(err, pgx.ErrNoRows):
		if input.ExpectedRevision > 0 {
			return SourceConfig{}, ErrConflict
		}
		current = SourceConfig{}
	default:
		return SourceConfig{}, repositoryError(err)
	}

	retentionKey, ttlSeconds, onExpiry := effectiveSourceRetention(input, current)
	policy, err := findPublishedRetentionPolicy(
		ctx, tx, accountID, retentionKey, ttlSeconds, onExpiry,
	)
	if err != nil {
		return SourceConfig{}, err
	}

	fields, _ := json.Marshal(input.FieldAllowlist)
	var item SourceConfig
	var rawFields, config []byte
	err = tx.QueryRow(ctx, `
		insert into intelligence.source_configs (
		    account_id, client_account_id, source_key, connection_key, status,
		    mode, purpose_key, field_allowlist, freshness_seconds,
		    retention_policy_key, retention_policy_version_id, config, revision,
		    last_health_status, created_by_user_id, updated_by_user_id
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, 1,
		        case when $5 = 'disabled' then 'disabled' else 'unknown' end,
		        nullif($13, '')::uuid, nullif($13, '')::uuid)
		on conflict (account_id, client_account_id, source_key, connection_key)
		do update set
		    status = excluded.status,
		    mode = excluded.mode,
		    purpose_key = excluded.purpose_key,
		    field_allowlist = excluded.field_allowlist,
		    freshness_seconds = excluded.freshness_seconds,
		    retention_policy_key = excluded.retention_policy_key,
		    retention_policy_version_id = excluded.retention_policy_version_id,
		    config = excluded.config,
		    revision = intelligence.source_configs.revision + 1,
		    last_health_status = case when excluded.status = 'disabled' then 'disabled'
		                              else intelligence.source_configs.last_health_status end,
		    updated_by_user_id = excluded.updated_by_user_id,
		    updated_at = now()
		where $14 > 0 and intelligence.source_configs.revision = $14
		returning id, account_id, client_account_id, source_key, connection_key,
		          status, mode, purpose_key, field_allowlist, freshness_seconds,
		          retention_policy_key, retention_policy_version_id,
		          config, revision, last_health_status, updated_at`,
		accountID, input.ClientAccountID, input.SourceKey, input.ConnectionKey,
		input.Status, input.Mode, input.PurposeKey, fields, input.FreshnessSeconds,
		policy.Key, policy.ID, normalizedJSON(input.Config, `{}`), actorID,
		input.ExpectedRevision,
	).Scan(
		&item.ID, &item.AccountID, &item.ClientAccountID, &item.SourceKey,
		&item.ConnectionKey, &item.Status, &item.Mode, &item.PurposeKey,
		&rawFields, &item.FreshnessSeconds, &item.RetentionPolicyKey,
		&item.RetentionPolicyVersionID, &config, &item.Revision,
		&item.LastHealthStatus, &item.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return SourceConfig{}, ErrConflict
	}
	if err != nil {
		return SourceConfig{}, repositoryError(err)
	}
	item.RetentionPolicyVersion = policy.Version
	item.SnapshotTTLSeconds = policy.SnapshotTTLSeconds
	item.OnExpiry = policy.OnExpiry
	item.FieldAllowlist = decodeStrings(rawFields)
	item.Config = normalizedJSON(config, `{}`)
	if err := tx.Commit(ctx); err != nil {
		return SourceConfig{}, err
	}
	return item, nil
}

func (s *PostgresRepository) CreateSourceRun(
	ctx context.Context,
	request SourceSyncRequest,
) (SourceRun, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return SourceRun{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var config SourceConfig
	err = tx.QueryRow(ctx, `
		select source.id, source.account_id, source.client_account_id,
		       source.source_key, source.connection_key, source.status, source.mode,
		       source.purpose_key, source.field_allowlist, source.freshness_seconds,
		       source.retention_policy_key, source.retention_policy_version_id,
		       policy.version, policy.snapshot_ttl_seconds, policy.on_expiry,
		       source.config, source.revision, source.last_health_status, source.updated_at
		from intelligence.source_configs source
		join intelligence.retention_policy_versions policy
		  on policy.account_id = source.account_id
		 and policy.id = source.retention_policy_version_id
		where source.account_id = $1 and source.client_account_id = $2
		  and source.id = $3
		for update of source`,
		request.AccountID, request.ClientAccountID, request.SourceConfigID,
	).Scan(
		&config.ID, &config.AccountID, &config.ClientAccountID, &config.SourceKey,
		&config.ConnectionKey, &config.Status, &config.Mode, &config.PurposeKey,
		new([]byte), &config.FreshnessSeconds, &config.RetentionPolicyKey,
		&config.RetentionPolicyVersionID, &config.RetentionPolicyVersion,
		&config.SnapshotTTLSeconds, &config.OnExpiry, new([]byte), &config.Revision,
		&config.LastHealthStatus, &config.UpdatedAt,
	)
	if err != nil {
		return SourceRun{}, false, repositoryError(err)
	}
	if config.Status != "enabled" {
		return SourceRun{}, false, ErrCapabilityDisabled
	}

	var run SourceRun
	err = tx.QueryRow(ctx, `
		insert into intelligence.source_ingestion_runs (
		    account_id, client_account_id, source_config_id, source_key,
		    source_config_revision, retention_policy_version_id, trigger,
		    idempotency_key
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8)
		on conflict (account_id, client_account_id, idempotency_key) do nothing
		returning id, account_id, client_account_id, source_config_id, source_key,
		          retention_policy_version_id, status, trigger, observed_count,
		          accepted_count, rejected_count, error_code, created_at`,
		request.AccountID, request.ClientAccountID, request.SourceConfigID,
		config.SourceKey, config.Revision, config.RetentionPolicyVersionID,
		request.Trigger, request.IdempotencyKey,
	).Scan(
		&run.ID, &run.AccountID, &run.ClientAccountID, &run.SourceConfigID,
		&run.SourceKey, &run.RetentionPolicyVersionID, &run.Status, &run.Trigger,
		&run.ObservedCount, &run.AcceptedCount, &run.RejectedCount,
		&run.ErrorCode, &run.CreatedAt,
	)
	created := true
	if errors.Is(err, pgx.ErrNoRows) {
		created = false
		err = tx.QueryRow(ctx, `
			select id, account_id, client_account_id, source_config_id, source_key,
			       retention_policy_version_id, status, trigger, observed_count,
			       accepted_count, rejected_count, error_code, created_at
			from intelligence.source_ingestion_runs
			where account_id = $1 and client_account_id = $2
			  and idempotency_key = $3`,
			request.AccountID, request.ClientAccountID, request.IdempotencyKey,
		).Scan(
			&run.ID, &run.AccountID, &run.ClientAccountID, &run.SourceConfigID,
			&run.SourceKey, &run.RetentionPolicyVersionID, &run.Status, &run.Trigger,
			&run.ObservedCount, &run.AcceptedCount, &run.RejectedCount,
			&run.ErrorCode, &run.CreatedAt,
		)
	}
	if err != nil {
		return SourceRun{}, false, repositoryError(err)
	}
	if created {
		payload, _ := json.Marshal(struct {
			RunID           string `json:"runId"`
			ClientAccountID string `json:"clientAccountId"`
			RelationshipID  string `json:"relationshipId,omitempty"`
		}{run.ID, run.ClientAccountID, request.RelationshipID})
		_, err = tx.Exec(ctx, `
			insert into intelligence.source_ingestion_jobs (
			    account_id, ordering_key, idempotency_key, kind, payload, max_attempts
			)
			values ($1, $2, $3, 'source.ingest', $4, 5)`,
			request.AccountID, request.SourceConfigID,
			"source-run:"+run.ID, payload,
		)
		if err != nil {
			return SourceRun{}, false, repositoryError(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return SourceRun{}, false, err
	}
	return run, created, nil
}

func (s *PostgresRepository) CompleteSourceRun(
	ctx context.Context,
	accountID, runID, status string,
	observed, accepted, rejected int,
	errorCode string,
) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var sourceConfigID string
	err = tx.QueryRow(ctx, `
		update intelligence.source_ingestion_runs
		set status = $3, observed_count = $4, accepted_count = $5,
		    rejected_count = $6, error_code = $7,
		    started_at = coalesce(started_at, now()), completed_at = now()
		where account_id = $1 and id = $2
		returning source_config_id::text`,
		accountID, runID, status, observed, accepted, rejected, errorCode,
	).Scan(&sourceConfigID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return repositoryError(err)
	}
	healthStatus := "error"
	switch status {
	case "completed":
		healthStatus = "ok"
	case "partial":
		healthStatus = "degraded"
	}
	_, err = tx.Exec(ctx, `
		update intelligence.source_configs
		set last_health_status = case
		        when status = 'disabled' then 'disabled'
		        else $3
		    end,
		    last_health_at = now(),
		    updated_at = now()
		where account_id = $1 and id = $2`,
		accountID, sourceConfigID, healthStatus,
	)
	if err != nil {
		return repositoryError(err)
	}
	return tx.Commit(ctx)
}

func (s *PostgresRepository) GetSourceRun(
	ctx context.Context,
	scope Scope,
	runID string,
) (SourceRun, error) {
	var run SourceRun
	err := s.pool.QueryRow(ctx, `
		select id, account_id, client_account_id, source_config_id, source_key,
		       retention_policy_version_id, status, trigger, observed_count,
		       accepted_count, rejected_count, error_code, created_at
		from intelligence.source_ingestion_runs
		where account_id = $1 and client_account_id = $2 and id = $3`,
		scope.AccountID, scope.ClientAccountID, runID,
	).Scan(
		&run.ID, &run.AccountID, &run.ClientAccountID, &run.SourceConfigID,
		&run.SourceKey, &run.RetentionPolicyVersionID, &run.Status, &run.Trigger,
		&run.ObservedCount, &run.AcceptedCount, &run.RejectedCount,
		&run.ErrorCode, &run.CreatedAt,
	)
	return run, repositoryError(err)
}

func (s *PostgresRepository) ListSourceRuns(
	ctx context.Context,
	scope Scope,
	sourceConfigID string,
	limit int,
) ([]SourceRun, error) {
	rows, err := s.pool.Query(ctx, `
		select id, account_id, client_account_id, source_config_id, source_key,
		       retention_policy_version_id, status, trigger, observed_count,
		       accepted_count, rejected_count, error_code, created_at
		from intelligence.source_ingestion_runs
		where account_id = $1 and client_account_id = $2 and source_config_id = $3
		order by created_at desc, id desc
		limit $4`,
		scope.AccountID, scope.ClientAccountID, sourceConfigID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]SourceRun, 0)
	for rows.Next() {
		var run SourceRun
		if err := rows.Scan(
			&run.ID, &run.AccountID, &run.ClientAccountID, &run.SourceConfigID,
			&run.SourceKey, &run.RetentionPolicyVersionID, &run.Status, &run.Trigger,
			&run.ObservedCount, &run.AcceptedCount, &run.RejectedCount,
			&run.ErrorCode, &run.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, run)
	}
	return items, rows.Err()
}

func (s *PostgresRepository) InsertObservations(
	ctx context.Context,
	run SourceRun,
	observations []Observation,
) (int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var retentionPolicyID string
	var retentionTTLSeconds int
	err = tx.QueryRow(ctx, `
		select run.retention_policy_version_id, policy.snapshot_ttl_seconds
		from intelligence.source_ingestion_runs run
		join intelligence.retention_policy_versions policy
		  on policy.account_id = run.account_id
		 and policy.id = run.retention_policy_version_id
		where run.account_id = $1
		  and run.client_account_id = $2
		  and run.id = $3
		  and run.source_config_id = $4
		  and run.source_key = $5
		for share of run, policy`,
		run.AccountID, run.ClientAccountID, run.ID, run.SourceConfigID, run.SourceKey,
	).Scan(&retentionPolicyID, &retentionTTLSeconds)
	if err != nil {
		return 0, repositoryError(err)
	}

	accepted := 0
	observedAt := time.Now().UTC()
	for _, item := range observations {
		classification := observationClassification(item.ScopeType)
		if classification == "" ||
			(item.Classification != "" && item.Classification != classification) {
			return 0, ErrInvalidInput
		}
		if classification == ObservationClassificationRelationship &&
			(item.SubjectID == "" || item.RelationshipID == "") {
			return 0, ErrInvalidInput
		}
		if classification == ObservationClassificationBusinessContext &&
			(item.SubjectID != "" || item.RelationshipID != "") {
			return 0, ErrInvalidInput
		}
		raw := normalizedJSON(item.Snapshot, `{}`)
		var snapshot any = raw
		var ciphertext any
		cipherVersion := ""
		if item.SnapshotCiphertext != "" {
			snapshot = nil
			ciphertext = item.SnapshotCiphertext
			cipherVersion = "v1"
		}
		idempotencyKey := strings.TrimSpace(item.IdempotencyKey)
		if idempotencyKey == "" {
			idempotencyKey = hashBytes([]byte(
				item.EntityType + "\x00" + item.EntityID + "\x00" +
					item.Version + "\x00" + hashBytes(raw),
			))
		}
		expiresAt := effectiveObservationExpiry(
			observedAt, item.ExpiresAt, retentionTTLSeconds,
		)
		tag, err := tx.Exec(ctx, `
			insert into intelligence.source_observations (
			    account_id, client_account_id, subject_id, relationship_id,
			    ingestion_run_id, source_config_id, source_key, source_entity_type,
			    source_entity_id, idempotency_key, source_version, source_occurred_at, payload_hash,
			    snapshot_json, snapshot_ciphertext, cipher_key_version, sensitivity,
			    classification, purpose_key, retention_policy_version_id,
			    observed_at, expires_at
			)
			values (
			    $1, $2, nullif($3, '')::uuid, nullif($4, '')::uuid,
			    $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20
			    , $21, $22
			)
			on conflict (account_id, source_config_id, idempotency_key) do nothing`,
			run.AccountID, run.ClientAccountID, item.SubjectID, item.RelationshipID,
			run.ID, run.SourceConfigID, run.SourceKey, item.EntityType, item.EntityID,
			idempotencyKey, item.Version, item.OccurredAt, hashBytes(raw), snapshot, ciphertext,
			cipherVersion, item.Sensitivity, classification, item.PurposeKey,
			retentionPolicyID, observedAt, expiresAt,
		)
		if err != nil {
			return 0, repositoryError(err)
		}
		accepted += int(tag.RowsAffected())
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return accepted, nil
}

func (s *PostgresRepository) InsertManualFact(
	ctx context.Context,
	accountID, actorID string,
	input ManualFactInput,
) (Fact, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Fact{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var definitionID string
	err = tx.QueryRow(ctx, `
		insert into intelligence.fact_definitions (
		    account_id, fact_key, created_by_user_id, updated_by_user_id
		)
		values ($1, $2, nullif($3, '')::uuid, nullif($3, '')::uuid)
		on conflict (account_id, fact_key)
		do update set updated_at = intelligence.fact_definitions.updated_at
		returning id`,
		accountID, input.FactKey, actorID,
	).Scan(&definitionID)
	if err != nil {
		return Fact{}, repositoryError(err)
	}
	var definitionVersionID string
	err = tx.QueryRow(ctx, `
		insert into intelligence.fact_definition_versions (
		    account_id, fact_definition_id, version, status, label, value_type,
		    value_schema, sensitivity, context_allowed, manual_verification_allowed,
		    created_by_user_id, validated_by_user_id, published_by_user_id,
		    validated_at, published_at
		)
		values (
		    $1, $2, 1, 'published', $3, $4, '{}'::jsonb, $5, true, true,
		    nullif($6, '')::uuid, nullif($6, '')::uuid, nullif($6, '')::uuid,
		    now(), now()
		)
		on conflict (account_id, fact_definition_id, version)
		do update set label = intelligence.fact_definition_versions.label
		returning id`,
		accountID, definitionID, input.FactKey, input.ValueType, input.Sensitivity, actorID,
	).Scan(&definitionVersionID)
	if err != nil {
		return Fact{}, repositoryError(err)
	}
	if _, err = tx.Exec(ctx, `
		update intelligence.fact_definitions
		set active_version_id = $3, updated_at = now()
		where account_id = $1 and id = $2 and active_version_id is null`,
		accountID, definitionID, definitionVersionID,
	); err != nil {
		return Fact{}, repositoryError(err)
	}

	retentionPolicy, err := findPublishedRetentionPolicy(
		ctx,
		tx,
		accountID,
		defaultRetentionPolicyKey,
		defaultSnapshotTTLSeconds,
		retentionActionTombstone,
	)
	if err != nil {
		return Fact{}, err
	}

	var sourceConfigID, sourceRetentionPolicyID string
	err = tx.QueryRow(ctx, `
		insert into intelligence.source_configs (
		    account_id, client_account_id, source_key, connection_key, status,
		    mode, purpose_key, field_allowlist, freshness_seconds, config,
		    retention_policy_key, retention_policy_version_id,
		    created_by_user_id, updated_by_user_id, last_health_status
		)
		values (
		    $1, $2, 'manual.offline', 'default', 'enabled', 'manual',
		    'customer_profile', '["value","note"]'::jsonb, 0, '{}'::jsonb,
		    $3, $4, nullif($5, '')::uuid, nullif($5, '')::uuid, 'ok'
		)
		on conflict (account_id, client_account_id, source_key, connection_key)
		do update set updated_at = intelligence.source_configs.updated_at
		returning id, retention_policy_version_id`,
		accountID, input.ClientAccountID, retentionPolicy.Key,
		retentionPolicy.ID, actorID,
	).Scan(&sourceConfigID, &sourceRetentionPolicyID)
	if err != nil {
		return Fact{}, repositoryError(err)
	}
	if sourceRetentionPolicyID != retentionPolicy.ID {
		err = tx.QueryRow(ctx, `
			select id, policy_key, version, snapshot_ttl_seconds, on_expiry
			from intelligence.retention_policy_versions
			where account_id = $1 and id = $2`,
			accountID, sourceRetentionPolicyID,
		).Scan(
			&retentionPolicy.ID, &retentionPolicy.Key, &retentionPolicy.Version,
			&retentionPolicy.SnapshotTTLSeconds, &retentionPolicy.OnExpiry,
		)
		if err != nil {
			return Fact{}, repositoryError(err)
		}
	}

	observationPayload, _ := json.Marshal(map[string]any{
		"factKey": input.FactKey,
		"value":   input.Value,
		"note":    input.EvidenceNote,
	})
	var observationJSON any = observationPayload
	var observationCiphertext any
	observationCipherVersion := ""
	observationHashSource := observationPayload
	if input.EvidenceCiphertext != "" {
		observationJSON = nil
		observationCiphertext = input.EvidenceCiphertext
		observationCipherVersion = "v1"
		observationHashSource = []byte(input.EvidenceCiphertext)
	}
	observationObservedAt := time.Now().UTC()
	observationExpiresAt := effectiveObservationExpiry(
		observationObservedAt,
		nil,
		retentionPolicy.SnapshotTTLSeconds,
	)
	var observationID string
	err = tx.QueryRow(ctx, `
		insert into intelligence.source_observations (
		    account_id, client_account_id, subject_id, relationship_id,
		    source_config_id, source_key, source_entity_type, source_entity_id, idempotency_key,
		    source_version, payload_hash, snapshot_json, snapshot_ciphertext,
		    cipher_key_version, sensitivity, purpose_key,
		    retention_policy_version_id, observed_at, expires_at
		)
		values (
		    $1, $2, $3, $4, $5, 'manual.offline', 'manual_fact',
		    gen_random_uuid()::text, $6, '1', $7, $8, $9, $10, $11, 'customer_profile',
		    $12, $13, $14
		)
		on conflict (account_id, source_config_id, idempotency_key) do nothing
		returning id`,
		accountID, input.ClientAccountID, input.SubjectID, input.RelationshipID,
		sourceConfigID, input.IdempotencyKey, hashBytes(observationHashSource),
		observationJSON, observationCiphertext, observationCipherVersion,
		input.Sensitivity, retentionPolicy.ID, observationObservedAt,
		observationExpiresAt,
	).Scan(&observationID)
	if errors.Is(err, pgx.ErrNoRows) {
		var existing Fact
		var rawValue, evidence []byte
		err = tx.QueryRow(ctx, `
			select f.id, f.subject_id, f.relationship_id, f.fact_key, f.version,
			       coalesce(f.value_resolved, 'null'::jsonb),
			       coalesce(f.value_ciphertext, ''), f.value_type, f.confidence,
			       f.resolution_state, d.sensitivity, f.valid_from, f.valid_until,
			       f.effective_at,
			       jsonb_build_array(jsonb_build_object(
			           'observationId', o.id,
			           'sourceKey', o.source_key,
			           'locator', 'manual'
			       ))
			from intelligence.source_observations o
			join intelligence.fact_evidence fe
			  on fe.account_id = o.account_id and fe.observation_id = o.id
			join intelligence.facts f
			  on f.account_id = fe.account_id and f.id = fe.fact_id
			join intelligence.fact_definition_versions d
			  on d.account_id = f.account_id and d.id = f.fact_definition_version_id
			where o.account_id = $1 and o.source_config_id = $2
			  and o.idempotency_key = $3
			order by f.created_at desc
			limit 1`,
			accountID, sourceConfigID, input.IdempotencyKey,
		).Scan(
			&existing.ID, &existing.SubjectID, &existing.RelationshipID,
			&existing.Key, &existing.Version, &rawValue, &existing.ValueCiphertext,
			&existing.ValueType, &existing.Confidence,
			&existing.VerificationState, &existing.Sensitivity,
			&existing.ValidFrom, &existing.ValidUntil, &existing.UpdatedAt,
			&evidence,
		)
		if err != nil {
			return Fact{}, repositoryError(err)
		}
		existing.Value = rawValue
		_ = json.Unmarshal(evidence, &existing.Evidence)
		if err := tx.Commit(ctx); err != nil {
			return Fact{}, err
		}
		return existing, nil
	}
	if err != nil {
		return Fact{}, repositoryError(err)
	}

	valueJSON := any(normalizedJSON(input.Value, `null`))
	var valueCiphertext any
	cipherVersion := ""
	fingerprintSource := []byte(input.Value)
	if input.ValueCiphertext != "" {
		valueJSON = nil
		valueCiphertext = input.ValueCiphertext
		cipherVersion = "v1"
		fingerprintSource = []byte(input.ValueCiphertext)
	}
	var claimID string
	err = tx.QueryRow(ctx, `
		insert into intelligence.claims (
		    account_id, client_account_id, subject_id, relationship_id,
		    fact_definition_id, fact_definition_version_id, fact_key, value_type,
		    value_normalized, value_ciphertext, cipher_key_version, value_fingerprint,
		    extraction_method, confidence, verification_state, valid_from, valid_until,
		    sensitivity, status, created_by_user_id
		)
		values (
		    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
		    'manual', 1, 'verified', $13, $14, $15, 'accepted', nullif($16, '')::uuid
		)
		returning id`,
		accountID, input.ClientAccountID, input.SubjectID, input.RelationshipID,
		definitionID, definitionVersionID, input.FactKey, input.ValueType,
		valueJSON, valueCiphertext, cipherVersion, hashBytes(fingerprintSource),
		input.ValidFrom, input.ValidUntil, input.Sensitivity, actorID,
	).Scan(&claimID)
	if err != nil {
		return Fact{}, repositoryError(err)
	}
	if _, err = tx.Exec(ctx, `
		update intelligence.facts
		set resolution_state = 'superseded'
		where account_id = $1 and relationship_id = $2 and fact_definition_id = $3
		  and resolution_state in ('resolved', 'verified', 'contested')`,
		accountID, input.RelationshipID, definitionID,
	); err != nil {
		return Fact{}, repositoryError(err)
	}

	var fact Fact
	var rawValue []byte
	err = tx.QueryRow(ctx, `
		insert into intelligence.facts (
		    account_id, client_account_id, subject_id, relationship_id,
		    fact_definition_id, fact_definition_version_id, fact_key, version,
		    value_type, value_resolved, value_ciphertext, cipher_key_version,
		    winning_claim_id, confidence, resolution_state, resolution_reason_code,
		    valid_from, valid_until, resolved_by_user_id
		)
		values (
		    $1, $2, $3, $4, $5, $6, $7,
		    (select coalesce(max(version), 0) + 1 from intelligence.facts
		     where account_id = $1 and relationship_id = $4 and fact_definition_id = $5),
		    $8, $9, $10, $11, $12, 1, 'verified', 'manual_verified',
		    $13, $14, nullif($15, '')::uuid
		)
		returning id, subject_id, relationship_id, fact_key, version,
		          coalesce(value_resolved, 'null'::jsonb), coalesce(value_ciphertext, ''),
		          value_type, confidence, resolution_state, valid_from, valid_until,
		          effective_at`,
		accountID, input.ClientAccountID, input.SubjectID, input.RelationshipID,
		definitionID, definitionVersionID, input.FactKey, input.ValueType,
		valueJSON, valueCiphertext, cipherVersion, claimID, input.ValidFrom,
		input.ValidUntil, actorID,
	).Scan(
		&fact.ID, &fact.SubjectID, &fact.RelationshipID, &fact.Key, &fact.Version,
		&rawValue,
		&fact.ValueCiphertext, &fact.ValueType, &fact.Confidence,
		&fact.VerificationState, &fact.ValidFrom, &fact.ValidUntil, &fact.UpdatedAt,
	)
	if err != nil {
		return Fact{}, repositoryError(err)
	}
	fact.Value = rawValue
	fact.Sensitivity = input.Sensitivity
	fact.Evidence = []EvidenceRef{{ObservationID: observationID, SourceKey: "manual.offline", Locator: "manual"}}
	if _, err = tx.Exec(ctx, `
		insert into intelligence.fact_evidence (account_id, fact_id, observation_id, claim_id, role)
		values ($1, $2, $3, $4, 'winning')`,
		accountID, fact.ID, observationID, claimID,
	); err != nil {
		return Fact{}, repositoryError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Fact{}, err
	}
	return fact, nil
}

func (s *PostgresRepository) ListFacts(
	ctx context.Context,
	scope Scope,
	relationshipID string,
	limit int,
) ([]Fact, error) {
	rows, err := s.pool.Query(ctx, `
		select f.id, f.subject_id, f.relationship_id, f.fact_key, f.version,
		       coalesce(f.value_resolved, 'null'::jsonb), coalesce(f.value_ciphertext, ''),
		       f.value_type, f.confidence, f.resolution_state, d.sensitivity,
		       f.valid_from, f.valid_until, f.effective_at,
		       coalesce(e.evidence, '[]'::jsonb)
		from intelligence.facts f
		join intelligence.fact_definition_versions d
		  on d.account_id = f.account_id and d.id = f.fact_definition_version_id
		left join lateral (
		    select jsonb_agg(jsonb_build_object(
		        'observationId', fe.observation_id,
		        'sourceKey', o.source_key,
		        'locator', ''
		    ) order by fe.created_at) as evidence
		    from intelligence.fact_evidence fe
		    join intelligence.source_observations o
		      on o.account_id = fe.account_id and o.id = fe.observation_id
		    where fe.account_id = f.account_id and fe.fact_id = f.id
		) e on true
		where f.account_id = $1 and f.client_account_id = $2
		  and f.relationship_id = $3
		  and f.resolution_state in ('resolved', 'verified', 'contested')
		  and d.context_allowed = true
		  and (f.valid_until is null or f.valid_until > now())
		order by f.effective_at desc, f.id desc
		limit $4`,
		scope.AccountID, scope.ClientAccountID, relationshipID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Fact, 0)
	for rows.Next() {
		var item Fact
		var value, evidence []byte
		if err := rows.Scan(
			&item.ID, &item.SubjectID, &item.RelationshipID, &item.Key,
			&item.Version, &value, &item.ValueCiphertext, &item.ValueType,
			&item.Confidence,
			&item.VerificationState, &item.Sensitivity, &item.ValidFrom,
			&item.ValidUntil, &item.UpdatedAt, &evidence,
		); err != nil {
			return nil, err
		}
		item.Value = value
		_ = json.Unmarshal(evidence, &item.Evidence)
		if item.Evidence == nil {
			item.Evidence = []EvidenceRef{}
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresRepository) LatestSummary(
	ctx context.Context,
	scope Scope,
	relationshipID string,
) (string, Summary, error) {
	var summary Summary
	var ciphertext string
	err := s.pool.QueryRow(ctx, `
		select id, relationship_id, summary_type, content_ciphertext,
		       coalesce(published_at, created_at)
		from intelligence.summary_versions
		where account_id = $1 and client_account_id = $2
		  and relationship_id = $3 and summary_type = 'relationship_profile'
		  and status = 'published'
		  and (expires_at is null or expires_at > now())
		order by version desc
		limit 1`,
		scope.AccountID, scope.ClientAccountID, relationshipID,
	).Scan(&summary.ID, &summary.RelationshipID, &summary.SummaryType, &ciphertext, &summary.GeneratedAt)
	return ciphertext, summary, repositoryError(err)
}

func (s *PostgresRepository) SaveContextSnapshot(
	ctx context.Context,
	envelope ContextEnvelope,
	ciphertext, hash string,
) (string, error) {
	keys, _ := json.Marshal(envelope.ProcessKeys)
	warnings, _ := json.Marshal(envelope.Warnings)
	var id string
	err := s.pool.QueryRow(ctx, `
		insert into intelligence.context_snapshots (
		    account_id, client_account_id, subject_id, relationship_id,
		    process_keys, purpose_key, as_of, payload_ciphertext,
		    cipher_key_version, payload_hash, item_count, token_estimate,
		    omission_codes, expires_at
		)
		values (
		    $1, $2, nullif($3, '')::uuid, $4, $5, $6, $7, $8, 'v1', $9,
		    $10, $11, $12, $13
		)
		returning id`,
		envelope.AccountID, envelope.ClientAccountID, envelope.SubjectID,
		envelope.RelationshipID, keys, envelope.Purpose, envelope.AsOf,
		ciphertext, hash, envelope.Budget.IncludedItems,
		envelope.Budget.EstimatedTokens, warnings, envelope.ExpiresAt,
	).Scan(&id)
	return id, repositoryError(err)
}

func (s *PostgresRepository) ListRecommendations(
	ctx context.Context,
	scope Scope,
	relationshipID string,
	limit int,
) ([]Recommendation, error) {
	rows, err := s.pool.Query(ctx, `
		select id, client_account_id, relationship_id, recommendation_type,
		       status, coalesce(payload_json, 'null'::jsonb),
		       coalesce(payload_ciphertext, ''), confidence,
		       case when rationale_code = '' then '[]'::jsonb
		            else jsonb_build_array(rationale_code) end,
		       expires_at, created_at
		from intelligence.recommendations
		where account_id = $1 and client_account_id = $2
		  and relationship_id = $3
		order by created_at desc
		limit $4`,
		scope.AccountID, scope.ClientAccountID, relationshipID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Recommendation, 0)
	for rows.Next() {
		var item Recommendation
		var payload, reasons []byte
		if err := rows.Scan(
			&item.ID, &item.ClientAccountID, &item.RelationshipID, &item.Type,
			&item.Status, &payload, &item.PayloadCiphertext,
			&item.Confidence, &reasons,
			&item.ValidUntil, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		item.Payload = payload
		item.ReasonCodes = decodeStrings(reasons)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresRepository) ReviewRecommendation(
	ctx context.Context,
	scope Scope,
	actorID, recommendationID string,
	input RecommendationFeedback,
) (Recommendation, error) {
	var item Recommendation
	var payload, reasons []byte
	err := s.pool.QueryRow(ctx, `
		update intelligence.recommendations
		set status = $4, feedback_code = $5,
		    reviewed_by_user_id = nullif($6, '')::uuid,
		    reviewed_at = now(), updated_at = now()
		where account_id = $1 and client_account_id = $2 and id = $3
		  and status = 'proposed'
		returning id, client_account_id, relationship_id, recommendation_type,
		          status, coalesce(payload_json, 'null'::jsonb),
		          coalesce(payload_ciphertext, ''), confidence,
		          case when rationale_code = '' then '[]'::jsonb
		               else jsonb_build_array(rationale_code) end,
		          expires_at, created_at`,
		scope.AccountID, scope.ClientAccountID, recommendationID,
		input.Status, input.Reason, actorID,
	).Scan(
		&item.ID, &item.ClientAccountID, &item.RelationshipID, &item.Type,
		&item.Status, &payload, &item.PayloadCiphertext,
		&item.Confidence, &reasons,
		&item.ValidUntil, &item.CreatedAt,
	)
	item.Payload = payload
	item.ReasonCodes = decodeStrings(reasons)
	if errors.Is(err, pgx.ErrNoRows) {
		return Recommendation{}, ErrConflict
	}
	return item, repositoryError(err)
}

func (s *PostgresRepository) RecordOutcome(
	ctx context.Context,
	outcome AcceptedOutcome,
) (bool, error) {
	payload := normalizedJSON(outcome.Payload, `{}`)
	var details struct {
		ReasonCode     string          `json:"reasonCode"`
		ProcessRunRefs json.RawMessage `json:"processRunRefs"`
		SubjectID      string          `json:"subjectId"`
	}
	_ = json.Unmarshal(payload, &details)
	if len(details.ProcessRunRefs) == 0 {
		details.ProcessRunRefs = json.RawMessage(`[]`)
	}
	tag, err := s.pool.Exec(ctx, `
		insert into intelligence.accepted_outcomes (
		    event_id, account_id, client_account_id, interaction_id, decision_id,
		    conversation_id, subject_id, relationship_id, outcome, reason_code,
		    process_run_refs, occurred_at
		)
		values (
		    $1, $2, $3, $4, $5, nullif($6, '')::uuid, nullif($7, '')::uuid,
		    nullif($8, '')::uuid, $9, $10, $11, $12
		)
		on conflict do nothing`,
		outcome.EventID, outcome.AccountID, outcome.ClientAccountID,
		outcome.InteractionID, outcome.DecisionID, outcome.ConversationID,
		details.SubjectID, outcome.RelationshipID, outcome.OutcomeType,
		details.ReasonCode, details.ProcessRunRefs, outcome.OccurredAt,
	)
	if err != nil {
		return false, repositoryError(err)
	}
	return tag.RowsAffected() == 1, nil
}

func (s *PostgresRepository) ListPortfolioOpportunities(
	ctx context.Context,
	accountID, targetClientAccountID string,
	limit int,
) ([]PortfolioOpportunity, error) {
	rows, err := s.pool.Query(ctx, `
		select id, target_client_account_id, coalesce(organization_id::text, ''),
		       segment_key, opportunity_type, rationale_code, cohort_size,
		       suppression_threshold, aggregate_metrics,
		       jsonb_build_object('minimumCohort', suppression_threshold),
		       confidence, status, expires_at, created_at
		from intelligence.portfolio_opportunities
		where account_id = $1 and target_client_account_id = $2
		  and cohort_size >= suppression_threshold
		order by created_at desc
		limit $3`,
		accountID, targetClientAccountID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]PortfolioOpportunity, 0)
	for rows.Next() {
		var item PortfolioOpportunity
		var aggregate, policy []byte
		if err := rows.Scan(
			&item.ID, &item.TargetClientAccountID, &item.OrganizationID,
			&item.SegmentKey, &item.OpportunityType, &item.RationaleCode,
			&item.CohortSize, &item.SuppressionThreshold, &aggregate, &policy,
			&item.Confidence, &item.Status, &item.ExpiresAt, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		item.Aggregate = aggregate
		item.SuppressionPolicy = policy
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresRepository) CreatePortfolioOpportunity(
	ctx context.Context,
	accountID, actorID string,
	item PortfolioOpportunity,
) (PortfolioOpportunity, error) {
	var aggregate, policy []byte
	err := s.pool.QueryRow(ctx, `
		insert into intelligence.portfolio_opportunities (
		    account_id, target_client_account_id, organization_id, segment_key,
		    cohort_size, suppression_threshold, aggregate_metrics,
		    opportunity_type, rationale_code, confidence, status, expires_at,
		    reviewed_by_user_id
		)
		values (
		    $1, $2, nullif($3, '')::uuid, $4, $5, $6, $7, $8, $9, $10,
		    'proposed', $11, nullif($12, '')::uuid
		)
		returning id, target_client_account_id, coalesce(organization_id::text, ''),
		          segment_key, opportunity_type, rationale_code, cohort_size,
		          suppression_threshold, aggregate_metrics,
		          jsonb_build_object('minimumCohort', suppression_threshold),
		          confidence, status, expires_at, created_at`,
		accountID, item.TargetClientAccountID, item.OrganizationID, item.SegmentKey,
		item.CohortSize, item.SuppressionThreshold,
		normalizedJSON(item.Aggregate, `{}`), item.OpportunityType,
		item.RationaleCode, item.Confidence, item.ExpiresAt, actorID,
	).Scan(
		&item.ID, &item.TargetClientAccountID, &item.OrganizationID,
		&item.SegmentKey, &item.OpportunityType, &item.RationaleCode,
		&item.CohortSize, &item.SuppressionThreshold, &aggregate, &policy,
		&item.Confidence, &item.Status, &item.ExpiresAt, &item.CreatedAt,
	)
	item.Aggregate = aggregate
	item.SuppressionPolicy = policy
	return item, repositoryError(err)
}

func (s *PostgresRepository) ListAuditEvents(
	ctx context.Context,
	scope Scope,
	limit int,
) ([]AuditEvent, error) {
	return s.ListAuditEventPage(ctx, scope, auditEventRepositoryQuery{Limit: limit})
}

func sourceConfigAllows(config SourceConfig, key string) bool {
	for _, allowed := range config.FieldAllowlist {
		if allowed == key {
			return true
		}
	}
	return false
}

func safeErrorCode(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrCapabilityDisabled):
		return "capability_disabled"
	case errors.Is(err, ErrInvalidInput):
		return "invalid_input"
	case errors.Is(err, ErrForbidden):
		return "forbidden"
	default:
		return "source_unavailable"
	}
}

func canonicalString(value string) string {
	return strings.TrimSpace(strings.ToLower(value))
}

var _ FoundationRepository = (*PostgresRepository)(nil)

func unexpectedState(label, value string) error {
	return fmt.Errorf("%w: %s=%s", ErrInvalidInput, label, value)
}

func nowOr(value time.Time, fallback time.Time) time.Time {
	if value.IsZero() {
		return fallback
	}
	return value
}
