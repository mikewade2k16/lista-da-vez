package transcriptions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

type fakeRepository struct {
	featureEnabled bool
	reference      ServiceReference
	recording      Recording
	chunks         map[int]Chunk
	listFilter     ListFilter
}

func (repository *fakeRepository) ExperimentalFeatureEnabled(context.Context) (bool, error) {
	return repository.featureEnabled, nil
}

func (repository *fakeRepository) ResolveService(context.Context, string, string, string) (ServiceReference, error) {
	if repository.reference.ServiceID == "" {
		return ServiceReference{}, ErrNotFound
	}
	return repository.reference, nil
}

func (repository *fakeRepository) CreateRecording(_ context.Context, recording Recording) (Recording, error) {
	recording.ID = "recording-1"
	repository.recording = recording
	return recording, nil
}

func (repository *fakeRepository) GetRecording(_ context.Context, _, _ string) (Recording, error) {
	if repository.recording.ID == "" {
		return Recording{}, ErrNotFound
	}
	return repository.recording, nil
}

func (repository *fakeRepository) GetChunk(_ context.Context, _, _ string, sequence int) (Chunk, error) {
	chunk, ok := repository.chunks[sequence]
	if !ok {
		return Chunk{}, ErrNotFound
	}
	return chunk, nil
}

func (repository *fakeRepository) SaveChunk(_ context.Context, _ string, chunk Chunk) (Recording, error) {
	repository.chunks[chunk.Sequence] = chunk
	repository.recording.ChunkCount = len(repository.chunks)
	repository.recording.SizeBytes += chunk.SizeBytes
	return repository.recording, nil
}

func (repository *fakeRepository) ListChunks(context.Context, string, string) ([]Chunk, error) {
	result := make([]Chunk, 0, len(repository.chunks))
	for sequence := 0; sequence < len(repository.chunks); sequence++ {
		result = append(result, repository.chunks[sequence])
	}
	return result, nil
}

func (repository *fakeRepository) CompleteRecording(_ context.Context, _, _ string, endedAt int64, audio ConsolidatedAudio) (Recording, error) {
	repository.recording.RecordingStatus = RecordingStatusReady
	repository.recording.EndedAt = endedAt
	repository.recording.AudioStorageKey = audio.StorageKey
	repository.recording.AudioSHA256 = audio.SHA256
	repository.recording.SizeBytes = audio.SizeBytes
	return repository.recording, nil
}

func (repository *fakeRepository) RequestTranscription(_ context.Context, _, _ string) (Recording, error) {
	now := time.Now()
	repository.recording.TranscriptionRequestedAt = &now
	repository.recording.TranscriptionStatus = TranscriptionStatusPending
	return repository.recording, nil
}

func (repository *fakeRepository) ClaimTranscription(context.Context, string, time.Duration) (Recording, error) {
	return Recording{}, ErrNotFound
}

func (repository *fakeRepository) CompleteTranscription(context.Context, string, string, string) error {
	return nil
}

func (repository *fakeRepository) FailTranscription(context.Context, string, string, string, *time.Time) error {
	return nil
}

func (repository *fakeRepository) ListRecordings(_ context.Context, _ string, filter ListFilter) ([]Recording, int, error) {
	repository.listFilter = filter
	return []Recording{repository.recording}, 1, nil
}

type fakeStorage struct {
	saveCalls int
}

func (storage *fakeStorage) SaveChunk(_ string, recordingID string, sequence int, mimeType string, source io.Reader, _ int64) (StoredChunk, error) {
	content, err := io.ReadAll(source)
	if err != nil {
		return StoredChunk{}, err
	}
	storage.saveCalls++
	digest := sha256.Sum256(content)
	return StoredChunk{
		StorageKey: "account/" + recordingID + "/chunk",
		MimeType:   mimeType,
		SizeBytes:  int64(len(content)),
		SHA256:     hex.EncodeToString(digest[:]),
	}, nil
}

func (storage *fakeStorage) Consolidate(string, string, string, []Chunk, int64) (ConsolidatedAudio, error) {
	return ConsolidatedAudio{StorageKey: "account/recording/audio.webm", SizeBytes: 10, SHA256: "final"}, nil
}

func (storage *fakeStorage) Open(string, string, string) (OpenedAudio, error) {
	return OpenedAudio{}, ErrNotFound
}

func (storage *fakeStorage) Remove(string) error {
	return nil
}

func testAccess() AccessContext {
	return AccessContext{
		UserID:      "user-1",
		AccountID:   "account-1",
		Role:        "manager",
		StoreIDs:    []string{"store-1"},
		Permissions: nil,
	}
}

