package calendar

import (
	"strings"
	"testing"
	"time"
)

func TestCalendarReadUsesTodayInsteadOfNegatedHistoricalDate(t *testing.T) {
	now := time.Date(2026, time.November, 13, 14, 0, 0, 0, time.UTC)
	for _, question := range []string{
		"oq temos para hoje?",
		"hoje ne é 22 de outubro e vc me listou apenas 1",
		"hoje não é 22 de outubro, liste os eventos corretos",
	} {
		query, ok := resolveChatCalendarRead(question, nil, now)
		if !ok || query.from != "2026-11-13" || query.to != query.from {
			t.Errorf("%q: período incorreto: %+v, %t", question, query, ok)
		}
		if got := inferChatMonth(question, "2026-10", now); got != "2026-11" {
			t.Errorf("%q: mês incorreto: %s", question, got)
		}
	}
}

func TestCalendarReadContinuationIgnoresModelDateAndCount(t *testing.T) {
	now := time.Date(2026, time.November, 13, 14, 0, 0, 0, time.UTC)
	prior := []ChatMessage{
		{Role: chatRoleUser, Content: "oq temos para hoje?"},
		{Role: chatRoleAssistant, Content: "Hoje, 22/10, há 13 eventos."},
	}
	query, ok := resolveChatCalendarRead("13 eventos? liste eles aqui", prior, now)
	if !ok || query.from != "2026-11-13" {
		t.Fatalf("continuação confiou no modelo: %+v, %t", query, ok)
	}
}

func TestCalendarRelativeDayUsesProductTimezone(t *testing.T) {
	now := time.Date(2026, time.November, 1, 1, 0, 0, 0, time.UTC)
	for question, want := range map[string]string{
		"eventos hoje":                "2026-10-31",
		"eventos amanhã":              "2026-11-01",
		"eventos ontem":               "2026-10-30",
		"mostre 22/10/2026, não hoje": "2026-10-22",
	} {
		query, ok := resolveChatCalendarRead(question, nil, now)
		if !ok || query.from != want {
			t.Errorf("%q: %+v, %t, esperado %s", question, query, ok, want)
		}
	}
}

func TestCalendarReadPreservesOtherIntents(t *testing.T) {
	for _, question := range []string{
		"crie 13 eventos hoje",
		"liste as tarefas de hoje",
		"eventos aprovados hoje",
		"eventos do cliente Bari hoje",
		"compare os eventos de hoje com ontem",
		"eventos Bari hoje",
		"eventos do João hoje",
		"faça 13 eventos hoje",
		"não liste eventos hoje",
		"liste apenas 1 evento hoje",
		"liste eventos de hoje e amanhã",
		"liste eventos de 13/11 a 14/11",
		"agenda de outubro e novembro",
	} {
		if query, ok := resolveChatCalendarRead(question, nil, time.Now()); ok {
			t.Errorf("%q foi tratado como listagem simples: %+v", question, query)
		}
	}
}

func TestCalendarReadRelativePeriodsAndExplicitMonth(t *testing.T) {
	now := time.Date(2026, time.November, 1, 14, 0, 0, 0, time.UTC)
	for question, want := range map[string]chatCalendarRead{
		"o que temos essa semana?": {from: "2026-10-26", to: "2026-11-01"},
		"liste eventos deste mês":  {from: "2026-11-01", to: "2026-11-30"},
		"agenda de outubro":        {from: "2026-10-01", to: "2026-10-31"},
		"agenda de 2027-02":        {from: "2027-02-01", to: "2027-02-28"},
		"eventos de 22 de outubro": {from: "2026-10-22", to: "2026-10-22"},
		"agenda de 2026-11-13":     {from: "2026-11-13", to: "2026-11-13"},
	} {
		query, ok := resolveChatCalendarRead(question, nil, now)
		if !ok || query != want {
			t.Errorf("%q: %+v, %t, esperado %+v", question, query, ok, want)
		}
	}
}

func TestCalendarReadCountComesFromAuthoritativeCards(t *testing.T) {
	query := chatCalendarRead{from: "2026-11-13", to: "2026-11-13"}
	events := make([]AIContextEvent, 7)
	answer := calendarReadAnswer(query, events, false)
	if !strings.Contains(answer, "7 eventos") || !strings.Contains(answer, "7 cards") || !strings.Contains(answer, "13/11/2026") {
		t.Fatal(answer)
	}
	if answer = calendarReadAnswer(query, nil, false); !strings.Contains(answer, "Não há eventos") {
		t.Fatal(answer)
	}
	if answer = calendarReadAnswer(query, events, true); !strings.Contains(answer, "primeiros") || !strings.Contains(answer, "Há mais") {
		t.Fatal("lista truncada apresentada como total: " + answer)
	}
}

func TestCalendarCardAnswerDoesNotKeepUnconfirmedCount(t *testing.T) {
	answer := "Hoje há 13 eventos. Veja os cards."
	items := []AIContextEvent{{ID: "real"}}
	if got := groundedCalendarCardAnswer(answer, []string{"real", "inventado"}, items); strings.Contains(got, "13 eventos") || !strings.Contains(got, "1 item") {
		t.Fatal(got)
	}
	if got := groundedCalendarCardAnswer(answer, []string{"inventado"}, nil); strings.Contains(got, "13 eventos") {
		t.Fatal(got)
	}
	if got := groundedCalendarCardAnswer("Um evento confirmado.", []string{"real", "real"}, items); got != "Um evento confirmado." {
		t.Fatal(got)
	}
}
