package customerintelligence

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
)

const (
	defaultAuditEventPageLimit = 50
	maxAuditEventPageLimit     = 200
	maxAuditEventCursorBytes   = 512
	maxAuditEventFilterBytes   = 160
)

var errAuditEventPageRepositoryUnavailable = errors.New(
	"customer intelligence: repositorio de paginacao de auditoria indisponivel",
)

type AuditEventPage struct {
	Items      []AuditEvent `json:"items"`
	NextCursor string       `json:"nextCursor"`
}

type AuditEventQuery struct {
	Action       string
	EntityType   string
	OccurredFrom string
	OccurredTo   string
	Cursor       string
	Limit        int
}

type auditEventCursor struct {
	OccurredAt time.Time
	ID         string
}

type auditEventCursorPayload struct {
	OccurredAt time.Time `json:"t"`
	ID         string    `json:"i"`
}

type auditEventRepositoryQuery struct {
	Action       string
	EntityType   string
	OccurredFrom *time.Time
	OccurredTo   *time.Time
	Cursor       *auditEventCursor
	Limit        int
}

type auditEventPageRepository interface {
	ListAuditEventPage(
		ctx context.Context,
		scope Scope,
		query auditEventRepositoryQuery,
	) ([]AuditEvent, error)
}

func (s *Service) AuditEventPage(
	ctx context.Context,
	scope Scope,
	input AuditEventQuery,
) (AuditEventPage, error) {
	if err := s.authorizeScope(ctx, scope); err != nil {
		return AuditEventPage{}, err
	}
	query, err := normalizeAuditEventQuery(input)
	if err != nil {
		return AuditEventPage{}, err
	}
	repository, ok := s.foundation.(auditEventPageRepository)
	if !ok {
		return AuditEventPage{}, errAuditEventPageRepositoryUnavailable
	}
	query.Limit++
	items, err := repository.ListAuditEventPage(ctx, scope, query)
	if err != nil {
		return AuditEventPage{}, err
	}
	page := AuditEventPage{Items: []AuditEvent{}}
	requestedLimit := query.Limit - 1
	if len(items) == 0 {
		return page, nil
	}
	if len(items) <= requestedLimit {
		page.Items = items
		return page, nil
	}
	page.Items = items[:requestedLimit]
	last := page.Items[len(page.Items)-1]
	page.NextCursor, err = encodeAuditEventCursor(auditEventCursor{
		OccurredAt: last.OccurredAt,
		ID:         last.ID,
	})
	if err != nil {
		return AuditEventPage{}, err
	}
	return page, nil
}

func normalizeAuditEventQuery(input AuditEventQuery) (auditEventRepositoryQuery, error) {
	action, err := validateAuditEventFilterKey(input.Action)
	if err != nil {
		return auditEventRepositoryQuery{}, err
	}
	entityType, err := validateAuditEventFilterKey(input.EntityType)
	if err != nil {
		return auditEventRepositoryQuery{}, err
	}
	occurredFrom, err := parseAuditEventFilterTime(input.OccurredFrom)
	if err != nil {
		return auditEventRepositoryQuery{}, err
	}
	occurredTo, err := parseAuditEventFilterTime(input.OccurredTo)
	if err != nil {
		return auditEventRepositoryQuery{}, err
	}
	if occurredFrom != nil && occurredTo != nil && occurredFrom.After(*occurredTo) {
		return auditEventRepositoryQuery{}, ErrInvalidInput
	}
	if input.Limit < 1 || input.Limit > maxAuditEventPageLimit {
		return auditEventRepositoryQuery{}, ErrInvalidInput
	}
	var cursor *auditEventCursor
	if input.Cursor != "" {
		decoded, decodeErr := decodeAuditEventCursor(input.Cursor)
		if decodeErr != nil {
			return auditEventRepositoryQuery{}, decodeErr
		}
		cursor = &decoded
	}
	return auditEventRepositoryQuery{
		Action:       action,
		EntityType:   entityType,
		OccurredFrom: occurredFrom,
		OccurredTo:   occurredTo,
		Cursor:       cursor,
		Limit:        input.Limit,
	}, nil
}

func validateAuditEventFilterKey(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if value != strings.TrimSpace(value) ||
		len(value) > maxAuditEventFilterBytes ||
		!safeKeyPattern.MatchString(value) {
		return "", ErrInvalidInput
	}
	return value, nil
}

func parseAuditEventFilterTime(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	if value != strings.TrimSpace(value) || len(value) > len(time.RFC3339Nano)+20 {
		return nil, ErrInvalidInput
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil, ErrInvalidInput
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

func encodeAuditEventCursor(cursor auditEventCursor) (string, error) {
	if cursor.OccurredAt.IsZero() || !validUUID(cursor.ID) {
		return "", ErrInvalidInput
	}
	payload, err := json.Marshal(auditEventCursorPayload{
		OccurredAt: cursor.OccurredAt.UTC(),
		ID:         cursor.ID,
	})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeAuditEventCursor(raw string) (auditEventCursor, error) {
	if raw == "" || raw != strings.TrimSpace(raw) || len(raw) > maxAuditEventCursorBytes {
		return auditEventCursor{}, ErrInvalidInput
	}
	payload, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil ||
		len(payload) == 0 ||
		base64.RawURLEncoding.EncodeToString(payload) != raw {
		return auditEventCursor{}, ErrInvalidInput
	}
	var decoded auditEventCursorPayload
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&decoded); err != nil {
		return auditEventCursor{}, ErrInvalidInput
	}
	var trailing any
	if err = decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return auditEventCursor{}, ErrInvalidInput
	}
	if decoded.OccurredAt.IsZero() || !validUUID(decoded.ID) {
		return auditEventCursor{}, ErrInvalidInput
	}
	canonical, err := json.Marshal(auditEventCursorPayload{
		OccurredAt: decoded.OccurredAt.UTC(),
		ID:         decoded.ID,
	})
	if err != nil || !bytes.Equal(payload, canonical) {
		return auditEventCursor{}, ErrInvalidInput
	}
	return auditEventCursor{
		OccurredAt: decoded.OccurredAt.UTC(),
		ID:         decoded.ID,
	}, nil
}
