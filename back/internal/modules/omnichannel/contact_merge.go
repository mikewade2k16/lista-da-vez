package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

type contactMergeSnapshot struct {
	IdentityIDs     []string `json:"identityIds"`
	TouchpointIDs   []string `json:"touchpointIds"`
	NoteIDs         []string `json:"noteIds"`
	ConversationIDs []string `json:"conversationIds"`
}

func (s *Service) MergeCRMContacts(ctx context.Context, accountID string, p auth.Principal, sourceID string, in ContactMergeInput) (ContactMergeView, error) {
	if err := s.requirePermission(ctx, accountID, p, crmPermissionManage); err != nil {
		return ContactMergeView{}, err
	}
	in.TargetID = strings.TrimSpace(in.TargetID)
	in.IdempotencyKey = strings.TrimSpace(in.IdempotencyKey)
	in.Reason = strings.TrimSpace(in.Reason)
	if !validCRMUUIDPair(sourceID, in.TargetID) || in.IdempotencyKey == "" || len([]rune(in.IdempotencyKey)) > 128 || len([]rune(in.Reason)) > 500 {
		return ContactMergeView{}, ErrInvalidBody
	}
	return s.store.MergeContacts(ctx, accountID, sourceID, in.TargetID, p.UserID, in.Reason, in.IdempotencyKey)
}

func (s *Service) UndoCRMContactMerge(ctx context.Context, accountID string, p auth.Principal, eventID string) (ContactMergeView, error) {
	if err := s.requirePermission(ctx, accountID, p, crmPermissionManage); err != nil {
		return ContactMergeView{}, err
	}
	if !omnichannelUUIDPattern.MatchString(strings.TrimSpace(eventID)) {
		return ContactMergeView{}, ErrNotFound
	}
	return s.store.UndoContactMerge(ctx, accountID, eventID, p.UserID)
}

func validCRMUUIDPair(sourceID, targetID string) bool {
	return omnichannelUUIDPattern.MatchString(strings.TrimSpace(sourceID)) && omnichannelUUIDPattern.MatchString(strings.TrimSpace(targetID)) && !strings.EqualFold(strings.TrimSpace(sourceID), strings.TrimSpace(targetID))
}

