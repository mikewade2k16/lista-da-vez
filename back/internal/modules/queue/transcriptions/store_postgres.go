package transcriptions

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (repository *PostgresRepository) GetRecordingFeature(ctx context.Context, accountID string) (RecordingFeature, error) {
	feature := RecordingFeature{AccountID: accountID}
	err := repository.pool.QueryRow(ctx, `
		select enabled, updated_at, coalesce(updated_by::text, '')
		from queue.attendance_recording_settings
		where account_id = $1::uuid;
	`, accountID).Scan(&feature.Enabled, &feature.UpdatedAt, &feature.UpdatedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return feature, nil
	}
	return feature, err
}

func (repository *PostgresRepository) PutRecordingFeature(ctx context.Context, accountID string, enabled bool, updatedBy string) (RecordingFeature, error) {
	feature := RecordingFeature{AccountID: accountID}
	err := repository.pool.QueryRow(ctx, `
		insert into queue.attendance_recording_settings (
			account_id, enabled, updated_by, updated_at
		)
		values ($1::uuid, $2, $3::uuid, now())
		on conflict (account_id) do update set
			enabled = excluded.enabled,
			updated_by = excluded.updated_by,
			updated_at = now()
		returning enabled, updated_at, coalesce(updated_by::text, '');
	`, accountID, enabled, updatedBy).Scan(&feature.Enabled, &feature.UpdatedAt, &feature.UpdatedBy)
	return feature, err
}

func (repository *PostgresRepository) ResolveService(ctx context.Context, accountID, storeID, serviceID string) (ServiceReference, error) {
	var reference ServiceReference
	err := repository.pool.QueryRow(ctx, `
		select
			source.store_id,
			source.store_name,
			source.service_id,
			source.consultant_id,
			source.consultant_name,
			source.started_at,
			source.finished_at,
			source.finish_outcome
		from (
			select
				active.store_id::text as store_id,
				store.name as store_name,
				active.service_id,
				active.consultant_id::text as consultant_id,
				consultant.name as consultant_name,
				active.service_started_at as started_at,
				0::bigint as finished_at,
				''::text as finish_outcome,
				0 as source_order
			from queue.operation_active_services active
			join queue.stores store
			  on store.id = active.store_id
			 and store.tenant_id = $1::uuid
			join queue.consultants consultant
			  on consultant.id = active.consultant_id
			 and consultant.store_id = active.store_id
			where active.store_id = $2::uuid
			  and active.service_id = $3

			union all

			select
				history.store_id::text,
				store.name,
				history.service_id,
				history.person_id::text,
				history.person_name,
				history.started_at,
				history.finished_at,
				history.finish_outcome,
				1 as source_order
			from queue.operation_service_history history
			join queue.stores store
			  on store.id = history.store_id
			 and store.tenant_id = $1::uuid
			where history.store_id = $2::uuid
			  and history.service_id = $3
		) source
		order by source.source_order
		limit 1;
	`, accountID, storeID, serviceID).Scan(
		&reference.StoreID,
		&reference.StoreName,
		&reference.ServiceID,
		&reference.ConsultantID,
		&reference.ConsultantName,
		&reference.StartedAt,
		&reference.FinishedAt,
		&reference.FinishOutcome,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ServiceReference{}, ErrNotFound
	}
	return reference, err
}

func (repository *PostgresRepository) CreateRecording(ctx context.Context, recording Recording) (Recording, error) {
	var id string
	err := repository.pool.QueryRow(ctx, `
		insert into queue.attendance_recordings (
			account_id,
			store_id,
			service_id,
			consultant_id,
			consultant_name,
			client_session_id,
			recording_status,
			transcription_status,
			mime_type,
			started_at,
			created_by
		)
		values (
			$1::uuid, $2::uuid, $3, $4::uuid, $5, $6,
			'recording', 'pending', $7, $8, $9::uuid
		)
		on conflict (account_id, client_session_id) do update set
			updated_at = now()
		where queue.attendance_recordings.store_id = excluded.store_id
		  and queue.attendance_recordings.service_id = excluded.service_id
		returning id::text;
	`,
		recording.AccountID,
		recording.StoreID,
		recording.ServiceID,
		recording.ConsultantID,
		recording.ConsultantName,
		recording.ClientSessionID,
		recording.MimeType,
		recording.StartedAt,
		recording.CreatedBy,
	).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Recording{}, ErrChunkConflict
	}
	if err != nil {
		return Recording{}, err
	}
	return repository.GetRecording(ctx, recording.AccountID, id)
}

