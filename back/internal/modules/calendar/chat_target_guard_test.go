package calendar

import (
	"strings"
	"testing"
)

// Cobertura da GUARDA DE ALVO (WAVE 14): o back RESOLVE o alvo (titulo-primeiro > dia/cliente),
// prioriza o calendario, nao filtra por cliente-que-e-valor-atribuido e avisa quando o item so
// existe em Tasks.

func guardFixture() ([]AIContextEvent, []AIContextTask, []AIContextClientLean) {
	events := []AIContextEvent{
		{ID: "ev-15", Title: "Story diario Perola", Date: "2026-07-15", ClientID: "cli-perola", ClientName: "Perola", TaskID: "tk-15"},
		{ID: "ev-15b", Title: "Post institucional", Date: "2026-07-15", ClientID: "cli-bari", ClientName: "Bari"},
		{ID: "ev-04", Title: "Story diario Perola", Date: "2026-07-04", ClientID: "cli-perola", ClientName: "Perola"},
	}
	tasks := []AIContextTask{
		// task vinculada ao evento do dia 15 (mesmo item, no calendario)
		{ID: "tk-15", Title: "Story diario Perola", DueDate: "2026-07-15T00:00:00Z", ClientID: "cli-perola", ClientName: "Perola"},
		// task SEM evento no calendario (DB002 do print) — sem data
		{ID: "tk-db002", Title: "DB002 Visita a clientes", DueDate: "", ClientID: ""},
	}
	clients := []AIContextClientLean{
		{ID: "cli-perola", Name: "Perola"}, {ID: "cli-bari", Name: "Bari"},
	}
	return events, tasks, clients
}

// bariFixture reproduz o print de 2026-07-14: "Postagem Bari" SEM cliente no dia 14;
// "Gravacao Perolas" COM cliente Bari no dia 14; "Tarefa teste" com cliente Bari no dia 08.
func bariFixture() ([]AIContextEvent, []AIContextTask, []AIContextClientLean) {
	events := []AIContextEvent{
		{ID: "ev-post", Title: "Postagem Bari", Date: "2026-07-14", ClientID: ""},
		{ID: "ev-grav", Title: "Gravacao Perolas", Date: "2026-07-14", ClientID: "cli-bari", ClientName: "Bari"},
		{ID: "ev-tt", Title: "Tarefa teste", Date: "2026-07-08", ClientID: "cli-bari", ClientName: "Bari"},
	}
	clients := []AIContextClientLean{{ID: "cli-bari", Name: "Bari"}}
	return events, nil, clients
}

func TestGuardTitleFirstWinsOverClientFilter(t *testing.T) {
	// CASO DO PRINT: "na postagem Bari o cliente deve ser bari..." — o titulo citado
	// ("Postagem Bari", SEM cliente hoje) tem que vencer o filtro de cliente (que excluia
	// o proprio alvo e forcava a Gravacao Perolas).
	events, tasks, clients := bariFixture()
	question := "pronto na postagem Bari o cliente deve ser bari e ja edita isso cliente Bari responsavel Mike e a descricao vai ser gravar um video"
	crit := extractTargetCriteria(question, "2026-07", clients)
	props := []ChatProposal{{Action: "update", Kind: "event", Fields: ChatProposalFields{TargetID: "ev-grav", ClientID: "cli-bari", Description: "Gravar um video"}}}
	kept, notice, resolved := guardProposalTargets(question, props, crit, clients, events, tasks)
	if len(kept) != 1 || notice != "" {
		t.Fatalf("titulo citado devia resolver sem aviso: kept=%d notice=%q", len(kept), notice)
	}
	if kept[0].Fields.TargetID != "ev-post" {
		t.Fatalf("alvo devia ser a Postagem Bari (ev-post), veio %q", kept[0].Fields.TargetID)
	}
	if !strings.Contains(resolved, "Postagem Bari") || !strings.Contains(resolved, "14/07") {
		t.Fatalf("resolvedTitle deve nomear o alvo com a data: %q", resolved)
	}
}

