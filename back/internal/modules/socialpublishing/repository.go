package socialpublishing

import (
	"context"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
)

type connectionRepository interface {
	GetConnection(ctx context.Context, accountID string) (ConnectionRecord, error)
	GetConnectionTarget(ctx context.Context, accountID, connectionID string) (connectionTarget, error)
	SaveConnection(
		ctx context.Context,
		accountID, userID string,
		profile InstagramProfile,
		ciphertext, tokenLast4 string,
	) (Connection, error)
	DeleteConnection(ctx context.Context, accountID string) error
}

type postRepository interface {
	CreatePost(ctx context.Context, command createPostCommand) (CreatePostResult, error)
	ListPosts(ctx context.Context, accountID string, filter ListPostsFilter) ([]Post, error)
	GetPost(ctx context.Context, accountID, postID string) (Post, error)
	UpdatePost(ctx context.Context, command updatePostCommand) (Post, error)
	SchedulePost(ctx context.Context, command schedulePostCommand) (Post, error)
	CancelPost(ctx context.Context, accountID, postID, userID string, version int) (Post, error)
	ListPublishedPostIDs(ctx context.Context, accountID string) ([]string, error)
}

type analyticsRepository interface {
	Overview(ctx context.Context, accountID string, now time.Time) (Overview, error)
	Summary(ctx context.Context, accountID string, now time.Time) (Summary, error)
	ListAnalytics(ctx context.Context, accountID string, filter ListAnalyticsFilter) ([]Analytics, error)
	RuntimeContext(ctx context.Context, accountID string, now time.Time) (RuntimeContext, error)
}

type portfolioRepository interface {
	PublishingScope(
		ctx context.Context,
		accountID, userID string,
		platformAdmin bool,
	) (PublishingScope, error)
	ListPortfolio(
		ctx context.Context,
		accountIDs []string,
		now time.Time,
	) ([]portfolioClientRecord, error)
}

type analyticsWorkerRepository interface {
	AnalyticsTarget(ctx context.Context, accountID, postID string) (analyticsTarget, error)
	SaveAnalytics(
		ctx context.Context,
		accountID, postID, jobKey string,
		analytics Analytics,
	) error
}

type workerRepository interface {
	ProtectPublishOutcome(ctx context.Context, accountID, postID string, revision int) (bool, error)
	PreparePublish(ctx context.Context, accountID, postID string, revision int) (publishTarget, bool, error)
	SaveCreationID(ctx context.Context, accountID, postID string, revision int, creationID string) error
	MarkPublishAttempted(ctx context.Context, accountID, postID string, revision int) (bool, error)
	MarkPublished(ctx context.Context, accountID, postID string, revision int, mediaID string, publishedAt time.Time) error
	SavePermalink(ctx context.Context, accountID, postID, mediaID, permalink string) error
	MarkPublishFailed(
		ctx context.Context,
		accountID, postID string,
		revision int,
		code, message string,
	) error
}

type serviceRepository interface {
	connectionRepository
	postRepository
	analyticsRepository
	portfolioRepository
	analyticsWorkerRepository
}

type jobEnqueuer interface {
	Enqueue(ctx context.Context, job jobs.NewJob) (id string, created bool, err error)
}
