package feedback

import (
	"context"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
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

	created, err := s.repository.Create(feedback)
	if err != nil {
		return nil, err
	}

	return created.ToView(), nil
}

func (s *Service) List(ctx context.Context, principal auth.Principal, input ListInput) ([]FeedbackView, error) {
	if !canManageFeedback(principal) {
		return nil, ErrForbidden
	}

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

func (s *Service) Update(ctx context.Context, principal auth.Principal, id string, input UpdateInput) (*FeedbackView, error) {
	if !canManageFeedback(principal) {
		return nil, ErrForbidden
	}

	feedback, err := s.repository.GetByID(id)
	if err != nil {
		return nil, err
	}

	if feedback.TenantID != principal.TenantID {
		return nil, ErrForbidden
	}

	if input.Status != nil {
		feedback.Status = *input.Status
	}

	if input.AdminNote != nil {
		feedback.AdminNote = *input.AdminNote
	}

	if err := s.repository.Update(feedback); err != nil {
		return nil, err
	}

	return feedback.ToView(), nil
}

func canManageFeedback(principal auth.Principal) bool {
	return principal.Role == auth.RolePlatformAdmin ||
		principal.Role == auth.RoleOwner ||
		principal.Role == auth.RoleManager
}
