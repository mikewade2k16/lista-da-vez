package calendar

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// GUARDA DE ALVO (WAVE 14): quando o usuario ESPECIFICA um dia ("a tarefa do dia 15") ou um
// cliente ("a tarefa da Perola") numa edicao/exclusao, a IA as vezes escolhe o alvo ERRADO
// (uma task de outro dia, ou ate sem data). O card ja mostra o titulo do alvo, mas o dono
// nao deve precisar reconferir a cada vez. Esta guarda e SERVER-SIDE e determinista: extrai o
// criterio (dia/cliente) da pergunta e, para cada proposta update/delete de event/task, confere
// se o alvo escolhido bate. Se NAO bater (ou nao houver alvo unico), a proposta e DESCARTADA e
// um aviso e acrescentado ao answer listando as tarefas/itens REAIS daquele dia para o dono
// escolher — nunca aplica no item errado silenciosamente. Decisao do dono (2026-07-13): barrar
// e pedir para escolher; validar por DATA e por CLIENTE.

// chatTargetCriteria e o criterio extraido da pergunta: dia citado, clientes citados e o ano
// do mes em foco (para o destaque/listas mostrarem o ano quando o item e de OUTRO ano).
type chatTargetCriteria struct {
	day       string // YYYY-MM-DD ou "" (sem dia citado)
	clientIDs map[string]bool
	focusYear string // "2026" (do mes em foco) ou ""
}

// extractTargetCriteria le a pergunta e o contexto para achar o DIA citado (numero de dia,
// data dd/mm, ou "dia N") e o(s) CLIENTE(s) citados por nome. baseMonth (YYYY-MM) resolve o
// mes de um "dia 15" solto. clients = clientes visiveis (id+nome) para casar nome->id.
func extractTargetCriteria(question, baseMonth string, clients []AIContextClientLean) chatTargetCriteria {
	crit := chatTargetCriteria{clientIDs: map[string]bool{}}
	if monthRe.MatchString(strings.TrimSpace(baseMonth)) {
		crit.focusYear = strings.TrimSpace(baseMonth)[:4]
	}
	norm := foldChatLabel(question)

	// DIA: prioridade para data completa dd/mm[/yyyy]; senao "dia N" / "no N".
	if m := chatNumericDateRe.FindStringSubmatch(norm); len(m) >= 3 {
		day, _ := parseNumericDay(m[0], baseMonth)
		crit.day = day
	} else if d := dayNumberFromQuestion(norm, baseMonth); d != "" {
		crit.day = d
	}

	// CLIENTE: nome citado que casa (unico) com um cliente visivel.
	for _, c := range clients {
		name := foldChatLabel(c.Name)
		if name == "" {
			continue
		}
		// palavra inteira: o nome do cliente aparece como token na pergunta.
		if containsWord(norm, name) {
			crit.clientIDs[c.ID] = true
		}
	}
	return crit
}

// dayNumberFromQuestion acha "dia 15" / "no dia 15" / "do dia 15" e monta YYYY-MM-DD com o
// baseMonth. Sem baseMonth valido (YYYY-MM) nao ha como ancorar o dia -> "".
func dayNumberFromQuestion(norm, baseMonth string) string {
	if !monthRe.MatchString(strings.TrimSpace(baseMonth)) {
		return ""
	}
	m := chatDayNumberRe.FindStringSubmatch(norm)
	if len(m) < 2 {
		return ""
	}
	n := atoiSafe(m[1])
	if n < 1 || n > 31 {
		return ""
	}
	return fmt.Sprintf("%s-%02d", baseMonth, n)
}

// parseNumericDay converte "15/07" ou "15/07/2026" em YYYY-MM-DD, usando o ano do baseMonth
// quando o ano nao vem na string.
func parseNumericDay(raw, baseMonth string) (string, bool) {
	parts := strings.FieldsFunc(raw, func(r rune) bool { return r == '/' || r == '-' })
	if len(parts) < 2 {
		return "", false
	}
	day := atoiSafe(parts[0])
	month := atoiSafe(parts[1])
	year := 0
	if len(parts) >= 3 {
		year = atoiSafe(parts[2])
		if year < 100 {
			year += 2000
		}
	} else if monthRe.MatchString(baseMonth) {
		year = atoiSafe(baseMonth[:4])
	}
	if day < 1 || day > 31 || month < 1 || month > 12 || year < 2000 {
		return "", false
	}
	return fmt.Sprintf("%04d-%02d-%02d", year, month, day), true
}

