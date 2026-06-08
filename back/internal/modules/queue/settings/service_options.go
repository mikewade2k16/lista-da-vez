package settings

import (
	"context"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

func (service *Service) SaveOptionSection(ctx context.Context, principal auth.Principal, optionGroup string, input OptionSectionInput) (MutationAck, error) {
	tenantID, currentItems, _, err := service.loadWritableOptionGroup(ctx, principal, input.TenantID, optionGroup)
	if err != nil {
		return MutationAck{}, err
	}

	nextItems := normalizeOptions(input.Items, currentItems)

	if !isValidOptionGroup(optionGroup) {
		return MutationAck{}, ErrValidation
	}

	savedAt, err := service.repository.ReplaceOptionGroup(ctx, tenantID, optionGroup, nextItems)
	if err != nil {
		return MutationAck{}, err
	}

	ack := MutationAck{
		OK:       true,
		TenantID: tenantID,
		SavedAt:  savedAt,
	}

	return service.finalizeMutation(ctx, ack, nil)
}

func (service *Service) SaveOptionItem(ctx context.Context, principal auth.Principal, optionGroup string, item OptionItem, requestedTenantID string) (MutationAck, error) {
	tenantID, currentItems, seededDefaults, err := service.loadWritableOptionGroup(ctx, principal, requestedTenantID, optionGroup)
	if err != nil {
		return MutationAck{}, err
	}

	if !isValidOptionGroup(optionGroup) {
		return MutationAck{}, ErrValidation
	}

	normalizedItems := normalizeOptions([]OptionItem{item}, nil)
	if len(normalizedItems) != 1 {
		return MutationAck{}, ErrValidation
	}

	var savedAt time.Time
	if seededDefaults {
		nextItems, _ := upsertOptionGroupItem(currentItems, normalizedItems[0])
		savedAt, err = service.repository.ReplaceOptionGroup(ctx, tenantID, optionGroup, nextItems)
	} else {
		savedAt, err = service.repository.UpsertOption(ctx, tenantID, optionGroup, normalizedItems[0])
	}
	if err != nil {
		return MutationAck{}, err
	}

	ack := MutationAck{
		OK:       true,
		TenantID: tenantID,
		SavedAt:  savedAt,
	}

	return service.finalizeMutation(ctx, ack, nil)
}

func (service *Service) DeleteOptionItem(ctx context.Context, principal auth.Principal, optionGroup string, optionID string, requestedTenantID string) (MutationAck, error) {
	if !isValidOptionGroup(optionGroup) {
		return MutationAck{}, ErrValidation
	}

	tenantID, err := service.resolveWritableTenantID(ctx, principal, requestedTenantID)
	if err != nil {
		return MutationAck{}, err
	}

	normalizedOptionID := strings.TrimSpace(optionID)
	if normalizedOptionID == "" {
		return MutationAck{}, ErrValidation
	}

	savedAt, err := service.repository.DeleteOption(ctx, tenantID, optionGroup, normalizedOptionID)
	if err != nil {
		return MutationAck{}, err
	}

	ack := MutationAck{
		OK:       true,
		TenantID: tenantID,
		SavedAt:  savedAt,
	}

	return service.finalizeMutation(ctx, ack, nil)
}

func (service *Service) loadWritableOptionGroup(ctx context.Context, principal auth.Principal, requestedTenantID string, optionGroup string) (string, []OptionItem, bool, error) {
	if !isValidOptionGroup(optionGroup) {
		return "", nil, false, ErrValidation
	}

	tenantID, err := service.resolveWritableTenantID(ctx, principal, requestedTenantID)
	if err != nil {
		return "", nil, false, err
	}

	items, err := service.repository.GetOptionGroup(ctx, tenantID, optionGroup)
	if err != nil {
		return "", nil, false, err
	}

	if len(items) > 0 {
		return tenantID, cloneOptions(items), false, nil
	}

	selectedTemplateID, err := service.loadSelectedOperationTemplateID(ctx, tenantID)
	if err != nil {
		return "", nil, false, err
	}

	defaultItems, err := defaultOptionGroupItems(selectedTemplateID, optionGroup)
	if err != nil {
		return "", nil, false, err
	}

	return tenantID, defaultItems, true, nil
}

func normalizeOptions(options []OptionItem, fallback []OptionItem) []OptionItem {
	if options == nil {
		return cloneOptions(fallback)
	}

	normalized := make([]OptionItem, 0, len(options))
	seen := make(map[string]struct{})
	for _, option := range options {
		id := strings.TrimSpace(option.ID)
		label := strings.TrimSpace(option.Label)
		if id == "" || label == "" {
			continue
		}

		if _, exists := seen[id]; exists {
			continue
		}

		seen[id] = struct{}{}
		normalized = append(normalized, OptionItem{
			ID:    id,
			Label: label,
		})
	}

	return normalized
}

func getOptionGroupItems(bundle Bundle, optionGroup string) ([]OptionItem, error) {
	switch optionGroup {
	case optionKindVisitReason:
		return cloneOptions(bundle.VisitReasonOptions), nil
	case optionKindCustomerSource:
		return cloneOptions(bundle.CustomerSourceOptions), nil
	case optionKindPauseReason:
		return cloneOptions(bundle.PauseReasonOptions), nil
	case optionKindCancelReason:
		return cloneOptions(bundle.CancelReasonOptions), nil
	case optionKindStopReason:
		return cloneOptions(bundle.StopReasonOptions), nil
	case optionKindQueueJump:
		return cloneOptions(bundle.QueueJumpReasonOptions), nil
	case optionKindLossReason:
		return cloneOptions(bundle.LossReasonOptions), nil
	case optionKindProfession:
		return cloneOptions(bundle.ProfessionOptions), nil
	default:
		return nil, ErrValidation
	}
}

func upsertOptionGroupItem(items []OptionItem, item OptionItem) ([]OptionItem, bool) {
	normalizedItems := normalizeOptions([]OptionItem{item}, nil)
	if len(normalizedItems) != 1 {
		return nil, false
	}

	nextItems := cloneOptions(items)
	nextItem := normalizedItems[0]

	for index, current := range nextItems {
		if current.ID == nextItem.ID {
			nextItems[index] = nextItem
			return nextItems, true
		}
	}

	return append(nextItems, nextItem), true
}

func removeOptionGroupItem(items []OptionItem, optionID string) []OptionItem {
	nextItems := make([]OptionItem, 0, len(items))
	for _, item := range items {
		if item.ID != optionID {
			nextItems = append(nextItems, item)
		}
	}

	return nextItems
}
