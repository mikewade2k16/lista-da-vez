package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

// Conditions de handoff são deliberadamente menores que uma expressão livre.
// Isso evita que uma policy administrativa vire código executável ou uma regra
// criada pelo modelo. Valores são comparados com igualdade, exceto os limites
// de confiança e a janela hourUtc.
var handoffPolicyConditionKeys = map[string]struct{}{
	"reasonCode":         {},
	"sourceState":        {},
	"departmentId":       {},
	"channel":            {},
	"instanceScopeKey":   {},
	"intent":             {},
	"relationshipStatus": {},
	"lifecycle":          {},
	"tag":                {},
	"slaRisk":            {},
	"confidenceMax":      {},
	"confidenceMin":      {},
	"hourUtc":            {},
}

type handoffPolicyRecord struct {
	ID                     string
	Name                   string
	Priority               int
	IsActive               bool
	Conditions             map[string]any
	TargetQueueID          *string
	FallbackQueueID        *string
	CustomerNoticeTemplate string
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

func (p handoffPolicyRecord) Snapshot() map[string]any {
	return map[string]any{
		"id": p.ID, "name": p.Name, "priority": p.Priority,
		"conditions": p.Conditions, "targetQueueId": p.TargetQueueID,
		"fallbackQueueId":        p.FallbackQueueID,
		"customerNoticeTemplate": p.CustomerNoticeTemplate,
	}
}

func (p handoffPolicyRecord) View() HandoffPolicyView {
	return HandoffPolicyView(p)
}

func scanHandoffPolicy(row rowScanner) (handoffPolicyRecord, error) {
	var p handoffPolicyRecord
	var raw []byte
	err := row.Scan(&p.ID, &p.Name, &p.Priority, &p.IsActive, &raw, &p.TargetQueueID,
		&p.FallbackQueueID, &p.CustomerNoticeTemplate, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return handoffPolicyRecord{}, err
	}
	p.Conditions = map[string]any{}
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &p.Conditions); err != nil || p.Conditions == nil {
			return handoffPolicyRecord{}, ErrInvalidBody
		}
	}
	return p, nil
}

const handoffPolicyColumns = `id::text, name, priority, is_active, conditions,
	target_queue_id::text, fallback_queue_id::text, customer_notice_template, created_at, updated_at`

func (s *Store) ListHandoffPolicies(ctx context.Context, accountID string) ([]HandoffPolicyView, error) {
	rows, err := s.pool.Query(ctx, `select `+handoffPolicyColumns+` from messaging.handoff_policies
		where account_id=$1::uuid order by priority asc, id asc`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]HandoffPolicyView, 0)
	for rows.Next() {
		p, err := scanHandoffPolicy(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p.View())
	}
	return out, rows.Err()
}

func (s *Store) CreateHandoffPolicy(ctx context.Context, accountID string, in HandoffPolicyInput, active bool) (HandoffPolicyView, error) {
	raw, err := json.Marshal(in.Conditions)
	if err != nil {
		return HandoffPolicyView{}, err
	}
	p, err := scanHandoffPolicy(s.pool.QueryRow(ctx, `insert into messaging.handoff_policies
		(account_id,name,priority,is_active,conditions,target_queue_id,fallback_queue_id,customer_notice_template)
		values ($1::uuid,$2,$3,$4,$5::jsonb,$6::uuid,$7::uuid,$8)
		returning `+handoffPolicyColumns, accountID, in.Name, in.Priority, active, raw,
		in.TargetQueueID, in.FallbackQueueID, in.CustomerNoticeTemplate))
	return p.View(), err
}

func (s *Store) UpdateHandoffPolicy(ctx context.Context, accountID, id string, patch HandoffPolicyPatch) (HandoffPolicyView, error) {
	var raw []byte
	var err error
	if patch.Conditions != nil {
		raw, err = json.Marshal(*patch.Conditions)
		if err != nil {
			return HandoffPolicyView{}, err
		}
	}
	p, err := scanHandoffPolicy(s.pool.QueryRow(ctx, `update messaging.handoff_policies set
		name=coalesce($3,name), priority=coalesce($4,priority), is_active=coalesce($5,is_active),
		conditions=coalesce($6::jsonb,conditions), target_queue_id=coalesce($7::uuid,target_queue_id),
		fallback_queue_id=coalesce($8::uuid,fallback_queue_id),
		customer_notice_template=coalesce($9,customer_notice_template), updated_at=now()
		where account_id=$1::uuid and id=$2::uuid returning `+handoffPolicyColumns,
		accountID, id, patch.Name, patch.Priority, patch.IsActive, nullableJSON(raw),
		patch.TargetQueueID, patch.FallbackQueueID, patch.CustomerNoticeTemplate))
	return p.View(), err
}

