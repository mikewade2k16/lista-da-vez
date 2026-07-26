package socialpublishing

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

func TestCreatePostRequiresIdempotencyKey(t *testing.T) {
	request := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/social-publishing/posts",
		strings.NewReader(`{
			"caption":"post",
			"mediaUrl":"https://cdn.example.com/post.jpg",
			"mediaType":"image",
			"status":"draft"
		}`),
	)
	response := httptest.NewRecorder()

	handlePostsPost(nil).ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
	if !strings.Contains(response.Body.String(), "invalid_idempotency_key") {
		t.Fatalf("body = %s", response.Body.String())
	}
}

func TestCreatePostRejectsReservedSourceFields(t *testing.T) {
	request := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/social-publishing/posts",
		strings.NewReader(`{
			"caption":"post",
			"mediaUrl":"https://cdn.example.com/post.jpg",
			"mediaType":"image",
			"status":"draft",
			"idempotencyKey":"request-1",
			"sourceType":"calendar"
		}`),
	)
	response := httptest.NewRecorder()

	handlePostsPost(nil).ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
	if !strings.Contains(response.Body.String(), "invalid_body") {
		t.Fatalf("body = %s", response.Body.String())
	}
}

func TestParseStatusesQuery(t *testing.T) {
	statuses, err := parseStatusesQuery(" scheduled, publishing ,failed ")
	if err != nil {
		t.Fatalf("parseStatusesQuery() error = %v", err)
	}
	want := []PostStatus{PostStatusScheduled, PostStatusPublishing, PostStatusFailed}
	if len(statuses) != len(want) {
		t.Fatalf("statuses = %v, want %v", statuses, want)
	}
	for index := range want {
		if statuses[index] != want[index] {
			t.Fatalf("statuses = %v, want %v", statuses, want)
		}
	}
	if _, err := parseStatusesQuery("scheduled,,failed"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("empty status error = %v, want ErrInvalidInput", err)
	}
}

func TestParsePostOrderQuery(t *testing.T) {
	order, err := parsePostOrderQuery(" scheduled ")
	if err != nil {
		t.Fatalf("parsePostOrderQuery() error = %v", err)
	}
	if order != PostListOrderScheduled {
		t.Fatalf("order = %q, want %q", order, PostListOrderScheduled)
	}

	defaultOrder, err := parsePostOrderQuery("")
	if err != nil {
		t.Fatalf("parsePostOrderQuery(default) error = %v", err)
	}
	if defaultOrder != PostListOrderCreated {
		t.Fatalf("default order = %q, want %q", defaultOrder, PostListOrderCreated)
	}

	if _, err := parsePostOrderQuery("scheduled desc"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("unsafe order error = %v, want ErrInvalidInput", err)
	}
}

func TestListPostsRejectsUnsafeOrderAtHTTPBoundary(t *testing.T) {
	request := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/v1/social-publishing/posts?order=scheduled%20desc",
		nil,
	)
	response := httptest.NewRecorder()

	handlePostsGet(nil).ServeHTTP(response, request)

	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", response.Code)
	}
}

func TestSummaryRouteUsesViewPermissionAndReturnsOnlySummary(t *testing.T) {
	repository := &stubRepository{summary: Summary{
		Counts: map[string]int64{"scheduled": 2},
		Upcoming: []Post{{
			ID:     "post-1",
			Status: PostStatusScheduled,
		}},
	}}
	checker := &httpPermissionChecker{allowed: true}
	mux := authenticatedSocialPublishingMux(
		t,
		NewService(repository, &stubProvider{}, nil, nil),
		checker,
	)
	request := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/v1/social-publishing/summary",
		nil,
	)
	request.Header.Set("Authorization", "Bearer test-token")
	request.Header.Set("X-Account-Id", "account-1")
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", response.Code, response.Body.String())
	}
	if checker.permission != PermissionView {
		t.Fatalf("permission = %q, want %q", checker.permission, PermissionView)
	}
	if repository.summaryAccountID != "account-1" {
		t.Fatalf("summary account = %q, want account-1", repository.summaryAccountID)
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, ok := payload["counts"]; !ok {
		t.Fatalf("summary response missing counts: %s", response.Body.String())
	}
	if _, ok := payload["upcoming"]; !ok {
		t.Fatalf("summary response missing upcoming: %s", response.Body.String())
	}
	for _, forbidden := range []string{"analytics", "connection", "accessToken"} {
		if _, ok := payload[forbidden]; ok {
			t.Fatalf("summary response leaked %q: %s", forbidden, response.Body.String())
		}
	}
}

