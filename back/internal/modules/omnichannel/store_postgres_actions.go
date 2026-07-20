package omnichannel

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// ============================================================================
// F7 — Persistencia das acoes do inbox (spec OMNI-F7 entrega 4).
// ============================================================================
//
// REGRA DA CASA, sem excecao: TODA query filtra por account_id (defesa em profundidade,
// principio 2). O gate de escopo (instancia + fila) e resolvido ANTES no service; aqui o
// account_id/conversation_id sao a rede de baixo. IDs = string + cast no SQL ($1::uuid).

// HideMessages grava "apagar para mim" em messaging.hidden_messages (ocultacao POR usuario,
// sem efeito externo). So oculta mensagens que existem na conversa+conta; ids de outra
// conversa/conta ou ja ocultos entram em skipped. Uma unica query (sem N+1): o select amarra
// o insert a conversa+conta e o `on conflict do nothing` filtra os ja ocultos — o returning
// devolve exatamente os recem-ocultados (= deletedIds).
func (s *Store) HideMessages(ctx context.Context, accountID, userID, conversationID string, ids []string) (deleted, skipped []string, err error) {
	rows, err := s.pool.Query(ctx, `insert into messaging.hidden_messages (account_id, user_id, message_id)
		select $1::uuid, $2::uuid, m.id
		from messaging.messages m
		where m.account_id = $1::uuid and m.conversation_id = $3::uuid and m.id = any($4::uuid[])
		on conflict (user_id, message_id) do nothing
		returning message_id::text`, accountID, userID, conversationID, ids)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	hidden := make(map[string]bool, len(ids))
	deleted = make([]string, 0, len(ids))
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, nil, err
		}
		hidden[id] = true
		deleted = append(deleted, id)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	skipped = make([]string, 0)
	for _, id := range ids {
		if !hidden[id] {
			skipped = append(skipped, id)
		}
	}
	return deleted, skipped, nil
}

// messageActionRow e a projecao minima de uma mensagem para as acoes que tocam o provider
// (reaction, delete-for-all): o id, o id externo (do provedor) e a direcao.
type messageActionRow struct {
	ID         string
	ExternalID *string
	Direction  string
}

// ListActionMessages carrega (id, external_message_id, direction) das mensagens pedidas que
// pertencem a conversa+conta. Ids fora do escopo simplesmente nao voltam (o service os trata
// como skipped). Serve reaction (um id) e delete-for-all (varios).
func (s *Store) ListActionMessages(ctx context.Context, accountID, conversationID string, ids []string) ([]messageActionRow, error) {
	rows, err := s.pool.Query(ctx, `select m.id::text, m.external_message_id, m.direction
		from messaging.messages m
		where m.account_id = $1::uuid and m.conversation_id = $2::uuid and m.id = any($3::uuid[])`,
		accountID, conversationID, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]messageActionRow, 0, len(ids))
	for rows.Next() {
		var r messageActionRow
		if err := rows.Scan(&r.ID, &r.ExternalID, &r.Direction); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// channelTarget descreve para onde uma acao sincrona vai no provedor: qual provider (chave do
// registry), a instancia (scope key), o telefone do contato e o external_id da conversa (o
// `remoteJid` do WhatsApp, chave das acoes de reaction/delete-for-all). Provider vazio =>
// conversa sem instancia resolvida (o service degrada com 409 acionavel).
type channelTarget struct {
	Provider         string
	InstanceScopeKey string
	ContactPhone     *string
	ExternalID       string
}

// ConversationChannelTarget resolve provider+instancia+telefone+external_id da conversa
// (escopado por conta). Conversa de outra conta => ErrNoRows -> o service traduz para 404.
func (s *Store) ConversationChannelTarget(ctx context.Context, accountID, conversationID string) (channelTarget, error) {
	var t channelTarget
	err := s.pool.QueryRow(ctx, `select coalesce(i.provider, ''), c.instance_scope_key, c.contact_phone, c.external_id
		from messaging.conversations c
		left join messaging.whatsapp_instances i on i.id = c.instance_id and i.account_id = c.account_id
		where c.account_id = $1::uuid and c.id = $2::uuid`, accountID, conversationID,
	).Scan(&t.Provider, &t.InstanceScopeKey, &t.ContactPhone, &t.ExternalID)
	return t, err
}

// FindContactConversationID devolve a conversa mais recente de um contato (por contact_id),
// escopada por conta. found=false quando o contato ainda nao tem conversa.
func (s *Store) FindContactConversationID(ctx context.Context, accountID, contactID string) (string, bool, error) {
	var id string
	err := s.pool.QueryRow(ctx, `select id::text from messaging.conversations
		where account_id = $1::uuid and contact_id = $2::uuid
		order by last_message_at desc limit 1`, accountID, contactID).Scan(&id)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return "", false, nil
	case err != nil:
		return "", false, err
	default:
		return id, true, nil
	}
}

// CreateContactConversation abre uma conversa para o contato (POST contacts/{id}/open-conversation
// quando ainda nao ha uma). Usa a chave natural (account_id, external_id, channel,
// instance_scope_key) com upsert: se ja existir uma conversa com esse telefone naquela
// instancia, devolve-a (idempotente) em vez de duplicar. instance_id/instance_scope_key vem da
// instancia default ativa (ou 'default' quando a conta ainda nao tem numero). channel=WHATSAPP.
func (s *Store) CreateContactConversation(ctx context.Context, accountID, contactID, phone, name string, avatar *string) (conversationRow, error) {
	var instanceID *string
	instanceScopeKey := "default"
	var id, iname string
	err := s.pool.QueryRow(ctx, `select id::text, instance_name from messaging.whatsapp_instances
		where account_id = $1::uuid and is_active = true
		order by is_default desc, instance_name limit 1`, accountID).Scan(&id, &iname)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		// Sem numero cadastrado: conversa fica em 'default' (o inbound re-escopa quando um
		// numero conectar; padrao do webhook de instancia desconhecida).
	case err != nil:
		return conversationRow{}, err
	default:
		instanceID, instanceScopeKey = &id, iname
	}

	var convID string
	err = s.pool.QueryRow(ctx, `insert into messaging.conversations
		(account_id, instance_id, instance_scope_key, contact_id, channel, external_id,
		 contact_name, contact_phone, contact_avatar_url, state, last_message_at)
		values ($1::uuid, nullif($2,'')::uuid, $3, $4::uuid, 'WHATSAPP', $5, $6, $5, $7, 'new', now())
		on conflict (account_id, external_id, channel, instance_scope_key) do update
			set contact_id = excluded.contact_id,
				contact_name = coalesce(nullif(excluded.contact_name, ''), conversations.contact_name),
				updated_at = now()
		returning id::text`,
		accountID, deref(instanceID), instanceScopeKey, contactID, phone, name, avatar,
	).Scan(&convID)
	if err != nil {
		return conversationRow{}, err
	}
	return s.GetConversation(ctx, accountID, convID)
}