func (s *Store) DisableHandoffPolicy(ctx context.Context, accountID, id string) error {
	tag, err := s.pool.Exec(ctx, `update messaging.handoff_policies
		set is_active=false, updated_at=now() where account_id=$1::uuid and id=$2::uuid`, accountID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func nullableJSON(raw []byte) any {
	if len(raw) == 0 {
		return nil
	}
	return raw
}

// normalizeHandoffPolicyConditions rejeita chaves desconhecidas e estruturas
// profundas. A API aceita arrays para igualdade "qualquer um" e limites
// numéricos explícitos; não aceita SQL, regex ou operadores arbitrários.
func normalizeHandoffPolicyConditions(in map[string]any) (map[string]any, error) {
	if in == nil {
		in = map[string]any{}
	}
	if len(in) > 16 {
		return nil, ErrValidation
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		key = strings.TrimSpace(key)
		if _, ok := handoffPolicyConditionKeys[key]; !ok {
			return nil, ErrValidation
		}
		lowerKey := strings.ToLower(key)
		for _, marker := range []string{"secret", "token", "password", "credential", "apikey", "api_key"} {
			if strings.Contains(lowerKey, marker) {
				return nil, ErrValidation
			}
		}
		switch key {
		case "confidenceMax", "confidenceMin":
			n, ok := numberValue(value)
			if !ok || n < 0 || n > 1 || math.IsNaN(n) {
				return nil, ErrValidation
			}
			out[key] = n
		case "hourUtc":
			window, ok := normalizeHourWindow(value)
			if !ok {
				return nil, ErrValidation
			}
			out[key] = window
		default:
			if !validPolicyScalarOrList(value) {
				return nil, ErrValidation
			}
			out[key] = normalizePolicyScalarOrList(value)
		}
	}
	raw, err := json.Marshal(out)
	if err != nil || len(raw) > 8192 {
		return nil, ErrValidation
	}
	return out, nil
}

func numberValue(value any) (float64, bool) {
	switch n := value.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case json.Number:
		v, err := n.Float64()
		return v, err == nil
	default:
		return 0, false
	}
}

func validPolicyScalarOrList(value any) bool {
	switch v := value.(type) {
	case string, bool:
		return true
	case []any:
		if len(v) > 32 {
			return false
		}
		for _, item := range v {
			if _, ok := item.(string); !ok {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func normalizePolicyScalarOrList(value any) any {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			out = append(out, strings.TrimSpace(item.(string)))
		}
		return out
	default:
		return v
	}
}

func normalizeHourWindow(value any) (map[string]int, bool) {
	obj, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	from, okFrom := numberValue(obj["from"])
	to, okTo := numberValue(obj["to"])
	if !okFrom || !okTo || from < 0 || from > 23 || to < 0 || to > 23 || from != math.Trunc(from) || to != math.Trunc(to) {
		return nil, false
	}
	return map[string]int{"from": int(from), "to": int(to)}, true
}

type handoffPolicyContext struct {
	Values  map[string]any
	Tags    []string
	HourUTC int
}

func policyMatches(conditions map[string]any, c handoffPolicyContext) bool {
	for key, wanted := range conditions {
		if key == "confidenceMax" || key == "confidenceMin" {
			actual, ok := numberValue(c.Values["confidence"])
			limit, valid := numberValue(wanted)
			if !ok || !valid || (key == "confidenceMax" && actual > limit) || (key == "confidenceMin" && actual < limit) {
				return false
			}
			continue
		}
		if key == "hourUtc" {
			window, ok := wanted.(map[string]any)
			if !ok {
				if typed, okTyped := wanted.(map[string]int); okTyped {
					from, fromOK := typed["from"]
					to, toOK := typed["to"]
					ok = fromOK && toOK
					if ok && !hourInWindow(c.HourUTC, from, to) {
						return false
					}
					continue
				}
				return false
			}
			from, fromOK := numberValue(window["from"])
			to, toOK := numberValue(window["to"])
			if !fromOK || !toOK || !hourInWindow(c.HourUTC, int(from), int(to)) {
				return false
			}
			continue
		}
		if key == "tag" {
			if !matchPolicyTag(wanted, c.Tags) {
				return false
			}
			continue
		}
		actual, present := c.Values[key]
		if !present || !matchPolicyValue(actual, wanted) {
			return false
		}
	}
	return true
}

func hourInWindow(hour, from, to int) bool {
	if from <= to {
		return hour >= from && hour <= to
	}
	return hour >= from || hour <= to
}

func matchPolicyTag(wanted any, tags []string) bool {
	values := []string{}
	switch v := wanted.(type) {
	case string:
		values = []string{v}
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				values = append(values, s)
			}
		}
	case []string:
		values = v
	default:
		return false
	}
	for _, want := range values {
		for _, tag := range tags {
			if strings.EqualFold(strings.TrimSpace(want), strings.TrimSpace(tag)) {
				return true
			}
		}
	}
	return false
}

func matchPolicyValue(actual, wanted any) bool {
	if list, ok := wanted.([]any); ok {
		for _, item := range list {
			if strings.EqualFold(fmt.Sprint(actual), fmt.Sprint(item)) {
				return true
			}
		}
		return false
	}
	if list, ok := wanted.([]string); ok {
		for _, item := range list {
			if strings.EqualFold(fmt.Sprint(actual), item) {
				return true
			}
		}
		return false
	}
	return strings.EqualFold(fmt.Sprint(actual), fmt.Sprint(wanted))
}

func (s *Store) selectHandoffPolicyTx(ctx context.Context, tx pgx.Tx, accountID string, snap convSnapshot, in HandoffRequest) (handoffPolicyRecord, bool, error) {
	rows, err := tx.Query(ctx, `select `+handoffPolicyColumns+` from messaging.handoff_policies
		where account_id=$1::uuid and is_active order by priority asc, id asc`, accountID)
	if err != nil {
		return handoffPolicyRecord{}, false, err
	}
	policies := make([]handoffPolicyRecord, 0)
	for rows.Next() {
		p, scanErr := scanHandoffPolicy(rows)
		if scanErr != nil {
			rows.Close()
			return handoffPolicyRecord{}, false, scanErr
		}
		policies = append(policies, p)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		rows.Close()
		return handoffPolicyRecord{}, false, rowsErr
	}
	// pgx does not allow another query on the same transaction connection while
	// a Rows cursor is open. Materialize policies before loading contact context.
	rows.Close()
	ctxValues := map[string]any{
		"reasonCode": in.ReasonCode, "sourceState": string(snap.State),
		"departmentId": valueOrEmpty(snap.DepartmentID), "channel": snap.Channel,
		"instanceScopeKey": snap.InstanceScopeKey,
	}
	for key, value := range decodeObject(snap.ExtractedFields) {
		ctxValues[key] = value
	}
	ctxValues["confidence"] = firstNumber(ctxValues["confidence"], 0)
	contactTags := []string{}
	if snap.ContactID != nil {
		var relationship string
		var confidence *float64
		var tagsRaw []byte
		if err := tx.QueryRow(ctx, `select relationship_status, classification_confidence, coalesce(tags,'[]'::jsonb)
			from messaging.contacts where account_id=$1::uuid and id=$2::uuid`, accountID, *snap.ContactID).Scan(&relationship, &confidence, &tagsRaw); err == nil {
			ctxValues["relationshipStatus"] = relationship
			ctxValues["lifecycle"] = relationship
			if confidence != nil {
				ctxValues["confidence"] = *confidence
			}
			_ = json.Unmarshal(tagsRaw, &contactTags)
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return handoffPolicyRecord{}, false, err
		}
	}
	ctxValues["slaRisk"] = firstNonEmptyString(ctxValues["slaRisk"], "none")
	policyCtx := handoffPolicyContext{Values: ctxValues, Tags: contactTags, HourUTC: time.Now().UTC().Hour()}
	for _, p := range policies {
		if policyMatches(p.Conditions, policyCtx) {
			return p, true, nil
		}
	}
	return handoffPolicyRecord{}, false, nil
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func decodeObject(raw json.RawMessage) map[string]any {
	result := map[string]any{}
	if len(raw) > 0 && string(raw) != "null" {
		_ = json.Unmarshal(raw, &result)
	}
	return result
}

func firstNumber(value any, fallback float64) float64 {
	if n, ok := numberValue(value); ok {
		return n
	}
	return fallback
}
func firstNonEmptyString(value any, fallback string) string {
	if s, ok := value.(string); ok && strings.TrimSpace(s) != "" {
		return s
	}
	return fallback
}

func resolvePolicyQueueTx(ctx context.Context, tx pgx.Tx, accountID string, p handoffPolicyRecord) (*string, *string, error) {
	for _, candidate := range []*string{p.TargetQueueID, p.FallbackQueueID} {
		if candidate == nil {
			continue
		}
		var queueID, departmentID string
		err := tx.QueryRow(ctx, `select q.id::text, q.department_id::text from messaging.queues q
			where q.account_id=$1::uuid and q.id=$2::uuid and q.is_active`, accountID, *candidate).Scan(&queueID, &departmentID)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		return &queueID, &departmentID, nil
	}
	return nil, nil, nil
}

// Service API for the settings panel. Policies are administrative data and
// therefore require the same permission as queues/routing rules.
func (s *Service) ListHandoffPolicies(ctx context.Context, accountID string, p auth.Principal) ([]HandoffPolicyView, error) {
	if err := s.requireSettingsManage(ctx, accountID, p); err != nil {
		return nil, err
	}
	return s.store.ListHandoffPolicies(ctx, accountID)
}

func (s *Service) CreateHandoffPolicy(ctx context.Context, accountID string, p auth.Principal, in HandoffPolicyInput) (HandoffPolicyView, error) {
	if err := s.requireSettingsManage(ctx, accountID, p); err != nil {
		return HandoffPolicyView{}, err
	}
	normalized, err := normalizeHandoffPolicyInput(&in)
	if err != nil {
		return HandoffPolicyView{}, err
	}
	for _, queueID := range []*string{in.TargetQueueID, in.FallbackQueueID} {
		if queueID != nil {
			if err := s.assertActiveQueue(ctx, accountID, *queueID); err != nil {
				return HandoffPolicyView{}, translate(err)
			}
		}
	}
	out, err := s.store.CreateHandoffPolicy(ctx, accountID, in, normalized)
	if isUniqueViolation(err) {
		return HandoffPolicyView{}, ErrConflict
	}
	return out, err
}

func (s *Service) UpdateHandoffPolicy(ctx context.Context, accountID string, p auth.Principal, id string, patch HandoffPolicyPatch) (HandoffPolicyView, error) {
	if err := s.requireSettingsManage(ctx, accountID, p); err != nil {
		return HandoffPolicyView{}, err
	}
	if !omnichannelUUIDPattern.MatchString(strings.TrimSpace(id)) {
		return HandoffPolicyView{}, ErrValidation
	}
	if err := normalizeHandoffPolicyPatch(&patch); err != nil {
		return HandoffPolicyView{}, err
	}
	for _, queueID := range []*string{patch.TargetQueueID, patch.FallbackQueueID} {
		if queueID != nil {
			if err := s.assertActiveQueue(ctx, accountID, *queueID); err != nil {
				return HandoffPolicyView{}, translate(err)
			}
		}
	}
	out, err := s.store.UpdateHandoffPolicy(ctx, accountID, id, patch)
	if isUniqueViolation(err) {
		return HandoffPolicyView{}, ErrConflict
	}
	return out, translate(err)
}

func (s *Service) DeleteHandoffPolicy(ctx context.Context, accountID string, p auth.Principal, id string) error {
	if err := s.requireSettingsManage(ctx, accountID, p); err != nil {
		return err
	}
	if !omnichannelUUIDPattern.MatchString(strings.TrimSpace(id)) {
		return ErrValidation
	}
	return s.store.DisableHandoffPolicy(ctx, accountID, id)
}

func normalizeHandoffPolicyInput(in *HandoffPolicyInput) (bool, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || len([]rune(in.Name)) > 200 || in.Priority < 0 || in.Priority > 100000 || len([]rune(in.CustomerNoticeTemplate)) > 2000 {
		return false, ErrValidation
	}
	conditions, err := normalizeHandoffPolicyConditions(in.Conditions)
	if err != nil {
		return false, err
	}
	in.Conditions = conditions
	if err := validatePolicyQueues(in.TargetQueueID, in.FallbackQueueID); err != nil {
		return false, err
	}
	active := true
	if in.IsActive != nil {
		active = *in.IsActive
	}
	return active, nil
}

func normalizeHandoffPolicyPatch(patch *HandoffPolicyPatch) error {
	if patch.Name != nil {
		value := strings.TrimSpace(*patch.Name)
		if value == "" || len([]rune(value)) > 200 {
			return ErrValidation
		}
		patch.Name = &value
	}
	if patch.Priority != nil && (*patch.Priority < 0 || *patch.Priority > 100000) {
		return ErrValidation
	}
	if patch.CustomerNoticeTemplate != nil && len([]rune(*patch.CustomerNoticeTemplate)) > 2000 {
		return ErrValidation
	}
	if patch.Conditions != nil {
		normalized, err := normalizeHandoffPolicyConditions(*patch.Conditions)
		if err != nil {
			return err
		}
		patch.Conditions = &normalized
	}
	return validatePolicyQueues(patch.TargetQueueID, patch.FallbackQueueID)
}

func validatePolicyQueues(target, fallback *string) error {
	for _, value := range []*string{target, fallback} {
		if value != nil {
			trimmed := strings.TrimSpace(*value)
			if !omnichannelUUIDPattern.MatchString(trimmed) {
				return ErrValidation
			}
			*value = trimmed
		}
	}
	return nil
}
