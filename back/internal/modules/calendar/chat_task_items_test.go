package calendar

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func boolPointer(value bool) *bool { return &value }

func TestSanitizeTaskItemProposalContract(t *testing.T) {
	t.Parallel()
	create := sanitizeProposal(&ChatProposal{
		Kind: "task_item",
		Fields: ChatProposalFields{
			TargetID: " Campanha Agosto ",
			TaskItem: &ChatProposalTaskItem{Title: " Reel 01 ", Status: "posted", Completed: boolPointer(false)},
		},
	})
	if create == nil {
		t.Fatal("create valido deveria sobreviver")
	}
	if create.Kind != "taskItem" || create.Fields.TargetID != "Campanha Agosto" {
		t.Fatalf("canonicalizacao inesperada: %#v", create)
	}
	item := create.Fields.TaskItem
	if item.Title != "Reel 01" || item.Status != "posted" || item.Completed == nil || *item.Completed {
		t.Fatalf("item normalizado incorretamente: %#v", item)
	}
	if item.CompletedDate != "" {
		t.Fatalf("completed=false deve limpar completedDate, veio %q", item.CompletedDate)
	}
	raw, err := json.Marshal(create)
	if err != nil || !strings.Contains(string(raw), `"completed":false`) {
		t.Fatalf("completed=false deve sobreviver no JSON: raw=%s err=%v", raw, err)
	}

	cases := []ChatProposal{
		{Kind: "taskItem", Fields: ChatProposalFields{TargetID: "task-1", TaskItem: &ChatProposalTaskItem{}}},
		{Action: "update", Kind: "taskItem", Fields: ChatProposalFields{TargetID: "task-1", TaskItem: &ChatProposalTaskItem{ID: "item-1"}}},
		{Action: "update", Kind: "taskItem", Fields: ChatProposalFields{TargetID: "task-1", TaskItem: &ChatProposalTaskItem{Title: "novo"}}},
		{Action: "delete", Kind: "taskItem", Fields: ChatProposalFields{TargetID: "task-1", TaskItem: &ChatProposalTaskItem{}}},
		{Action: "delete", Kind: "taskItem", Fields: ChatProposalFields{TaskItem: &ChatProposalTaskItem{ID: "item-1"}}},
	}
	for i := range cases {
		proposal := cases[i]
		if clean := sanitizeProposal(&proposal); clean != nil {
			t.Fatalf("caso invalido %d deveria ser descartado: %#v", i, clean)
		}
	}
}

func TestSanitizeTaskItemRejectsInvalidFieldEvenWithAnotherEdit(t *testing.T) {
	t.Parallel()
	cases := []ChatProposal{
		{Action: "update", Kind: "taskItem", Fields: ChatProposalFields{TargetID: "task-1", TaskItem: &ChatProposalTaskItem{ID: "item-1", Title: "Novo", Status: "POSTED"}}},
		{Action: "update", Kind: "taskItem", Fields: ChatProposalFields{TargetID: "task-1", TaskItem: &ChatProposalTaskItem{ID: "item-1", Title: "Novo", StatusDate: "ontem"}}},
	}
	for i := range cases {
		proposal := cases[i]
		if clean := sanitizeProposal(&proposal); clean != nil {
			t.Fatalf("campo taskItem invalido %d nao pode ser ignorado: %#v", i, clean)
		}
	}
}

func TestSanitizeTaskItemDatesAndStatusAreStrict(t *testing.T) {
	t.Parallel()
	proposal := ChatProposal{
		Action: "update", Kind: "taskItem",
		Fields: ChatProposalFields{TargetID: "task-1", TaskItem: &ChatProposalTaskItem{
			ID: "item-1", Status: "waiting", StatusDate: "2026-02-30", CompletedDate: "ontem",
		}},
	}
	if clean := sanitizeProposal(&proposal); clean != nil {
		t.Fatalf("status/datas invalidos nao podem formar update editavel: %#v", clean)
	}
}