func TestGuardTitleFirstShortFollowUp(t *testing.T) {
	// Follow-up do print: "Postagem bari dia 14" — titulo + dia => resolve direto.
	events, tasks, clients := bariFixture()
	question := "Postagem bari dia 14"
	crit := extractTargetCriteria(question, "2026-07", clients)
	props := []ChatProposal{{Action: "update", Kind: "event", Fields: ChatProposalFields{TargetID: "ev-grav", Description: "x"}}}
	kept, notice, _ := guardProposalTargets(question, props, crit, clients, events, tasks)
	if len(kept) != 1 || notice != "" || kept[0].Fields.TargetID != "ev-post" {
		t.Fatalf("titulo+dia devia resolver p/ ev-post: kept=%d notice=%q target=%q",
			len(kept), notice, func() string {
				if len(kept) > 0 {
					return kept[0].Fields.TargetID
				}
				return ""
			}())
	}
}

func TestGuardTitleMaximalMatch(t *testing.T) {
	// "gravacao perolas" na pergunta casa "Gravacao" E "Gravacao Perolas" — o match MAXIMAL
	// (mais longo) vence; nao pode virar ambiguidade falsa.
	events := []AIContextEvent{
		{ID: "ev-g", Title: "Gravacao", Date: "2026-07-13", ClientID: ""},
		{ID: "ev-gp", Title: "Gravacao Perolas", Date: "2026-07-14", ClientID: "cli-bari"},
	}
	clients := []AIContextClientLean{{ID: "cli-bari", Name: "Bari"}}
	question := "muda a gravacao perolas pra aprovada"
	crit := extractTargetCriteria(question, "2026-07", clients)
	props := []ChatProposal{{Action: "update", Kind: "event", Fields: ChatProposalFields{TargetID: "ev-g", Status: "aprovada"}}}
	kept, notice, _ := guardProposalTargets(question, props, crit, clients, events, nil)
	if len(kept) != 1 || notice != "" || kept[0].Fields.TargetID != "ev-gp" {
		t.Fatalf("match maximal devia resolver p/ ev-gp: notice=%q", notice)
	}
}

func TestGuardTitleSameTitleTwoDaysUsesDay(t *testing.T) {
	// Mesmo titulo em 2 dias + "dia 4" na pergunta => desempata pelo dia.
	events, tasks, clients := guardFixture()
	question := "muda o story diario perola do dia 4"
	crit := extractTargetCriteria(question, "2026-07", clients)
	props := []ChatProposal{{Action: "update", Kind: "event", Fields: ChatProposalFields{TargetID: "ev-15", Status: "aprovada"}}}
	kept, notice, resolved := guardProposalTargets(question, props, crit, clients, events, tasks)
	if len(kept) != 1 || notice != "" || kept[0].Fields.TargetID != "ev-04" {
		t.Fatalf("titulo+dia devia resolver p/ ev-04: notice=%q resolved=%q", notice, resolved)
	}
}

func TestGuardResolvesSingleDayTarget(t *testing.T) {
	// Sem titulo citado; 1 evento no dia 04 => FORCA o alvo p/ ele, mesmo com a IA
	// escolhendo um id qualquer (a DB002, fora do calendario e sem data).
	events, tasks, clients := guardFixture()
	question := "coloca a iasmin na tarefa do dia 4"
	crit := extractTargetCriteria(question, "2026-07", clients)
	if crit.day != "2026-07-04" {
		t.Fatalf("dia nao extraido: %q", crit.day)
	}
	props := []ChatProposal{{Action: "update", Kind: "task", Fields: ChatProposalFields{TargetID: "tk-db002", ResponsibleID: "u-iasmin"}}}
	kept, notice, resolved := guardProposalTargets(question, props, crit, clients, events, tasks)
	if len(kept) != 1 || notice != "" {
		t.Fatalf("1 evento no dia deve resolver sem aviso: kept=%d notice=%q", len(kept), notice)
	}
	if kept[0].Fields.TargetID != "ev-04" {
		t.Fatalf("targetId devia ser reescrito p/ ev-04, veio %q", kept[0].Fields.TargetID)
	}
	if kept[0].Fields.ResponsibleID != "u-iasmin" {
		t.Fatalf("campos da IA (responsavel) devem ser mantidos")
	}
	if !strings.Contains(resolved, "Story diario Perola") {
		t.Fatalf("resolvedTitle deve nomear o alvo: %q", resolved)
	}
}

