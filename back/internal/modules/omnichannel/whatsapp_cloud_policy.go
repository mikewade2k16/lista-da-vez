package omnichannel

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
)

var (
	ErrWhatsAppWindowClosed        = errors.New("omnichannel: whatsapp customer-service window closed")
	ErrWhatsAppTemplateNotApproved = errors.New("omnichannel: whatsapp template is not approved")
)

// enforceWhatsAppCloudPolicy is called while the conversation is locked and
// immediately before the provider side effect. The decision is therefore made
// from current PostgreSQL state, never from n8n or a stale frontend badge.
func (s *Store) enforceWhatsAppCloudPolicy(ctx context.Context, tx pgx.Tx, accountID string, data outboundSendData) error {
	if data.Provider != "meta_whatsapp_cloud" {
		return nil
	}
	var instanceID string
	if err := tx.QueryRow(ctx, `select id::text from messaging.whatsapp_instances
		where account_id = $1::uuid and provider = 'meta_whatsapp_cloud' and instance_name = $2 and is_active = true`,
		accountID, data.InstanceScopeKey).Scan(&instanceID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &jobs.StatusError{StatusCode: 422, Unrecoverable: true, Err: errors.New("whatsapp instance inactive")}
		}
		return err
	}

	var open bool
	if err := tx.QueryRow(ctx, `select exists(select 1 from messaging.channel_windows
		where account_id = $1::uuid and conversation_id = $2::uuid and provider = 'meta_whatsapp_cloud'
		  and window_kind = 'customer_service' and expires_at > now())`, accountID, data.ConversationID).Scan(&open); err != nil {
		return err
	}
	template := strings.EqualFold(strings.TrimSpace(data.MessageType), "TEMPLATE")
	if template || !open {
		if !template {
			return &jobs.StatusError{StatusCode: 422, Unrecoverable: true, Err: ErrWhatsAppWindowClosed}
		}
		if strings.TrimSpace(data.TemplateName) == "" || strings.TrimSpace(data.TemplateLanguage) == "" {
			return &jobs.StatusError{StatusCode: 422, Unrecoverable: true, Err: ErrWhatsAppTemplateNotApproved}
		}
		var approved bool
		if err := tx.QueryRow(ctx, `select exists(select 1 from messaging.whatsapp_templates
			where account_id = $1::uuid and instance_id = $2::uuid and name = $3 and language = $4 and status = 'APPROVED')`,
			accountID, instanceID, strings.TrimSpace(data.TemplateName), strings.TrimSpace(data.TemplateLanguage)).Scan(&approved); err != nil {
			return err
		}
		if !approved {
			return &jobs.StatusError{StatusCode: 422, Unrecoverable: true, Err: ErrWhatsAppTemplateNotApproved}
		}
	}
	return nil
}
