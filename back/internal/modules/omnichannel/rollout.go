package omnichannel

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var ErrRolloutRevisionConflict = errors.New("omnichannel: rollout revision conflict")

const (
	RolloutModeOff       = "off"
	RolloutModeObserve   = "observe"
	RolloutModeShadow    = "shadow"
	RolloutModeAssist    = "assist"
	RolloutModeAutoPilot = "auto_pilot"
	RolloutModeActive    = "active"
	RolloutModePaused    = "paused"
)

type RolloutWindow struct {
	Days  []int  `json:"days"`
	Start string `json:"start"`
	End   string `json:"end"`
}

type RolloutHours struct {
	Timezone string          `json:"timezone"`
	Windows  []RolloutWindow `json:"windows"`
}

type RolloutConfigView struct {
	Mode                       string       `json:"mode"`
	AllowedInstanceIDs         []string     `json:"allowedInstanceIds"`
	AllowedInstagramAccountIDs []string     `json:"allowedInstagramAccountIds"`
	AllowedQueueIDs            []string     `json:"allowedQueueIds"`
	AutoReplyPercent           int          `json:"autoReplyPercent"`
	AllowedHours               RolloutHours `json:"allowedHours"`
	ExcludedTags               []string     `json:"excludedTags"`
	MaxDailyAutoReplies        int          `json:"maxDailyAutoReplies"`
	KillSwitchReason           *string      `json:"killSwitchReason"`
	Revision                   int64        `json:"revision"`
	LegacyDefault              bool         `json:"legacyDefault"`
	UpdatedByUserID            *string      `json:"updatedByUserId"`
	UpdatedAt                  *time.Time   `json:"updatedAt"`
}

type RolloutConfigInput struct {
	Mode                       string       `json:"mode"`
	AllowedInstanceIDs         []string     `json:"allowedInstanceIds"`
	AllowedInstagramAccountIDs []string     `json:"allowedInstagramAccountIds"`
	AllowedQueueIDs            []string     `json:"allowedQueueIds"`
	AutoReplyPercent           int          `json:"autoReplyPercent"`
	AllowedHours               RolloutHours `json:"allowedHours"`
	ExcludedTags               []string     `json:"excludedTags"`
	MaxDailyAutoReplies        int          `json:"maxDailyAutoReplies"`
	KillSwitchReason           *string      `json:"killSwitchReason"`
	ExpectedRevision           int64        `json:"expectedRevision"`
	Reason                     string       `json:"reason"`
}

type RolloutDecision struct {
	Mode     string
	RunAI    bool
	AutoSend bool
	Reason   string
}

type rolloutConversationContext struct {
	InstanceID string
	QueueID    string
	Tags       []string
}

type rolloutQueryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

type RolloutService struct {
	store *Store
}

func NewRolloutService(store *Store) *RolloutService {
	return &RolloutService{store: store}
}

func (s *RolloutService) Get(ctx context.Context, accountID, userID string) (RolloutConfigView, error) {
	scope, err := s.store.LoadConversationAccessScope(ctx, accountID, userID)
	if err != nil {
		return RolloutConfigView{}, err
	}
	if !scope.Eligible || (!scope.allowsPermission("omnichannel.audit.view") &&
		!scope.allowsPermission("omnichannel.settings.manage")) {
		return RolloutConfigView{}, ErrForbidden
	}
	return s.store.GetRolloutConfig(ctx, accountID)
}

func (s *RolloutService) Update(ctx context.Context, accountID, userID string, in RolloutConfigInput) (RolloutConfigView, error) {
	scope, err := s.store.LoadConversationAccessScope(ctx, accountID, userID)
	if err != nil {
		return RolloutConfigView{}, err
	}
	if !scope.Eligible || !scope.allowsPermission("omnichannel.settings.manage") {
		return RolloutConfigView{}, ErrForbidden
	}
	if err := normalizeRolloutInput(&in); err != nil {
		return RolloutConfigView{}, err
	}
	validScope, err := s.store.ValidateRolloutScope(ctx, accountID, in)
	if err != nil {
		return RolloutConfigView{}, err
	}
	if !validScope {
		return RolloutConfigView{}, ErrNotFound
	}
	return s.store.UpdateRolloutConfig(ctx, accountID, userID, in)
}

