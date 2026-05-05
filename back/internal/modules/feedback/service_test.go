package feedback

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

type serviceTestRepository struct {
	feedback        *Feedback
	feedbacks       []Feedback
	messages        []FeedbackMessage
	createdMessage  *FeedbackMessage
	markReadResult  *Feedback
	purgedPaths     []string
	updateCallCount int
	updatedFeedback *Feedback

	getByIDErr        error
	listErr           error
	updateErr         error
	createMessageErr  error
	listMessagesErr   error
	markReadErr       error
	markReadCallCount int
	lastMarkRead      struct {
		feedbackID string
		userID     string
		readAt     time.Time
	}
}

type serviceTestImageStorage struct {
	savedImage   *StoredImage
	saveErr      error
	deletedPaths []string
	deleteErr    error
}

func (storage *serviceTestImageStorage) Save(_ context.Context, _ string, _ string, _ string, _ []byte) (*StoredImage, error) {
	if storage.saveErr != nil {
		return nil, storage.saveErr
	}
	if storage.savedImage == nil {
		return &StoredImage{Path: "/uploads/feedback/test.webp", ContentType: "image/webp", SizeBytes: 12345}, nil
	}
	return storage.savedImage, nil
}

func (storage *serviceTestImageStorage) Delete(path string) error {
	storage.deletedPaths = append(storage.deletedPaths, path)
	return storage.deleteErr
}

func (repository *serviceTestRepository) Create(feedback *Feedback) (*Feedback, error) {
	return feedback, nil
}

func (repository *serviceTestRepository) GetByID(_ string) (*Feedback, error) {
	if repository.getByIDErr != nil {
		return nil, repository.getByIDErr
	}
	if repository.feedback == nil {
		return nil, ErrNotFound
	}
	return repository.feedback, nil
}

func (repository *serviceTestRepository) List(_ string, _ ListInput) ([]Feedback, error) {
	return repository.feedbacks, repository.listErr
}

func (repository *serviceTestRepository) MarkRead(feedbackID string, userID string, readAt time.Time) (*Feedback, error) {
	repository.markReadCallCount++
	repository.lastMarkRead.feedbackID = feedbackID
	repository.lastMarkRead.userID = userID
	repository.lastMarkRead.readAt = readAt
	if repository.markReadErr != nil {
		return nil, repository.markReadErr
	}
	if repository.markReadResult != nil {
		return repository.markReadResult, nil
	}
	if repository.feedback == nil {
		return nil, ErrNotFound
	}
	updated := *repository.feedback
	updated.UserLastReadAt = readAt
	return &updated, nil
}

func (repository *serviceTestRepository) Update(_ *Feedback) error {
	repository.updateCallCount++
	if repository.feedback != nil {
		updated := *repository.feedback
		repository.updatedFeedback = &updated
	}
	return repository.updateErr
}

func (repository *serviceTestRepository) CreateMessage(_ *FeedbackMessage) (*FeedbackMessage, error) {
	if repository.createMessageErr != nil {
		return nil, repository.createMessageErr
	}
	if repository.createdMessage == nil {
		return nil, ErrInvalid
	}
	return repository.createdMessage, nil
}

func (repository *serviceTestRepository) ListMessages(_ string, _ ListMessagesInput) ([]FeedbackMessage, error) {
	return repository.messages, repository.listMessagesErr
}

func (repository *serviceTestRepository) PurgeExpiredAttachments(_ time.Time, _ int) ([]string, error) {
	if len(repository.purgedPaths) == 0 {
		return nil, nil
	}
	paths := append([]string(nil), repository.purgedPaths...)
	repository.purgedPaths = nil
	return paths, nil
}

func TestMarkReadRejectsNonOwner(t *testing.T) {
	repository := &serviceTestRepository{
		feedback: &Feedback{ID: "feedback-1", TenantID: "tenant-1", UserID: "owner-1"},
	}
	service := NewService(repository, nil)

	_, err := service.MarkRead(context.Background(), auth.Principal{
		UserID:   "other-user",
		TenantID: "tenant-1",
		Role:     auth.RoleConsultant,
	}, "feedback-1")
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
	if repository.markReadCallCount != 0 {
		t.Fatalf("expected mark read not to be called, got %d calls", repository.markReadCallCount)
	}
}

func TestMarkReadUpdatesCursorForOwner(t *testing.T) {
	feedback := &Feedback{ID: "feedback-1", TenantID: "tenant-1", UserID: "owner-1"}
	readAt := time.Date(2026, time.May, 5, 18, 30, 0, 0, time.UTC)
	repository := &serviceTestRepository{
		feedback:       feedback,
		markReadResult: &Feedback{ID: feedback.ID, TenantID: feedback.TenantID, UserID: feedback.UserID, UserLastReadAt: readAt},
	}
	service := NewService(repository, nil)

	result, err := service.MarkRead(context.Background(), auth.Principal{
		UserID:   "owner-1",
		TenantID: "tenant-1",
		Role:     auth.RoleConsultant,
	}, "feedback-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if repository.markReadCallCount != 1 {
		t.Fatalf("expected mark read to be called once, got %d", repository.markReadCallCount)
	}
	if repository.lastMarkRead.feedbackID != "feedback-1" {
		t.Fatalf("expected feedback id feedback-1, got %q", repository.lastMarkRead.feedbackID)
	}
	if repository.lastMarkRead.userID != "owner-1" {
		t.Fatalf("expected user id owner-1, got %q", repository.lastMarkRead.userID)
	}
	if repository.lastMarkRead.readAt.IsZero() {
		t.Fatalf("expected non-zero read timestamp")
	}
	if !result.UserLastReadAt.Equal(readAt) {
		t.Fatalf("expected returned read timestamp %s, got %s", readAt, result.UserLastReadAt)
	}
}

