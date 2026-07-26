package omnichannel

import (
	"context"
	"testing"
)

// TestTriageEventFor prova o mapeamento desfecho->evento do auto-disparo: SO `triaged` fecha a
// triagem com sucesso (ai.triage.done); TODO desfecho degradado falha OPEN (ai.triage.failed),
// para a conversa nunca ficar presa em ai_active esperando a IA. Ambos os eventos, na maquina,
// levam ai_active -> routing (o motor deterministico decide a fila depois).
func TestTriageEventFor(t *testing.T) {
	cases := []struct {
		outcome DispatchOutcome
		want    Event
	}{
		{dispatchTriaged, EventAITriageDone},
		{dispatchBlocked, EventAITriageFailed},
		{dispatchNoAgent, EventAITriageFailed},
		{dispatchLimitExceeded, EventAITriageFailed},
		{dispatchProviderError, EventAITriageFailed},
		{dispatchSchemaInvalid, EventAITriageFailed},
		{DispatchOutcome(""), EventAITriageFailed}, // desfecho vazio (erro de infra) tambem fail-open
	}
	for _, tc := range cases {
		if got := triageEventFor(tc.outcome); got != tc.want {
			t.Errorf("triageEventFor(%q) = %q, quer %q", tc.outcome, got, tc.want)
		}
	}
}

// TestAutoTriagePathWithAgent prova o caminho QUENTE que o wire dirige quando ha agente ativo:
// new -(msg.inbound, HasActiveAgent)-> ai_active -(ai.triage.done)-> routing -(route.matched)-> queued.
// Se qualquer degrau quebrasse, a conversa nao chegaria em queued e o auto-disparo estaria furado.
func TestAutoTriagePathWithAgent(t *testing.T) {
	assertGoto(t, StateNew, EventMsgInbound, TransitionContext{HasActiveAgent: true}, StateAIActive)
	assertGoto(t, StateAIActive, EventAITriageDone, TransitionContext{}, StateRouting)
	assertGoto(t, StateAIActive, EventAITriageFailed, TransitionContext{}, StateRouting) // fail-open
	assertGoto(t, StateRouting, EventRouteMatched, TransitionContext{}, StateQueued)
	assertGoto(t, StateRouting, EventRouteUnmatched, TransitionContext{}, StateQueued)
}

// TestAutoTriagePathWithoutAgent prova o caminho SEM agente: new -(msg.inbound, !HasActiveAgent)->
// routing -> queued, roteando DIRETO sem passar por ai_active (a IA nao dispara, nao grava ai_runs).
func TestAutoTriagePathWithoutAgent(t *testing.T) {
	assertGoto(t, StateNew, EventMsgInbound, TransitionContext{HasActiveAgent: false}, StateRouting)
	assertGoto(t, StateRouting, EventRouteMatched, TransitionContext{}, StateQueued)
}

func TestInboundTransitionCanUsePreResolvedAgentInsidePersistenceTransaction(t *testing.T) {
	service := &Service{}
	for _, tc := range []struct {
		hasAgent bool
		want     State
	}{
		{hasAgent: true, want: StateAIActive},
		{hasAgent: false, want: StateRouting},
	} {
		update, decision, err := service.decideTransitionWithContext(
			context.Background(),
			"account-1",
			EventMsgInbound,
			TransitionPayload{},
			convSnapshot{State: StateNew},
			TransitionContext{HasActiveAgent: tc.hasAgent},
		)
		if err != nil {
			t.Fatalf("hasAgent=%v: %v", tc.hasAgent, err)
		}
		if decision != nil || update.State != tc.want ||
			!update.BumpLastMessage || !update.AdvanceAIGeneration {
			t.Fatalf(
				"hasAgent=%v: update=%+v decision=%+v, want state=%s with last-message bump",
				tc.hasAgent, update, decision, tc.want,
			)
		}
	}
}

// TestAutoTriageHumanStatesDoNotInviteAI prova que uma mensagem inbound num estado humano NAO
// convida a IA: msg.inbound e `self` (estado nao muda) e AIAllowed=false. O wire, vendo o estado
// resultante != ai_active/routing, nao dispara triagem nem re-roteia — o atendente segue no controle.
func TestAutoTriageHumanStatesDoNotInviteAI(t *testing.T) {
	for _, st := range []State{StateHumanActive, StatePending, StateQueued} {
		out, err := Apply(st, EventMsgInbound, TransitionContext{HasActiveAgent: true})
		if err != nil {
			t.Fatalf("Apply(%q, msg.inbound) erro inesperado: %v", st, err)
		}
		if out.To != st {
			t.Errorf("msg.inbound em %q deveria ser self (state=%q), veio %q", st, st, out.To)
		}
		if AIAllowed(st) {
			t.Errorf("AIAllowed(%q) deveria ser false (estado humano nao convida IA)", st)
		}
	}
}

// assertGoto afirma que Apply(from, ev, tc) leva ao estado `want` sem erro e sem no-op.
func assertGoto(t *testing.T, from State, ev Event, tc TransitionContext, want State) {
	t.Helper()
	out, err := Apply(from, ev, tc)
	if err != nil {
		t.Fatalf("Apply(%q, %q) erro inesperado: %v", from, ev, err)
	}
	if out.NoOp {
		t.Fatalf("Apply(%q, %q) veio no-op, esperava goto %q", from, ev, want)
	}
	if out.To != want {
		t.Fatalf("Apply(%q, %q) = %q, esperava %q", from, ev, out.To, want)
	}
}