func testRepository() *fakeRepository {
	return &fakeRepository{
		featureEnabled: true,
		reference: ServiceReference{
			StoreID:        "store-1",
			StoreName:      "Centro",
			ServiceID:      "service-1",
			ConsultantID:   "consultant-1",
			ConsultantName: "Ana",
			StartedAt:      100,
		},
		chunks: make(map[int]Chunk),
	}
}

func TestCreateUsesAuthoritativeServiceReference(t *testing.T) {
	t.Parallel()
	repository := testRepository()
	service := NewService(repository, &fakeStorage{})

	view, err := service.Create(context.Background(), testAccess(), CreateRecordingInput{
		StoreID:         "store-1",
		ServiceID:       "service-1",
		ClientSessionID: "browser-session",
		MimeType:        "audio/webm;codecs=opus",
		StartedAt:       999,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if view.ConsultantName != "Ana" || view.StartedAt != 100 {
		t.Fatalf("view = %#v, want authoritative consultant and start", view)
	}
	if repository.recording.AccountID != "account-1" {
		t.Fatalf("account = %q, want principal account", repository.recording.AccountID)
	}
}

func TestCreateFailsClosedWhenFeatureDisabled(t *testing.T) {
	t.Parallel()
	repository := testRepository()
	repository.featureEnabled = false
	service := NewService(repository, &fakeStorage{})

	_, err := service.Create(context.Background(), testAccess(), CreateRecordingInput{
		StoreID:         "store-1",
		ServiceID:       "service-1",
		ClientSessionID: "browser-session",
		MimeType:        "audio/webm",
	})
	if !errors.Is(err, ErrFeatureDisabled) {
		t.Fatalf("err = %v, want ErrFeatureDisabled", err)
	}
}

func TestSaveChunkIsIdempotent(t *testing.T) {
	t.Parallel()
	repository := testRepository()
	repository.recording = Recording{
		ID:              "recording-1",
		AccountID:       "account-1",
		StoreID:         "store-1",
		MimeType:        "audio/webm",
		RecordingStatus: RecordingStatusRecording,
	}
	storage := &fakeStorage{}
	service := NewService(repository, storage)

	if _, err := service.SaveChunk(
		context.Background(),
		testAccess(),
		"recording-1",
		0,
		"audio/webm;codecs=opus",
		strings.NewReader("audio"),
	); err != nil {
		t.Fatalf("first SaveChunk: %v", err)
	}
	if _, err := service.SaveChunk(
		context.Background(),
		testAccess(),
		"recording-1",
		0,
		"audio/webm",
		strings.NewReader("audio"),
	); err != nil {
		t.Fatalf("idempotent SaveChunk: %v", err)
	}
	if storage.saveCalls != 1 {
		t.Fatalf("save calls = %d, want 1", storage.saveCalls)
	}
}

func TestListRestrictsStoreScopedRoles(t *testing.T) {
	t.Parallel()
	repository := testRepository()
	service := NewService(repository, &fakeStorage{})

	if _, err := service.List(context.Background(), testAccess(), ListFilter{}); err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(repository.listFilter.StoreIDs) != 1 || repository.listFilter.StoreIDs[0] != "store-1" {
		t.Fatalf("store filter = %#v, want manager store", repository.listFilter.StoreIDs)
	}
}

func TestRequestTranscriptionUsesScopedReadyRecording(t *testing.T) {
	t.Parallel()
	repository := testRepository()
	repository.recording = Recording{
		ID:              "recording-1",
		AccountID:       "account-1",
		StoreID:         "store-1",
		RecordingStatus: RecordingStatusReady,
		AudioStorageKey: "account/recording/audio.webm",
	}
	service := NewService(repository, &fakeStorage{})

	view, err := service.RequestTranscription(context.Background(), testAccess(), "recording-1")
	if err != nil {
		t.Fatalf("RequestTranscription: %v", err)
	}
	if !view.TranscriptionRequested || view.TranscriptionStatus != TranscriptionStatusPending {
		t.Fatalf("view = %#v, want durable pending request", view)
	}
}

func TestCompleteAutomaticallyRequestsTranscription(t *testing.T) {
	t.Parallel()
	repository := testRepository()
	repository.recording = Recording{
		ID:                  "recording-1",
		AccountID:           "account-1",
		StoreID:             "store-1",
		MimeType:            "audio/webm",
		RecordingStatus:     RecordingStatusRecording,
		TranscriptionStatus: TranscriptionStatusPending,
	}
	repository.chunks[0] = Chunk{RecordingID: "recording-1", Sequence: 0}
	service := NewService(repository, &fakeStorage{})

	view, err := service.Complete(
		context.Background(),
		testAccess(),
		"recording-1",
		CompleteRecordingInput{EndedAt: 200},
	)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if view.RecordingStatus != RecordingStatusReady || !view.TranscriptionRequested {
		t.Fatalf("view = %#v, want ready recording queued for transcription", view)
	}
}
