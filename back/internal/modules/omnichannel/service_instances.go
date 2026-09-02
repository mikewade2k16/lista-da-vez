package omnichannel

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/modules"
)

// Gestao de instancia (criar/atualizar/atribuir usuarios). Metodos no SessionService: ele ja
// tem store, registry, secretBox (para cifrar a evolutionApiKey), limits (teto de canais) e
// logger. Toda operacao e escopada por account (do Principal, nunca do body) e restrita a
// admin da conta. account_id fora de escopo -> 404, nunca 403.

// InstanceWriteInput e o body de POST/PATCH /tenant/whatsapp/instances[/{id}]. Ponteiros para
// distinguir campo AUSENTE de campo vazio.
//
// Semantica (o front verbatim manda o formulario inteiro, com os opcionais vazios OMITIDOS —
// `|| undefined` some no JSON.stringify):
//   - instanceName/userScopePolicy/isDefault/isActive: o front sempre envia (no update caem no
//     valor atual quando ausentes, para tolerar um PATCH parcial).
//   - displayName/phoneNumber/queueLabel/responsibleUserId: full-replace do formulario —
//     ausente = vazio = grava NULL.
//   - evolutionApiKey: SO-se-presente. Ausente = mantem a credencial (o campo nasce vazio na
//     edicao; limpar o input NUNCA apaga a chave gravada). Chave crua e cifrada no secretbox.
type InstanceWriteInput struct {
	InstanceName      *string `json:"instanceName"`
	DisplayName       *string `json:"displayName"`
	PhoneNumber       *string `json:"phoneNumber"`
	EvolutionAPIKey   *string `json:"evolutionApiKey"`
	QueueLabel        *string `json:"queueLabel"`
	UserScopePolicy   *string `json:"userScopePolicy"`
	ResponsibleUserID *string `json:"responsibleUserId"`
	Provider          *string `json:"provider"`
	IsDefault         *bool   `json:"isDefault"`
	IsActive          *bool   `json:"isActive"`
}

// InstanceAccessRequest aceita o contrato relacional do P2 e, temporariamente, o
// formato legado userIds. Os ponteiros distinguem campo ausente de lista vazia.
type InstanceAccessRequest struct {
	AccessRevision    *int64                `json:"accessRevision"`
	AccessPolicy      *InstanceAccessPolicy `json:"accessPolicy"`
	ResponsibleUserID *string               `json:"responsibleUserId"`
	Grants            *[]InstanceGrantInput `json:"grants"`
	UserIDs           *[]string             `json:"userIds"`
}

// InstanceAccessUpdateInput e o contrato de service preparado para a API administrativa do P2.
// Account/instance/actor nunca vêm do body; o handler futuro deve obtê-los do path/Principal.
type InstanceAccessUpdateInput struct {
	AccessPolicy      InstanceAccessPolicy
	ExpectedRevision  int64
	ResponsibleUserID string
	Grants            []InstanceGrantInput
}

