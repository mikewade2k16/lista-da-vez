package calendar

import (
	"context"
	"strings"
	"time"
	"unicode"
)

// Resolucao server-side dos ALVOS das propostas de CRUD do chat (WAVE 12). Dois problemas
// reais que isso fecha: (1) a IA as vezes manda o NOME/titulo do item em targetId em vez do
// UUID — aqui cruzamos com o contexto AUTORITATIVO (events/tasks enviados a ela) e reescrevemos
// para o id real; (2) o card do front precisa SEMPRE mostrar o titulo e o "antes" do item que
// sera alterado — para task pura (sem evento vinculado) nao havia snapshot nenhum na mensagem.
// A solucao anexa o alvo de cada update/delete aos calendarItems persistidos da mensagem
// (task vira um AIContextEvent sintetizado com TaskID=ID); o front ja resolve titulo/antes
// pelos calendarItems e ja esconde esses itens da secao "Calendario" (filtro por targetId).

// Busca ampla por titulo (WAVE 14, decisao do dono 2026-07-14): "procurar primeiro no mes
// alvo; nao encontrando nele, ir em QUALQUER mes/ano". Caso real: a tela estava em 2025 e a
// "Postagem Bari" e de 07/2026 — o contexto (mes em foco) nao tinha o evento e a IA dizia
// "nao encontrei". Se a pergunta cita o titulo de um evento que NAO esta no mes em foco,
// buscamos numa janela ampla (±24 meses) com as MESMAS queries scoped e ANEXAMOS os matches
// ao contexto ANTES do LLM — o modelo passa a ver o evento e o fluxo normal (proposta +
// guarda titulo-primeiro) segue.
const (
	wideSearchLimit   = 1000
	wideSearchMaxHits = 8
)

func (s *Service) appendWideTitleMatches(ctx context.Context, accountID string, access ChatAccess, mode, clientID, month, question string, block any) any {
	if len(titleMatchedEvents(question, contextEvents(block))) > 0 {
		return block // achou no mes alvo: nada a fazer
	}
	from, to := wideSearchWindow(month)
	var wide []AIContextEvent
	var err error
	if mode == chatScopeAll {
		wide, err = s.store.ListEventsLeanForClients(ctx, accountID, from, to,
			capClientIDs(access.VisibleClientIDs, maxContextClients), wideSearchLimit)
	} else {
		wide, err = s.store.ListEventsLean(ctx, accountID, from, to, clientID, wideSearchLimit)
	}
	if err != nil {
		s.logTaskWarn(ctx, "calendar: busca ampla por titulo falhou", accountID, "", err)
		return block
	}
	hits := titleMatchedEvents(question, wide)
	if len(hits) == 0 {
		return block
	}
	if len(hits) > wideSearchMaxHits {
		hits = hits[:wideSearchMaxHits]
	}
	names := map[string]string{}
	for _, c := range contextClients(block) {
		names[c.ID] = c.Name
	}
	setContextClientNames(hits, names)
	switch b := block.(type) {
	case calendarChatContext:
		b.Events = mergeCalendarItems(b.Events, hits)
		return b
	case AIContextAll:
		b.Events = mergeCalendarItems(b.Events, hits)
		return b
	}
	return block
}

