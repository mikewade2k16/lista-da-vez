package transcriptions

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"strings"

	accesscontrol "github.com/mikewade2k16/lista-da-vez/back/internal/modules/access"
)

const (
	maxChunkBytes     int64 = 2 << 20
	maxRecordingBytes int64 = 1 << 30
	maxChunkSequence        = 100_000
)

type Service struct {
	repository  Repository
	storage     AudioStorage
	analysis    AnalysisRepository
	credentials CredentialResolver
}

func NewService(repository Repository, storage AudioStorage, credentials ...CredentialResolver) *Service {
	service := &Service{repository: repository, storage: storage}
	service.analysis, _ = repository.(AnalysisRepository)
	if len(credentials) > 0 {
		service.credentials = credentials[0]
	}
	return service
}

func canRead(access AccessContext) bool {
	if access.PermissionsResolved {
		return accesscontrol.HasPermission(
			access.Permissions,
			accesscontrol.PermissionOperationsView,
		)
	}
	switch access.Role {
	case "consultant", "store_terminal", "manager", "marketing", "director", "owner", "platform_admin":
		return true
	default:
		return false
	}
}

func canWrite(access AccessContext) bool {
	if access.PermissionsResolved {
		return accesscontrol.HasPermission(
			access.Permissions,
			accesscontrol.PermissionOperationsEdit,
		)
	}
	switch access.Role {
	case "consultant", "store_terminal", "manager", "owner", "platform_admin":
		return true
	default:
		return false
	}
}

func storeAllowed(access AccessContext, storeID string) bool {
	switch access.Role {
	case "owner", "marketing", "director", "platform_admin":
		return true
	}
	for _, allowedStoreID := range access.StoreIDs {
		if strings.TrimSpace(allowedStoreID) == storeID {
			return true
		}
	}
	return false
}

func normalizeCreateInput(input CreateRecordingInput) CreateRecordingInput {
	input.StoreID = strings.TrimSpace(input.StoreID)
	input.ServiceID = strings.TrimSpace(input.ServiceID)
	input.ClientSessionID = strings.TrimSpace(input.ClientSessionID)
	input.MimeType = normalizeAudioMime(input.MimeType)
	return input
}

func (service *Service) ensureFeature(ctx context.Context) error {
	enabled, err := service.repository.ExperimentalFeatureEnabled(ctx)
	if err != nil {
		return err
	}
	if !enabled {
		return ErrFeatureDisabled
	}
	return nil
}

func (service *Service) Create(ctx context.Context, access AccessContext, input CreateRecordingInput) (RecordingView, error) {
	input = normalizeCreateInput(input)
	if !canWrite(access) {
		return RecordingView{}, ErrForbidden
	}
	if access.AccountID == "" || input.StoreID == "" || input.ServiceID == "" ||
		input.ClientSessionID == "" || len(input.ClientSessionID) > 180 {
		return RecordingView{}, ErrValidation
	}
	if !storeAllowed(access, input.StoreID) {
		return RecordingView{}, ErrNotFound
	}
	if _, supported := audioExtension(input.MimeType); !supported {
		return RecordingView{}, ErrUnsupported
	}
	if err := service.ensureFeature(ctx); err != nil {
		return RecordingView{}, err
	}

	reference, err := service.repository.ResolveService(
		ctx,
		access.AccountID,
		input.StoreID,
		input.ServiceID,
	)
	if err != nil {
		return RecordingView{}, err
	}
	recording, err := service.repository.CreateRecording(ctx, Recording{
		AccountID:           access.AccountID,
		StoreID:             reference.StoreID,
		ServiceID:           reference.ServiceID,
		ConsultantID:        reference.ConsultantID,
		ConsultantName:      reference.ConsultantName,
		ClientSessionID:     input.ClientSessionID,
		RecordingStatus:     RecordingStatusRecording,
		TranscriptionStatus: TranscriptionStatusPending,
		MimeType:            input.MimeType,
		StartedAt:           reference.StartedAt,
		CreatedBy:           access.UserID,
	})
	if err != nil {
		return RecordingView{}, err
	}
	return recordingView(recording), nil
}