func normalizeRolloutInput(in *RolloutConfigInput) error {
	in.Mode = strings.ToLower(strings.TrimSpace(in.Mode))
	in.Reason = strings.TrimSpace(in.Reason)
	if in.ExpectedRevision < 0 || len(in.Reason) < 3 || len(in.Reason) > 500 ||
		in.AutoReplyPercent < 0 || in.AutoReplyPercent > 100 || in.MaxDailyAutoReplies < 0 {
		return ErrValidation
	}
	validMode := map[string]bool{
		RolloutModeOff: true, RolloutModeObserve: true, RolloutModeShadow: true,
		RolloutModeAssist: true, RolloutModeAutoPilot: true, RolloutModeActive: true,
		RolloutModePaused: true,
	}
	if !validMode[in.Mode] {
		return ErrValidation
	}
	var ok bool
	if in.AllowedInstanceIDs, ok = normalizedRolloutIDs(in.AllowedInstanceIDs); !ok {
		return ErrValidation
	}
	if in.AllowedInstagramAccountIDs, ok = normalizedRolloutIDs(in.AllowedInstagramAccountIDs); !ok {
		return ErrValidation
	}
	if in.AllowedQueueIDs, ok = normalizedRolloutIDs(in.AllowedQueueIDs); !ok {
		return ErrValidation
	}
	tags, ok := normalizedRolloutTags(in.ExcludedTags)
	if !ok {
		return ErrValidation
	}
	in.ExcludedTags = tags
	if err := validateRolloutHours(&in.AllowedHours); err != nil {
		return err
	}
	if in.KillSwitchReason != nil {
		value := strings.TrimSpace(*in.KillSwitchReason)
		in.KillSwitchReason = &value
	}
	if in.Mode == RolloutModePaused {
		if in.KillSwitchReason == nil || len(*in.KillSwitchReason) < 3 || len(*in.KillSwitchReason) > 500 {
			return ErrValidation
		}
	} else {
		in.KillSwitchReason = nil
	}
	return nil
}

func normalizedRolloutTags(values []string) ([]string, bool) {
	if len(values) > 50 {
		return nil, false
	}
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if len(value) == 0 || len(value) > 50 {
			return nil, false
		}
		set[value] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out, true
}

func validateRolloutHours(hours *RolloutHours) error {
	hours.Timezone = strings.TrimSpace(hours.Timezone)
	if hours.Timezone == "" {
		hours.Timezone = "America/Sao_Paulo"
	}
	if _, err := time.LoadLocation(hours.Timezone); err != nil || len(hours.Windows) > 50 {
		return ErrValidation
	}
	for i := range hours.Windows {
		window := &hours.Windows[i]
		start, startOK := parseClockMinute(window.Start)
		end, endOK := parseClockMinute(window.End)
		if !startOK || !endOK || start >= end || len(window.Days) > 7 {
			return ErrValidation
		}
		seen := map[int]bool{}
		for _, day := range window.Days {
			if day < 0 || day > 6 || seen[day] {
				return ErrValidation
			}
			seen[day] = true
		}
		sort.Ints(window.Days)
		window.Start = strings.TrimSpace(window.Start)
		window.End = strings.TrimSpace(window.End)
	}
	if hours.Windows == nil {
		hours.Windows = []RolloutWindow{}
	}
	return nil
}

func defaultRolloutConfig() RolloutConfigView {
	return RolloutConfigView{
		Mode: RolloutModeActive, AutoReplyPercent: 100, Revision: 0, LegacyDefault: true,
		AllowedInstanceIDs: []string{}, AllowedInstagramAccountIDs: []string{},
		AllowedQueueIDs: []string{}, ExcludedTags: []string{},
		AllowedHours: RolloutHours{Timezone: "America/Sao_Paulo", Windows: []RolloutWindow{}},
	}
}

func (s *Store) GetRolloutConfig(ctx context.Context, accountID string) (RolloutConfigView, error) {
	return loadRolloutConfig(ctx, s.pool, accountID)
}

func loadRolloutConfig(ctx context.Context, query rolloutQueryer, accountID string) (RolloutConfigView, error) {
	var view RolloutConfigView
	var hoursJSON []byte
	err := query.QueryRow(ctx, `select mode,allowed_instance_ids::text[],
		allowed_instagram_account_ids::text[],allowed_queue_ids::text[],auto_reply_percent,
		allowed_hours,excluded_tags,max_daily_auto_replies,kill_switch_reason,revision,
		updated_by_user_id::text,updated_at
		from messaging.rollout_configs where account_id=$1::uuid`, accountID).
		Scan(&view.Mode, &view.AllowedInstanceIDs, &view.AllowedInstagramAccountIDs,
			&view.AllowedQueueIDs, &view.AutoReplyPercent, &hoursJSON, &view.ExcludedTags,
			&view.MaxDailyAutoReplies, &view.KillSwitchReason, &view.Revision,
			&view.UpdatedByUserID, &view.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return defaultRolloutConfig(), nil
	}
	if err != nil {
		return RolloutConfigView{}, err
	}
	if err := json.Unmarshal(hoursJSON, &view.AllowedHours); err != nil {
		return RolloutConfigView{}, err
	}
	view.LegacyDefault = false
	normalizeRolloutView(&view)
	return view, nil
}

