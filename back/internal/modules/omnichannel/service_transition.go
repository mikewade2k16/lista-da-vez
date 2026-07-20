package omnichannel

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

// ============================================================================
// F8 — Service de runtime: maquina de estados + motor de roteamento (Contratos 2/3/4)
// ============================================================================
//
// `conversations.state` e a UNICA verdade do ciclo de vida. NUNCA existe `update ... set
// status` (risco 4): quem muda o ciclo chama Transition. A F7 chama Transition; a F4/F9
// chama RouteConversation depois da triagem. Toda escrita roda sob lock (ApplyTransition).

// Transition aplica UM evento a conversa (Contrato 2). O estado destino e persistido, com os
// efeitos colaterais de cada evento (assigned_user_id, queue_id, last_message_at) conforme as
// notas da matriz. Transicao invalida => ErrInvalidTransition (409). Conversa fora da conta
// => ErrNotFound (404).
func (s *Service) Transition(ctx context.Context, p auth.Principal, convID string, ev Event, payload TransitionPayload) (ConversationView, error) {
	row, err := s.applyTransition(ctx, strings.TrimSpace(p.AccountID), convID, ev, payload)
	if err != nil {
		return ConversationView{}, err
	}
	return conversationView(row)
}

// SystemTransition aplica um evento como o ATOR SISTEMA (webhook inbound / auto-triagem da IA),
// nao um usuario: NAO ha gate de permissao — o webhook ja e autenticado no transporte e o
// accountID vem do slug resolvido no server (nunca do body). Devolve o STATE resultante para a
// pipeline de auto-triagem (service_inbound) decidir o proximo passo. Transicao invalida =>
// ErrInvalidTransition; conversa fora da conta => ErrNotFound. Roda sob o mesmo lock da Transition.
func (s *Service) SystemTransition(ctx context.Context, accountID, convID string, ev Event, payload TransitionPayload) (State, error) {
	row, err := s.applyTransition(ctx, strings.TrimSpace(accountID), convID, ev, payload)
	if err != nil {
		return "", err
	}
	return State(row.State), nil
}

// applyTransition e o nucleo compartilhado por Transition (Principal) e SystemTransition
// (accountID cru do webhook): valida o escopo, roda a maquina sob lock e devolve a linha crua.
func (s *Service) applyTransition(ctx context.Context, accountID, convID string, ev Event, payload TransitionPayload) (conversationRow, error) {
	if accountID == "" {
		return conversationRow{}, ErrForbidden
	}
	return s.store.ApplyTransition(ctx, accountID, convID, func(snap convSnapshot) (stateUpdate, *decisionRecord, error) {
		tc, err := s.transitionContextFor(ctx, accountID, ev, snap)
		if err != nil {
			return stateUpdate{}, nil, err
		}
		out, err := Apply(snap.State, ev, tc)
		if err != nil {
			return stateUpdate{}, nil, err
		}
		if out.NoOp {
			return stateUpdate{NoChange: true}, nil, nil
		}
		upd := stateUpdate{
			State:          out.To,
			QueueID:        snap.QueueID,
			DepartmentID:   snap.DepartmentID,
			AssignedUserID: snap.AssignedUserID,
		}
		dec, err := s.applyEventEffects(ctx, accountID, ev, payload, snap, &upd)
		if err != nil {
			return stateUpdate{}, nil, err
		}
		return upd, dec, nil
	})
}

// RouteConversation e o MOTOR (Contrato 4): decide deterministicamente para qual fila a
// conversa (em `routing`) vai, aplica route.matched/route.unmatched e grava a decisao — tudo
// numa transacao. So e valido em `routing` (Apply de outro estado com route.* => 409). "IA
// sugere; o motor decide": aqui nao ha LLM, so a regra.
func (s *Service) RouteConversation(ctx context.Context, p auth.Principal, convID string) (ConversationView, error) {
	row, err := s.routeConversation(ctx, strings.TrimSpace(p.AccountID), convID)
	if err != nil {
		return ConversationView{}, err
	}
	return conversationView(row)
}

// SystemRoute roda o MOTOR como ator sistema (auto-triagem, pos ai.triage.done/failed), sem gate
// de permissao. Devolve o state resultante (queued apos route.matched/unmatched). Chamar fora de
// `routing` => ErrInvalidTransition (a pipeline so chama quando o state e routing).
func (s *Service) SystemRoute(ctx context.Context, accountID, convID string) (State, error) {
	row, err := s.routeConversation(ctx, strings.TrimSpace(accountID), convID)
	if err != nil {
		return "", err
	}
	return State(row.State), nil
}

