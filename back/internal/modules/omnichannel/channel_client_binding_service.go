package omnichannel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

const (
	defaultChannelBindingLimit = 50
	maxChannelBindingLimit     = 100
)

type ChannelClientBindingService struct {
	repo        channelClientBindingRepository
	permissions automationPermissionChecker
	clients     AutomationClientCatalog
	now         func() time.Time
}

func NewChannelClientBindingService(
	repo channelClientBindingRepository,
	permissions automationPermissionChecker,
	clients AutomationClientCatalog,
) *ChannelClientBindingService {
	return &ChannelClientBindingService{
		repo: repo, permissions: permissions, clients: clients, now: time.Now,
	}
}

func (s *ChannelClientBindingService) requireManage(ctx context.Context, accountID string, p auth.Principal) error {
	if s.permissions == nil {
		return ErrForbidden
	}
	return s.permissions.requirePermission(ctx, accountID, p, channelBindingManagePermission)
}

func (s *ChannelClientBindingService) requireResourceManage(ctx context.Context, accountID string, p auth.Principal, channel, resourceID string) error {
	if !strings.EqualFold(strings.TrimSpace(channel), "WHATSAPP") {
		return nil
	}
	return s.permissions.requireInstanceAccess(ctx, accountID, p.UserID, resourceID,
		channelBindingManagePermission, InstanceGrantManage)
}

func (s *ChannelClientBindingService) accessibleClient(
	ctx context.Context,
	accountID string,
	p auth.Principal,
	clientAccountID string,
) (AutomationClientRef, error) {
	clientAccountID = strings.TrimSpace(clientAccountID)
	if !omnichannelUUIDPattern.MatchString(clientAccountID) || s.clients == nil {
		return AutomationClientRef{}, ErrNotFound
	}
	clients, err := s.clients.ListAccessible(ctx, p)
	if err != nil {
		return AutomationClientRef{}, err
	}
	var selected AutomationClientRef
	for _, client := range clients {
		if strings.EqualFold(client.ID, clientAccountID) {
			selected = client
			break
		}
	}
	if selected.ID == "" {
		return AutomationClientRef{}, ErrNotFound
	}
	eligible, err := s.repo.ChannelBindingClientEligible(ctx, accountID, selected.ID)
	if err != nil {
		return AutomationClientRef{}, err
	}
	if !eligible {
		return AutomationClientRef{}, ErrNotFound
	}
	return selected, nil
}

func (s *ChannelClientBindingService) List(
	ctx context.Context,
	accountID string,
	p auth.Principal,
	filter ChannelClientBindingFilter,
) (ChannelClientBindingPage, error) {
	if err := s.requireManage(ctx, accountID, p); err != nil {
		return ChannelClientBindingPage{}, err
	}
	filter.Channel = strings.ToUpper(strings.TrimSpace(filter.Channel))
	filter.State = strings.ToLower(strings.TrimSpace(filter.State))
	filter.Cursor = strings.TrimSpace(filter.Cursor)
	if filter.Channel != "" && filter.Channel != "WHATSAPP" && filter.Channel != "INSTAGRAM" {
		return ChannelClientBindingPage{}, ErrValidation
	}
	if filter.State != "" && filter.State != "active" && filter.State != "ended" {
		return ChannelClientBindingPage{}, ErrValidation
	}
	if filter.Cursor != "" && !omnichannelUUIDPattern.MatchString(filter.Cursor) {
		return ChannelClientBindingPage{}, ErrValidation
	}
	if strings.TrimSpace(filter.ClientAccountID) != "" {
		if _, err := s.accessibleClient(ctx, accountID, p, filter.ClientAccountID); err != nil {
			return ChannelClientBindingPage{}, err
		}
	}
	if filter.Limit <= 0 {
		filter.Limit = defaultChannelBindingLimit
	}
	if filter.Limit > maxChannelBindingLimit {
		filter.Limit = maxChannelBindingLimit
	}
	requestedLimit := filter.Limit
	filter.Limit++
	rows, err := s.repo.ListChannelClientBindings(ctx, accountID, filter)
	if err != nil {
		return ChannelClientBindingPage{}, err
	}

	accessible, err := s.accessibleClientIDs(ctx, accountID, p)
	if err != nil {
		return ChannelClientBindingPage{}, err
	}
	filtered := make([]ChannelClientBindingView, 0, len(rows))
	for _, row := range rows {
		if _, ok := accessible[row.ClientAccountID]; !ok {
			continue
		}
		if err := s.requireResourceManage(ctx, accountID, p, row.Channel, row.ChannelResource.ID); err != nil {
			if errors.Is(err, ErrNotFound) {
				continue
			}
			return ChannelClientBindingPage{}, err
		}
		filtered = append(filtered, row)
	}
	hasMore := len(filtered) > requestedLimit
	if hasMore {
		filtered = filtered[:requestedLimit]
	}
	nextCursor := ""
	if hasMore && len(filtered) > 0 {
		nextCursor = filtered[len(filtered)-1].ID
	}
	return ChannelClientBindingPage{
		Items: filtered, HasMore: hasMore, NextCursor: nextCursor,
	}, nil
}

