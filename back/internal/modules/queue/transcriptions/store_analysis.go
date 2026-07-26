package transcriptions

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

func (repository *PostgresRepository) GetAnalysisConfig(ctx context.Context, accountID string) (AnalysisConfig, error) {
	config := defaultAnalysisConfig()
	err := repository.pool.QueryRow(ctx, `
		select
			enabled,
			transcription_provider,
			transcription_model,
			transcription_language,
			coalesce(credential_id::text, ''),
			provider,
			model,
			system_prompt,
			temperature
		from queue.attendance_analysis_configs
		where account_id = $1::uuid;
	`, accountID).Scan(
		&config.Enabled,
		&config.TranscriptionProvider,
		&config.TranscriptionModel,
		&config.TranscriptionLanguage,
		&config.CredentialID,
		&config.Provider,
		&config.Model,
		&config.SystemPrompt,
		&config.Temperature,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return config, nil
	}
	return config, err
}

func (repository *PostgresRepository) PutAnalysisConfig(ctx context.Context, accountID string, config AnalysisConfig, updatedBy string) error {
	_, err := repository.pool.Exec(ctx, `
		insert into queue.attendance_analysis_configs (
			account_id, enabled, transcription_provider, transcription_model,
			transcription_language, credential_id, provider, model, system_prompt, temperature, updated_by
		)
		values ($1::uuid, $2, $3, $4, $5, nullif($6, '')::uuid, $7, $8, $9, $10, nullif($11, '')::uuid)
		on conflict (account_id) do update set
			enabled = excluded.enabled,
			transcription_provider = excluded.transcription_provider,
			transcription_model = excluded.transcription_model,
			transcription_language = excluded.transcription_language,
			credential_id = excluded.credential_id,
			provider = excluded.provider,
			model = excluded.model,
			system_prompt = excluded.system_prompt,
			temperature = excluded.temperature,
			updated_by = excluded.updated_by,
			updated_at = now();
	`, accountID, config.Enabled, config.TranscriptionProvider, config.TranscriptionModel,
		config.TranscriptionLanguage, config.CredentialID, config.Provider, config.Model, config.SystemPrompt,
		config.Temperature, updatedBy)
	if err != nil {
		return err
	}
	if config.Enabled {
		_, err = repository.pool.Exec(ctx, `
			update queue.attendance_recordings
			set
				analysis_status = 'pending',
				analysis_attempt_count = 0,
				analysis_next_attempt_at = now(),
				analysis_locked_at = null,
				analysis_locked_by = '',
				analysis_error = '',
				updated_at = now()
			where account_id = $1::uuid
			  and transcription_status = 'completed'
			  and analysis_requested_at is not null
			  and summary_text = ''
			  and analysis_status in ('pending', 'failed');
		`, accountID)
		return err
	}
	_, err = repository.pool.Exec(ctx, `
		update queue.attendance_recordings
		set
			analysis_status = 'not_requested',
			analysis_requested_at = null,
			analysis_next_attempt_at = null,
			analysis_locked_at = null,
			analysis_locked_by = '',
			analysis_error = '',
			updated_at = now()
		where account_id = $1::uuid
		  and analysis_status in ('pending', 'failed');
	`, accountID)
	return err
}

func (repository *PostgresRepository) RequestAnalysis(ctx context.Context, accountID, recordingID string) (Recording, error) {
	command, err := repository.pool.Exec(ctx, `
		update queue.attendance_recordings
		set
			analysis_status = case when analysis_status = 'processing' then analysis_status else 'pending' end,
			analysis_requested_at = coalesce(analysis_requested_at, now()),
			analysis_next_attempt_at = case when analysis_status = 'processing' then analysis_next_attempt_at else now() end,
			analysis_locked_at = case when analysis_status = 'processing' then analysis_locked_at else null end,
			analysis_locked_by = case when analysis_status = 'processing' then analysis_locked_by else '' end,
			analysis_attempt_count = case when analysis_status = 'failed' then 0 else analysis_attempt_count end,
			analysis_error = case when analysis_status = 'processing' then analysis_error else '' end,
			updated_at = now()
		where account_id = $1::uuid
		  and id = $2::uuid
		  and transcription_status = 'completed';
	`, accountID, recordingID)
	if err != nil {
		return Recording{}, err
	}
	if command.RowsAffected() == 0 {
		return Recording{}, ErrNotReady
	}
	return repository.GetRecording(ctx, accountID, recordingID)
}

func (repository *PostgresRepository) ClaimAnalysis(ctx context.Context, workerID string, lease time.Duration) (Recording, error) {
	leaseSeconds := max(1, int(lease.Seconds()))
	recording, err := scanRecording(repository.pool.QueryRow(ctx, `
		with candidate as (
			select account_id, id
			from queue.attendance_recordings
			where transcription_status = 'completed'
			  and analysis_requested_at is not null
			  and analysis_attempt_count < 3
			  and (
				(analysis_status = 'pending' and coalesce(analysis_next_attempt_at, now()) <= now())
				or
				(analysis_status = 'processing' and analysis_locked_at < now() - make_interval(secs => $2))
			  )
			order by coalesce(analysis_next_attempt_at, analysis_requested_at), id
			for update skip locked
			limit 1
		),
		claimed as (
			update queue.attendance_recordings recording
			set
				analysis_status = 'processing',
				analysis_attempt_count = recording.analysis_attempt_count + 1,
				analysis_locked_at = now(),
				analysis_locked_by = $1,
				updated_at = now()
			from candidate
			where recording.account_id = candidate.account_id
			  and recording.id = candidate.id
			returning recording.account_id, recording.id
		)
	`+recordingSelect+`
		join claimed
		  on claimed.account_id = recording.account_id
		 and claimed.id = recording.id;
	`, workerID, leaseSeconds))
	if errors.Is(err, pgx.ErrNoRows) {
		return Recording{}, ErrNotFound
	}
	return recording, err
}

func (repository *PostgresRepository) CompleteAnalysis(ctx context.Context, accountID, recordingID string, result AnalysisResult, configSnapshot json.RawMessage) error {
	if len(result.Report) == 0 {
		result.Report = json.RawMessage(`{}`)
	}
	if len(configSnapshot) == 0 {
		configSnapshot = json.RawMessage(`{}`)
	}
	command, err := repository.pool.Exec(ctx, `
		update queue.attendance_recordings
		set
			analysis_status = 'completed',
			summary_text = $3,
			analysis_report = $4::jsonb,
			analysis_config_snapshot = $5::jsonb,
			analysis_error = '',
			analysis_next_attempt_at = null,
			analysis_locked_at = null,
			analysis_locked_by = '',
			updated_at = now()
		where account_id = $1::uuid
		  and id = $2::uuid
		  and analysis_status = 'processing';
	`, accountID, recordingID, result.Summary, result.Report, configSnapshot)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (repository *PostgresRepository) FailAnalysis(ctx context.Context, accountID, recordingID, errorMessage string, retryAt *time.Time) error {
	status := AnalysisStatusFailed
	if retryAt != nil {
		status = AnalysisStatusPending
	}
	command, err := repository.pool.Exec(ctx, `
		update queue.attendance_recordings
		set
			analysis_status = $3,
			analysis_error = $4,
			analysis_next_attempt_at = $5,
			analysis_locked_at = null,
			analysis_locked_by = '',
			updated_at = now()
		where account_id = $1::uuid
		  and id = $2::uuid;
	`, accountID, recordingID, status, errorMessage, retryAt)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
