package omnichannel

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type crmContactRow struct {
	ID                       string
	AccountID                string
	Name                     string
	Phone                    string
	AvatarURL                *string
	Source                   string
	PrimaryEmail             *string
	OwnerUserID              *string
	MergedIntoContactID      *string
	ArchivedAt               *time.Time
	RelationshipStatus       string
	Tags                     json.RawMessage
	CustomFields             json.RawMessage
	ClassificationSource     string
	ClassificationConfidence *float64
	LastQualifiedAt          *time.Time
	FirstSeenAt              *time.Time
	LastSeenAt               *time.Time
	FirstChannel             *string
	LastChannel              *string
	CreatedAt                time.Time
	UpdatedAt                time.Time
	LastConversationID       *string
	LastConversationAt       *time.Time
	LastConversationChannel  *string
	LastConversationState    *string
}

const crmContactColumns = `ct.id::text, ct.account_id::text, ct.name, coalesce(ct.phone, ''), ct.avatar_url,
	ct.source, ct.primary_email, ct.owner_user_id::text, ct.merged_into_contact_id::text, ct.archived_at,
	ct.relationship_status, ct.tags, ct.custom_fields, ct.classification_source, ct.classification_confidence::float8,
	ct.last_qualified_at, ct.first_seen_at, ct.last_seen_at, ct.first_channel, ct.last_channel,
	ct.created_at, ct.updated_at, lc.id::text, lc.last_message_at, lc.channel, lc.state`

const crmContactJoin = `
	left join lateral (
		select cv.id, cv.last_message_at, cv.channel, cv.state
		from messaging.conversations cv
		where cv.contact_id = ct.id and cv.account_id = ct.account_id
		order by cv.last_message_at desc, cv.id desc
		limit 1
	) lc on true`

func scanCRMContact(row rowScanner) (crmContactRow, error) {
	var out crmContactRow
	err := row.Scan(
		&out.ID, &out.AccountID, &out.Name, &out.Phone, &out.AvatarURL, &out.Source,
		&out.PrimaryEmail, &out.OwnerUserID, &out.MergedIntoContactID, &out.ArchivedAt,
		&out.RelationshipStatus, &out.Tags, &out.CustomFields, &out.ClassificationSource,
		&out.ClassificationConfidence, &out.LastQualifiedAt, &out.FirstSeenAt, &out.LastSeenAt,
		&out.FirstChannel, &out.LastChannel, &out.CreatedAt, &out.UpdatedAt,
		&out.LastConversationID, &out.LastConversationAt, &out.LastConversationChannel,
		&out.LastConversationState,
	)
	return out, err
}

type crmContactCursor struct {
	UpdatedAt time.Time
	ID        string
}

func encodeCRMContactCursor(updatedAt time.Time, id string) string {
	raw := updatedAt.UTC().Format(time.RFC3339Nano) + "|" + id
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCRMContactCursor(raw string) (crmContactCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return crmContactCursor{}, ErrInvalidBody
	}
	parts := strings.SplitN(string(decoded), "|", 2)
	if len(parts) != 2 || !omnichannelUUIDPattern.MatchString(parts[1]) {
		return crmContactCursor{}, ErrInvalidBody
	}
	t, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return crmContactCursor{}, ErrInvalidBody
	}
	return crmContactCursor{UpdatedAt: t, ID: parts[1]}, nil
}

