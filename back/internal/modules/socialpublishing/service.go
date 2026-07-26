package socialpublishing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

type Service struct {
	repo          serviceRepository
	provider      InstagramProvider
	secrets       *secretbox.Box
	analyticsJobs jobEnqueuer
	permissions   permissionChecker
	now           func() time.Time
}

type ServiceOption func(*Service)

func WithServiceClock(now func() time.Time) ServiceOption {
	return func(service *Service) {
		if now != nil {
			service.now = now
		}
	}
}

func WithServicePermissionChecker(checker permissionChecker) ServiceOption {
	return func(service *Service) {
		service.permissions = checker
	}
}

func NewService(
	repo serviceRepository,
	provider InstagramProvider,
	secrets *secretbox.Box,
	analyticsJobs jobEnqueuer,
	options ...ServiceOption,
) *Service {
	service := &Service{
		repo:          repo,
		provider:      provider,
		secrets:       secrets,
		analyticsJobs: analyticsJobs,
		now:           time.Now,
	}
	for _, option := range options {
		if option != nil {
			option(service)
		}
	}
	return service
}

func (s *Service) Connection(ctx context.Context, accountID string) (Connection, error) {
	if strings.TrimSpace(accountID) == "" {
		return Connection{}, ErrForbidden
	}
	record, err := s.repo.GetConnection(ctx, accountID)
	if errors.Is(err, ErrNotConnected) {
		return Connection{
			Provider: "instagram",
			Status:   "disconnected",
			Secret:   secretbox.Status{Set: false},
		}, nil
	}
	if err != nil {
		return Connection{}, err
	}
	return record.Connection, nil
}

func (s *Service) Connect(
	ctx context.Context,
	accountID, userID, accessToken string,
) (Connection, error) {
	if strings.TrimSpace(accountID) == "" || strings.TrimSpace(userID) == "" {
		return Connection{}, ErrForbidden
	}
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return Connection{}, ErrInvalidToken
	}
	if s.secrets == nil {
		return Connection{}, ErrSecretsUnavailable
	}
	profile, err := s.provider.ValidateToken(ctx, accessToken)
	if err != nil {
		return Connection{}, err
	}
	ciphertext, err := s.secrets.Encrypt(accessToken)
	if err != nil {
		return Connection{}, fmt.Errorf("social publishing: cifrar token: %w", err)
	}
	masked := secretbox.Mask(accessToken)
	return s.repo.SaveConnection(
		ctx,
		accountID,
		userID,
		profile,
		ciphertext,
		masked.Last4,
	)
}

func (s *Service) Disconnect(ctx context.Context, accountID string) error {
	if strings.TrimSpace(accountID) == "" {
		return ErrForbidden
	}
	return s.repo.DeleteConnection(ctx, accountID)
}

func (s *Service) CreatePost(
	ctx context.Context,
	accountID, userID string,
	input CreatePostInput,
) (CreatePostResult, error) {
	if strings.TrimSpace(accountID) == "" || strings.TrimSpace(userID) == "" {
		return CreatePostResult{}, ErrForbidden
	}
	normalized, err := s.normalizeCreate(input)
	if err != nil {
		return CreatePostResult{}, err
	}
	connectionID := ""
	if normalized.Status == PostStatusScheduled {
		connection, err := s.activeConnection(ctx, accountID)
		if err != nil {
			return CreatePostResult{}, err
		}
		connectionID = connection.ID
	}
	return s.repo.CreatePost(ctx, createPostCommand{
		AccountID:    accountID,
		UserID:       userID,
		ConnectionID: connectionID,
		Input:        normalized,
	})
}

func (s *Service) ListPosts(
	ctx context.Context,
	accountID string,
	filter ListPostsFilter,
) ([]Post, error) {
	if strings.TrimSpace(accountID) == "" {
		return nil, ErrForbidden
	}
	statuses, err := normalizePostStatuses(filter)
	if err != nil {
		return nil, err
	}
	order, err := normalizePostListOrder(filter.Order)
	if err != nil {
		return nil, err
	}
	filter.Statuses = statuses
	filter.Status = ""
	filter.Order = order
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	return s.repo.ListPosts(ctx, accountID, filter)
}

