package omnichannel

import "errors"

// ============================================================================
// F8 — Maquina de estados do atendimento (Contrato 2 da spec OMNI-F8.md)
// ============================================================================
//
// `conversations.state` e a UNICA fonte de verdade do ciclo de vida (canonico §7.3). Esta
// maquina implementa a matriz completa 7 estados x 12 eventos = 84 pares, NENHUM implicito:
// cada celula existe na tabela `transitions`. Transicao ausente/rejeitada => ErrInvalidTransition,
// que o caller mapeia em 409. E o risco 4 do canonico.
//
// Nao ha timer/paused_until: o hard-block da IA e o estado `human_active`/`pending` via
// AIAllowed (canonico §6). Toda transicao roda sob `select ... for update` (concorrencia,
// risco 5) — o lock e do store (store_postgres_routing.go), aqui e logica pura e testavel.

// ErrInvalidTransition e o par (estado, evento) que a matriz rejeita ("—"). O caller
// (service_transition.go) mapeia para 409 invalid_transition com mensagem acionavel.
var ErrInvalidTransition = errors.New("omnichannel: invalid state transition")

// State e apelido de ConversationState (model.go) — a assinatura do Contrato 2 fala `State`.
type State = ConversationState

// Event e um dos 12 gatilhos do ciclo de vida (Contrato 2). A projecao inversa
// (status/assign do front -> evento) esta no Contrato 3 e mora em service_transition.go.
type Event string

const (
	EventMsgInbound       Event = "msg.inbound"
	EventMsgOutboundHuman Event = "msg.outbound.human"
	EventAITriageDone     Event = "ai.triage.done"
	EventAITriageFailed   Event = "ai.triage.failed"
	EventRouteMatched     Event = "route.matched"
	EventRouteUnmatched   Event = "route.unmatched"
	EventHumanAssign      Event = "human.assign"
	EventHumanUnassign    Event = "human.unassign"
	EventHumanPending     Event = "human.pending"
	EventQueueTransfer    Event = "queue.transfer"
	EventConvClose        Event = "conv.close"
	EventConvReopen       Event = "conv.reopen"
)

// TransitionContext carrega os fatos externos que resolvem as celulas condicionais da
// matriz: nota 1 (ha agente de IA ativo para o numero?) e nota 9 (a conversa tem fila?).
type TransitionContext struct {
	// HasActiveAgent = existe ai_agent ativo para o numero (F9). Sem agente, msg.inbound em
	// `new`/`closed` roteia direto (nao trava): ai_active com agente, routing sem (nota 1).
	HasActiveAgent bool
	// HasQueue = a conversa ja tem queue_id. Resolve human.unassign (nota 9): sem fila, a
	// conversa devolvida vai para `routing` e re-roteia — nunca vira orfa.
	HasQueue bool
}

// Outcome e o resultado de Apply. NoOp distingue "aceita e nao muda estado, mas ha efeito
// colateral" (self: atualiza last_message_at) de "aceita e nao muda NADA" (no-op: resultado
// tardio de run/duplicata — responde 200 e ignora). Contrato 2, legendas `self`/`no-op`.
type Outcome struct {
	To   State
	NoOp bool
}

// cellKind classifica cada celula da matriz. Enumerar torna a tabela auto-explicativa e o
// teste tabela-driven consegue afirmar par a par (prova das 84 transicoes).
type cellKind int

const (
	cellReject      cellKind = iota // "—": rejeita, ErrInvalidTransition (409)
	cellGoto                        // vai para um estado fixo
	cellSelf                        // aceita, estado nao muda, efeito colateral roda
	cellNoOp                        // aceita, nada muda (resultado tardio/duplicata)
	cellConditional                 // msg.inbound em new/closed: HasActiveAgent ? ai_active : routing (nota 1)
	cellUnassign                    // human.unassign: HasQueue ? queued : routing + re-roteia (nota 9)
)

type cell struct {
	kind cellKind
	to   State
}