func (s *ChannelClientBindingService) accessibleClientIDs(
	ctx context.Context,
	accountID string,
	p auth.Principal,
) (map[string]struct{}, error) {
	if s.clients == nil {
		return map[string]struct{}{}, nil
	}
	clients, err := s.clients.ListAccessible(ctx, p)
	if err != nil {
		return nil, err
	}
	out := make(map[string]struct{}, len(clients))
	for _, client := range clients {
		eligible, eligibleErr := s.repo.ChannelBindingClientEligible(ctx, accountID, client.ID)
		if eligibleErr != nil {
			return nil, eligibleErr
		}
		if eligible {
			out[client.ID] = struct{}{}
		}
	}
	return out, nil
}

func (s *ChannelClientBindingService) Create(
	ctx context.Context,
	accountID string,
	p auth.Principal,
	in CreateChannelClientBindingInput,
) (ChannelClientBindingView, error) {
	if err := s.requireManage(ctx, accountID, p); err != nil {
		return ChannelClientBindingView{}, err
	}
	client, err := s.accessibleClient(ctx, accountID, p, in.ClientAccountID)
	if err != nil {
		return ChannelClientBindingView{}, err
	}
	channel, resourceID, reason, key, err := normalizeChannelBindingWrite(
		in.Channel, in.ChannelResourceID, in.Reason, in.IdempotencyKey,
	)
	if err != nil {
		return ChannelClientBindingView{}, err
	}
	if err := s.requireResourceManage(ctx, accountID, p, channel, resourceID); err != nil {
		return ChannelClientBindingView{}, err
	}
	exists, active, err := s.repo.ChannelBindingResourceExists(ctx, accountID, channel, resourceID)
	if err != nil {
		return ChannelClientBindingView{}, err
	}
	if !exists {
		return ChannelClientBindingView{}, ErrNotFound
	}
	if !active {
		return ChannelClientBindingView{}, ErrConflict
	}
	effectiveFrom := s.now().UTC()
	if in.EffectiveFrom != nil {
		effectiveFrom = in.EffectiveFrom.UTC()
	}
	if effectiveFrom.After(s.now().UTC().Add(5 * time.Minute)) {
		return ChannelClientBindingView{}, ErrValidation
	}
	requestHash := channelBindingRequestHash("create", map[string]any{
		"clientAccountId": client.ID,
		"channel":         channel,
		"resourceId":      resourceID,
		"effectiveFrom":   effectiveFrom.Format(time.RFC3339Nano),
		"reason":          reason,
	})
	bindingID, err := s.repo.CreateChannelClientBinding(ctx, accountID, channelClientBindingWrite{
		ClientAccountID: client.ID,
		Channel:         channel,
		ResourceID:      resourceID,
		EffectiveFrom:   effectiveFrom,
		Reason:          reason,
		IdempotencyKey:  key,
		RequestHash:     requestHash,
		ActorUserID:     p.UserID,
		Source:          "manual",
	})
	if err != nil {
		return ChannelClientBindingView{}, err
	}
	return s.repo.GetChannelClientBinding(ctx, accountID, bindingID)
}

