package calendar

import (
	"context"
	"encoding/json"
	"html"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/tasks"
	platformmodules "github.com/mikewade2k16/lista-da-vez/back/internal/platform/modules"
)

const (
	// taskSourceCalendar = ui_metadata.source da task que NASCEU de um evento (C10). O handler
	// de sync ignora essas tasks ao criar espelho (elas ja tem o evento delas) — anti-loop E3.
	taskSourceCalendar = "calendar"
	// taskSourceMirror = events.source do evento-espelho nascido de uma task (E3). O CreateEvent
	// reverso nunca cria task para eventos assim; o delete/apply so toca eventos source='task'.
	taskSourceMirror = "task"
)

// eventTypeSet sao os tipos VALIDOS de evento do calendario (mesma lista do front,
// calendar.ts: post/story/reels/reuniao/gravacao/evento). O sync task->evento so adota o tipo
// se ele estiver aqui — a task tem vocabulario proprio (ex.: "site") que quebraria a UI do
// calendario (EVENT_TYPE_META[tipo].icon estoura). WAVE 6.
var eventTypeSet = map[string]bool{
	"post": true, "story": true, "reels": true, "reuniao": true, "gravacao": true, "evento": true,
}

// Sync bidirecional calendario<->tasks (WAVE 5, E2/E3/E4/E5). Este arquivo cobre o sentido
// task -> evento (o handler invertido) e os helpers de espelho/status-map. O sentido
// evento -> task (forward, no UpdateEvent) mora em service.go chamando syncTaskFromEvent
// daqui. GUARDA ANTI-LOOP: todo caminho de escrita usa metodos TERMINAIS (store direto /
// ApplyCalendarSync) que NAO re-disparam o outro sentido; o campo events.source e
// ui_metadata.source barram a recriacao do espelho (E3).

// taskSyncHandler implementa platformmodules.RelationSyncHandler: reage a mudancas de task
// para manter o evento-espelho. Resolve o Service do calendar LAZY (imune a ordem de Build).
type taskSyncHandler struct {
	provider func() *Service
}

// NewTaskSyncHandler cria o handler para registrar no RelationSyncRegistry (app.go). provider
// resolve o Service do calendar LAZY: func() *calendar.Service { return calendarModule.Service() }.
func NewTaskSyncHandler(provider func() *Service) platformmodules.RelationSyncHandler {
	return &taskSyncHandler{provider: provider}
}

func (h *taskSyncHandler) ModuleID() string { return relationModule }

// OnTaskChanged mantem o evento-espelho a cada mudanca de task. Best-effort: erro so vira log.
func (h *taskSyncHandler) OnTaskChanged(ctx context.Context, accountID string, snap platformmodules.TaskSyncSnapshot, deleted bool) error {
	if h.provider == nil {
		return nil
	}
	svc := h.provider()
	if svc == nil {
		return nil
	}
	svc.handleTaskSync(ctx, accountID, snap, deleted)
	return nil
}

// handleTaskSync aplica no calendario a mudanca de UMA task (E2/E3). eventID = a relation
// calendar/event do snapshot (vazio = task sem espelho ainda). Best-effort.
func (s *Service) handleTaskSync(ctx context.Context, accountID string, snap platformmodules.TaskSyncSnapshot, deleted bool) {
	account := strings.TrimSpace(accountID)
	eventID := snap.ResourceID(relationModule, relationResourceEvent)

	if deleted {
		// Arquivou a task: remove SO o evento-espelho (source='task'); evento manual fica.
		if eventID != "" {
			s.deleteMirrorEvent(ctx, account, eventID)
		}
		return
	}
	if eventID != "" {
		// Ja tem espelho/vinculo: reflete os campos da task no evento (TERMINAL).
		s.applyTaskSyncToEvent(ctx, account, eventID, snap)
		return
	}
	// Sem espelho ainda: cria um se o espelho estiver ligado, a task for do board mapeado,
	// tiver prazo e NAO tiver nascido do calendario (source='calendar' => ja tem seu evento).
	s.maybeCreateMirrorEvent(ctx, account, snap)
}