// CreateInstance cria uma instancia gerenciada e devolve o registro (WhatsAppInstanceRecord).
func (s *SessionService) CreateInstance(ctx context.Context, accountID string, caller Caller, in InstanceWriteInput) (InstanceView, error) {
	ready, err := s.store.HasActiveOmnichannelMembership(ctx, accountID, caller.UserID)
	if err != nil {
		return InstanceView{}, err
	}
	if !ready {
		return InstanceView{}, ErrForbidden
	}
	canManage, err := s.store.hasEffectivePermission(ctx, accountID, caller.UserID, "omnichannel.instances.manage")
	if err != nil {
		return InstanceView{}, err
	}
	if !canManage {
		return InstanceView{}, ErrForbidden
	}
	name := strings.TrimSpace(deref(in.InstanceName))
	if name == "" {
		return InstanceView{}, ErrInvalidBody
	}
	policy, err := normalizeScopePolicy(in.UserScopePolicy)
	if err != nil {
		return InstanceView{}, err
	}
	provider := strings.TrimSpace(deref(in.Provider))
	if provider == "" {
		provider = "evolution"
	}
	if !s.registry.Has(provider) {
		return InstanceView{}, ErrProviderUnsupported
	}
	responsible, err := s.resolveResponsible(ctx, accountID, in.ResponsibleUserID)
	if err != nil {
		return InstanceView{}, err
	}
	isActive := derefBool(in.IsActive, true)
	isDefault := derefBool(in.IsDefault, false)

	phone := optTrim(in.PhoneNumber)
	if phone != nil {
		if err := ensureNumberFree(ctx, s.store, accountID, *phone, ""); err != nil {
			return InstanceView{}, err
		}
	}

	current, err := s.store.CountActiveInstances(ctx, accountID)
	if err != nil {
		return InstanceView{}, err
	}
	// Teto de numeros so pesa para instancia ATIVA (uma inativa nao ocupa canal).
	if isActive {
		if err := s.limits.Check(ctx, accountID, moduleID, limitKeyChannels, int64(current)); err != nil {
			if modules.IsLimitExceeded(err) {
				return InstanceView{}, ErrChannelLimit
			}
			return InstanceView{}, err
		}
	}

	id, err := s.store.InsertInstance(ctx, accountID, instanceWrite{
		InstanceName:      name,
		DisplayName:       optTrim(in.DisplayName),
		PhoneNumber:       phone,
		QueueLabel:        optTrim(in.QueueLabel),
		ResponsibleUserID: responsible,
		IsActive:          isActive,
		UserScopePolicy:   policy,
		Provider:          provider,
	}, caller.UserID)
	if err != nil {
		return InstanceView{}, mapInstanceWriteError(err)
	}
	// InsertInstance so retorna depois do commit da instancia + primeiro manage. A
	// invalidacao de escopo jamais e publicada de dentro da transacao.
	s.publisher.PublishOmnichannelEvent(ctx, newInvalidationSignal(
		accountID, RealtimeInvalidationReasonAccessScopeChanged, time.Now().UTC()))

	// Primeira instancia ativa da conta vira default (paridade com o bootstrap); alem disso
	// o front controla isDefault explicitamente.
	if isDefault || (isActive && current == 0) {
		if err := s.store.PromoteDefault(ctx, accountID, id); err != nil {
			return InstanceView{}, err
		}
	}
	if err := s.applyCredential(ctx, accountID, id, in.EvolutionAPIKey); err != nil {
		return InstanceView{}, err
	}
	return s.instanceViewForCaller(ctx, accountID, id, caller)
}

