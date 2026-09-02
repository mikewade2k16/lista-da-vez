package omnichannel

import (
	"context"
	"strings"
)

// Regras de contatos (CRUD do inbox). Ver service_admin.go para conta/instancias.

// ============================================================================
// Contatos
// ============================================================================

// ListContacts devolve os contatos da account.
func (s *Service) ListContacts(ctx context.Context, accountID string, caller Caller) ([]ContactView, error) {
	visibility, err := s.resolveConversationVisibility(ctx, accountID, caller.UserID,
		"omnichannel.conversations.view", InstanceGrantView)
	if err != nil {
		return nil, err
	}
	rows, err := s.store.ListVisibleContacts(ctx, accountID, visibility)
	if err != nil {
		return nil, err
	}
	out := make([]ContactView, 0, len(rows))
	for _, row := range rows {
		out = append(out, contactView(row))
	}
	return out, nil
}

// contactView projeta a linha crua: o `state` da ultima conversa vira `status` (o front
// tipa lastConversationStatus como ConversationStatus, nao como state).
func contactView(row contactRow) ContactView {
	var lastStatus *string
	if row.LastConversationState != nil {
		s := string(projectStatus(*row.LastConversationState))
		lastStatus = &s
	}
	return ContactView{
		ID:                      row.ID,
		TenantID:                row.AccountID,
		Name:                    row.Name,
		Phone:                   row.Phone,
		AvatarURL:               row.AvatarURL,
		Source:                  row.Source,
		CreatedAt:               row.CreatedAt,
		UpdatedAt:               row.UpdatedAt,
		LastConversationID:      row.LastConversationID,
		LastConversationAt:      row.LastConversationAt,
		LastConversationChannel: row.LastConversationChannel,
		LastConversationStatus:  lastStatus,
	}
}

// CreateContact grava um contato (upsert por account+telefone, como o legado) e devolve
// { contact, conversation } — o shape que o front le
// (useOmnichannelInboxContactActions.ts:41-56).
//
// Quando vem `conversationId`, os dados faltantes sao herdados da conversa e a conversa
// recebe os dados do contato de volta. A conversa e resolvida DENTRO da account:
// conversationId de outra conta => 404, nunca 403.
//
// A publicacao do realtime `conversation.updated` que o legado faz aqui e da F5 — esta
// fase nao emite evento nenhum.
func (s *Service) CreateContact(ctx context.Context, accountID string, caller Caller, in ContactInput) (SaveContactView, error) {
	access, err := s.store.LoadConversationAccessScope(ctx, accountID, caller.UserID)
	if err != nil {
		return SaveContactView{}, err
	}
	if !access.Eligible || !access.allowsPermission("omnichannel.contacts.manage") {
		return SaveContactView{}, ErrForbidden
	}
	visibility := access.conversationVisibility(InstanceGrantView)
	phone := strings.TrimSpace(in.Phone)
	name := strings.TrimSpace(in.Name)
	avatar := normalizeAvatar(in.AvatarURL)
	conversationID := strings.TrimSpace(in.ConversationID)

	if conversationID == "" {
		return SaveContactView{}, ErrNotFound
	}
	if conversationID != "" {
		conv, err := s.store.GetVisibleConversation(ctx, accountID, visibility, conversationID)
		if err != nil {
			return SaveContactView{}, translate(err)
		}
		if phone == "" {
			phone = firstNonEmpty(deref(conv.ContactPhone), conv.ExternalID)
		}
		if name == "" {
			name = deref(conv.ContactName)
		}
		if avatar == nil {
			avatar = conv.ContactAvatarURL
		}
	}

	phone = normalizePhoneDigits(phone)
	if phone == "" {
		return SaveContactView{}, ErrInvalidPhone
	}
	name = normalizeContactName(name, phone)

	source := strings.TrimSpace(in.Source)
	if source == "" {
		source = "MANUAL"
	}

	id, err := s.store.UpsertContact(ctx, accountID, name, phone, source, avatar)
	if err != nil {
		return SaveContactView{}, err
	}
	// Amarra todas as conversas com esse telefone ao contato (legado: updateMany).
	if err := s.store.LinkVisibleConversationsByPhone(ctx, accountID, phone, id, visibility); err != nil {
		return SaveContactView{}, err
	}

	row, err := s.store.GetContact(ctx, accountID, id)
	if err != nil {
		return SaveContactView{}, translate(err)
	}
	out := SaveContactView{Contact: contactView(row)}

	// Sem conversationId => conversation: null. O front trata (recarrega a lista).
	if conversationID == "" {
		return out, nil
	}
	if err := s.store.UpdateVisibleConversationContact(ctx, accountID, conversationID, id, name, phone, avatar, visibility); err != nil {
		return SaveContactView{}, translate(err)
	}
	convRow, err := s.store.GetVisibleConversation(ctx, accountID, visibility, conversationID)
	if err != nil {
		return SaveContactView{}, translate(err)
	}
	convView, err := conversationView(convRow)
	if err != nil {
		return SaveContactView{}, err
	}
	out.Conversation = &convView
	return out, nil
}

// UpdateContact aplica o PATCH. Campo ausente = nao mexe; avatarUrl null = limpa
// (o legado distingue os dois casos — replicar).
func (s *Service) UpdateContact(ctx context.Context, accountID string, caller Caller, id string, patch ContactPatch) (ContactView, error) {
	access, err := s.store.LoadConversationAccessScope(ctx, accountID, caller.UserID)
	if err != nil {
		return ContactView{}, err
	}
	if !access.Eligible || !access.allowsPermission("omnichannel.contacts.manage") {
		return ContactView{}, ErrForbidden
	}
	visibility := access.conversationVisibility(InstanceGrantView)
	existing, err := s.store.GetVisibleContact(ctx, accountID, id, visibility)
	if err != nil {
		return ContactView{}, translate(err)
	}

	phone := existing.Phone
	if patch.Phone != nil {
		phone = normalizePhoneDigits(*patch.Phone)
		if phone == "" {
			return ContactView{}, ErrInvalidPhone
		}
	}

	name := existing.Name
	if patch.Name != nil {
		name = *patch.Name
	}
	name = normalizeContactName(name, phone)

	avatar := existing.AvatarURL
	if patch.AvatarURL != nil {
		avatar = normalizeAvatar(*patch.AvatarURL)
	}

	if err := s.store.UpdateContact(ctx, accountID, id, name, phone, avatar); err != nil {
		return ContactView{}, err
	}
	// Telefone mudou: propaga para as conversas do contato (o legado faz o mesmo).
	if phone != existing.Phone {
		if err := s.store.SyncVisibleConversationPhone(ctx, accountID, id, phone, visibility); err != nil {
			return ContactView{}, err
		}
	}

	row, err := s.store.GetVisibleContact(ctx, accountID, id, visibility)
	if err != nil {
		return ContactView{}, translate(err)
	}
	return contactView(row), nil
}

// ============================================================================
// Helpers
// ============================================================================

// normalizePhoneDigits deixa so os digitos (o legado guarda telefone normalizado e
// dedupa por (account, phone)).
func normalizePhoneDigits(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// normalizeContactName cai para o telefone quando o nome vem vazio (o legado nunca
// grava contato sem nome).
func normalizeContactName(name, phone string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return phone
	}
	return name
}

// normalizeAvatar devolve nil para string vazia (a coluna e nullable).
func normalizeAvatar(raw string) *string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	return &raw
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
