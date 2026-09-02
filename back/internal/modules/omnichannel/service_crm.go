package omnichannel

import (
	"context"
	"encoding/json"
	"net/mail"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

const (
	crmPermissionView   = "omnichannel.conversations.view"
	crmPermissionManage = "omnichannel.contacts.manage"
)

// ListCRMContacts is the paginated, tenant-scoped 360-degree contact view.
// The account is always obtained from the authenticated Principal by the HTTP layer.
func (s *Service) ListCRMContacts(ctx context.Context, accountID string, p auth.Principal, f CRMContactFilter) (CRMContactPageView, error) {
	visibility, err := s.resolveConversationVisibility(ctx, accountID, p.UserID, crmPermissionView, InstanceGrantView)
	if err != nil {
		return CRMContactPageView{}, err
	}
	normalized, err := normalizeCRMFilter(f)
	if err != nil {
		return CRMContactPageView{}, err
	}
	rows, err := s.store.ListCRMContacts(ctx, accountID, normalized, visibility)
	if err != nil {
		return CRMContactPageView{}, translate(err)
	}
	hasMore := len(rows) > normalized.Limit
	if hasMore {
		rows = rows[:normalized.Limit]
	}
	contacts := make([]CRMContactView, 0, len(rows))
	for _, row := range rows {
		contacts = append(contacts, crmContactView(row))
	}
	next := ""
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		next = encodeCRMContactCursor(last.UpdatedAt, last.ID)
	}
	return CRMContactPageView{Contacts: contacts, HasMore: hasMore, NextCursor: next}, nil
}

func (s *Service) GetCRMContactProfile(ctx context.Context, accountID string, p auth.Principal, contactID, touchBefore, notesBefore string, limit int) (CRMContactProfileView, error) {
	visibility, err := s.resolveConversationVisibility(ctx, accountID, p.UserID, crmPermissionView, InstanceGrantView)
	if err != nil {
		return CRMContactProfileView{}, err
	}
	if !omnichannelUUIDPattern.MatchString(strings.TrimSpace(contactID)) {
		return CRMContactProfileView{}, ErrNotFound
	}
	contact, err := s.store.GetCRMContact(ctx, accountID, contactID, visibility)
	if err != nil {
		return CRMContactProfileView{}, translate(err)
	}
	intelligence, err := s.store.GetContactIntelligence(ctx, accountID, contactID)
	if err != nil {
		return CRMContactProfileView{}, translate(err)
	}
	limit = ensureCRMLimit(limit)
	identities, err := s.store.ListContactIdentities(ctx, accountID, contactID)
	if err != nil {
		return CRMContactProfileView{}, err
	}
	touchpoints, touchMore, touchNext, err := s.store.ListContactTouchpoints(ctx, accountID, contactID, strings.TrimSpace(touchBefore), limit)
	if err != nil {
		return CRMContactProfileView{}, err
	}
	notes, notesMore, notesNext, err := s.store.ListContactNotes(ctx, accountID, contactID, strings.TrimSpace(notesBefore), limit)
	if err != nil {
		return CRMContactProfileView{}, err
	}
	conversations, err := s.store.ListContactConversations(ctx, accountID, contactID, limit, visibility)
	if err != nil {
		return CRMContactProfileView{}, err
	}
	return CRMContactProfileView{
		Contact: crmContactView(contact), Intelligence: intelligence, Identities: identities, Touchpoints: touchpoints,
		Notes: notes, Conversations: conversations, TouchpointsHasMore: touchMore,
		TouchpointsNextCursor: touchNext, NotesHasMore: notesMore, NotesNextCursor: notesNext,
	}, nil
}

func (s *Service) UpdateCRMContact(ctx context.Context, accountID string, p auth.Principal, contactID string, patch CRMContactPatch) (CRMContactView, error) {
	visibility, err := s.resolveConversationVisibility(ctx, accountID, p.UserID, crmPermissionManage, InstanceGrantView)
	if err != nil {
		return CRMContactView{}, err
	}
	if !omnichannelUUIDPattern.MatchString(strings.TrimSpace(contactID)) {
		return CRMContactView{}, ErrNotFound
	}
	current, err := s.store.GetCRMContact(ctx, accountID, contactID, visibility)
	if err != nil {
		return CRMContactView{}, translate(err)
	}
	name := current.Name
	if patch.Name != nil {
		name = strings.TrimSpace(*patch.Name)
		if len([]rune(name)) > 160 {
			return CRMContactView{}, ErrInvalidBody
		}
	}
	primaryEmail, emailSet, err := normalizeCRMEmail(patch.PrimaryEmail)
	if err != nil {
		return CRMContactView{}, err
	}
	ownerID, ownerSet, err := normalizeCRMOwner(patch.OwnerUserID)
	if err != nil {
		return CRMContactView{}, err
	}
	status := ""
	if patch.RelationshipStatus != nil {
		status = crmStatus(*patch.RelationshipStatus)
		if !validCRMStatus(status) {
			return CRMContactView{}, ErrInvalidBody
		}
	}
	tags, tagsSet, err := normalizeCRMTags(patch.Tags)
	if err != nil {
		return CRMContactView{}, err
	}
	customFields, customSet, err := normalizeCRMCustomFields(patch.CustomFields)
	if err != nil {
		return CRMContactView{}, err
	}
	if err := s.store.UpdateCRMContact(ctx, accountID, contactID, name, primaryEmail, ownerID, status,
		tags, customFields, patch.ExpectedUpdatedAt, emailSet, ownerSet, tagsSet, customSet); err != nil {
		return CRMContactView{}, translate(err)
	}
	updated, err := s.store.GetCRMContact(ctx, accountID, contactID, visibility)
	if err != nil {
		return CRMContactView{}, translate(err)
	}
	return crmContactView(updated), nil
}

