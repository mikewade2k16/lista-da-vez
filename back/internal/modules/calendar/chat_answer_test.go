package calendar

import "testing"

func TestCompactCalendarCardAnswerRemovesRepeatedList(t *testing.T) {
	t.Parallel()
	answer := "Para julho/2026, temos 29 eventos no total:\n\n" +
		"• 02/07: 1 evento — Video de Animacao\n" +
		"• 04/07: 1 evento — Reuniao RMR\n" +
		"• 08/07: 2 eventos — Reels + Video\n" +
		"• 09/07: 2 eventos — Reels + Entrega\n\n" +
		"Feriado do mes: 08/07 — Emancipacao Politica de Sergipe."

	got := compactCalendarCardAnswer(answer, 29)
	want := "Encontrei 29 itens no calendario. Feriado do mes: 08/07 — Emancipacao Politica de Sergipe. A lista completa esta nos cards abaixo."
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestCompactCalendarCardAnswerKeepsNormalSummary(t *testing.T) {
	t.Parallel()
	answer := "Temos uma reuniao no dia 04/07 e uma entrega no dia 09/07."

	got := compactCalendarCardAnswer(answer, 2)
	if got != answer {
		t.Fatalf("got %q, want original %q", got, answer)
	}
}

func TestSanitizeProposalAcceptsPartialUpdateFields(t *testing.T) {
	t.Parallel()
	clean := sanitizeProposal(&ChatProposal{
		Action: "update",
		Kind:   "event",
		Fields: ChatProposalFields{
			TargetID:    "event-1",
			Priority:    " alta ",
			Description: " Nova descricao ",
			InvolvedIDs: []string{" user-1 ", "", "user-1", "user-2"},
		},
	})
	if clean == nil {
		t.Fatal("expected partial update proposal to be accepted")
	}
	if clean.Fields.Title != "" {
		t.Fatalf("title should stay optional on update, got %q", clean.Fields.Title)
	}
	if clean.Fields.Priority != "alta" {
		t.Fatalf("priority got %q", clean.Fields.Priority)
	}
	if clean.Fields.Description != "Nova descricao" {
		t.Fatalf("description got %q", clean.Fields.Description)
	}
	if got := clean.Fields.InvolvedIDs; len(got) != 2 || got[0] != "user-1" || got[1] != "user-2" {
		t.Fatalf("involved ids got %#v", got)
	}
}

func TestSanitizeProposalRejectsEmptyUpdate(t *testing.T) {
	t.Parallel()
	clean := sanitizeProposal(&ChatProposal{
		Action: "update",
		Kind:   "event",
		Fields: ChatProposalFields{TargetID: "event-1"},
	})
	if clean != nil {
		t.Fatalf("expected empty update proposal to be rejected: %#v", clean)
	}
}

func TestSanitizeProposalListDeduplicatesEquivalentItems(t *testing.T) {
	t.Parallel()
	list := sanitizeProposalList(nil, []ChatProposal{
		{
			Action: "update",
			Kind:   "event",
			Fields: ChatProposalFields{TargetID: "event-1", ResponsibleID: "user-1"},
		},
		{
			Action: "update",
			Kind:   "event",
			Fields: ChatProposalFields{TargetID: "event-1", ResponsibleID: "user-1"},
		},
	})
	if len(list) != 1 {
		t.Fatalf("expected 1 proposal after dedupe, got %d: %#v", len(list), list)
	}
}
