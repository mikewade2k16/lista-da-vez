package omnichannel

import (
	"context"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

// Regras da conta (GET/PATCH /account) e das instancias de WhatsApp.
// Os contatos vivem em service_contacts.go; conversas/mensagens em service.go.

const (
	// Defaults do legado para a config do atendimento (Prisma AtendimentoTenantConfig:50-51).
	// Sao os MESMOS defaults das colunas na migration 0200 — nao ha segunda verdade: a
	// coluna manda, isto so cobre a linha que ainda nao existe.
	defaultRetentionDays = 15
	defaultMaxUploadMb   = 500

	// Ultimo recurso dos limites, quando nem core.account_modules.config nem
	// core.platform_settings tem o valor. Sao os fallbacks do PROPRIO legado
	// (routes-whatsapp-instances.ts:147-148: fallbackMaxUsers 3, fallbackMaxChannels 1) —
	// nao valores inventados aqui.
	defaultMaxChannels = 1
	defaultMaxUsers    = 3

	// userScopePolicyMultiInstance e o default do legado (routes-whatsapp-instances.ts:41).
	// Deixou de ser constante fixa: a gestao de instancia persiste a escolha em
	// provider_config.userScopePolicy (o front tem seletor real). Este valor e so o fallback
	// quando a chave ainda nao foi gravada.
	userScopePolicyMultiInstance = "MULTI_INSTANCE"
	// userScopePolicySingleInstance e o outro valor que o front tipa
	// (types/index.ts:40: "MULTI_INSTANCE" | "SINGLE_INSTANCE").
	userScopePolicySingleInstance = "SINGLE_INSTANCE"
)

// Papeis legados que o front verbatim conhece (types/index.ts:1).
const (
	legacyRoleAdmin      = "ADMIN"
	legacyRoleSupervisor = "SUPERVISOR"
	legacyRoleAgent      = "AGENT"
	legacyRoleViewer     = "VIEWER"
)

// legacyRole traduz o papel do Omni para o papel que o front verbatim tipa. O legado nao
// tem equivalente para os papeis do Omni: este mapa e da fusao, nao do port.
func legacyRole(role auth.Role) string {
	switch role {
	case auth.RolePlatformAdmin, auth.RoleOwner, auth.RoleDirector:
		return legacyRoleAdmin
	case auth.RoleManager:
		return legacyRoleSupervisor
	case auth.RoleMarketing, auth.RoleConsultant:
		return legacyRoleAgent
	default:
		return legacyRoleViewer
	}
}

// ============================================================================
// Conta (GET/PATCH /account)
// ============================================================================

// GetAccountSettings monta o shape do mapTenantResponse do legado, com as tres trocas
// de fonte da spec C4: id/slug/name de core.accounts; maxChannels/maxUsers de
// core.account_modules.config; hasEvolutionApiKey de credentials_ciphertext.
// ListTenantUsers lista os membros ativos da conta para o picker de atribuicao do inbox
// (GET /users). Reusa ListAssignableUsers (mesma fonte: core.account_users ativos). O papel por
// usuario segue o GAP DECLARADO do modulo (o RBAC custom por conta nao e resolvivel aqui — ver
// GetInstanceManagement): emite o papel neutro so para a tipagem do front verbatim fechar; o
// picker usa id/nome.
func (s *Service) ListTenantUsers(ctx context.Context, accountID string, caller Caller) ([]TenantUserView, error) {
	scope, err := s.store.LoadConversationAccessScope(ctx, accountID, caller.UserID)
	if err != nil {
		return nil, err
	}
	if !scope.Eligible || !scope.allowsPermission("omnichannel.conversations.assign") {
		return nil, ErrForbidden
	}
	users, err := s.store.ListAssignableUsers(ctx, accountID)
	if err != nil {
		return nil, err
	}
	out := make([]TenantUserView, 0, len(users))
	for _, u := range users {
		role := strings.TrimSpace(u.Role)
		if role == "" {
			role = "AGENT"
		}
		out = append(out, TenantUserView{ID: u.ID, TenantID: accountID, Email: u.Email, Name: u.Name, Role: role})
	}
	return out, nil
}

func (s *Service) GetAccountSettings(ctx context.Context, accountID string, caller Caller) (AccountSettingsView, error) {
	scope, err := s.store.LoadConversationAccessScope(ctx, accountID, caller.UserID)
	if err != nil {
		return AccountSettingsView{}, err
	}
	canViewInbox := scope.allowsPermission("omnichannel.conversations.view")
	canManageSettings := scope.allowsPermission("omnichannel.settings.manage")
	if !scope.Eligible || (!canViewInbox && !canManageSettings) {
		return AccountSettingsView{}, ErrForbidden
	}
	account, err := s.store.GetAccount(ctx, accountID)
	if err != nil {
		return AccountSettingsView{}, translate(err)
	}
	retentionDays, maxUploadMb, err := s.store.GetAccountConfig(ctx, accountID)
	if err != nil {
		return AccountSettingsView{}, err
	}
	maxChannels, maxUsers, err := s.store.GetModuleLimits(ctx, accountID)
	if err != nil {
		return AccountSettingsView{}, err
	}
	currentUsers, err := s.store.CountAccountUsers(ctx, accountID)
	if err != nil {
		return AccountSettingsView{}, err
	}
	requiredInstanceLevel := InstanceGrantView
	if canManageSettings {
		requiredInstanceLevel = InstanceGrantManage
	}
	instances, err := s.store.ListInstances(ctx, accountID, InstanceFilter{
		IDs: scope.visibleInstanceIDs(requiredInstanceLevel, false),
	})
	if err != nil {
		return AccountSettingsView{}, err
	}
	if err := decorateInstanceCapabilities(ctx, s.store, accountID, caller, instances); err != nil {
		return AccountSettingsView{}, err
	}

	currentChannels := 0
	hasEvolutionAPIKey := false
	var defaultInstance *string
	for i := range instances {
		if !instances[i].IsActive {
			continue
		}
		currentChannels++
		if instances[i].HasEvolutionAPIKey {
			hasEvolutionAPIKey = true
		}
		if instances[i].IsDefault && defaultInstance == nil {
			name := instances[i].InstanceName
			defaultInstance = &name
		}
	}

	return AccountSettingsView{
		ID:                account.ID,
		Slug:              account.Slug,
		Name:              account.Name,
		WhatsAppInstance:  defaultInstance,
		WhatsAppInstances: instances,
		MaxChannels:       maxChannels,
		MaxUsers:          maxUsers,
		RetentionDays:     retentionDays,
		MaxUploadMb:       maxUploadMb,
		// Quem administra a conta mexe nos limites do atendimento.
		CanManageAtendimentoLimits: caller.IsPlatformAdmin,
		CurrentChannels:            currentChannels,
		CurrentUsers:               currentUsers,
		HasEvolutionAPIKey:         hasEvolutionAPIKey,
		// webhookUrl fica VAZIO na F2: a URL vem de WEBHOOK_RECEIVER_BASE_URL, que e env
		// da F4 (canonico §13, item 5). Inventar uma URL aqui seria mentir para o painel.
		WebhookURL:       "",
		CreatedAt:        account.CreatedAt,
		UpdatedAt:        account.UpdatedAt,
		CanViewSensitive: scope.allowsPermission("omnichannel.instances.manage"),
		// evolutionApiKey responde SEMPRE null — o legado ja faz isso. A chave nunca sai
		// do server (canonico §10: credencial so via secretbox, nunca de volta pro front).
		EvolutionAPIKey: nil,
	}, nil
}

// UpdateAccountSettings grava os limites do atendimento. Na F2 so retentionDays e
// maxUploadMb sao gravaveis: os demais campos do TenantSettings tem outra fonte
// (core.accounts, core.account_modules) e nao se editam por este endpoint.
func (s *Service) UpdateAccountSettings(ctx context.Context, accountID string, caller Caller, patch AccountSettingsPatch) (AccountSettingsView, error) {
	scope, err := s.store.LoadConversationAccessScope(ctx, accountID, caller.UserID)
	if err != nil {
		return AccountSettingsView{}, err
	}
	if !scope.Eligible || !scope.allowsPermission("omnichannel.settings.manage") {
		return AccountSettingsView{}, ErrForbidden
	}
	retentionDays, maxUploadMb, err := s.store.GetAccountConfig(ctx, accountID)
	if err != nil {
		return AccountSettingsView{}, err
	}
	if patch.RetentionDays != nil {
		if *patch.RetentionDays < 1 {
			return AccountSettingsView{}, ErrInvalidBody
		}
		retentionDays = *patch.RetentionDays
	}
	if patch.MaxUploadMb != nil {
		if *patch.MaxUploadMb < 1 {
			return AccountSettingsView{}, ErrInvalidBody
		}
		maxUploadMb = *patch.MaxUploadMb
	}
	if err := s.store.UpsertAccountConfig(ctx, accountID, retentionDays, maxUploadMb); err != nil {
		return AccountSettingsView{}, err
	}
	return s.GetAccountSettings(ctx, accountID, caller)
}

// ============================================================================
// Instancias
// ============================================================================

// ListInstances monta o WhatsAppInstanceManagementResponse. E a tela de gestao de
// numeros: so admin da conta.
func (s *Service) ListInstances(ctx context.Context, accountID string, caller Caller) (InstanceManagementView, error) {
	scope, err := s.store.LoadConversationAccessScope(ctx, accountID, caller.UserID)
	if err != nil {
		return InstanceManagementView{}, err
	}
	if !scope.Eligible || !scope.allowsPermission("omnichannel.instances.manage") {
		return InstanceManagementView{}, ErrForbidden
	}
	instances, err := s.store.ListInstances(ctx, accountID, InstanceFilter{
		IDs: scope.visibleInstanceIDs(InstanceGrantManage, false),
	})
	if err != nil {
		return InstanceManagementView{}, err
	}
	if err := decorateInstanceCapabilities(ctx, s.store, accountID, caller, instances); err != nil {
		return InstanceManagementView{}, err
	}
	maxChannels, _, err := s.store.GetModuleLimits(ctx, accountID)
	if err != nil {
		return InstanceManagementView{}, err
	}
	currentChannels := 0
	for _, instance := range instances {
		if instance.IsActive {
			currentChannels++
		}
	}
	users, err := s.store.ListAssignableUsers(ctx, accountID)
	if err != nil {
		return InstanceManagementView{}, err
	}
	// GAP DECLARADO (nao e derivacao inventada): `role` e `atendimentoAccess` por usuario
	// nao tem fonte resolvivel nesta fase. Os papeis do Omni sao CUSTOM por conta
	// (core.roles + core.user_role_assignments + overrides) e quem os resolve e o
	// ResolveEffectivePermissions do modulo `access` — que modules.Dependencies NAO expoe.
	// Refazer esse JOIN aqui criaria uma segunda verdade de RBAC (fere o principio 1), e
	// chutar ADMIN/SUPERVISOR por usuario seria pior: o front gateia por este campo.
	// Emitimos o valor neutro (AGENT/true) so para a tipagem do front verbatim fechar; o
	// array `users` alimenta o seletor de responsavel, que nao depende do papel.
	// Fonte real = F10 (telas de config), junto com o enforcement por key da F6/F7.
	// Registrado em docs/LEGADO.md e no AGENT.md do modulo.
	for i := range users {
		users[i].Role = legacyRoleAgent
		users[i].AtendimentoAccess = true
	}
	return InstanceManagementView{
		MaxChannels:     maxChannels,
		CurrentChannels: currentChannels,
		Instances:       instances,
		Users:           users,
	}, nil
}

// ListAccessibleInstances monta o WhatsAppInstanceAccessResponse — as instancias que o
// USUARIO alcanca.
//
// Aqui mora o A2 CORRIGIDO: o legado devolve a mesma lista nos dois ramos do ternario
// (whatsapp-instances.ts:681-683), ou seja, todo usuario ve todas as instancias. Como
// isso e isolamento (principio 2), portamos corrigido: nao-admin ve as instancias sem
// responsavel + aquelas das quais e responsible_user_id. O gate de dado definitivo e
// queue_members e chega na F8 — nao inventar um segundo gate aqui.
func (s *Service) ListAccessibleInstances(ctx context.Context, accountID string, caller Caller) (InstanceAccessView, error) {
	scope, err := s.store.LoadConversationAccessScope(ctx, accountID, caller.UserID)
	if err != nil {
		return InstanceAccessView{}, err
	}
	if !scope.Eligible || !scope.allowsPermission("omnichannel.conversations.view") {
		return InstanceAccessView{}, ErrForbidden
	}
	instances, err := s.store.ListInstances(ctx, accountID, InstanceFilter{
		ActiveOnly: true,
		IDs:        scope.visibleInstanceIDs(InstanceGrantView, true),
	})
	if err != nil {
		return InstanceAccessView{}, err
	}
	if err := decorateInstanceCapabilities(ctx, s.store, accountID, caller, instances); err != nil {
		return InstanceAccessView{}, err
	}
	// hasMultipleActiveInstances olha os canais ATIVOS da conta (nao a lista filtrada):
	// e o que o legado responde, e o front usa isso para decidir se mostra o seletor.
	activeCount := len(instances)
	// O /access do legado nao expoe o e-mail do responsavel nem os assignedUserIds
	// (mapAccessibleInstancePayload:53-59) — e uma rota de operador, nao de admin.
	for i := range instances {
		instances[i].ResponsibleUserMail = nil
		instances[i].AssignedUserIDs = []string{}
	}
	return InstanceAccessView{
		HasMultipleActiveInstances: activeCount > 1,
		Instances:                  instances,
	}, nil
}

func loadInstanceCapabilities(ctx context.Context, store *Store, accountID string, caller Caller) (InstanceCapabilities, error) {
	if store == nil || strings.TrimSpace(accountID) == "" || strings.TrimSpace(caller.UserID) == "" {
		return InstanceCapabilities{}, nil
	}
	capabilities := InstanceCapabilities{}
	checks := []struct {
		key    string
		target *bool
	}{
		{key: "omnichannel.conversations.view", target: &capabilities.View},
		{key: "omnichannel.conversations.reply", target: &capabilities.Reply},
		{key: "omnichannel.instances.manage", target: &capabilities.Manage},
	}
	for _, check := range checks {
		allowed, err := store.hasEffectivePermission(ctx, accountID, caller.UserID, check.key)
		if err != nil {
			return InstanceCapabilities{}, err
		}
		*check.target = allowed
	}
	// Reset exige as duas permissoes efetivas; nenhum papel legado cria bypass.
	privacy, err := store.hasEffectivePermission(ctx, accountID, caller.UserID, conversationPrivacyManagePermission)
	if err != nil {
		return InstanceCapabilities{}, err
	}
	capabilities.ResetHistory = capabilities.Manage && privacy
	return capabilities, nil
}

func decorateInstanceCapabilities(ctx context.Context, store *Store, accountID string, caller Caller, instances []InstanceView) error {
	scope, err := store.LoadConversationAccessScope(ctx, accountID, caller.UserID)
	if err != nil {
		return err
	}
	for index := range instances {
		decision, ok := scope.instanceDecision(instances[index].ID)
		if ok {
			instances[index].MyCapabilities = decision.Capabilities
		}
	}
	return nil
}
