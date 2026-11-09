package finance

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// CT-15 pacote 3 regression set; see docs/accounting/experiments/CT-15-P3-regressao.json.
func TestConversationIntentRegression(t *testing.T) {
	raw, err := os.ReadFile("../../../../docs/accounting/experiments/CT-15-P3-regressao.json")
	if err != nil {
		t.Fatal(err)
	}
	var data struct {
		Cases []struct{ ID, Question, Month, Kind, Route, Reason, Period string }
	}
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	for _, c := range data.Cases {
		intent := interpretConversation(c.Question, c.Month)
		if intent.Kind != c.Kind {
			t.Errorf("%s: kind %q, expected %q", c.ID, intent.Kind, c.Kind)
		}
		action, reason := routePlus(PlusInvocation{Question: c.Question, Month: c.Month})
		route := "clarify"
		if action != nil {
			route = action.ToolID
		}
		if route != c.Route || (c.Reason != "" && reason != c.Reason) {
			t.Errorf("%s: route %s/%s, expected %s/%s", c.ID, route, reason, c.Route, c.Reason)
			continue
		}
		if c.Period != "" {
			var input struct{ Period string }
			if json.Unmarshal(action.Input, &input) != nil || input.Period != c.Period {
				t.Errorf("%s: period %s, expected %s", c.ID, string(action.Input), c.Period)
			}
		}
	}
}

func TestConversationIntentNeverTreatsCommandsAsAnswers(t *testing.T) {
	for _, text := range []string{"qual meu saldo?", "faz uma nota pra mim", "deixa pra lá", "vamos continuar", "mostra o saldo e depois faz uma nota", "saldo de outubro"} {
		if !conversationCommand(text) {
			t.Errorf("command consumed as answer: %s", text)
		}
	}
	for _, text := range []string{"Cliente Nota QA", "Pague Menos Farmácias", "consultoria e manutenção", "não sei", "1500", "12/10/2026"} {
		if conversationCommand(text) {
			t.Errorf("answer treated as command: %s", text)
		}
	}
}

func TestMultipleRequestsReplyNeverExecutes(t *testing.T) {
	parts := multipleRequests("resumo e depois pague o imposto", "")
	if len(parts) != 2 {
		t.Fatal(parts)
	}
	reply := multipleRequestsReply(parts)
	if !strings.Contains(reply, "Qual você quer que eu faça primeiro") || !strings.Contains(reply, "não são executados") {
		t.Fatal(reply)
	}
}
