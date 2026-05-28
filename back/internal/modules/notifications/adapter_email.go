package notifications

import "context"

type EmailAdapter struct{}

func NewEmailAdapter() *EmailAdapter {
	return &EmailAdapter{}
}

func (adapter *EmailAdapter) Channel() string {
	return ChannelEmail
}

func (adapter *EmailAdapter) Send(context.Context, Notification) error {
	return ErrNotConfigured
}
