package feedback

import (
	"context"
	"strings"
	"time"

	accesscontrol "github.com/mikewade2k16/lista-da-vez/back/internal/modules/access"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

type Service struct {
	repository Repository
	imageStorage ImageStorage
}

func NewService(repository Repository, imageStorage ImageStorage) *Service {
	return &Service{repository: repository, imageStorage: imageStorage}
}

func (s *Service) Create(ctx context.Context, principal auth.Principal, input CreateInput) (*FeedbackView, error) {
	storeID := ""
	if len(principal.StoreIDs) > 0 {
		storeID = principal.StoreIDs[0]
	}

	feedback := &Feedback{
		TenantID:  principal.TenantID,
		StoreID:   storeID,
		UserID:    principal.UserID,
		UserName:  strings.TrimSpace(principal.DisplayName),
		Kind:      input.Kind,
		Status:    StatusOpen,
		Subject:   strings.TrimSpace(input.Subject),
		Body:      strings.TrimSpace(input.Body),
		AdminNote: "",
	}

	if input.Image != nil {
		storedImage, err := s.saveImage(ctx, principal.UserID, input.Image)
		if err != nil {
			return nil, err
		}
		if storedImage != nil {
			feedback.ImagePath = storedImage.Path
			feedback.ImageContentType = storedImage.ContentType
			feedback.ImageSizeBytes = storedImage.SizeBytes
		}
	}

	created, err := s.repository.Create(feedback)
	if err != nil {
		if feedback.ImagePath != "" {
			_ = s.deleteImage(feedback.ImagePath)
		}
		return nil, err
	}

	return created.ToView(), nil
}

func (s *Service) List(ctx context.Context, principal auth.Principal, input ListInput) ([]FeedbackView, error) {
	if !canViewFeedback(principal) {
		return nil, ErrForbidden
	}
	input.ViewerUserID = principal.UserID

	feedbacks, err := s.repository.List(principal.TenantID, input)
	if err != nil {
		return nil, err
	}

	views := make([]FeedbackView, 0, len(feedbacks))
	for _, f := range feedbacks {
		views = append(views, *f.ToView())
	}

	return views, nil
}

func (s *Service) ListMine(ctx context.Context, principal auth.Principal, input ListInput) ([]FeedbackView, error) {
	input.UserID = principal.UserID
	input.ViewerUserID = principal.UserID

	feedbacks, err := s.repository.List(principal.TenantID, input)
	if err != nil {
		return nil, err
	}

	views := make([]FeedbackView, 0, len(feedbacks))
	for _, f := range feedbacks {
		views = append(views, *f.ToView())
	}

	return views, nil
}

func (s *Service) MarkRead(ctx context.Context, principal auth.Principal, id string) (*FeedbackView, error) {
	feedback, err := s.repository.GetByID(id)
	if err != nil {
		return nil, err
	}

	if !canAccessFeedback(principal, feedback) {
		return nil, ErrForbidden
	}

	updated, err := s.repository.MarkRead(feedback.ID, principal.UserID, time.Now().UTC())
	if err != nil {
		return nil, err
	}

	return updated.ToView(), nil
}

func (s *Service) ListMessages(ctx context.Context, principal auth.Principal, id string, input ListMessagesInput) ([]FeedbackMessageView, error) {
	feedback, err := s.repository.GetByID(id)
	if err != nil {
		return nil, err
	}

	if !canAccessFeedback(principal, feedback) {
		return nil, ErrForbidden
	}

	messages, err := s.repository.ListMessages(id, input)
	if err != nil {
		return nil, err
	}

	views := make([]FeedbackMessageView, 0, len(messages))
	for _, message := range messages {
		views = append(views, *message.ToView())
	}

	return views, nil
}

func (s *Service) CreateMessage(ctx context.Context, principal auth.Principal, id string, input CreateMessageInput) (*FeedbackMessageView, error) {
	feedback, err := s.repository.GetByID(id)
	if err != nil {
		return nil, err
	}

	if feedback.Status == StatusClosed {
		return nil, ErrClosed
	}

	if !canReplyToFeedback(principal, feedback) {
		return nil, ErrForbidden
	}

	body := strings.TrimSpace(input.Body)
	if body == "" && input.Image == nil {
		return nil, ErrInvalid
	}

	message := &FeedbackMessage{
		TenantID:     feedback.TenantID,
		FeedbackID:   feedback.ID,
		AuthorUserID: principal.UserID,
		AuthorName:   strings.TrimSpace(principal.DisplayName),
		AuthorRole:   string(principal.Role),
		Body:         body,
	}

	if input.Image != nil {
		storedImage, err := s.saveImage(ctx, feedback.ID, input.Image)
		if err != nil {
			return nil, err
		}
		if storedImage != nil {
			message.ImagePath = storedImage.Path
			message.ImageContentType = storedImage.ContentType
			message.ImageSizeBytes = storedImage.SizeBytes
		}
	}

	created, err := s.repository.CreateMessage(message)
	if err != nil {
		if message.ImagePath != "" {
			_ = s.deleteImage(message.ImagePath)
		}
		return nil, err
	}
	if _, err := s.repository.MarkRead(feedback.ID, principal.UserID, created.CreatedAt); err != nil {
		return nil, err
	}

	return created.ToView(), nil
}

func (s *Service) Update(ctx context.Context, principal auth.Principal, id string, input UpdateInput) (*FeedbackView, error) {
	if !canEditFeedback(principal) {
		return nil, ErrForbidden
	}

	feedback, err := s.repository.GetByID(id)
	if err != nil {
		return nil, err
	}

	if feedback.TenantID != principal.TenantID && principal.Role != auth.RolePlatformAdmin {
		return nil, ErrForbidden
	}

	if input.Status != nil {
		nextStatus := strings.TrimSpace(*input.Status)
		wasClosed := feedback.Status == StatusClosed
		feedback.Status = nextStatus
		if nextStatus == StatusClosed {
			if !wasClosed || feedback.ClosedAt == nil {
				closedAt := time.Now().UTC()
				feedback.ClosedAt = &closedAt
			}
		} else if wasClosed {
			feedback.ClosedAt = nil
		}
	}

	if input.AdminNote != nil {
		feedback.AdminNote = *input.AdminNote
	}

	if err := s.repository.Update(feedback); err != nil {
		return nil, err
	}

	return feedback.ToView(), nil
}

func (s *Service) CleanupExpiredAttachments(ctx context.Context) (int, error) {
	totalDeleted := 0
	for {
		paths, err := s.repository.PurgeExpiredAttachments(time.Now().UTC(), feedbackAttachmentCleanupBatch)
		if err != nil {
			return totalDeleted, err
		}
		if len(paths) == 0 {
			return totalDeleted, nil
		}

		totalDeleted += len(paths)
		for _, path := range paths {
			if err := s.deleteImage(path); err != nil {
				return totalDeleted, err
			}
		}

		if len(paths) < feedbackAttachmentCleanupBatch {
			return totalDeleted, nil
		}
	}
}

func (s *Service) saveImage(ctx context.Context, ownerID string, upload *ImageUpload) (*StoredImage, error) {
	if upload == nil || len(upload.Content) == 0 {
		return nil, nil
	}
	if s.imageStorage == nil {
		return nil, ErrInvalidImage
	}
	return s.imageStorage.Save(ctx, ownerID, upload.FileName, upload.ContentType, upload.Content)
}

func (s *Service) deleteImage(path string) error {
	if strings.TrimSpace(path) == "" || s.imageStorage == nil {
		return nil
	}
	return s.imageStorage.Delete(path)
}

func canViewFeedback(principal auth.Principal) bool {
	if principal.PermissionsResolved {
		return accesscontrol.HasPermission(principal.Permissions, accesscontrol.PermissionFeedbackView) ||
			accesscontrol.HasPermission(principal.Permissions, accesscontrol.PermissionFeedbackEdit)
	}

	return principal.Role == auth.RolePlatformAdmin ||
		principal.Role == auth.RoleOwner ||
		principal.Role == auth.RoleManager
}

func canEditFeedback(principal auth.Principal) bool {
	if principal.PermissionsResolved {
		return accesscontrol.HasPermission(principal.Permissions, accesscontrol.PermissionFeedbackEdit)
	}

	return principal.Role == auth.RolePlatformAdmin ||
		principal.Role == auth.RoleOwner ||
		principal.Role == auth.RoleManager
}

func canAccessFeedback(principal auth.Principal, feedback *Feedback) bool {
	if feedback == nil {
		return false
	}

	if feedback.TenantID != principal.TenantID && principal.Role != auth.RolePlatformAdmin {
		return false
	}

	return canViewFeedback(principal) || feedback.UserID == principal.UserID
}

func canReplyToFeedback(principal auth.Principal, feedback *Feedback) bool {
	if feedback == nil {
		return false
	}

	if feedback.TenantID != principal.TenantID && principal.Role != auth.RolePlatformAdmin {
		return false
	}

	return canEditFeedback(principal) || feedback.UserID == principal.UserID
}
