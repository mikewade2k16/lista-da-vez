package customerintelligence

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type auditEventPageFoundationFake struct {
	FoundationRepository
	queries []auditEventRepositoryQuery
	scopes  []Scope
	items   []AuditEvent
	err     error
}

func (f *auditEventPageFoundationFake) ListAuditEventPage(
	_ context.Context,
	scope Scope,
	query auditEventRepositoryQuery,
) ([]AuditEvent, error) {
	f.scopes = append(f.scopes, scope)
	f.queries = append(f.queries, query)
	return append([]AuditEvent(nil), f.items...), f.err
}

func newAuditEventPageService(repository FoundationRepository) *Service {
	return NewServiceWithRepositories(
		repository,
		nil,
		nil,
		nil,
		nil,
		WithClientScopeAuthorizer(ClientScopeAuthorizerFunc(allowEveryClient)),
	)
}

func TestAuditEventPageNormalizesFiltersAndBuildsOpaqueCursor(t *testing.T) {
	t.Parallel()

	firstTime := time.Date(2026, 7, 23, 15, 0, 0, 0, time.UTC)
	secondTime := firstTime.Add(-time.Minute)
	thirdTime := secondTime.Add(-time.Minute)
	repository := &auditEventPageFoundationFake{items: []AuditEvent{
		{ID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", OccurredAt: firstTime},
		{ID: "99999999-9999-4999-8999-999999999999", OccurredAt: secondTime},
		{ID: "88888888-8888-4888-8888-888888888888", OccurredAt: thirdTime},
	}}
	service := newAuditEventPageService(repository)

	page, err := service.AuditEventPage(
		context.Background(),
		Scope{AccountID: testAccount, ClientAccountID: testClient},
		AuditEventQuery{
			Action:       "fact.created",
			EntityType:   "fact",
			OccurredFrom: "2026-07-20T10:15:30-03:00",
			OccurredTo:   "2026-07-23T15:00:00Z",
			Limit:        2,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 || page.Items[0].ID != repository.items[0].ID ||
		page.Items[1].ID != repository.items[1].ID {
		t.Fatalf("items = %#v", page.Items)
	}
	if page.NextCursor == "" ||
		strings.ContainsAny(page.NextCursor, "+/=") {
		t.Fatalf("cursor nao e base64url sem padding: %q", page.NextCursor)
	}
	if len(repository.queries) != 1 || len(repository.scopes) != 1 {
		t.Fatalf(
			"repository calls: queries=%d scopes=%d",
			len(repository.queries),
			len(repository.scopes),
		)
	}
	query := repository.queries[0]
	if query.Action != "fact.created" || query.EntityType != "fact" ||
		query.Limit != 3 {
		t.Fatalf("query = %#v", query)
	}
	wantFrom := time.Date(2026, 7, 20, 13, 15, 30, 0, time.UTC)
	wantTo := time.Date(2026, 7, 23, 15, 0, 0, 0, time.UTC)
	if query.OccurredFrom == nil || !query.OccurredFrom.Equal(wantFrom) ||
		query.OccurredTo == nil || !query.OccurredTo.Equal(wantTo) {
		t.Fatalf(
			"range normalizado: from=%v to=%v",
			query.OccurredFrom,
			query.OccurredTo,
		)
	}
	decoded, err := decodeAuditEventCursor(page.NextCursor)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.ID != page.Items[1].ID ||
		!decoded.OccurredAt.Equal(page.Items[1].OccurredAt) {
		t.Fatalf("cursor = %#v", decoded)
	}

	repository.items = []AuditEvent{repository.items[2]}
	nextPage, err := service.AuditEventPage(
		context.Background(),
		Scope{AccountID: testAccount, ClientAccountID: testClient},
		AuditEventQuery{Cursor: page.NextCursor, Limit: 2},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(nextPage.Items) != 1 || nextPage.Items[0].ID != repository.items[0].ID ||
		nextPage.NextCursor != "" {
		t.Fatalf("next page = %#v", nextPage)
	}
	nextQuery := repository.queries[1]
	if nextQuery.Cursor == nil ||
		nextQuery.Cursor.ID != decoded.ID ||
		!nextQuery.Cursor.OccurredAt.Equal(decoded.OccurredAt) {
		t.Fatalf("cursor repassado = %#v", nextQuery.Cursor)
	}
}

func TestAuditEventPageReturnsStableEmptyCollection(t *testing.T) {
	t.Parallel()

	repository := &auditEventPageFoundationFake{}
	page, err := newAuditEventPageService(repository).AuditEventPage(
		context.Background(),
		Scope{AccountID: testAccount, ClientAccountID: testClient},
		AuditEventQuery{Limit: 50},
	)
	if err != nil {
		t.Fatal(err)
	}
	if page.Items == nil || len(page.Items) != 0 || page.NextCursor != "" {
		t.Fatalf("page = %#v", page)
	}
	encoded, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != `{"items":[],"nextCursor":""}` {
		t.Fatalf("wire contract = %s", encoded)
	}
}

func TestAuditEventPageRejectsInvalidFiltersBeforeRepository(t *testing.T) {
	t.Parallel()

	tests := map[string]AuditEventQuery{
		"action unsafe": {
			Action: "Fact.Created",
			Limit:  50,
		},
		"entity whitespace": {
			EntityType: " fact",
			Limit:      50,
		},
		"from malformed": {
			OccurredFrom: "2026-07-23",
			Limit:        50,
		},
		"to malformed": {
			OccurredTo: "not-a-time",
			Limit:      50,
		},
		"inverted range": {
			OccurredFrom: "2026-07-24T00:00:00Z",
			OccurredTo:   "2026-07-23T00:00:00Z",
			Limit:        50,
		},
		"invalid cursor": {
			Cursor: "not_base64url",
			Limit:  50,
		},
		"zero limit": {
			Limit: 0,
		},
		"limit above maximum": {
			Limit: 201,
		},
	}
	for name, input := range tests {
		name, input := name, input
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			repository := &auditEventPageFoundationFake{}
			_, err := newAuditEventPageService(repository).AuditEventPage(
				context.Background(),
				Scope{AccountID: testAccount, ClientAccountID: testClient},
				input,
			)
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("err = %v", err)
			}
			if len(repository.queries) != 0 {
				t.Fatalf("repository recebeu %d queries", len(repository.queries))
			}
		})
	}
}

func TestDecodeAuditEventCursorRejectsMalformedPayloads(t *testing.T) {
	t.Parallel()

	validTime := "2026-07-23T15:00:00Z"
	validID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	encode := func(payload string) string {
		return base64.RawURLEncoding.EncodeToString([]byte(payload))
	}
	valid, err := encodeAuditEventCursor(auditEventCursor{
		OccurredAt: time.Date(2026, 7, 23, 15, 0, 0, 0, time.UTC),
		ID:         validID,
	})
	if err != nil {
		t.Fatal(err)
	}
	tests := map[string]string{
		"not base64url":     "!",
		"padding":           valid + "=",
		"surrounding space": " " + valid,
		"unknown field":     encode(`{"t":"` + validTime + `","i":"` + validID + `","x":1}`),
		"trailing json":     encode(`{"t":"` + validTime + `","i":"` + validID + `" } {}`),
		"reordered fields":  encode(`{"i":"` + validID + `","t":"` + validTime + `"}`),
		"zero timestamp":    encode(`{"t":"0001-01-01T00:00:00Z","i":"` + validID + `"}`),
		"invalid uuid":      encode(`{"t":"` + validTime + `","i":"not-a-uuid"}`),
	}
	for name, raw := range tests {
		name, raw := name, raw
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := decodeAuditEventCursor(raw); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestAuditEventQueryFromRequest(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequestWithContext(
		context.Background(),
		"GET",
		"/v1/customer-intelligence/audit-events"+
			"?action=fact.created"+
			"&entityType=fact"+
			"&occurredFrom=2026-07-20T13%3A15%3A30Z"+
			"&occurredTo=2026-07-23T15%3A00%3A00Z"+
			"&cursor=opaque"+
			"&limit=25",
		nil,
	)
	query, err := auditEventQueryFromRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	if query.Action != "fact.created" ||
		query.EntityType != "fact" ||
		query.OccurredFrom != "2026-07-20T13:15:30Z" ||
		query.OccurredTo != "2026-07-23T15:00:00Z" ||
		query.Cursor != "opaque" ||
		query.Limit != 25 {
		t.Fatalf("query = %#v", query)
	}

	defaultQuery, err := auditEventQueryFromRequest(httptest.NewRequestWithContext(
		context.Background(),
		"GET",
		"/v1/customer-intelligence/audit-events",
		nil,
	))
	if err != nil || defaultQuery.Limit != defaultAuditEventPageLimit {
		t.Fatalf("default query = %#v err=%v", defaultQuery, err)
	}
}

func TestAuditEventQueryFromRequestRejectsAmbiguousOrInvalidLimit(t *testing.T) {
	t.Parallel()

	for _, rawQuery := range []string{
		"action=fact.created&action=fact.updated",
		"entityType=fact&entityType=summary",
		"limit=",
		"limit=abc",
		"limit=01",
		"limit=0",
		"limit=201",
	} {
		rawQuery := rawQuery
		t.Run(rawQuery, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequestWithContext(
				context.Background(),
				"GET",
				"/v1/customer-intelligence/audit-events?"+rawQuery,
				nil,
			)
			if _, err := auditEventQueryFromRequest(request); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestAuditEventsHandlerReturnsValidationErrorForInvalidQuery(t *testing.T) {
	t.Parallel()

	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(
		context.Background(),
		"GET",
		"/v1/customer-intelligence/audit-events?limit=0",
		nil,
	)
	auditEventsGet(nil).ServeHTTP(response, request)
	if response.Code != 422 {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Error.Code != "validation_error" {
		t.Fatalf("error code = %q body=%s", payload.Error.Code, response.Body.String())
	}
}