// appendNamedClientProfile hidrata o perfil COMPLETO do cliente CITADO na pergunta no escopo
// 'all' (WAVE 16, decisao do dono 2026-07-15). No escopo 'all' os clientes vao ENXUTOS
// (nome/segmento/tom + campos vazios) para nao estourar o contexto com muitos clientes — entao
// "traz os dados do cliente X" fazia a IA dizer "nao temos" (o dado EXISTE no banco, so nao
// viajava). Aqui, quando a pergunta nomeia UM cliente visivel, buscamos o perfil completo dele
// (GetClientProfile, scoped por account) e o anexamos como ctx.client — o workflow ja renderiza
// "Cliente em foco" com todos os campos, SEM mudar o escopo nem o workflow. Conservador: so o
// cliente inequivocamente citado (1). No escopo 'client' o perfil completo ja viaja: no-op.
func (s *Service) appendNamedClientProfile(ctx context.Context, accountID, question string, block any) any {
	all, ok := block.(AIContextAll)
	if !ok || all.Client != nil {
		return block // so 'all'; se ja tem cliente em foco, idempotente
	}
	target := singleNamedClient(question, all.Clients)
	if target == nil {
		return block // ninguem citado, ou ambiguo (2+): mantem leve
	}
	prof, found, err := s.store.GetClientProfile(ctx, strings.TrimSpace(accountID), target.ID)
	if err != nil {
		s.logTaskWarn(ctx, "calendar: hidratar perfil do cliente citado falhou", accountID, target.ID, err)
		return block
	}
	pc := planClient{ID: target.ID, Name: target.Name}
	if found {
		pc.Profile = planProfileFromClientProfile(prof)
	}
	all.Client = &pc
	return all
}

// singleNamedClient devolve o UNICO cliente visivel citado por NOME na pergunta (palavra inteira,
// sem acento; nome >= 3 chars p/ evitar ruido). nil se nenhum ou mais de um (ambiguo => nao hidrata).
func singleNamedClient(question string, clients []AIContextClientLean) *AIContextClientLean {
	norm := foldChatLabel(question)
	var found *AIContextClientLean
	for i := range clients {
		name := foldChatLabel(clients[i].Name)
		if len([]rune(name)) < 3 || !containsWord(norm, name) {
			continue
		}
		if found != nil && found.ID != clients[i].ID {
			return nil // dois clientes citados: ambiguo, nao arrisca
		}
		found = &clients[i]
	}
	return found
}

// planProfileFromClientProfile converte o ClientProfile (store) no planProfile do contexto da IA
// (mesmos campos; o planClient ja carrega id/nome). Insumo do "Cliente em foco" hidratado.
func planProfileFromClientProfile(p ClientProfile) planProfile {
	return planProfile{
		Segment:     p.Segment,
		Positioning: p.Positioning,
		Description: p.Description,
		History:     p.History,
		SiteURL:     p.SiteURL,
		Instagram:   p.Instagram,
		Address:     p.Address,
		Objectives:  p.Objectives,
		BrandVoice:  p.BrandVoice,
		Extra:       p.Extra,
	}
}

// wideSearchWindow devolve a janela ampla (±24 meses) ancorada no mes em foco.
func wideSearchWindow(month string) (string, string) {
	anchor, err := time.Parse("2006-01", strings.TrimSpace(month))
	if err != nil {
		anchor = time.Now()
	}
	return anchor.AddDate(-2, 0, 0).Format("2006-01-02"), anchor.AddDate(2, 0, 0).Format("2006-01-02")
}

// chatPeopleContext devolve as pessoas da equipe (id+nome) para o contexto da IA —
// MESMA fonte do GET /responsibles (subconjunto configurado ou todos os membros).
// Falha vira lista vazia (o chat continua funcionando, so sem resolver nomes).
func (s *Service) chatPeopleContext(ctx context.Context, accountID string) []Member {
	people, err := s.ListResponsibles(ctx, strings.TrimSpace(accountID))
	if err != nil {
		s.logTaskWarn(ctx, "calendar: listar pessoas para contexto do chat falhou", accountID, "", err)
		return nil
	}
	return people
}

// contextTasks extrai as tasks do bloco de contexto (espelho de contextEvents).
func contextTasks(block any) []AIContextTask {
	switch value := block.(type) {
	case calendarChatContext:
		return value.Tasks
	case AIContextAll:
		return value.Tasks
	default:
		return nil
	}
}

