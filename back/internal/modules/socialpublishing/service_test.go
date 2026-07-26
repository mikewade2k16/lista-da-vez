package socialpublishing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

func TestConnectionWithoutRecordReturnsDisconnected(t *testing.T) {
	repository := &stubRepository{connectionErr: ErrNotConnected}
	service := NewService(repository, &stubProvider{}, nil, nil)

	connection, err := service.Connection(context.Background(), "account-1")
	if err != nil {
		t.Fatalf("Connection() error = %v", err)
	}
	if connection.Status != "disconnected" || connection.Provider != "instagram" {
		t.Fatalf("Connection() = %#v, want safe disconnected state", connection)
	}
	if connection.Secret.Set {
		t.Fatal("disconnected connection must not expose a configured secret")
	}
}

func TestConnectValidatesAndEncryptsToken(t *testing.T) {
	box, err := secretbox.New(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	repository := &stubRepository{}
	provider := &stubProvider{profile: InstagramProfile{
		UserID: "ig-1", Username: "omni", AccountType: "BUSINESS",
	}}
	service := NewService(repository, provider, box, nil)

	connection, err := service.Connect(
		context.Background(),
		"account-1",
		"user-1",
		"token-super-secret-1234",
	)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if provider.validatedWith != "token-super-secret-1234" {
		t.Fatal("provider did not receive the transient token")
	}
	if repository.savedCiphertext == "" ||
		repository.savedCiphertext == "token-super-secret-1234" {
		t.Fatal("repository must receive ciphertext, never plaintext")
	}
	if repository.savedLast4 != "1234" {
		t.Fatalf("last4 = %q, want 1234", repository.savedLast4)
	}
	if !connection.Secret.Set || connection.Secret.Last4 != "1234" {
		t.Fatalf("secret status = %#v, want masked last4", connection.Secret)
	}
}

func TestCreateScheduledPostNormalizesUTCAndConnection(t *testing.T) {
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	when := time.Date(2026, 7, 23, 12, 30, 0, 0, time.FixedZone("BRT", -3*60*60))
	repository := &stubRepository{connection: ConnectionRecord{
		Connection:            Connection{ID: "connection-1", Status: "connected"},
		AccessTokenCiphertext: "v1:ciphertext",
	}}
	service := NewService(
		repository,
		&stubProvider{},
		nil,
		nil,
		WithServiceClock(func() time.Time { return now }),
	)

	result, err := service.CreatePost(context.Background(), "account-1", "user-1", CreatePostInput{
		Caption:      "  campanha  ",
		MediaURL:     "https://cdn.example.com/post.jpg",
		MediaType:    "image",
		Status:       PostStatusScheduled,
		ScheduledFor: &when,
		Timezone:     "America/Sao_Paulo",
	})
	if err != nil {
		t.Fatalf("CreatePost() error = %v", err)
	}
	if !result.Created || repository.createCommand.ConnectionID != "connection-1" {
		t.Fatalf("CreatePost() command = %#v", repository.createCommand)
	}
	if got := repository.createCommand.Input.ScheduledFor; got == nil ||
		!got.Equal(when.UTC()) || got.Location() != time.UTC {
		t.Fatalf("scheduledFor = %v, want UTC %v", got, when.UTC())
	}
	if repository.createCommand.Input.Caption != "campanha" {
		t.Fatalf("caption = %q, want trimmed", repository.createCommand.Input.Caption)
	}
}

func TestPatchScheduledPostIsAllowedAndReturnsDraft(t *testing.T) {
	scheduled := time.Now().UTC().Add(time.Hour)
	repository := &stubRepository{post: Post{
		ID:           "post-1",
		Status:       PostStatusScheduled,
		Version:      3,
		Caption:      "antes",
		MediaURL:     "https://cdn.example.com/post.jpg",
		ScheduledFor: &scheduled,
		Timezone:     "UTC",
		ConnectionID: "historical-connection",
	}}
	service := NewService(repository, &stubProvider{}, nil, nil)
	caption := "depois"

	post, err := service.PatchPost(
		context.Background(),
		"account-1",
		"user-1",
		"post-1",
		PatchPostInput{Caption: &caption, Version: 3},
	)
	if err != nil {
		t.Fatalf("PatchPost() error = %v", err)
	}
	if post.Status != PostStatusDraft || post.ScheduledFor != nil {
		t.Fatalf("PatchPost() = %#v, want draft without schedule", post)
	}
	if repository.updateCommand.Post.ConnectionID != "historical-connection" {
		t.Fatalf(
			"PatchPost() connection = %q, want historical target",
			repository.updateCommand.Post.ConnectionID,
		)
	}
}

func TestCreateRejectsNonHTTPSMedia(t *testing.T) {
	service := NewService(&stubRepository{}, &stubProvider{}, nil, nil)
	_, err := service.CreatePost(context.Background(), "account-1", "user-1", CreatePostInput{
		MediaURL:  "http://cdn.example.com/post.jpg",
		MediaType: "image",
		Status:    PostStatusDraft,
	})
	if !errors.Is(err, ErrInvalidMediaURL) {
		t.Fatalf("CreatePost() error = %v, want ErrInvalidMediaURL", err)
	}
}

func TestUnknownPublishOutcomeCannotBePatchedOrRescheduled(t *testing.T) {
	repository := &stubRepository{post: Post{
		ID:            "post-1",
		Status:        PostStatusFailed,
		Version:       4,
		Caption:       "post",
		MediaURL:      "https://cdn.example.com/post.jpg",
		Timezone:      "UTC",
		LastErrorCode: "publish_outcome_unknown",
	}}
	service := NewService(repository, &stubProvider{}, nil, nil)
	caption := "alterado"

	_, patchErr := service.PatchPost(
		context.Background(),
		"account-1",
		"user-1",
		"post-1",
		PatchPostInput{Caption: &caption, Version: 4},
	)
	if !errors.Is(patchErr, ErrInvalidState) {
		t.Fatalf("PatchPost() error = %v, want ErrInvalidState", patchErr)
	}
	_, scheduleErr := service.SchedulePost(
		context.Background(),
		"account-1",
		"user-1",
		"post-1",
		SchedulePostInput{
			ScheduledFor: time.Now().UTC().Add(time.Hour),
			Timezone:     "UTC",
			Version:      4,
		},
	)
	if !errors.Is(scheduleErr, ErrInvalidState) {
		t.Fatalf("SchedulePost() error = %v, want ErrInvalidState", scheduleErr)
	}
}

func TestListPostsCombinesAndDeduplicatesStatusFilters(t *testing.T) {
	repository := &stubRepository{}
	service := NewService(repository, &stubProvider{}, nil, nil)

	_, err := service.ListPosts(context.Background(), "account-1", ListPostsFilter{
		Status: PostStatusScheduled,
		Statuses: []PostStatus{
			PostStatusPublishing,
			PostStatusScheduled,
			PostStatusFailed,
		},
	})
	if err != nil {
		t.Fatalf("ListPosts() error = %v", err)
	}
	got := repository.listFilter.Statuses
	want := []PostStatus{PostStatusScheduled, PostStatusPublishing, PostStatusFailed}
	if len(got) != len(want) {
		t.Fatalf("statuses = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("statuses = %v, want %v", got, want)
		}
	}
	if repository.listFilter.Status != "" {
		t.Fatalf("legacy status was not normalized: %q", repository.listFilter.Status)
	}
	if repository.listFilter.Order != PostListOrderCreated {
		t.Fatalf("order = %q, want %q", repository.listFilter.Order, PostListOrderCreated)
	}
}

func TestListPostsRejectsUnknownStatus(t *testing.T) {
	service := NewService(&stubRepository{}, &stubProvider{}, nil, nil)

	_, err := service.ListPosts(context.Background(), "account-1", ListPostsFilter{
		Statuses: []PostStatus{"queued"},
	})

	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("ListPosts() error = %v, want ErrInvalidInput", err)
	}
}