func TestGuardBlocksMultipleAndLists(t *testing.T) {
	events, tasks, clients := guardFixture()
	// dia 15 tem 2 eventos no calendario (Story + Post) => barra e lista com as datas.
	question := "tarefa do dia 15"
	crit := extractTargetCriteria(question, "2026-07", clients)
	props := []ChatProposal{{Action: "update", Kind: "event", Fields: ChatProposalFields{TargetID: "ev-15", Status: "x"}}}
	kept, notice, _ := guardProposalTargets(question, props, crit, clients, events, tasks)
	if len(kept) != 0 {
		t.Fatalf("varios no dia => nenhuma proposta aplicada, kept=%d", len(kept))
	}
	if !strings.Contains(notice, "15") || !strings.Contains(notice, "calendario") {
		t.Fatalf("aviso deve citar o dia e o calendario: %q", notice)
	}
	if !strings.Contains(notice, "Story diario Perola") || !strings.Contains(notice, "Post institucional") {
		t.Fatalf("aviso deve listar os 2 eventos do dia 15: %q", notice)
	}
}

func TestGuardOnlyInTasksWarns(t *testing.T) {
	events, tasks, clients := guardFixture()
	// dia 20: SEM evento no calendario; a IA escolheu a DB002 (so em tasks) => avisa Tasks.
	question := "tarefa do dia 20"
	crit := extractTargetCriteria(question, "2026-07", clients)
	props := []ChatProposal{{Action: "update", Kind: "task", Fields: ChatProposalFields{TargetID: "tk-db002"}}}
	kept, notice, _ := guardProposalTargets(question, props, crit, clients, events, tasks)
	if len(kept) != 0 {
		t.Fatalf("proposta fora do calendario devia ser barrada, kept=%d", len(kept))
	}
	if !strings.Contains(notice, "Tasks") || !strings.Contains(notice, "DB002 Visita a clientes") {
		t.Fatalf("aviso deve dizer que so existe em Tasks e nomear o item: %q", notice)
	}
}

func TestGuardClientCriteriaResolves(t *testing.T) {
	events, tasks, clients := guardFixture()
	// "da Bari" no dia 15 (cliente como DONO atual, nada sendo atribuido) => so o Post
	// institucional bate => resolve p/ ele.
	question := "a tarefa da Bari do dia 15"
	crit := extractTargetCriteria(question, "2026-07", clients)
	if !crit.clientIDs["cli-bari"] || crit.day != "2026-07-15" {
		t.Fatalf("criterio cliente/dia nao extraido: %+v", crit)
	}
	props := []ChatProposal{{Action: "update", Kind: "event", Fields: ChatProposalFields{TargetID: "ev-15", Status: "x"}}} // IA escolheu Perola
	kept, notice, resolved := guardProposalTargets(question, props, crit, clients, events, tasks)
	if len(kept) != 1 || notice != "" {
		t.Fatalf("1 evento da Bari no dia deve resolver: kept=%d notice=%q", len(kept), notice)
	}
	if kept[0].Fields.TargetID != "ev-15b" || !strings.Contains(resolved, "Post institucional") {
		t.Fatalf("deve resolver p/ o evento da Bari: target=%q resolved=%q", kept[0].Fields.TargetID, resolved)
	}
}

func TestGuardNoCriteriaPassesThrough(t *testing.T) {
	events, tasks, clients := guardFixture()
	question := "edita essa task"
	crit := extractTargetCriteria(question, "2026-07", clients)
	props := []ChatProposal{{Action: "update", Kind: "task", Fields: ChatProposalFields{TargetID: "tk-db002"}}}
	kept, notice, resolved := guardProposalTargets(question, props, crit, clients, events, tasks)
	if len(kept) != 1 || notice != "" || resolved != "" {
		t.Fatalf("sem criterio deve passar intacto: kept=%d notice=%q resolved=%q", len(kept), notice, resolved)
	}
}