// guardProposalTargets RESOLVE o alvo — o modelo erra demais escolhendo (ate diz "nao ha evento"
// tendo um), entao o BACK decide, determinista, com PRIORIDADE-CALENDARIO. Ordem de resolucao:
//
//  1. TITULO-PRIMEIRO: se a PERGUNTA cita o titulo de um evento do calendario ("na postagem
//     Bari..."), ESSE e o alvo — dia vira so desempate quando o mesmo titulo existe em varios
//     dias. Vence qualquer filtro (caso real: "Postagem Bari" sem cliente + "cliente deve ser
//     bari" fazia o filtro de cliente excluir o proprio alvo e forcar outro item).
//  2. Criterio DIA/CLIENTE: cliente citado como VALOR A ATRIBUIR (fields.clientId/clientName
//     das propostas) NAO filtra — so filtra cliente citado como dono atual. 1 evento batendo =>
//     reescreve o targetId; varios => barra + lista (com as datas); 0 no calendario mas ha task
//     => barra + avisa "so existe em Tasks: X".
//
// Propostas sem alvo a validar (create) e perguntas sem titulo/criterio passam intactas. Devolve
// as propostas mantidas + aviso (vazio se nada barrado) + o titulo resolvido ("Titulo (dd/mm)").
func guardProposalTargets(question string, proposals []ChatProposal, crit chatTargetCriteria, clients []AIContextClientLean, events []AIContextEvent, tasks []AIContextTask) (kept []ChatProposal, notice string, resolvedTitle string) {
	hasEditable := false
	for i := range proposals {
		if isEditableTargetProposal(proposals[i]) {
			hasEditable = true
			break
		}
	}
	if !hasEditable {
		return proposals, "", ""
	}

	// (1) TITULO-PRIMEIRO: o titulo citado na pergunta vence dia/cliente.
	hits := titleMatchedEvents(question, events)
	if len(hits) > 1 && crit.day != "" {
		if filtered := filterEventsByDay(hits, crit.day); len(filtered) > 0 {
			hits = filtered
		}
	}
	switch {
	case len(hits) == 1:
		target := hits[0]
		for i := range proposals {
			if isEditableTargetProposal(proposals[i]) {
				proposals[i].Fields.TargetID = target.ID
			}
		}
		return proposals, "", titleWithDay(target, crit.focusYear)
	case len(hits) > 1:
		return dropEditable(proposals), buildTargetMismatchNotice(crit, hits, ""), ""
	}

	// (2) Criterio dia/cliente. Cliente que e VALOR atribuido nas propostas nao filtra.
	crit = dropAssignedClients(crit, proposals, clients)
	if crit.day == "" && len(crit.clientIDs) == 0 {
		return proposals, "", ""
	}
	matches := calendarMatches(crit, events)
	kept = make([]ChatProposal, 0, len(proposals))
	byID := targetIndex(events, tasks)
	blocked := false
	onlyTaskTitle := ""
	for i := range proposals {
		p := proposals[i]
		if !isEditableTargetProposal(p) {
			kept = append(kept, p)
			continue
		}
		switch len(matches) {
		case 1:
			// 1 evento no criterio: FORCA o alvo (ignora o que a IA escolheu) e mantem os campos.
			p.Fields.TargetID = matches[0].ID
			resolvedTitle = titleWithDay(matches[0], crit.focusYear)
			kept = append(kept, p)
		case 0:
			blocked = true
			if snap, ok := byID[strings.TrimSpace(p.Fields.TargetID)]; ok && !snap.inCalendar && onlyTaskTitle == "" {
				onlyTaskTitle = snap.title
			}
		default:
			blocked = true // varios: pede para escolher (nao aplica nenhum)
		}
	}
	if blocked {
		return kept, buildTargetMismatchNotice(crit, matches, onlyTaskTitle), resolvedTitle
	}
	return kept, "", resolvedTitle
}