func readChunk(source io.Reader) ([]byte, string, error) {
	content, err := io.ReadAll(io.LimitReader(source, maxChunkBytes+1))
	if err != nil {
		return nil, "", err
	}
	if int64(len(content)) > maxChunkBytes {
		return nil, "", ErrTooLarge
	}
	if len(content) == 0 {
		return nil, "", ErrValidation
	}
	digest := sha256.Sum256(content)
	return content, hex.EncodeToString(digest[:]), nil
}

func (service *Service) SaveChunk(ctx context.Context, access AccessContext, recordingID string, sequence int, mimeType string, source io.Reader) (RecordingView, error) {
	recordingID = strings.TrimSpace(recordingID)
	mimeType = normalizeAudioMime(mimeType)
	if !canWrite(access) {
		return RecordingView{}, ErrForbidden
	}
	if access.AccountID == "" || recordingID == "" || sequence < 0 || sequence > maxChunkSequence {
		return RecordingView{}, ErrValidation
	}

	recording, err := service.repository.GetRecording(ctx, access.AccountID, recordingID)
	if err != nil {
		return RecordingView{}, err
	}
	if !storeAllowed(access, recording.StoreID) {
		return RecordingView{}, ErrNotFound
	}
	if recording.RecordingStatus != RecordingStatusRecording {
		return RecordingView{}, ErrNotReady
	}
	if mimeType != recording.MimeType {
		return RecordingView{}, ErrUnsupported
	}

	content, sha, err := readChunk(source)
	if err != nil {
		return RecordingView{}, err
	}
	existing, err := service.repository.GetChunk(ctx, access.AccountID, recordingID, sequence)
	if err == nil {
		if existing.SHA256 != sha {
			return RecordingView{}, ErrChunkConflict
		}
		return recordingView(recording), nil
	}
	if !errors.Is(err, ErrNotFound) {
		return RecordingView{}, err
	}
	if recording.SizeBytes+int64(len(content)) > maxRecordingBytes {
		return RecordingView{}, ErrTooLarge
	}

	stored, err := service.storage.SaveChunk(
		access.AccountID,
		recordingID,
		sequence,
		mimeType,
		bytes.NewReader(content),
		maxChunkBytes,
	)
	if err != nil {
		return RecordingView{}, err
	}
	updated, err := service.repository.SaveChunk(ctx, access.AccountID, Chunk{
		RecordingID: recordingID,
		Sequence:    sequence,
		StorageKey:  stored.StorageKey,
		MimeType:    stored.MimeType,
		SizeBytes:   stored.SizeBytes,
		SHA256:      stored.SHA256,
	})
	if err != nil {
		_ = service.storage.Remove(stored.StorageKey)
		return RecordingView{}, err
	}
	return recordingView(updated), nil
}

func (service *Service) Complete(ctx context.Context, access AccessContext, recordingID string, input CompleteRecordingInput) (RecordingView, error) {
	recordingID = strings.TrimSpace(recordingID)
	if !canWrite(access) {
		return RecordingView{}, ErrForbidden
	}
	if access.AccountID == "" || recordingID == "" {
		return RecordingView{}, ErrValidation
	}

	recording, err := service.repository.GetRecording(ctx, access.AccountID, recordingID)
	if err != nil {
		return RecordingView{}, err
	}
	if !storeAllowed(access, recording.StoreID) {
		return RecordingView{}, ErrNotFound
	}
	if recording.RecordingStatus == RecordingStatusReady {
		requested, requestErr := service.repository.RequestTranscription(
			ctx,
			access.AccountID,
			recordingID,
		)
		if requestErr != nil {
			return RecordingView{}, requestErr
		}
		return recordingView(requested), nil
	}

	chunks, err := service.repository.ListChunks(ctx, access.AccountID, recordingID)
	if err != nil {
		return RecordingView{}, err
	}
	audio, err := service.storage.Consolidate(
		access.AccountID,
		recordingID,
		recording.MimeType,
		chunks,
		maxRecordingBytes,
	)
	if err != nil {
		return RecordingView{}, err
	}
	endedAt := input.EndedAt
	if endedAt <= 0 {
		endedAt = recording.StartedAt
	}
	_, err = service.repository.CompleteRecording(
		ctx,
		access.AccountID,
		recordingID,
		endedAt,
		audio,
	)
	if err != nil {
		_ = service.storage.Remove(audio.StorageKey)
		return RecordingView{}, err
	}
	requested, err := service.repository.RequestTranscription(
		ctx,
		access.AccountID,
		recordingID,
	)
	if err != nil {
		return RecordingView{}, err
	}
	return recordingView(requested), nil
}