const recordingSelect = `
	select
		recording.id::text,
		recording.account_id::text,
		recording.store_id::text,
		store.name,
		recording.service_id,
		recording.consultant_id::text,
		recording.consultant_name,
		recording.client_session_id,
		recording.recording_status,
		recording.transcription_status,
		recording.mime_type,
		recording.started_at,
		coalesce(recording.ended_at, 0),
		recording.chunk_count,
		recording.size_bytes,
		coalesce(recording.audio_storage_key, ''),
		recording.audio_sha256,
		recording.transcript_text,
		recording.live_transcript_text,
		recording.live_transcript_updated_at,
		recording.transcript_error,
		recording.transcription_requested_at,
		recording.transcription_attempt_count,
		recording.analysis_status,
		recording.summary_text,
		recording.analysis_report,
		recording.analysis_error,
		recording.analysis_requested_at,
		recording.analysis_attempt_count,
		recording.created_by::text,
		recording.created_at,
		recording.updated_at,
		coalesce(history.finish_outcome, '')
	from queue.attendance_recordings recording
	join queue.stores store
	  on store.id = recording.store_id
	 and store.tenant_id = recording.account_id
	left join queue.operation_service_history history
	  on history.store_id = recording.store_id
	 and history.service_id = recording.service_id
`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRecording(scanner rowScanner) (Recording, error) {
	var recording Recording
	err := scanner.Scan(
		&recording.ID,
		&recording.AccountID,
		&recording.StoreID,
		&recording.StoreName,
		&recording.ServiceID,
		&recording.ConsultantID,
		&recording.ConsultantName,
		&recording.ClientSessionID,
		&recording.RecordingStatus,
		&recording.TranscriptionStatus,
		&recording.MimeType,
		&recording.StartedAt,
		&recording.EndedAt,
		&recording.ChunkCount,
		&recording.SizeBytes,
		&recording.AudioStorageKey,
		&recording.AudioSHA256,
		&recording.TranscriptText,
		&recording.LiveTranscriptText,
		&recording.LiveTranscriptUpdatedAt,
		&recording.TranscriptError,
		&recording.TranscriptionRequestedAt,
		&recording.TranscriptionAttemptCount,
		&recording.AnalysisStatus,
		&recording.SummaryText,
		&recording.AnalysisReport,
		&recording.AnalysisError,
		&recording.AnalysisRequestedAt,
		&recording.AnalysisAttemptCount,
		&recording.CreatedBy,
		&recording.CreatedAt,
		&recording.UpdatedAt,
		&recording.FinishOutcome,
	)
	return recording, err
}