// titleMatchedEvents acha os eventos cujo TITULO (frase inteira, sem acento, >= 4 chars) aparece
// na pergunta. Mantem so os matches MAXIMAIS: se "gravacao perolas" casou, o titulo "gravacao"
// (contido nele) sai — evita ambiguidade falsa entre titulos aninhados.
func titleMatchedEvents(question string, events []AIContextEvent) []AIContextEvent {
	norm := foldChatLabel(question)
	hits := make([]AIContextEvent, 0)
	seen := map[string]bool{}
	for _, e := range events {
		title := foldChatLabel(e.Title)
		if len(title) < 4 || seen[e.ID] {
			continue
		}
		if containsWord(norm, title) {
			seen[e.ID] = true
			hits = append(hits, e)
		}
	}
	out := make([]AIContextEvent, 0, len(hits))
	for _, a := range hits {
		fa := foldChatLabel(a.Title)
		maximal := true
		for _, b := range hits {
			fb := foldChatLabel(b.Title)
			if len(fb) > len(fa) && strings.Contains(fb, fa) {
				maximal = false
				break
			}
		}
		if maximal {
			out = append(out, a)
		}
	}
	if len(out) == 0 {
		// WAVE 15: sem match exato, tier FUZZY (typo/erro de transcricao: "multi dia testi").
		// Conservador — so 1 candidato inequivoco; ambiguidade cai no fluxo normal/modelo.
		return fuzzyTitleMatch(question, events)
	}
	return out
}

// filterEventsByDay mantem so os eventos do dia (YYYY-MM-DD).
func filterEventsByDay(events []AIContextEvent, day string) []AIContextEvent {
	out := make([]AIContextEvent, 0, len(events))
	for _, e := range events {
		if e.Date == day {
			out = append(out, e)
		}
	}
	return out
}

// titleWithDay devolve "Titulo (dd/mm)" para o destaque de confirmacao — com o ANO
// ("dd/mm/aaaa") quando o item e de outro ano que o mes em foco (a busca ampla pode
// resolver um item de outro ano; sem o ano o dono seria enganado).
func titleWithDay(e AIContextEvent, focusYear string) string {
	t, err := time.Parse("2006-01-02", e.Date)
	if err != nil {
		return e.Title
	}
	if focusYear != "" && t.Format("2006") != focusYear {
		return e.Title + " (" + t.Format("02/01/2006") + ")"
	}
	return e.Title + " (" + t.Format("02/01") + ")"
}

// dropTargetlessEditable descarta update/delete de event/task que TERMINARAM sem targetId mesmo
// depois da guarda (o modelo nao mandou e nem o titulo/dia da pergunta resolveu). Um PATCH sem
// alvo nao tem como aplicar; melhor um aviso deterministico pedindo o titulo do que um card
// quebrado ou um answer que mente "preparei a proposta". Devolve as mantidas + se descartou.
func dropTargetlessEditable(proposals []ChatProposal) ([]ChatProposal, bool) {
	kept := make([]ChatProposal, 0, len(proposals))
	dropped := false
	for _, p := range proposals {
		if isEditableTargetProposal(p) && strings.TrimSpace(p.Fields.TargetID) == "" {
			dropped = true
			continue
		}
		kept = append(kept, p)
	}
	return kept, dropped
}

// dropEditable devolve so as propostas que NAO sao update/delete de event/task (as barradas saem).
func dropEditable(proposals []ChatProposal) []ChatProposal {
	out := make([]ChatProposal, 0, len(proposals))
	for _, p := range proposals {
		if !isEditableTargetProposal(p) {
			out = append(out, p)
		}
	}
	return out
}