func (s *Store) ListCRMContacts(ctx context.Context, accountID string, f CRMContactFilter) ([]crmContactRow, error) {
	query := `select ` + crmContactColumns + ` from messaging.contacts ct` + crmContactJoin +
		` where ct.account_id = $1::uuid and ct.archived_at is null
		  and not exists (select 1 from messaging.contact_suppressions suppression
		      where suppression.account_id=ct.account_id and suppression.contact_id=ct.id
		        and suppression.is_hidden=true)`
	args := []any{accountID}
	if f.Search != "" {
		args = append(args, "%"+escapeLike(strings.ToLower(f.Search))+"%")
		pos := strconv.Itoa(len(args))
		query += ` and (lower(ct.name) like $` + pos + ` escape '\\'
			or lower(coalesce(ct.phone, '')) like $` + pos + ` escape '\\'
			or lower(coalesce(ct.primary_email, '')) like $` + pos + ` escape '\\')`
	}
	if f.Channel != "" {
		args = append(args, f.Channel)
		query += ` and exists (select 1 from messaging.contact_identities ci
			where ci.account_id = ct.account_id and ci.contact_id = ct.id and ci.channel = $` + strconv.Itoa(len(args)) + `)`
	}
	if f.Status != "" {
		args = append(args, f.Status)
		query += ` and case ct.relationship_status when 'lead' then 'new_lead' when 'prospect' then 'known_lead'
			else ct.relationship_status end = $` + strconv.Itoa(len(args))
	}
	if f.Tag != "" {
		args = append(args, f.Tag)
		query += ` and ct.tags ? $` + strconv.Itoa(len(args))
	}
	if f.OwnerID != "" {
		args = append(args, f.OwnerID)
		query += ` and ct.owner_user_id = $` + strconv.Itoa(len(args)) + `::uuid`
	}
	if f.Source != "" {
		args = append(args, f.Source)
		query += ` and ct.source = $` + strconv.Itoa(len(args))
	}
	if f.LastSeenAfter != nil {
		args = append(args, *f.LastSeenAfter)
		query += ` and ct.last_seen_at >= $` + strconv.Itoa(len(args))
	}
	if f.LastSeenBefore != nil {
		args = append(args, *f.LastSeenBefore)
		query += ` and ct.last_seen_at < $` + strconv.Itoa(len(args))
	}
	if f.BeforeCursor != "" {
		cursor, err := decodeCRMContactCursor(f.BeforeCursor)
		if err != nil {
			return nil, err
		}
		args = append(args, cursor.UpdatedAt, cursor.ID)
		query += ` and (ct.updated_at, ct.id) < ($` + strconv.Itoa(len(args)-1) + `, $` + strconv.Itoa(len(args)) + `::uuid)`
	}
	args = append(args, f.Limit)
	query += ` order by ct.updated_at desc, ct.id desc limit $` + strconv.Itoa(len(args))

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]crmContactRow, 0, f.Limit)
	for rows.Next() {
		row, err := scanCRMContact(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func scanContactSegment(row rowScanner) (ContactSegmentView, error) {
	var out ContactSegmentView
	err := row.Scan(&out.ID, &out.Name, &out.Filter, &out.Version, &out.OwnerUserID, &out.IsActive, &out.CreatedAt, &out.UpdatedAt)
	return out, err
}

const contactSegmentColumns = `id::text, name, filter_json, version, owner_user_id::text, is_active, created_at, updated_at`

func (s *Store) ListContactSegments(ctx context.Context, accountID string) ([]ContactSegmentView, error) {
	rows, err := s.pool.Query(ctx, `select `+contactSegmentColumns+` from messaging.contact_segments
		where account_id=$1::uuid and is_active order by updated_at desc, id desc`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ContactSegmentView, 0)
	for rows.Next() {
		row, err := scanContactSegment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) CreateContactSegment(ctx context.Context, accountID, ownerID string, in ContactSegmentInput) (ContactSegmentView, error) {
	return scanContactSegment(s.pool.QueryRow(ctx, `insert into messaging.contact_segments
		(account_id, name, filter_json, owner_user_id) values ($1::uuid,$2,$3::jsonb,nullif($4,'')::uuid)
		returning `+contactSegmentColumns, accountID, in.Name, in.Filter, ownerID))
}

func (s *Store) UpdateContactSegment(ctx context.Context, accountID, id string, patch ContactSegmentPatch) (ContactSegmentView, error) {
	var name any
	if patch.Name != nil {
		name = *patch.Name
	}
	var filter any
	if len(patch.Filter) > 0 {
		filter = patch.Filter
	}
	row, err := scanContactSegment(s.pool.QueryRow(ctx, `update messaging.contact_segments set
		name=coalesce($3,name), filter_json=coalesce($4::jsonb,filter_json), is_active=coalesce($5,is_active),
		version=version+1, updated_at=now()
		where account_id=$1::uuid and id=$2::uuid returning `+contactSegmentColumns, accountID, id, name, filter, patch.IsActive))
	if errors.Is(err, pgx.ErrNoRows) {
		return ContactSegmentView{}, ErrNotFound
	}
	return row, err
}

func (s *Store) GetCRMContact(ctx context.Context, accountID, contactID string) (crmContactRow, error) {
	row, err := scanCRMContact(s.pool.QueryRow(ctx, `select `+crmContactColumns+` from messaging.contacts ct`+crmContactJoin+
		` where ct.account_id = $1::uuid and ct.id = $2::uuid`, accountID, contactID))
	if errors.Is(err, pgx.ErrNoRows) {
		return crmContactRow{}, ErrNotFound
	}
	return row, err
}

func scanContactIdentity(row rowScanner) (ContactIdentityView, error) {
	var out ContactIdentityView
	err := row.Scan(&out.ID, &out.Channel, &out.Provider, &out.InstanceScopeKey, &out.ExternalID,
		&out.DisplayName, &out.AvatarURL, &out.Metadata, &out.FirstSeenAt, &out.LastSeenAt)
	return out, err
}

func (s *Store) ListContactIdentities(ctx context.Context, accountID, contactID string) ([]ContactIdentityView, error) {
	rows, err := s.pool.Query(ctx, `select id::text, channel, provider, instance_scope_key, external_id,
		display_name, avatar_url, metadata, first_seen_at, last_seen_at
		from messaging.contact_identities where account_id = $1::uuid and contact_id = $2::uuid
		order by last_seen_at desc, id desc`, accountID, contactID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ContactIdentityView, 0)
	for rows.Next() {
		row, err := scanContactIdentity(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func scanContactTouchpoint(row rowScanner) (ContactTouchpointView, error) {
	var out ContactTouchpointView
	err := row.Scan(&out.ID, &out.ConversationID, &out.MessageID, &out.Channel, &out.Provider,
		&out.ExternalEventID, &out.SourceKind, &out.SourceRef, &out.LandingPageID,
		&out.LandingSourceID, &out.CampaignID, &out.UTMSource, &out.UTMMedium,
		&out.UTMCampaign, &out.UTMTerm, &out.UTMContent, &out.ReferrerHost,
		&out.Metadata, &out.OccurredAt)
	return out, err
}

const contactTouchpointColumns = `id::text, conversation_id::text, message_id::text, channel, provider,
	external_event_id, source_kind, source_ref, landing_page_id, landing_source_id::text,
	campaign_id, utm_source, utm_medium, utm_campaign, utm_term, utm_content, referrer_host,
	metadata, occurred_at`

func (s *Store) ListContactTouchpoints(ctx context.Context, accountID, contactID, before string, limit int) ([]ContactTouchpointView, bool, string, error) {
	query := `select ` + contactTouchpointColumns + ` from messaging.contact_touchpoints
		where account_id = $1::uuid and contact_id = $2::uuid`
	args := []any{accountID, contactID}
	if strings.TrimSpace(before) != "" {
		cursor, err := decodeCRMContactCursor(before)
		if err != nil {
			return nil, false, "", err
		}
		args = append(args, cursor.UpdatedAt, cursor.ID)
		query += ` and (occurred_at, id) < ($3, $4::uuid)`
	}
	args = append(args, limit+1)
	query += ` order by occurred_at desc, id desc limit $` + strconv.Itoa(len(args))
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, false, "", err
	}
	defer rows.Close()
	out := make([]ContactTouchpointView, 0, limit)
	for rows.Next() {
		row, err := scanContactTouchpoint(rows)
		if err != nil {
			return nil, false, "", err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, false, "", err
	}
	hasMore := len(out) > limit
	if hasMore {
		out = out[:limit]
	}
	next := ""
	if hasMore && len(out) > 0 {
		next = encodeCRMContactCursor(out[len(out)-1].OccurredAt, out[len(out)-1].ID)
	}
	return out, hasMore, next, nil
}

func scanContactNote(row rowScanner) (ContactNoteView, error) {
	var out ContactNoteView
	err := row.Scan(&out.ID, &out.ConversationID, &out.AuthorUserID, &out.Content, &out.CreatedAt, &out.UpdatedAt)
	return out, err
}

func (s *Store) ListContactNotes(ctx context.Context, accountID, contactID, before string, limit int) ([]ContactNoteView, bool, string, error) {
	query := `select id::text, conversation_id::text, author_user_id::text, content, created_at, updated_at
		from messaging.contact_notes where account_id = $1::uuid and contact_id = $2::uuid`
	args := []any{accountID, contactID}
	if strings.TrimSpace(before) != "" {
		cursor, err := decodeCRMContactCursor(before)
		if err != nil {
			return nil, false, "", err
		}
		args = append(args, cursor.UpdatedAt, cursor.ID)
		query += ` and (created_at, id) < ($3, $4::uuid)`
	}
	args = append(args, limit+1)
	query += ` order by created_at desc, id desc limit $` + strconv.Itoa(len(args))
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, false, "", err
	}
	defer rows.Close()
	out := make([]ContactNoteView, 0, limit)
	for rows.Next() {
		row, err := scanContactNote(rows)
		if err != nil {
			return nil, false, "", err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, false, "", err
	}
	hasMore := len(out) > limit
	if hasMore {
		out = out[:limit]
	}
	next := ""
	if hasMore && len(out) > 0 {
		next = encodeCRMContactCursor(out[len(out)-1].CreatedAt, out[len(out)-1].ID)
	}
	return out, hasMore, next, nil
}

func (s *Store) ListContactConversations(ctx context.Context, accountID, contactID string, limit int) ([]ContactConversationRefView, error) {
	rows, err := s.pool.Query(ctx, `select id::text, channel, state, external_id, last_message_at
		from messaging.conversations where account_id = $1::uuid and contact_id = $2::uuid
		order by last_message_at desc, id desc limit $3`, accountID, contactID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ContactConversationRefView, 0, limit)
	for rows.Next() {
		var item ContactConversationRefView
		if err := rows.Scan(&item.ID, &item.Channel, &item.State, &item.ExternalID, &item.LastMessageAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) UpdateCRMContact(ctx context.Context, accountID, contactID, name string, primaryEmail, ownerID *string,
	status string, tags, customFields json.RawMessage, expected *time.Time, emailSet, ownerSet, tagsSet, customSet bool) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if expected != nil {
		var ok bool
		if err := tx.QueryRow(ctx, `select exists(select 1 from messaging.contacts where account_id=$1::uuid and id=$2::uuid and updated_at=$3)`, accountID, contactID, *expected).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return ErrConflict
		}
	}
	if ownerSet && ownerID != nil {
		var ok bool
		if err := tx.QueryRow(ctx, `select exists(select 1 from core.account_users where account_id=$1::uuid and user_id=$2::uuid and is_active)`, accountID, *ownerID).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return ErrNotFound
		}
	}
	if name == "" {
		name = ""
	}
	args := []any{accountID, contactID, name, emailSet, deref(primaryEmail), ownerSet, deref(ownerID), status,
		tagsSet, tags, customSet, customFields}
	_, err = tx.Exec(ctx, `update messaging.contacts set
		name = case when $3 <> '' then $3 else name end,
		primary_email = case when $4 then nullif($5,'') else primary_email end,
		owner_user_id = case when $6 then nullif($7,'')::uuid else owner_user_id end,
		relationship_status = case when $8 <> '' then $8 else relationship_status end,
		classification_source = case when $8 <> '' then 'manual' else classification_source end,
		classification_confidence = case when $8 <> '' then 1 else classification_confidence end,
		last_qualified_at = case when $8 <> '' then now() else last_qualified_at end,
		tags = case when $9 then $10::jsonb else tags end,
		custom_fields = case when $11 then $12::jsonb else custom_fields end,
		updated_at = now()
		where account_id=$1::uuid and id=$2::uuid`, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrConflict
		}
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) CreateContactNote(ctx context.Context, accountID, contactID, authorID string, in ContactNoteInput) (ContactNoteView, error) {
	var out ContactNoteView
	err := s.pool.QueryRow(ctx, `insert into messaging.contact_notes
		(account_id, contact_id, conversation_id, author_user_id, content)
		select $1::uuid, c.id, nullif($3,'')::uuid, nullif($4,'')::uuid, $5
		from messaging.contacts c
		where c.account_id=$1::uuid and c.id=$2::uuid and c.archived_at is null
		  and (nullif($3,'') is null or exists (select 1 from messaging.conversations cv where cv.account_id=$1::uuid and cv.id=$3::uuid and cv.contact_id=c.id))
		returning id::text, conversation_id::text, author_user_id::text, content, created_at, updated_at`,
		accountID, contactID, deref(in.ConversationID), authorID, strings.TrimSpace(in.Content)).Scan(
		&out.ID, &out.ConversationID, &out.AuthorUserID, &out.Content, &out.CreatedAt, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ContactNoteView{}, ErrNotFound
	}
	return out, err
}

func (s *Store) GetContactNote(ctx context.Context, accountID, contactID, noteID string) (ContactNoteView, error) {
	var out ContactNoteView
	err := s.pool.QueryRow(ctx, `select id::text, conversation_id::text, author_user_id::text, content, created_at, updated_at
		from messaging.contact_notes where account_id=$1::uuid and contact_id=$2::uuid and id=$3::uuid`, accountID, contactID, noteID).
		Scan(&out.ID, &out.ConversationID, &out.AuthorUserID, &out.Content, &out.CreatedAt, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ContactNoteView{}, ErrNotFound
	}
	return out, err
}

func ensureCRMLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > 200 {
		return 200
	}
	return limit
}

func crmStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "lead":
		return "new_lead"
	case "prospect":
		return "known_lead"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func crmContactView(row crmContactRow) CRMContactView {
	return CRMContactView{
		ID: row.ID, TenantID: row.AccountID, Name: row.Name, Phone: row.Phone, AvatarURL: row.AvatarURL,
		Source: row.Source, PrimaryEmail: row.PrimaryEmail, OwnerUserID: row.OwnerUserID,
		MergedIntoContactID: row.MergedIntoContactID, ArchivedAt: row.ArchivedAt,
		RelationshipStatus: crmStatus(row.RelationshipStatus), Tags: jsonOrEmptyArray(row.Tags),
		CustomFields: jsonOrEmpty(row.CustomFields), ClassificationSource: row.ClassificationSource,
		ClassificationConfidence: row.ClassificationConfidence, LastQualifiedAt: row.LastQualifiedAt,
		FirstSeenAt: row.FirstSeenAt, LastSeenAt: row.LastSeenAt, FirstChannel: row.FirstChannel,
		LastChannel: row.LastChannel, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		LastConversationID: row.LastConversationID, LastConversationAt: row.LastConversationAt,
		LastConversationChannel: row.LastConversationChannel,
		LastConversationStatus:  crmStatusPointer(row.LastConversationState),
	}
}

func crmStatusPointer(value *string) *string {
	if value == nil {
		return nil
	}
	out := string(projectStatus(*value))
	return &out
}
