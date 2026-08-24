package calendar

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

// chatProposalExecutionAccess e o recorte de acesso reidratado dentro da mesma
// transacao que aplica o efeito. Valores resolvidos antes da transacao servem
// apenas para montar o comando; nunca autorizam a escrita por si so.
type chatProposalExecutionAccess struct {
	ActiveAccountID    string
	StorageAccountID   string
	OrganizationID     string
	ActiveIsAgency     bool
	ActorIsPlatform    bool
	ActorIsAgencyOwner bool
	ConversationScope  string
	ConversationClient string
	CanInspectAllChats bool
}

type calendarExecutionTopology struct {
	ActiveAccountID  string
	StorageAccountID string
	OrganizationID   string
	ActiveIsAgency   bool
}

// resolveCalendarStorageAccountTx espelha Store.ResolveCalendarScope dentro da
// transacao do card. Os rows da conta ativa e da conta de armazenamento recebem
// KEY SHARE; em Repeatable Read, uma mudanca concorrente de organizacao/owner
// serializa ou aborta, em vez de trocar silenciosamente o destino da escrita.
func resolveCalendarStorageAccountTx(
	ctx context.Context,
	tx pgx.Tx,
	activeAccountID string,
) (calendarExecutionTopology, error) {
	const q = `select a.id::text, coalesce(a.organization_id::text, ''), a.is_agency,
		case when a.is_agency then a.id::text else coalesce((
			select owner.id::text
			from core.accounts owner
			where owner.organization_id = a.organization_id
			  and owner.is_agency = true and owner.is_active = true
			order by owner.created_at asc, owner.id asc
			limit 1
		), a.id::text) end
	from core.accounts a
	where a.id = $1::uuid and a.is_active = true
	for key share of a`
	var topology calendarExecutionTopology
	if err := tx.QueryRow(ctx, q, activeAccountID).Scan(
		&topology.ActiveAccountID,
		&topology.OrganizationID,
		&topology.ActiveIsAgency,
		&topology.StorageAccountID,
	); err != nil {
		return calendarExecutionTopology{}, err
	}
	var lockedStorageID string
	if err := tx.QueryRow(ctx, `select id::text from core.accounts
		where id = $1::uuid and is_active = true for key share`, topology.StorageAccountID).
		Scan(&lockedStorageID); err != nil {
		return calendarExecutionTopology{}, err
	}
	if lockedStorageID != topology.StorageAccountID {
		return calendarExecutionTopology{}, ErrProposalSnapshotMissing
	}
	return topology, nil
}

