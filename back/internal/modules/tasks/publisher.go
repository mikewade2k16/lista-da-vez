package tasks

import "context"

type TaskEvent struct {
	Type      string
	AccountID string
	BoardID   string
	TaskID    string
	Version   int
}

type BoardEvent struct {
	Type      string
	AccountID string
	BoardID   string
}

type PresenceEvent struct {
	Type        string
	AccountID   string
	BoardID     string
	TaskID      string
	UserID      string
	DisplayName string
	AvatarPath  string
	FieldKey    string
	LockID      string
}

type Publisher interface {
	PublishTaskEvent(ctx context.Context, event TaskEvent)
	PublishBoardEvent(ctx context.Context, event BoardEvent)
	PublishPresenceEvent(ctx context.Context, event PresenceEvent)
}

type noopPublisher struct{}

func (noopPublisher) PublishTaskEvent(context.Context, TaskEvent)   {}
func (noopPublisher) PublishBoardEvent(context.Context, BoardEvent) {}
func (noopPublisher) PublishPresenceEvent(context.Context, PresenceEvent) {
}
