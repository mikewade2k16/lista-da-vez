package socialpublishing

import (
	"context"
	"errors"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

type stubRepository struct {
	connection          ConnectionRecord
	connectionErr       error
	connectionTarget    connectionTarget
	targetErr           error
	savedCiphertext     string
	savedLast4          string
	createCommand       createPostCommand
	updateCommand       updatePostCommand
	scheduleCommand     schedulePostCommand
	listFilter          ListPostsFilter
	analyticsFilter     ListAnalyticsFilter
	summary             Summary
	summaryAccountID    string
	summaryNow          time.Time
	post                Post
	postErr             error
	publishedPostIDs    []string
	scope               PublishingScope
	scopeErr            error
	scopeAccountID      string
	scopeUserID         string
	scopePlatformAdmin  bool
	portfolioRecords    []portfolioClientRecord
	portfolioErr        error
	portfolioAccountIDs []string
	portfolioNow        time.Time
	portfolioCalls      int
}

func (s *stubRepository) GetConnection(context.Context, string) (ConnectionRecord, error) {
	return s.connection, s.connectionErr
}

func (s *stubRepository) GetConnectionTarget(
	context.Context,
	string,
	string,
) (connectionTarget, error) {
	return s.connectionTarget, s.targetErr
}

func (s *stubRepository) SaveConnection(
	_ context.Context,
	_, _ string,
	profile InstagramProfile,
	ciphertext, last4 string,
) (Connection, error) {
	s.savedCiphertext = ciphertext
	s.savedLast4 = last4
	return Connection{
		ID:          "connection-1",
		Provider:    "instagram",
		IGUserID:    profile.UserID,
		Username:    profile.Username,
		AccountType: profile.AccountType,
		Status:      "connected",
		Secret:      secretbox.Status{Set: true, Last4: last4},
	}, nil
}

func (s *stubRepository) DeleteConnection(context.Context, string) error {
	return nil
}

func (s *stubRepository) CreatePost(
	_ context.Context,
	command createPostCommand,
) (CreatePostResult, error) {
	s.createCommand = command
	post := Post{
		ID:               "post-1",
		AccountID:        command.AccountID,
		ConnectionID:     command.ConnectionID,
		Caption:          command.Input.Caption,
		MediaURL:         command.Input.MediaURL,
		MediaType:        "image",
		AltText:          command.Input.AltText,
		Status:           command.Input.Status,
		ScheduledFor:     command.Input.ScheduledFor,
		Timezone:         command.Input.Timezone,
		ScheduleRevision: 1,
		Version:          1,
	}
	return CreatePostResult{Post: post, Created: true}, nil
}

func (s *stubRepository) ListPosts(
	_ context.Context,
	_ string,
	filter ListPostsFilter,
) ([]Post, error) {
	s.listFilter = filter
	if s.post.ID == "" {
		return []Post{}, nil
	}
	return []Post{s.post}, nil
}

func (s *stubRepository) GetPost(context.Context, string, string) (Post, error) {
	return s.post, s.postErr
}

func (s *stubRepository) UpdatePost(
	_ context.Context,
	command updatePostCommand,
) (Post, error) {
	s.updateCommand = command
	post := command.Post
	post.Status = PostStatusDraft
	post.ScheduledFor = nil
	post.Version++
	return post, nil
}

func (s *stubRepository) SchedulePost(
	_ context.Context,
	command schedulePostCommand,
) (Post, error) {
	s.scheduleCommand = command
	post := s.post
	post.Status = PostStatusScheduled
	post.ScheduledFor = &command.ScheduledFor
	post.Timezone = command.Timezone
	post.ScheduleRevision++
	post.Version++
	return post, nil
}

func (s *stubRepository) CancelPost(
	context.Context,
	string,
	string,
	string,
	int,
) (Post, error) {
	post := s.post
	post.Status = PostStatusCancelled
	return post, nil
}

func (s *stubRepository) ListPublishedPostIDs(
	context.Context,
	string,
) ([]string, error) {
	return append([]string(nil), s.publishedPostIDs...), nil
}

func (s *stubRepository) Overview(context.Context, string, time.Time) (Overview, error) {
	return Overview{}, nil
}

func (s *stubRepository) Summary(
	_ context.Context,
	accountID string,
	now time.Time,
) (Summary, error) {
	s.summaryAccountID = accountID
	s.summaryNow = now
	return s.summary, nil
}

func (s *stubRepository) ListAnalytics(
	_ context.Context,
	_ string,
	filter ListAnalyticsFilter,
) ([]Analytics, error) {
	s.analyticsFilter = filter
	return []Analytics{}, nil
}

func (s *stubRepository) RuntimeContext(
	context.Context,
	string,
	time.Time,
) (RuntimeContext, error) {
	return RuntimeContext{}, nil
}

func (s *stubRepository) PublishingScope(
	_ context.Context,
	accountID, userID string,
	platformAdmin bool,
) (PublishingScope, error) {
	s.scopeAccountID = accountID
	s.scopeUserID = userID
	s.scopePlatformAdmin = platformAdmin
	return s.scope, s.scopeErr
}

func (s *stubRepository) ListPortfolio(
	_ context.Context,
	accountIDs []string,
	now time.Time,
) ([]portfolioClientRecord, error) {
	s.portfolioCalls++
	s.portfolioAccountIDs = append([]string(nil), accountIDs...)
	s.portfolioNow = now
	return append([]portfolioClientRecord(nil), s.portfolioRecords...), s.portfolioErr
}

func (s *stubRepository) AnalyticsTarget(
	context.Context,
	string,
	string,
) (analyticsTarget, error) {
	return analyticsTarget{}, ErrNotFound
}

func (s *stubRepository) SaveAnalytics(
	context.Context,
	string,
	string,
	string,
	Analytics,
) error {
	return nil
}

type stubProvider struct {
	profile       InstagramProfile
	validateErr   error
	validatedWith string
	createCalls   int
	publishCalls  int
	publishErr    error
}

func (s *stubProvider) ValidateToken(
	_ context.Context,
	token string,
) (InstagramProfile, error) {
	s.validatedWith = token
	return s.profile, s.validateErr
}

func (s *stubProvider) CreateImageContainer(
	context.Context,
	string,
	string,
	string,
	string,
	string,
) (string, error) {
	s.createCalls++
	return "creation-1", nil
}

func (s *stubProvider) PublishContainer(
	context.Context,
	string,
	string,
	string,
) (string, error) {
	s.publishCalls++
	if s.publishErr != nil {
		return "", s.publishErr
	}
	return "media-1", nil
}

func (s *stubProvider) FetchPermalink(context.Context, string, string) (string, error) {
	return "https://www.instagram.com/p/example/", nil
}

func (s *stubProvider) FetchMediaInsights(
	context.Context,
	string,
	string,
) (Analytics, error) {
	return Analytics{}, nil
}

type stubModules struct {
	enabled bool
	err     error
}

func (s stubModules) IsEnabled(context.Context, string, string) (bool, error) {
	return s.enabled, s.err
}

type stubTargetPermissionChecker struct {
	allowedByAccount   map[string]bool
	checkedAccounts    []string
	checkedPermissions []string
	err                error
}

func (s *stubTargetPermissionChecker) HasAccountPermission(
	_ context.Context,
	accountID, _ string,
	permission string,
) (bool, error) {
	s.checkedAccounts = append(s.checkedAccounts, accountID)
	s.checkedPermissions = append(s.checkedPermissions, permission)
	if s.err != nil {
		return false, s.err
	}
	return s.allowedByAccount[accountID], nil
}

var errStub = errors.New("stub error")