func (s *Store) MergeContacts(ctx context.Context, accountID, sourceID, targetID, actorID, reason, idempotencyKey string) (ContactMergeView, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ContactMergeView{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// One deterministic tenant-scoped lock serializes all merges for this pair.
	if _, err := tx.Exec(ctx, `select pg_advisory_xact_lock(hashtextextended($1, 0))`, accountID+":"+minString(sourceID, targetID)+":"+maxString(sourceID, targetID)); err != nil {
		return ContactMergeView{}, err
	}
	var existing ContactMergeView
	err = tx.QueryRow(ctx, `select id::text, source_contact_id::text, target_contact_id::text, null::timestamptz, created_at
		from messaging.contact_merge_events where account_id=$1::uuid and idempotency_key=$2`, accountID, idempotencyKey).
		Scan(&existing.EventID, &existing.SourceContactID, &existing.TargetContactID, &existing.UndoneAt, &existing.CreatedAt)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return ContactMergeView{}, err
	}
	var sourceExists, targetExists bool
	if err := tx.QueryRow(ctx, `select exists(select 1 from messaging.contacts where account_id=$1::uuid and id=$2::uuid and archived_at is null), exists(select 1 from messaging.contacts where account_id=$1::uuid and id=$3::uuid and archived_at is null)`, accountID, sourceID, targetID).Scan(&sourceExists, &targetExists); err != nil {
		return ContactMergeView{}, err
	}
	if !sourceExists || !targetExists {
		return ContactMergeView{}, ErrNotFound
	}
	var collision bool
	if err := tx.QueryRow(ctx, `select exists(select 1 from messaging.contact_identities s join messaging.contact_identities t on t.account_id=s.account_id and t.channel=s.channel and t.provider=s.provider and t.instance_scope_key=s.instance_scope_key and t.external_id=s.external_id where s.account_id=$1::uuid and s.contact_id=$2::uuid and t.contact_id=$3::uuid)`, accountID, sourceID, targetID).Scan(&collision); err != nil {
		return ContactMergeView{}, err
	}
	if collision {
		return ContactMergeView{}, ErrConflict
	}
	snapshot, err := readMergeSnapshot(ctx, tx, accountID, sourceID)
	if err != nil {
		return ContactMergeView{}, err
	}
	// Move children first; source is retained as an archived tombstone for undo/audit.
	for _, stmt := range []string{
		`update messaging.contact_identities set contact_id=$3::uuid, updated_at=now() where account_id=$1::uuid and contact_id=$2::uuid`,
		`update messaging.contact_touchpoints set contact_id=$3::uuid where account_id=$1::uuid and contact_id=$2::uuid`,
		`update messaging.contact_notes set contact_id=$3::uuid, updated_at=now() where account_id=$1::uuid and contact_id=$2::uuid`,
		`update messaging.conversations set contact_id=$3::uuid, updated_at=now() where account_id=$1::uuid and contact_id=$2::uuid`,
	} {
		if _, err := tx.Exec(ctx, stmt, accountID, sourceID, targetID); err != nil {
			return ContactMergeView{}, err
		}
	}
	if _, err := tx.Exec(ctx, `update messaging.contacts target set
		name=case when nullif(target.name,'') is null then source.name else target.name end,
		primary_email=coalesce(target.primary_email, source.primary_email),
		tags=(select coalesce(jsonb_agg(distinct value), '[]'::jsonb) from jsonb_array_elements_text(coalesce(target.tags,'[]'::jsonb) || coalesce(source.tags,'[]'::jsonb)) value), updated_at=now()
		from messaging.contacts source where target.account_id=$1::uuid and target.id=$3::uuid and source.account_id=$1::uuid and source.id=$2::uuid`, accountID, sourceID, targetID); err != nil {
		return ContactMergeView{}, err
	}
	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		return ContactMergeView{}, err
	}
	if err := tx.QueryRow(ctx, `update messaging.contacts set archived_at=now(), merged_into_contact_id=$3::uuid, updated_at=now() where account_id=$1::uuid and id=$2::uuid returning archived_at`, accountID, sourceID, targetID).Scan(new(time.Time)); err != nil {
		return ContactMergeView{}, err
	}
	var out ContactMergeView
	err = tx.QueryRow(ctx, `insert into messaging.contact_merge_events (account_id, source_contact_id, target_contact_id, actor_user_id, reason, idempotency_key, snapshot) values ($1::uuid,$2::uuid,$3::uuid,nullif($4,'')::uuid,$5,$6,$7::jsonb) returning id::text, source_contact_id::text, target_contact_id::text, null::timestamptz, created_at`, accountID, sourceID, targetID, actorID, reason, idempotencyKey, snapshotJSON).Scan(&out.EventID, &out.SourceContactID, &out.TargetContactID, &out.UndoneAt, &out.CreatedAt)
	if err != nil {
		return ContactMergeView{}, err
	}
	if _, err := tx.Exec(ctx, `insert into messaging.audit_events (account_id, actor_user_id, event_type, payload_json) values ($1::uuid, nullif($2,'')::uuid, 'CONTACT_MERGED', jsonb_build_object('eventId',$3,'sourceContactId',$4,'targetContactId',$5))`, accountID, actorID, out.EventID, out.SourceContactID, out.TargetContactID); err != nil {
		return ContactMergeView{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ContactMergeView{}, err
	}
	return out, nil
}

