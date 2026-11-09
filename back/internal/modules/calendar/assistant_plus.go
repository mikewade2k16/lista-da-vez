package calendar

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"regexp"
	"strings"
	"time"
)

type AssistantPlusError struct {
	Code   string
	Status int
}

func (e *AssistantPlusError) Error() string       { return e.Code }
func plusChatError(code string, status int) error { return &AssistantPlusError{code, status} }

type AssistantPlusAction struct {
	ToolID string          `json:"toolId"`
	Input  json.RawMessage `json:"input"`
}
type AssistantPlusScope struct{ AccountID, ActorID, IssuerID, RequestID string }
type AssistantPlusPorts struct {
	ValidateOutput func(context.Context, AssistantPlusScope, json.RawMessage) error

	Authorize func(context.Context, AssistantPlusScope) error
	Execute   func(context.Context, AssistantPlusScope, ChatAskRequest) (json.RawMessage, error)
}
type AssistantPlusService struct {
	store *assistantPlusStore
	ports AssistantPlusPorts
}

func WithFinancePlus(ports AssistantPlusPorts) Option { return func(m *Module) { m.plusPorts = ports } }

var plusSecretMention = regexp.MustCompile(`(?i)\b(senha|password|passwd|api[_ -]?key|token|cookies?|authorization|certificado privado)\b`)

func validPlusKey(key string) bool {
	if len(key) < 8 || len(key) > 200 {
		return false
	}
	for _, r := range key {
		if r < 33 || r > 126 {
			return false
		}
	}
	return true
}
func (s *AssistantPlusService) authorize(ctx context.Context, scope AssistantPlusScope) error {
	if s == nil || s.ports.Authorize == nil || s.ports.Execute == nil {
		return plusChatError("capability_unavailable", 503)
	}
	return s.ports.Authorize(ctx, scope)
}
func (s *AssistantPlusService) Ask(ctx context.Context, scope AssistantPlusScope, req ChatAskRequest) (json.RawMessage, error) {
	if req.Mode != "plus" || req.Surface != "finance" || req.ContractVersion != "1.0" || !validPlusKey(req.IdempotencyKey) || len(strings.TrimSpace(req.Question)) == 0 || len(req.Question) > 4000 {
		return nil, plusChatError("invalid_input", 400)
	}
	if req.ScopeMode != "" || req.ScopeClientID != "" || req.ClientID != "" {
		return nil, plusChatError("conversation_context_conflict", 409)
	}
	if plusSecretMention.MatchString(req.Question) || (req.Action != nil && plusSecretMention.Match(req.Action.Input)) {
		return nil, plusChatError("protected_form_required", 400)
	}
	raw, e := json.Marshal(req)
	if e != nil {
		return nil, e
	}
	if len(raw) > 16*1024 {
		return nil, plusChatError("input_too_large", 413)
	}
	if req.IssuerID != "" && !uuidRe.MatchString(req.IssuerID) {
		return nil, plusChatError("not_found", 404)
	}
	if req.ConversationID != "" && !uuidRe.MatchString(req.ConversationID) {
		return nil, plusChatError("not_found", 404)
	}
	scope.IssuerID = req.IssuerID
	if e = s.authorize(ctx, scope); e != nil {
		return nil, e
	}
	var canonical any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if e = decoder.Decode(&canonical); e != nil {
		return nil, e
	}
	raw, e = json.Marshal(canonical)
	if e != nil {
		return nil, e
	}
	digest := sha256.Sum256(raw)
	claim, e := s.store.claim(ctx, scope, req, digest[:])
	if e != nil {
		return nil, e
	}
	if len(claim.Response) > 0 {
		if s.ports.ValidateOutput != nil {
			if e = s.ports.ValidateOutput(ctx, scope, claim.Response); e != nil {
				return nil, e
			}
		}
		return canonicalPlusJSON(claim.Response)
	}
	scope.RequestID = claim.ID
	call, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	output, e := s.ports.Execute(call, scope, req)
	if e != nil {
		markError := s.failRequest(ctx, scope, claim.ID)
		return nil, errors.Join(e, markError)
	}
	var envelope map[string]json.RawMessage
	if len(output) > 64*1024 || json.Unmarshal(output, &envelope) != nil || envelope == nil {
		markError := s.failRequest(ctx, scope, claim.ID)
		return nil, errors.Join(plusChatError("invalid_tool_output", 502), markError)
	}
	envelope["contractVersion"] = json.RawMessage(`"1.0"`)
	envelope["requestId"], _ = json.Marshal(claim.ID)
	envelope["conversationId"], _ = json.Marshal(claim.ConversationID)
	envelope["context"], _ = json.Marshal(map[string]any{"surface": "finance", "mode": "plus", "issuerId": scope.IssuerID})
	output, e = json.Marshal(envelope)
	if e == nil {
		output, e = canonicalPlusJSON(output)
	}
	if e != nil {
		return nil, e
	}
	// Revalidate access before exposing or persisting a result after a slow tool.
	if e = s.authorize(ctx, scope); e != nil {
		markError := s.failRequest(ctx, scope, claim.ID)
		return nil, errors.Join(e, markError)
	}
	if s.ports.ValidateOutput != nil {
		if e = s.ports.ValidateOutput(ctx, scope, output); e != nil {
			return nil, errors.Join(e, s.failRequest(ctx, scope, claim.ID))
		}
	}
	if e = s.store.complete(ctx, scope, claim, req.Question, output); e != nil {
		return nil, e
	}
	return output, nil
}
func (s *AssistantPlusService) History(ctx context.Context, scope AssistantPlusScope, id string, page, limit int) (json.RawMessage, error) {
	if e := s.authorize(ctx, scope); e != nil {
		return nil, e
	}
	if page < 1 || page > 100000 || limit < 1 || limit > 100 {
		return nil, plusChatError("invalid_page", 400)
	}
	value, e := s.store.history(ctx, scope, id, page, limit)
	if e != nil {
		return nil, e
	}
	if s.ports.ValidateOutput != nil {
		if e = s.ports.ValidateOutput(ctx, scope, value); e != nil {
			return nil, e
		}
	}
	return value, nil
}
func (s *AssistantPlusService) Delete(ctx context.Context, scope AssistantPlusScope, id string) error {
	if e := s.authorize(ctx, scope); e != nil {
		return e
	}
	return s.store.remove(ctx, scope, id)
}
func assistantPlusStatus(err error) (int, string) {
	var e *AssistantPlusError
	if errors.As(err, &e) {
		return e.Status, e.Code
	}
	return 500, "plus_error"
}

func (s *Service) WithPlus(ports AssistantPlusPorts, pool *pgxpool.Pool) *Service {
	s.plus = &AssistantPlusService{store: &assistantPlusStore{pool: pool}, ports: ports}
	return s
}

func canonicalPlusJSON(raw json.RawMessage) (json.RawMessage, error) {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if e := decoder.Decode(&value); e != nil {
		return nil, e
	}
	return json.Marshal(value)
}

func (s *AssistantPlusService) failRequest(ctx context.Context, scope AssistantPlusScope, id string) error {
	bounded, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	return s.store.fail(bounded, scope, id)
}