func (s *Service) Post(ctx context.Context, accountID, postID string) (Post, error) {
	if strings.TrimSpace(accountID) == "" || strings.TrimSpace(postID) == "" {
		return Post{}, ErrNotFound
	}
	return s.repo.GetPost(ctx, accountID, postID)
}

func (s *Service) PatchPost(
	ctx context.Context,
	accountID, userID, postID string,
	input PatchPostInput,
) (Post, error) {
	if input.Version <= 0 {
		return Post{}, ErrConflict
	}
	post, err := s.Post(ctx, accountID, postID)
	if err != nil {
		return Post{}, err
	}
	if post.Version != input.Version {
		return Post{}, ErrConflict
	}
	if post.Status == PostStatusPublishing || post.Status == PostStatusPublished {
		return Post{}, ErrInvalidState
	}
	if ambiguousPublishOutcome(post) {
		return Post{}, ErrInvalidState
	}
	if input.Caption != nil {
		post.Caption = strings.TrimSpace(*input.Caption)
	}
	if input.MediaURL != nil {
		post.MediaURL = strings.TrimSpace(*input.MediaURL)
	}
	if input.AltText != nil {
		post.AltText = strings.TrimSpace(*input.AltText)
	}
	if input.Timezone != nil {
		post.Timezone = strings.TrimSpace(*input.Timezone)
	}
	if err := validateContent(post.Caption, post.MediaURL, post.AltText); err != nil {
		return Post{}, err
	}
	if post.Timezone != "" {
		if err := validateTimezone(post.Timezone); err != nil {
			return Post{}, err
		}
	}
	return s.repo.UpdatePost(ctx, updatePostCommand{
		AccountID: accountID,
		UserID:    userID,
		Post:      post,
		Version:   input.Version,
	})
}

func (s *Service) SchedulePost(
	ctx context.Context,
	accountID, userID, postID string,
	input SchedulePostInput,
) (Post, error) {
	if input.Version <= 0 {
		return Post{}, ErrConflict
	}
	post, err := s.Post(ctx, accountID, postID)
	if err != nil {
		return Post{}, err
	}
	if post.Version != input.Version {
		return Post{}, ErrConflict
	}
	if post.Status != PostStatusDraft && post.Status != PostStatusScheduled &&
		post.Status != PostStatusFailed && post.Status != PostStatusCancelled {
		return Post{}, ErrInvalidState
	}
	if ambiguousPublishOutcome(post) {
		return Post{}, ErrInvalidState
	}
	when, timezone, err := s.validateSchedule(input.ScheduledFor, input.Timezone)
	if err != nil {
		return Post{}, err
	}
	connection, err := s.activeConnection(ctx, accountID)
	if err != nil {
		return Post{}, err
	}
	connectionID, err := s.scheduleConnectionID(ctx, accountID, post, connection)
	if err != nil {
		return Post{}, err
	}
	return s.repo.SchedulePost(ctx, schedulePostCommand{
		AccountID:       accountID,
		UserID:          userID,
		PostID:          postID,
		ConnectionID:    connectionID,
		ScheduledFor:    when,
		Timezone:        timezone,
		ExpectedVersion: input.Version,
	})
}

func (s *Service) CancelPost(
	ctx context.Context,
	accountID, userID, postID string,
	input VersionInput,
) (Post, error) {
	if input.Version <= 0 {
		return Post{}, ErrConflict
	}
	post, err := s.Post(ctx, accountID, postID)
	if err != nil {
		return Post{}, err
	}
	if post.Version != input.Version {
		return Post{}, ErrConflict
	}
	if post.Status != PostStatusScheduled && post.Status != PostStatusFailed {
		return Post{}, ErrInvalidState
	}
	return s.repo.CancelPost(ctx, accountID, postID, userID, input.Version)
}