func TestGuardIgnoresCreate(t *testing.T) {
	events, tasks, clients := guardFixture()
	question := "cria uma tarefa no dia 15"
	crit := extractTargetCriteria(question, "2026-07", clients)
	props := []ChatProposal{{Action: "create", Kind: "task", Fields: ChatProposalFields{Title: "Nova"}}}
	kept, notice, _ := guardProposalTargets(question, props, crit, clients, events, tasks)
	if len(kept) != 1 || notice != "" {
		t.Fatalf("create nao tem alvo a validar: kept=%d notice=%q", len(kept), notice)
	}
}

func TestExtractCriteriaNumericDate(t *testing.T) {
	crit := extractTargetCriteria("muda a do 15/07 pra concluida", "2026-07", nil)
	if crit.day != "2026-07-15" {
		t.Fatalf("data numerica 15/07 nao extraida: %q", crit.day)
	}
}

func TestGuardTitleFromOtherYearShowsFullDate(t *testing.T) {
	// Busca ampla pode trazer um evento de OUTRO ano para o contexto (tela em 2025, item em
	// 2026): o destaque tem que mostrar o ANO para nao enganar.
	events := []AIContextEvent{{ID: "ev-post", Title: "Postagem Bari", Date: "2026-07-14", ClientID: ""}}
	clients := []AIContextClientLean{{ID: "cli-bari", Name: "Bari"}}
	question := "vamos pegar postagem Bari e trocar o responsavel"
	crit := extractTargetCriteria(question, "2025-07", clients) // foco em 2025!
	props := []ChatProposal{{Action: "update", Kind: "event", Fields: ChatProposalFields{TargetID: "x", ResponsibleID: "u-iasmin"}}}
	kept, notice, resolved := guardProposalTargets(question, props, crit, clients, events, nil)
	if len(kept) != 1 || notice != "" || kept[0].Fields.TargetID != "ev-post" {
		t.Fatalf("titulo devia resolver: notice=%q", notice)
	}
	if !strings.Contains(resolved, "14/07/2026") {
		t.Fatalf("item de outro ano deve mostrar o ano completo: %q", resolved)
	}
}

func TestWideSearchWindow(t *testing.T) {
	from, to := wideSearchWindow("2025-07")
	if from != "2023-07-01" || to != "2027-07-01" {
		t.Fatalf("janela ampla errada: %q..%q", from, to)
	}
}

func TestGuardTitleHyphenMatchesSpokenSpaces(t *testing.T) {
	// "Campanha multi-dia teste" (hifen) tem que casar com a fala "campanha multi dia teste".
	events := []AIContextEvent{{ID: "ev-camp", Title: "Campanha multi-dia teste", Date: "2026-07-21", ClientID: ""}}
	clients := []AIContextClientLean{{ID: "cli-perola", Name: "Perola"}}
	question := "Vamos alterar na campanha multi dia teste eu quero colocar o responsavel Mike"
	crit := extractTargetCriteria(question, "2026-07", clients)
	props := []ChatProposal{{Action: "update", Kind: "task", Fields: ChatProposalFields{TargetID: "qualquer", ResponsibleID: "u-mike"}}}
	kept, notice, _ := guardProposalTargets(question, props, crit, clients, events, nil)
	if len(kept) != 1 || notice != "" || kept[0].Fields.TargetID != "ev-camp" {
		t.Fatalf("titulo com hifen devia casar a fala sem hifen: notice=%q", notice)
	}
}

func TestResolveClientsJunkIDFromQuestion(t *testing.T) {
	// clientId lixo + cliente citado na pergunta => resolve para o id real e preenche o nome.
	clients := []AIContextClientLean{{ID: "cli-perola", Name: "Perola"}}
	props := []ChatProposal{{Action: "update", Kind: "task", Fields: ChatProposalFields{TargetID: "t", ClientID: "perola-id-lixo"}}}
	resolveClientsInProposals(props, "coloca o cliente Perola nela", clients)
	if props[0].Fields.ClientID != "cli-perola" || props[0].Fields.ClientName != "Perola" {
		t.Fatalf("clientId lixo devia resolver p/ cli-perola: %+v", props[0].Fields)
	}
}

