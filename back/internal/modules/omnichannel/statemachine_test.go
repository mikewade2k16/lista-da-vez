package omnichannel

import (
	"errors"
	"testing"
)

// allStates e allEvents fixam os eixos da matriz (7 x 12). O teste falha se a maquina
// (transitions) e este eixo divergirem em tamanho — a prova de que sao 84 pares exatos.
var allStates = []State{
	StateNew, StateAIActive, StateRouting, StateQueued, StateHumanActive, StatePending, StateClosed,
}

var allEvents = []Event{
	EventMsgInbound, EventMsgOutboundHuman, EventAITriageDone, EventAITriageFailed,
	EventRouteMatched, EventRouteUnmatched, EventHumanAssign, EventHumanUnassign,
	EventHumanPending, EventQueueTransfer, EventConvClose, EventConvReopen,
}

// exp e o resultado ESPERADO de uma celula, transcrito A MAO da tabela do Contrato 2 (NAO
// derivado de `transitions`) — e o cross-check independente que prova a matriz.
type exp struct {
	err      bool  // "—": rejeita (ErrInvalidTransition / 409)
	to       State // destino; para self/noop = o proprio estado de origem
	noop     bool  // no-op (aceita, nada muda)
	cond     bool  // condicional: msg.inbound em new/closed (ai_active se ha agente, senao routing)
	unassign bool  // human.unassign: HasQueue ? queued : routing (nota 9)
}

func rej() exp             { return exp{err: true} }
func to(s State) exp       { return exp{to: s} }
func selfCell(s State) exp { return exp{to: s} }
func noopCell(s State) exp { return exp{to: s, noop: true} }
func cond() exp            { return exp{cond: true} }
func unassignCell() exp    { return exp{unassign: true} }

// expected reproduz a matriz do Contrato 2 celula a celula, independente de transitions.
// Cada linha tem exatamente 12 colunas; cada estado tem uma linha => 84 pares.
var expected = map[State]map[Event]exp{
	StateNew: {
		EventMsgInbound: cond(), EventMsgOutboundHuman: to(StateHumanActive),
		EventAITriageDone: rej(), EventAITriageFailed: rej(),
		EventRouteMatched: rej(), EventRouteUnmatched: rej(),
		EventHumanAssign: to(StateHumanActive), EventHumanUnassign: rej(),
		EventHumanPending: to(StatePending), EventQueueTransfer: rej(),
		EventConvClose: to(StateClosed), EventConvReopen: rej(),
	},
	StateAIActive: {
		EventMsgInbound: selfCell(StateAIActive), EventMsgOutboundHuman: to(StateHumanActive),
		EventAITriageDone: to(StateRouting), EventAITriageFailed: to(StateRouting),
		EventRouteMatched: rej(), EventRouteUnmatched: rej(),
		EventHumanAssign: to(StateHumanActive), EventHumanUnassign: rej(),
		EventHumanPending: to(StatePending), EventQueueTransfer: to(StateQueued),
		EventConvClose: to(StateClosed), EventConvReopen: rej(),
	},
	StateRouting: {
		EventMsgInbound: selfCell(StateRouting), EventMsgOutboundHuman: to(StateHumanActive),
		EventAITriageDone: noopCell(StateRouting), EventAITriageFailed: noopCell(StateRouting),
		EventRouteMatched: to(StateQueued), EventRouteUnmatched: to(StateQueued),
		EventHumanAssign: to(StateHumanActive), EventHumanUnassign: rej(),
		EventHumanPending: to(StatePending), EventQueueTransfer: to(StateQueued),
		EventConvClose: to(StateClosed), EventConvReopen: rej(),
	},
	StateQueued: {
		EventMsgInbound: selfCell(StateQueued), EventMsgOutboundHuman: to(StateHumanActive),
		EventAITriageDone: noopCell(StateQueued), EventAITriageFailed: noopCell(StateQueued),
		EventRouteMatched: rej(), EventRouteUnmatched: rej(),
		EventHumanAssign: to(StateHumanActive), EventHumanUnassign: rej(),
		EventHumanPending: to(StatePending), EventQueueTransfer: selfCell(StateQueued),
		EventConvClose: to(StateClosed), EventConvReopen: rej(),
	},
	StateHumanActive: {
		EventMsgInbound: selfCell(StateHumanActive), EventMsgOutboundHuman: selfCell(StateHumanActive),
		EventAITriageDone: noopCell(StateHumanActive), EventAITriageFailed: noopCell(StateHumanActive),
		EventRouteMatched: rej(), EventRouteUnmatched: rej(),
		EventHumanAssign: selfCell(StateHumanActive), EventHumanUnassign: unassignCell(),
		EventHumanPending: to(StatePending), EventQueueTransfer: to(StateQueued),
		EventConvClose: to(StateClosed), EventConvReopen: rej(),
	},
	StatePending: {
		EventMsgInbound: selfCell(StatePending), EventMsgOutboundHuman: to(StateHumanActive),
		EventAITriageDone: noopCell(StatePending), EventAITriageFailed: noopCell(StatePending),
		EventRouteMatched: rej(), EventRouteUnmatched: rej(),
		EventHumanAssign: to(StateHumanActive), EventHumanUnassign: unassignCell(),
		EventHumanPending: noopCell(StatePending), EventQueueTransfer: to(StateQueued),
		EventConvClose: to(StateClosed), EventConvReopen: rej(),
	},
	StateClosed: {
		EventMsgInbound: cond(), EventMsgOutboundHuman: to(StateHumanActive),
		EventAITriageDone: noopCell(StateClosed), EventAITriageFailed: noopCell(StateClosed),
		EventRouteMatched: rej(), EventRouteUnmatched: rej(),
		EventHumanAssign: rej(), EventHumanUnassign: rej(),
		EventHumanPending: rej(), EventQueueTransfer: rej(),
		EventConvClose: noopCell(StateClosed), EventConvReopen: to(StateRouting),
	},
}

