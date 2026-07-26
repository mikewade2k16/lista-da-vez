package omnichannel

import (
	"context"
	"errors"
	"strings"
	"time"
)

// CustomerEvidenceRequest is the headless, read-only contract exposed to
// composition-root source adapters. Scope is always explicit and the store
// rechecks account + client snapshots before returning a message.
type CustomerEvidenceRequest struct {
	AccountID            string
	ClientAccountID      string
	ContactSourceIDs     []string
	LookbackDays         int
	Limit                int
	IncludeMediaMetadata bool
}

type CustomerMessageEvidence struct {
	MessageID         string    `json:"messageId"`
	ConversationID    string    `json:"conversationId"`
	ContactSourceID   string    `json:"contactSourceId"`
	Channel           string    `json:"channel"`
	Direction         string    `json:"direction"`
	MessageType       string    `json:"messageType"`
	Content           string    `json:"content"`
	Status            string    `json:"status"`
	MediaMimeType     string    `json:"mediaMimeType,omitempty"`
	MediaFileName     string    `json:"mediaFileName,omitempty"`
	MediaCaption      string    `json:"mediaCaption,omitempty"`
	MediaDurationSecs *int      `json:"mediaDurationSeconds,omitempty"`
	OccurredAt        time.Time `json:"occurredAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

func (s *Service) CustomerEvidence(
	ctx context.Context,
	request CustomerEvidenceRequest,
) ([]CustomerMessageEvidence, error) {
	if s == nil || s.store == nil ||
		!omnichannelUUIDPattern.MatchString(strings.TrimSpace(request.AccountID)) ||
		!omnichannelUUIDPattern.MatchString(strings.TrimSpace(request.ClientAccountID)) {
		return nil, ErrNotFound
	}
	seen := make(map[string]struct{}, len(request.ContactSourceIDs))
	contactIDs := make([]string, 0, len(request.ContactSourceIDs))
	for _, value := range request.ContactSourceIDs {
		value = strings.TrimSpace(value)
		if !omnichannelUUIDPattern.MatchString(value) {
			return nil, ErrNotFound
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		contactIDs = append(contactIDs, value)
		if len(contactIDs) == 50 {
			break
		}
	}
	if len(contactIDs) == 0 {
		return []CustomerMessageEvidence{}, nil
	}
	if request.LookbackDays <= 0 {
		request.LookbackDays = 90
	}
	if request.LookbackDays > 365 {
		request.LookbackDays = 365
	}
	if request.Limit <= 0 {
		request.Limit = 100
	}
	if request.Limit > 200 {
		request.Limit = 200
	}
	items, err := s.store.ListCustomerMessageEvidence(ctx, request, contactIDs)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	for index := range items {
		if len(items[index].Content) > 20000 {
			items[index].Content = items[index].Content[:20000]
		}
		if !request.IncludeMediaMetadata {
			items[index].MediaMimeType = ""
			items[index].MediaFileName = ""
			items[index].MediaCaption = ""
			items[index].MediaDurationSecs = nil
		}
	}
	return items, nil
}