func validateChatProposalExecutionScopeTx(
	ctx context.Context,
	tx pgx.Tx,
	command chatProposalExecutionCommand,
	storedStorageAccountID, conversationOwnerID, entrySurface, scopeMode, scopeClientID string,
) (chatProposalExecutionAccess, error) {
	topology, err := resolveCalendarStorageAccountTx(ctx, tx, command.AccountID)
	if errors.Is(err, pgx.ErrNoRows) {
		return chatProposalExecutionAccess{}, ErrNotFound
	}
	if err != nil {
		return chatProposalExecutionAccess{}, err
	}
	if normalizeUUID(storedStorageAccountID) == "" ||
		topology.StorageAccountID != normalizeUUID(storedStorageAccountID) ||
		topology.StorageAccountID != normalizeUUID(command.StorageAccountID) {
		return chatProposalExecutionAccess{}, ErrProposalSnapshotMissing
	}

	moduleID, writePermission := "calendar", "calendar.manage"
	if command.Proposal.Kind == "task" || command.Proposal.Kind == "taskItem" {
		moduleID = "tasks"
		switch {
		case command.Proposal.Kind == "taskItem":
			writePermission = "tasks.tasks.edit"
		case command.Proposal.Action == "update":
			writePermission = "tasks.tasks.edit"
		case command.Proposal.Action == "delete":
			writePermission = "tasks.tasks.delete"
		default:
			writePermission = "tasks.tasks.create"
		}
	}
	actor, err := loadChatProposalActorAccessTx(
		ctx, tx, topology, command.ActorUserID, moduleID, writePermission,
	)
	if err != nil {
		return chatProposalExecutionAccess{}, err
	}
	if !actor.canAccess || !actor.canWrite || !actor.moduleEnabled {
		return chatProposalExecutionAccess{}, ErrForbidden
	}
	normalizedSurface, err := normalizeAssistantSurface(entrySurface)
	if err != nil || command.CapabilityMode == nil {
		return chatProposalExecutionAccess{}, ErrForbidden
	}
	mode, err := command.CapabilityMode(
		ctx, tx, topology.ActiveAccountID, normalizedSurface, moduleID,
	)
	if err != nil {
		return chatProposalExecutionAccess{}, err
	}
	if normalizeAssistantMode(mode) != assistantModeWrite {
		return chatProposalExecutionAccess{}, ErrForbidden
	}
	access := chatProposalExecutionAccess{
		ActiveAccountID:    topology.ActiveAccountID,
		StorageAccountID:   topology.StorageAccountID,
		OrganizationID:     topology.OrganizationID,
		ActiveIsAgency:     topology.ActiveIsAgency,
		ActorIsPlatform:    actor.isPlatform,
		ActorIsAgencyOwner: actor.isAgencyOwner,
		ConversationScope:  strings.TrimSpace(scopeMode),
		ConversationClient: normalizeUUID(scopeClientID),
	}
	access.CanInspectAllChats = access.ActiveIsAgency &&
		(access.ActorIsPlatform || access.ActorIsAgencyOwner)
	if normalizeUUID(conversationOwnerID) != normalizeUUID(command.ActorUserID) &&
		!access.CanInspectAllChats {
		return chatProposalExecutionAccess{}, ErrNotFound
	}

	switch access.ConversationScope {
	case chatScopeClient:
		if access.ConversationClient == "" ||
			normalizeUUID(command.LockedClientID) != access.ConversationClient {
			return chatProposalExecutionAccess{}, ErrProposalSnapshotMissing
		}
		if err := validateExecutionClientTx(ctx, tx, command, access, access.ConversationClient); err != nil {
			return chatProposalExecutionAccess{}, err
		}
	case chatScopeAll:
		if !access.ActiveIsAgency || normalizeUUID(command.LockedClientID) != "" {
			return chatProposalExecutionAccess{}, ErrProposalSnapshotMissing
		}
		if !access.CanInspectAllChats {
			count, countErr := visibleCalendarClientCountTx(ctx, tx, command, access)
			if countErr != nil {
				return chatProposalExecutionAccess{}, countErr
			}
			if count < 2 {
				return chatProposalExecutionAccess{}, ErrNotFound
			}
		}
	default:
		return chatProposalExecutionAccess{}, ErrProposalSnapshotMissing
	}

	if command.Proposal.Kind == "note" &&
		(access.ConversationScope != chatScopeAll || !access.CanInspectAllChats) {
		return chatProposalExecutionAccess{}, ErrForbidden
	}
	proposalClientID := normalizeUUID(command.Proposal.Fields.ClientID)
	if command.Proposal.Kind == "clientProfile" && proposalClientID == "" {
		return chatProposalExecutionAccess{}, ErrInvalidClient
	}
	if proposalClientID != "" {
		if err := validateExecutionClientTx(ctx, tx, command, access, proposalClientID); err != nil {
			return chatProposalExecutionAccess{}, err
		}
	}
	return access, nil
}

type chatProposalActorAccess struct {
	isPlatform    bool
	isAgencyOwner bool
	canAccess     bool
	canWrite      bool
	moduleEnabled bool
}

