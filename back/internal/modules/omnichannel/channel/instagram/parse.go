package instagram

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel/channel"
)

type webhookEnvelope struct {
	Object string  `json:"object"`
	Entry  []entry `json:"entry"`
}

type entry struct {
	ID        string           `json:"id"`
	Time      int64            `json:"time"`
	Messaging []messagingEvent `json:"messaging"`
	Changes   []change         `json:"changes"`
}

type messagingEvent struct {
	Sender    ref        `json:"sender"`
	Recipient ref        `json:"recipient"`
	Timestamp int64      `json:"timestamp"`
	Message   *dmMessage `json:"message"`
}

type dmMessage struct {
	MID         string       `json:"mid"`
	Text        string       `json:"text"`
	Attachments []attachment `json:"attachments"`
}

type attachment struct {
	Type    string `json:"type"`
	Payload struct {
		URL string `json:"url"`
	} `json:"payload"`
}

type ref struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

type change struct {
	Field string       `json:"field"`
	Value commentValue `json:"value"`
}

type commentValue struct {
	ID    string `json:"id"`
	Text  string `json:"text"`
	From  ref    `json:"from"`
	Media struct {
		ID               string `json:"id"`
		MediaProductType string `json:"media_product_type"`
	} `json:"media"`
	ParentID  string `json:"parent_id"`
	Timestamp int64  `json:"timestamp"`
	Live      bool   `json:"is_live"`
}

func (p *Provider) ParseWebhook(_ context.Context, _ http.Header, body []byte) ([]channel.Event, error) {
	var envelope webhookEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil || (envelope.Object != "instagram" && envelope.Object != "page") {
		return nil, errors.New("instagram: webhook payload invalid")
	}
	out := make([]channel.Event, 0)
	for _, item := range envelope.Entry {
		accountID := strings.TrimSpace(item.ID)
		if accountID == "" {
			continue
		}
		for _, dm := range item.Messaging {
			if dm.Message == nil || strings.TrimSpace(dm.Message.MID) == "" {
				continue
			}
			fromMe := strings.TrimSpace(dm.Sender.ID) == accountID
			contact := dm.Sender
			if fromMe {
				contact = dm.Recipient
			}
			kind, content, mediaID, mime := "TEXT", strings.TrimSpace(dm.Message.Text), "", ""
			if len(dm.Message.Attachments) > 0 {
				kind = "IMAGE"
				mediaID = strings.TrimSpace(dm.Message.Attachments[0].Payload.URL)
				mime = "application/octet-stream"
			}
			out = append(out, channel.Event{
				Kind:            channel.EventMessageReceived,
				ExternalEventID: accountID + ":dm:" + dm.Message.MID,
				InstanceName:    accountID,
				OccurredAt:      timestamp(dm.Timestamp),
				Message: &channel.InboundMessage{
					ExternalMessageID: dm.Message.MID,
					Channel:           "INSTAGRAM",
					ContactExternalID: contact.ID,
					ContactName:       contact.Username,
					FromMe:            fromMe,
					MessageType:       kind,
					Content:           content,
					MediaURL:          mediaID,
					MediaMimeType:     mime,
					SocialEventKind:   "dm",
					SocialContentID:   dm.Message.MID,
				},
			})
		}
		for _, change := range item.Changes {
			if change.Field != "comments" && change.Field != "mentions" {
				continue
			}
			comment := change.Value
			if strings.TrimSpace(comment.ID) == "" || strings.TrimSpace(comment.From.ID) == "" {
				continue
			}
			eventKind := "comment"
			if change.Field == "mentions" {
				eventKind = "mention"
			}
			occurred := timestamp(comment.Timestamp)
			if comment.Timestamp == 0 {
				occurred = time.Unix(item.Time/1000, 0).UTC()
			}
			out = append(out, channel.Event{
				Kind:            channel.EventMessageReceived,
				ExternalEventID: accountID + ":" + eventKind + ":" + comment.ID,
				InstanceName:    accountID,
				OccurredAt:      occurred,
				Message: &channel.InboundMessage{
					ExternalMessageID:    comment.ID,
					Channel:              "INSTAGRAM",
					ContactExternalID:    comment.From.ID,
					ContactName:          comment.From.Username,
					MessageType:          "TEXT",
					Content:              comment.Text,
					SocialEventKind:      eventKind,
					SocialContentID:      comment.ID,
					SocialMediaID:        comment.Media.ID,
					SocialParentID:       comment.ParentID,
					SocialIsLive:         comment.Live,
					SocialReplyExpiresAt: ptrTime(occurred.Add(7 * 24 * time.Hour)),
				},
			})
		}
	}
	if len(out) == 0 {
		return []channel.Event{{Kind: channel.EventIgnored}}, nil
	}
	return out, nil
}

func timestamp(value int64) time.Time {
	if value <= 0 {
		return time.Now().UTC()
	}
	return time.Unix(value/1000, (value%1000)*int64(time.Millisecond)).UTC()
}

func ptrTime(value time.Time) *time.Time { return &value }