func normalizeRolloutView(view *RolloutConfigView) {
	if view.AllowedInstanceIDs == nil {
		view.AllowedInstanceIDs = []string{}
	}
	if view.AllowedInstagramAccountIDs == nil {
		view.AllowedInstagramAccountIDs = []string{}
	}
	if view.AllowedQueueIDs == nil {
		view.AllowedQueueIDs = []string{}
	}
	if view.ExcludedTags == nil {
		view.ExcludedTags = []string{}
	}
	if view.AllowedHours.Windows == nil {
		view.AllowedHours.Windows = []RolloutWindow{}
	}
}

func (s *Store) ValidateRolloutScope(ctx context.Context, accountID string, in RolloutConfigInput) (bool, error) {
	var instancesOK, instagramOK, queuesOK bool
	err := s.pool.QueryRow(ctx, `select
		not exists (select unnest($2::uuid[]) except select id from messaging.whatsapp_instances where account_id=$1::uuid),
		not exists (select unnest($3::uuid[]) except select id from messaging.instagram_accounts where account_id=$1::uuid),
		not exists (select unnest($4::uuid[]) except select id from messaging.queues where account_id=$1::uuid)`,
		accountID, in.AllowedInstanceIDs, in.AllowedInstagramAccountIDs, in.AllowedQueueIDs).
		Scan(&instancesOK, &instagramOK, &queuesOK)
	return instancesOK && instagramOK && queuesOK, err
}