// dropAssignedClients tira do criterio os clientes que as propostas ATRIBUEM (fields.clientId ou
// clientName): "o cliente deve ser Bari" cita Bari como NOVO VALOR, nao como dono atual do item —
// filtrar por ele excluiria o proprio alvo (que ainda esta sem cliente).
func dropAssignedClients(crit chatTargetCriteria, proposals []ChatProposal, clients []AIContextClientLean) chatTargetCriteria {
	if len(crit.clientIDs) == 0 {
		return crit
	}
	nameToID := make(map[string]string, len(clients))
	for _, c := range clients {
		if n := foldChatLabel(c.Name); n != "" {
			nameToID[n] = c.ID
		}
	}
	assigned := map[string]bool{}
	for _, p := range proposals {
		if id := strings.TrimSpace(p.Fields.ClientID); id != "" {
			assigned[id] = true
		}
		if n := foldChatLabel(p.Fields.ClientName); n != "" {
			if id, ok := nameToID[n]; ok {
				assigned[id] = true
			}
		}
	}
	if len(assigned) == 0 {
		return crit
	}
	out := map[string]bool{}
	for id := range crit.clientIDs {
		if !assigned[id] {
			out[id] = true
		}
	}
	crit.clientIDs = out
	return crit
}

// calendarMatches devolve os EVENTOS do calendario que batem o criterio (dia+cliente), dedup por
// id, preservando a ordem. Base da resolucao determinista do alvo (prioridade-calendario).
func calendarMatches(crit chatTargetCriteria, events []AIContextEvent) []AIContextEvent {
	out := make([]AIContextEvent, 0)
	seen := map[string]bool{}
	for _, e := range events {
		if e.ID == "" || seen[e.ID] {
			continue
		}
		if targetMatchesCriteria(targetSnapshot{day: e.Date, clientID: e.ClientID}, crit) {
			seen[e.ID] = true
			out = append(out, e)
		}
	}
	return out
}

// targetSnapshot e a projecao minima do alvo (dia + cliente + se e do calendario) usada pela guarda.
type targetSnapshot struct {
	id         string
	title      string
	day        string
	clientID   string
	inCalendar bool // true = evento do calendario; false = task sem evento (fora do calendario)
}

// targetIndex mapeia id do contexto (evento OU task, e o taskId do evento vinculado) para o
// snapshot. Eventos do calendario => inCalendar=true; task SEM evento vinculado => inCalendar=false.
func targetIndex(events []AIContextEvent, tasks []AIContextTask) map[string]targetSnapshot {
	out := make(map[string]targetSnapshot, len(events)+len(tasks))
	for _, e := range events {
		snap := targetSnapshot{id: e.ID, title: e.Title, day: e.Date, clientID: e.ClientID, inCalendar: true}
		out[e.ID] = snap
		if e.TaskID != "" {
			out[e.TaskID] = snap // a task vinculada aponta para o MESMO evento (esta no calendario)
		}
	}
	for _, t := range tasks {
		if _, exists := out[t.ID]; exists {
			continue // task com evento vinculado ja indexou (inCalendar=true)
		}
		day, _ := taskDueDateParts(t.DueDate)
		out[t.ID] = targetSnapshot{id: t.ID, title: t.Title, day: day, clientID: t.ClientID, inCalendar: false}
	}
	return out
}

// targetMatchesCriteria diz se o alvo bate TODOS os criterios presentes (dia E cliente). Um
// criterio ausente (dia vazio ou sem cliente citado) nao restringe.
func targetMatchesCriteria(snap targetSnapshot, crit chatTargetCriteria) bool {
	if crit.day != "" && snap.day != crit.day {
		return false
	}
	if len(crit.clientIDs) > 0 && !crit.clientIDs[snap.clientID] {
		return false
	}
	return true
}

// isEditableTargetProposal: update/delete de event/task (as unicas que tem targetId a validar).
func isEditableTargetProposal(p ChatProposal) bool {
	if p.Action != "update" && p.Action != "delete" {
		return false
	}
	return p.Kind == "event" || p.Kind == "task"
}

