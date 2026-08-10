package transcriptions

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"time"
)

const (
	RecordingStatusRecording   = "recording"
	RecordingStatusReady       = "ready"
	RecordingStatusInterrupted = "interrupted"
	RecordingStatusFailed      = "failed"

	TranscriptionStatusPending    = "pending"
	TranscriptionStatusProcessing = "processing"
	TranscriptionStatusCompleted  = "completed"
	TranscriptionStatusFailed     = "failed"

	AnalysisStatusNotRequested = "not_requested"
	AnalysisStatusPending      = "pending"
	AnalysisStatusProcessing   = "processing"
	AnalysisStatusCompleted    = "completed"
	AnalysisStatusFailed       = "failed"
)

type AccessContext struct {
	UserID              string
	AccountID           string
	Role                string
	StoreIDs            []string
	Permissions         []string
	PermissionsResolved bool
}

type CreateRecordingInput struct {
	StoreID         string `json:"storeId"`
	ServiceID       string `json:"serviceId"`
	ClientSessionID string `json:"clientSessionId"`
	MimeType        string `json:"mimeType"`
	StartedAt       int64  `json:"startedAt"`
}

type CompleteRecordingInput struct {
	EndedAt int64 `json:"endedAt"`
}

type ServiceReference struct {
	StoreID        string
	StoreName      string
	ServiceID      string
	ConsultantID   string
	ConsultantName string
	StartedAt      int64
	FinishedAt     int64
	FinishOutcome  string
}

type Recording struct {
	ID                        string
	AccountID                 string
	StoreID                   string
	StoreName                 string
	ServiceID                 string
	ConsultantID              string
	ConsultantName            string
	ClientSessionID           string
	RecordingStatus           string
	TranscriptionStatus       string
	MimeType                  string
	StartedAt                 int64
	EndedAt                   int64
	ChunkCount                int
	SizeBytes                 int64
	AudioStorageKey           string
	AudioSHA256               string
	TranscriptText            string
	LiveTranscriptText        string
	LiveTranscriptUpdatedAt   *time.Time
	TranscriptError           string
	TranscriptionRequestedAt  *time.Time
	TranscriptionAttemptCount int
	AnalysisStatus            string
	SummaryText               string
	AnalysisReport            json.RawMessage
	AnalysisError             string
	AnalysisRequestedAt       *time.Time
	AnalysisAttemptCount      int
	CreatedBy                 string
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
	FinishOutcome             string
}

type RecordingView struct {
	ID                      string          `json:"id"`
	StoreID                 string          `json:"storeId"`
	StoreName               string          `json:"storeName"`
	ServiceID               string          `json:"serviceId"`
	ConsultantID            string          `json:"consultantId"`
	ConsultantName          string          `json:"consultantName"`
	RecordingStatus         string          `json:"recordingStatus"`
	TranscriptionStatus     string          `json:"transcriptionStatus"`
	MimeType                string          `json:"mimeType"`
	StartedAt               int64           `json:"startedAt"`
	EndedAt                 int64           `json:"endedAt,omitempty"`
	ChunkCount              int             `json:"chunkCount"`
	SizeBytes               int64           `json:"sizeBytes"`
	HasAudio                bool            `json:"hasAudio"`
	TranscriptText          string          `json:"transcriptText"`
	TranscriptLive          bool            `json:"transcriptLive"`
	LiveTranscriptUpdatedAt *time.Time      `json:"liveTranscriptUpdatedAt,omitempty"`
	TranscriptError         string          `json:"transcriptError,omitempty"`
	TranscriptionRequested  bool            `json:"transcriptionRequested"`
	AnalysisStatus          string          `json:"analysisStatus"`
	SummaryText             string          `json:"summaryText"`
	AnalysisReport          json.RawMessage `json:"analysisReport"`
	AnalysisError           string          `json:"analysisError,omitempty"`
	AnalysisRequested       bool            `json:"analysisRequested"`
	FinishOutcome           string          `json:"finishOutcome,omitempty"`
	CreatedAt               time.Time       `json:"createdAt"`
	UpdatedAt               time.Time       `json:"updatedAt"`
}

