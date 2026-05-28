package notifications

import "context"

type WhatsAppAdapter struct{}

func NewWhatsAppAdapter() *WhatsAppAdapter {
	return &WhatsAppAdapter{}
}

func (adapter *WhatsAppAdapter) Channel() string {
	return ChannelWhatsApp
}

func (adapter *WhatsAppAdapter) Send(context.Context, Notification) error {
	return ErrNotConfigured
}