func (repository *PostgresRepository) GetRecording(ctx context.Context, accountID, recordingID string) (Recording, error) {
	recording, err := scanRecording(repository.pool.QueryRow(ctx, recordingSelect+`
		where recording.account_id = $1::uuid
		  and recording.id = $2::uuid;
	`, accountID, recordingID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Recording{}, ErrNotFound
	}
	return recording, err
}

func (repository *PostgresRepository) GetChunk(ctx context.Context, accountID, recordingID string, sequence int) (Chunk, error) {
	var chunk Chunk
	err := repository.pool.QueryRow(ctx, `
		select recording_id::text, sequence, storage_key, mime_type, size_bytes, sha256
		from queue.attendance_recording_chunks
		where account_id = $1::uuid
		  and recording_id = $2::uuid
		  and sequence = $3;
	`, accountID, recordingID, sequence).Scan(
		&chunk.RecordingID,
		&chunk.Sequence,
		&chunk.StorageKey,
		&chunk.MimeType,
		&chunk.SizeBytes,
		&chunk.SHA256,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Chunk{}, ErrNotFound
	}
	return chunk, err
}

func (repository *PostgresRepository) SaveChunk(ctx context.Context, accountID string, chunk Chunk) (Recording, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return Recording{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var savedSequence int
	err = tx.QueryRow(ctx, `
		insert into queue.attendance_recording_chunks (
			account_id, recording_id, sequence, storage_key, mime_type, size_bytes, sha256
		)
		values ($1::uuid, $2::uuid, $3, $4, $5, $6, $7)
		on conflict (account_id, recording_id, sequence) do update set
			storage_key = excluded.storage_key,
			mime_type = excluded.mime_type,
			size_bytes = excluded.size_bytes
		where queue.attendance_recording_chunks.sha256 = excluded.sha256
		returning sequence;
	`, accountID, chunk.RecordingID, chunk.Sequence, chunk.StorageKey, chunk.MimeType, chunk.SizeBytes, chunk.SHA256).Scan(&savedSequence)
	if errors.Is(err, pgx.ErrNoRows) {
		return Recording{}, ErrChunkConflict
	}
	if err != nil {
		return Recording{}, err
	}

	command, err := tx.Exec(ctx, `
		update queue.attendance_recordings recording
		set
			chunk_count = aggregate.chunk_count,
			size_bytes = aggregate.size_bytes,
			updated_at = now()
		from (
			select count(*)::integer as chunk_count, coalesce(sum(size_bytes), 0)::bigint as size_bytes
			from queue.attendance_recording_chunks
			where account_id = $1::uuid
			  and recording_id = $2::uuid
		) aggregate
		where recording.account_id = $1::uuid
		  and recording.id = $2::uuid
		  and recording.recording_status = 'recording';
	`, accountID, chunk.RecordingID)
	if err != nil {
		return Recording{}, err
	}
	if command.RowsAffected() == 0 {
		return Recording{}, ErrNotFound
	}

	_, err = tx.Exec(ctx, `
		with bounds as (
			select max(sequence) as max_sequence
			from queue.attendance_recording_chunks
			where account_id = $1::uuid
			  and recording_id = $2::uuid
		),
		contiguous as (
			select coalesce(
				(
					select min(expected.sequence)
					from bounds
					cross join lateral generate_series(0, bounds.max_sequence) expected(sequence)
					left join queue.attendance_recording_chunks present
					  on present.account_id = $1::uuid
					 and present.recording_id = $2::uuid
					 and present.sequence = expected.sequence
					where present.sequence is null
				),
				coalesce((select max_sequence + 1 from bounds), 0)
			)::integer as chunk_count
		)
		insert into queue.attendance_live_transcription_segments (
			account_id,
			recording_id,
			segment_index,
			start_sequence,
			end_sequence,
			trim_start_ms
		)
		select
			$1::uuid,
			$2::uuid,
			segment_index,
			case when segment_index = 0 then 0 else segment_index * 5 - 1 end,
			segment_index * 5 + 4,
			case when segment_index = 0 then 0 else 2500 end
		from contiguous
		cross join lateral generate_series(0, (contiguous.chunk_count - 5) / 5) segment_index
		where contiguous.chunk_count >= 5
		on conflict (account_id, recording_id, segment_index) do nothing;
	`, accountID, chunk.RecordingID)
	if err != nil {
		return Recording{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Recording{}, err
	}
	return repository.GetRecording(ctx, accountID, chunk.RecordingID)
}

func (repository *PostgresRepository) ListChunks(ctx context.Context, accountID, recordingID string) ([]Chunk, error) {
	rows, err := repository.pool.Query(ctx, `
		select recording_id::text, sequence, storage_key, mime_type, size_bytes, sha256
		from queue.attendance_recording_chunks
		where account_id = $1::uuid
		  and recording_id = $2::uuid
		order by sequence asc;
	`, accountID, recordingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	chunks := make([]Chunk, 0)
	for rows.Next() {
		var chunk Chunk
		if err := rows.Scan(&chunk.RecordingID, &chunk.Sequence, &chunk.StorageKey, &chunk.MimeType, &chunk.SizeBytes, &chunk.SHA256); err != nil {
			return nil, err
		}
		chunks = append(chunks, chunk)
	}
	return chunks, rows.Err()
}

func (repository *PostgresRepository) ClaimLiveTranscriptSegment(
	ctx context.Context,
	workerID string,
	lease time.Duration,
) (LiveTranscriptSegment, error) {
	leaseSeconds := max(1, int(lease.Seconds()))
	var segment LiveTranscriptSegment
	err := repository.pool.QueryRow(ctx, `
		with candidate as (
			select segment.account_id, segment.id
			from queue.attendance_live_transcription_segments segment
			join queue.attendance_recordings recording
			  on recording.account_id = segment.account_id
			 and recording.id = segment.recording_id
			where recording.transcription_status <> 'completed'
			  and (
				(
					segment.status = 'pending'
					and segment.next_attempt_at <= now()
				)
				or (
					segment.status = 'processing'
					and segment.locked_at < now() - make_interval(secs => $2)
				)
			  )
			  and segment.attempt_count < 3
			  and not exists (
				select 1
				from queue.attendance_live_transcription_segments earlier
				where earlier.account_id = segment.account_id
				  and earlier.recording_id = segment.recording_id
				  and earlier.segment_index < segment.segment_index
				  and earlier.status in ('pending', 'processing')
			  )
			order by segment.next_attempt_at, segment.recording_id, segment.segment_index
			for update of segment skip locked
			limit 1
		),
		claimed as (
			update queue.attendance_live_transcription_segments segment
			set
				status = 'processing',
				attempt_count = segment.attempt_count + 1,
				locked_at = now(),
				locked_by = $1,
				updated_at = now()
			from candidate
			where segment.account_id = candidate.account_id
			  and segment.id = candidate.id
			returning
				segment.id::text,
				segment.account_id::text,
				segment.recording_id::text,
				segment.segment_index,
				segment.start_sequence,
				segment.end_sequence,
				segment.trim_start_ms,
				segment.attempt_count
		)
		select * from claimed;
	`, workerID, leaseSeconds).Scan(
		&segment.ID,
		&segment.AccountID,
		&segment.RecordingID,
		&segment.SegmentIndex,
		&segment.StartSequence,
		&segment.EndSequence,
		&segment.TrimStartMS,
		&segment.AttemptCount,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return LiveTranscriptSegment{}, ErrNotFound
	}
	return segment, err
}

func (repository *PostgresRepository) ListLiveTranscriptChunks(
	ctx context.Context,
	segment LiveTranscriptSegment,
) ([]Chunk, error) {
	rows, err := repository.pool.Query(ctx, `
		select recording_id::text, sequence, storage_key, mime_type, size_bytes, sha256
		from queue.attendance_recording_chunks
		where account_id = $1::uuid
		  and recording_id = $2::uuid
		  and (
			sequence between $3 and $4
			or ($3 > 0 and sequence = 0)
		  )
		order by sequence asc;
	`, segment.AccountID, segment.RecordingID, segment.StartSequence, segment.EndSequence)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	chunks := make([]Chunk, 0, 7)
	for rows.Next() {
		var chunk Chunk
		if err := rows.Scan(
			&chunk.RecordingID,
			&chunk.Sequence,
			&chunk.StorageKey,
			&chunk.MimeType,
			&chunk.SizeBytes,
			&chunk.SHA256,
		); err != nil {
			return nil, err
		}
		chunks = append(chunks, chunk)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	expected := segment.EndSequence - segment.StartSequence + 1
	if segment.StartSequence > 0 {
		expected++
	}
	if len(chunks) != expected {
		return nil, ErrNotReady
	}
	return chunks, nil
}

func (repository *PostgresRepository) CompleteLiveTranscriptSegment(
	ctx context.Context,
	segment LiveTranscriptSegment,
	transcriptText string,
	mergedTranscriptText string,
) error {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	command, err := tx.Exec(ctx, `
		update queue.attendance_live_transcription_segments
		set
			status = 'completed',
			transcript_text = $3,
			error_message = '',
			locked_at = null,
			locked_by = '',
			updated_at = now()
		where account_id = $1::uuid
		  and id = $2::uuid
		  and status = 'processing';
	`, segment.AccountID, segment.ID, transcriptText)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return ErrNotFound
	}

	_, err = tx.Exec(ctx, `
		update queue.attendance_recordings
		set
			live_transcript_text = $3,
			live_transcript_updated_at = now(),
			updated_at = now()
		where account_id = $1::uuid
		  and id = $2::uuid
		  and transcription_status <> 'completed';
	`, segment.AccountID, segment.RecordingID, mergedTranscriptText)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (repository *PostgresRepository) FailLiveTranscriptSegment(
	ctx context.Context,
	segment LiveTranscriptSegment,
	errorMessage string,
	retryAt *time.Time,
) error {
	status := TranscriptionStatusFailed
	if retryAt != nil {
		status = TranscriptionStatusPending
	}
	command, err := repository.pool.Exec(ctx, `
		update queue.attendance_live_transcription_segments
		set
			status = $3,
			error_message = $4,
			next_attempt_at = coalesce($5, next_attempt_at),
			locked_at = null,
			locked_by = '',
			updated_at = now()
		where account_id = $1::uuid
		  and id = $2::uuid;
	`, segment.AccountID, segment.ID, status, errorMessage, retryAt)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (repository *PostgresRepository) CompleteRecording(ctx context.Context, accountID, recordingID string, endedAt int64, audio ConsolidatedAudio) (Recording, error) {
	command, err := repository.pool.Exec(ctx, `
		update queue.attendance_recordings
		set
			recording_status = 'ready',
			ended_at = $3,
			audio_storage_key = $4,
			audio_sha256 = $5,
			size_bytes = $6,
			updated_at = now()
		where account_id = $1::uuid
		  and id = $2::uuid;
	`, accountID, recordingID, endedAt, audio.StorageKey, audio.SHA256, audio.SizeBytes)
	if err != nil {
		return Recording{}, err
	}
	if command.RowsAffected() == 0 {
		return Recording{}, ErrNotFound
	}
	return repository.GetRecording(ctx, accountID, recordingID)
}

func (repository *PostgresRepository) RequestTranscription(ctx context.Context, accountID, recordingID string) (Recording, error) {
	command, err := repository.pool.Exec(ctx, `
		update queue.attendance_recordings
		set
			transcription_status = case
				when transcription_status in ('completed', 'processing') then transcription_status
				else 'pending'
			end,
			transcription_requested_at = coalesce(transcription_requested_at, now()),
			transcription_next_attempt_at = case
				when transcription_status in ('completed', 'processing') then transcription_next_attempt_at
				else now()
			end,
			transcription_locked_at = case
				when transcription_status = 'processing' then transcription_locked_at
				else null
			end,
			transcription_locked_by = case
				when transcription_status = 'processing' then transcription_locked_by
				else ''
			end,
			transcription_attempt_count = case
				when transcription_status = 'failed' then 0
				else transcription_attempt_count
			end,
			transcript_error = case
				when transcription_status in ('completed', 'processing') then transcript_error
				else ''
			end,
			updated_at = now()
		where account_id = $1::uuid
		  and id = $2::uuid
		  and recording_status = 'ready'
		  and audio_storage_key is not null;
	`, accountID, recordingID)
	if err != nil {
		return Recording{}, err
	}
	if command.RowsAffected() == 0 {
		return Recording{}, ErrNotReady
	}
	return repository.GetRecording(ctx, accountID, recordingID)
}

func (repository *PostgresRepository) ClaimTranscription(ctx context.Context, workerID string, lease time.Duration) (Recording, error) {
	leaseSeconds := max(1, int(lease.Seconds()))
	recording, err := scanRecording(repository.pool.QueryRow(ctx, `
		with candidate as (
			select account_id, id
			from queue.attendance_recordings
			where recording_status = 'ready'
			  and audio_storage_key is not null
			  and transcription_requested_at is not null
			  and transcription_attempt_count < 3
			  and (
				(
					transcription_status = 'pending'
					and coalesce(transcription_next_attempt_at, now()) <= now()
				)
				or (
					transcription_status = 'processing'
					and transcription_locked_at < now() - make_interval(secs => $2)
				)
			  )
			order by coalesce(transcription_next_attempt_at, transcription_requested_at), id
			for update skip locked
			limit 1
		),
		claimed as (
			update queue.attendance_recordings recording
			set
				transcription_status = 'processing',
				transcription_attempt_count = recording.transcription_attempt_count + 1,
				transcription_locked_at = now(),
				transcription_locked_by = $1,
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

func (repository *PostgresRepository) CompleteTranscription(ctx context.Context, accountID, recordingID, transcriptText string) error {
	command, err := repository.pool.Exec(ctx, `
		update queue.attendance_recordings
		set
			transcription_status = 'completed',
			transcript_text = $3,
			transcript_error = '',
			transcription_next_attempt_at = null,
			transcription_locked_at = null,
			transcription_locked_by = '',
			analysis_status = case
				when coalesce((
					select config.enabled
					from queue.attendance_analysis_configs config
					where config.account_id = $1::uuid
				), true) then 'pending'
				else 'not_requested'
			end,
			analysis_requested_at = case
				when coalesce((
					select config.enabled
					from queue.attendance_analysis_configs config
					where config.account_id = $1::uuid
				), true) then now()
				else null
			end,
			analysis_next_attempt_at = case
				when coalesce((
					select config.enabled
					from queue.attendance_analysis_configs config
					where config.account_id = $1::uuid
				), true) then now()
				else null
			end,
			analysis_error = '',
			updated_at = now()
		where account_id = $1::uuid
		  and id = $2::uuid
		  and transcription_status = 'processing';
	`, accountID, recordingID, transcriptText)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (repository *PostgresRepository) FailTranscription(ctx context.Context, accountID, recordingID, errorMessage string, retryAt *time.Time) error {
	status := TranscriptionStatusFailed
	if retryAt != nil {
		status = TranscriptionStatusPending
	}
	command, err := repository.pool.Exec(ctx, `
		update queue.attendance_recordings
		set
			transcription_status = $3,
			transcript_error = $4,
			transcription_next_attempt_at = $5,
			transcription_locked_at = null,
			transcription_locked_by = '',
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

func (repository *PostgresRepository) ListRecordings(ctx context.Context, accountID string, filter ListFilter) ([]Recording, int, error) {
	listArgs := []any{accountID, filter.Limit, filter.Offset}
	countArgs := []any{accountID}
	listWhere := " where recording.account_id = $1::uuid"
	countWhere := listWhere
	if filter.StoreID != "" {
		listArgs = append(listArgs, filter.StoreID)
		countArgs = append(countArgs, filter.StoreID)
		listWhere += fmt.Sprintf(" and recording.store_id = $%d::uuid", len(listArgs))
		countWhere += " and recording.store_id = $2::uuid"
	} else if len(filter.StoreIDs) > 0 {
		listArgs = append(listArgs, filter.StoreIDs)
		countArgs = append(countArgs, filter.StoreIDs)
		listWhere += fmt.Sprintf(" and recording.store_id = any($%d::uuid[])", len(listArgs))
		countWhere += " and recording.store_id = any($2::uuid[])"
	}
	if filter.ConsultantID != "" {
		listArgs = append(listArgs, filter.ConsultantID)
		countArgs = append(countArgs, filter.ConsultantID)
		listWhere += fmt.Sprintf(" and recording.consultant_id = $%d::uuid", len(listArgs))
		countWhere += fmt.Sprintf(" and recording.consultant_id = $%d::uuid", len(countArgs))
	}

	var total int
	if err := repository.pool.QueryRow(ctx, `
		select count(*)
		from queue.attendance_recordings recording
		join queue.stores store
		  on store.id = recording.store_id
		 and store.tenant_id = recording.account_id
	`+countWhere, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := strings.TrimSpace(recordingSelect) + listWhere + `
		order by recording.started_at desc, recording.id desc
		limit $2 offset $3;
	`
	rows, err := repository.pool.Query(ctx, query, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	recordings := make([]Recording, 0)
	for rows.Next() {
		recording, err := scanRecording(rows)
		if err != nil {
			return nil, 0, err
		}
		recordings = append(recordings, recording)
	}
	return recordings, total, rows.Err()
}
