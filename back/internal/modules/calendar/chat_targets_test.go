package calendar

import "testing"

// Cobertura da resolucao server-side dos alvos das propostas (WAVE 12): reescrita de
// targetId por titulo, snapshot de task sintetizado e merge sem duplicatas.

func targetsFixture() ([]AIContextEvent, []AIContextTask) {
	events := []AIContextEvent{
		{ID: "ev-1", Title: "Reel de Dona Evania", TaskID: "task-9", ClientID: "cli-1", ClientName: "Evania"},
		{ID: "ev-2", Title: "Post institucional", ClientID: "cli-2"},
	}
	tasks := []AIContextTask{
		{ID: "task-7", Title: "Brasil GMS Tooop", DueDate: "2026-07-14T00:00:00Z", Status: "producao"},
		{ID: "task-8", Title: "Roteiro do video", DueDate: "2026-07-15T18:30:00Z"},
	}
	return events, tasks
}

func TestResolveProposalTargetsRewritesTitleToRealID(t *testing.T) {
	events, tasks := targetsFixture()
	proposals := []ChatProposal{
		{Action: "update", Kind: "task", Fields: ChatProposalFields{TargetID: "Brasil GMS", Status: "revisao"}},
	}
	snapshots := resolveProposalTargets(proposals, events, tasks)
	if proposals[0].Fields.TargetID != "task-7" {
		t.Fatalf("targetId nao reescrito: %q", proposals[0].Fields.TargetID)
	}
	if len(snapshots) != 1 || snapshots[0].ID != "task-7" || snapshots[0].TaskID != "task-7" {
		t.Fatalf("snapshot da task esperado, veio %+v", snapshots)
	}
	// Meia-noite UTC = data-only (heuristica do mirror): data em UTC, sem hora.
	if snapshots[0].Date != "2026-07-14" || snapshots[0].Time != "" {
		t.Fatalf("data/hora do snapshot erradas: %q %q", snapshots[0].Date, snapshots[0].Time)
	}
}

func TestResolveProposalTargetsEventByIDAndTaskID(t *testing.T) {
	events, tasks := targetsFixture()
	proposals := []ChatProposal{
		{Action: "delete", Kind: "event", Fields: ChatProposalFields{TargetID: "ev-2"}},
		// targetId = taskId de evento vinculado => snapshot do EVENTO (nao da task).
		{Action: "update", Kind: "task", Fields: ChatProposalFields{TargetID: "task-9", Priority: "alta"}},
	}
	snapshots := resolveProposalTargets(proposals, events, tasks)
	if len(snapshots) != 2 || snapshots[0].ID != "ev-2" || snapshots[1].ID != "ev-1" {
		t.Fatalf("snapshots inesperados: %+v", snapshots)
	}
}

func TestResolveProposalTargetsAmbiguousOrCreateUntouched(t *testing.T) {
	events, tasks := targetsFixture()
	proposals := []ChatProposal{
		// "de" casa com varios titulos => ambiguo, nao mexe.
		{Action: "update", Kind: "event", Fields: ChatProposalFields{TargetID: "de", Title: "x"}},
		{Action: "create", Kind: "event", Fields: ChatProposalFields{Title: "Novo post"}},
	}
	snapshots := resolveProposalTargets(proposals, events, tasks)
	if len(snapshots) != 0 {
		t.Fatalf("nenhum snapshot esperado, veio %+v", snapshots)
	}
	if proposals[0].Fields.TargetID != "de" {
		t.Fatalf("targetId ambiguo nao devia ser reescrito: %q", proposals[0].Fields.TargetID)
	}
}

func TestResolveProposalTargetsAccentInsensitive(t *testing.T) {
	events, tasks := targetsFixture()
	proposals := []ChatProposal{
		{Action: "update", Kind: "event", Fields: ChatProposalFields{TargetID: "reel de dona evânia", Status: "aprovada"}},
	}
	snapshots := resolveProposalTargets(proposals, events, tasks)
	if proposals[0].Fields.TargetID != "ev-1" || len(snapshots) != 1 {
		t.Fatalf("titulo com acento devia casar ev-1: %q %+v", proposals[0].Fields.TargetID, snapshots)
	}
}

