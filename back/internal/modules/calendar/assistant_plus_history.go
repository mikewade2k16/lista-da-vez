package calendar

import (
	"context"
	"encoding/json"
)

func (s *assistantPlusStore) history(ctx context.Context, scope AssistantPlusScope, id string, page, limit int) (json.RawMessage, error) {
	if id == "" {
		rows, e := s.pool.Query(ctx, `select jsonb_build_object('id',id,'title',title,'surface',entry_surface,'mode',assistant_mode,'issuerId',issuer_id,'createdByUserId',created_by_user_id,'updatedAt',updated_at) from calendar.chat_conversations where account_id=$1::uuid and created_by_user_id=$2::uuid and issuer_id is not distinct from nullif($3,'')::uuid and assistant_mode='plus' and entry_surface='finance' and deleted_at is null order by updated_at desc,id desc limit $4 offset $5`, scope.AccountID, scope.ActorID, scope.IssuerID, limit, (page-1)*limit)
		if e != nil {
			return nil, e
		}
		defer rows.Close()
		out := []json.RawMessage{}
		for rows.Next() {
			var b json.RawMessage
			if e = rows.Scan(&b); e != nil {
				return nil, e
			}
			out = append(out, b)
		}
		if e = rows.Err(); e != nil {
			return nil, e
		}
		return json.Marshal(map[string]any{"conversations": out, "page": page, "limit": limit})
	}
	if e := s.validateConversation(ctx, s.pool, scope, id); e != nil {
		return nil, e
	}
	rows, e := s.pool.Query(ctx, `select jsonb_build_object('id',id,'role',role,'content',content,'createdAt',created_at,'plusEnvelope',case when role='assistant' then plus_envelope else null end) from (select * from calendar.chat_messages where account_id=$1::uuid and conversation_id=$2::uuid order by created_at desc,id desc limit $3 offset $4) recent order by created_at,id`, scope.AccountID, id, limit, (page-1)*limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []json.RawMessage{}
	for rows.Next() {
		var b json.RawMessage
		if e = rows.Scan(&b); e != nil {
			return nil, e
		}
		out = append(out, b)
	}
	if e = rows.Err(); e != nil {
		return nil, e
	}
	return json.Marshal(map[string]any{"id": id, "surface": "finance", "mode": "plus", "issuerId": scope.IssuerID, "messages": out, "page": page, "limit": limit})
}
func (s *assistantPlusStore) remove(ctx context.Context, scope AssistantPlusScope, id string) error {
	if e := s.validateConversation(ctx, s.pool, scope, id); e != nil {
		return e
	}
	_, e := s.pool.Exec(ctx, `update calendar.chat_conversations set deleted_at=clock_timestamp() where account_id=$1::uuid and created_by_user_id=$2::uuid and id=$3::uuid and assistant_mode='plus'`, scope.AccountID, scope.ActorID, id)
	return e
}
