package socialpublishing

import (
	"context"
	"errors"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

type stubRepository struct {
	connection       ConnectionRecord
	connectionErr    error
	savedCiphertext  string
	savedLast4       string
	createCommand    createPostCommand
	updateCommand    updatePostCommand
	scheduleCommand  schedulePostCommand
	post             Post
	postErr          error
	publishedPostIDs []string
}

func (s *stubRepository) GetConnection(context.Context, string) (ConnectionRecord, error) {
	return s.connection, s.connectionErr
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
	context.Context,
	string,
	ListPostsFilter,
) ([]Post, error) {
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
	int,
) ([]string, error) {
	return append([]string(nil), s.publishedPostIDs...), nil
}

func (s *stubRepository) Overview(context.Context, string, time.Time) (Overview, error) {
	return Overview{Counts: map[string]int64{}, Upcoming: []Post{}}, nil
}

func (s *stubRepository) ListAnalytics(context.Context, string, int) ([]Analytics, error) {
	return []Analytics{}, nil
}

func (s *stubRepository) RuntimeContext(
	context.Context,
	string,
	time.Time,
) (RuntimeContext, error) {
	return RuntimeContext{}, nil
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

type stubEnqueuer struct {
	jobs []jobs.NewJob
	err  error
}

func (s *stubEnqueuer) Enqueue(
	_ context.Context,
	job jobs.NewJob,
) (string, bool, error) {
	if s.err != nil {
		return "", false, s.err
	}
	s.jobs = append(s.jobs, job)
	return "job", true, nil
}

type stubModules struct {
	enabled bool
	err     error
}

func (s stubModules) IsEnabled(context.Context, string, string) (bool, error) {
	return s.enabled, s.err
}

var errStub = errors.New("stub error")