func TestListPostsAcceptsScheduledOrder(t *testing.T) {
	repository := &stubRepository{}
	service := NewService(repository, &stubProvider{}, nil, nil)

	_, err := service.ListPosts(context.Background(), "account-1", ListPostsFilter{
		Order: PostListOrderScheduled,
	})

	if err != nil {
		t.Fatalf("ListPosts() error = %v", err)
	}
	if repository.listFilter.Order != PostListOrderScheduled {
		t.Fatalf("order = %q, want %q", repository.listFilter.Order, PostListOrderScheduled)
	}
}

func TestListPostsRejectsUnknownOrder(t *testing.T) {
	service := NewService(&stubRepository{}, &stubProvider{}, nil, nil)

	_, err := service.ListPosts(context.Background(), "account-1", ListPostsFilter{
		Order: "scheduled_for desc",
	})

	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("ListPosts() error = %v, want ErrInvalidInput", err)
	}
}

func TestSummaryUsesTenantScopeAndServiceClock(t *testing.T) {
	now := time.Date(2026, 7, 23, 15, 0, 0, 0, time.UTC)
	repository := &stubRepository{summary: Summary{
		Counts:   map[string]int64{"scheduled": 3},
		Upcoming: []Post{},
	}}
	service := NewService(
		repository,
		&stubProvider{},
		nil,
		nil,
		WithServiceClock(func() time.Time { return now }),
	)

	summary, err := service.Summary(context.Background(), "account-1")

	if err != nil {
		t.Fatalf("Summary() error = %v", err)
	}
	if summary.Counts["scheduled"] != 3 {
		t.Fatalf("Summary() = %#v", summary)
	}
	if repository.summaryAccountID != "account-1" || !repository.summaryNow.Equal(now) {
		t.Fatalf(
			"Summary repository scope = (%q, %s), want (account-1, %s)",
			repository.summaryAccountID,
			repository.summaryNow,
			now,
		)
	}
}