func (s *Service) RetryPost(
	ctx context.Context,
	accountID, userID, postID string,
	input VersionInput,
) (Post, error) {
	if input.Version <= 0 {
		return Post{}, ErrConflict
	}
	post, err := s.Post(ctx, accountID, postID)
	if err != nil {
		return Post{}, err
	}
	if post.Version != input.Version {
		return Post{}, ErrConflict
	}
	if post.Status != PostStatusFailed {
		return Post{}, ErrInvalidState
	}
	if ambiguousPublishOutcome(post) {
		return Post{}, ErrInvalidState
	}
	timezone := post.Timezone
	if timezone == "" {
		timezone = "UTC"
	}
	return s.SchedulePost(ctx, accountID, userID, postID, SchedulePostInput{
		ScheduledFor: s.now().UTC().Add(time.Second),
		Timezone:     timezone,
		Version:      input.Version,
	})
}

func (s *Service) Overview(ctx context.Context, accountID string) (Overview, error) {
	if strings.TrimSpace(accountID) == "" {
		return Overview{}, ErrForbidden
	}
	return s.repo.Overview(ctx, accountID, s.now().UTC())
}

func (s *Service) Summary(ctx context.Context, accountID string) (Summary, error) {
	if strings.TrimSpace(accountID) == "" {
		return Summary{}, ErrForbidden
	}
	return s.repo.Summary(ctx, accountID, s.now().UTC())
}

func (s *Service) PublishingScope(
	ctx context.Context,
	principal auth.Principal,
) (PublishingScope, error) {
	return s.publishingScopeForPermission(ctx, principal, PermissionView)
}

func (s *Service) publishingScopeForPermission(
	ctx context.Context,
	principal auth.Principal,
	permission string,
) (PublishingScope, error) {
	accountID := strings.TrimSpace(principal.AccountID)
	if accountID == "" || strings.TrimSpace(principal.UserID) == "" {
		return PublishingScope{}, ErrForbidden
	}
	scope, err := s.repo.PublishingScope(
		ctx,
		accountID,
		strings.TrimSpace(principal.UserID),
		principal.Role == auth.RolePlatformAdmin,
	)
	if err != nil {
		return PublishingScope{}, err
	}
	if principal.Role == auth.RolePlatformAdmin || principal.Role == auth.RoleOwner {
		return scope, nil
	}
	if s.permissions == nil {
		return PublishingScope{}, ErrForbidden
	}
	clients := make([]PublishingClient, 0, len(scope.Clients))
	for _, client := range scope.Clients {
		allowed, checkErr := s.permissions.HasAccountPermission(
			ctx,
			client.ID,
			principal.UserID,
			permission,
		)
		if checkErr != nil {
			return PublishingScope{}, checkErr
		}
		if allowed {
			clients = append(clients, client)
		}
	}
	scope.Clients = clients
	if !scope.CanSelect &&
		(scope.LockedClientID == "" ||
			len(scope.Clients) != 1 ||
			scope.Clients[0].ID != scope.LockedClientID) {
		return PublishingScope{}, ErrForbidden
	}
	return scope, nil
}

func (s *Service) Portfolio(
	ctx context.Context,
	principal auth.Principal,
) (Portfolio, error) {
	scope, err := s.publishingScopeForPermission(ctx, principal, PermissionAnalytics)
	if err != nil {
		return Portfolio{}, err
	}
	if !scope.CanSelect {
		return Portfolio{}, ErrForbidden
	}

	accountIDs := make([]string, 0, len(scope.Clients))
	seen := make(map[string]struct{}, len(scope.Clients))
	for _, client := range scope.Clients {
		accountID := strings.TrimSpace(client.ID)
		if accountID == "" {
			continue
		}
		if _, exists := seen[accountID]; exists {
			continue
		}
		seen[accountID] = struct{}{}
		accountIDs = append(accountIDs, accountID)
	}
	records, err := s.repo.ListPortfolio(ctx, accountIDs, s.now().UTC())
	if err != nil {
		return Portfolio{}, err
	}

	portfolio := Portfolio{Clients: make([]PortfolioClient, 0, len(records))}
	for _, record := range records {
		portfolio.Clients = append(portfolio.Clients, record.Client)
		if record.Client.Connected {
			portfolio.ConnectedClients++
		}
		portfolio.Draft += record.Client.Draft
		portfolio.Scheduled += record.Client.Scheduled
		portfolio.Publishing += record.Client.Publishing
		portfolio.Published += record.Client.Published
		portfolio.Failed += record.Client.Failed
		portfolio.Views += record.Views
		portfolio.Reach += record.Client.Reach
		portfolio.TotalInteractions += record.Client.TotalInteractions
		portfolio.Likes += record.Likes
		portfolio.Comments += record.Comments
		portfolio.Saved += record.Saved
		portfolio.Shares += record.Shares
		if record.CapturedAt != nil &&
			(portfolio.CapturedAt == nil || record.CapturedAt.After(*portfolio.CapturedAt)) {
			capturedAt := record.CapturedAt.UTC()
			portfolio.CapturedAt = &capturedAt
		}
	}
	portfolio.ClientCount = len(portfolio.Clients)
	return portfolio, nil
}