// TestApplyMatrixCoverage prova as 84 transicoes: itera os 7x12 pares, confere que cada um
// existe em `transitions` (nenhum implicito) e que Apply devolve exatamente o que a tabela
// `expected` (transcrita a mao do Contrato 2) manda. Celulas condicionais sao provadas nos
// dois ramos de HasActiveAgent.
func TestApplyMatrixCoverage(t *testing.T) {
	pairs := 0
	for _, from := range allStates {
		for _, ev := range allEvents {
			pairs++
			want, ok := expected[from][ev]
			if !ok {
				t.Fatalf("celula esperada ausente no teste: %s x %s", from, ev)
			}
			if _, ok := transitions[from][ev]; !ok {
				t.Errorf("transicao IMPLICITA (ausente na matriz): %s x %s", from, ev)
				continue
			}

			if want.cond {
				assertConditional(t, from, ev)
				continue
			}
			if want.unassign {
				assertUnassign(t, from, ev)
				continue
			}

			out, err := Apply(from, ev, TransitionContext{})
			switch {
			case want.err:
				if !errors.Is(err, ErrInvalidTransition) {
					t.Errorf("%s x %s: esperava ErrInvalidTransition, veio (%+v, %v)", from, ev, out, err)
				}
			case err != nil:
				t.Errorf("%s x %s: erro inesperado %v", from, ev, err)
			case out.To != want.to || out.NoOp != want.noop:
				t.Errorf("%s x %s: veio {To:%s NoOp:%v}, esperava {To:%s NoOp:%v}",
					from, ev, out.To, out.NoOp, want.to, want.noop)
			}
		}
	}
	if pairs != 84 {
		t.Fatalf("cobertura da matriz = %d pares, esperava 84", pairs)
	}
}

// assertConditional prova os dois ramos da nota 1: com agente ativo -> ai_active; sem
// agente -> routing. So msg.inbound em new/closed e condicional.
func assertConditional(t *testing.T, from State, ev Event) {
	t.Helper()
	withAgent, err := Apply(from, ev, TransitionContext{HasActiveAgent: true})
	if err != nil || withAgent.To != StateAIActive || withAgent.NoOp {
		t.Errorf("%s x %s (com agente): veio (%+v, %v), esperava ai_active", from, ev, withAgent, err)
	}
	noAgent, err := Apply(from, ev, TransitionContext{HasActiveAgent: false})
	if err != nil || noAgent.To != StateRouting || noAgent.NoOp {
		t.Errorf("%s x %s (sem agente): veio (%+v, %v), esperava routing", from, ev, noAgent, err)
	}
}

// assertUnassign prova os dois ramos da nota 9: com fila -> queued; sem fila -> routing
// (re-roteia). So human.unassign em human_active/pending e condicional.
func assertUnassign(t *testing.T, from State, ev Event) {
	t.Helper()
	withQueue, err := Apply(from, ev, TransitionContext{HasQueue: true})
	if err != nil || withQueue.To != StateQueued || withQueue.NoOp {
		t.Errorf("%s x %s (com fila): veio (%+v, %v), esperava queued", from, ev, withQueue, err)
	}
	noQueue, err := Apply(from, ev, TransitionContext{HasQueue: false})
	if err != nil || noQueue.To != StateRouting || noQueue.NoOp {
		t.Errorf("%s x %s (sem fila): veio (%+v, %v), esperava routing", from, ev, noQueue, err)
	}
}

// TestTransitionsHasNoExtraCells garante que a maquina nao tem par ALEM dos 84 — coluna/
// linha a mais tambem e divergencia da spec.
func TestTransitionsHasNoExtraCells(t *testing.T) {
	total := 0
	for from, row := range transitions {
		if _, ok := expected[from]; !ok {
			t.Errorf("estado fora da matriz do Contrato 2: %s", from)
		}
		for ev := range row {
			total++
			if _, ok := expected[from][ev]; !ok {
				t.Errorf("evento fora da matriz do Contrato 2: %s x %s", from, ev)
			}
		}
	}
	if total != 84 {
		t.Fatalf("transitions tem %d celulas, esperava 84", total)
	}
}

func TestAIAllowed(t *testing.T) {
	allowed := map[State]bool{StateNew: true, StateAIActive: true}
	for _, s := range allStates {
		if got := AIAllowed(s); got != allowed[s] {
			t.Errorf("AIAllowed(%s) = %v, esperava %v", s, got, allowed[s])
		}
	}
}

func TestApplyUnknownEvent(t *testing.T) {
	if _, err := Apply(StateNew, Event("bogus.event"), TransitionContext{}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("evento desconhecido deveria dar ErrInvalidTransition, veio %v", err)
	}
}