func (s *Service) ListCRMContactNotes(ctx context.Context, accountID string, p auth.Principal, contactID, before string, limit int) (ContactNotePageView, error) {
	visibility, err := s.resolveConversationVisibility(ctx, accountID, p.UserID, crmPermissionView, InstanceGrantView)
	if err != nil {
		return ContactNotePageView{}, err
	}
	if !omnichannelUUIDPattern.MatchString(strings.TrimSpace(contactID)) {
		return ContactNotePageView{}, ErrNotFound
	}
	if _, err := s.store.GetCRMContact(ctx, accountID, contactID, visibility); err != nil {
		return ContactNotePageView{}, translate(err)
	}
	limit = ensureCRMLimit(limit)
	notes, more, next, err := s.store.ListContactNotes(ctx, accountID, contactID, before, limit)
	if err != nil {
		return ContactNotePageView{}, err
	}
	return ContactNotePageView{Notes: notes, HasMore: more, NextCursor: next}, nil
}

func (s *Service) CreateCRMContactNote(ctx context.Context, accountID string, p auth.Principal, contactID string, in ContactNoteInput) (ContactNoteView, error) {
	visibility, err := s.resolveConversationVisibility(ctx, accountID, p.UserID, crmPermissionManage, InstanceGrantView)
	if err != nil {
		return ContactNoteView{}, err
	}
	if !omnichannelUUIDPattern.MatchString(strings.TrimSpace(contactID)) || (in.ConversationID != nil && !omnichannelUUIDPattern.MatchString(strings.TrimSpace(*in.ConversationID))) {
		return ContactNoteView{}, ErrNotFound
	}
	in.Content = strings.TrimSpace(in.Content)
	if in.Content == "" || len([]rune(in.Content)) > 4000 {
		return ContactNoteView{}, ErrInvalidBody
	}
	if _, err := s.store.GetCRMContact(ctx, accountID, contactID, visibility); err != nil {
		return ContactNoteView{}, translate(err)
	}
	if in.ConversationID != nil {
		conversation, err := s.store.GetVisibleConversation(ctx, accountID, visibility, strings.TrimSpace(*in.ConversationID))
		if err != nil || conversation.ContactID == nil || *conversation.ContactID != contactID {
			return ContactNoteView{}, ErrNotFound
		}
	}
	return s.store.CreateContactNote(ctx, accountID, contactID, p.UserID, in)
}

func (s *Service) ListLeadSources(ctx context.Context, accountID string, p auth.Principal) ([]LeadSourceView, error) {
	if err := s.requirePermission(ctx, accountID, p, "omnichannel.settings.manage"); err != nil {
		return nil, err
	}
	return s.store.ListLeadSources(ctx, accountID)
}

func (s *Service) CreateLeadSource(ctx context.Context, accountID string, p auth.Principal, in LeadSourceInput) (LeadSourceWriteView, error) {
	if err := s.requirePermission(ctx, accountID, p, "omnichannel.settings.manage"); err != nil {
		return LeadSourceWriteView{}, err
	}
	if err := normalizeLeadSourceInput(&in); err != nil {
		return LeadSourceWriteView{}, err
	}
	return s.store.CreateLeadSource(ctx, accountID, in)
}