func TestScopeRouteUsesViewPermissionAndPrincipalAccount(t *testing.T) {
	repository := &stubRepository{scope: PublishingScope{
		CanSelect:      false,
		LockedClientID: "account-1",
		Clients: []PublishingClient{{
			ID: "account-1", Name: "Cliente Um",
		}},
	}}
	checker := &httpPermissionChecker{allowed: true}
	service := NewService(
		repository,
		&stubProvider{},
		nil,
		nil,
		WithServicePermissionChecker(checker),
	)
	mux := authenticatedSocialPublishingMux(t, service, checker)
	request := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/v1/social-publishing/scope",
		nil,
	)
	request.Header.Set("Authorization", "Bearer test-token")
	request.Header.Set("X-Account-Id", "account-1")
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", response.Code, response.Body.String())
	}
	if checker.permission != PermissionView {
		t.Fatalf("permission = %q, want %q", checker.permission, PermissionView)
	}
	if repository.scopeAccountID != "account-1" || repository.scopeUserID != "user-1" {
		t.Fatalf(
			"scope principal = (%q, %q)",
			repository.scopeAccountID,
			repository.scopeUserID,
		)
	}
	var payload PublishingScope
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.CanSelect || payload.LockedClientID != "account-1" ||
		len(payload.Clients) != 1 {
		t.Fatalf("scope response = %#v", payload)
	}
}

func TestPortfolioRouteRequiresAnalyticsAndNeverReturnsSecrets(t *testing.T) {
	capturedAt := time.Date(2026, 7, 23, 17, 0, 0, 0, time.UTC)
	repository := &stubRepository{
		scope: PublishingScope{
			CanSelect: true,
			Clients: []PublishingClient{{
				ID: "client-1", Name: "Cliente Um",
			}},
		},
		portfolioRecords: []portfolioClientRecord{{
			Client: PortfolioClient{
				AccountID: "client-1", AccountName: "Cliente Um",
				Connected: true, Username: "cliente_um", Published: 2,
				Reach: 30, TotalInteractions: 8,
			},
			Views: 40, Likes: 3, Comments: 2, Saved: 2, Shares: 1,
			CapturedAt: &capturedAt,
		}},
	}
	checker := &httpPermissionChecker{allowed: true}
	service := NewService(
		repository,
		&stubProvider{},
		nil,
		nil,
		WithServicePermissionChecker(checker),
	)
	mux := authenticatedSocialPublishingMux(t, service, checker)
	request := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/v1/social-publishing/portfolio",
		nil,
	)
	request.Header.Set("Authorization", "Bearer test-token")
	request.Header.Set("X-Account-Id", "agency-1")
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", response.Code, response.Body.String())
	}
	if checker.permission != PermissionAnalytics {
		t.Fatalf("permission = %q, want %q", checker.permission, PermissionAnalytics)
	}
	body := strings.ToLower(response.Body.String())
	for _, forbidden := range []string{"accesstoken", "ciphertext", "secretlast4"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("portfolio leaked %q: %s", forbidden, response.Body.String())
		}
	}
	var payload Portfolio
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.ClientCount != 1 || payload.ConnectedClients != 1 ||
		payload.Views != 40 || len(payload.Clients) != 1 {
		t.Fatalf("portfolio response = %#v", payload)
	}
}