// maybeCreateMirrorEvent cria o evento-espelho (source='task') de uma task com prazo e o
// vincula (E3). Guardas: mirror ligado + board == board da config + task com dueDate + task
// nao-nascida do calendario. Best-effort — cada falha vira log e nao propaga.
func (s *Service) maybeCreateMirrorEvent(ctx context.Context, accountID string, snap platformmodules.TaskSyncSnapshot) {
	if strings.EqualFold(strings.TrimSpace(snap.Source), taskSourceCalendar) {
		return // nasceu de um evento; ja tem o dele (anti-loop)
	}
	if snap.DueDate == nil {
		return // sem data nao ha evento
	}
	cfg, err := s.store.GetConfig(ctx, accountID)
	if err != nil {
		return
	}
	board := strings.TrimSpace(cfg.Tasks.BoardID)
	if !cfg.Tasks.MirrorTasks || board == "" || board != strings.TrimSpace(snap.BoardID) {
		return // espelho desligado ou task de outro board
	}

	dueDate, dueTime := mirrorEventDateTime(*snap.DueDate)
	in := EventInput{
		Date:          dueDate,
		Time:          dueTime,
		ClientID:      ptrToStr(snap.ClientAccountID),
		Type:          firstNonEmpty(strings.TrimSpace(cfg.Tasks.DefaultEventType), "post"),
		Title:         strings.TrimSpace(snap.Title),
		Status:        firstNonEmpty(statusForColumn(cfg, ptrToStr(snap.ColumnID)), "planejado"),
		Priority:      "media",
		ResponsibleID: ptrToStr(snap.ResponsibleUserID),
		InvolvedIDs:   []string{},
		Media:         []MediaItem{},
		Source:        taskSourceMirror, // 'task' => guarda anti-loop no CreateEvent reverso
	}
	in, err = validateEvent(accountID, in)
	if err != nil {
		s.logTaskWarn(ctx, "calendar: espelho de task invalido", accountID, snap.TaskID, err)
		return
	}
	ev, err := s.store.CreateEvent(ctx, accountID, in)
	if err != nil {
		s.logTaskWarn(ctx, "calendar: criar evento-espelho falhou", accountID, snap.TaskID, err)
		return
	}
	// Vincula task->evento (mesma relation do C10). Falha no vinculo so loga: o evento existe
	// e, na releitura, o taskId sai do join; sem a relation o proximo sync nao acha o espelho.
	s.linkTaskToEvent(ctx, accountID, snap.TaskID, ev)
	// WAVE 6 (cruzamento B): se a task ja tem videos, espelha em events.linked_media.
	if len(snap.Media) > 0 {
		if lerr := s.store.SetEventLinkedMedia(ctx, ev.ID, accountID, mediaItemsFromSnapshots(snap.Media)); lerr != nil {
			s.logTaskWarn(ctx, "calendar: espelhar midia da task no evento-espelho falhou", accountID, ev.ID, lerr)
		}
	}
	s.publishCalendar(ctx, RealtimeEvent{Type: realtimeEventCreated, AccountID: accountID, ResourceID: ev.ID, Date: ev.Date, Version: ev.Version})
}

// applyTaskSyncToEvent reflete os campos da task no evento vinculado (E4/E5), TERMINAL (usa o
// store direto, sem re-disparar o forward). Espelho (source='task') sem prazo => apaga (a
// task perdeu a data). Evento manual vinculado mantem a data se a task zerar o prazo.
func (s *Service) applyTaskSyncToEvent(ctx context.Context, accountID, eventID string, snap platformmodules.TaskSyncSnapshot) {
	ev, err := s.store.GetEvent(ctx, eventID, accountID)
	if err != nil {
		return // evento sumiu; nada a sincronizar
	}
	if snap.DueDate == nil && ev.Source == taskSourceMirror {
		s.deleteMirrorEvent(ctx, accountID, eventID)
		return
	}
	cfg, err := s.store.GetConfig(ctx, accountID)
	if err != nil {
		cfg = CalendarConfig{}
	}
	in := eventToInput(ev)
	in.Title = firstNonEmpty(strings.TrimSpace(snap.Title), in.Title)
	if snap.DueDate != nil {
		in.Date, in.Time = mirrorEventDateTime(*snap.DueDate)
	}
	in.ClientID = ptrToStr(snap.ClientAccountID)
	in.ResponsibleID = ptrToStr(snap.ResponsibleUserID)
	if p := strings.TrimSpace(snap.Priority); p != "" {
		in.Priority = p // WAVE 6: prioridade espelhada task->evento (mesmo vocabulario)
	}
	if t := strings.TrimSpace(snap.Type); t != "" && eventTypeSet[strings.ToLower(t)] {
		// WAVE 6: tipo espelhado task->evento SO se for um tipo valido de evento. A task tem
		// vocabulario proprio (ex.: "site") que nao existe no calendario e quebraria a UI.
		in.Type = t
	}
	if st := statusForColumn(cfg, ptrToStr(snap.ColumnID)); st != "" {
		in.Status = st // sync de status (task mudou de coluna -> status do evento, E5)
	}
	in, err = validateEvent(accountID, in)
	if err != nil {
		return
	}
	updated, err := s.store.UpdateEvent(ctx, eventID, accountID, in, nil)
	if err != nil {
		s.logTaskWarn(ctx, "calendar: aplicar sync task->evento falhou", accountID, eventID, err)
		return
	}
	// WAVE 6 (cruzamento B): espelha os videos da task em events.linked_media (read-only).
	if lerr := s.store.SetEventLinkedMedia(ctx, eventID, accountID, mediaItemsFromSnapshots(snap.Media)); lerr != nil {
		s.logTaskWarn(ctx, "calendar: espelhar midia da task no evento falhou", accountID, eventID, lerr)
	}
	s.publishCalendar(ctx, RealtimeEvent{Type: realtimeEventUpdated, AccountID: accountID, ResourceID: updated.ID, Date: updated.Date, Version: updated.Version})
}