func TestPublishingScopeUsesPrincipalAndFiltersTargetPermissions(t *testing.T) {
	repository := &stubRepository{scope: PublishingScope{
		CanSelect: true,
		Clients: []PublishingClient{
			{ID: "client-a", Name: "Cliente A"},
			{ID: "client-b", Name: "Cliente B"},
		},
	}}
	checker := &stubTargetPermissionChecker{
		allowedByAccount: map[string]bool{"client-a": true},
	}
	service := NewService(
		repository,
		&stubProvider{},
		nil,
		nil,
		WithServicePermissionChecker(checker),
	)

	scope, err := service.PublishingScope(context.Background(), auth.Principal{
		UserID:    "user-1",
		AccountID: "agency-1",
		Role:      auth.RoleMarketing,
	})

	if err != nil {
		t.Fatalf("PublishingScope() error = %v", err)
	}
	if repository.scopeAccountID != "agency-1" ||
		repository.scopeUserID != "user-1" ||
		repository.scopePlatformAdmin {
		t.Fatalf(
			"repository scope = (%q, %q, %t)",
			repository.scopeAccountID,
			repository.scopeUserID,
			repository.scopePlatformAdmin,
		)
	}
	if len(scope.Clients) != 1 || scope.Clients[0].ID != "client-a" {
		t.Fatalf("scope clients = %#v, want only authorized client-a", scope.Clients)
	}
	if len(checker.checkedAccounts) != 2 {
		t.Fatalf("checked accounts = %#v, want both candidates", checker.checkedAccounts)
	}
}

func TestPublishingScopeRequiresResolvedUserAndAccount(t *testing.T) {
	service := NewService(&stubRepository{}, &stubProvider{}, nil, nil)
	for name, principal := range map[string]auth.Principal{
		"missing user":    {AccountID: "account-1", Role: auth.RolePlatformAdmin},
		"missing account": {UserID: "user-1", Role: auth.RolePlatformAdmin},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := service.PublishingScope(context.Background(), principal)
			if !errors.Is(err, ErrForbidden) {
				t.Fatalf("PublishingScope() error = %v, want ErrForbidden", err)
			}
		})
	}
}

