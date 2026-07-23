package omnichannel

import (
	"encoding/json"
	"time"
)

// CRMContactFilter e a busca 360° do CRM. O filtro nunca carrega account_id: o
// serviço recebe a conta exclusivamente do Principal.
type CRMContactFilter struct {
	Limit          int
	BeforeCursor   string
	Search         string
	Channel        string
	Status         string
	Tag            string
	OwnerID        string
	Source         string
	LastSeenAfter  *time.Time
	LastSeenBefore *time.Time
}

type CRMContactPageView struct {
	Contacts   []CRMContactView `json:"contacts"`
	HasMore    bool             `json:"hasMore"`
	NextCursor string           `json:"nextCursor,omitempty"`
}

// CRMContactView amplia ContactView sem alterar o contrato enxuto usado pelo inbox.
// JSONB dinâmico permanece RawMessage para não introduzir any no contrato HTTP.
type CRMContactView struct {
	ID                       string          `json:"id"`
	TenantID                 string          `json:"tenantId"`
	Name                     string          `json:"name"`
	Phone                    string          `json:"phone"`
	AvatarURL                *string         `json:"avatarUrl"`
	Source                   string          `json:"source"`
	PrimaryEmail             *string         `json:"primaryEmail"`
	OwnerUserID              *string         `json:"ownerUserId"`
	MergedIntoContactID      *string         `json:"mergedIntoContactId"`
	ArchivedAt               *time.Time      `json:"archivedAt"`
	RelationshipStatus       string          `json:"relationshipStatus"`
	Tags                     json.RawMessage `json:"tags"`
	CustomFields             json.RawMessage `json:"customFields"`
	ClassificationSource     string          `json:"classificationSource"`
	ClassificationConfidence *float64        `json:"classificationConfidence"`
	LastQualifiedAt          *time.Time      `json:"lastQualifiedAt"`
	FirstSeenAt              *time.Time      `json:"firstSeenAt"`
	LastSeenAt               *time.Time      `json:"lastSeenAt"`
	FirstChannel             *string         `json:"firstChannel"`
	LastChannel              *string         `json:"lastChannel"`
	CreatedAt                time.Time       `json:"createdAt"`
	UpdatedAt                time.Time       `json:"updatedAt"`
	LastConversationID       *string         `json:"lastConversationId"`
	LastConversationAt       *time.Time      `json:"lastConversationAt"`
	LastConversationChannel  *string         `json:"lastConversationChannel"`
	LastConversationStatus   *string         `json:"lastConversationStatus"`
}

type ContactIdentityView struct {
	ID               string          `json:"id"`
	Channel          string          `json:"channel"`
	Provider         string          `json:"provider"`
	InstanceScopeKey string          `json:"instanceScopeKey"`
	ExternalID       string          `json:"externalId"`
	DisplayName      *string         `json:"displayName"`
	AvatarURL        *string         `json:"avatarUrl"`
	Metadata         json.RawMessage `json:"metadata"`
	FirstSeenAt      time.Time       `json:"firstSeenAt"`
	LastSeenAt       time.Time       `json:"lastSeenAt"`
}

type ContactTouchpointView struct {
	ID              string          `json:"id"`
	ConversationID  *string         `json:"conversationId"`
	MessageID       *string         `json:"messageId"`
	Channel         string          `json:"channel"`
	Provider        string          `json:"provider"`
	ExternalEventID *string         `json:"externalEventId"`
	SourceKind      string          `json:"sourceKind"`
	SourceRef       *string         `json:"sourceRef"`
	LandingPageID   *string         `json:"landingPageId"`
	LandingSourceID *string         `json:"landingSourceId"`
	CampaignID      *string         `json:"campaignId"`
	UTMSource       *string         `json:"utmSource"`
	UTMMedium       *string         `json:"utmMedium"`
	UTMCampaign     *string         `json:"utmCampaign"`
	UTMTerm         *string         `json:"utmTerm"`
	UTMContent      *string         `json:"utmContent"`
	ReferrerHost    *string         `json:"referrerHost"`
	Metadata        json.RawMessage `json:"metadata"`
	OccurredAt      time.Time       `json:"occurredAt"`
}