func TestResolveClientsValidIDFillsName(t *testing.T) {
	clients := []AIContextClientLean{{ID: "cli-perola", Name: "Perola"}}
	props := []ChatProposal{{Action: "update", Kind: "task", Fields: ChatProposalFields{TargetID: "t", ClientID: "cli-perola"}}}
	resolveClientsInProposals(props, "edita a tarefa", clients)
	if props[0].Fields.ClientName != "Perola" {
		t.Fatalf("id valido devia ganhar o nome: %+v", props[0].Fields)
	}
}

func TestGuardTitleFuzzyTypo(t *testing.T) {
	// "campanha multi dia testi" (typo no fim) tem que resolver a "Campanha multi-dia teste"
	// via tier fuzzy — rede de seguranca quando o modelo escorrega.
	events := []AIContextEvent{
		{ID: "ev-camp", Title: "Campanha multi-dia teste", Date: "2026-07-21", ClientID: ""},
		{ID: "ev-outro", Title: "Postagem Bari", Date: "2026-07-14", ClientID: ""},
	}
	clients := []AIContextClientLean{{ID: "cli-perola", Name: "Perola"}}
	question := "muda a campanha multi dia testi para prioridade media"
	crit := extractTargetCriteria(question, "2026-07", clients)
	props := []ChatProposal{{Action: "update", Kind: "task", Fields: ChatProposalFields{TargetID: "x", Priority: "media"}}}
	kept, notice, resolved := guardProposalTargets(question, props, crit, clients, events, nil)
	if len(kept) != 1 || notice != "" || kept[0].Fields.TargetID != "ev-camp" {
		t.Fatalf("typo devia resolver via fuzzy p/ ev-camp: notice=%q resolved=%q", notice, resolved)
	}
}

func TestFuzzyAmbiguousDoesNotForce(t *testing.T) {
	// dois titulos igualmente proximos => nao forca nenhum (0 hits).
	events := []AIContextEvent{
		{ID: "a", Title: "Campanha de verao", Date: "2026-07-01"},
		{ID: "b", Title: "Campanha de verao 2", Date: "2026-07-02"},
	}
	hits := fuzzyTitleMatch("muda a campanha de verau", events)
	if len(hits) != 0 {
		t.Fatalf("ambiguo nao devia forcar: %v", hits)
	}
}

func TestSnapProposalTypes(t *testing.T) {
	// A rede mecanica cobre TYPO (<=2 edicoes) e normalizacao; erro FONETICO ("rios"->"reels",
	// distancia 3) e papel do MODELO (prompt de dominio) — aqui so garantimos que o lixo NUNCA
	// chega ao card (limpa).
	props := []ChatProposal{
		{Action: "update", Kind: "task", Fields: ChatProposalFields{Type: "rels"}},     // typo -> reels
		{Action: "update", Kind: "task", Fields: ChatProposalFields{Type: "Reels"}},    // valido (case)
		{Action: "update", Kind: "task", Fields: ChatProposalFields{Type: "rios"}},     // fonetico: modelo corrige; a rede limpa
		{Action: "update", Kind: "task", Fields: ChatProposalFields{Type: "gravacão"}}, // acento -> gravacao
	}
	snapProposalTypes(props)
	if props[0].Fields.Type != "reels" {
		t.Fatalf("'rels' devia virar reels, veio %q", props[0].Fields.Type)
	}
	if props[1].Fields.Type != "reels" {
		t.Fatalf("'Reels' devia normalizar reels, veio %q", props[1].Fields.Type)
	}
	if props[2].Fields.Type != "" {
		t.Fatalf("'rios' (fonetico) devia ser limpo pela rede, veio %q", props[2].Fields.Type)
	}
	if props[3].Fields.Type != "gravacao" {
		t.Fatalf("acento devia normalizar gravacao, veio %q", props[3].Fields.Type)
	}
}

func TestLevenshtein(t *testing.T) {
	if d := levenshtein("rios", "reels"); d != 3 {
		t.Fatalf("rios~reels = %d (esperava 3)", d)
	}
	if d := levenshtein("testi", "teste"); d != 1 {
		t.Fatalf("testi~teste = %d (esperava 1)", d)
	}
}