func TestSanitizeTaskItemCreateRejectsOrphanDates(t *testing.T) {
	t.Parallel()
	cases := []ChatProposal{
		{Kind: "taskItem", Fields: ChatProposalFields{TargetID: "task-1", TaskItem: &ChatProposalTaskItem{Title: "Reel", StatusDate: "2026-08-13"}}},
		{Kind: "taskItem", Fields: ChatProposalFields{TargetID: "task-1", TaskItem: &ChatProposalTaskItem{Title: "Reel", CompletedDate: "2026-08-13"}}},
		{Kind: "taskItem", Fields: ChatProposalFields{TargetID: "task-1", TaskItem: &ChatProposalTaskItem{Title: "Reel", Completed: boolPointer(false), CompletedDate: "2026-08-13"}}},
	}
	for i := range cases {
		proposal := cases[i]
		if clean := sanitizeProposal(&proposal); clean != nil {
			t.Fatalf("data orfa no create %d deveria ser rejeitada: %#v", i, clean)
		}
	}
}

func TestResolveTaskItemCreateUsesRealTaskAndActionDates(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 14, 1, 0, 0, 0, time.UTC) // ainda 13/08 em Sao Paulo
	proposals := []ChatProposal{{
		Action: "create", Kind: "taskItem",
		Fields: ChatProposalFields{TargetID: "campanha agosto", TaskItem: &ChatProposalTaskItem{
			ID: "inventado", ItemTitle: "inventado", TaskTitle: "inventada", Title: "Reel 02",
			Status: "editing", Completed: boolPointer(true),
		}},
	}}
	tasks := []AIContextTask{{ID: "task-real", Title: "Campanha Agosto"}}
	got, dropped := resolveTaskItemProposals(proposals, tasks, now)
	if dropped != 0 {
		t.Fatalf("create valido nao deveria ser descartado: %d", dropped)
	}
	if len(got) != 1 {
		t.Fatalf("esperava uma proposta, veio %#v", got)
	}
	fields := got[0].Fields
	if fields.TargetID != "task-real" || fields.TaskItem.TaskTitle != "Campanha Agosto" {
		t.Fatalf("task nao foi autoritativamente resolvida: %#v", fields)
	}
	if fields.TaskItem.ID != "" || fields.TaskItem.ItemTitle != "" {
		t.Fatalf("create nao pode preservar id/titulo de item inventado: %#v", fields.TaskItem)
	}
	if fields.TaskItem.StatusDate != "2026-08-13" || fields.TaskItem.CompletedDate != "2026-08-13" {
		t.Fatalf("datas automaticas devem usar hoje em Sao Paulo: %#v", fields.TaskItem)
	}
}

func TestResolveTaskItemUpdateAndDeleteRequireRealUniqueTargets(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 13, 15, 0, 0, 0, time.UTC)
	tasks := []AIContextTask{{
		ID: "task-real", Title: "Campanha Agosto",
		Items: []AIContextTaskItem{
			{ID: "item-1", Title: "Reel Dr Lucas", Completed: true, Status: "approval", StatusDate: "2026-08-10", CompletedDate: "2026-08-11"},
			{ID: "item-2", Title: "Reel Dr Lucas Corte", Completed: false},
		},
	}}
	proposals := []ChatProposal{
		{Action: "update", Kind: "taskItem", Fields: ChatProposalFields{TargetID: "task-real", TaskItem: &ChatProposalTaskItem{ID: "item-1", Status: "posted", Completed: boolPointer(false), CompletedDate: "2026-08-11"}}},
		{Action: "delete", Kind: "taskItem", Fields: ChatProposalFields{TargetID: "Campanha Agosto", TaskItem: &ChatProposalTaskItem{ID: "inventado", ItemTitle: "Reel Dr Lucas Corte"}}},
		{Action: "delete", Kind: "taskItem", Fields: ChatProposalFields{TargetID: "Campanha Agosto", TaskItem: &ChatProposalTaskItem{ItemTitle: "Reel"}}},
		{Action: "update", Kind: "taskItem", Fields: ChatProposalFields{TargetID: "nao existe", TaskItem: &ChatProposalTaskItem{ID: "item-1", Title: "x"}}},
	}
	got, dropped := resolveTaskItemProposals(proposals, tasks, now)
	if len(got) != 2 {
		t.Fatalf("apenas alvos inequivocos deveriam sobreviver, veio %#v", got)
	}
	if dropped != 2 {
		t.Fatalf("deveria sinalizar os dois alvos invalidos/ambiguos, veio %d", dropped)
	}
	updated := got[0].Fields.TaskItem
	if updated.ItemTitle != "Reel Dr Lucas" || updated.TaskTitle != "Campanha Agosto" {
		t.Fatalf("snapshots autoritativos ausentes: %#v", updated)
	}
	if updated.StatusDate != "2026-08-13" || updated.CompletedDate != "" || updated.Completed == nil || *updated.Completed {
		t.Fatalf("transicoes/datas incorretas: %#v", updated)
	}
	deleted := got[1].Fields.TaskItem
	if deleted.ID != "item-2" || deleted.ItemTitle != "Reel Dr Lucas Corte" {
		t.Fatalf("fallback por titulo unico nao resolveu item real: %#v", deleted)
	}
}