func loadChatProposalActorAccessTx(
	ctx context.Context,
	tx pgx.Tx,
	topology calendarExecutionTopology,
	actorUserID, moduleID, writePermission string,
) (chatProposalActorAccess, error) {
	const q = `select u.is_platform_admin,
		exists (
			select 1 from core.organization_users ou
			where ou.user_id = u.id and ou.organization_id = $2::uuid
			  and ou.org_role = 'agency_owner'
		),
		exists (
			select 1 from core.account_users au
			where au.account_id = $1::uuid and au.user_id = u.id and au.is_active = true
		),
		exists (
			select 1 from core.user_role_assignments ura
			join core.roles r on r.id = ura.role_id and r.account_id = ura.account_id
			where ura.account_id = $1::uuid and ura.user_id = u.id
			  and lower(r.code) = any($3::text[])
		),
		(
			(
				exists (
					select 1 from core.user_role_assignments ura
					join core.role_permissions rp on rp.role_id = ura.role_id
					join core.permissions p on p.key = rp.permission_key and p.deprecated_at is null
					where ura.account_id = $1::uuid and ura.user_id = u.id
					  and rp.permission_key = $5
				)
				or exists (
					select 1 from core.user_permission_overrides upo
					join core.permissions p on p.key = upo.permission_key and p.deprecated_at is null
					where upo.account_id = $1::uuid and upo.user_id = u.id
					  and upo.permission_key = $5
					  and upo.effect = 'allow' and upo.is_active = true
				)
			)
			and not exists (
				select 1 from core.user_permission_overrides upo
				where upo.account_id = $1::uuid and upo.user_id = u.id
				  and upo.permission_key = $5
				  and upo.effect = 'deny' and upo.is_active = true
			)
		),
		exists (
			select 1 from core.account_modules am
			where am.account_id = $1::uuid and am.module_id = $6 and am.enabled = true
		)
	from core.users u
	where u.id = $4::uuid and u.is_active = true
	for key share of u`
	organizationID := any(nil)
	if topology.OrganizationID != "" {
		organizationID = topology.OrganizationID
	}
	var platform, agencyOwner, member, ownerRole, managePermission, moduleEnabled bool
	if err := tx.QueryRow(ctx, q, topology.ActiveAccountID, organizationID,
		auth.CoreRoleCodesForCoarse(auth.RoleOwner), actorUserID, writePermission, moduleID).Scan(
		&platform, &agencyOwner, &member, &ownerRole, &managePermission, &moduleEnabled,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return chatProposalActorAccess{}, ErrNotFound
		}
		return chatProposalActorAccess{}, err
	}
	canAccess := platform || agencyOwner || member
	canWrite := platform || agencyOwner || ownerRole || managePermission
	return chatProposalActorAccess{
		isPlatform: platform, isAgencyOwner: agencyOwner,
		canAccess: canAccess, canWrite: canWrite, moduleEnabled: moduleEnabled,
	}, nil
}