var (
	reject                   = cell{kind: cellReject}
	self                     = cell{kind: cellSelf}
	noop                     = cell{kind: cellNoOp}
	conditionalTriageOrRoute = cell{kind: cellConditional}
	unassignReroute          = cell{kind: cellUnassign}
)

func goTo(s State) cell { return cell{kind: cellGoto, to: s} }

// transitions e a matriz do Contrato 2, transcrita celula a celula (nenhuma implicita).
// Ordem das colunas = ordem dos eventos na tabela da spec. As notas (superscritos) viram
// comentarios; a semantica de efeito colateral de cada celula mora em service_transition.go.
var transitions = map[State]map[Event]cell{
	// new
	StateNew: {
		EventMsgInbound:       conditionalTriageOrRoute, // 1: ai_active se ha agente, senao routing
		EventMsgOutboundHuman: goTo(StateHumanActive),   // 2: atendente tomou a conversa
		EventAITriageDone:     reject,
		EventAITriageFailed:   reject,
		EventRouteMatched:     reject,
		EventRouteUnmatched:   reject,
		EventHumanAssign:      goTo(StateHumanActive),
		EventHumanUnassign:    reject,
		EventHumanPending:     goTo(StatePending), // 10: rotulo manual, aceito como o new ja aceita assign/close
		EventQueueTransfer:    reject,             // 0: conversa nao triada nao tem o que transferir
		EventConvClose:        goTo(StateClosed),
		EventConvReopen:       reject,
	},
	// ai_active
	StateAIActive: {
		EventMsgInbound:       self, // 3: atualiza last_message_at, nao re-dispara triagem
		EventMsgOutboundHuman: goTo(StateHumanActive),
		EventAITriageDone:     goTo(StateRouting),
		EventAITriageFailed:   goTo(StateRouting), // 4: fail-open, extracted_fields vazio, outcome=ai_failed
		EventRouteMatched:     reject,
		EventRouteUnmatched:   reject,
		EventHumanAssign:      goTo(StateHumanActive),
		EventHumanUnassign:    reject,
		EventHumanPending:     goTo(StatePending), // 11: cancela run em voo, AIAllowed(pending)=false
		EventQueueTransfer:    goTo(StateQueued),  // 5: transferencia manual cancela run em voo
		EventConvClose:        goTo(StateClosed),
		EventConvReopen:       reject,
	},
	// routing
	StateRouting: {
		EventMsgInbound:       self,
		EventMsgOutboundHuman: goTo(StateHumanActive),
		EventAITriageDone:     noop, // 6: resultado tardio, ja saiu de ai_active
		EventAITriageFailed:   noop,
		EventRouteMatched:     goTo(StateQueued),
		EventRouteUnmatched:   goTo(StateQueued), // 7: fila default; sem default => queue_id NULL + unrouted
		EventHumanAssign:      goTo(StateHumanActive),
		EventHumanUnassign:    reject,
		EventHumanPending:     goTo(StatePending), // 11
		EventQueueTransfer:    goTo(StateQueued),  // 5
		EventConvClose:        goTo(StateClosed),
		EventConvReopen:       reject,
	},
	// queued
	StateQueued: {
		EventMsgInbound:       self,
		EventMsgOutboundHuman: goTo(StateHumanActive),
		EventAITriageDone:     noop,
		EventAITriageFailed:   noop,
		EventRouteMatched:     reject,
		EventRouteUnmatched:   reject,
		EventHumanAssign:      goTo(StateHumanActive), // 8: guarda queue_member ativo ou settings.manage
		EventHumanUnassign:    reject,
		EventHumanPending:     goTo(StatePending), // 10
		EventQueueTransfer:    self,               // 5: ja em fila, troca queue_id, estado nao muda
		EventConvClose:        goTo(StateClosed),
		EventConvReopen:       reject,
	},
	// human_active
	StateHumanActive: {
		EventMsgInbound:       self,
		EventMsgOutboundHuman: self, // atendente responde de novo, segue dono
		EventAITriageDone:     noop,
		EventAITriageFailed:   noop,
		EventRouteMatched:     reject,
		EventRouteUnmatched:   reject,
		EventHumanAssign:      self,               // 8: re-atribuir ao mesmo/outro, guarda vale
		EventHumanUnassign:    unassignReroute,    // 9: devolve para a fila; sem fila => routing (re-roteia)
		EventHumanPending:     goTo(StatePending), // 10
		EventQueueTransfer:    goTo(StateQueued),  // 5
		EventConvClose:        goTo(StateClosed),
		EventConvReopen:       reject,
	},
	// pending (7o estado — decisao do dono 2026-07-17, notas 10-13)
	StatePending: {
		EventMsgInbound:       self,                   // 3: cliente responde NAO limpa o rotulo do operador
		EventMsgOutboundHuman: goTo(StateHumanActive), // 2: responder tira do pendente
		EventAITriageDone:     noop,
		EventAITriageFailed:   noop,
		EventRouteMatched:     reject,
		EventRouteUnmatched:   reject,
		EventHumanAssign:      goTo(StateHumanActive), // 8
		EventHumanUnassign:    unassignReroute,        // 13: devolve p/ fila derruba o rotulo (nota 9 inteira)
		EventHumanPending:     noop,                   // 12: marcar duas vezes nao muda nada
		EventQueueTransfer:    goTo(StateQueued),      // 5
		EventConvClose:        goTo(StateClosed),
		EventConvReopen:       reject,
	},
	// closed
	StateClosed: {
		EventMsgInbound:       conditionalTriageOrRoute, // 1: reabre, zera assigned_user_id, re-roteia do zero
		EventMsgOutboundHuman: goTo(StateHumanActive),   // 2
		EventAITriageDone:     noop,
		EventAITriageFailed:   noop,
		EventRouteMatched:     reject,
		EventRouteUnmatched:   reject,
		EventHumanAssign:      reject,
		EventHumanUnassign:    reject,
		EventHumanPending:     reject, // 12: aceitar reabriria em silencio; reabrir antes (conv.reopen)
		EventQueueTransfer:    reject,
		EventConvClose:        noop, // fechar de novo nao muda nada
		EventConvReopen:       goTo(StateRouting),
	},
}