// buildTargetMismatchNotice monta o aviso que prefixa o answer quando um alvo foi barrado. A
// busca prioriza o CALENDARIO: lista os EVENTOS reais do dia (e/ou cliente) para o dono escolher
// (matches ja calculado por calendarMatches). Se nao ha item no calendario mas o alvo pedido
// existe SO em Tasks (onlyTaskTitle), avisa isso e pergunta se quer alterar em Tasks.
func buildTargetMismatchNotice(crit chatTargetCriteria, matches []AIContextEvent, onlyTaskTitle string) string {
	cands := dedupCandidates(matches, crit.focusYear)
	dayLabel := crit.day
	if dayLabel != "" {
		if t, err := time.Parse("2006-01-02", crit.day); err == nil {
			dayLabel = t.Format("02/01")
		}
	}
	var b strings.Builder
	if len(cands) > 0 {
		if dayLabel != "" {
			b.WriteString("No calendario, no dia " + dayLabel + ", ha " + pluralItens(len(cands)) + ". Qual voce quer alterar?")
		} else {
			b.WriteString("No calendario encontrei " + pluralItens(len(cands)) + " que batem. Qual voce quer alterar?")
		}
		for _, c := range cands {
			b.WriteString("\n- " + c.title)
			if c.clientLabel != "" {
				b.WriteString(" (" + c.clientLabel + ")")
			}
			if c.dayLabel != "" {
				b.WriteString(" - " + c.dayLabel)
			}
		}
		return b.String()
	}
	// Nada no calendario. Se o alvo pedido existe so em Tasks, avisa e pergunta.
	if strings.TrimSpace(onlyTaskTitle) != "" {
		if dayLabel != "" {
			b.WriteString("Nao ha esse item no calendario no dia " + dayLabel + ", ")
		} else {
			b.WriteString("Isso nao esta no calendario, ")
		}
		b.WriteString("mas encontrei em Tasks: \"" + onlyTaskTitle + "\". Quer que eu altere la?")
		return b.String()
	}
	if dayLabel != "" {
		b.WriteString("Nao encontrei nenhum item no calendario no dia " + dayLabel + ". Confira a data e me diga qual e o item.")
	} else {
		b.WriteString("Nao encontrei um item no calendario que bata com o que voce pediu. Confira a data/cliente.")
	}
	return b.String()
}

type targetCandidate struct {
	title       string
	clientLabel string
	dayLabel    string
}

// dedupCandidates projeta os eventos que batem o criterio na lista de escolha do aviso, dedup por
// ID (titulos IGUAIS em dias diferentes aparecem todos, com a data para desambiguar; ano quando
// difere do foco) e ordenados. So o calendario (prioridade do dono).
func dedupCandidates(matches []AIContextEvent, focusYear string) []targetCandidate {
	seen := map[string]bool{}
	out := make([]targetCandidate, 0)
	for _, e := range matches {
		if e.ID == "" || seen[e.ID] {
			continue
		}
		seen[e.ID] = true
		day := ""
		if t, err := time.Parse("2006-01-02", e.Date); err == nil {
			if focusYear != "" && t.Format("2006") != focusYear {
				day = t.Format("02/01/2006")
			} else {
				day = t.Format("02/01")
			}
		}
		out = append(out, targetCandidate{title: e.Title, clientLabel: e.ClientName, dayLabel: day})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].title == out[j].title {
			return out[i].dayLabel < out[j].dayLabel
		}
		return out[i].title < out[j].title
	})
	return out
}

func pluralItens(n int) string {
	if n == 1 {
		return "1 item"
	}
	return fmt.Sprintf("%d itens", n)
}

// containsWord diz se needle aparece como palavra inteira em haystack (ambos ja normalizados).
func containsWord(haystack, needle string) bool {
	for _, tok := range strings.Fields(haystack) {
		if tok == needle {
			return true
		}
	}
	// nome com espaco (ex.: "dona evania"): substring com bordas de espaco.
	return strings.Contains(" "+haystack+" ", " "+needle+" ")
}

func atoiSafe(s string) int {
	n := 0
	for _, r := range strings.TrimSpace(s) {
		if r < '0' || r > '9' {
			return n
		}
		n = n*10 + int(r-'0')
	}
	return n
}

// ---------------------------------------------------------------------------
// Fuzzy (WAVE 15): REDE DE SEGURANCA para typos/erros de transcricao — a inteligencia
// principal e a IA (prompt de dominio); isto so garante que um typo leve nao quebre o
// fluxo quando o modelo escorregar. Conservador: 1 candidato claro ou nada.
// ---------------------------------------------------------------------------