func TestCreateMessageMarksReadForFeedbackOwner(t *testing.T) {
	createdAt := time.Date(2026, time.May, 5, 19, 15, 0, 0, time.UTC)
	repository := &serviceTestRepository{
		feedback: &Feedback{ID: "feedback-1", TenantID: "tenant-1", UserID: "owner-1"},
		createdMessage: &FeedbackMessage{
			ID:           "message-1",
			FeedbackID:   "feedback-1",
			AuthorUserID: "owner-1",
			AuthorName:   "Owner",
			AuthorRole:   string(auth.RoleConsultant),
			Body:         "Resposta",
			CreatedAt:    createdAt,
		},
	}
	service := NewService(repository, nil)

	message, err := service.CreateMessage(context.Background(), auth.Principal{
		UserID:      "owner-1",
		DisplayName: "Owner",
		TenantID:    "tenant-1",
		Role:        auth.RoleConsultant,
	}, "feedback-1", CreateMessageInput{Body: "Resposta"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if message == nil || message.ID != "message-1" {
		t.Fatalf("expected created message to be returned")
	}
	if repository.markReadCallCount != 1 {
		t.Fatalf("expected mark read to be called once, got %d", repository.markReadCallCount)
	}
	if !repository.lastMarkRead.readAt.Equal(createdAt) {
		t.Fatalf("expected read cursor to advance to message timestamp %s, got %s", createdAt, repository.lastMarkRead.readAt)
	}
}

func TestCreateMessageRejectsClosedFeedback(t *testing.T) {
	repository := &serviceTestRepository{
		feedback: &Feedback{ID: "feedback-1", TenantID: "tenant-1", UserID: "owner-1", Status: StatusClosed},
	}
	service := NewService(repository, nil)

	_, err := service.CreateMessage(context.Background(), auth.Principal{
		UserID:      "owner-1",
		DisplayName: "Owner",
		TenantID:    "tenant-1",
		Role:        auth.RoleConsultant,
	}, "feedback-1", CreateMessageInput{Body: "Nao deveria enviar"})
	if !errors.Is(err, ErrClosed) {
		t.Fatalf("expected ErrClosed, got %v", err)
	}
	if repository.markReadCallCount != 0 {
		t.Fatalf("expected mark read not to be called, got %d calls", repository.markReadCallCount)
	}
}

func TestListAllowsResolvedFeedbackViewPermission(t *testing.T) {
	repository := &serviceTestRepository{
		feedbacks: []Feedback{{ID: "feedback-1", TenantID: "tenant-1", UserID: "owner-1", Subject: "Assunto"}},
	}
	service := NewService(repository, nil)

	result, err := service.List(context.Background(), auth.Principal{
		UserID:              "viewer-1",
		TenantID:            "tenant-1",
		Role:                auth.RoleConsultant,
		PermissionsResolved: true,
		Permissions:         []string{"workspace.feedback.view"},
	}, ListInput{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected one feedback, got %d", len(result))
	}
}

func TestListMessagesAllowsResolvedFeedbackViewPermission(t *testing.T) {
	repository := &serviceTestRepository{
		feedback: &Feedback{ID: "feedback-1", TenantID: "tenant-1", UserID: "owner-1"},
		messages: []FeedbackMessage{{ID: "message-1", FeedbackID: "feedback-1", Body: "Resposta"}},
	}
	service := NewService(repository, nil)

	result, err := service.ListMessages(context.Background(), auth.Principal{
		UserID:              "viewer-1",
		TenantID:            "tenant-1",
		Role:                auth.RoleConsultant,
		PermissionsResolved: true,
		Permissions:         []string{"workspace.feedback.view"},
	}, "feedback-1", ListMessagesInput{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected one message, got %d", len(result))
	}
}

func TestUpdateAllowsResolvedFeedbackEditPermission(t *testing.T) {
	feedback := &Feedback{ID: "feedback-1", TenantID: "tenant-1", UserID: "owner-1", Status: StatusOpen}
	repository := &serviceTestRepository{
		feedback: feedback,
	}
	service := NewService(repository, nil)
	status := StatusResolved

	result, err := service.Update(context.Background(), auth.Principal{
		UserID:              "editor-1",
		TenantID:            "tenant-1",
		Role:                auth.RoleConsultant,
		PermissionsResolved: true,
		Permissions:         []string{"workspace.feedback.edit"},
	}, "feedback-1", UpdateInput{Status: &status})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if repository.updateCallCount != 1 {
		t.Fatalf("expected update to be called once, got %d", repository.updateCallCount)
	}
	if repository.updatedFeedback == nil || repository.updatedFeedback.Status != StatusResolved {
		t.Fatalf("expected updated feedback status %q, got %#v", StatusResolved, repository.updatedFeedback)
	}
	if result.Status != StatusResolved {
		t.Fatalf("expected returned status %q, got %q", StatusResolved, result.Status)
	}
}