func (s *Service) UpdateLeadSource(ctx context.Context, accountID string, p auth.Principal, id string, patch LeadSourcePatch) (LeadSourceWriteView, error) {
	if err := s.requirePermission(ctx, accountID, p, "omnichannel.settings.manage"); err != nil {
		return LeadSourceWriteView{}, err
	}
	if patch.Name != nil {
		value := strings.TrimSpace(*patch.Name)
		if value == "" || len([]rune(value)) > 160 {
			return LeadSourceWriteView{}, ErrInvalidBody
		}
		patch.Name = &value
	}
	if patch.Domain != nil {
		value := strings.TrimSpace(*patch.Domain)
		if len([]rune(value)) > 240 {
			return LeadSourceWriteView{}, ErrInvalidBody
		}
		patch.Domain = &value
	}
	if patch.AllowedOrigins != nil {
		origins, err := normalizeLeadOrigins(*patch.AllowedOrigins)
		if err != nil {
			return LeadSourceWriteView{}, err
		}
		patch.AllowedOrigins = &origins
	}
	return s.store.UpdateLeadSource(ctx, accountID, id, patch)
}

func (s *Service) ListContactSegments(ctx context.Context, accountID string, p auth.Principal) ([]ContactSegmentView, error) {
	if err := s.requirePermission(ctx, accountID, p, "omnichannel.settings.manage"); err != nil {
		return nil, err
	}
	return s.store.ListContactSegments(ctx, accountID)
}

func (s *Service) CreateContactSegment(ctx context.Context, accountID string, p auth.Principal, in ContactSegmentInput) (ContactSegmentView, error) {
	if err := s.requirePermission(ctx, accountID, p, "omnichannel.settings.manage"); err != nil {
		return ContactSegmentView{}, err
	}
	if err := normalizeContactSegmentInput(&in); err != nil {
		return ContactSegmentView{}, err
	}
	out, err := s.store.CreateContactSegment(ctx, accountID, p.UserID, in)
	if isUniqueViolation(err) {
		return ContactSegmentView{}, ErrConflict
	}
	return out, err
}

func (s *Service) UpdateContactSegment(ctx context.Context, accountID string, p auth.Principal, id string, patch ContactSegmentPatch) (ContactSegmentView, error) {
	if err := s.requirePermission(ctx, accountID, p, "omnichannel.settings.manage"); err != nil {
		return ContactSegmentView{}, err
	}
	if !omnichannelUUIDPattern.MatchString(strings.TrimSpace(id)) {
		return ContactSegmentView{}, ErrNotFound
	}
	if patch.Name != nil {
		name := strings.TrimSpace(*patch.Name)
		if name == "" || len([]rune(name)) > 120 {
			return ContactSegmentView{}, ErrInvalidBody
		}
		patch.Name = &name
	}
	if len(patch.Filter) > 0 {
		if _, err := normalizeSegmentFilter(patch.Filter); err != nil {
			return ContactSegmentView{}, err
		}
	}
	out, err := s.store.UpdateContactSegment(ctx, accountID, id, patch)
	if isUniqueViolation(err) {
		return ContactSegmentView{}, ErrConflict
	}
	return out, err
}

func normalizeLeadSourceInput(in *LeadSourceInput) error {
	in.Slug = strings.ToLower(strings.TrimSpace(in.Slug))
	in.Name = strings.TrimSpace(in.Name)
	in.Domain = strings.TrimSpace(in.Domain)
	if in.Slug == "" || len([]rune(in.Slug)) > 80 || in.Name == "" || len([]rune(in.Name)) > 160 || len([]rune(in.Domain)) > 240 {
		return ErrInvalidBody
	}
	origins, err := normalizeLeadOrigins(in.AllowedOrigins)
	if err != nil {
		return err
	}
	in.AllowedOrigins = origins
	return nil
}

type contactSegmentFilter struct {
	Search         string     `json:"search"`
	Channel        string     `json:"channel"`
	Status         string     `json:"status"`
	Tag            string     `json:"tag"`
	OwnerID        string     `json:"ownerId"`
	Source         string     `json:"source"`
	LastSeenAfter  *time.Time `json:"lastSeenAfter"`
	LastSeenBefore *time.Time `json:"lastSeenBefore"`
}

func normalizeContactSegmentInput(in *ContactSegmentInput) error {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || len([]rune(in.Name)) > 120 {
		return ErrInvalidBody
	}
	normalized, err := normalizeSegmentFilter(in.Filter)
	if err != nil {
		return err
	}
	in.Filter = normalized
	return nil
}