func recordingView(recording Recording) RecordingView {
	transcriptText := recording.TranscriptText
	transcriptLive := false
	if recording.TranscriptionStatus != TranscriptionStatusCompleted &&
		transcriptText == "" &&
		recording.LiveTranscriptText != "" {
		transcriptText = recording.LiveTranscriptText
		transcriptLive = true
	}
	return RecordingView{
		ID:                      recording.ID,
		StoreID:                 recording.StoreID,
		StoreName:               recording.StoreName,
		ServiceID:               recording.ServiceID,
		ConsultantID:            recording.ConsultantID,
		ConsultantName:          recording.ConsultantName,
		RecordingStatus:         recording.RecordingStatus,
		TranscriptionStatus:     recording.TranscriptionStatus,
		MimeType:                recording.MimeType,
		StartedAt:               recording.StartedAt,
		EndedAt:                 recording.EndedAt,
		ChunkCount:              recording.ChunkCount,
		SizeBytes:               recording.SizeBytes,
		HasAudio:                recording.AudioStorageKey != "",
		TranscriptText:          transcriptText,
		TranscriptLive:          transcriptLive,
		LiveTranscriptUpdatedAt: recording.LiveTranscriptUpdatedAt,
		TranscriptError:         recording.TranscriptError,
		TranscriptionRequested:  recording.TranscriptionRequestedAt != nil,
		AnalysisStatus:          recording.AnalysisStatus,
		SummaryText:             recording.SummaryText,
		AnalysisReport:          recording.AnalysisReport,
		AnalysisError:           recording.AnalysisError,
		AnalysisRequested:       recording.AnalysisRequestedAt != nil,
		FinishOutcome:           recording.FinishOutcome,
		CreatedAt:               recording.CreatedAt,
		UpdatedAt:               recording.UpdatedAt,
	}
}

type Chunk struct {
	RecordingID string
	Sequence    int
	StorageKey  string
	MimeType    string
	SizeBytes   int64
	SHA256      string
}

type StoredChunk struct {
	StorageKey string
	MimeType   string
	SizeBytes  int64
	SHA256     string
}

type ConsolidatedAudio struct {
	StorageKey string
	SizeBytes  int64
	SHA256     string
}

type OpenedAudio struct {
	File      *os.File
	FileName  string
	MimeType  string
	ModTime   time.Time
	Temporary bool
}

type LiveTranscriptSegment struct {
	ID            string
	AccountID     string
	RecordingID   string
	SegmentIndex  int
	StartSequence int
	EndSequence   int
	TrimStartMS   int
	AttemptCount  int
}

type ListFilter struct {
	StoreID      string
	StoreIDs     []string
	ConsultantID string
	Limit        int
	Offset       int
}

type ListResponse struct {
	Items  []RecordingView `json:"items"`
	Total  int             `json:"total"`
	Limit  int             `json:"limit"`
	Offset int             `json:"offset"`
}

type RecordingFeature struct {
	AccountID string     `json:"accountId"`
	Enabled   bool       `json:"enabled"`
	UpdatedAt *time.Time `json:"updatedAt"`
	UpdatedBy string     `json:"updatedBy,omitempty"`
}

type PutRecordingFeatureInput struct {
	Enabled bool `json:"enabled"`
}

type Repository interface {
	GetRecordingFeature(ctx context.Context, accountID string) (RecordingFeature, error)
	PutRecordingFeature(ctx context.Context, accountID string, enabled bool, updatedBy string) (RecordingFeature, error)
	ResolveService(ctx context.Context, accountID, storeID, serviceID string) (ServiceReference, error)
	CreateRecording(ctx context.Context, recording Recording) (Recording, error)
	GetRecording(ctx context.Context, accountID, recordingID string) (Recording, error)
	GetChunk(ctx context.Context, accountID, recordingID string, sequence int) (Chunk, error)
	SaveChunk(ctx context.Context, accountID string, chunk Chunk) (Recording, error)
	ListChunks(ctx context.Context, accountID, recordingID string) ([]Chunk, error)
	CompleteRecording(ctx context.Context, accountID, recordingID string, endedAt int64, audio ConsolidatedAudio) (Recording, error)
	RequestTranscription(ctx context.Context, accountID, recordingID string) (Recording, error)
	ClaimTranscription(ctx context.Context, workerID string, lease time.Duration) (Recording, error)
	CompleteTranscription(ctx context.Context, accountID, recordingID, transcriptText string) error
	FailTranscription(ctx context.Context, accountID, recordingID, errorMessage string, retryAt *time.Time) error
	ListRecordings(ctx context.Context, accountID string, filter ListFilter) ([]Recording, int, error)
}

