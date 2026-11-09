package calendar

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type assistantPlusStore struct{ pool *pgxpool.Pool }
type assistantPlusClaim struct {
	ID, ConversationID string
	Response           json.RawMessage
}

func (s *assistantPlusStore) validateConversation(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, scope AssistantPlusScope, id string) error {
	if !uuidRe.MatchString(id) {
		return plusChatError("not_found", 404)
	}
	var issuer string
	e := q.QueryRow(ctx, `select coalesce(issuer_id::text,'') from calendar.chat_conversations where account_id=$1::uuid and created_by_user_id=$2::uuid and id=$3::uuid and deleted_at is null and entry_surface='finance' and assistant_mode='plus'`, scope.AccountID, scope.ActorID, id).Scan(&issuer)
	if errors.Is(e, pgx.ErrNoRows) {
		return plusChatError("not_found", 404)
	}
	if e != nil {
		return e
	}
	if issuer != scope.IssuerID {
		return plusChatError("conversation_context_conflict", 409)
	}
	return nil
}
func (s *assistantPlusStore) claim(ctx context.Context, scope AssistantPlusScope, req ChatAskRequest, hash []byte) (assistantPlusClaim, error) {
	var out assistantPlusClaim
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return out, e
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var locked bool
	e = tx.QueryRow(ctx, `select pg_try_advisory_xact_lock(hashtextextended($1,0))`, "plus:"+scope.AccountID+":"+scope.ActorID+":"+req.IdempotencyKey).Scan(&locked)
	if e != nil {
		return out, e
	}
	if !locked {
		return out, plusChatError("request_in_progress", 409)
	}
	var previous []byte
	var status, mode string
	e = tx.QueryRow(ctx, `select id::text,coalesce(conversation_id::text,''),request_hash,status,assistant_mode,response_snapshot from calendar.chat_ask_requests where account_id=$1::uuid and actor_user_id=$2::uuid and idempotency_key=$3 for update`, scope.AccountID, scope.ActorID, req.IdempotencyKey).Scan(&out.ID, &out.ConversationID, &previous, &status, &mode, &out.Response)
	if e == nil {
		if mode != "plus" || !bytes.Equal(previous, hash) {
			return out, plusChatError("idempotency_conflict", 409)
		}
		if e = s.validateConversation(ctx, tx, scope, out.ConversationID); e != nil {
			return out, e
		}
		if status != "succeeded" {
			return out, plusChatError("request_in_progress", 409)
		}
		return out, tx.Commit(ctx)
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return out, e
	}
	out.Response = nil
	conversation := req.ConversationID
	if conversation != "" {
		if e = s.validateConversation(ctx, tx, scope, conversation); e != nil {
			return out, e
		}
	} else {
		title := []rune(strings.TrimSpace(req.Question))
		if len(title) > 80 {
			title = title[:80]
		}
		e = tx.QueryRow(ctx, `insert into calendar.chat_conversations(account_id,created_by_user_id,title,entry_surface,scope_mode,scope_client_id,assistant_mode,issuer_id,contract_version) values($1::uuid,$2::uuid,$3,'finance','client',$1::uuid,'plus',nullif($4,'')::uuid,'1.0') returning id::text`, scope.AccountID, scope.ActorID, string(title), scope.IssuerID).Scan(&conversation)
		if e != nil {
			return out, e
		}
	}
	out.ConversationID = conversation
	e = tx.QueryRow(ctx, `insert into calendar.chat_ask_requests(account_id,actor_user_id,idempotency_key,request_hash,status,requested_conversation_id,conversation_id,entry_surface,scope_mode,scope_client_id,assistant_mode) values($1::uuid,$2::uuid,$3,$4,'executing',nullif($5,'')::uuid,$6::uuid,'finance','client',$1::uuid,'plus') returning id::text`, scope.AccountID, scope.ActorID, req.IdempotencyKey, hash, req.ConversationID, conversation).Scan(&out.ID)
	if e != nil {
		return out, e
	}
	return out, tx.Commit(ctx)
}
func (s *assistantPlusStore) complete(ctx context.Context, scope AssistantPlusScope, claim assistantPlusClaim, question string, response json.RawMessage) error {
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if e = s.validateConversation(ctx, tx, scope, claim.ConversationID); e != nil {
		return e
	}
	var envelope struct {
		Message string `json:"message"`
	}
	if e = json.Unmarshal(response, &envelope); e != nil {
		return e
	}
	for _, message := range []struct{ role, text string }{{"user", question}, {"assistant", envelope.Message}} {
		_, e = tx.Exec(ctx, `insert into calendar.chat_messages(account_id,conversation_id,role,content,plus_envelope,created_at) values($1::uuid,$2::uuid,$3,$4,$5,clock_timestamp())`, scope.AccountID, claim.ConversationID, message.role, message.text, response)
		if e != nil {
			return e
		}
	}
	_, e = tx.Exec(ctx, `update calendar.chat_conversations set updated_at=clock_timestamp() where account_id=$1::uuid and id=$2::uuid and created_by_user_id=$3::uuid`, scope.AccountID, claim.ConversationID, scope.ActorID)
	if e != nil {
		return e
	}
	tag, e := tx.Exec(ctx, `update calendar.chat_ask_requests set status='succeeded',response_snapshot=$4,completed_at=now(),updated_at=now() where account_id=$1::uuid and actor_user_id=$2::uuid and id=$3::uuid and assistant_mode='plus' and status='executing'`, scope.AccountID, scope.ActorID, claim.ID, response)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		return plusChatError("request_in_progress", 409)
	}
	return tx.Commit(ctx)
}
func (s *assistantPlusStore) fail(ctx context.Context, scope AssistantPlusScope, id string) error {
	_, e := s.pool.Exec(ctx, `update calendar.chat_ask_requests set status='unknown',error_code='tool_outcome_unknown',updated_at=now() where account_id=$1::uuid and actor_user_id=$2::uuid and id=$3::uuid and assistant_mode='plus' and status='executing'`, scope.AccountID, scope.ActorID, id)
	return e
}