func (s *Store) UndoContactMerge(ctx context.Context, accountID, eventID, actorID string) (ContactMergeView, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ContactMergeView{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var out ContactMergeView
	var snapshotJSON []byte
	err = tx.QueryRow(ctx, `select id::text, source_contact_id::text, target_contact_id::text, undone_at, created_at, snapshot from messaging.contact_merge_events where account_id=$1::uuid and id=$2::uuid for update`, accountID, eventID).Scan(&out.EventID, &out.SourceContactID, &out.TargetContactID, &out.UndoneAt, &out.CreatedAt, &snapshotJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return ContactMergeView{}, ErrNotFound
	}
	if err != nil {
		return ContactMergeView{}, err
	}
	if out.UndoneAt != nil {
		return out, nil
	}
	var snapshot contactMergeSnapshot
	if err := json.Unmarshal(snapshotJSON, &snapshot); err != nil {
		return ContactMergeView{}, ErrConflict
	}
	if _, err := tx.Exec(ctx, `select pg_advisory_xact_lock(hashtextextended($1, 0))`, accountID+":"+minString(out.SourceContactID, out.TargetContactID)+":"+maxString(out.SourceContactID, out.TargetContactID)); err != nil {
		return ContactMergeView{}, err
	}
	var active, mergedInto *string
	if err := tx.QueryRow(ctx, `select merged_into_contact_id::text, case when archived_at is null then id::text end from messaging.contacts where account_id=$1::uuid and id=$2::uuid for update`, accountID, out.SourceContactID).Scan(&mergedInto, &active); err != nil {
		return ContactMergeView{}, translate(err)
	}
	if mergedInto == nil || *mergedInto != out.TargetContactID || active != nil {
		return ContactMergeView{}, ErrConflict
	}
	if len(snapshot.IdentityIDs) > 0 {
		if _, err := tx.Exec(ctx, `update messaging.contact_identities set contact_id=$3::uuid, updated_at=now() where account_id=$1::uuid and id=any($2::uuid[])`, accountID, snapshot.IdentityIDs, out.SourceContactID); err != nil {
			return ContactMergeView{}, err
		}
	}
	if len(snapshot.TouchpointIDs) > 0 {
		if _, err := tx.Exec(ctx, `update messaging.contact_touchpoints set contact_id=$3::uuid where account_id=$1::uuid and id=any($2::uuid[])`, accountID, snapshot.TouchpointIDs, out.SourceContactID); err != nil {
			return ContactMergeView{}, err
		}
	}
	if len(snapshot.NoteIDs) > 0 {
		if _, err := tx.Exec(ctx, `update messaging.contact_notes set contact_id=$3::uuid, updated_at=now() where account_id=$1::uuid and id=any($2::uuid[])`, accountID, snapshot.NoteIDs, out.SourceContactID); err != nil {
			return ContactMergeView{}, err
		}
	}
	if len(snapshot.ConversationIDs) > 0 {
		if _, err := tx.Exec(ctx, `update messaging.conversations set contact_id=$3::uuid, updated_at=now() where account_id=$1::uuid and id=any($2::uuid[])`, accountID, snapshot.ConversationIDs, out.SourceContactID); err != nil {
			return ContactMergeView{}, err
		}
	}
	if _, err := tx.Exec(ctx, `update messaging.contacts set archived_at=null, merged_into_contact_id=null, updated_at=now() where account_id=$1::uuid and id=$2::uuid`, accountID, out.SourceContactID); err != nil {
		return ContactMergeView{}, err
	}
	if err := tx.QueryRow(ctx, `update messaging.contact_merge_events set undone_at=now(), undo_actor_user_id=nullif($3,'')::uuid where account_id=$1::uuid and id=$2::uuid returning undone_at`, accountID, eventID, actorID).Scan(&out.UndoneAt); err != nil {
		return ContactMergeView{}, err
	}
	if _, err := tx.Exec(ctx, `insert into messaging.audit_events (account_id, actor_user_id, event_type, payload_json) values ($1::uuid, nullif($2,'')::uuid, 'CONTACT_MERGE_UNDONE', jsonb_build_object('eventId',$3,'sourceContactId',$4,'targetContactId',$5))`, accountID, actorID, out.EventID, out.SourceContactID, out.TargetContactID); err != nil {
		return ContactMergeView{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ContactMergeView{}, err
	}
	return out, nil
}

func readMergeSnapshot(ctx context.Context, tx pgx.Tx, accountID, contactID string) (contactMergeSnapshot, error) {
	var out contactMergeSnapshot
	for query, dst := range map[string]*[]string{
		`select id::text from messaging.contact_identities where account_id=$1::uuid and contact_id=$2::uuid order by id`:  &out.IdentityIDs,
		`select id::text from messaging.contact_touchpoints where account_id=$1::uuid and contact_id=$2::uuid order by id`: &out.TouchpointIDs,
		`select id::text from messaging.contact_notes where account_id=$1::uuid and contact_id=$2::uuid order by id`:       &out.NoteIDs,
		`select id::text from messaging.conversations where account_id=$1::uuid and contact_id=$2::uuid order by id`:       &out.ConversationIDs,
	} {
		rows, err := tx.Query(ctx, query, accountID, contactID)
		if err != nil {
			return contactMergeSnapshot{}, err
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return contactMergeSnapshot{}, err
			}
			*dst = append(*dst, id)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return contactMergeSnapshot{}, err
		}
		rows.Close()
	}
	return out, nil
}

func minString(a, b string) string {
	if a < b {
		return a
	}
	return b
}
func maxString(a, b string) string {
	if a > b {
		return a
	}
	return b
}