type Transcriber interface {
	Transcribe(ctx context.Context, audio OpenedAudio) (string, error)
}

type TranscriptionOptions struct {
	Model    string
	Language string
}

type ConfiguredTranscriber interface {
	TranscribeWithOptions(ctx context.Context, audio OpenedAudio, options TranscriptionOptions) (string, error)
}

type LiveTranscriptionRepository interface {
	ClaimLiveTranscriptSegment(ctx context.Context, workerID string, lease time.Duration) (LiveTranscriptSegment, error)
	ListLiveTranscriptChunks(ctx context.Context, segment LiveTranscriptSegment) ([]Chunk, error)
	CompleteLiveTranscriptSegment(ctx context.Context, segment LiveTranscriptSegment, transcriptText, mergedTranscriptText string) error
	FailLiveTranscriptSegment(ctx context.Context, segment LiveTranscriptSegment, errorMessage string, retryAt *time.Time) error
}

type LiveAudioStorage interface {
	BuildLiveWindow(ctx context.Context, accountID, recordingID, mimeType string, segment LiveTranscriptSegment, chunks []Chunk, maxBytes int64) (OpenedAudio, error)
}

type AnalysisConfig struct {
	Enabled               bool    `json:"enabled"`
	TranscriptionProvider string  `json:"transcriptionProvider"`
	TranscriptionModel    string  `json:"transcriptionModel"`
	TranscriptionLanguage string  `json:"transcriptionLanguage"`
	CredentialID          string  `json:"credentialId"`
	Provider              string  `json:"provider"`
	Model                 string  `json:"model"`
	SystemPrompt          string  `json:"systemPrompt"`
	Temperature           float64 `json:"temperature"`
}

type AnalysisConfigView struct {
	AnalysisConfig
}

type PutAnalysisConfigInput struct {
	AnalysisConfig
}

type AnalysisResult struct {
	Summary string          `json:"summary"`
	Report  json.RawMessage `json:"report"`
}

type AnalysisRepository interface {
	GetAnalysisConfig(ctx context.Context, accountID string) (AnalysisConfig, error)
	PutAnalysisConfig(ctx context.Context, accountID string, config AnalysisConfig, updatedBy string) error
	RequestAnalysis(ctx context.Context, accountID, recordingID string) (Recording, error)
	ClaimAnalysis(ctx context.Context, workerID string, lease time.Duration) (Recording, error)
	CompleteAnalysis(ctx context.Context, accountID, recordingID string, result AnalysisResult, configSnapshot json.RawMessage) error
	FailAnalysis(ctx context.Context, accountID, recordingID, errorMessage string, retryAt *time.Time) error
}

type AttendanceAnalyzer interface {
	Analyze(ctx context.Context, recording Recording, config AnalysisConfig, apiKey string) (AnalysisResult, error)
}

type RuntimeCredential struct {
	ID       string
	Provider string
	APIKey   string
}

type CredentialResolver interface {
	ResolveCredential(ctx context.Context, accountID, credentialID string) (RuntimeCredential, error)
}

type AudioStorage interface {
	SaveChunk(accountID, recordingID string, sequence int, mimeType string, source io.Reader, maxBytes int64) (StoredChunk, error)
	Consolidate(accountID, recordingID, mimeType string, chunks []Chunk, maxBytes int64) (ConsolidatedAudio, error)
	Open(storageKey, mimeType, fileName string) (OpenedAudio, error)
	Remove(storageKey string) error
}
