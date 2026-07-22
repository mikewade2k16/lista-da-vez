package meta_whatsapp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel/channel"
)

type webhookEnvelope struct {
	Object string  `json:"object"`
	Entry  []entry `json:"entry"`
}

type entry struct {
	Changes []change `json:"changes"`
}

type change struct {
	Value changeValue `json:"value"`
}

type changeValue struct {
	Metadata metadata  `json:"metadata"`
	Contacts []contact `json:"contacts"`
	Messages []message `json:"messages"`
	Statuses []status  `json:"statuses"`
}

type metadata struct {
	PhoneNumberID string `json:"phone_number_id"`
}

type contact struct {
	WAMID   string  `json:"wa_id"`
	Profile profile `json:"profile"`
}

type profile struct {
	Name string `json:"name"`
}

type message struct {
	From      string           `json:"from"`
	ID        string           `json:"id"`
	Timestamp string           `json:"timestamp"`
	Type      string           `json:"type"`
	Text      *textContent     `json:"text"`
	Image     *mediaContent    `json:"image"`
	Audio     *mediaContent    `json:"audio"`
	Video     *mediaContent    `json:"video"`
	Document  *documentContent `json:"document"`
	Context   *messageContext  `json:"context"`
}

type textContent struct {
	Body string `json:"body"`
}

type mediaContent struct {
	ID       string `json:"id"`
	MimeType string `json:"mime_type"`
	Caption  string `json:"caption"`
}

type documentContent struct {
	ID       string `json:"id"`
	MimeType string `json:"mime_type"`
	Caption  string `json:"caption"`
	Filename string `json:"filename"`
}

type messageContext struct {
	ID string `json:"id"`
}

type status struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
	Errors    []struct {
		Code int `json:"code"`
	} `json:"errors"`
}

// parseWebhook contains the provider-specific translation. HTTP authentication is
// performed by VerifyWebhook before this method is called.
func (p *Provider) parseWebhook(_ context.Context, body []byte) ([]channel.Event, error) {
	var envelope webhookEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil || envelope.Object != "whatsapp_business_account" {
		return nil, errors.New("meta whatsapp: webhook payload invalid")
	}
	out := make([]channel.Event, 0)
	for _, entry := range envelope.Entry {
		for _, change := range entry.Changes {
			phoneID := strings.TrimSpace(change.Value.Metadata.PhoneNumberID)
			if phoneID == "" {
				continue
			}
			contacts := make(map[string]string, len(change.Value.Contacts))
			for _, c := range change.Value.Contacts {
				contacts[strings.TrimSpace(c.WAMID)] = strings.TrimSpace(c.Profile.Name)
			}
			for _, m := range change.Value.Messages {
				id := strings.TrimSpace(m.ID)
				from := strings.TrimSpace(m.From)
				if id == "" || from == "" {
					continue
				}
				msgType, content, mediaID, mime, fileName, caption := normalizeMessage(m)
				item := channel.Event{Kind: channel.EventMessageReceived, ExternalEventID: phoneID + ":msg:" + id,
					InstanceName: phoneID, OccurredAt: timestamp(m.Timestamp), Message: &channel.InboundMessage{
						ExternalMessageID: id, Channel: "WHATSAPP", ContactExternalID: from, ContactPhone: from,
						ContactName: contacts[from], MessageType: msgType, Content: content, MediaURL: mediaID,
						MediaMimeType: mime, MediaFileName: fileName, MediaCaption: caption,
						FromMe: false,
					},
				}
				if m.Context != nil && strings.TrimSpace(m.Context.ID) != "" {
					item.Message.Reply = &channel.ReplyReference{ExternalMessageID: strings.TrimSpace(m.Context.ID)}
				}
				out = append(out, item)
			}
			for _, status := range change.Value.Statuses {
				id := strings.TrimSpace(status.ID)
				if id == "" {
					continue
				}
				code := ""
				if len(status.Errors) > 0 && status.Errors[0].Code > 0 {
					code = "meta_" + strconv.Itoa(status.Errors[0].Code)
				}
				out = append(out, channel.Event{Kind: channel.EventMessageStatus, ExternalEventID: phoneID + ":status:" + id + ":" + strings.ToLower(strings.TrimSpace(status.Status)), InstanceName: phoneID, OccurredAt: timestamp(status.Timestamp), Status: &channel.StatusUpdate{ExternalMessageID: id, Status: normalizeStatus(status.Status), ErrorCode: code}})
			}
		}
	}
	if len(out) == 0 {
		return []channel.Event{{Kind: channel.EventIgnored}}, nil
	}
	return out, nil
}

func (p *Provider) ParseWebhook(ctx context.Context, _ http.Header, body []byte) ([]channel.Event, error) {
	return p.parseWebhook(ctx, body)
}

func normalizeMessage(m message) (kind, content, mediaID, mime, fileName, caption string) {
	switch strings.ToLower(strings.TrimSpace(m.Type)) {
	case "text":
		if m.Text != nil {
			return "TEXT", strings.TrimSpace(m.Text.Body), "", "", "", ""
		}
	case "image":
		if m.Image != nil {
			return "IMAGE", strings.TrimSpace(m.Image.Caption), strings.TrimSpace(m.Image.ID), strings.TrimSpace(m.Image.MimeType), "", strings.TrimSpace(m.Image.Caption)
		}
	case "audio":
		if m.Audio != nil {
			return "AUDIO", "", strings.TrimSpace(m.Audio.ID), strings.TrimSpace(m.Audio.MimeType), "", ""
		}
	case "video":
		if m.Video != nil {
			return "VIDEO", strings.TrimSpace(m.Video.Caption), strings.TrimSpace(m.Video.ID), strings.TrimSpace(m.Video.MimeType), "", strings.TrimSpace(m.Video.Caption)
		}
	case "document":
		if m.Document != nil {
			return "DOCUMENT", strings.TrimSpace(m.Document.Caption), strings.TrimSpace(m.Document.ID), strings.TrimSpace(m.Document.MimeType), strings.TrimSpace(m.Document.Filename), strings.TrimSpace(m.Document.Caption)
		}
	}
	return "TEXT", "", "", "", "", ""
}

func normalizeStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "sent":
		return "SENT"
	case "delivered":
		return "DELIVERED"
	case "read":
		return "READ"
	case "failed":
		return "FAILED"
	default:
		return "SENT"
	}
}
