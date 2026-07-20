package omnichannel

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel/channel"
)

// ============================================================================
// F7 — Acoes de MENSAGEM: reaction, forward, delete-for-me, delete-for-all.
// ============================================================================
//
// reaction/delete-for-all sao SINCRONAS ao provider (o legado chama o Evolution na hora — 502
// na falha), NAO vao pelo outbox. forward e a EXCECAO: reusa o caminho de envio da F6
// (SendService -> outbox -> FIFO por conversa) — e o que os campos queued/failedToQueue da
// resposta significam. Nunca um segundo caminho de envio (risco 5 do canonico).

// maxReactionEmojiLen e o teto do contrato do legado ({ emoji?: string(max 32) | null }).
const maxReactionEmojiLen = 32

// React reage a uma mensagem (emoji=nil remove a reacao). Requer conversations.reply. Gate por
// numero (Capabilities): numero sem suporte a reacao => 409 acionavel; mensagem sem
// external_message_id => 409. A reacao e SINCRONA ao provider (SendReaction); falha de
// transporte/HTTP do provider => 502 (ErrProviderUnavailable).
func (a *ActionsService) React(ctx context.Context, accountID string, p auth.Principal, convID, messageID string, emoji *string) error {
	if err := a.svc.requirePermission(ctx, accountID, p, "omnichannel.conversations.reply"); err != nil {
		return err
	}
	if emoji != nil && len(*emoji) > maxReactionEmojiLen {
		return ErrInvalidBody
	}
	if _, err := a.resolveConversation(ctx, accountID, p, convID); err != nil {
		return err
	}
	msgs, err := a.store.ListActionMessages(ctx, accountID, convID, []string{messageID})
	if err != nil {
		return err
	}
	if len(msgs) == 0 {
		return ErrNotFound // mensagem nao pertence a conversa+conta
	}
	target, err := a.store.ConversationChannelTarget(ctx, accountID, convID)
	if err != nil {
		return translate(err)
	}
	if err := a.assertProviderCapability(target.Provider, capReaction); err != nil {
		return err
	}
	if msgs[0].ExternalID == nil || strings.TrimSpace(*msgs[0].ExternalID) == "" {
		return ErrMessageNotSent
	}
	// Envio SINCRONO ao provider (F7 ligada — nao passa pelo outbox). O gate de
	// Capabilities().SupportsReaction ja barrou numero sem suporte (assertProviderCapability
	// acima); aqui a chamada real. Falha de transporte/HTTP => ErrProviderUnavailable (502).
	prov, err := a.resolveProvider(target.Provider)
	if err != nil {
		return err
	}
	cred, err := a.resolveActionCredentials(ctx, accountID, target.Provider)
	if err != nil {
		return err
	}
	reactionEmoji := ""
	if emoji != nil {
		reactionEmoji = *emoji
	}
	if rErr := prov.SendReaction(ctx, cred, channel.ReactionInput{
		InstanceName:      target.InstanceScopeKey,
		RemoteJID:         target.ExternalID,
		ExternalMessageID: strings.TrimSpace(*msgs[0].ExternalID),
		FromMe:            msgs[0].Direction == directionOutbound,
		Emoji:             reactionEmoji,
	}); rErr != nil {
		// Erro generico do adapter (sem body/chave — canonico §10); logamos so identificadores.
		a.logger.Error("omnichannel_reaction_provider_failed", "account_id", accountID, "conversation_id", convID)
		return ErrProviderUnavailable
	}
	return nil
}

// directionOutbound e o rotulo de mensagem de saida em messaging.messages.direction (usado no
// FromMe das acoes sincronas e no filtro de elegibilidade do delete-for-all).
const directionOutbound = "OUTBOUND"

// ForwardResult e a resposta de POST .../messages/forward (contrato verbatim do legado).
type ForwardResult struct {
	SourceConversationID string        `json:"sourceConversationId"`
	TargetConversationID string        `json:"targetConversationId"`
	CreatedCount         int           `json:"createdCount"`
	QueuedCount          int           `json:"queuedCount"`
	FailedToQueueCount   int           `json:"failedToQueueCount"`
	FailedToQueueIDs     []string      `json:"failedToQueueIds"`
	Messages             []MessageView `json:"messages"`
}