// deleteMirrorEvent apaga o evento-espelho (source='task') ao arquivar a task e desvincula.
// So toca eventos source='task' (evento manual vinculado a uma task arquivada permanece).
func (s *Service) deleteMirrorEvent(ctx context.Context, accountID, eventID string) {
	ev, err := s.store.GetEvent(ctx, eventID, accountID)
	if err != nil {
		return
	}
	if ev.Source != taskSourceMirror {
		return // evento manual: nao apaga
	}
	if err := s.store.DeleteEvent(ctx, eventID, accountID); err != nil {
		s.logTaskWarn(ctx, "calendar: apagar evento-espelho falhou", accountID, eventID, err)
		return
	}
	s.publishCalendar(ctx, RealtimeEvent{Type: realtimeEventDeleted, AccountID: accountID, ResourceID: eventID, Date: ev.Date})
}

// syncTaskFromEvent reflete a edicao do evento na task vinculada (E4/E5) — sentido forward,
// chamado pelo UpdateEvent. TERMINAL do lado tasks (ApplyCalendarSync nao re-dispara o
// handler). taskID vazio = evento sem task (no-op). Best-effort.
func (s *Service) syncTaskFromEvent(ctx context.Context, accountID string, ev CalendarEvent, taskID string, syncContent bool) {
	svc := s.tasksSvc()
	if svc == nil || strings.TrimSpace(taskID) == "" {
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		return
	}
	access, err := svc.ResolveAccessContext(ctx, principal, accountID)
	if err != nil {
		s.logTaskWarn(ctx, "calendar: acesso a tasks no forward sync falhou", accountID, ev.ID, err)
		return
	}
	title := strings.TrimSpace(ev.Title)
	due := eventDueDate(ev.Date, ev.Time)
	client := nonEmptyPtr(normalizeUUID(ptrToStr(ev.ClientID)))
	resp := nonEmptyPtr(normalizeUUID(ev.ResponsibleID))
	priority := strings.TrimSpace(ev.Priority) // WAVE 6: prioridade espelhada evento->task
	input := tasks.UpdateTaskInput{
		ID:                strings.TrimSpace(taskID),
		Title:             &title,
		Priority:          &priority,
		DueDate:           &due,
		ClientAccountID:   &client,
		ResponsibleUserID: &resp,
	}
	// Descricao do evento -> corpo (ContentHTML) da task (WAVE 6.1). SO quando a descricao
	// MUDOU (syncContent): evitar sobrescrever o texto rico da task numa edicao de outro campo
	// do evento (ex.: mudar status). A descricao do calendario e texto simples; vira <p> no editor.
	if syncContent {
		body := descToHTML(ev.Description)
		input.ContentHTML = &body
	}
	// WAVE 6: campos que a task guarda em ui_metadata (o repo faz merge, nao apaga o resto).
	// tipo -> ui_metadata.type; e o NOME do responsavel resolvido do id -> ui_metadata.responsible
	// (a task exibe esse nome cacheado; antes so o id sincronizava e o nome ficava velho).
	md := map[string]any{}
	if t := strings.TrimSpace(ev.Type); t != "" {
		md["type"] = t
	}
	if rid := normalizeUUID(ev.ResponsibleID); rid != "" {
		md["responsible"] = s.store.ResolveUserLabel(ctx, rid)
	} else {
		md["responsible"] = ""
	}
	// WAVE 6 (cruzamento A): espelha a midia do evento na task (read-only, ui_metadata.calendarMedia).
	// A task nao guarda imagem (so video); aqui e' DISPLAY, entao imagem+video do evento cruzam.
	// Sem duplicar arquivo (a task exibe a mesma URL /uploads/calendar/{conta}/, servida global).
	md["calendarMedia"] = s.eventMediaForTask(ctx, accountID, ev)
	if len(md) > 0 {
		input.UIMetadata = &md
	}
	// status do evento -> coluna da task (E5), se mapeado. Move pro topo da coluna destino.
	if cfg, cerr := s.store.GetConfig(ctx, accountID); cerr == nil {
		if col := columnForStatus(cfg, ev.Status); col != "" {
			colPtr := &col
			input.ColumnID = &colPtr // UpdateTaskInput.ColumnID e **string (present/null/absent)
			top := topSortOrder()
			input.SortOrder = &top
		}
	}
	if _, err := svc.ApplyCalendarSync(ctx, access, input); err != nil {
		s.logTaskWarn(ctx, "calendar: forward sync evento->task falhou", accountID, ev.ID, err)
	}
}