// contextClients extrai a lista lean de clientes (id+nome) do bloco de contexto para a guarda de
// alvo casar cliente citado por nome. No escopo 'all' vem de Clients; no 'client' ha um cliente
// unico em foco (Client) — projeta esse para o mesmo shape.
func contextClients(block any) []AIContextClientLean {
	switch value := block.(type) {
	case AIContextAll:
		return value.Clients
	case calendarChatContext:
		if value.Client != nil {
			return []AIContextClientLean{{ID: value.Client.ID, Name: value.Client.Name}}
		}
	}
	return nil
}

// contextPeople extrai a lista de pessoas da equipe (id+nome) do bloco de contexto — mesma fonte
// que a IA recebe (People), usada para resolver responsavel/envolvidos por NOME no back.
func contextPeople(block any) []Member {
	switch value := block.(type) {
	case AIContextAll:
		return value.People
	case calendarChatContext:
		return value.People
	}
	return nil
}

var chatFoldReplacer = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a",
	"é", "e", "ê", "e", "è", "e", "ë", "e",
	"í", "i", "î", "i", "ì", "i", "ï", "i",
	"ó", "o", "ô", "o", "ò", "o", "õ", "o", "ö", "o",
	"ú", "u", "û", "u", "ù", "u", "ü", "u", "ç", "c",
	// hifen/underscore viram espaco: o titulo "Campanha multi-dia teste" tem que casar com
	// a fala "campanha multi dia teste" (caso real do titulo-primeiro que nao pegou).
	"-", " ", "_", " ",
)

// foldChatLabel normaliza um rotulo para comparacao por nome/titulo (minusculo, sem
// acento pt-BR, hifens como espaco, espacos colapsados) — espelho do normalizePersonLabel do front.
func foldChatLabel(s string) string {
	folded := chatFoldReplacer.Replace(strings.ToLower(strings.TrimSpace(s)))
	// Pontuacao vira espaco: perguntas reais tem "?", ",", "." e ":" grudados no nome/titulo
	// ("objetivos da Perola?" / "da Perola:" tem que casar "Perola"). PRESERVA "/" e ":" SO
	// quando estao ENTRE DIGITOS (separador de data/hora: "15/07", "14:30" seguem lidos do
	// texto foldado; "perola:" perde o dois-pontos). So letra/digito/espaco/[/:] sobrevivem.
	runes := []rune(folded)
	out := make([]rune, len(runes))
	for i, r := range runes {
		switch {
		case r == ' ' || unicode.IsLetter(r) || unicode.IsDigit(r):
			out[i] = r
		case (r == '/' || r == ':') && i > 0 && i < len(runes)-1 &&
			unicode.IsDigit(runes[i-1]) && unicode.IsDigit(runes[i+1]):
			out[i] = r
		default:
			out[i] = ' '
		}
	}
	return strings.Join(strings.Fields(string(out)), " ")
}

// chatLabelsMatch diz se dois rotulos casam (iguais ou um contido no outro), ja normalizados.
func chatLabelsMatch(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return a == b || strings.Contains(a, b) || strings.Contains(b, a)
}