// Apply resolve a matriz para (from, ev). Par ausente ou celula `reject` =>
// ErrInvalidTransition (409). Logica pura: nao toca banco, nao tem efeito colateral — os
// efeitos (assigned_user_id, queue_id, last_message_at, decisao) sao do service.
func Apply(from State, ev Event, tc TransitionContext) (Outcome, error) {
	row, ok := transitions[from]
	if !ok {
		return Outcome{}, ErrInvalidTransition
	}
	c, ok := row[ev]
	if !ok {
		return Outcome{}, ErrInvalidTransition
	}
	switch c.kind {
	case cellGoto:
		return Outcome{To: c.to}, nil
	case cellSelf:
		return Outcome{To: from}, nil
	case cellNoOp:
		return Outcome{To: from, NoOp: true}, nil
	case cellConditional:
		if tc.HasActiveAgent {
			return Outcome{To: StateAIActive}, nil
		}
		return Outcome{To: StateRouting}, nil
	case cellUnassign:
		// Nota 9: devolver para a fila. Com fila => queued; sem fila (atribuida direto do
		// `new`) => routing, para re-rotear e nunca virar orfa.
		if tc.HasQueue {
			return Outcome{To: StateQueued}, nil
		}
		return Outcome{To: StateRouting}, nil
	default: // cellReject e qualquer coisa nao mapeada
		return Outcome{}, ErrInvalidTransition
	}
}

// AIAllowed e o hard-block da IA (canonico §6): a IA so pode falar em `new` e `ai_active`.
// Em human_active/pending/queued/routing/closed a IA cala — estado e mais honesto que timer.
func AIAllowed(s State) bool {
	switch s {
	case StateNew, StateAIActive:
		return true
	default:
		return false
	}
}