// levenshtein e a distancia de edicao classica (runas), sem dependencia externa.
func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	if len(ra) == 0 {
		return len(rb)
	}
	if len(rb) == 0 {
		return len(ra)
	}
	prev := make([]int, len(rb)+1)
	curr := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		curr[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			curr[j] = minInt(prev[j]+1, minInt(curr[j-1]+1, prev[j-1]+cost))
		}
		prev, curr = curr, prev
	}
	return prev[len(rb)]
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// normalizedLev devolve a distancia normalizada (0=igual, 1=nada a ver).
func normalizedLev(a, b string) float64 {
	la, lb := len([]rune(a)), len([]rune(b))
	longest := la
	if lb > longest {
		longest = lb
	}
	if longest == 0 {
		return 0
	}
	return float64(levenshtein(a, b)) / float64(longest)
}

// fuzzyTitleThreshold: distancia normalizada maxima para considerar um titulo "quase igual"
// ("campanha multi dia testi" vs "campanha multi dia teste" ~ 0.04). fuzzyTitleMargin: o melhor
// candidato precisa dessa folga sobre o 2o para ser inequivoco.
const (
	fuzzyTitleThreshold = 0.25
	fuzzyTitleMargin    = 0.10
	fuzzyTitleMinRunes  = 6
)

// fuzzyTitleMatch procura o UNICO evento cujo titulo casa por SIMILARIDADE com algum trecho da
// pergunta (janela deslizante de tokens do tamanho do titulo ±1). Devolve 0 ou 1 eventos —
// ambiguidade nao forca nada (o fluxo normal/modelo decide).
func fuzzyTitleMatch(question string, events []AIContextEvent) []AIContextEvent {
	qTokens := strings.Fields(foldChatLabel(question))
	if len(qTokens) == 0 {
		return nil
	}
	best, second := 1.0, 1.0
	var bestEvent *AIContextEvent
	seen := map[string]bool{}
	for i := range events {
		e := &events[i]
		title := foldChatLabel(e.Title)
		if len([]rune(title)) < fuzzyTitleMinRunes || seen[e.ID] {
			continue
		}
		seen[e.ID] = true
		score := bestWindowScore(qTokens, title)
		switch {
		case score < best:
			second = best
			best = score
			bestEvent = e
		case score < second:
			second = score
		}
	}
	if bestEvent == nil || best > fuzzyTitleThreshold {
		return nil
	}
	if second-best < fuzzyTitleMargin {
		return nil // dois candidatos proximos: ambiguo, nao forca
	}
	return []AIContextEvent{*bestEvent}
}

// bestWindowScore compara o titulo com todas as janelas de tokens da pergunta (tamanho do
// titulo ±1) e devolve a menor distancia normalizada.
func bestWindowScore(qTokens []string, title string) float64 {
	k := len(strings.Fields(title))
	if k == 0 {
		return 1
	}
	best := 1.0
	for _, size := range []int{k - 1, k, k + 1} {
		if size < 1 || size > len(qTokens) {
			continue
		}
		for start := 0; start+size <= len(qTokens); start++ {
			window := strings.Join(qTokens[start:start+size], " ")
			if d := normalizedLev(window, title); d < best {
				best = d
			}
		}
	}
	return best
}

// eventStatusSet: status VALIDOS da taxonomia compartilhada (content-taxonomy.ts; mesma
// lista CONTENT_STATUSES do workflow). Status inventado pelo modelo ("Raw") derrubava o
// CREATE inteiro (400 no POST) — caso real do roteiro 2026-07-14.
var eventStatusSet = map[string]bool{
	"planejado": true, "producao": true, "revisao": true, "aprovada": true, "standby": true, "publicado": true,
}

// snapProposalTypes normaliza Type e Status das propostas contra a taxonomia REAL
// (eventTypeSet/eventStatusSet): valor valido passa; typo/erro de voz proximo (<=2 edicoes,
// ex.: "rios"->"reels") vira o mais parecido; irrecuperavel e LIMPO — valor inventado nunca
// chega ao card/POST. Rede de seguranca: o prompt ja instrui o modelo a corrigir e avisar.
func snapProposalTypes(proposals []ChatProposal) {
	snap := func(raw string, valid map[string]bool) string {
		folded := foldChatLabel(raw)
		if folded == "" || valid[folded] {
			return folded
		}
		best, bestDist := "", 3
		for v := range valid {
			if d := levenshtein(folded, v); d < bestDist {
				bestDist = d
				best = v
			}
		}
		return best // "" quando nada chega perto (limpa o lixo)
	}
	for i := range proposals {
		proposals[i].Fields.Type = snap(proposals[i].Fields.Type, eventTypeSet)
		proposals[i].Fields.Status = snap(proposals[i].Fields.Status, eventStatusSet)
	}
}