// descToHTML converte a descricao (texto SIMPLES do calendario) no ContentHTML do editor da
// task: escapa HTML e transforma quebras de linha em <br>, num paragrafo. Vazio => "" (limpa o
// corpo). Assim o texto digitado no calendario aparece no editor rico da task sem injetar HTML.
func descToHTML(desc string) string {
	d := strings.TrimSpace(desc)
	if d == "" {
		return ""
	}
	return "<p>" + strings.ReplaceAll(html.EscapeString(d), "\n", "<br>") + "</p>"
}

// syncEventMediaToTask espelha SO a midia do evento na task (ui_metadata.calendarMedia), sem
// tocar titulo/status/etc. Usado no gatilho de day_media (taggear anexo a um evento nao deve
// reverter os campos da task). Terminal (ApplyCalendarSync nao re-dispara). Best-effort.
func (s *Service) syncEventMediaToTask(ctx context.Context, accountID string, ev CalendarEvent, taskID string) {
	svc := s.tasksSvc()
	if svc == nil || strings.TrimSpace(taskID) == "" {
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		return
	}
	access, err := svc.ResolveAccessContext(ctx, principal, accountID)
	if err != nil {
		return
	}
	md := map[string]any{"calendarMedia": s.eventMediaForTask(ctx, accountID, ev)}
	if _, err := svc.ApplyCalendarSync(ctx, access, tasks.UpdateTaskInput{
		ID:         strings.TrimSpace(taskID),
		UIMetadata: &md,
	}); err != nil {
		s.logTaskWarn(ctx, "calendar: espelhar midia do dia na task falhou", accountID, ev.ID, err)
	}
}

// linkTaskToEvent cria a relation task->evento (mesma do C10) para o espelho nascido de task.
func (s *Service) linkTaskToEvent(ctx context.Context, accountID, taskID string, e CalendarEvent) {
	svc := s.tasksSvc()
	if svc == nil {
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		return
	}
	access, err := svc.ResolveAccessContext(ctx, principal, accountID)
	if err != nil {
		s.logTaskWarn(ctx, "calendar: acesso a tasks no vinculo do espelho falhou", accountID, e.ID, err)
		return
	}
	if _, err := svc.AddRelation(ctx, access, tasks.AddRelationInput{
		TaskID:       strings.TrimSpace(taskID),
		Module:       relationModule,
		ResourceType: relationResourceEvent,
		ResourceID:   e.ID,
		LabelCache:   calendarRelationLabel(e.Date, e.Title),
	}); err != nil {
		s.logTaskWarn(ctx, "calendar: vincular espelho ao evento falhou", accountID, e.ID, err)
	}
}

// statusForColumn devolve o status de evento mapeado para uma coluna (E5), ou "" se nao ha
// mapeamento (task mudou de coluna -> qual status o evento ganha).
func statusForColumn(cfg CalendarConfig, columnID string) string {
	columnID = strings.TrimSpace(columnID)
	if columnID == "" {
		return ""
	}
	for _, e := range cfg.Tasks.StatusColumnMap {
		if strings.EqualFold(strings.TrimSpace(e.ColumnID), columnID) {
			return strings.TrimSpace(e.EventStatus)
		}
	}
	return ""
}