func TestResolveClientsUnrecoverableCleared(t *testing.T) {
	// lixo sem pista nenhuma => limpa (nunca manda id inventado pro PATCH).
	clients := []AIContextClientLean{{ID: "cli-perola", Name: "Perola"}, {ID: "cli-bari", Name: "Bari"}}
	props := []ChatProposal{{Action: "update", Kind: "task", Fields: ChatProposalFields{TargetID: "t", ClientID: "zzz-invalido"}}}
	resolveClientsInProposals(props, "edita a tarefa ai", clients)
	if props[0].Fields.ClientID != "" {
		t.Fatalf("clientId irrecuperavel devia ser limpo: %q", props[0].Fields.ClientID)
	}
}

func peopleFixture() []Member {
	return []Member{
		{ID: "u-iasmin", Name: "Iasmin"},
		{ID: "u-tony", Name: "Tony Prado"},
	}
}

func TestResolvePeopleJunkValue(t *testing.T) {
	// modelo mandou "iasmin-id" (lixo) e a pergunta cita "iasmin" => resolve p/ u-iasmin.
	props := []ChatProposal{{Action: "update", Kind: "task", Fields: ChatProposalFields{ResponsibleID: "iasmin-id"}}}
	resolvePeopleInProposals(props, "coloca a iasmin como responsavel", peopleFixture())
	if props[0].Fields.ResponsibleID != "u-iasmin" {
		t.Fatalf("responsibleId lixo devia resolver p/ u-iasmin, veio %q", props[0].Fields.ResponsibleID)
	}
}

func TestResolvePeopleMissingField(t *testing.T) {
	// modelo ESQUECEU o responsibleId, mas a pergunta cita a pessoa + "responsavel" => infere.
	props := []ChatProposal{{Action: "update", Kind: "task", Fields: ChatProposalFields{}}}
	resolvePeopleInProposals(props, "poe a Iasmin como responsavel na tarefa", peopleFixture())
	if props[0].Fields.ResponsibleID != "u-iasmin" {
		t.Fatalf("responsibleId ausente devia ser inferido, veio %q", props[0].Fields.ResponsibleID)
	}
}

func TestResolvePeopleByName(t *testing.T) {
	// modelo mandou o NOME em vez do id.
	props := []ChatProposal{{Action: "update", Kind: "task", Fields: ChatProposalFields{ResponsibleID: "Iasmin"}}}
	resolvePeopleInProposals(props, "edita a tarefa", peopleFixture())
	if props[0].Fields.ResponsibleID != "u-iasmin" {
		t.Fatalf("nome devia resolver p/ id, veio %q", props[0].Fields.ResponsibleID)
	}
}

func TestResolvePeopleKeepsValidID(t *testing.T) {
	// id valido nao e mexido.
	props := []ChatProposal{{Action: "update", Kind: "task", Fields: ChatProposalFields{ResponsibleID: "u-tony"}}}
	resolvePeopleInProposals(props, "coloca a iasmin", peopleFixture())
	if props[0].Fields.ResponsibleID != "u-tony" {
		t.Fatalf("id valido nao devia mudar, veio %q", props[0].Fields.ResponsibleID)
	}
}

func TestGuardResolvesTargetlessUpdateByTitle(t *testing.T) {
	// CASO REAL (roteiro CRUD 2g): "o cliente da Lancamento... e a Bari" veio como update SEM
	// targetId — a sanitizacao nao pode matar antes da guarda; o titulo citado resolve o alvo.
	events, tasks, clients := bariFixture()
	question := "o cliente da postagem bari na verdade e a Bari"
	crit := extractTargetCriteria(question, "2026-07", clients)
	props := []ChatProposal{{Action: "update", Kind: "task", Fields: ChatProposalFields{ClientID: "cli-bari", ClientName: "Bari"}}}
	kept, notice, _ := guardProposalTargets(question, props, crit, clients, events, tasks)
	kept, dropped := dropTargetlessEditable(kept)
	if dropped || len(kept) != 1 || notice != "" {
		t.Fatalf("update sem targetId com titulo citado devia resolver: kept=%d dropped=%v notice=%q", len(kept), dropped, notice)
	}
	if kept[0].Fields.TargetID != "ev-post" {
		t.Fatalf("alvo devia ser ev-post, veio %q", kept[0].Fields.TargetID)
	}
}