// Forward encaminha 1..100 mensagens para outra conversa, reusando o outbox da F6 (uma
// SendMessage por mensagem no destino). Valida AS DUAS conversas no escopo do ator (origem E
// destino, cada uma com seu 404). Requer conversations.reply.
func (a *ActionsService) Forward(ctx context.Context, accountID string, p auth.Principal, sourceConvID string, messageIDs []string, targetConvID string) (ForwardResult, error) {
	if err := a.svc.requirePermission(ctx, accountID, p, "omnichannel.conversations.reply"); err != nil {
		return ForwardResult{}, err
	}
	if _, err := a.resolveConversation(ctx, accountID, p, sourceConvID); err != nil {
		return ForwardResult{}, err
	}
	if _, err := a.resolveConversation(ctx, accountID, p, targetConvID); err != nil {
		return ForwardResult{}, err
	}
	caller := Caller{UserID: p.UserID, IsAdmin: isAdminPrincipal(p)}
	canReply := legacyRole(p.Role) != legacyRoleViewer

	res := ForwardResult{
		SourceConversationID: sourceConvID,
		TargetConversationID: targetConvID,
		FailedToQueueIDs:     []string{},
		Messages:             []MessageView{},
	}
	for _, mid := range messageIDs {
		src, err := a.store.GetMessage(ctx, accountID, p.UserID, sourceConvID, mid)
		if err != nil {
			res.FailedToQueueCount++
			res.FailedToQueueIDs = append(res.FailedToQueueIDs, mid)
			continue
		}
		view, outcome, err := a.send.SendMessage(ctx, accountID, caller, canReply, targetConvID, forwardInput(src, targetConvID, mid))
		if err != nil {
			res.FailedToQueueCount++
			res.FailedToQueueIDs = append(res.FailedToQueueIDs, mid)
			continue
		}
		res.CreatedCount++
		res.Messages = append(res.Messages, view)
		if outcome == outcomeQueued {
			res.QueuedCount++
		} else {
			res.FailedToQueueCount++
			res.FailedToQueueIDs = append(res.FailedToQueueIDs, mid)
		}
	}
	a.auditForward(ctx, accountID, p.UserID, sourceConvID, targetConvID, res.CreatedCount)
	return res, nil
}

// forwardInput reconstroi o body de envio a partir da mensagem de origem. idempotencyKey
// estavel por (destino, origem) => reenviar o mesmo forward nao duplica no outbox.
func forwardInput(src MessageView, targetConvID, sourceMessageID string) SendMessageInput {
	return SendMessageInput{
		Type:                 src.MessageType,
		Content:              src.Content,
		MediaURL:             deref(src.MediaURL),
		MediaMimeType:        deref(src.MediaMimeType),
		MediaFileName:        deref(src.MediaFileName),
		MediaFileSizeBytes:   src.MediaFileSizeBytes,
		MediaCaption:         deref(src.MediaCaption),
		MediaDurationSeconds: src.MediaDurationSeconds,
		IdempotencyKey:       "forward:" + targetConvID + ":" + sourceMessageID,
	}
}

// auditForward grava MESSAGE_FORWARDED (best-effort). O envio em si ja audita
// MESSAGE_OUTBOUND_QUEUED por mensagem no destino (F6).
func (a *ActionsService) auditForward(ctx context.Context, accountID, actorUserID, sourceConvID, targetConvID string, createdCount int) {
	payload, _ := json.Marshal(map[string]any{
		"sourceConversationId": sourceConvID,
		"targetConversationId": targetConvID,
		"createdCount":         createdCount,
		"changedBy":            actorUserID,
	})
	if err := a.store.InsertAudit(ctx, accountID, actorUserID, sourceConvID, "", "MESSAGE_FORWARDED", payload); err != nil {
		a.logger.Error("omnichannel_forward_audit", "account_id", accountID, "error", err.Error())
	}
}

// DeleteForMeResult e a resposta de POST .../messages/delete-for-me.
type DeleteForMeResult struct {
	DeletedIDs   []string          `json:"deletedIds"`
	SkippedIDs   []string          `json:"skippedIds"`
	Conversation *ConversationView `json:"conversation"`
}

// DeleteForMe oculta mensagens SO para o usuario (messaging.hidden_messages) — nada sai da
// conversa nem some do banco. Requer conversations.reply. Ids fora do escopo => skipped.
func (a *ActionsService) DeleteForMe(ctx context.Context, accountID string, p auth.Principal, convID string, messageIDs []string) (DeleteForMeResult, error) {
	if err := a.svc.requirePermission(ctx, accountID, p, "omnichannel.conversations.reply"); err != nil {
		return DeleteForMeResult{}, err
	}
	row, err := a.resolveConversation(ctx, accountID, p, convID)
	if err != nil {
		return DeleteForMeResult{}, err
	}
	deleted, skipped, err := a.store.HideMessages(ctx, accountID, p.UserID, convID, messageIDs)
	if err != nil {
		return DeleteForMeResult{}, err
	}
	view, err := conversationView(row)
	if err != nil {
		return DeleteForMeResult{}, err
	}
	return DeleteForMeResult{DeletedIDs: deleted, SkippedIDs: skipped, Conversation: &view}, nil
}