func (service *Service) List(ctx context.Context, access AccessContext, filter ListFilter) (ListResponse, error) {
	if !canRead(access) {
		return ListResponse{}, ErrForbidden
	}
	if access.AccountID == "" {
		return ListResponse{}, ErrValidation
	}
	if err := service.ensureFeature(ctx); err != nil {
		return ListResponse{}, err
	}

	filter.StoreID = strings.TrimSpace(filter.StoreID)
	filter.ConsultantID = strings.TrimSpace(filter.ConsultantID)
	if filter.StoreID != "" && !storeAllowed(access, filter.StoreID) {
		return ListResponse{}, ErrNotFound
	}
	if filter.Limit < 1 || filter.Limit > 100 {
		filter.Limit = 30
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	if filter.StoreID == "" {
		switch access.Role {
		case "consultant", "store_terminal", "manager":
			filter.StoreIDs = append([]string{}, access.StoreIDs...)
		}
	}

	recordings, total, err := service.repository.ListRecordings(
		ctx,
		access.AccountID,
		filter,
	)
	if err != nil {
		return ListResponse{}, err
	}
	items := make([]RecordingView, 0, len(recordings))
	for _, recording := range recordings {
		items = append(items, recordingView(recording))
	}
	return ListResponse{
		Items:  items,
		Total:  total,
		Limit:  filter.Limit,
		Offset: filter.Offset,
	}, nil
}

func (service *Service) RequestTranscription(ctx context.Context, access AccessContext, recordingID string) (RecordingView, error) {
	recordingID = strings.TrimSpace(recordingID)
	if !canWrite(access) {
		return RecordingView{}, ErrForbidden
	}
	if access.AccountID == "" || recordingID == "" {
		return RecordingView{}, ErrValidation
	}
	if err := service.ensureFeature(ctx); err != nil {
		return RecordingView{}, err
	}

	recording, err := service.repository.GetRecording(ctx, access.AccountID, recordingID)
	if err != nil {
		return RecordingView{}, err
	}
	if !storeAllowed(access, recording.StoreID) {
		return RecordingView{}, ErrNotFound
	}
	if recording.RecordingStatus != RecordingStatusReady || recording.AudioStorageKey == "" {
		return RecordingView{}, ErrNotReady
	}
	recording, err = service.repository.RequestTranscription(ctx, access.AccountID, recordingID)
	if err != nil {
		return RecordingView{}, err
	}
	return recordingView(recording), nil
}

func (service *Service) OpenAudio(ctx context.Context, access AccessContext, recordingID string) (OpenedAudio, error) {
	if !canRead(access) {
		return OpenedAudio{}, ErrForbidden
	}
	if access.AccountID == "" || strings.TrimSpace(recordingID) == "" {
		return OpenedAudio{}, ErrValidation
	}
	recording, err := service.repository.GetRecording(
		ctx,
		access.AccountID,
		strings.TrimSpace(recordingID),
	)
	if err != nil {
		return OpenedAudio{}, err
	}
	if !storeAllowed(access, recording.StoreID) {
		return OpenedAudio{}, ErrNotFound
	}
	if recording.RecordingStatus != RecordingStatusReady || recording.AudioStorageKey == "" {
		return OpenedAudio{}, ErrNotReady
	}
	extension, _ := audioExtension(recording.MimeType)
	return service.storage.Open(
		recording.AudioStorageKey,
		recording.MimeType,
		audioDownloadName(recording)+extension,
	)
}
