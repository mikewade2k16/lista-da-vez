package communications

import (
	"context"
	"strings"

	accesscontrol "github.com/mikewade2k16/lista-da-vez/back/internal/modules/access"
)

const (
	maxTitleLength   = 160
	maxExcerptLength = 300
	maxBodyLength    = 20_000
)

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
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

func canManage(access AccessContext) bool {
	if access.Role == "platform_admin" {
		return true
	}
	if access.PermissionsResolved {
		return accesscontrol.HasPermission(
			access.Permissions,
			accesscontrol.PermissionQueueCommunicationsManage,
		)
	}
	switch access.Role {
	case "manager", "marketing", "owner", "platform_admin":
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

func normalizeInput(input UpsertInput) UpsertInput {
	input.Title = strings.TrimSpace(input.Title)
	input.Excerpt = strings.TrimSpace(input.Excerpt)
	input.Body = strings.TrimSpace(input.Body)
	seen := make(map[string]struct{}, len(input.StoreIDs))
	storeIDs := make([]string, 0, len(input.StoreIDs))
	for _, candidate := range input.StoreIDs {
		storeID := strings.TrimSpace(candidate)
		if storeID == "" {
			continue
		}
		if _, exists := seen[storeID]; exists {
			continue
		}
		seen[storeID] = struct{}{}
		storeIDs = append(storeIDs, storeID)
	}
	if input.TargetsAllStores {
		storeIDs = []string{}
	}
	input.StoreIDs = storeIDs
	return input
}

func validateInput(input UpsertInput) error {
	if input.Title == "" || len([]rune(input.Title)) > maxTitleLength ||
		input.Body == "" || len([]rune(input.Body)) > maxBodyLength ||
		len([]rune(input.Excerpt)) > maxExcerptLength ||
		input.DisplayOrder < -10_000 || input.DisplayOrder > 10_000 {
		return ErrValidation
	}
	if !input.TargetsAllStores && len(input.StoreIDs) == 0 {
		return ErrValidation
	}
	if input.StartsAt != nil && input.EndsAt != nil && !input.EndsAt.After(*input.StartsAt) {
		return ErrValidation
	}
	return nil
}

func (service *Service) validateTargets(
	ctx context.Context,
	access AccessContext,
	input UpsertInput,
) error {
	if input.TargetsAllStores {
		return nil
	}
	for _, storeID := range input.StoreIDs {
		if !storeAllowed(access, storeID) {
			return ErrForbidden
		}
	}
	valid, err := service.repository.StoresBelongToAccount(
		ctx,
		access.AccountID,
		input.StoreIDs,
	)
	if err != nil {
		return err
	}
	if !valid {
		return ErrValidation
	}
	return nil
}

func (service *Service) List(
	ctx context.Context,
	access AccessContext,
	filter ListFilter,
) (ListResponse, error) {
	filter.StoreID = strings.TrimSpace(filter.StoreID)
	if !canRead(access) {
		return ListResponse{}, ErrForbidden
	}
	if access.AccountID == "" {
		return ListResponse{}, ErrValidation
	}
	if filter.StoreID != "" && !storeAllowed(access, filter.StoreID) {
		return ListResponse{}, ErrForbidden
	}
	items, err := service.repository.List(ctx, access.AccountID, filter)
	if err != nil {
		return ListResponse{}, err
	}
	response := ListResponse{Items: make([]CommunicationView, 0, len(items))}
	for _, item := range items {
		response.Items = append(response.Items, communicationView(item))
	}
	return response, nil
}

func (service *Service) Create(
	ctx context.Context,
	access AccessContext,
	input UpsertInput,
) (CommunicationView, error) {
	input = normalizeInput(input)
	if !canManage(access) {
		return CommunicationView{}, ErrForbidden
	}
	if access.AccountID == "" || access.UserID == "" {
		return CommunicationView{}, ErrValidation
	}
	if err := validateInput(input); err != nil {
		return CommunicationView{}, err
	}
	if err := service.validateTargets(ctx, access, input); err != nil {
		return CommunicationView{}, err
	}
	created, err := service.repository.Create(ctx, Communication{
		AccountID:        access.AccountID,
		Title:            input.Title,
		Excerpt:          input.Excerpt,
		Body:             input.Body,
		StartsAt:         input.StartsAt,
		EndsAt:           input.EndsAt,
		IsPublished:      input.IsPublished,
		DisplayOrder:     input.DisplayOrder,
		TargetsAllStores: input.TargetsAllStores,
		StoreIDs:         input.StoreIDs,
		CreatedBy:        access.UserID,
		UpdatedBy:        access.UserID,
	})
	if err != nil {
		return CommunicationView{}, err
	}
	return communicationView(created), nil
}

func (service *Service) Update(
	ctx context.Context,
	access AccessContext,
	communicationID string,
	input UpsertInput,
) (CommunicationView, error) {
	communicationID = strings.TrimSpace(communicationID)
	input = normalizeInput(input)
	if !canManage(access) {
		return CommunicationView{}, ErrForbidden
	}
	if access.AccountID == "" || access.UserID == "" || communicationID == "" {
		return CommunicationView{}, ErrValidation
	}
	if err := validateInput(input); err != nil {
		return CommunicationView{}, err
	}
	if err := service.validateTargets(ctx, access, input); err != nil {
		return CommunicationView{}, err
	}
	if _, err := service.repository.Get(ctx, access.AccountID, communicationID); err != nil {
		return CommunicationView{}, err
	}
	updated, err := service.repository.Update(ctx, Communication{
		ID:               communicationID,
		AccountID:        access.AccountID,
		Title:            input.Title,
		Excerpt:          input.Excerpt,
		Body:             input.Body,
		StartsAt:         input.StartsAt,
		EndsAt:           input.EndsAt,
		IsPublished:      input.IsPublished,
		DisplayOrder:     input.DisplayOrder,
		TargetsAllStores: input.TargetsAllStores,
		StoreIDs:         input.StoreIDs,
		UpdatedBy:        access.UserID,
	})
	if err != nil {
		return CommunicationView{}, err
	}
	return communicationView(updated), nil
}

func (service *Service) Delete(
	ctx context.Context,
	access AccessContext,
	communicationID string,
) error {
	communicationID = strings.TrimSpace(communicationID)
	if !canManage(access) {
		return ErrForbidden
	}
	if access.AccountID == "" || access.UserID == "" || communicationID == "" {
		return ErrValidation
	}
	return service.repository.Archive(
		ctx,
		access.AccountID,
		communicationID,
		access.UserID,
	)
}
