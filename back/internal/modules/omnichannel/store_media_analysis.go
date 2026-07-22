package omnichannel

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

// CreateMediaAnalysis is idempotent by the database dedupe key. The message and
// conversation are selected together under the same account, so cross-tenant IDs
// cannot create an analysis row.
func (s *Store) CreateMediaAnalysis(ctx context.Context, accountID string, in mediaAnalysisCreate) (mediaAnalysisRow, bool, error) {
	if err := validateMediaAnalysisCreate(in); err != nil {
		return mediaAnalysisRow{}, false, err
	}
	row := s.pool.QueryRow(ctx, `insert into messaging.media_analyses
		(account_id, message_id, conversation_id, analysis_kind, content_hash, provider, model,
		 agent_version_id, expires_at)
		select $1::uuid, m.id, c.id, $4, $5, $6, $7, $8::uuid, $9
		from messaging.messages m
		join messaging.conversations c on c.account_id=m.account_id and c.id=m.conversation_id
		where m.account_id=$1::uuid and m.id=$2::uuid and c.id=$3::uuid
		on conflict (account_id, message_id, analysis_kind, content_hash, provider, model, agent_version_id)
		do nothing
		returning `+mediaAnalysisColumns,
		accountID, in.MessageID, in.ConversationID, in.Kind, strings.ToLower(in.ContentHash),
		strings.TrimSpace(in.Provider), strings.TrimSpace(in.Model), in.AgentVersionID, in.ExpiresAt)
	created, err := scanMediaAnalysis(row)
	if err == nil {
		return created, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return mediaAnalysisRow{}, false, err
	}
	existing, err := s.GetMediaAnalysisByDedupe(ctx, accountID, in)
	return existing, false, err
}