func validateExecutionClientTx(
	ctx context.Context,
	tx pgx.Tx,
	command chatProposalExecutionCommand,
	access chatProposalExecutionAccess,
	clientID string,
) error {
	clientID = normalizeUUID(clientID)
	if clientID == "" {
		if access.ConversationScope == chatScopeClient {
			return ErrNotFound
		}
		return nil
	}
	if access.ConversationScope == chatScopeClient && clientID != access.ConversationClient {
		return ErrNotFound
	}
	const topologyQuery = `select c.id::text
		from core.accounts c
		where c.id = $1::uuid and c.is_active = true and c.is_agency = false
		  and (
			($2::boolean = false and c.id = $3::uuid)
			or ($2::boolean = true and $4::uuid is not null and c.organization_id = $4::uuid)
		  )
		for key share of c`
	organizationID := any(nil)
	if access.OrganizationID != "" {
		organizationID = access.OrganizationID
	}
	var currentClientID string
	if err := tx.QueryRow(ctx, topologyQuery, clientID, access.ActiveIsAgency,
		access.ActiveAccountID, organizationID).Scan(&currentClientID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	visible, err := actorCanSeeCalendarClientTx(ctx, tx, command, access, currentClientID)
	if err != nil {
		return err
	}
	if !visible {
		return ErrNotFound
	}
	return nil
}

func actorCanSeeCalendarClientTx(
	ctx context.Context,
	tx pgx.Tx,
	command chatProposalExecutionCommand,
	access chatProposalExecutionAccess,
	clientID string,
) (bool, error) {
	if access.ActorIsPlatform {
		return true, nil
	}
	roleCodes := calendarVisibleRoleCodes(command.ActorRole)
	if len(roleCodes) == 0 {
		return false, nil
	}
	if command.ActorRole == auth.RoleOwner || command.ActorRole == auth.RoleDirector ||
		command.ActorRole == auth.RoleMarketing {
		const q = `select exists (
			select 1 from core.account_users au
			join core.user_role_assignments ura
			  on ura.account_id = au.account_id and ura.user_id = au.user_id
			join core.roles r on r.id = ura.role_id and r.account_id = ura.account_id
			where au.account_id = $1::uuid and au.user_id = $2::uuid and au.is_active = true
			  and lower(r.code) = any($3::text[])
		)`
		var visible bool
		err := tx.QueryRow(ctx, q, clientID, command.ActorUserID, roleCodes).Scan(&visible)
		return visible, err
	}
	const q = `select exists (
		select 1 from core.account_users au
		join core.user_role_assignments ura
		  on ura.account_id = au.account_id and ura.user_id = au.user_id
		join core.roles r on r.id = ura.role_id and r.account_id = ura.account_id
		join core.user_module_settings settings
		  on settings.user_id = au.user_id and settings.module_id = 'queue'
		join lateral jsonb_array_elements_text(
			coalesce(settings.config #> array['storeIdsByAccount', au.account_id::text], '[]'::jsonb)
		) configured(store_id) on true
		join queue.stores store on store.tenant_id = au.account_id
		  and store.id::text = configured.store_id and store.is_active = true
		where au.account_id = $1::uuid and au.user_id = $2::uuid and au.is_active = true
		  and lower(r.code) = any($3::text[])
	)`
	var visible bool
	err := tx.QueryRow(ctx, q, clientID, command.ActorUserID, roleCodes).Scan(&visible)
	return visible, err
}

func visibleCalendarClientCountTx(
	ctx context.Context,
	tx pgx.Tx,
	command chatProposalExecutionCommand,
	access chatProposalExecutionAccess,
) (int, error) {
	if access.ActorIsPlatform || access.ActorIsAgencyOwner {
		return 2, nil
	}
	if access.OrganizationID == "" {
		return 0, nil
	}
	roleCodes := calendarVisibleRoleCodes(command.ActorRole)
	if len(roleCodes) == 0 {
		return 0, nil
	}
	if command.ActorRole == auth.RoleOwner || command.ActorRole == auth.RoleDirector ||
		command.ActorRole == auth.RoleMarketing {
		const q = `select count(distinct c.id)
			from core.accounts c
			join core.account_users au on au.account_id = c.id
			  and au.user_id = $1::uuid and au.is_active = true
			join core.user_role_assignments ura
			  on ura.account_id = au.account_id and ura.user_id = au.user_id
			join core.roles r on r.id = ura.role_id and r.account_id = ura.account_id
			where c.organization_id = $2::uuid and c.is_active = true and c.is_agency = false
			  and lower(r.code) = any($3::text[])`
		var count int
		err := tx.QueryRow(ctx, q, command.ActorUserID, access.OrganizationID, roleCodes).Scan(&count)
		return count, err
	}
	const q = `select count(distinct c.id)
		from core.accounts c
		join core.account_users au on au.account_id = c.id
		  and au.user_id = $1::uuid and au.is_active = true
		join core.user_role_assignments ura
		  on ura.account_id = au.account_id and ura.user_id = au.user_id
		join core.roles r on r.id = ura.role_id and r.account_id = ura.account_id
		join core.user_module_settings settings
		  on settings.user_id = au.user_id and settings.module_id = 'queue'
		join lateral jsonb_array_elements_text(
			coalesce(settings.config #> array['storeIdsByAccount', au.account_id::text], '[]'::jsonb)
		) configured(store_id) on true
		join queue.stores store on store.tenant_id = c.id
		  and store.id::text = configured.store_id and store.is_active = true
		where c.organization_id = $2::uuid and c.is_active = true and c.is_agency = false
		  and lower(r.code) = any($3::text[])`
	var count int
	err := tx.QueryRow(ctx, q, command.ActorUserID, access.OrganizationID, roleCodes).Scan(&count)
	return count, err
}

// calendarVisibleRoleCodes espelha os grupos de tenants.ListAccessible.
func calendarVisibleRoleCodes(role auth.Role) []string {
	roles := []auth.Role{auth.RoleManager, auth.RoleConsultant, auth.RoleStoreTerminal}
	if role == auth.RoleOwner || role == auth.RoleDirector || role == auth.RoleMarketing {
		roles = []auth.Role{auth.RoleOwner, auth.RoleDirector, auth.RoleMarketing}
	}
	seen := make(map[string]struct{})
	codes := make([]string, 0)
	for _, current := range roles {
		for _, code := range auth.CoreRoleCodesForCoarse(current) {
			code = strings.ToLower(strings.TrimSpace(code))
			if code == "" {
				continue
			}
			if _, ok := seen[code]; ok {
				continue
			}
			seen[code] = struct{}{}
			codes = append(codes, code)
		}
	}
	return codes
}
