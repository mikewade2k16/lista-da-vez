package omnichannel

import (
	"context"
	"errors"
	"strings"

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

// SetInstanceUsersInput e o body de PUT /tenant/whatsapp/instances/{id}/users. O front manda
// `userIds` (nao `assignedUserIds`) — ver useOmnichannelAdmin.ts:293.
type SetInstanceUsersInput struct {
	UserIDs []string `json:"userIds"`
}

// CreateInstance cria uma instancia gerenciada e devolve o registro (WhatsAppInstanceRecord).
func (s *SessionService) CreateInstance(ctx context.Context, accountID string, caller Caller, in InstanceWriteInput) (InstanceView, error) {
	if !caller.IsAdmin {
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
	return s.store.GetInstanceView(ctx, accountID, id)
}

// UpdateInstance aplica o PATCH e devolve o registro atualizado. Instancia de outra conta ->
// 404. Full-replace do formulario (menos a credencial, so-se-presente).
func (s *SessionService) UpdateInstance(ctx context.Context, accountID string, caller Caller, id string, in InstanceWriteInput) (InstanceView, error) {
	if !caller.IsAdmin {
		return InstanceView{}, ErrForbidden
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
	return s.store.GetInstanceView(ctx, accountID, id)
}

// SetInstanceUsers grava os usuarios atribuidos (PUT .../users). Filtra a lista para membros
// ativos da conta (isolamento) e devolve o registro atualizado.
func (s *SessionService) SetInstanceUsers(ctx context.Context, accountID string, caller Caller, id string, in SetInstanceUsersInput) (InstanceView, error) {
	if !caller.IsAdmin {
		return InstanceView{}, ErrForbidden
	}
	if _, err := s.store.GetInstanceView(ctx, accountID, id); err != nil {
		if noRows(err) {
			return InstanceView{}, ErrSessionUnavailable
		}
		return InstanceView{}, err
	}
	valid, err := s.store.FilterAccountMemberIDs(ctx, accountID, in.UserIDs)
	if err != nil {
		return InstanceView{}, err
	}
	if err := s.store.SetInstanceAssignedUsers(ctx, accountID, id, valid); err != nil {
		return InstanceView{}, err
	}
	return s.store.GetInstanceView(ctx, accountID, id)
}

// ============================================================================
// Helpers
// ============================================================================

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
