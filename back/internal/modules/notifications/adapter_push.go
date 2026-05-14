package notifications

import "context"

type PushAdapter struct{}

func NewPushAdapter() *PushAdapter {
	return &PushAdapter{}
}

func (adapter *PushAdapter) Channel() string {
	return ChannelPush
}

func (adapter *PushAdapter) Send(context.Context, Notification) error {
	return ErrNotConfigured
}