func TestAnalyticsPostsRouteNormalizesPostIDs(t *testing.T) {
	repository := &stubRepository{}
	checker := &httpPermissionChecker{allowed: true}
	mux := authenticatedSocialPublishingMux(
		t,
		NewService(repository, &stubProvider{}, nil, nil),
		checker,
	)
	const first = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	const second = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	request := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/v1/social-publishing/analytics/posts?postIds="+
			strings.ToUpper(first)+","+second+","+first+"&limit=25",
		nil,
	)
	request.Header.Set("Authorization", "Bearer test-token")
	request.Header.Set("X-Account-Id", "account-1")
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", response.Code, response.Body.String())
	}
	if checker.permission != PermissionAnalytics {
		t.Fatalf("permission = %q, want %q", checker.permission, PermissionAnalytics)
	}
	got := repository.analyticsFilter
	if got.Limit != 25 {
		t.Fatalf("analytics limit = %d, want 25", got.Limit)
	}
	if len(got.PostIDs) != 2 || got.PostIDs[0] != first || got.PostIDs[1] != second {
		t.Fatalf("analytics post IDs = %#v, want deduplicated IDs", got.PostIDs)
	}
}

func TestParsePostIDsQueryRejectsEmptyCSVItem(t *testing.T) {
	if _, err := parsePostIDsQuery("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa,,bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("parsePostIDsQuery() error = %v, want ErrInvalidInput", err)
	}
}

type httpTokenManager struct{}

func (httpTokenManager) Issue(string, auth.User) (auth.SessionView, error) {
	return auth.SessionView{}, nil
}

func (httpTokenManager) Parse(string) (auth.Principal, error) {
	return auth.Principal{UserID: "user-1"}, nil
}

type httpUserRepository struct{}

func (httpUserRepository) FindByEmail(context.Context, string) (auth.User, error) {
	return auth.User{}, auth.ErrUnauthorized
}

func (httpUserRepository) FindByID(context.Context, string) (auth.User, error) {
	return auth.User{}, auth.ErrUnauthorized
}

func (httpUserRepository) LoadUserForAuth(context.Context, string) (auth.User, error) {
	return auth.User{
		ID:       "user-1",
		Role:     auth.RoleMarketing,
		TenantID: "tenant-1",
		Active:   true,
	}, nil
}

func (httpUserRepository) UpdateProfile(
	context.Context,
	string,
	string,
	string,
) (auth.User, error) {
	return auth.User{}, nil
}

func (httpUserRepository) UpdatePassword(
	context.Context,
	string,
	string,
	bool,
) (auth.User, error) {
	return auth.User{}, nil
}

func (httpUserRepository) UpdateAvatarPath(
	context.Context,
	string,
	string,
) (auth.User, error) {
	return auth.User{}, nil
}

type httpAccountChecker struct{}

func (httpAccountChecker) IsMember(context.Context, string, string) (bool, error) {
	return true, nil
}

type httpPermissionChecker struct {
	allowed    bool
	permission string
}

func (c *httpPermissionChecker) HasAccountPermission(
	_ context.Context,
	_, _ string,
	permission string,
) (bool, error) {
	c.permission = permission
	return c.allowed, nil
}

func authenticatedSocialPublishingMux(
	t *testing.T,
	service *Service,
	checker *httpPermissionChecker,
) *http.ServeMux {
	t.Helper()
	authService := auth.NewService(
		httpUserRepository{},
		nil,
		httpTokenManager{},
		nil,
		nil,
		nil,
		nil,
	)
	middleware := auth.NewMiddleware(authService)
	middleware.SetAccountChecker(httpAccountChecker{})
	mux := http.NewServeMux()
	RegisterRoutes(mux, service, middleware, newPermissionGate(checker))
	return mux
}