func (s *ChannelClientBindingService) Reassign(
	ctx context.Context,
	accountID, bindingID string,
	p auth.Principal,
	in ReassignChannelClientBindingInput,
) (ChannelClientBindingView, error) {
	if err := s.requireManage(ctx, accountID, p); err != nil {
		return ChannelClientBindingView{}, err
	}
	if !omnichannelUUIDPattern.MatchString(strings.TrimSpace(bindingID)) || in.ExpectedRevision < 1 {
		return ChannelClientBindingView{}, ErrValidation
	}
	current, err := s.repo.GetChannelClientBinding(ctx, accountID, bindingID)
	if err != nil {
		return ChannelClientBindingView{}, err
	}
	if err := s.requireResourceManage(ctx, accountID, p, current.Channel, current.ChannelResource.ID); err != nil {
		return ChannelClientBindingView{}, err
	}
	if _, err = s.accessibleClient(ctx, accountID, p, current.ClientAccountID); err != nil {
		return ChannelClientBindingView{}, err
	}
	target, err := s.accessibleClient(ctx, accountID, p, in.TargetClientAccountID)
	if err != nil {
		return ChannelClientBindingView{}, err
	}
	reason, key, effectiveAt, err := normalizeBindingMutation(in.Reason, in.IdempotencyKey, in.EffectiveAt)
	if err != nil {
		return ChannelClientBindingView{}, err
	}
	in.TargetClientAccountID = target.ID
	in.Reason = reason
	in.IdempotencyKey = key
	in.EffectiveAt = effectiveAt
	requestHash := channelBindingRequestHash("reassign", map[string]any{
		"bindingId":             bindingID,
		"targetClientAccountId": target.ID,
		"effectiveAt":           effectiveAt.Format(time.RFC3339Nano),
		"reason":                reason,
		"expectedRevision":      in.ExpectedRevision,
	})
	successorID, err := s.repo.ReassignChannelClientBinding(
		ctx, accountID, bindingID, requestHash, in, p.UserID,
	)
	if err != nil {
		return ChannelClientBindingView{}, err
	}
	return s.repo.GetChannelClientBinding(ctx, accountID, successorID)
}

func (s *ChannelClientBindingService) End(
	ctx context.Context,
	accountID, bindingID string,
	p auth.Principal,
	in EndChannelClientBindingInput,
) (ChannelClientBindingView, error) {
	if err := s.requireManage(ctx, accountID, p); err != nil {
		return ChannelClientBindingView{}, err
	}
	if !omnichannelUUIDPattern.MatchString(strings.TrimSpace(bindingID)) || in.ExpectedRevision < 1 {
		return ChannelClientBindingView{}, ErrValidation
	}
	current, err := s.repo.GetChannelClientBinding(ctx, accountID, bindingID)
	if err != nil {
		return ChannelClientBindingView{}, err
	}
	if err := s.requireResourceManage(ctx, accountID, p, current.Channel, current.ChannelResource.ID); err != nil {
		return ChannelClientBindingView{}, err
	}
	if _, err = s.accessibleClient(ctx, accountID, p, current.ClientAccountID); err != nil {
		return ChannelClientBindingView{}, err
	}
	reason, key, effectiveAt, err := normalizeBindingMutation(in.Reason, in.IdempotencyKey, in.EffectiveAt)
	if err != nil {
		return ChannelClientBindingView{}, err
	}
	in.Reason = reason
	in.IdempotencyKey = key
	in.EffectiveAt = effectiveAt
	requestHash := channelBindingRequestHash("end", map[string]any{
		"bindingId":        bindingID,
		"effectiveAt":      effectiveAt.Format(time.RFC3339Nano),
		"reason":           reason,
		"expectedRevision": in.ExpectedRevision,
	})
	endedID, err := s.repo.EndChannelClientBinding(
		ctx, accountID, bindingID, requestHash, in, p.UserID,
	)
	if err != nil {
		return ChannelClientBindingView{}, err
	}
	return s.repo.GetChannelClientBinding(ctx, accountID, endedID)
}

func normalizeChannelBindingWrite(channel, resourceID, reason, key string) (string, string, string, string, error) {
	channel = strings.ToUpper(strings.TrimSpace(channel))
	resourceID = strings.TrimSpace(resourceID)
	if channel != "WHATSAPP" && channel != "INSTAGRAM" {
		return "", "", "", "", ErrValidation
	}
	if !omnichannelUUIDPattern.MatchString(resourceID) {
		return "", "", "", "", ErrValidation
	}
	reason = strings.TrimSpace(reason)
	key = strings.TrimSpace(key)
	if reason == "" || len(reason) > 500 || key == "" || len(key) > 200 {
		return "", "", "", "", ErrValidation
	}
	return channel, resourceID, reason, key, nil
}

func normalizeBindingMutation(reason, key string, at time.Time) (string, string, time.Time, error) {
	reason = strings.TrimSpace(reason)
	key = strings.TrimSpace(key)
	if reason == "" || len(reason) > 500 || key == "" || len(key) > 200 || at.IsZero() {
		return "", "", time.Time{}, ErrValidation
	}
	return reason, key, at.UTC(), nil
}

func channelBindingRequestHash(operation string, payload any) string {
	raw, _ := json.Marshal(struct {
		Operation string `json:"operation"`
		Payload   any    `json:"payload"`
	}{Operation: operation, Payload: payload})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