func TestPublishingScopePlatformAdminBypassesTargetRBAC(t *testing.T) {
	repository := &stubRepository{scope: PublishingScope{
		CanSelect: true,
		Clients:   []PublishingClient{{ID: "client-a", Name: "Cliente A"}},
	}}
	service := NewService(repository, &stubProvider{}, nil, nil)

	scope, err := service.PublishingScope(context.Background(), auth.Principal{
		UserID:    "admin-1",
		AccountID: "any-active-account",
		Role:      auth.RolePlatformAdmin,
	})

	if err != nil {
		t.Fatalf("PublishingScope() error = %v", err)
	}
	if !repository.scopePlatformAdmin || len(scope.Clients) != 1 {
		t.Fatalf("platform scope = %#v", scope)
	}
}

func TestPublishingScopeLockedClientFailsClosedWithoutTargetPermission(t *testing.T) {
	repository := &stubRepository{scope: PublishingScope{
		CanSelect:      false,
		LockedClientID: "client-a",
		Clients:        []PublishingClient{{ID: "client-a", Name: "Cliente A"}},
	}}
	service := NewService(
		repository,
		&stubProvider{},
		nil,
		nil,
		WithServicePermissionChecker(&stubTargetPermissionChecker{
			allowedByAccount: map[string]bool{},
		}),
	)

	_, err := service.PublishingScope(context.Background(), auth.Principal{
		UserID:    "user-1",
		AccountID: "client-a",
		Role:      auth.RoleMarketing,
	})

	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("PublishingScope() error = %v, want ErrForbidden", err)
	}
}

func TestPortfolioDerivesAccountIDsFromScopeAndConsolidatesRows(t *testing.T) {
	now := time.Date(2026, 7, 23, 18, 0, 0, 0, time.UTC)
	firstCapture := now.Add(-time.Hour)
	lastCapture := now.Add(-time.Minute)
	repository := &stubRepository{
		scope: PublishingScope{
			CanSelect: true,
			Clients: []PublishingClient{
				{ID: "client-a", Name: "Cliente A"},
				{ID: "client-b", Name: "Cliente B"},
				{ID: "client-a", Name: "Duplicado"},
			},
		},
		portfolioRecords: []portfolioClientRecord{
			{
				Client: PortfolioClient{
					AccountID: "client-a", Connected: true, Draft: 1, Scheduled: 2,
					Published: 3, Reach: 10, TotalInteractions: 4,
				},
				Views: 20, Likes: 2, Comments: 1, Saved: 1, Shares: 1,
				CapturedAt: &firstCapture,
			},
			{
				Client: PortfolioClient{
					AccountID: "client-b", Failed: 1, Publishing: 1,
					Reach: 7, TotalInteractions: 3,
				},
				Views: 9, Likes: 1, Comments: 2, Saved: 3, Shares: 4,
				CapturedAt: &lastCapture,
			},
		},
	}
	service := NewService(
		repository,
		&stubProvider{},
		nil,
		nil,
		WithServiceClock(func() time.Time { return now }),
	)

	portfolio, err := service.Portfolio(context.Background(), auth.Principal{
		UserID:    "admin-1",
		AccountID: "agency-1",
		Role:      auth.RolePlatformAdmin,
	})

	if err != nil {
		t.Fatalf("Portfolio() error = %v", err)
	}
	if got := repository.portfolioAccountIDs; len(got) != 2 ||
		got[0] != "client-a" || got[1] != "client-b" {
		t.Fatalf("portfolio account IDs = %#v, want server-derived deduplicated IDs", got)
	}
	if !repository.portfolioNow.Equal(now) {
		t.Fatalf("portfolio now = %s, want %s", repository.portfolioNow, now)
	}
	if portfolio.ClientCount != 2 || portfolio.ConnectedClients != 1 ||
		portfolio.Draft != 1 || portfolio.Scheduled != 2 ||
		portfolio.Publishing != 1 || portfolio.Published != 3 || portfolio.Failed != 1 {
		t.Fatalf("portfolio counts = %#v", portfolio)
	}
	if portfolio.Views != 29 || portfolio.Reach != 17 ||
		portfolio.TotalInteractions != 7 || portfolio.Likes != 3 ||
		portfolio.Comments != 3 || portfolio.Saved != 4 || portfolio.Shares != 5 {
		t.Fatalf("portfolio analytics = %#v", portfolio)
	}
	if portfolio.CapturedAt == nil || !portfolio.CapturedAt.Equal(lastCapture) {
		t.Fatalf("capturedAt = %v, want %s", portfolio.CapturedAt, lastCapture)
	}
}

