package calendar

import "testing"

// Testes dos sanitizers de proposta de ANOTACAO e PERFIL do chat (WAVE 7). sanitizeProposal e o
// limite de confianca: valida a saida do LLM antes de persistir/expor. Espelham a simulacao do no
// "Extrair resposta" do workflow calendar-chat.

func TestSanitizeProposalNoteAppendNormalizes(t *testing.T) {
	t.Parallel()
	clean := sanitizeProposal(&ChatProposal{
		Kind: "note",
		Fields: ChatProposalFields{
			Note: &ChatProposalNote{Content: " gravar reels ", Mode: "BANANA", Month: "2026-07"},
		},
	})
	if clean == nil {
		t.Fatal("expected note proposal to be accepted")
	}
	if clean.Kind != "note" || clean.Action != "create" {
		t.Fatalf("got kind=%q action=%q", clean.Kind, clean.Action)
	}
	if clean.Fields.Note.Content != "gravar reels" {
		t.Fatalf("content got %q", clean.Fields.Note.Content)
	}
	if clean.Fields.Note.Mode != "" {
		t.Fatalf("invalid mode should normalize to empty (append no front), got %q", clean.Fields.Note.Mode)
	}
	if clean.Fields.Note.Month != "2026-07" {
		t.Fatalf("month got %q", clean.Fields.Note.Month)
	}
}

func TestSanitizeProposalNoteDeleteWithoutContent(t *testing.T) {
	t.Parallel()
	clean := sanitizeProposal(&ChatProposal{Action: "delete", Kind: "note"})
	if clean == nil {
		t.Fatal("delete note (clear) should be accepted without content")
	}
}

func TestSanitizeProposalNoteRejectsEmptyContent(t *testing.T) {
	t.Parallel()
	clean := sanitizeProposal(&ChatProposal{
		Kind:   "note",
		Fields: ChatProposalFields{Note: &ChatProposalNote{Content: "   "}},
	})
	if clean != nil {
		t.Fatalf("empty note create should be rejected: %#v", clean)
	}
}

func TestSanitizeProposalNoteInvalidMonthCleared(t *testing.T) {
	t.Parallel()
	clean := sanitizeProposal(&ChatProposal{
		Kind:   "note",
		Fields: ChatProposalFields{Note: &ChatProposalNote{Content: "x", Month: "julho"}},
	})
	if clean == nil {
		t.Fatal("expected accepted")
	}
	if clean.Fields.Note.Month != "" {
		t.Fatalf("invalid month should be cleared, got %q", clean.Fields.Note.Month)
	}
}

func TestSanitizeProposalClientProfileMergeCanonicalizesKind(t *testing.T) {
	t.Parallel()
	clean := sanitizeProposal(&ChatProposal{
		Action: "update",
		Kind:   "clientprofile", // variante -> canonicaliza para clientProfile
		Fields: ChatProposalFields{
			ClientID: " abc ",
			Profile:  &ChatProposalProfile{Segment: " Joias ", Extra: &ProfileExtra{Audience: " classe A "}},
		},
	})
	if clean == nil {
		t.Fatal("expected clientProfile proposal to be accepted")
	}
	if clean.Kind != "clientProfile" {
		t.Fatalf("kind should canonicalize to clientProfile, got %q", clean.Kind)
	}
	if clean.Fields.Profile.Segment != "Joias" {
		t.Fatalf("segment got %q", clean.Fields.Profile.Segment)
	}
	if clean.Fields.Profile.Extra == nil || clean.Fields.Profile.Extra.Audience != "classe A" {
		t.Fatalf("extra audience got %#v", clean.Fields.Profile.Extra)
	}
}

func TestSanitizeProposalClientProfileRejectsEmpty(t *testing.T) {
	t.Parallel()
	clean := sanitizeProposal(&ChatProposal{
		Kind:   "clientProfile",
		Fields: ChatProposalFields{ClientID: "abc", Profile: &ChatProposalProfile{}},
	})
	if clean != nil {
		t.Fatalf("clientProfile with no field should be rejected: %#v", clean)
	}
}

func TestSanitizeProposalClientProfileDelete(t *testing.T) {
	t.Parallel()
	all := sanitizeProposal(&ChatProposal{
		Action: "delete", Kind: "clientProfile",
		Fields: ChatProposalFields{ClientID: "abc", Profile: &ChatProposalProfile{ClearAll: true}},
	})
	if all == nil {
		t.Fatal("delete clearAll should be accepted")
	}
	fields := sanitizeProposal(&ChatProposal{
		Action: "delete", Kind: "clientProfile",
		Fields: ChatProposalFields{
			ClientID: "abc",
			Profile:  &ChatProposalProfile{ClearFields: []string{"history", "naoexiste", "history"}},
		},
	})
	if fields == nil {
		t.Fatal("delete clearFields should be accepted")
	}
	if got := fields.Fields.Profile.ClearFields; len(got) != 1 || got[0] != "history" {
		t.Fatalf("clearFields should keep only known+dedup, got %#v", got)
	}
	empty := sanitizeProposal(&ChatProposal{
		Action: "delete", Kind: "clientProfile",
		Fields: ChatProposalFields{ClientID: "abc", Profile: &ChatProposalProfile{}},
	})
	if empty != nil {
		t.Fatalf("empty profile delete should be rejected: %#v", empty)
	}
}

func TestMissingProfileFields(t *testing.T) {
	t.Parallel()
	got := missingProfileFields(planProfile{Segment: "Joias", BrandVoice: "luxo"})
	if len(got) != 7 {
		t.Fatalf("expected 7 missing stable fields, got %d: %#v", len(got), got)
	}
	for _, k := range got {
		if k == "segment" || k == "brandVoice" {
			t.Fatalf("filled field %q should not be reported missing", k)
		}
	}
}