// routeConversation e o nucleo do motor, compartilhado por RouteConversation (Principal) e
// SystemRoute (accountID cru). O motor le routing_rules e DECIDE a fila — a IA nao escreve queue_id.
func (s *Service) routeConversation(ctx context.Context, accountID, convID string) (conversationRow, error) {
	if accountID == "" {
		return conversationRow{}, ErrForbidden
	}
	engine := newRoutingEngine(s.store)
	return s.store.ApplyTransition(ctx, accountID, convID, func(snap convSnapshot) (stateUpdate, *decisionRecord, error) {
		text, err := s.store.LatestMessageText(ctx, accountID, convID)
		if err != nil {
			return stateUpdate{}, nil, err
		}
		decision, err := engine.Decide(ctx, accountID, buildRoutingContext(snap, text))
		if err != nil {
			return stateUpdate{}, nil, err
		}
		ev := EventRouteUnmatched
		if decision.Outcome == outcomeMatched {
			ev = EventRouteMatched
		}
		tc, err := s.transitionContextFor(ctx, accountID, ev, snap)
		if err != nil {
			return stateUpdate{}, nil, err
		}
		out, err := Apply(snap.State, ev, tc)
		if err != nil {
			return stateUpdate{}, nil, err // conversa nao esta em `routing`
		}
		upd := stateUpdate{
			State:          out.To, // queued
			QueueID:        decision.QueueID,
			DepartmentID:   decision.DepartmentID,
			AssignedUserID: nil,
		}
		rec := &decisionRecord{
			RuleID:             decision.RuleID,
			Outcome:            decision.Outcome,
			Reason:             decision.Reason,
			Input:              decision.Input,
			TargetDepartmentID: decision.DepartmentID,
			TargetQueueID:      decision.QueueID,
		}
		return upd, rec, nil
	})
}

// transitionContextFor resolve as celulas condicionais da matriz. HasQueue sai do snapshot;
// HasActiveAgent (nota 1) e resolvido do banco (F9) — mas SO para msg.inbound, a UNICA celula
// condicional que o consulta. Evita uma query por transicao no caminho quente (assign/close/
// route nao tocam ai_agents). Sem agente ativo, msg.inbound em new/closed roteia direto; com
// agente habilitado + version ativa, entra em ai_active e a triagem e convidada.
func (s *Service) transitionContextFor(ctx context.Context, accountID string, ev Event, snap convSnapshot) (TransitionContext, error) {
	tc := TransitionContext{HasQueue: snap.QueueID != nil}
	if ev == EventMsgInbound {
		_, hasAgent, err := s.store.ActiveAgent(ctx, accountID)
		if err != nil {
			return TransitionContext{}, err
		}
		tc.HasActiveAgent = hasAgent
	}
	return tc, nil
}

// applyEventEffects aplica os efeitos colaterais de cada evento sobre a tupla destino
// (Contrato 2, notas). O service e o dono destas regras; o store so grava a tupla pronta.
// Devolve a decisao de auditoria quando o evento a produz (queue.transfer => manual_transfer).
func (s *Service) applyEventEffects(ctx context.Context, accountID string, ev Event, payload TransitionPayload, snap convSnapshot, upd *stateUpdate) (*decisionRecord, error) {
	switch ev {
	case EventMsgInbound:
		upd.BumpLastMessage = true
		if snap.State == StateClosed { // nota 1: reabrir zera atribuicao e re-roteia do zero
			upd.QueueID, upd.DepartmentID, upd.AssignedUserID = nil, nil, nil
		}
	case EventMsgOutboundHuman:
		upd.BumpLastMessage = true
		if actor := strings.TrimSpace(payload.ActorUserID); actor != "" {
			upd.AssignedUserID = &actor // nota 2: atendente tomou a conversa
		}
	case EventHumanAssign:
		target := strings.TrimSpace(payload.TargetUserID)
		if target == "" {
			return nil, ErrValidation
		}
		if err := s.assertAssignable(ctx, accountID, snap.QueueID, target); err != nil {
			return nil, err
		}
		upd.AssignedUserID = &target
	case EventHumanUnassign:
		upd.AssignedUserID = nil // nota 9
		if snap.QueueID == nil { // sem fila => vai para routing; limpa dept para re-rotear
			upd.QueueID, upd.DepartmentID = nil, nil
		}
	case EventHumanPending:
		// nota 10: preserva assigned_user_id E queue_id (ortogonal ao roteamento) — nao mexe.
	case EventQueueTransfer:
		return s.applyQueueTransfer(ctx, accountID, payload, upd)
	case EventConvReopen:
		upd.QueueID, upd.DepartmentID, upd.AssignedUserID = nil, nil, nil // nota 1
	}
	return nil, nil
}

// applyQueueTransfer valida a fila destino (inativa/de outra conta => 404), grava queue_id/
// department_id, limpa a atribuicao (nota 5) e produz a decisao manual_transfer.
func (s *Service) applyQueueTransfer(ctx context.Context, accountID string, payload TransitionPayload, upd *stateUpdate) (*decisionRecord, error) {
	queueID := strings.TrimSpace(payload.TargetQueueID)
	if queueID == "" {
		return nil, ErrValidation
	}
	deptID, err := s.store.queueDepartment(ctx, accountID, queueID)
	if err != nil {
		return nil, translate(err)
	}
	upd.QueueID = &queueID
	upd.DepartmentID = &deptID
	upd.AssignedUserID = nil
	return &decisionRecord{
		Outcome:            outcomeManualTransfer,
		Reason:             "transferencia manual de fila",
		Input:              map[string]any{},
		TargetQueueID:      &queueID,
		TargetDepartmentID: &deptID,
	}, nil
}