// DeleteForAllResult e a resposta de POST .../messages/delete-for-all.
type DeleteForAllResult struct {
	UpdatedIDs []string      `json:"updatedIds"`
	SkippedIDs []string      `json:"skippedIds"`
	FailedIDs  []string      `json:"failedIds"`
	Messages   []MessageView `json:"messages"`
}

// DeleteForAll apaga para todos (irreversivel, visivel ao cliente). So mensagens OUTBOUND com
// external_message_id sao elegiveis (ninguem apaga mensagem do cliente); as demais => skipped.
// Sem provider/WhatsApp configurado => 409 (precedente do legado). O apagar e SINCRONO ao
// provider (DeleteForAll): cada elegivel executa; sucesso => updatedIds, falha do provider =>
// failedIds (a rota devolve 200 com os tres conjuntos, nunca 502 no multi-id). Requer
// conversations.reply.
func (a *ActionsService) DeleteForAll(ctx context.Context, accountID string, p auth.Principal, convID string, messageIDs []string) (DeleteForAllResult, error) {
	if err := a.svc.requirePermission(ctx, accountID, p, "omnichannel.conversations.reply"); err != nil {
		return DeleteForAllResult{}, err
	}
	if _, err := a.resolveConversation(ctx, accountID, p, convID); err != nil {
		return DeleteForAllResult{}, err
	}
	target, err := a.store.ConversationChannelTarget(ctx, accountID, convID)
	if err != nil {
		return DeleteForAllResult{}, translate(err)
	}
	prov, err := a.resolveProvider(target.Provider)
	if err != nil {
		return DeleteForAllResult{}, err // 409: conversa sem provider/WhatsApp configurado
	}
	cred, err := a.resolveActionCredentials(ctx, accountID, target.Provider)
	if err != nil {
		return DeleteForAllResult{}, err
	}
	rows, err := a.store.ListActionMessages(ctx, accountID, convID, messageIDs)
	if err != nil {
		return DeleteForAllResult{}, err
	}
	byID := make(map[string]messageActionRow, len(rows))
	for _, r := range rows {
		byID[r.ID] = r
	}
	res := DeleteForAllResult{UpdatedIDs: []string{}, SkippedIDs: []string{}, FailedIDs: []string{}, Messages: []MessageView{}}
	for _, mid := range messageIDs {
		m, ok := byID[mid]
		if !ok || m.Direction != directionOutbound || m.ExternalID == nil || strings.TrimSpace(*m.ExternalID) == "" {
			res.SkippedIDs = append(res.SkippedIDs, mid)
			continue
		}
		// Envio SINCRONO ao provider (F7 ligada): so OUTBOUND com id externo e elegivel; fromMe e
		// sempre true. Falha por-id => failedIds (NUNCA 502 no multi-id — contrato do legado:
		// updatedIds/skippedIds/failedIds no 200). Erro do adapter e generico (sem body/chave).
		if dErr := prov.DeleteForAll(ctx, cred, channel.DeleteInput{
			InstanceName:      target.InstanceScopeKey,
			RemoteJID:         target.ExternalID,
			ExternalMessageID: strings.TrimSpace(*m.ExternalID),
			FromMe:            true,
		}); dErr != nil {
			a.logger.Error("omnichannel_delete_for_all_provider_failed", "account_id", accountID, "conversation_id", convID)
			res.FailedIDs = append(res.FailedIDs, mid)
			continue
		}
		res.UpdatedIDs = append(res.UpdatedIDs, mid)
	}
	if len(res.UpdatedIDs) > 0 {
		a.auditConversation(ctx, accountID, p.UserID, convID, "MESSAGE_DELETED_FOR_ALL", nil, res.UpdatedIDs)
	}
	return res, nil
}

// resolveActionCredentials monta as Credentials de (conta, provider) para as acoes SINCRONAS
// (reaction/delete-for-all): provider_config nao-secreto + token decifrado pelo secretbox. Sem
// secretBox injetado ou sem ciphertext (ex.: mock) => cai no provider_config + fallback de
// ambiente do adapter. Mesma mecanica de inbound/outbound/media. A chave crua NUNCA vai a log
// (canonico §10).
func (a *ActionsService) resolveActionCredentials(ctx context.Context, accountID, provider string) (channel.Credentials, error) {
	cipher, config, found, err := a.store.FindProviderCredential(ctx, accountID, provider)
	if err != nil {
		return channel.Credentials{}, err
	}
	cred := channel.Credentials{Config: config}
	if !found || strings.TrimSpace(cipher) == "" || a.secretBox == nil {
		return cred, nil
	}
	token, err := a.secretBox.Decrypt(cipher)
	if err != nil {
		a.logger.Error("omnichannel_action_credential_decrypt_failed", "account_id", accountID, "provider", provider)
		return channel.Credentials{}, err
	}
	cred.Token = token
	return cred, nil
}