// UpdateInstance aplica o PATCH e devolve o registro atualizado. Instancia de outra conta ->
// 404. Full-replace do formulario (menos a credencial, so-se-presente).
func (s *SessionService) UpdateInstance(ctx context.Context, accountID string, caller Caller, id string, in InstanceWriteInput) (InstanceView, error) {
	if _, err := s.store.RequireInstanceAccess(ctx, accountID, caller.UserID, id,
		"omnichannel.instances.manage", InstanceGrantManage); err != nil {
		return InstanceView{}, err
	}
	existing, err := s.store.GetInstanceView(ctx, accountID, id)
	if err != nil {
		if noRows(err) {
			return InstanceView{}, ErrSessionUnavailable
		}
		return InstanceView{}, err
	}
	// instanceName: o front sempre envia; ausente/vazio cai no atual (nao deixa a instancia
	// sem nome). Se enviado vazio explicitamente, e body invalido.
	name := existing.InstanceName
	if in.InstanceName != nil {
		name = strings.TrimSpace(*in.InstanceName)
		if name == "" {
			return InstanceView{}, ErrInvalidBody
		}
	}
	policy, err := normalizeScopePolicy(in.UserScopePolicy)
	if err != nil {
		return InstanceView{}, err
	}
	responsible, err := s.resolveResponsible(ctx, accountID, in.ResponsibleUserID)
	if err != nil {
		return InstanceView{}, err
	}
	isActive := derefBool(in.IsActive, existing.IsActive)
	isDefault := derefBool(in.IsDefault, existing.IsDefault)
	if isActive && !existing.IsActive {
		current, countErr := s.store.CountActiveInstances(ctx, accountID)
		if countErr != nil {
			return InstanceView{}, countErr
		}
		if limitErr := s.limits.Check(ctx, accountID, moduleID, limitKeyChannels, int64(current)); limitErr != nil {
			if modules.IsLimitExceeded(limitErr) {
				return InstanceView{}, ErrChannelLimit
			}
			return InstanceView{}, limitErr
		}
	}

	phone := optTrim(in.PhoneNumber)
	if phone != nil {
		if err := ensureNumberFree(ctx, s.store, accountID, *phone, id); err != nil {
			return InstanceView{}, err
		}
	}

	if err := s.store.UpdateInstance(ctx, accountID, id, instanceWrite{
		InstanceName:      name,
		DisplayName:       optTrim(in.DisplayName),
		PhoneNumber:       phone,
		QueueLabel:        optTrim(in.QueueLabel),
		ResponsibleUserID: responsible,
		IsActive:          isActive,
		UserScopePolicy:   policy,
	}); err != nil {
		return InstanceView{}, mapInstanceWriteError(err)
	}

	if isDefault {
		if err := s.store.PromoteDefault(ctx, accountID, id); err != nil {
			return InstanceView{}, err
		}
	} else if err := s.store.SetInstanceNotDefault(ctx, accountID, id); err != nil {
		return InstanceView{}, err
	}
	if err := s.applyCredential(ctx, accountID, id, in.EvolutionAPIKey); err != nil {
		return InstanceView{}, err
	}
	s.publisher.PublishOmnichannelEvent(ctx, newInvalidationSignal(
		accountID, RealtimeInvalidationReasonAccessScopeChanged, time.Now().UTC()))
	return s.instanceViewForCaller(ctx, accountID, id, caller)
}

// ReplaceInstanceAccess aplica policy/grants pelo gate efetivo e publica a invalidação somente
// depois que o repository confirma o commit. O endpoint HTTP correspondente pertence ao P2.
func (s *SessionService) ReplaceInstanceAccess(ctx context.Context, accountID, instanceID string, caller Caller, in InstanceAccessUpdateInput) (InstanceAccessWriteResult, error) {
	ready, err := s.store.HasActiveOmnichannelMembership(ctx, accountID, caller.UserID)
	if err != nil {
		return InstanceAccessWriteResult{}, err
	}
	if !ready {
		return InstanceAccessWriteResult{}, ErrForbidden
	}
	canManage, err := s.store.hasEffectivePermission(ctx, accountID, caller.UserID, "omnichannel.instances.manage")
	if err != nil {
		return InstanceAccessWriteResult{}, err
	}
	if !canManage {
		return InstanceAccessWriteResult{}, ErrForbidden
	}
	if _, err := s.store.RequireInstanceAccess(ctx, accountID, caller.UserID, instanceID,
		"omnichannel.instances.manage", InstanceGrantManage); err != nil {
		return InstanceAccessWriteResult{}, err
	}
	result, err := s.store.ReplaceInstanceAccess(ctx, InstanceAccessWrite{
		AccountID: accountID, InstanceID: instanceID, ActorUserID: caller.UserID,
		ResponsibleUserID: in.ResponsibleUserID, AccessPolicy: in.AccessPolicy,
		ExpectedRevision: in.ExpectedRevision, Grants: in.Grants,
	})
	if err != nil {
		return InstanceAccessWriteResult{}, err
	}
	if result.Changed {
		s.publisher.PublishOmnichannelEvent(ctx, newInvalidationSignal(
			accountID, RealtimeInvalidationReasonAccessScopeChanged, time.Now().UTC()))
	}
	return result, nil
}

// GetInstanceAccess devolve o estado relacional autoritativo para o card administrativo.
// A propria permissao de feature nao basta: o chamador tambem precisa de grant manage na
// instancia solicitada.
func (s *SessionService) GetInstanceAccess(ctx context.Context, accountID, instanceID string, caller Caller) (InstanceAccessAdminView, error) {
	decision, err := s.requireInstanceAccessAdministration(ctx, accountID, instanceID, caller)
	if err != nil {
		return InstanceAccessAdminView{}, err
	}
	return s.instanceAccessAdminView(ctx, accountID, instanceID, decision.Capabilities)
}

