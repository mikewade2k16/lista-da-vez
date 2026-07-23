package socialpublishing

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

const (
	maxCaptionRunes   = 2200
	maxAltTextRunes   = 1000
	maxMediaURLBytes  = 4096
	maxSourceRefBytes = 256
)

type Service struct {
	repo     serviceRepository
	provider InstagramProvider
	secrets  *secretbox.Box
	enqueuer jobEnqueuer
	now      func() time.Time
}

type ServiceOption func(*Service)

func WithServiceClock(now func() time.Time) ServiceOption {
	return func(service *Service) {
		if now != nil {
			service.now = now
		}
	}
}

func NewService(
	repo serviceRepository,
	provider InstagramProvider,
	secrets *secretbox.Box,
	enqueuer jobEnqueuer,
	options ...ServiceOption,
) *Service {
	service := &Service{
		repo:     repo,
		provider: provider,
		secrets:  secrets,
		enqueuer: enqueuer,
		now:      time.Now,
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
	if filter.Status != "" && !validPostStatus(filter.Status) {
		return nil, ErrInvalidInput
	}
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
	if post.LastErrorCode == "publish_outcome_unknown" {
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
	if post.LastErrorCode == "publish_outcome_unknown" {
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
	return s.repo.SchedulePost(ctx, schedulePostCommand{
		AccountID:       accountID,
		UserID:          userID,
		PostID:          postID,
		ConnectionID:    connection.ID,
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
	if post.LastErrorCode == "publish_outcome_unknown" {
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

func (s *Service) ListAnalytics(
	ctx context.Context,
	accountID string,
	limit int,
) ([]Analytics, error) {
	if strings.TrimSpace(accountID) == "" {
		return nil, ErrForbidden
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 100 {
		limit = 100
	}
	return s.repo.ListAnalytics(ctx, accountID, limit)
}

func (s *Service) QueueAnalyticsSync(ctx context.Context, accountID string) (int, error) {
	if strings.TrimSpace(accountID) == "" {
		return 0, ErrForbidden
	}
	if s.enqueuer == nil {
		return 0, ErrProviderUnavailable
	}
	postIDs, err := s.repo.ListPublishedPostIDs(ctx, accountID, 100)
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
		_, wasCreated, enqueueErr := s.enqueuer.Enqueue(ctx, jobs.NewJob{
			AccountID:      accountID,
			OrderingKey:    "analytics:" + postID,
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

func (s *Service) normalizeCreate(input CreatePostInput) (CreatePostInput, error) {
	input.Caption = strings.TrimSpace(input.Caption)
	input.MediaURL = strings.TrimSpace(input.MediaURL)
	input.MediaType = strings.ToLower(strings.TrimSpace(input.MediaType))
	input.AltText = strings.TrimSpace(input.AltText)
	input.Timezone = strings.TrimSpace(input.Timezone)
	input.SourceType = strings.TrimSpace(input.SourceType)
	input.SourceRef = strings.TrimSpace(input.SourceRef)
	if input.Status == "" {
		input.Status = PostStatusDraft
	}
	if input.Status != PostStatusDraft && input.Status != PostStatusScheduled {
		return CreatePostInput{}, ErrInvalidState
	}
	if input.MediaType == "" {
		input.MediaType = "image"
	}
	if input.MediaType != "image" {
		return CreatePostInput{}, ErrInvalidInput
	}
	if err := validateContent(input.Caption, input.MediaURL, input.AltText); err != nil {
		return CreatePostInput{}, err
	}
	if input.SourceType == "" {
		input.SourceType = "manual"
	}
	if !validSourceType(input.SourceType) || len(input.SourceRef) > maxSourceRefBytes {
		return CreatePostInput{}, ErrInvalidInput
	}
	if input.Status == PostStatusDraft {
		input.ScheduledFor = nil
		if input.Timezone != "" {
			if err := validateTimezone(input.Timezone); err != nil {
				return CreatePostInput{}, err
			}
		}
		return input, nil
	}
	if input.ScheduledFor == nil {
		return CreatePostInput{}, ErrInvalidInput
	}
	when, timezone, err := s.validateSchedule(*input.ScheduledFor, input.Timezone)
	if err != nil {
		return CreatePostInput{}, err
	}
	input.ScheduledFor = &when
	input.Timezone = timezone
	return input, nil
}

func (s *Service) validateSchedule(
	when time.Time,
	timezone string,
) (time.Time, string, error) {
	timezone = strings.TrimSpace(timezone)
	if err := validateTimezone(timezone); err != nil {
		return time.Time{}, "", err
	}
	if !when.After(s.now().UTC()) {
		return time.Time{}, "", ErrScheduleInPast
	}
	return when.UTC(), timezone, nil
}

func validateContent(caption, mediaURL, altText string) error {
	if utf8.RuneCountInString(caption) > maxCaptionRunes ||
		utf8.RuneCountInString(altText) > maxAltTextRunes {
		return ErrInvalidInput
	}
	if len(mediaURL) == 0 || len(mediaURL) > maxMediaURLBytes {
		return ErrInvalidMediaURL
	}
	parsed, err := url.Parse(mediaURL)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" ||
		parsed.User != nil || parsed.Fragment != "" {
		return ErrInvalidMediaURL
	}
	return nil
}

func validateTimezone(value string) error {
	if strings.TrimSpace(value) == "" {
		return ErrInvalidTimezone
	}
	if _, err := time.LoadLocation(value); err != nil {
		return ErrInvalidTimezone
	}
	return nil
}

func validSourceType(value string) bool {
	switch value {
	case "manual", "calendar", "crow_assistant":
		return true
	default:
		return false
	}
}

func validPostStatus(value PostStatus) bool {
	switch value {
	case PostStatusDraft, PostStatusScheduled, PostStatusPublishing,
		PostStatusPublished, PostStatusFailed, PostStatusCancelled:
		return true
	default:
		return false
	}
}