func TestDropTargetlessEditable(t *testing.T) {
	// update/delete que TERMINAM sem targetId saem; create e note ficam.
	props := []ChatProposal{
		{Action: "update", Kind: "task", Fields: ChatProposalFields{Priority: "alta"}},
		{Action: "create", Kind: "task", Fields: ChatProposalFields{Title: "Nova"}},
		{Action: "delete", Kind: "event", Fields: ChatProposalFields{}},
		{Action: "update", Kind: "note", Fields: ChatProposalFields{Note: &ChatProposalNote{Content: "x"}}},
	}
	kept, dropped := dropTargetlessEditable(props)
	if !dropped || len(kept) != 2 {
		t.Fatalf("devia manter create+note e descartar os 2 sem alvo: kept=%d dropped=%v", len(kept), dropped)
	}
	if kept[0].Action != "create" || kept[1].Kind != "note" {
		t.Fatalf("mantidas erradas: %+v", kept)
	}
}

func TestSanitizeAllowsTargetlessUpdateThrough(t *testing.T) {
	// A sanitizacao NAO descarta mais update/delete sem targetId (a guarda resolve depois);
	// mas update sem NENHUM campo editavel continua caindo.
	if !sanitizeContentProposal("update", ChatProposalFields{Priority: "alta"}) {
		t.Fatal("update sem targetId mas com campo editavel devia passar (guarda resolve)")
	}
	if !sanitizeContentProposal("delete", ChatProposalFields{}) {
		t.Fatal("delete sem targetId devia passar (guarda resolve)")
	}
	if sanitizeContentProposal("update", ChatProposalFields{}) {
		t.Fatal("update sem campo editavel devia cair")
	}
}

func TestSnapNoteMonths(t *testing.T) {
	// CASO REAL: "reescreve a anotacao do mes para: Planejamento agosto em andamento" gravava
	// em 2026-08 (mes citado no CONTEUDO) — tem que ir pro mes em foco.
	note := &ChatProposalNote{Mode: "replace", Month: "2026-08", Content: "Planejamento agosto em andamento"}
	props := []ChatProposal{{Action: "update", Kind: "note", Fields: ChatProposalFields{Note: note}}}
	snapNoteMonths(props, "2026-07", "reescreve a anotação do mês para somente: Planejamento agosto em andamento")
	if note.Month != "2026-07" {
		t.Fatalf("mes da anotacao devia ser o em foco (2026-07), veio %q", note.Month)
	}
	// mes-ALVO explicito ("mes de agosto") e respeitado.
	note2 := &ChatProposalNote{Mode: "append", Month: "2026-08", Content: "x"}
	props2 := []ChatProposal{{Action: "create", Kind: "note", Fields: ChatProposalFields{Note: note2}}}
	snapNoteMonths(props2, "2026-07", "anota no mês de agosto: x")
	if note2.Month != "2026-08" {
		t.Fatalf("mes-alvo explicito devia ser respeitado, veio %q", note2.Month)
	}
}

func TestSnapProposalStatuses(t *testing.T) {
	// CASO REAL: status "Raw" inventado pelo modelo derrubava o CREATE (400). Lixo e limpo;
	// typo leve ("planejando" nao, mas "publicad" sim) casa o mais proximo.
	props := []ChatProposal{
		{Action: "create", Kind: "task", Fields: ChatProposalFields{Status: "Raw"}},
		{Action: "create", Kind: "task", Fields: ChatProposalFields{Status: "publicad"}},
		{Action: "create", Kind: "task", Fields: ChatProposalFields{Status: "standby"}},
	}
	snapProposalTypes(props)
	if props[0].Fields.Status != "" {
		t.Fatalf("status lixo devia ser limpo, veio %q", props[0].Fields.Status)
	}
	if props[1].Fields.Status != "publicado" {
		t.Fatalf("typo leve devia casar publicado, veio %q", props[1].Fields.Status)
	}
	if props[2].Fields.Status != "standby" {
		t.Fatalf("status valido nao podia mudar, veio %q", props[2].Fields.Status)
	}
}
