package transcriptions

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"
	"unicode"
)

const (
	transcriptionLease              = 12 * time.Minute
	transcriptionPollInterval       = time.Second
	maxTranscriptionAttempts        = 3
	maxLiveWindowBytes        int64 = 16 << 20
)

type Worker struct {
	repository  Repository
	storage     AudioStorage
	transcriber Transcriber
	logger      *slog.Logger
	workerID    string
}

func NewWorker(repository Repository, storage AudioStorage, transcriber Transcriber, logger *slog.Logger) *Worker {
	return &Worker{
		repository:  repository,
		storage:     storage,
		transcriber: transcriber,
		logger:      logger,
		workerID:    "queue-transcriptions",
	}
}

func (worker *Worker) Run(ctx context.Context) {
	go worker.runLive(ctx)
	worker.runFinal(ctx)
}

func (worker *Worker) runFinal(ctx context.Context) {
	ticker := time.NewTicker(transcriptionPollInterval)
	defer ticker.Stop()

	for {
		for {
			processed, err := worker.processNext(ctx)
			if err != nil && !errors.Is(err, context.Canceled) {
				worker.logger.Warn("attendance_transcription_worker_failed", "error", err)
			}
			if !processed || err != nil {
				break
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (worker *Worker) runLive(ctx context.Context) {
	ticker := time.NewTicker(transcriptionPollInterval)
	defer ticker.Stop()

	for {
		for {
			processed, err := worker.processNextLive(ctx)
			if err != nil && !errors.Is(err, context.Canceled) {
				worker.logger.Warn("attendance_live_transcription_worker_failed", "error", err)
			}
			if !processed || err != nil {
				break
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (worker *Worker) processNext(ctx context.Context) (bool, error) {
	recording, err := worker.repository.ClaimTranscription(ctx, worker.workerID, transcriptionLease)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	extension, _ := audioExtension(recording.MimeType)
	audio, err := worker.storage.Open(
		recording.AudioStorageKey,
		recording.MimeType,
		audioDownloadName(recording)+extension,
	)
	if err != nil {
		return true, worker.persistFailure(ctx, recording, err)
	}
	defer func() { _ = audio.File.Close() }()

	transcriptText, err := worker.transcribe(ctx, recording, audio)
	if err != nil {
		return true, worker.persistFailure(ctx, recording, err)
	}
	if err := worker.repository.CompleteTranscription(
		ctx,
		recording.AccountID,
		recording.ID,
		transcriptText,
	); err != nil {
		return true, err
	}
	worker.logger.Info("attendance_transcription_completed", "recording_id", recording.ID)
	return true, nil
}

func (worker *Worker) transcribe(
	ctx context.Context,
	recording Recording,
	audio OpenedAudio,
) (string, error) {
	if configured, ok := worker.transcriber.(ConfiguredTranscriber); ok {
		options := TranscriptionOptions{}
		if configs, ok := worker.repository.(AnalysisRepository); ok {
			config, configErr := configs.GetAnalysisConfig(ctx, recording.AccountID)
			if configErr != nil {
				return "", configErr
			}
			options.Model = config.TranscriptionModel
			options.Language = config.TranscriptionLanguage
		}
		return configured.TranscribeWithOptions(ctx, audio, options)
	}
	return worker.transcriber.Transcribe(ctx, audio)
}

func (worker *Worker) processNextLive(ctx context.Context) (bool, error) {
	repository, ok := worker.repository.(LiveTranscriptionRepository)
	if !ok {
		return false, nil
	}
	storage, ok := worker.storage.(LiveAudioStorage)
	if !ok {
		return false, nil
	}

	segment, err := repository.ClaimLiveTranscriptSegment(
		ctx,
		worker.workerID+"-live",
		transcriptionLease,
	)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	recording, err := worker.repository.GetRecording(ctx, segment.AccountID, segment.RecordingID)
	if err != nil {
		return true, worker.persistLiveFailure(ctx, repository, segment, err)
	}
	chunks, err := repository.ListLiveTranscriptChunks(ctx, segment)
	if err != nil {
		return true, worker.persistLiveFailure(ctx, repository, segment, err)
	}
	audio, err := storage.BuildLiveWindow(
		ctx,
		segment.AccountID,
		segment.RecordingID,
		recording.MimeType,
		segment,
		chunks,
		maxLiveWindowBytes,
	)
	if err != nil {
		return true, worker.persistLiveFailure(ctx, repository, segment, err)
	}
	defer cleanupOpenedAudio(audio)

	transcriptText, err := worker.transcribe(ctx, recording, audio)
	if err != nil {
		return true, worker.persistLiveFailure(ctx, repository, segment, err)
	}
	mergedTranscriptText := mergeTranscriptText(recording.LiveTranscriptText, transcriptText)
	if err := repository.CompleteLiveTranscriptSegment(
		ctx,
		segment,
		transcriptText,
		mergedTranscriptText,
	); err != nil {
		return true, err
	}
	worker.logger.Info(
		"attendance_live_transcription_completed",
		"recording_id", segment.RecordingID,
		"segment_index", segment.SegmentIndex,
	)
	return true, nil
}

func cleanupOpenedAudio(audio OpenedAudio) {
	if audio.File == nil {
		return
	}
	path := audio.File.Name()
	_ = audio.File.Close()
	if audio.Temporary {
		_ = os.Remove(path) //nolint:gosec // path vem de os.CreateTemp no storage privado.
	}
}

func normalizedTranscriptWord(value string) string {
	return strings.Map(func(character rune) rune {
		if unicode.IsLetter(character) || unicode.IsNumber(character) {
			return unicode.ToLower(character)
		}
		return -1
	}, value)
}

func mergeTranscriptText(existing, incoming string) string {
	existing = strings.TrimSpace(existing)
	incoming = strings.TrimSpace(incoming)
	if existing == "" {
		return incoming
	}
	if incoming == "" {
		return existing
	}

	existingWords := strings.Fields(existing)
	incomingWords := strings.Fields(incoming)
	maxOverlap := min(48, len(existingWords), len(incomingWords))
	overlap := 0
	for size := maxOverlap; size >= 2; size-- {
		matches := true
		for index := 0; index < size; index++ {
			left := normalizedTranscriptWord(existingWords[len(existingWords)-size+index])
			right := normalizedTranscriptWord(incomingWords[index])
			if left == "" || left != right {
				matches = false
				break
			}
		}
		if matches {
			overlap = size
			break
		}
	}
	if overlap == len(incomingWords) {
		return existing
	}
	return strings.TrimSpace(existing + " " + strings.Join(incomingWords[overlap:], " "))
}

func (worker *Worker) persistLiveFailure(
	ctx context.Context,
	repository LiveTranscriptionRepository,
	segment LiveTranscriptSegment,
	cause error,
) error {
	var retryAt *time.Time
	if segment.AttemptCount < maxTranscriptionAttempts {
		delay := 5 * time.Second
		if segment.AttemptCount > 1 {
			delay = 30 * time.Second
		}
		nextAttempt := time.Now().UTC().Add(delay)
		retryAt = &nextAttempt
	}
	message := "Nao foi possivel transcrever esta janela ao vivo."
	if errors.Is(cause, context.DeadlineExceeded) {
		message = "O Whisper excedeu o tempo limite desta janela ao vivo."
	}
	if err := repository.FailLiveTranscriptSegment(
		ctx,
		segment,
		message,
		retryAt,
	); err != nil {
		return fmt.Errorf("persist attendance live transcription failure: %w", err)
	}
	worker.logger.Warn(
		"attendance_live_transcription_attempt_failed",
		"recording_id", segment.RecordingID,
		"segment_index", segment.SegmentIndex,
		"attempt", segment.AttemptCount,
		"will_retry", retryAt != nil,
	)
	return nil
}

func (worker *Worker) persistFailure(ctx context.Context, recording Recording, cause error) error {
	var retryAt *time.Time
	if recording.TranscriptionAttemptCount < maxTranscriptionAttempts {
		delay := 5 * time.Second
		if recording.TranscriptionAttemptCount > 1 {
			delay = 30 * time.Second
		}
		nextAttempt := time.Now().UTC().Add(delay)
		retryAt = &nextAttempt
	}
	message := "Whisper local indisponivel. Tente novamente em instantes."
	if errors.Is(cause, context.DeadlineExceeded) {
		message = "O Whisper excedeu o tempo limite da transcricao."
	}
	if err := worker.repository.FailTranscription(
		ctx,
		recording.AccountID,
		recording.ID,
		message,
		retryAt,
	); err != nil {
		return fmt.Errorf("persist attendance transcription failure: %w", err)
	}
	failureKind := "transcriber"
	if errors.Is(cause, context.DeadlineExceeded) {
		failureKind = "timeout"
	}
	worker.logger.Warn(
		"attendance_transcription_attempt_failed",
		"recording_id", recording.ID,
		"attempt", recording.TranscriptionAttemptCount,
		"will_retry", retryAt != nil,
		"failure_kind", failureKind,
	)
	return nil
}
