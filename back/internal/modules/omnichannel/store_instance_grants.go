package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	ErrInstanceAccessRevisionConflict = errors.New("omnichannel: instance access revision conflict")
	ErrLastInstanceManager            = errors.New("omnichannel: instance requires an active manager")
)

type InstanceGrantInput struct {
	UserID      string             `json:"userId"`
	AccessLevel InstanceGrantLevel `json:"accessLevel"`
}

type InstanceAccessWrite struct {
	AccountID         string
	InstanceID        string
	ActorUserID       string
	ResponsibleUserID string
	AccessPolicy      InstanceAccessPolicy
	ExpectedRevision  int64
	Grants            []InstanceGrantInput
}

type InstanceAccessWriteResult struct {
	AccessRevision    int64
	ResponsibleUserID string
	Changed           bool
}

type storedInstanceAccess struct {
	AccessRevision    int64
	AccessPolicy      InstanceAccessPolicy
	ResponsibleUserID *string
	Grants            []storedInstanceGrant
}

type InstanceAccessBackfillReport struct {
	TotalInstances int64
	Shared         int64
	Restricted     int64
	WithoutManage  int64
	ViewGrants     int64
	ReplyGrants    int64
	ManageGrants   int64
	IgnoredUsers   int64
	Issues         []InstanceAccessBackfillIssue
}

type InstanceAccessBackfillIssue struct {
	AccountID    string
	InstanceID   string
	InstanceName string
	Source       string
	RawUserID    string
	Reason       string
}

type storedInstanceGrant struct {
	UserID      string             `json:"userId"`
	AccessLevel InstanceGrantLevel `json:"accessLevel"`
	IsActive    bool               `json:"isActive"`
	Revision    int64              `json:"revision"`
}

// GetInstanceAccessState le policy, revision, responsavel e a trilha completa de grants
// da conexao. O filtro de account e repetido em todas as consultas para impedir mistura
// cross-tenant mesmo quando o id da instancia for conhecido.
func (s *Store) GetInstanceAccessState(ctx context.Context, accountID, instanceID string) (storedInstanceAccess, error) {
	accountID = strings.TrimSpace(accountID)
	instanceID = strings.TrimSpace(instanceID)
	if accountID == "" || instanceID == "" {
		return storedInstanceAccess{}, ErrNotFound
	}

	var state storedInstanceAccess
	var policy string
	err := s.pool.QueryRow(ctx, `select access_policy,access_revision,responsible_user_id::text
		from messaging.whatsapp_instances
		where account_id=$1::uuid and id=$2::uuid`, accountID, instanceID).
		Scan(&policy, &state.AccessRevision, &state.ResponsibleUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return storedInstanceAccess{}, ErrNotFound
	}
	if err != nil {
		return storedInstanceAccess{}, err
	}
	state.AccessPolicy = InstanceAccessPolicy(policy)

	rows, err := s.pool.Query(ctx, `select user_id::text,access_level,is_active,revision
		from messaging.whatsapp_instance_user_grants
		where account_id=$1::uuid and instance_id=$2::uuid
		order by user_id`, accountID, instanceID)
	if err != nil {
		return storedInstanceAccess{}, err
	}
	defer rows.Close()
	state.Grants = make([]storedInstanceGrant, 0)
	for rows.Next() {
		var grant storedInstanceGrant
		var level string
		if err := rows.Scan(&grant.UserID, &level, &grant.IsActive, &grant.Revision); err != nil {
			return storedInstanceAccess{}, err
		}
		grant.AccessLevel = InstanceGrantLevel(level)
		state.Grants = append(state.Grants, grant)
	}
	if err := rows.Err(); err != nil {
		return storedInstanceAccess{}, err
	}
	return state, nil
}