type ContactNoteView struct {
	ID             string    `json:"id"`
	ConversationID *string   `json:"conversationId"`
	AuthorUserID   *string   `json:"authorUserId"`
	Content        string    `json:"content"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type ContactConversationRefView struct {
	ID            string    `json:"id"`
	Channel       string    `json:"channel"`
	State         string    `json:"state"`
	ExternalID    string    `json:"externalId"`
	LastMessageAt time.Time `json:"lastMessageAt"`
}

type CRMContactProfileView struct {
	Contact               CRMContactView               `json:"contact"`
	Intelligence          ContactIntelligenceView      `json:"intelligence"`
	Identities            []ContactIdentityView        `json:"identities"`
	Touchpoints           []ContactTouchpointView      `json:"touchpoints"`
	Notes                 []ContactNoteView            `json:"notes"`
	Conversations         []ContactConversationRefView `json:"conversations"`
	TouchpointsHasMore    bool                         `json:"touchpointsHasMore"`
	TouchpointsNextCursor string                       `json:"touchpointsNextCursor,omitempty"`
	NotesHasMore          bool                         `json:"notesHasMore"`
	NotesNextCursor       string                       `json:"notesNextCursor,omitempty"`
}

// ContactIntelligenceView is the compact, contact-scoped memory used by the
// brain and exposed in the 360 profile. It contains derived facts and counters,
// never raw conversation history or provider credentials.
type ContactIntelligenceView struct {
	PreferredName      *string         `json:"preferredName"`
	NameSource         string          `json:"nameSource"`
	RelationshipStatus string          `json:"relationshipStatus"`
	Tags               json.RawMessage `json:"tags"`
	Summary            string          `json:"summary"`
	Facts              json.RawMessage `json:"facts"`
	Preferences        json.RawMessage `json:"preferences"`
	InteractionCount   int             `json:"interactionCount"`
	AIReplyCount       int             `json:"aiReplyCount"`
	HandoffCount       int             `json:"handoffCount"`
	LastIntent         string          `json:"lastIntent"`
	LastSentiment      string          `json:"lastSentiment"`
	LastConfidence     *float64        `json:"lastConfidence"`
	LastOutcome        string          `json:"lastOutcome"`
	LastConversationID *string         `json:"lastConversationId"`
	LastLearnedAt      *time.Time      `json:"lastLearnedAt"`
	UpdatedAt          *time.Time      `json:"updatedAt"`
}

type CRMContactPatch struct {
	Name               *string         `json:"name"`
	PrimaryEmail       json.RawMessage `json:"primaryEmail"`
	OwnerUserID        json.RawMessage `json:"ownerUserId"`
	RelationshipStatus *string         `json:"relationshipStatus"`
	Tags               json.RawMessage `json:"tags"`
	CustomFields       json.RawMessage `json:"customFields"`
	ExpectedUpdatedAt  *time.Time      `json:"expectedUpdatedAt"`
}

type ContactNoteInput struct {
	Content        string  `json:"content"`
	ConversationID *string `json:"conversationId"`
}

type ContactNotePageView struct {
	Notes      []ContactNoteView `json:"notes"`
	HasMore    bool              `json:"hasMore"`
	NextCursor string            `json:"nextCursor,omitempty"`
}

type ContactMergeInput struct {
	TargetID       string `json:"targetId"`
	Reason         string `json:"reason"`
	IdempotencyKey string `json:"idempotencyKey"`
}

type ContactMergeView struct {
	EventID         string     `json:"eventId"`
	SourceContactID string     `json:"sourceContactId"`
	TargetContactID string     `json:"targetContactId"`
	UndoneAt        *time.Time `json:"undoneAt"`
	CreatedAt       time.Time  `json:"createdAt"`
}

// ContactSegmentView representa um filtro salvo; a lista de contatos continua
// sendo calculada sob demanda e nunca vira uma cópia materializada.
type ContactSegmentView struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Filter      json.RawMessage `json:"filter"`
	Version     int             `json:"version"`
	OwnerUserID *string         `json:"ownerUserId"`
	IsActive    bool            `json:"isActive"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

type ContactSegmentInput struct {
	Name   string          `json:"name"`
	Filter json.RawMessage `json:"filter"`
}

type ContactSegmentPatch struct {
	Name     *string         `json:"name"`
	Filter   json.RawMessage `json:"filter"`
	IsActive *bool           `json:"isActive"`
}
