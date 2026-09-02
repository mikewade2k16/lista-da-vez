package omnichannel

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const maxAIReplyDraftRunes = 4000

// AIReplyDraftView e a sugestao operacional do modo assist. Ela nunca e uma mensagem:
// somente um POST humano pode transforma-la em outbound + outbox do provider.
type AIReplyDraftView struct {
	ID             string     `json:"id"`
	ConversationID string     `json:"conversationId"`
	Generation     int64      `json:"generation"`
	Content        string     `json:"content"`
	Status         string     `json:"status"`
	Edited         bool       `json:"edited"`
	CreatedAt      time.Time  `json:"createdAt"`
	DecidedAt      *time.Time `json:"decidedAt"`
}

type AIReplyDraftEnvelope struct {
	Draft *AIReplyDraftView `json:"draft"`
}

func scanAIReplyDraft(row rowScanner) (AIReplyDraftView, error) {
	var out AIReplyDraftView
	err := row.Scan(
		&out.ID, &out.ConversationID, &out.Generation, &out.Content,
		&out.Status, &out.Edited, &out.CreatedAt, &out.DecidedAt,
	)
	return out, err
}

const aiReplyDraftColumns = `id::text,conversation_id::text,generation,content,status,edited,created_at,decided_at`

func normalizeAIReplyDraftContent(value string) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) > maxAIReplyDraftRunes {
		value = string(runes[:maxAIReplyDraftRunes])
	}
	return strings.TrimSpace(value)
}

// CompleteAIDispatchWithReplyDraft materializa a sugestao e conclui o dispatch na mesma
// transacao. O lock compartilhado da config impede que uma mudanca para paused/shadow/active
// passe entre o gate assist e o INSERT; o fence da instancia impede conteudo anterior ao reset.
func (s *Store) CompleteAIDispatchWithReplyDraft(
	ctx context.Context,
	accountID, dispatchID string,
	generation int64,
	resultRunID, content string,
) (AIReplyDraftView, bool, error) {
	accountID = strings.TrimSpace(accountID)
	dispatchID = strings.TrimSpace(dispatchID)
	content = normalizeAIReplyDraftContent(content)
	if accountID == "" || dispatchID == "" || generation < 0 || content == "" {
		return AIReplyDraftView{}, false, ErrAIDispatchInvalidInput
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return AIReplyDraftView{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var rolloutMode string
	err = tx.QueryRow(ctx, `select mode from messaging.rollout_configs
		where account_id=$1::uuid for key share`, accountID).Scan(&rolloutMode)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && rolloutMode != RolloutModeAssist) {
		return AIReplyDraftView{}, false, nil
	}
	if err != nil {
		return AIReplyDraftView{}, false, err
	}

	var conversationID string
	err = tx.QueryRow(ctx, `select conversation_id::text from messaging.ai_dispatches
		where account_id=$1::uuid and id=$2::uuid and generation=$3`,
		accountID, dispatchID, generation).Scan(&conversationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return AIReplyDraftView{}, false, nil
	}
	if err != nil {
		return AIReplyDraftView{}, false, err
	}
	if err := lockHistoryExternalEffectScope(ctx, tx, accountID, conversationID, "update"); err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, ErrHistoryResetInvalidated) {
			return AIReplyDraftView{}, false, nil
		}
		return AIReplyDraftView{}, false, err
	}

	var allowed bool
	err = tx.QueryRow(ctx, `select exists (
		select 1 from messaging.ai_dispatches dispatch
		join messaging.conversations conversation
		  on conversation.account_id=dispatch.account_id and conversation.id=dispatch.conversation_id
		where dispatch.account_id=$1::uuid and dispatch.id=$2::uuid
		  and dispatch.generation=$3 and dispatch.status='processing'
		  and conversation.ai_generation=dispatch.generation and conversation.state='ai_active'
		  and cardinality(dispatch.message_ids)>0
		  and not exists (
			select 1 from unnest(dispatch.message_ids) captured(message_id)
			where not exists (
				select 1 from messaging.messages history_message
				where history_message.account_id=dispatch.account_id
				  and history_message.conversation_id=dispatch.conversation_id
				  and history_message.id=captured.message_id`+
		s.historyVisibleMessagePredicate("history_message", "conversation")+`
			)
		  )
	)`, accountID, dispatchID, generation).Scan(&allowed)
	if err != nil || !allowed {
		return AIReplyDraftView{}, false, err
	}

	if _, err := tx.Exec(ctx, `update messaging.ai_reply_drafts
		set status='expired',decision_reason='superseded',decided_at=now(),updated_at=now()
		where account_id=$1::uuid and conversation_id=$2::uuid and status='pending'
		  and dispatch_id<>$3::uuid`, accountID, conversationID, dispatchID); err != nil {
		return AIReplyDraftView{}, false, err
	}

	draft, err := scanAIReplyDraft(tx.QueryRow(ctx, `insert into messaging.ai_reply_drafts
		(account_id,conversation_id,dispatch_id,generation,content)
		values ($1::uuid,$2::uuid,$3::uuid,$4,$5)
		on conflict (account_id,dispatch_id) do update set
		 content=excluded.content,generation=excluded.generation,status='pending',
		 used_message_id=null,decided_by_user_id=null,decision_reason='',edited=false,
		 decided_at=null,updated_at=now()
		returning `+aiReplyDraftColumns,
		accountID, conversationID, dispatchID, generation, content))
	if err != nil {
		return AIReplyDraftView{}, false, err
	}
	result, err := tx.Exec(ctx, `update messaging.ai_dispatches
		set status='completed',completed_at=now(),result_run_id=nullif($4,'')::uuid,
		    locked_at=null,updated_at=now()
		where account_id=$1::uuid and id=$2::uuid and generation=$3 and status='processing'`,
		accountID, dispatchID, generation, strings.TrimSpace(resultRunID))
	if err != nil {
		return AIReplyDraftView{}, false, err
	}
	if result.RowsAffected() != 1 {
		return AIReplyDraftView{}, false, nil
	}
	if err := tx.Commit(ctx); err != nil {
		return AIReplyDraftView{}, false, err
	}
	return draft, true, nil
}