// buildRoutingContext monta a entrada do motor a partir do snapshot + o texto da ultima
// mensagem. extracted_fields malformado nao derruba o roteamento (best-effort => vazio).
func buildRoutingContext(snap convSnapshot, messageText string) RoutingContext {
	fields := map[string]any{}
	if len(snap.ExtractedFields) > 0 && string(snap.ExtractedFields) != "null" {
		_ = json.Unmarshal(snap.ExtractedFields, &fields) //nolint:errcheck // best-effort
	}
	phone := ""
	if snap.ContactPhone != nil {
		phone = *snap.ContactPhone
	}
	return RoutingContext{
		ExtractedFields: fields,
		MessageText:     messageText,
		ContactPhone:    phone,
		InstanceName:    snap.InstanceScopeKey,
	}
}

// ============================================================================
// Transferencia de fila (rota PATCH /conversations/{id}/queue) e auditoria
// ============================================================================

// TransferQueue e o handler de PATCH /conversations/{id}/queue: exige
// `omnichannel.conversations.assign` (feature => 403) e dispara o evento queue.transfer.
func (s *Service) TransferQueue(ctx context.Context, p auth.Principal, convID, queueID string) (ConversationView, error) {
	if err := s.requirePermission(ctx, p.AccountID, p, "omnichannel.conversations.assign"); err != nil {
		return ConversationView{}, err
	}
	return s.Transition(ctx, p, convID, EventQueueTransfer, TransitionPayload{TargetQueueID: queueID})
}

// ListRoutingDecisions devolve a auditoria de roteamento de uma conversa. Exige
// `omnichannel.conversations.view` (feature) + visibilidade (gate de dado). Conversa fora
// do gate => 404.
func (s *Service) ListRoutingDecisions(ctx context.Context, p auth.Principal, convID string) ([]RoutingDecisionView, error) {
	if err := s.requirePermission(ctx, p.AccountID, p, "omnichannel.conversations.view"); err != nil {
		return nil, err
	}
	scope, err := s.resolveVisibility(ctx, p.AccountID, p)
	if err != nil {
		return nil, err
	}
	if _, err := s.store.GetVisibleConversation(ctx, p.AccountID, scope, convID); err != nil {
		return nil, translate(err)
	}
	return s.store.ListRoutingDecisions(ctx, p.AccountID, convID)
}

// ============================================================================
// Autorizacao e visibilidade (permissao gateia FEATURE; fila gateia DADO)
// ============================================================================

// requireSettingsManage exige a permissao de config (403 se faltar).
func (s *Service) requireSettingsManage(ctx context.Context, accountID string, p auth.Principal) error {
	return s.requirePermission(ctx, accountID, p, "omnichannel.settings.manage")
}

// requirePermission resolve a permissao efetiva NA CONTA e devolve ErrForbidden (403) se
// faltar. platform_admin passa (has()=false no front, mas e admin de fato). Cai tambem no
// principal.Permissions global quando resolvido.
func (s *Service) requirePermission(ctx context.Context, accountID string, p auth.Principal, key string) error {
	ok, err := s.hasPermission(ctx, accountID, p, key)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	return nil
}

func (s *Service) hasPermission(ctx context.Context, accountID string, p auth.Principal, key string) (bool, error) {
	if strings.TrimSpace(accountID) == "" {
		return false, nil
	}
	if p.Role == auth.RolePlatformAdmin {
		return true, nil
	}
	ok, err := s.store.hasEffectivePermission(ctx, accountID, p.UserID, key)
	if err != nil {
		return false, err
	}
	if ok {
		return true, nil
	}
	if p.PermissionsResolved && containsPermission(p.Permissions, key) {
		return true, nil
	}
	return false, nil
}

// resolveVisibility monta o escopo de dado (Contrato 5): amplo = platform_admin OU
// settings.manage (ve inclusive conversa unrouted); caso contrario, so as filas onde e
// membro ativo + as atribuidas a ele.
func (s *Service) resolveVisibility(ctx context.Context, accountID string, p auth.Principal) (VisibilityScope, error) {
	broad, err := s.hasPermission(ctx, accountID, p, "omnichannel.settings.manage")
	if err != nil {
		return VisibilityScope{}, err
	}
	return VisibilityScope{UserID: p.UserID, IsBroad: broad}, nil
}

// assertAssignable e a guarda da nota 8: o usuario destino e queue_member ATIVO da fila da
// conversa OU tem settings.manage. Fora disso => 404 do usuario destino (nunca 403).
func (s *Service) assertAssignable(ctx context.Context, accountID string, queueID *string, targetUserID string) error {
	if queueID != nil {
		ok, err := s.store.IsActiveQueueMember(ctx, accountID, *queueID, targetUserID)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
	}
	ok, err := s.store.HasSettingsManage(ctx, accountID, targetUserID)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	return ErrNotFound
}

// containsPermission responde se a key esta na lista de permissoes global do Principal.
func containsPermission(values []string, key string) bool {
	for _, v := range values {
		if strings.TrimSpace(v) == key {
			return true
		}
	}
	return false
}