func TestPortfolioRejectsLockedScopeBeforeAggregateQuery(t *testing.T) {
	repository := &stubRepository{scope: PublishingScope{
		CanSelect:      false,
		LockedClientID: "client-a",
		Clients:        []PublishingClient{{ID: "client-a", Name: "Cliente A"}},
	}}
	checker := &stubTargetPermissionChecker{
		allowedByAccount: map[string]bool{"client-a": true},
	}
	service := NewService(
		repository,
		&stubProvider{},
		nil,
		nil,
		WithServicePermissionChecker(checker),
	)

	_, err := service.Portfolio(context.Background(), auth.Principal{
		UserID:    "user-1",
		AccountID: "client-a",
		Role:      auth.RoleMarketing,
	})

	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("Portfolio() error = %v, want ErrForbidden", err)
	}
	if repository.portfolioCalls != 0 {
		t.Fatalf("portfolio repository calls = %d, want zero", repository.portfolioCalls)
	}
	if len(checker.checkedPermissions) != 1 ||
		checker.checkedPermissions[0] != PermissionAnalytics {
		t.Fatalf(
			"portfolio target permissions = %#v, want %q",
			checker.checkedPermissions,
			PermissionAnalytics,
		)
	}
}

func TestListAnalyticsNormalizesDeduplicatesAndCapsFilter(t *testing.T) {
	repository := &stubRepository{}
	service := NewService(repository, &stubProvider{}, nil, nil)
	const first = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	const second = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"

	_, err := service.ListAnalytics(
		context.Background(),
		"account-1",
		ListAnalyticsFilter{
			PostIDs: []string{strings.ToUpper(first), second, first},
			Limit:   250,
		},
	)

	if err != nil {
		t.Fatalf("ListAnalytics() error = %v", err)
	}
	got := repository.analyticsFilter
	if got.Limit != 100 {
		t.Fatalf("limit = %d, want 100", got.Limit)
	}
	if len(got.PostIDs) != 2 || got.PostIDs[0] != first || got.PostIDs[1] != second {
		t.Fatalf("post IDs = %#v, want normalized and deduplicated", got.PostIDs)
	}
}

func TestListAnalyticsRejectsInvalidOrMoreThanOneHundredPostIDs(t *testing.T) {
	service := NewService(&stubRepository{}, &stubProvider{}, nil, nil)

	_, err := service.ListAnalytics(
		context.Background(),
		"account-1",
		ListAnalyticsFilter{PostIDs: []string{"not-a-uuid"}},
	)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid UUID error = %v, want ErrInvalidInput", err)
	}

	postIDs := make([]string, 0, 101)
	for index := 0; index < 101; index++ {
		postIDs = append(
			postIDs,
			fmt.Sprintf("00000000-0000-4000-8000-%012x", index),
		)
	}
	_, err = service.ListAnalytics(
		context.Background(),
		"account-1",
		ListAnalyticsFilter{PostIDs: postIDs},
	)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("101 UUIDs error = %v, want ErrInvalidInput", err)
	}
}

func TestSchedulePostPreservesHistoricalInstagramTarget(t *testing.T) {
	now := time.Date(2026, 7, 23, 15, 0, 0, 0, time.UTC)
	repository := &stubRepository{
		post: Post{
			ID:           "post-1",
			Status:       PostStatusDraft,
			Version:      2,
			ConnectionID: "historical-connection",
		},
		connection: ConnectionRecord{
			Connection: Connection{
				ID:       "active-credential",
				IGUserID: "ig-original",
				Status:   "connected",
			},
			AccessTokenCiphertext: "ciphertext",
		},
		connectionTarget: connectionTarget{
			ID:       "historical-connection",
			IGUserID: "ig-original",
		},
	}
	service := NewService(
		repository,
		&stubProvider{},
		nil,
		nil,
		WithServiceClock(func() time.Time { return now }),
	)

	_, err := service.SchedulePost(
		context.Background(),
		"account-1",
		"user-1",
		"post-1",
		SchedulePostInput{
			ScheduledFor: now.Add(time.Hour),
			Timezone:     "UTC",
			Version:      2,
		},
	)

	if err != nil {
		t.Fatalf("SchedulePost() error = %v", err)
	}
	if repository.scheduleCommand.ConnectionID != "historical-connection" {
		t.Fatalf(
			"connection ID = %q, want historical target",
			repository.scheduleCommand.ConnectionID,
		)
	}
}