func TestMergeCalendarItemsDedupes(t *testing.T) {
	items := []AIContextEvent{{ID: "ev-1"}}
	extra := []AIContextEvent{{ID: "ev-1"}, {ID: "task-7"}, {ID: ""}}
	out := mergeCalendarItems(items, extra)
	if len(out) != 2 || out[1].ID != "task-7" {
		t.Fatalf("merge errado: %+v", out)
	}
}

func TestTaskDueDatePartsWithRealTime(t *testing.T) {
	// 18:30 UTC = 15:30 em Sao Paulo (UTC-3).
	date, tm := taskDueDateParts("2026-07-15T18:30:00Z")
	if date != "2026-07-15" || tm != "15:30" {
		t.Fatalf("conversao SP errada: %q %q", date, tm)
	}
	if d, tme := taskDueDateParts("2026-07-20"); d != "2026-07-20" || tme != "" {
		t.Fatalf("data pura errada: %q %q", d, tme)
	}
}

// WAVE 16: hidratacao do perfil do cliente CITADO no escopo 'all'.

func namedClientFixture() []AIContextClientLean {
	return []AIContextClientLean{
		{ID: "cli-am", Name: "AM Malls"},
		{ID: "cli-bari", Name: "Bari"},
		{ID: "cli-perola", Name: "Perola"},
	}
}

func TestSingleNamedClientResolvesMultiWord(t *testing.T) {
	// "traz os dados do AM Malls que temos" cita 1 cliente (nome com 2 palavras) => resolve.
	got := singleNamedClient("me traga os dados do AM Malls que temos", namedClientFixture())
	if got == nil || got.ID != "cli-am" {
		t.Fatalf("devia resolver AM Malls (cli-am), veio %#v", got)
	}
}

func TestSingleNamedClientAccentInsensitive(t *testing.T) {
	got := singleNamedClient("quais os objetivos da Pérola?", namedClientFixture())
	if got == nil || got.ID != "cli-perola" {
		t.Fatalf("devia resolver Perola sem acento, veio %#v", got)
	}
}

func TestSingleNamedClientAmbiguousReturnsNil(t *testing.T) {
	// dois clientes citados => ambiguo, nao hidrata (nil).
	got := singleNamedClient("compara a Bari com a Perola", namedClientFixture())
	if got != nil {
		t.Fatalf("dois clientes citados deviam dar nil (ambiguo), veio %#v", got)
	}
}

func TestSingleNamedClientNoneReturnsNil(t *testing.T) {
	if got := singleNamedClient("quantos eventos tem esse mes?", namedClientFixture()); got != nil {
		t.Fatalf("nenhum cliente citado devia dar nil, veio %#v", got)
	}
}

func TestPlanProfileFromClientProfileCopiesAllFields(t *testing.T) {
	src := ClientProfile{
		Segment: "Joias", Positioning: "Luxo acessivel", Description: "Loja de joias",
		History: "Fundada em 2010", SiteURL: "x.com", Instagram: "@x", Address: "Rua X",
		Objectives: "Vender mais", BrandVoice: "Elegante",
		Extra: ProfileExtra{Audience: "Mulheres 30+", Offer: "Aneis"},
	}
	got := planProfileFromClientProfile(src)
	if got.Positioning != "Luxo acessivel" || got.Objectives != "Vender mais" ||
		got.History != "Fundada em 2010" || got.Extra.Audience != "Mulheres 30+" {
		t.Fatalf("campos pesados nao copiados: %#v", got)
	}
}

func TestFoldChatLabelPunctuation(t *testing.T) {
	cases := map[string]string{
		"da Pérola: segmento":   "da perola segmento",    // ":" apos palavra vira espaco
		"objetivos da Bari?":    "objetivos da bari",     // "?" vira espaco
		"evento 15/07 as 14:30": "evento 15/07 as 14:30", // "/" e ":" ENTRE digitos preservados
		"Campanha multi-dia":    "campanha multi dia",    // hifen ja virava espaco
	}
	for in, want := range cases {
		if got := foldChatLabel(in); got != want {
			t.Fatalf("foldChatLabel(%q) = %q, quer %q", in, got, want)
		}
	}
}