// monthNamesPt: nome do mes citado EXPLICITAMENTE como alvo ("anotacao do mes de agosto").
var monthNamesPt = []string{"janeiro", "fevereiro", "marco", "abril", "maio", "junho", "julho", "agosto", "setembro", "outubro", "novembro", "dezembro"}

// snapNoteMonths forca o MES da anotacao para o mes em foco (decisao do dono, 2026-07-14): o
// modelo gravava a nota no mes citado no CONTEUDO ("reescreve para: Planejamento agosto" ia
// para 2026-08). So respeita o month do modelo quando a PERGUNTA nomeia o mes como ALVO
// ("mes de agosto" / "em agosto:") ou traz um YYYY-MM explicito. Muta in-place.
func snapNoteMonths(proposals []ChatProposal, contextMonth, question string) {
	if !monthRe.MatchString(strings.TrimSpace(contextMonth)) {
		return
	}
	// YYYY-MM checa na pergunta CRUA (foldChatLabel troca hifen por espaco e quebraria "2026-08").
	norm := foldChatLabel(question)
	explicit := strings.Contains(strings.ToLower(question), strings.TrimSpace(contextMonth))
	for _, m := range monthNamesPt {
		if strings.Contains(norm, "mes de "+m) || strings.Contains(norm, "em "+m+":") {
			explicit = true
			break
		}
	}
	if explicit {
		return
	}
	for i := range proposals {
		if proposals[i].Kind != "note" || proposals[i].Fields.Note == nil {
			continue
		}
		proposals[i].Fields.Note.Month = strings.TrimSpace(contextMonth)
	}
}

// resolvePeopleInProposals RESOLVE responsibleId/involvedIds por NOME no back — o modelo manda
// valores lixo ("iasmin-id"), inventa ou esquece o campo. Regra: se o valor NAO e um id conhecido
// de `people`, tenta casar por NOME (unico, sem acento). Se nao casar e a PERGUNTA citar uma
// pessoa conhecida (ex.: "coloca a iasmin"), usa essa. `people` = id+nome (mesma fonte do
// GET /responsibles, ja no contexto). Nao mexe em valor que ja e um id valido. Muta in-place.
func resolvePeopleInProposals(proposals []ChatProposal, question string, people []Member) {
	if len(people) == 0 {
		return
	}
	byID := make(map[string]bool, len(people))
	byName := make(map[string]string, len(people)) // nome normalizado -> id (unico)
	dupName := map[string]bool{}
	for _, p := range people {
		byID[p.ID] = true
		n := foldChatLabel(p.Name)
		if n == "" {
			continue
		}
		if _, seen := byName[n]; seen {
			dupName[n] = true
			continue
		}
		byName[n] = p.ID
	}
	fromQuestion := peopleNamedInQuestion(question, people) // ids citados na pergunta (unico por nome)
	resolveOne := func(raw string) string {
		v := strings.TrimSpace(raw)
		if v == "" || byID[v] {
			return v // vazio ou ja e um id valido: nao mexe
		}
		if id, ok := byName[foldChatLabel(v)]; ok && !dupName[foldChatLabel(v)] {
			return id // valor era um NOME conhecido
		}
		if len(fromQuestion) == 1 {
			return fromQuestion[0] // valor lixo, mas a pergunta cita 1 pessoa: usa ela
		}
		return v // sem como resolver com seguranca: deixa como veio (o front tenta)
	}
	for i := range proposals {
		f := &proposals[i].Fields
		f.ResponsibleID = resolveOne(f.ResponsibleID)
		if len(f.InvolvedIDs) > 0 {
			out := make([]string, 0, len(f.InvolvedIDs))
			for _, id := range f.InvolvedIDs {
				out = append(out, resolveOne(id))
			}
			f.InvolvedIDs = out
		}
		// "coloca a iasmin como responsavel" sem responsibleId (modelo esqueceu): usa a pessoa da pergunta.
		if strings.TrimSpace(f.ResponsibleID) == "" && len(fromQuestion) == 1 && mentionsResponsible(question) {
			f.ResponsibleID = fromQuestion[0]
		}
	}
}