func (s *Store) UpdateRolloutConfig(ctx context.Context, accountID, userID string, in RolloutConfigInput) (RolloutConfigView, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return RolloutConfigView{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	current, err := loadRolloutConfig(ctx, tx, accountID)
	if err != nil {
		return RolloutConfigView{}, err
	}
	if current.Revision != in.ExpectedRevision {
		return RolloutConfigView{}, ErrRolloutRevisionConflict
	}
	hoursJSON, err := json.Marshal(in.AllowedHours)
	if err != nil {
		return RolloutConfigView{}, err
	}
	var updated RolloutConfigView
	var updatedHours []byte
	err = tx.QueryRow(ctx, `insert into messaging.rollout_configs
		(account_id,mode,allowed_instance_ids,allowed_instagram_account_ids,allowed_queue_ids,
		 auto_reply_percent,allowed_hours,excluded_tags,max_daily_auto_replies,kill_switch_reason,
		 revision,updated_by_user_id)
		values ($1::uuid,$2,$3::uuid[],$4::uuid[],$5::uuid[],$6,$7::jsonb,$8::text[],$9,
		 nullif($10,''),1,nullif($11,'')::uuid)
		on conflict (account_id) do update set
		 mode=excluded.mode,allowed_instance_ids=excluded.allowed_instance_ids,
		 allowed_instagram_account_ids=excluded.allowed_instagram_account_ids,
		 allowed_queue_ids=excluded.allowed_queue_ids,auto_reply_percent=excluded.auto_reply_percent,
		 allowed_hours=excluded.allowed_hours,excluded_tags=excluded.excluded_tags,
		 max_daily_auto_replies=excluded.max_daily_auto_replies,
		 kill_switch_reason=excluded.kill_switch_reason,
		 revision=messaging.rollout_configs.revision+1,updated_by_user_id=excluded.updated_by_user_id,
		 updated_at=now()
		returning mode,allowed_instance_ids::text[],allowed_instagram_account_ids::text[],
		 allowed_queue_ids::text[],auto_reply_percent,allowed_hours,excluded_tags,
		 max_daily_auto_replies,kill_switch_reason,revision,updated_by_user_id::text,updated_at`,
		accountID, in.Mode, in.AllowedInstanceIDs, in.AllowedInstagramAccountIDs,
		in.AllowedQueueIDs, in.AutoReplyPercent, string(hoursJSON), in.ExcludedTags,
		in.MaxDailyAutoReplies, rolloutStringValue(in.KillSwitchReason), userID).
		Scan(&updated.Mode, &updated.AllowedInstanceIDs, &updated.AllowedInstagramAccountIDs,
			&updated.AllowedQueueIDs, &updated.AutoReplyPercent, &updatedHours,
			&updated.ExcludedTags, &updated.MaxDailyAutoReplies, &updated.KillSwitchReason,
			&updated.Revision, &updated.UpdatedByUserID, &updated.UpdatedAt)
	if err != nil {
		return RolloutConfigView{}, err
	}
	if err := json.Unmarshal(updatedHours, &updated.AllowedHours); err != nil {
		return RolloutConfigView{}, err
	}
	updated.LegacyDefault = false
	normalizeRolloutView(&updated)
	beforeJSON, _ := json.Marshal(current)
	afterJSON, _ := json.Marshal(updated)
	if _, err := tx.Exec(ctx, `insert into messaging.rollout_changes
		(account_id,actor_user_id,before_config,after_config,reason)
		values ($1::uuid,nullif($2,'')::uuid,$3::jsonb,$4::jsonb,$5)`,
		accountID, userID, string(beforeJSON), string(afterJSON), in.Reason); err != nil {
		return RolloutConfigView{}, err
	}
	if updated.Mode != RolloutModeAssist {
		if _, err := tx.Exec(ctx, `update messaging.ai_reply_drafts
			set status='expired',decision_reason='rollout_'||$2,decided_at=now(),updated_at=now()
			where account_id=$1::uuid and status='pending'`, accountID, updated.Mode); err != nil {
			return RolloutConfigView{}, err
		}
	}
	if rolloutStopsInference(updated.Mode) {
		if err := stopAccountAutomationTx(ctx, tx, accountID, "rollout_"+updated.Mode); err != nil {
			return RolloutConfigView{}, err
		}
	} else if updated.Mode == RolloutModeShadow || updated.Mode == RolloutModeAssist {
		if err := cancelAccountAIOutboundTx(ctx, tx, accountID, "rollout_"+updated.Mode); err != nil {
			return RolloutConfigView{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return RolloutConfigView{}, err
	}
	return updated, nil
}

func rolloutStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func rolloutStopsInference(mode string) bool {
	return mode == RolloutModeOff || mode == RolloutModeObserve || mode == RolloutModePaused
}

func stopAccountAutomationTx(ctx context.Context, tx pgx.Tx, accountID, reason string) error {
	if _, err := tx.Exec(ctx, `update messaging.conversations
		set state=case when state='ai_active' then 'routing' else state end,
		    ai_generation=ai_generation+1,updated_at=now()
		where account_id=$1::uuid and state='ai_active'`, accountID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `update messaging.ai_dispatches
		set status='cancelled',last_error=$2,locked_at=null,updated_at=now()
		where account_id=$1::uuid and status in ('buffering','queued','processing')`, accountID, reason); err != nil {
		return err
	}
	return cancelAccountAIOutboundTx(ctx, tx, accountID, reason)
}

func cancelAccountAIOutboundTx(ctx context.Context, tx pgx.Tx, accountID, reason string) error {
	if _, err := tx.Exec(ctx, `update messaging.outbox outbox
		set status='dead',last_error=$2,locked_at=null,locked_by='',updated_at=now()
		from messaging.messages message
		where outbox.account_id=$1::uuid and outbox.kind=$3
		  and outbox.status in ('pending','processing')
		  and message.account_id=outbox.account_id and message.origin='ai'
		  and message.status='PENDING' and outbox.payload->>'messageId'=message.id::text`,
		accountID, reason, OutboundJobKind); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `update messaging.messages set status='FAILED',provider_error_code=$2,updated_at=now()
		where account_id=$1::uuid and origin='ai' and status='PENDING'`, accountID, reason)
	return err
}

func (s *Store) EvaluateAIRollout(ctx context.Context, accountID, conversationID string) (RolloutDecision, error) {
	return evaluateAIRollout(ctx, s.pool, accountID, conversationID, time.Now().UTC())
}

func evaluateAIRollout(ctx context.Context, query rolloutQueryer, accountID, conversationID string, now time.Time) (RolloutDecision, error) {
	config, err := loadRolloutConfig(ctx, query, accountID)
	if err != nil {
		return RolloutDecision{}, err
	}
	decision := RolloutDecision{Mode: config.Mode, RunAI: true, AutoSend: true, Reason: "allowed"}
	switch config.Mode {
	case RolloutModeOff, RolloutModeObserve, RolloutModePaused:
		decision.RunAI, decision.AutoSend, decision.Reason = false, false, "mode_"+config.Mode
		return decision, nil
	case RolloutModeShadow, RolloutModeAssist:
		decision.AutoSend, decision.Reason = false, "mode_"+config.Mode
		return decision, nil
	case RolloutModeActive:
		return decision, nil
	case RolloutModeAutoPilot:
		decision.AutoSend = false
	default:
		decision.RunAI, decision.AutoSend, decision.Reason = false, false, "invalid_mode"
		return decision, nil
	}

	var scope rolloutConversationContext
	var rawTags []byte
	err = query.QueryRow(ctx, `select coalesce(conversation.instance_id::text,''),
		coalesce(conversation.queue_id::text,''),coalesce(contact.tags,'[]'::jsonb)
		from messaging.conversations conversation
		left join messaging.contacts contact on contact.account_id=conversation.account_id
		 and contact.id=conversation.contact_id
		where conversation.account_id=$1::uuid and conversation.id=$2::uuid`, accountID, conversationID).
		Scan(&scope.InstanceID, &scope.QueueID, &rawTags)
	if err != nil {
		return RolloutDecision{}, err
	}
	_ = json.Unmarshal(rawTags, &scope.Tags)
	if !containsOrUnrestricted(config.AllowedInstanceIDs, scope.InstanceID) {
		decision.Reason = "instance_not_allowed"
		return decision, nil
	}
	if !containsOrUnrestricted(config.AllowedQueueIDs, scope.QueueID) {
		decision.Reason = "queue_not_allowed"
		return decision, nil
	}
	if intersectsFold(config.ExcludedTags, scope.Tags) {
		decision.Reason = "tag_excluded"
		return decision, nil
	}
	if !rolloutHourAllowed(config.AllowedHours, now) {
		decision.Reason = "outside_allowed_hours"
		return decision, nil
	}
	if rolloutBucket(accountID, conversationID) >= config.AutoReplyPercent {
		decision.Reason = "outside_percentage"
		return decision, nil
	}
	if config.MaxDailyAutoReplies > 0 {
		start := rolloutLocalDayStart(config.AllowedHours.Timezone, now)
		var sent int
		if err := query.QueryRow(ctx, `select count(*) from messaging.messages
			where account_id=$1::uuid and origin='ai' and direction='OUTBOUND'
			  and status <> 'FAILED' and created_at >= $2`, accountID, start).Scan(&sent); err != nil {
			return RolloutDecision{}, err
		}
		if sent >= config.MaxDailyAutoReplies {
			decision.Reason = "daily_limit_reached"
			return decision, nil
		}
	}
	decision.AutoSend, decision.Reason = true, "cohort_allowed"
	return decision, nil
}

func containsOrUnrestricted(values []string, candidate string) bool {
	if len(values) == 0 {
		return true
	}
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func intersectsFold(excluded, actual []string) bool {
	set := make(map[string]struct{}, len(excluded))
	for _, value := range excluded {
		set[strings.ToLower(strings.TrimSpace(value))] = struct{}{}
	}
	for _, value := range actual {
		if _, ok := set[strings.ToLower(strings.TrimSpace(value))]; ok {
			return true
		}
	}
	return false
}

func rolloutBucket(accountID, conversationID string) int {
	sum := sha256.Sum256([]byte(accountID + ":" + conversationID))
	return int(binary.BigEndian.Uint64(sum[:8]) % 100)
}

func rolloutLocalDayStart(timezone string, now time.Time) time.Time {
	location, err := time.LoadLocation(strings.TrimSpace(timezone))
	if err != nil {
		location = time.UTC
	}
	local := now.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location).UTC()
}

func rolloutHourAllowed(hours RolloutHours, now time.Time) bool {
	if len(hours.Windows) == 0 {
		return true
	}
	location, err := time.LoadLocation(strings.TrimSpace(hours.Timezone))
	if err != nil {
		return false
	}
	local := now.In(location)
	minute := local.Hour()*60 + local.Minute()
	day := int(local.Weekday())
	for _, window := range hours.Windows {
		if len(window.Days) > 0 && !containsInt(window.Days, day) {
			continue
		}
		start, startOK := parseClockMinute(window.Start)
		end, endOK := parseClockMinute(window.End)
		if startOK && endOK && start < end && minute >= start && minute < end {
			return true
		}
	}
	return false
}

func containsInt(values []int, candidate int) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func parseClockMinute(value string) (int, bool) {
	parsed, err := time.Parse("15:04", strings.TrimSpace(value))
	if err != nil {
		return 0, false
	}
	return parsed.Hour()*60 + parsed.Minute(), true
}

func normalizedRolloutIDs(values []string) ([]string, bool) {
	if len(values) > 200 {
		return nil, false
	}
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if !omnichannelUUIDPattern.MatchString(value) {
			return nil, false
		}
		set[value] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out, true
}