func (s *Store) GetPendingAIReplyDraft(ctx context.Context, accountID, conversationID string) (AIReplyDraftView, bool, error) {
	draft, err := scanAIReplyDraft(s.pool.QueryRow(ctx, `select `+aiReplyDraftColumns+`
		from messaging.ai_reply_drafts
		where account_id=$1::uuid and conversation_id=$2::uuid and status='pending'
		order by created_at desc,id desc limit 1`, accountID, conversationID))
	if errors.Is(err, pgx.ErrNoRows) {
		return AIReplyDraftView{}, false, nil
	}
	return draft, err == nil, err
}

func (s *Store) DismissAIReplyDraft(ctx context.Context, accountID, conversationID, draftID, actorUserID, reason string) (bool, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "operator_dismissed"
	}
	if len([]rune(reason)) > 500 {
		return false, ErrInvalidBody
	}
	result, err := s.pool.Exec(ctx, `update messaging.ai_reply_drafts
		set status='dismissed',decided_by_user_id=nullif($4,'')::uuid,
		    decision_reason=$5,decided_at=now(),updated_at=now()
		where account_id=$1::uuid and conversation_id=$2::uuid and id=$3::uuid
		  and status='pending'`, accountID, conversationID, draftID, actorUserID, reason)
	return result.RowsAffected() == 1, err
}

// resolveHumanAIReplyDraftTx fecha a metrica assist dentro da mesma transacao do outbound.
// Sem draft selecionado, qualquer sugestao pendente e expirada porque o humano respondeu por fora.
func resolveHumanAIReplyDraftTx(ctx context.Context, tx pgx.Tx, in outboundMessageInsert, messageID string) error {
	draftID := strings.TrimSpace(in.AIReplyDraftID)
	if draftID != "" {
		result, err := tx.Exec(ctx, `update messaging.ai_reply_drafts
			set status='used',used_message_id=$4::uuid,
			    decided_by_user_id=nullif($5,'')::uuid,decision_reason='operator_used',
			    edited=(btrim(content)<>btrim($6)),decided_at=now(),updated_at=now()
			where account_id=$1::uuid and conversation_id=$2::uuid and id=$3::uuid
			  and status='pending'`, in.AccountID, in.ConversationID, draftID,
			messageID, in.SenderUserID, in.Content)
		if err != nil {
			return err
		}
		if result.RowsAffected() != 1 {
			return ErrInvalidBody
		}
	}
	_, err := tx.Exec(ctx, `update messaging.ai_reply_drafts
		set status='expired',decided_by_user_id=nullif($3,'')::uuid,
		    decision_reason='human_reply',decided_at=now(),updated_at=now()
		where account_id=$1::uuid and conversation_id=$2::uuid and status='pending'`,
		in.AccountID, in.ConversationID, in.SenderUserID)
	return err
}