// PutInstanceAccess aplica o contrato novo ou traduz temporariamente userIds para grants
// reply. A resposta sempre e relida do PostgreSQL depois do commit; o frontend nunca precisa
// completar revision/capabilities localmente.
func (s *SessionService) PutInstanceAccess(ctx context.Context, accountID, instanceID string, caller Caller, in InstanceAccessRequest) (InstanceAccessAdminView, error) {
	decision, err := s.requireInstanceAccessAdministration(ctx, accountID, instanceID, caller)
	if err != nil {
		return InstanceAccessAdminView{}, err
	}

	current, err := s.store.GetInstanceAccessState(ctx, accountID, instanceID)
	if err != nil {
		return InstanceAccessAdminView{}, err
	}
	update, err := normalizeInstanceAccessRequest(in, current)
	if err != nil {
		return InstanceAccessAdminView{}, err
	}
	result, err := s.ReplaceInstanceAccess(ctx, accountID, instanceID, caller, update)
	if err != nil {
		return InstanceAccessAdminView{}, err
	}
	if result.Changed {
		var scope ConversationAccessScope
		scope, err = s.store.LoadConversationAccessScope(ctx, accountID, caller.UserID)
		if err != nil {
			return InstanceAccessAdminView{}, err
		}
		decision, _ = scope.instanceDecision(instanceID)
	}
	return s.instanceAccessAdminView(ctx, accountID, instanceID, decision.Capabilities)
}

func (s *SessionService) requireInstanceAccessAdministration(ctx context.Context, accountID, instanceID string, caller Caller) (InstanceAccessDecision, error) {
	ready, err := s.store.HasActiveOmnichannelMembership(ctx, accountID, caller.UserID)
	if err != nil {
		return InstanceAccessDecision{}, err
	}
	if !ready {
		return InstanceAccessDecision{}, ErrForbidden
	}
	return s.store.RequireInstanceAccess(ctx, accountID, caller.UserID, instanceID,
		"omnichannel.instances.manage", InstanceGrantManage)
}

func (s *SessionService) instanceAccessAdminView(ctx context.Context, accountID, instanceID string, capabilities InstanceCapabilities) (InstanceAccessAdminView, error) {
	state, err := s.store.GetInstanceAccessState(ctx, accountID, instanceID)
	if err != nil {
		return InstanceAccessAdminView{}, err
	}
	grants := make([]InstanceUserGrantView, 0, len(state.Grants))
	for _, grant := range state.Grants {
		grants = append(grants, InstanceUserGrantView{
			UserID: grant.UserID, AccessLevel: string(grant.AccessLevel),
			IsActive: grant.IsActive, Revision: grant.Revision,
		})
	}
	return InstanceAccessAdminView{
		AccessRevision: state.AccessRevision, AccessPolicy: string(state.AccessPolicy),
		ResponsibleUserID: state.ResponsibleUserID, Grants: grants,
		MyCapabilities: capabilities,
	}, nil
}

