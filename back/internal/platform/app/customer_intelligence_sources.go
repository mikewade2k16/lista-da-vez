package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/customerdata"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/customerintelligence"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel"
)

// customerDataOfflineSourceAdapter translates the deterministic Customer Data
// façade into Intelligence observations. It never reads customer_data.* SQL.
type customerDataOfflineSourceAdapter struct {
	service func() *customerdata.Service
}

func (a customerDataOfflineSourceAdapter) Fetch(
	ctx context.Context,
	config customerintelligence.SourceConfig,
	relationshipID string,
) ([]customerintelligence.Observation, error) {
	service := a.service()
	if service == nil || strings.TrimSpace(relationshipID) == "" {
		return nil, customerintelligence.ErrNotFound
	}
	bundle, err := service.GetSourceEvidence(ctx, customerdata.SourceEvidenceRequest{
		AccountID:       config.AccountID,
		ClientAccountID: config.ClientAccountID,
		RelationshipID:  relationshipID,
		Limit:           200,
	})
	if err != nil {
		return nil, err
	}
	items := make([]customerintelligence.Observation, 0, len(bundle.Interactions))
	for _, interaction := range bundle.Interactions {
		snapshot := map[string]any{
			"interaction_type": interaction.InteractionType,
			"occurred_at":      interaction.OccurredAt,
			"timezone":         interaction.Timezone,
			"title":            interaction.Title,
			"status":           interaction.Status,
		}
		if interaction.DurationSeconds != nil {
			snapshot["duration_seconds"] = *interaction.DurationSeconds
		}
		if interaction.Content != nil {
			snapshot["content"] = *interaction.Content
		}
		if interaction.SourceExternalRef != nil {
			snapshot["source_external_ref"] = *interaction.SourceExternalRef
		}
		raw, marshalErr := json.Marshal(snapshot)
		if marshalErr != nil {
			return nil, marshalErr
		}
		occurredAt := interaction.OccurredAt
		items = append(items, customerintelligence.Observation{
			IdempotencyKey: fmt.Sprintf(
				"manual.offline:%s:%d",
				interaction.ID,
				interaction.Revision,
			),
			EntityType:     "offline_interaction",
			EntityID:       interaction.ID,
			Version:        fmt.Sprint(interaction.Revision),
			SubjectID:      bundle.SubjectID,
			RelationshipID: bundle.RelationshipID,
			OccurredAt:     &occurredAt,
			Snapshot:       raw,
			Sensitivity:    interaction.Sensitivity,
			PurposeKey:     firstNonEmptyApp(config.PurposeKey, interaction.PurposeKey),
		})
	}
	return items, nil
}

// omnichannelSourceAdapter resolves contact IDs through Customer Data first,
// then uses the Omnichannel-owned read façade. This prevents a source config
// from selecting arbitrary contacts or querying messaging.* directly.
type omnichannelSourceAdapter struct {
	customerData func() *customerdata.Service
	omnichannel  func() *omnichannel.Service
}

func (a omnichannelSourceAdapter) Fetch(
	ctx context.Context,
	config customerintelligence.SourceConfig,
	relationshipID string,
) ([]customerintelligence.Observation, error) {
	dataService := a.customerData()
	channelService := a.omnichannel()
	if dataService == nil || channelService == nil || strings.TrimSpace(relationshipID) == "" {
		return nil, customerintelligence.ErrNotFound
	}
	bundle, err := dataService.GetSourceEvidence(ctx, customerdata.SourceEvidenceRequest{
		AccountID:       config.AccountID,
		ClientAccountID: config.ClientAccountID,
		RelationshipID:  relationshipID,
		Limit:           1,
	})
	if err != nil {
		return nil, err
	}
	contactIDs := make([]string, 0)
	for _, link := range bundle.SourceLinks {
		if link.SourceModule == "omnichannel" &&
			link.SourceEntityType == "contact" &&
			strings.TrimSpace(link.SourceEntityID) != "" {
			contactIDs = append(contactIDs, link.SourceEntityID)
		}
	}
	var options struct {
		LookbackDays         int  `json:"lookbackDays"`
		IncludeMediaMetadata bool `json:"includeMediaMetadata"`
	}
	if len(config.Config) > 0 && json.Unmarshal(config.Config, &options) != nil {
		return nil, customerintelligence.ErrInvalidInput
	}
	messages, err := channelService.CustomerEvidence(ctx, omnichannel.CustomerEvidenceRequest{
		AccountID:            config.AccountID,
		ClientAccountID:      config.ClientAccountID,
		ContactSourceIDs:     contactIDs,
		LookbackDays:         options.LookbackDays,
		Limit:                200,
		IncludeMediaMetadata: options.IncludeMediaMetadata,
	})
	if err != nil {
		return nil, err
	}
	items := make([]customerintelligence.Observation, 0, len(messages))
	for _, message := range messages {
		snapshot := map[string]any{
			"message_id":        message.MessageID,
			"conversation_id":   message.ConversationID,
			"contact_source_id": message.ContactSourceID,
			"channel":           message.Channel,
			"direction":         message.Direction,
			"message_type":      message.MessageType,
			"content":           message.Content,
			"status":            message.Status,
			"occurred_at":       message.OccurredAt,
		}
		if options.IncludeMediaMetadata {
			snapshot["media_mime_type"] = message.MediaMimeType
			snapshot["media_file_name"] = message.MediaFileName
			snapshot["media_caption"] = message.MediaCaption
			if message.MediaDurationSecs != nil {
				snapshot["media_duration_seconds"] = *message.MediaDurationSecs
			}
		}
		raw, marshalErr := json.Marshal(snapshot)
		if marshalErr != nil {
			return nil, marshalErr
		}
		occurredAt := message.OccurredAt
		items = append(items, customerintelligence.Observation{
			IdempotencyKey: fmt.Sprintf(
				"omnichannel:%s:%d",
				message.MessageID,
				message.UpdatedAt.UnixNano(),
			),
			EntityType:     "message",
			EntityID:       message.MessageID,
			Version:        message.UpdatedAt.UTC().Format("20060102T150405.000000000Z"),
			SubjectID:      bundle.SubjectID,
			RelationshipID: bundle.RelationshipID,
			OccurredAt:     &occurredAt,
			Snapshot:       raw,
			Sensitivity:    "personal",
			PurposeKey:     config.PurposeKey,
		})
	}
	return items, nil
}

var (
	_ customerintelligence.SourceAdapter = customerDataOfflineSourceAdapter{}
	_ customerintelligence.SourceAdapter = omnichannelSourceAdapter{}
)