// resolveProposalTargets reescreve targetId por titulo quando a IA nao mandou um id real
// (match UNICO entre eventos+tasks do contexto) e devolve os snapshots dos alvos de
// update/delete para anexar aos calendarItems da mensagem. Muta proposals in-place.
func resolveProposalTargets(proposals []ChatProposal, events []AIContextEvent, tasks []AIContextTask) []AIContextEvent {
	eventsByID := make(map[string]*AIContextEvent, len(events))
	eventsByTaskID := make(map[string]*AIContextEvent, len(events))
	for i := range events {
		eventsByID[events[i].ID] = &events[i]
		if events[i].TaskID != "" {
			eventsByTaskID[events[i].TaskID] = &events[i]
		}
	}
	tasksByID := make(map[string]*AIContextTask, len(tasks))
	for i := range tasks {
		tasksByID[tasks[i].ID] = &tasks[i]
	}
	snapshots := make([]AIContextEvent, 0, len(proposals))
	seen := map[string]bool{}
	appendSnapshot := func(item AIContextEvent) {
		if item.ID == "" || seen[item.ID] {
			return
		}
		seen[item.ID] = true
		snapshots = append(snapshots, item)
	}
	for i := range proposals {
		p := &proposals[i]
		if p.Action != "update" && p.Action != "delete" {
			continue
		}
		if p.Kind != "event" && p.Kind != "task" {
			continue
		}
		target := strings.TrimSpace(p.Fields.TargetID)
		if target == "" {
			continue
		}
		if ev := eventsByID[target]; ev != nil {
			appendSnapshot(*ev)
			continue
		}
		if ev := eventsByTaskID[target]; ev != nil {
			appendSnapshot(*ev)
			continue
		}
		if tk := tasksByID[target]; tk != nil {
			appendSnapshot(aiContextEventFromTask(*tk))
			continue
		}
		// targetId nao e id do contexto: tenta resolver como TITULO (match unico).
		if ev, tk := matchTargetByTitle(target, events, tasks); ev != nil {
			p.Fields.TargetID = ev.ID
			appendSnapshot(*ev)
		} else if tk != nil {
			p.Fields.TargetID = tk.ID
			appendSnapshot(aiContextEventFromTask(*tk))
		}
	}
	return snapshots
}

// matchTargetByTitle procura o UNICO evento ou task cujo titulo casa com o rotulo
// (igual/contido, sem acento). Ambiguo ou nenhum => (nil, nil) — o front ainda tem a
// propria rede de seguranca por titulo na execucao.
func matchTargetByTitle(label string, events []AIContextEvent, tasks []AIContextTask) (*AIContextEvent, *AIContextTask) {
	needle := foldChatLabel(label)
	if needle == "" {
		return nil, nil
	}
	var ev *AIContextEvent
	var tk *AIContextTask
	count := 0
	for i := range events {
		if chatLabelsMatch(foldChatLabel(events[i].Title), needle) {
			ev = &events[i]
			count++
		}
	}
	for i := range tasks {
		if chatLabelsMatch(foldChatLabel(tasks[i].Title), needle) {
			tk = &tasks[i]
			count++
		}
	}
	if count != 1 {
		return nil, nil
	}
	if ev != nil {
		return ev, nil
	}
	return nil, tk
}

// mergeCalendarItems une os itens escolhidos pela IA (eventIds) com os snapshots dos
// alvos das propostas, sem duplicar por id e preservando a ordem.
func mergeCalendarItems(items, extra []AIContextEvent) []AIContextEvent {
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		seen[item.ID] = true
	}
	for _, item := range extra {
		if item.ID == "" || seen[item.ID] {
			continue
		}
		seen[item.ID] = true
		items = append(items, item)
	}
	return items
}

// aiContextEventFromTask sintetiza o snapshot de card de uma TASK no shape dos
// calendarItems (AIContextEvent). TaskID=ID marca a origem; data/hora derivam do
// dueDate com a MESMA heuristica de fuso do espelho task->evento (mirrorEventDateTime).
func aiContextEventFromTask(t AIContextTask) AIContextEvent {
	date, timeOfDay := taskDueDateParts(t.DueDate)
	return AIContextEvent{
		ID:            t.ID,
		Date:          date,
		Time:          timeOfDay,
		Type:          t.Type,
		Title:         t.Title,
		Status:        t.Status,
		Priority:      t.Priority,
		ResponsibleID: t.ResponsibleID,
		InvolvedIDs:   t.InvolvedIDs,
		ClientID:      t.ClientID,
		ClientName:    t.ClientName,
		Description:   t.Description,
		TaskID:        t.ID,
		Media:         []MediaItem{},
	}
}

// taskDueDateParts converte o dueDate string da task (RFC3339 ou data pura) em
// (data, hora) para exibicao, reusando mirrorEventDateTime (meia-noite UTC = sem hora).
func taskDueDateParts(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return mirrorEventDateTime(parsed)
	}
	if len(raw) >= 10 && dateRe.MatchString(raw[:10]) {
		return raw[:10], ""
	}
	return "", ""
}