func (s *Service) ListAnalytics(
	ctx context.Context,
	accountID string,
	filter ListAnalyticsFilter,
) ([]Analytics, error) {
	if strings.TrimSpace(accountID) == "" {
		return nil, ErrForbidden
	}
	postIDs, err := normalizeAnalyticsPostIDs(filter.PostIDs)
	if err != nil {
		return nil, err
	}
	filter.PostIDs = postIDs
	if filter.Limit <= 0 {
		filter.Limit = 100
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	return s.repo.ListAnalytics(ctx, accountID, filter)
}

func (s *Service) QueueAnalyticsSync(ctx context.Context, accountID string) (int, error) {
	if strings.TrimSpace(accountID) == "" {
		return 0, ErrForbidden
	}
	if s.analyticsJobs == nil {
		return 0, ErrProviderUnavailable
	}
	postIDs, err := s.repo.ListPublishedPostIDs(ctx, accountID)
	if err != nil {
		return 0, err
	}
	bucket := s.now().UTC().Truncate(15 * time.Minute).Format("20060102T1504")
	created := 0
	for _, postID := range postIDs {
		payload, marshalErr := marshalJobPayload(analyticsJobPayload{PostID: postID})
		if marshalErr != nil {
			return created, marshalErr
		}
		_, wasCreated, enqueueErr := s.analyticsJobs.Enqueue(ctx, jobs.NewJob{
			AccountID:      accountID,
			OrderingKey:    "analytics:" + postID + ":manual",
			IdempotencyKey: "analytics:" + postID + ":" + bucket,
			Kind:           AnalyticsJobKind,
			Payload:        payload,
			MaxAttempts:    4,
		})
		if enqueueErr != nil {
			return created, enqueueErr
		}
		if wasCreated {
			created++
		}
	}
	return created, nil
}

func (s *Service) RuntimeContext(ctx context.Context, accountID string) (RuntimeContext, error) {
	if strings.TrimSpace(accountID) == "" {
		return RuntimeContext{}, ErrForbidden
	}
	return s.repo.RuntimeContext(ctx, accountID, s.now().UTC())
}

func (s *Service) activeConnection(ctx context.Context, accountID string) (ConnectionRecord, error) {
	connection, err := s.repo.GetConnection(ctx, accountID)
	if err != nil {
		return ConnectionRecord{}, err
	}
	if connection.Status != "connected" || strings.TrimSpace(connection.AccessTokenCiphertext) == "" {
		return ConnectionRecord{}, ErrNotConnected
	}
	return connection, nil
}

func (s *Service) scheduleConnectionID(
	ctx context.Context,
	accountID string,
	post Post,
	active ConnectionRecord,
) (string, error) {
	if post.ConnectionID == "" {
		return active.ID, nil
	}
	target, err := s.repo.GetConnectionTarget(ctx, accountID, post.ConnectionID)
	if errors.Is(err, ErrNotFound) {
		return "", ErrConnectionTarget
	}
	if err != nil {
		return "", err
	}
	if target.IGUserID == "" || target.IGUserID != active.IGUserID {
		return "", ErrConnectionTarget
	}
	return target.ID, nil
}

func ambiguousPublishOutcome(post Post) bool {
	return post.LastErrorCode == "publish_outcome_unknown" ||
		(post.PublishAttemptedAt != nil && post.ExternalMediaID == "")
}