func TestSchedulePostRejectsDifferentActiveInstagramTarget(t *testing.T) {
	now := time.Date(2026, 7, 23, 15, 0, 0, 0, time.UTC)
	repository := &stubRepository{
		post: Post{
			ID:           "post-1",
			Status:       PostStatusDraft,
			Version:      2,
			ConnectionID: "historical-connection",
		},
		connection: ConnectionRecord{
			Connection: Connection{
				ID:       "active-credential",
				IGUserID: "ig-different",
				Status:   "connected",
			},
			AccessTokenCiphertext: "ciphertext",
		},
		connectionTarget: connectionTarget{
			ID:       "historical-connection",
			IGUserID: "ig-original",
		},
	}
	service := NewService(
		repository,
		&stubProvider{},
		nil,
		nil,
		WithServiceClock(func() time.Time { return now }),
	)

	_, err := service.SchedulePost(
		context.Background(),
		"account-1",
		"user-1",
		"post-1",
		SchedulePostInput{
			ScheduledFor: now.Add(time.Hour),
			Timezone:     "UTC",
			Version:      2,
		},
	)

	if !errors.Is(err, ErrConnectionTarget) {
		t.Fatalf("SchedulePost() error = %v, want ErrConnectionTarget", err)
	}
	if repository.scheduleCommand.PostID != "" {
		t.Fatal("repository SchedulePost must not run after target mismatch")
	}
}

func TestAttemptedPublishCannotStartNewRevision(t *testing.T) {
	attemptedAt := time.Date(2026, 7, 23, 14, 0, 0, 0, time.UTC)
	repository := &stubRepository{post: Post{
		ID:                 "post-1",
		Status:             PostStatusFailed,
		Version:            4,
		ConnectionID:       "historical-connection",
		PublishAttemptedAt: &attemptedAt,
		LastErrorCode:      "module_disabled",
	}}
	service := NewService(repository, &stubProvider{}, nil, nil)

	_, err := service.SchedulePost(
		context.Background(),
		"account-1",
		"user-1",
		"post-1",
		SchedulePostInput{
			ScheduledFor: attemptedAt.Add(24 * time.Hour),
			Timezone:     "UTC",
			Version:      4,
		},
	)

	if !errors.Is(err, ErrInvalidState) {
		t.Fatalf("SchedulePost() error = %v, want ErrInvalidState", err)
	}
}

func TestQueueAnalyticsSyncEnqueuesEveryPublishedPost(t *testing.T) {
	postIDs := make([]string, 0, 137)
	for index := 0; index < 137; index++ {
		postIDs = append(postIDs, fmt.Sprintf("post-%d", index))
	}
	repository := &stubRepository{publishedPostIDs: postIDs}
	enqueuer := &recordingJobEnqueuer{}
	service := NewService(repository, &stubProvider{}, nil, enqueuer)

	queued, err := service.QueueAnalyticsSync(context.Background(), "account-1")

	if err != nil {
		t.Fatalf("QueueAnalyticsSync() error = %v", err)
	}
	if queued != len(postIDs) || len(enqueuer.enqueued) != len(postIDs) {
		t.Fatalf(
			"queued = %d, enqueued = %d, want %d",
			queued,
			len(enqueuer.enqueued),
			len(postIDs),
		)
	}
}

type recordingJobEnqueuer struct {
	enqueued []jobs.NewJob
}

func (e *recordingJobEnqueuer) Enqueue(
	_ context.Context,
	job jobs.NewJob,
) (string, bool, error) {
	e.enqueued = append(e.enqueued, job)
	return fmt.Sprintf("job-%d", len(e.enqueued)), true, nil
}