func TestTaskItemResolutionNoticeDoesNotLetModelClaimSuccess(t *testing.T) {
	t.Parallel()
	if got := taskItemResolutionNotice(1, false); !strings.Contains(got, "Nao consegui identificar") || !strings.Contains(got, "titulo exato") {
		t.Fatalf("aviso sem orientacao deterministica: %q", got)
	}
	if got := taskItemResolutionNotice(1, true); !strings.HasPrefix(got, "Preparei os outros cartoes, mas") {
		t.Fatalf("aviso deve preservar outras propostas validas: %q", got)
	}
	if got := taskItemResolutionNotice(0, true); got != "" {
		t.Fatalf("sem descarte nao deve haver aviso: %q", got)
	}
}

func TestResolveTaskItemUpdateDateOnlyRequiresCurrentState(t *testing.T) {
	t.Parallel()
	tasks := []AIContextTask{{
		ID: "task-1", Title: "Campanha",
		Items: []AIContextTaskItem{
			{ID: "sem-estado", Title: "Sem estado"},
			{ID: "com-estado", Title: "Com estado", Status: "approval", Completed: true},
		},
	}}
	proposals := []ChatProposal{
		{Action: "update", Kind: "taskItem", Fields: ChatProposalFields{TargetID: "task-1", TaskItem: &ChatProposalTaskItem{ID: "sem-estado", StatusDate: "2026-08-12"}}},
		{Action: "update", Kind: "taskItem", Fields: ChatProposalFields{TargetID: "task-1", TaskItem: &ChatProposalTaskItem{ID: "sem-estado", CompletedDate: "2026-08-12"}}},
		{Action: "update", Kind: "taskItem", Fields: ChatProposalFields{TargetID: "task-1", TaskItem: &ChatProposalTaskItem{ID: "com-estado", StatusDate: "2026-08-12"}}},
		{Action: "update", Kind: "taskItem", Fields: ChatProposalFields{TargetID: "task-1", TaskItem: &ChatProposalTaskItem{ID: "com-estado", CompletedDate: "2026-08-12"}}},
	}
	got, dropped := resolveTaskItemProposals(proposals, tasks, time.Date(2026, 8, 13, 15, 0, 0, 0, time.UTC))
	if dropped != 2 || len(got) != 2 {
		t.Fatalf("datas isoladas devem depender do estado atual: kept=%#v dropped=%d", got, dropped)
	}
	if got[0].Fields.TaskItem.ID != "com-estado" || got[0].Fields.TaskItem.StatusDate != "2026-08-12" {
		t.Fatalf("statusDate valida foi alterada: %#v", got[0])
	}
	if got[1].Fields.TaskItem.ID != "com-estado" || got[1].Fields.TaskItem.CompletedDate != "2026-08-12" {
		t.Fatalf("completedDate valida foi alterada: %#v", got[1])
	}
}

func TestTaskItemDoesNotEnterCalendarPriorityGuard(t *testing.T) {
	t.Parallel()
	proposal := ChatProposal{Action: "update", Kind: "taskItem", Fields: ChatProposalFields{TargetID: "task-1"}}
	if isEditableTargetProposal(proposal) {
		t.Fatal("taskItem nao pode passar pela guarda de prioridade do Calendar")
	}
}

func TestDescribeStoredProposalTaskItem(t *testing.T) {
	t.Parallel()
	got := describeStoredProposal(StoredProposal{
		Action: "update", Kind: "taskItem", Status: "pending",
		Fields: ChatProposalFields{TaskItem: &ChatProposalTaskItem{
			ItemTitle: "Reel 01", TaskTitle: "Campanha Agosto", Status: "posted",
			StatusDate: "2026-08-13", Completed: boolPointer(true), CompletedDate: "2026-08-13",
		}},
	})
	for _, want := range []string{"editar item da tarefa", `"Reel 01"`, "na tarefa Campanha Agosto", "status posted em 2026-08-13", "concluido em 2026-08-13", "(pendente)"} {
		if !strings.Contains(got, want) {
			t.Fatalf("resumo sem %q: %s", want, got)
		}
	}
}