func normalizeInstanceAccessRequest(in InstanceAccessRequest, current storedInstanceAccess) (InstanceAccessUpdateInput, error) {
	if in.UserIDs != nil && (in.AccessRevision != nil || in.AccessPolicy != nil || in.ResponsibleUserID != nil || in.Grants != nil) {
		return InstanceAccessUpdateInput{}, ErrInvalidBody
	}
	if in.UserIDs != nil {
		grants := make([]InstanceGrantInput, 0, len(*in.UserIDs)+1)
		levels := make(map[string]InstanceGrantLevel, len(current.Grants)+len(*in.UserIDs))
		for _, grant := range current.Grants {
			if grant.IsActive && grant.AccessLevel == InstanceGrantManage {
				levels[grant.UserID] = InstanceGrantManage
			}
		}
		for _, rawUserID := range *in.UserIDs {
			userID := strings.TrimSpace(rawUserID)
			if userID != "" {
				if levels[userID] != InstanceGrantManage {
					levels[userID] = InstanceGrantReply
				}
			}
		}
		for userID, level := range levels {
			grants = append(grants, InstanceGrantInput{UserID: userID, AccessLevel: level})
		}
		responsible := ""
		if current.ResponsibleUserID != nil {
			responsible = *current.ResponsibleUserID
		}
		return InstanceAccessUpdateInput{
			AccessPolicy: current.AccessPolicy, ExpectedRevision: current.AccessRevision,
			ResponsibleUserID: responsible, Grants: grants,
		}, nil
	}
	if in.AccessRevision == nil || in.AccessPolicy == nil || in.Grants == nil {
		return InstanceAccessUpdateInput{}, ErrInvalidBody
	}
	responsible := ""
	if in.ResponsibleUserID != nil {
		responsible = strings.TrimSpace(*in.ResponsibleUserID)
	}
	return InstanceAccessUpdateInput{
		AccessPolicy: *in.AccessPolicy, ExpectedRevision: *in.AccessRevision,
		ResponsibleUserID: responsible, Grants: *in.Grants,
	}, nil
}

// ============================================================================
// Helpers
// ============================================================================

func (s *SessionService) instanceViewForCaller(ctx context.Context, accountID, id string, caller Caller) (InstanceView, error) {
	view, err := s.store.GetInstanceView(ctx, accountID, id)
	if err != nil {
		return InstanceView{}, err
	}
	scope, err := s.store.LoadConversationAccessScope(ctx, accountID, caller.UserID)
	if err != nil {
		return InstanceView{}, err
	}
	decision, ok := scope.instanceDecision(id)
	if !ok {
		return InstanceView{}, ErrNotFound
	}
	view.MyCapabilities = decision.Capabilities
	return view, nil
}

// resolveResponsible valida o responsible_user_id contra a membership da conta (isolamento).
// Vazio => sem responsavel (nil). Nao-membro => body invalido (nunca grava um usuario de
// outra conta como responsavel).
func (s *SessionService) resolveResponsible(ctx context.Context, accountID string, raw *string) (*string, error) {
	id := strings.TrimSpace(deref(raw))
	if id == "" {
		return nil, nil
	}
	member, err := s.store.IsAccountMember(ctx, accountID, id)
	if err != nil {
		return nil, err
	}
	if !member {
		return nil, ErrInvalidBody
	}
	return &id, nil
}

// applyCredential cifra e grava a evolutionApiKey QUANDO presente e nao-vazia (so-se-presente:
// ausente = mantem a credencial atual). A chave crua nunca volta ao front nem vai a log.
func (s *SessionService) applyCredential(ctx context.Context, accountID, id string, raw *string) error {
	apiKey := strings.TrimSpace(deref(raw))
	if apiKey == "" {
		return nil
	}
	if s.secretBox == nil {
		return errors.New("omnichannel: secretbox nao inicializado")
	}
	ciphertext, err := s.secretBox.Encrypt(apiKey)
	if err != nil {
		return err
	}
	return s.store.SetInstanceCredentials(ctx, accountID, id, ciphertext)
}

// normalizeScopePolicy valida o valor contra os dois que o front tipa; ausente => default.
func normalizeScopePolicy(p *string) (string, error) {
	v := strings.TrimSpace(deref(p))
	if v == "" {
		return userScopePolicyMultiInstance, nil
	}
	switch v {
	case userScopePolicyMultiInstance, userScopePolicySingleInstance:
		return v, nil
	default:
		return "", ErrInvalidBody
	}
}

// optTrim devolve nil para *string ausente OU vazio-apos-trim; senao o valor aparado. Usado
// nos campos nullable de texto (display_name/phone_number/queue_label): vazio grava NULL.
func optTrim(p *string) *string {
	if p == nil {
		return nil
	}
	v := strings.TrimSpace(*p)
	if v == "" {
		return nil
	}
	return &v
}

func derefBool(p *bool, fallback bool) bool {
	if p == nil {
		return fallback
	}
	return *p
}