// columnForStatus devolve a coluna mapeada para um status de evento (E5), ou "" se nao ha
// mapeamento (evento mudou de status -> pra qual coluna a task vai).
func columnForStatus(cfg CalendarConfig, status string) string {
	status = strings.TrimSpace(status)
	if status == "" {
		return ""
	}
	for _, e := range cfg.Tasks.StatusColumnMap {
		if strings.EqualFold(strings.TrimSpace(e.EventStatus), status) {
			return strings.TrimSpace(e.ColumnID)
		}
	}
	return ""
}

// eventToInput reconstroi o EventInput (full-replace) a partir de um evento ja salvo, para o
// sync terminal preservar os campos que a task nao toca (tipo/prioridade/descricao/midia/
// envolvidos) e sobrescrever so o necessario. involved/media saem do jsonb.
func eventToInput(e CalendarEvent) EventInput {
	in := EventInput{
		Date:          e.Date,
		Time:          e.Time,
		ClientID:      ptrToStr(e.ClientID),
		Type:          e.Type,
		Title:         e.Title,
		Status:        e.Status,
		Priority:      e.Priority,
		ResponsibleID: e.ResponsibleID,
		Description:   e.Description,
		InvolvedIDs:   []string{},
		Media:         []MediaItem{},
	}
	_ = json.Unmarshal(normalizeArray(e.InvolvedIDs), &in.InvolvedIDs)
	_ = json.Unmarshal(normalizeArray(e.Media), &in.Media)
	return in
}

// mediaItemsFromSnapshots converte as MediaSnapshot neutras do TaskSyncSnapshot em MediaItem
// (WAVE 6 cruzamento B), para gravar em events.linked_media (midia da task espelhada no evento,
// read-only). Descarta item sem url. url interna /uploads/tasks/ (nao passa por normalizeMedia — e
// coluna de exibicao propria, populada server-side a partir dos videos da propria task/conta).
func mediaItemsFromSnapshots(snaps []platformmodules.MediaSnapshot) []MediaItem {
	out := make([]MediaItem, 0, len(snaps))
	for _, m := range snaps {
		url := strings.TrimSpace(m.URL)
		if url == "" {
			continue
		}
		t := strings.ToLower(strings.TrimSpace(m.Type))
		if t != "image" {
			t = "video"
		}
		out = append(out, MediaItem{
			ID:          strings.TrimSpace(m.ID),
			URL:         url,
			Name:        strings.TrimSpace(m.Name),
			Type:        t,
			ContentType: strings.TrimSpace(m.ContentType),
			SizeBytes:   m.SizeBytes,
			PosterURL:   strings.TrimSpace(m.PosterURL),
		})
	}
	return out
}

// eventMediaForTask coleta a midia que a task vinculada deve EXIBIR (read-only, cruzamento A):
// a midia do proprio evento (ev.Media) + os anexos do dia apontados a este evento (day_media com
// eventId == ev.ID). Devolve []map[string]any no shape que o front da task le em
// ui_metadata.calendarMedia (id/url/name/type/contentType/sizeBytes/posterUrl). Dedup por id.
// Best-effort: falha ao ler day_media apenas ignora os anexos do dia.
func (s *Service) eventMediaForTask(ctx context.Context, accountID string, ev CalendarEvent) []map[string]any {
	var items []MediaItem
	_ = json.Unmarshal(normalizeArray(ev.Media), &items)
	if days, err := s.store.ListDayMedia(ctx, accountID, ev.Date, ev.Date); err == nil {
		id := strings.TrimSpace(ev.ID)
		for _, d := range days {
			for _, m := range d.Media {
				if strings.EqualFold(strings.TrimSpace(m.EventID), id) {
					items = append(items, m)
				}
			}
		}
	}
	out := make([]map[string]any, 0, len(items))
	seen := map[string]bool{}
	for _, m := range items {
		mediaID := strings.TrimSpace(m.ID)
		url := strings.TrimSpace(m.URL)
		if url == "" || seen[mediaID] {
			continue
		}
		seen[mediaID] = true
		out = append(out, map[string]any{
			"id":          mediaID,
			"url":         url,
			"name":        m.Name,
			"type":        m.Type,
			"contentType": m.ContentType,
			"sizeBytes":   m.SizeBytes,
			"posterUrl":   m.PosterURL,
		})
	}
	return out
}

// topSortOrder devolve um sort_order que coloca a task no TOPO da coluna (WAVE 5, item 2): a
// listagem ordena por sort_order asc, entao "topo" = valor bem abaixo dos existentes. Usa o
// tempo (segundos, negativo) para a mais recente ficar acima da anterior sem consultar o banco.
func topSortOrder() float64 {
	return -float64(time.Now().Unix())
}