func normalizeSegmentFilter(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 || string(raw) == "null" {
		raw = json.RawMessage(`{}`)
	}
	var filter contactSegmentFilter
	if err := json.Unmarshal(raw, &filter); err != nil {
		return nil, ErrInvalidBody
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return nil, ErrInvalidBody
	}
	for key := range fields {
		switch key {
		case "search", "channel", "status", "tag", "ownerId", "source", "lastSeenAfter", "lastSeenBefore":
		default:
			return nil, ErrInvalidBody
		}
	}
	normalized, err := normalizeCRMFilter(CRMContactFilter{Search: filter.Search, Channel: filter.Channel, Status: filter.Status,
		Tag: filter.Tag, OwnerID: filter.OwnerID, Source: filter.Source, LastSeenAfter: filter.LastSeenAfter, LastSeenBefore: filter.LastSeenBefore})
	if err != nil {
		return nil, err
	}
	return json.Marshal(contactSegmentFilter{Search: normalized.Search, Channel: normalized.Channel, Status: normalized.Status,
		Tag: normalized.Tag, OwnerID: normalized.OwnerID, Source: normalized.Source,
		LastSeenAfter: normalized.LastSeenAfter, LastSeenBefore: normalized.LastSeenBefore})
}

func normalizeLeadOrigins(values []string) ([]string, error) {
	if len(values) > 20 {
		return nil, ErrInvalidBody
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" || len([]rune(value)) > 240 || (!strings.HasPrefix(value, "https://") && !strings.HasPrefix(value, "http://")) {
			return nil, ErrInvalidBody
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out, nil
}

func normalizeCRMFilter(f CRMContactFilter) (CRMContactFilter, error) {
	f.Search = strings.Join(strings.Fields(strings.TrimSpace(f.Search)), " ")
	if len([]rune(f.Search)) > 120 {
		return CRMContactFilter{}, ErrInvalidBody
	}
	f.Channel = strings.ToUpper(strings.TrimSpace(f.Channel))
	if f.Channel == "ALL" {
		f.Channel = ""
	}
	if f.Channel != "" && f.Channel != "WHATSAPP" && f.Channel != "INSTAGRAM" {
		return CRMContactFilter{}, ErrInvalidBody
	}
	f.Status = crmStatus(f.Status)
	if f.Status == "ALL" {
		f.Status = ""
	}
	if f.Status != "" && !validCRMStatus(f.Status) {
		return CRMContactFilter{}, ErrInvalidBody
	}
	for _, id := range []string{f.OwnerID} {
		if strings.TrimSpace(id) != "" && !omnichannelUUIDPattern.MatchString(strings.TrimSpace(id)) {
			return CRMContactFilter{}, ErrInvalidBody
		}
	}
	f.OwnerID = strings.TrimSpace(f.OwnerID)
	f.Source = strings.TrimSpace(f.Source)
	f.Tag = strings.TrimSpace(strings.ToLower(f.Tag))
	if len([]rune(f.Source)) > 80 || len([]rune(f.Tag)) > 64 {
		return CRMContactFilter{}, ErrInvalidBody
	}
	f.Limit = ensureCRMLimit(f.Limit)
	return f, nil
}

func validCRMStatus(value string) bool {
	switch value {
	case "new_lead", "known_lead", "customer", "inactive":
		return true
	default:
		return false
	}
}

func normalizeCRMEmail(raw json.RawMessage) (*string, bool, error) {
	if len(raw) == 0 {
		return nil, false, nil
	}
	if string(raw) == "null" {
		return nil, true, nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, false, ErrInvalidBody
	}
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return nil, true, nil
	}
	if len([]rune(value)) > 320 {
		return nil, false, ErrInvalidBody
	}
	if _, err := mail.ParseAddress(value); err != nil {
		return nil, false, ErrInvalidBody
	}
	return &value, true, nil
}

func normalizeCRMOwner(raw json.RawMessage) (*string, bool, error) {
	if len(raw) == 0 {
		return nil, false, nil
	}
	if string(raw) == "null" {
		return nil, true, nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || !omnichannelUUIDPattern.MatchString(strings.TrimSpace(value)) {
		return nil, false, ErrInvalidBody
	}
	value = strings.TrimSpace(value)
	return &value, true, nil
}

func normalizeCRMTags(raw json.RawMessage) (json.RawMessage, bool, error) {
	if len(raw) == 0 {
		return nil, false, nil
	}
	if string(raw) == "null" {
		return json.RawMessage(`[]`), true, nil
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || len(values) > 50 {
		return nil, false, ErrInvalidBody
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, item := range values {
		item = strings.ToLower(strings.TrimSpace(item))
		if item == "" || len([]rune(item)) > 64 {
			return nil, false, ErrInvalidBody
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	encoded, err := json.Marshal(out)
	return encoded, true, err
}

func normalizeCRMCustomFields(raw json.RawMessage) (json.RawMessage, bool, error) {
	if len(raw) == 0 {
		return nil, false, nil
	}
	if string(raw) == "null" {
		return json.RawMessage(`{}`), true, nil
	}
	var value map[string]json.RawMessage
	if err := json.Unmarshal(raw, &value); err != nil || value == nil || len(value) > 100 {
		return nil, false, ErrInvalidBody
	}
	encoded, err := json.Marshal(value)
	return encoded, true, err
}