// LoadConversationAccessScope calcula o resolver relacional do P1. Durante o P1A ele roda em
// shadow mode: produz a decisao canônica sem substituir ainda os filtros REST do P1B.
func (s *Store) LoadConversationAccessScope(ctx context.Context, accountID, userID string) (ConversationAccessScope, error) {
	scope := ConversationAccessScope{
		AccountID: strings.TrimSpace(accountID),
		UserID:    strings.TrimSpace(userID),
		Instances: map[string]InstanceAccessDecision{},
	}
	if scope.AccountID == "" || scope.UserID == "" {
		scope.Reason = "principal_scope_missing"
		return scope, nil
	}

	var accountActive, membershipActive, moduleEnabled bool
	err := s.pool.QueryRow(ctx, `select a.is_active,
		exists(select 1 from core.account_users au
			where au.account_id=a.id and au.user_id=$2::uuid and au.is_active),
		exists(select 1 from core.account_modules am
			where am.account_id=a.id and am.module_id='omnichannel' and am.enabled)
		from core.accounts a where a.id=$1::uuid`, scope.AccountID, scope.UserID).
		Scan(&accountActive, &membershipActive, &moduleEnabled)
	if errors.Is(err, pgx.ErrNoRows) {
		scope.Reason = "account_not_found"
		return scope, nil
	}
	if err != nil {
		return ConversationAccessScope{}, err
	}
	switch {
	case !accountActive:
		scope.Reason = "account_inactive"
		return scope, nil
	case !membershipActive:
		scope.Reason = "membership_inactive"
		return scope, nil
	case !moduleEnabled:
		scope.Reason = "module_disabled"
		return scope, nil
	}

	permissions := instanceFeaturePermissions{}
	checks := []struct {
		key    string
		target *bool
	}{
		{key: "omnichannel.conversations.view", target: &permissions.View},
		{key: "omnichannel.conversations.reply", target: &permissions.Reply},
		{key: "omnichannel.conversations.assign", target: &permissions.Assign},
		{key: "omnichannel.conversations.close", target: &permissions.Close},
		{key: "omnichannel.instances.manage", target: &permissions.Manage},
		{key: conversationPrivacyManagePermission, target: &permissions.ResetHistory},
		{key: "omnichannel.contacts.manage", target: &permissions.Contacts},
		{key: "omnichannel.settings.manage", target: &permissions.Settings},
		{key: "omnichannel.agents.manage", target: &permissions.Agents},
		{key: "omnichannel.audit.view", target: &permissions.Audit},
	}
	for _, check := range checks {
		allowed, permissionErr := s.hasEffectivePermission(ctx, scope.AccountID, scope.UserID, check.key)
		if permissionErr != nil {
			return ConversationAccessScope{}, permissionErr
		}
		*check.target = allowed
	}

	rows, err := s.pool.Query(ctx, `select wi.id::text, wi.instance_name, wi.access_policy,
		coalesce(grant_row.access_level, ''), wi.is_active
		from messaging.whatsapp_instances wi
		left join messaging.whatsapp_instance_user_grants grant_row
		  on grant_row.account_id=wi.account_id and grant_row.instance_id=wi.id
		 and grant_row.user_id=$2::uuid and grant_row.is_active
		where wi.account_id=$1::uuid
		order by wi.instance_name`, scope.AccountID, scope.UserID)
	if err != nil {
		return ConversationAccessScope{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var decision InstanceAccessDecision
		var policy, grant string
		if err := rows.Scan(&decision.InstanceID, &decision.InstanceName, &policy, &grant, &decision.IsActive); err != nil {
			return ConversationAccessScope{}, err
		}
		decision.Policy = InstanceAccessPolicy(policy)
		decision.GrantLevel = InstanceGrantLevel(grant)
		decision.Capabilities, decision.Reason = resolveInstanceCapabilities(decision.Policy, decision.GrantLevel, permissions)
		scope.Instances[decision.InstanceID] = decision
	}
	if err := rows.Err(); err != nil {
		return ConversationAccessScope{}, err
	}
	scope.Eligible = true
	scope.Reason = "resolved"
	scope.features = permissions
	return scope, nil
}

// RequireInstanceAccess aplica os gates cumulativos de conta, membership, modulo,
// permissao da feature e grant relacional. Recurso fora do grant retorna 404.
func (s *Store) RequireInstanceAccess(ctx context.Context, accountID, userID, instanceID, permissionKey string, required InstanceGrantLevel) (InstanceAccessDecision, error) {
	scope, err := s.LoadConversationAccessScope(ctx, accountID, userID)
	if err != nil {
		return InstanceAccessDecision{}, err
	}
	if !scope.Eligible || !scope.allowsPermission(permissionKey) {
		return InstanceAccessDecision{}, ErrForbidden
	}
	decision, ok := scope.instanceDecision(instanceID)
	if !ok || !instanceCapabilityAllows(decision.Capabilities, required) {
		return InstanceAccessDecision{}, ErrNotFound
	}
	return decision, nil
}

// HasActiveOmnichannelMembership valida o primeiro gate sem depender do middleware: conta,
// membership e modulo precisam estar ativos. Escritas sensiveis usam isto antes da permissao.
func (s *Store) HasActiveOmnichannelMembership(ctx context.Context, accountID, userID string) (bool, error) {
	accountID = strings.TrimSpace(accountID)
	userID = strings.TrimSpace(userID)
	if accountID == "" || userID == "" {
		return false, nil
	}
	var ok bool
	err := s.pool.QueryRow(ctx, `select exists (
		select 1 from core.accounts account_row
		join core.account_users membership on membership.account_id=account_row.id
		join core.account_modules account_module
		  on account_module.account_id=account_row.id and account_module.module_id='omnichannel'
		where account_row.id=$1::uuid and account_row.is_active
		  and membership.user_id=$2::uuid and membership.is_active
		  and account_module.enabled
	)`, accountID, userID).Scan(&ok)
	return ok, err
}

// CreateRestrictedInstanceWithManager grava a instancia e seu primeiro manage na mesma
// transacao. O access_revision nasce em 1 porque a criacao ja contem uma escrita de grant.
func (s *Store) CreateRestrictedInstanceWithManager(ctx context.Context, accountID string, w instanceWrite, createdByUserID string) (string, error) {
	managerUserID := strings.TrimSpace(createdByUserID)
	responsibleUserID := strings.TrimSpace(deref(w.ResponsibleUserID))
	if responsibleUserID == "" {
		responsibleUserID = managerUserID
	}
	if managerUserID == "" {
		return "", ErrInvalidBody
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id string
	err = tx.QueryRow(ctx, `insert into messaging.whatsapp_instances
		(account_id, instance_name, display_name, phone_number, queue_label,
		 responsible_user_id, is_active, provider, provider_config, created_by_user_id,
		 access_policy, access_revision)
		values ($1::uuid, $2, $3, $4, $5, $6::uuid, $7, $8,
		 jsonb_build_object('userScopePolicy', $9::text), $10::uuid, 'RESTRICTED', 1)
		returning id::text`, accountID, w.InstanceName, w.DisplayName, w.PhoneNumber,
		w.QueueLabel, responsibleUserID, w.IsActive, w.Provider, w.UserScopePolicy,
		createdByUserID).Scan(&id)
	if err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `insert into messaging.whatsapp_instance_user_grants
		(account_id,instance_id,user_id,access_level,granted_by_user_id,updated_by_user_id)
		values ($1::uuid,$2::uuid,$3::uuid,'manage',$4::uuid,$4::uuid)`,
		accountID, id, managerUserID, createdByUserID); err != nil {
		return "", err
	}
	if responsibleUserID != managerUserID {
		if _, err := tx.Exec(ctx, `insert into messaging.whatsapp_instance_user_grants
			(account_id,instance_id,user_id,access_level,granted_by_user_id,updated_by_user_id)
			values ($1::uuid,$2::uuid,$3::uuid,'manage',$4::uuid,$4::uuid)`,
			accountID, id, responsibleUserID, createdByUserID); err != nil {
			return "", err
		}
	}
	grants := []map[string]any{{"userId": managerUserID, "accessLevel": InstanceGrantManage, "isActive": true}}
	if responsibleUserID != managerUserID {
		grants = append(grants, map[string]any{"userId": responsibleUserID, "accessLevel": InstanceGrantManage, "isActive": true})
	}
	after := map[string]any{
		"accessPolicy":      InstanceAccessPolicyRestricted,
		"accessRevision":    int64(1),
		"responsibleUserId": responsibleUserID,
		"grants":            grants,
	}
	if err := insertInstanceAccessAuditTx(ctx, tx, accountID, id, createdByUserID, "instance_created", nil, after); err != nil {
		return "", err
	}
	effectiveFrom := time.Now().UTC()
	binding := channelClientBindingWrite{
		ClientAccountID: accountID,
		Channel:         "WHATSAPP",
		ResourceID:      id,
		EffectiveFrom:   effectiveFrom,
		Reason:          "Vinculo padrao da propria conta",
		IdempotencyKey:  "standalone-default:" + id,
		ActorUserID:     createdByUserID,
		Source:          "standalone_default",
	}
	binding.RequestHash = channelBindingRequestHash("create", map[string]any{
		"clientAccountId": accountID,
		"channel":         binding.Channel,
		"resourceId":      id,
		"effectiveFrom":   effectiveFrom.Format(time.RFC3339Nano),
		"reason":          binding.Reason,
	})
	if _, err := createChannelClientBindingTx(ctx, tx, accountID, binding); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return id, nil
}

// ReplaceInstanceAccess troca policy/grants com optimistic locking. Revogacoes preservam a
// linha e o ultimo manage nunca desaparece; toda mudanca incrementa access_revision uma vez.
func (s *Store) ReplaceInstanceAccess(ctx context.Context, in InstanceAccessWrite) (InstanceAccessWriteResult, error) {
	in.AccountID = strings.TrimSpace(in.AccountID)
	in.InstanceID = strings.TrimSpace(in.InstanceID)
	in.ActorUserID = strings.TrimSpace(in.ActorUserID)
	in.ResponsibleUserID = strings.TrimSpace(in.ResponsibleUserID)
	if in.AccountID == "" || in.InstanceID == "" || in.ActorUserID == "" || in.ExpectedRevision < 0 || !validInstanceAccessPolicy(in.AccessPolicy) {
		return InstanceAccessWriteResult{}, ErrInvalidBody
	}
	target, err := normalizeInstanceGrantInputs(in.Grants)
	if err != nil {
		return InstanceAccessWriteResult{}, err
	}
	managers := activeManagers(target)
	if len(managers) == 0 {
		return InstanceAccessWriteResult{}, ErrLastInstanceManager
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return InstanceAccessWriteResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var currentPolicy string
	var currentRevision int64
	var currentResponsible *string
	err = tx.QueryRow(ctx, `select access_policy,access_revision,responsible_user_id::text
		from messaging.whatsapp_instances
		where account_id=$1::uuid and id=$2::uuid for update`, in.AccountID, in.InstanceID).
		Scan(&currentPolicy, &currentRevision, &currentResponsible)
	if errors.Is(err, pgx.ErrNoRows) {
		return InstanceAccessWriteResult{}, ErrNotFound
	}
	if err != nil {
		return InstanceAccessWriteResult{}, err
	}
	if currentRevision != in.ExpectedRevision {
		return InstanceAccessWriteResult{}, ErrInstanceAccessRevisionConflict
	}

	if err := validateActiveGrantMembersTx(ctx, tx, in.AccountID, target); err != nil {
		return InstanceAccessWriteResult{}, err
	}
	existing, err := listInstanceGrantsTx(ctx, tx, in.AccountID, in.InstanceID)
	if err != nil {
		return InstanceAccessWriteResult{}, err
	}

	responsible := in.ResponsibleUserID
	if responsible != "" && target[responsible] != InstanceGrantManage {
		return InstanceAccessWriteResult{}, ErrInvalidBody
	}
	if responsible == "" && currentResponsible != nil && target[*currentResponsible] == InstanceGrantManage {
		responsible = *currentResponsible
	}
	if responsible == "" {
		responsible = managers[0]
	}

	changed := currentPolicy != string(in.AccessPolicy) || !sameActiveInstanceGrants(existing, target)
	if currentResponsible == nil || *currentResponsible != responsible {
		changed = true
	}
	if !changed {
		return InstanceAccessWriteResult{AccessRevision: currentRevision, ResponsibleUserID: responsible}, tx.Commit(ctx)
	}

	for userID, level := range target {
		current, exists := existing[userID]
		if exists && current.IsActive && current.AccessLevel == level {
			continue
		}
		if _, err := tx.Exec(ctx, `insert into messaging.whatsapp_instance_user_grants
			(account_id,instance_id,user_id,access_level,is_active,revision,
			 granted_by_user_id,updated_by_user_id,revoked_by_user_id,revoked_at)
			values ($1::uuid,$2::uuid,$3::uuid,$4,true,1,$5::uuid,$5::uuid,null,null)
			on conflict (account_id,instance_id,user_id) do update set
			 access_level=excluded.access_level,is_active=true,
			 revision=messaging.whatsapp_instance_user_grants.revision+1,
			 updated_by_user_id=excluded.updated_by_user_id,
			 revoked_by_user_id=null,revoked_at=null,updated_at=now()`,
			in.AccountID, in.InstanceID, userID, string(level), in.ActorUserID); err != nil {
			return InstanceAccessWriteResult{}, err
		}
	}
	for userID, current := range existing {
		if !current.IsActive {
			continue
		}
		if _, remains := target[userID]; remains {
			continue
		}
		if _, err := tx.Exec(ctx, `update messaging.whatsapp_instance_user_grants
			set is_active=false,revision=revision+1,updated_by_user_id=$4::uuid,
			 revoked_by_user_id=$4::uuid,revoked_at=now(),updated_at=now()
			where account_id=$1::uuid and instance_id=$2::uuid and user_id=$3::uuid and is_active`,
			in.AccountID, in.InstanceID, userID, in.ActorUserID); err != nil {
			return InstanceAccessWriteResult{}, err
		}
	}

	result := InstanceAccessWriteResult{AccessRevision: currentRevision + 1, ResponsibleUserID: responsible, Changed: true}
	assignedJSON, err := json.Marshal(sortedAssignedUserIDs(target))
	if err != nil {
		return InstanceAccessWriteResult{}, err
	}
	if _, err := tx.Exec(ctx, `update messaging.whatsapp_instances
		set access_policy=$3,responsible_user_id=$4::uuid,
			 access_revision=access_revision+1,
			 provider_config=coalesce(provider_config, '{}'::jsonb) || jsonb_build_object('assignedUserIds',$5::jsonb),
			 updated_at=now()
		where account_id=$1::uuid and id=$2::uuid`,
		in.AccountID, in.InstanceID, string(in.AccessPolicy), responsible, string(assignedJSON)); err != nil {
		return InstanceAccessWriteResult{}, err
	}

	before := map[string]any{"accessPolicy": currentPolicy, "accessRevision": currentRevision,
		"responsibleUserId": currentResponsible, "grants": sortedGrantViews(existing)}
	after := map[string]any{"accessPolicy": in.AccessPolicy, "accessRevision": result.AccessRevision,
		"responsibleUserId": responsible, "grants": sortedTargetGrantViews(target)}
	if err := insertInstanceAccessAuditTx(ctx, tx, in.AccountID, in.InstanceID, in.ActorUserID, "access_replaced", before, after); err != nil {
		return InstanceAccessWriteResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return InstanceAccessWriteResult{}, err
	}
	return result, nil
}

func (s *Store) InstanceAccessBackfillReport(ctx context.Context) (InstanceAccessBackfillReport, error) {
	var report InstanceAccessBackfillReport
	err := s.pool.QueryRow(ctx, `select
		count(*)::bigint,
		count(*) filter (where wi.access_policy='ACCOUNT_SHARED')::bigint,
		count(*) filter (where wi.access_policy='RESTRICTED')::bigint,
		count(*) filter (where not exists (
			select 1 from messaging.whatsapp_instance_user_grants manager
			where manager.account_id=wi.account_id and manager.instance_id=wi.id
			  and manager.is_active and manager.access_level='manage'
		))::bigint,
		(select count(*) from messaging.whatsapp_instance_user_grants where is_active and access_level='view')::bigint,
		(select count(*) from messaging.whatsapp_instance_user_grants where is_active and access_level='reply')::bigint,
		(select count(*) from messaging.whatsapp_instance_user_grants where is_active and access_level='manage')::bigint
		from messaging.whatsapp_instances wi`).Scan(&report.TotalInstances, &report.Shared,
		&report.Restricted, &report.WithoutManage, &report.ViewGrants, &report.ReplyGrants, &report.ManageGrants)
	if err != nil {
		return InstanceAccessBackfillReport{}, err
	}
	report.Issues, err = s.InstanceAccessBackfillIssues(ctx)
	report.IgnoredUsers = int64(len(report.Issues))
	return report, err
}

func (s *Store) InstanceAccessBackfillIssues(ctx context.Context) ([]InstanceAccessBackfillIssue, error) {
	rows, err := s.pool.Query(ctx, `with candidates as (
		select wi.account_id,wi.id instance_id,wi.instance_name,'responsible_user_id' source,
		       wi.responsible_user_id::text raw_user_id
		from messaging.whatsapp_instances wi where wi.responsible_user_id is not null
		union all
		select wi.account_id,wi.id,wi.instance_name,'created_by_user_id',wi.created_by_user_id::text
		from messaging.whatsapp_instances wi
		where wi.created_by_user_id is not null and not exists (
			select 1 from core.account_users responsible
			where responsible.account_id=wi.account_id
			  and responsible.user_id=wi.responsible_user_id and responsible.is_active)
		union all
		select wi.account_id,wi.id,wi.instance_name,'assignedUserIds',btrim(assigned.raw_user_id)
		from messaging.whatsapp_instances wi
		cross join lateral jsonb_array_elements_text(case
			when jsonb_typeof(wi.provider_config->'assignedUserIds')='array'
			then wi.provider_config->'assignedUserIds' else '[]'::jsonb end) assigned(raw_user_id)
	), classified as (
		select candidate.*,
		case
			when candidate.raw_user_id !~* '^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'
				then 'invalid_user_id'
			when user_row.id is null then 'unknown_user'
			when membership.user_id is null then 'cross_account_or_not_member'
			when not membership.is_active then 'inactive_membership'
		end reason
		from candidates candidate
		left join core.users user_row on user_row.id::text=candidate.raw_user_id
		left join core.account_users membership
		  on membership.account_id=candidate.account_id and membership.user_id=user_row.id
	)
	select account_id::text,instance_id::text,instance_name,source,raw_user_id,reason
	from classified where reason is not null
	order by account_id,instance_name,source,raw_user_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	issues := make([]InstanceAccessBackfillIssue, 0)
	for rows.Next() {
		var issue InstanceAccessBackfillIssue
		if err := rows.Scan(&issue.AccountID, &issue.InstanceID, &issue.InstanceName,
			&issue.Source, &issue.RawUserID, &issue.Reason); err != nil {
			return nil, err
		}
		issues = append(issues, issue)
	}
	return issues, rows.Err()
}

func normalizeInstanceGrantInputs(inputs []InstanceGrantInput) (map[string]InstanceGrantLevel, error) {
	out := make(map[string]InstanceGrantLevel, len(inputs))
	for _, input := range inputs {
		userID := strings.TrimSpace(input.UserID)
		if userID == "" || !validInstanceGrantLevel(input.AccessLevel) {
			return nil, ErrInvalidBody
		}
		if previous, exists := out[userID]; exists && previous != input.AccessLevel {
			return nil, ErrInvalidBody
		}
		out[userID] = input.AccessLevel
	}
	return out, nil
}

func activeManagers(grants map[string]InstanceGrantLevel) []string {
	managers := make([]string, 0)
	for userID, level := range grants {
		if level == InstanceGrantManage {
			managers = append(managers, userID)
		}
	}
	sort.Strings(managers)
	return managers
}

func validateActiveGrantMembersTx(ctx context.Context, tx pgx.Tx, accountID string, grants map[string]InstanceGrantLevel) error {
	if len(grants) == 0 {
		return ErrLastInstanceManager
	}
	ids := make([]string, 0, len(grants))
	for userID := range grants {
		ids = append(ids, userID)
	}
	var count int
	if err := tx.QueryRow(ctx, `select count(*) from core.account_users
		where account_id=$1::uuid and user_id=any($2::uuid[]) and is_active`, accountID, ids).Scan(&count); err != nil {
		return err
	}
	if count != len(ids) {
		return ErrInvalidBody
	}
	return nil
}

func listInstanceGrantsTx(ctx context.Context, tx pgx.Tx, accountID, instanceID string) (map[string]storedInstanceGrant, error) {
	rows, err := tx.Query(ctx, `select user_id::text,access_level,is_active,revision
		from messaging.whatsapp_instance_user_grants
		where account_id=$1::uuid and instance_id=$2::uuid for update`, accountID, instanceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]storedInstanceGrant{}
	for rows.Next() {
		var grant storedInstanceGrant
		var level string
		if err := rows.Scan(&grant.UserID, &level, &grant.IsActive, &grant.Revision); err != nil {
			return nil, err
		}
		grant.AccessLevel = InstanceGrantLevel(level)
		out[grant.UserID] = grant
	}
	return out, rows.Err()
}

func sameActiveInstanceGrants(existing map[string]storedInstanceGrant, target map[string]InstanceGrantLevel) bool {
	activeCount := 0
	for userID, grant := range existing {
		if !grant.IsActive {
			continue
		}
		activeCount++
		if target[userID] != grant.AccessLevel {
			return false
		}
	}
	return activeCount == len(target)
}

func sortedGrantViews(grants map[string]storedInstanceGrant) []storedInstanceGrant {
	out := make([]storedInstanceGrant, 0, len(grants))
	for _, grant := range grants {
		out = append(out, grant)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UserID < out[j].UserID })
	return out
}

func sortedTargetGrantViews(grants map[string]InstanceGrantLevel) []storedInstanceGrant {
	out := make([]storedInstanceGrant, 0, len(grants))
	for userID, level := range grants {
		out = append(out, storedInstanceGrant{UserID: userID, AccessLevel: level, IsActive: true})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UserID < out[j].UserID })
	return out
}

func sortedAssignedUserIDs(grants map[string]InstanceGrantLevel) []string {
	out := make([]string, 0, len(grants))
	for userID, level := range grants {
		if level != InstanceGrantManage {
			out = append(out, userID)
		}
	}
	sort.Strings(out)
	return out
}

func insertInstanceAccessAuditTx(ctx context.Context, tx pgx.Tx, accountID, instanceID, actorUserID, operation string, before, after any) error {
	payload, err := json.Marshal(map[string]any{
		"operation":  operation,
		"instanceId": instanceID,
		"before":     before,
		"after":      after,
	})
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `insert into messaging.audit_events
		(account_id,actor_user_id,event_type,payload_json)
		values ($1::uuid,nullif($2,'')::uuid,'WHATSAPP_INSTANCE_ACCESS_CHANGED',$3::jsonb)`,
		accountID, actorUserID, string(payload))
	return err
}