func (s *Store) GetMediaAnalysis(ctx context.Context, accountID, id string) (mediaAnalysisRow, error) {
	row, err := scanMediaAnalysis(s.pool.QueryRow(ctx, `select `+mediaAnalysisColumns+`
		from messaging.media_analyses where account_id=$1::uuid and id=$2::uuid`, accountID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return mediaAnalysisRow{}, ErrNotFound
	}
	return row, err
}

func (s *Store) GetMediaAnalysisByDedupe(ctx context.Context, accountID string, in mediaAnalysisCreate) (mediaAnalysisRow, error) {
	return scanMediaAnalysis(s.pool.QueryRow(ctx, `select `+mediaAnalysisColumns+`
		from messaging.media_analyses
		where account_id=$1::uuid and message_id=$2::uuid and analysis_kind=$3
		  and content_hash=$4 and provider=$5 and model=$6 and agent_version_id=$7::uuid`,
		accountID, in.MessageID, in.Kind, strings.ToLower(in.ContentHash), strings.TrimSpace(in.Provider), strings.TrimSpace(in.Model), in.AgentVersionID))
}

func (s *Store) ListMediaAnalyses(ctx context.Context, accountID, messageID string) ([]mediaAnalysisRow, error) {
	rows, err := s.pool.Query(ctx, `select `+mediaAnalysisColumns+`
		from messaging.media_analyses where account_id=$1::uuid and message_id=$2::uuid
		order by created_at desc, id desc`, accountID, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]mediaAnalysisRow, 0)
	for rows.Next() {
		row, err := scanMediaAnalysis(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// GetMediaDescriptorForAnalysis binds the stream token to the analysis row and
// message in one tenant-scoped query. It deliberately omits hidden-message
// semantics because this is an internal server-to-server route, not a browser API.
func (s *Store) GetMediaDescriptorForAnalysis(ctx context.Context, accountID, analysisID, messageID string) (mediaDescriptor, string, error) {
	var d mediaDescriptor
	var status string
	err := s.pool.QueryRow(ctx, `select m.id::text, m.conversation_id::text, m.instance_scope_key,
		m.media_storage_key, m.media_mime_type, m.media_file_name, m.media_url, m.media_source_kind,
		m.external_message_id, m.metadata_json, i.provider, a.status
		from messaging.media_analyses a
		join messaging.messages m on m.account_id=a.account_id and m.id=a.message_id
		join messaging.conversations c on c.account_id=m.account_id and c.id=m.conversation_id
		left join messaging.whatsapp_instances i on i.account_id=m.account_id and i.id=m.instance_id
		where a.account_id=$1::uuid and a.id=$2::uuid and a.message_id=$3::uuid`,
		accountID, analysisID, messageID).Scan(&d.MessageID, &d.ConversationID, &d.InstanceScopeKey,
		&d.StorageKey, &d.MimeType, &d.FileName, &d.MediaURL, &d.SourceKind,
		&d.ExternalMessageID, &d.Metadata, &d.Provider, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return mediaDescriptor{}, "", ErrNotFound
	}
	return d, status, err
}

// ClaimMediaAnalysis permits one worker to move a queued/failed analysis to
// processing. The row lock is implicit in UPDATE and retries remain bounded.
func (s *Store) ClaimMediaAnalysis(ctx context.Context, accountID, id string) (mediaAnalysisRow, bool, error) {
	row, err := scanMediaAnalysis(s.pool.QueryRow(ctx, `update messaging.media_analyses
		set status='processing', attempts=attempts+1, last_error='', completed_at=null
		where account_id=$1::uuid and id=$2::uuid and status in ('queued','failed') and attempts < 5
		returning `+mediaAnalysisColumns, accountID, id))
	if err == nil {
		return row, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return mediaAnalysisRow{}, false, err
	}
	existing, getErr := s.GetMediaAnalysis(ctx, accountID, id)
	if errors.Is(getErr, ErrNotFound) {
		return mediaAnalysisRow{}, false, getErr
	}
	return existing, false, getErr
}

func (s *Store) CompleteMediaAnalysis(ctx context.Context, accountID, id string, in mediaAnalysisComplete) (mediaAnalysisRow, error) {
	if err := validateMediaAnalysisResult(in); err != nil {
		return mediaAnalysisRow{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return mediaAnalysisRow{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var kind, status string
	if err := tx.QueryRow(ctx, `select analysis_kind, status from messaging.media_analyses
		where account_id=$1::uuid and id=$2::uuid for update`, accountID, id).Scan(&kind, &status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return mediaAnalysisRow{}, ErrNotFound
		}
		return mediaAnalysisRow{}, err
	}
	if status != MediaAnalysisStatusProcessing {
		return mediaAnalysisRow{}, ErrConflict
	}
	if err := validateMediaAnalysisShape(kind, in.ResultJSON); err != nil {
		return mediaAnalysisRow{}, err
	}
	row, err := scanMediaAnalysis(tx.QueryRow(ctx, `update messaging.media_analyses set
		status='completed', result_text=$3, result_json=$4::jsonb, prompt_tokens=$5,
		completion_tokens=$6, cost_usd=$7, latency_ms=$8, last_error='', completed_at=now()
		where account_id=$1::uuid and id=$2::uuid and status='processing'
		returning `+mediaAnalysisColumns, accountID, id, in.ResultText, in.ResultJSON,
		in.PromptTokens, in.CompletionTokens, in.CostUSD, in.LatencyMS))
	if errors.Is(err, pgx.ErrNoRows) {
		return mediaAnalysisRow{}, ErrConflict
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	return row, err
}

func (s *Store) FailMediaAnalysis(ctx context.Context, accountID, id, status, code string) (mediaAnalysisRow, error) {
	if status != MediaAnalysisStatusFailed && status != MediaAnalysisStatusBlocked {
		return mediaAnalysisRow{}, ErrMediaAnalysisInvalid
	}
	code = safeMediaAnalysisError(code)
	row, err := scanMediaAnalysis(s.pool.QueryRow(ctx, `update messaging.media_analyses set
		status=$3, last_error=$4, completed_at=null
		where account_id=$1::uuid and id=$2::uuid and status in ('queued','processing','failed')
		returning `+mediaAnalysisColumns, accountID, id, status, code))
	if errors.Is(err, pgx.ErrNoRows) {
		return mediaAnalysisRow{}, ErrConflict
	}
	return row, err
}

func safeMediaAnalysisError(code string) string {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "policy_blocked", "unsupported_media", "media_too_large", "provider_unavailable",
		"provider_error", "schema_invalid", "timeout", "content_unavailable":
		return strings.ToLower(strings.TrimSpace(code))
	default:
		return "provider_error"
	}
}