// resolveClientsInProposals faz para o CLIENTE o mesmo que resolvePeopleInProposals faz para
// pessoas: clientId que NAO e um id conhecido dos clientes visiveis e resolvido pelo clientName
// da proposta ou pelo cliente citado na pergunta (unico); irrecuperavel => LIMPO (id lixo nunca
// segue para o PATCH). Sempre que resolve, preenche tambem o ClientName (label legivel).
func resolveClientsInProposals(proposals []ChatProposal, question string, clients []AIContextClientLean) {
	if len(clients) == 0 {
		return
	}
	nameByID := make(map[string]string, len(clients))
	idByName := make(map[string]string, len(clients))
	dupName := map[string]bool{}
	for _, c := range clients {
		nameByID[c.ID] = c.Name
		n := foldChatLabel(c.Name)
		if n == "" {
			continue
		}
		if _, seen := idByName[n]; seen {
			dupName[n] = true
			continue
		}
		idByName[n] = c.ID
	}
	norm := foldChatLabel(question)
	fromQuestion := ""
	for _, c := range clients {
		n := foldChatLabel(c.Name)
		if n == "" || dupName[n] || !containsWord(norm, n) {
			continue
		}
		if fromQuestion != "" && fromQuestion != c.ID {
			fromQuestion = "" // mais de um cliente citado: nao da para inferir
			break
		}
		fromQuestion = c.ID
	}
	for i := range proposals {
		f := &proposals[i].Fields
		id := strings.TrimSpace(f.ClientID)
		if id == "" {
			continue // sem clientId proposto: o front/card resolve (nao inventamos)
		}
		if _, ok := nameByID[id]; ok {
			if strings.TrimSpace(f.ClientName) == "" {
				f.ClientName = nameByID[id]
			}
			continue // id valido
		}
		// id desconhecido: tenta o clientName da proposta, senao o cliente citado na pergunta.
		if byName, ok := idByName[foldChatLabel(f.ClientName)]; ok && !dupName[foldChatLabel(f.ClientName)] {
			f.ClientID = byName
			f.ClientName = nameByID[byName]
			continue
		}
		if fromQuestion != "" {
			f.ClientID = fromQuestion
			f.ClientName = nameByID[fromQuestion]
			continue
		}
		f.ClientID = "" // lixo irrecuperavel: melhor sem cliente (o dono escolhe no card)
	}
}

// peopleNamedInQuestion devolve os ids das pessoas citadas por NOME na pergunta (palavra inteira,
// sem acento). Dedup; nomes ambiguos (2 pessoas com o mesmo nome) sao ignorados.
func peopleNamedInQuestion(question string, people []Member) []string {
	norm := foldChatLabel(question)
	counts := map[string]int{}
	firstID := map[string]string{}
	for _, p := range people {
		n := foldChatLabel(p.Name)
		if n == "" {
			continue
		}
		counts[n]++
		if _, ok := firstID[n]; !ok {
			firstID[n] = p.ID
		}
	}
	out := make([]string, 0)
	seen := map[string]bool{}
	for _, p := range people {
		n := foldChatLabel(p.Name)
		if n == "" || counts[n] > 1 || seen[p.ID] {
			continue
		}
		if containsWord(norm, n) {
			seen[p.ID] = true
			out = append(out, p.ID)
		}
	}
	return out
}

var responsibleWords = []string{"responsavel", "responsaveis", "responsable"}

// mentionsResponsible diz se a pergunta fala em "responsavel" (para inferir a atribuicao quando o
// modelo esquece o responsibleId mas cita a pessoa).
func mentionsResponsible(question string) bool {
	norm := foldChatLabel(question)
	for _, w := range responsibleWords {
		if strings.Contains(norm, w) {
			return true
		}
	}
	return false
}
