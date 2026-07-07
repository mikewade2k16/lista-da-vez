package calendar

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/tasks"
)

// Integracao calendario<->tasks (contrato C10). O calendar consome o Service do modulo
// tasks via um provider LAZY (a ordem de Build no Registry nao importa: a closure resolve
// no primeiro uso). Tudo aqui e best-effort a partir do evento ja salvo: a criacao/vinculo
// da task NUNCA desfaz o evento (falha vira aviso no 201 ou log no delete).

// TasksServiceProvider resolve o Service do modulo tasks de forma LAZY. Devolver nil =
// tasks indisponivel (integracao desligada).
type TasksServiceProvider func() *tasks.Service

// WithTasksService injeta o provider LAZY do modulo tasks no Module (contrato C10).
// Uso no app.go: calendar.New(storage, calendar.WithTasksService(func() *tasks.Service {
// return tasksModule.Service() })).
func WithTasksService(provider TasksServiceProvider) Option {
	return func(m *Module) { m.tasksProvider = provider }
}

// WithTasks encadeia o provider de tasks no Service (chamado no Build, no mesmo estilo de
// WithAI/WithChat). provider nil = integracao desligada.
func (s *Service) WithTasks(provider TasksServiceProvider) *Service {
	s.tasksProvider = provider
	return s
}

// tasksSvc resolve o Service do modulo tasks (nil quando a integracao esta desligada ou o
// provider ainda nao tem service pronto).
func (s *Service) tasksSvc() *tasks.Service {
	if s.tasksProvider == nil {
		return nil
	}
	return s.tasksProvider()
}

// createLinkedTask cria a task no board da config C6 e a vincula ao evento (contrato C10).
// Best-effort: qualquer falha vira aviso (warning) — o evento ja foi salvo pelo caller e
// NAO e desfeito. Devolve o taskId criado (vazio se nao criou) e o warning (vazio no
// sucesso).
//
// cfg ja foi lido e validado (tasks.boardId nao-vazio) pelo CreateEvent; principal vem do
// context (nunca do body). ResolveAccessContext amarra o acesso a ESTA account (task de
// outra conta jamais e lida/escrita) e aplica as permissoes de tasks do usuario.
func (s *Service) createLinkedTask(ctx context.Context, accountID string, cfg CalendarConfig, e CalendarEvent) (string, string) {
	svc := s.tasksSvc()
	if svc == nil {
		return "", "Integracao com tasks indisponivel no momento."
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		return "", "Nao foi possivel identificar o usuario para criar a task."
	}
	access, err := svc.ResolveAccessContext(ctx, principal, accountID)
	if err != nil {
		s.logTaskWarn(ctx, "calendar: resolver acesso a tasks falhou", accountID, e.ID, err)
		return "", "Sem acesso ao modulo tasks nesta conta."
	}

	input := tasks.CreateTaskInput{
		BoardID:           cfg.Tasks.BoardID,
		ColumnID:          s.resolveTaskColumn(ctx, svc, access, cfg, e.Status),
		Title:             e.Title,
		DueDate:           eventDueDate(e.Date, e.Time),
		ResponsibleUserID: nonEmptyPtr(normalizeUUID(e.ResponsibleID)),
		ClientAccountID:   e.ClientID,
		// WAVE 5 (item 2): task nova nasce no TOPO da coluna (sort_order asc => menor = topo).
		SortOrder: topSortOrder(),
		// source=calendar marca a procedencia (C10): guarda anti-loop do espelho (E3) — o
		// handler de sync nao cria outro evento para esta task. O vinculo reverso fica na relation.
		UIMetadata: map[string]any{"source": taskSourceCalendar},
	}
	task, err := svc.CreateTask(ctx, access, input)
	if err != nil {
		s.logTaskWarn(ctx, "calendar: criar task vinculada falhou", accountID, e.ID, err)
		return "", "Nao foi possivel criar a task no board configurado."
	}

	_, err = svc.AddRelation(ctx, access, tasks.AddRelationInput{
		TaskID:       task.ID,
		Module:       relationModule,
		ResourceType: relationResourceEvent,
		ResourceID:   e.ID,
		LabelCache:   calendarRelationLabel(e.Date, e.Title),
	})
	if err != nil {
		s.logTaskWarn(ctx, "calendar: vincular task ao evento falhou", accountID, e.ID, err)
		// A task existe; so o vinculo falhou. Reporta o taskId + aviso (na releitura o
		// badge some porque o EventView le o taskId da relation).
		return task.ID, "Task criada, mas o vinculo com o evento falhou."
	}
	return task.ID, ""
}

// CreateTaskForEvent cria (e vincula) uma task para um evento que AINDA nao tem task — o botao
// "Criar task" do badge "evento sem task" (WAVE 6). Idempotente: se o evento ja tem task, devolve
// o taskId existente. Sem board configurado => ErrTasksNotConfigured (400). accountID = dono do
// calendario (do Principal, nunca do body). Publica update para o front reler (o badge some sozinho).
func (s *Service) CreateTaskForEvent(ctx context.Context, accountID, eventID string) (string, error) {
	account := strings.TrimSpace(accountID)
	ev, err := s.store.GetEvent(ctx, strings.TrimSpace(eventID), account)
	if err != nil {
		return "", mapNotFound(err)
	}
	if ev.TaskID != nil && strings.TrimSpace(*ev.TaskID) != "" {
		return strings.TrimSpace(*ev.TaskID), nil // ja tem task
	}
	cfg, err := s.store.GetConfig(ctx, account)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(cfg.Tasks.BoardID) == "" {
		return "", ErrTasksNotConfigured
	}
	taskID, warn := s.createLinkedTask(ctx, account, cfg, ev)
	if taskID == "" {
		return "", errors.New(warn)
	}
	s.publishCalendar(ctx, RealtimeEvent{Type: realtimeEventUpdated, AccountID: account, ResourceID: ev.ID, Date: ev.Date, Version: ev.Version})
	return taskID, nil
}

// resolveTaskColumn devolve a coluna da task (WAVE 5, E5): 1o a coluna mapeada para o status
// do evento (statusColumnMap), senao o defaultColumnId da config, senao a 1a coluna do board
// (best-effort via GetBoard); nil se nao der para resolver (a task nasce sem coluna). GetBoard
// exige tasks.boards.view — se faltar, cai em nil sem derrubar a criacao da task.
func (s *Service) resolveTaskColumn(ctx context.Context, svc *tasks.Service, access tasks.AccessContext, cfg CalendarConfig, status string) *string {
	if col := columnForStatus(cfg, status); col != "" {
		return &col
	}
	if col := strings.TrimSpace(cfg.Tasks.DefaultColumnID); col != "" {
		return &col
	}
	board, err := svc.GetBoard(ctx, access, cfg.Tasks.BoardID)
	if err != nil || len(board.Columns) == 0 {
		return nil
	}
	first := board.Columns[0].ID
	return &first
}

// unlinkTask remove a relation calendario->task ao apagar o evento (contrato C10; a task
// NAO e arquivada). Best-effort: erros so viram log (o caller ja segue com a exclusao).
func (s *Service) unlinkTask(ctx context.Context, accountID, taskID, eventID string) {
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
		s.logTaskWarn(ctx, "calendar: resolver acesso a tasks no unlink falhou", accountID, eventID, err)
		return
	}
	if err := svc.RemoveRelation(ctx, access, taskID, relationModule, relationResourceEvent, eventID); err != nil {
		s.logTaskWarn(ctx, "calendar: remover vinculo da task falhou", accountID, eventID, err)
	}
}

// archiveLinkedTask ARQUIVA a task vinculada ao apagar o evento (WAVE 6, politica "excluir os dois"
// do modal). Best-effort: falha so vira log. O archive dispara o sync (deleted=true) que ja apaga o
// evento-espelho source='task'; para eventos manuais o caller apaga o evento em seguida.
func (s *Service) archiveLinkedTask(ctx context.Context, accountID, taskID, eventID string) {
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
		s.logTaskWarn(ctx, "calendar: resolver acesso a tasks no archive falhou", accountID, eventID, err)
		return
	}
	if err := svc.ArchiveTask(ctx, access, taskID); err != nil {
		s.logTaskWarn(ctx, "calendar: arquivar task vinculada falhou", accountID, eventID, err)
	}
}

// logTaskWarn registra uma degradacao recuperavel da integracao com tasks (nunca derruba o
// fluxo do evento). No-op quando nao ha logger.
func (s *Service) logTaskWarn(ctx context.Context, msg, accountID, eventID string, err error) {
	if s.logger == nil {
		return
	}
	s.logger.WarnContext(ctx, msg, "account_id", accountID, "event_id", eventID, "error", err.Error())
}

// saoPauloLoc e o fuso do produto (America/Sao_Paulo). Vem do tzdata embutido (time/tzdata
// no main); se por algum motivo nao carregar, cai no offset fixo UTC-3 (Brasil sem horario
// de verao desde 2019), mantendo a hora correta hoje.
var saoPauloLoc = loadSaoPaulo()

func loadSaoPaulo() *time.Location {
	if loc, err := time.LoadLocation("America/Sao_Paulo"); err == nil {
		return loc
	}
	return time.FixedZone("America/Sao_Paulo", -3*60*60)
}

// eventDueDate combina a data (YYYY-MM-DD) e a hora do evento num due date (contrato C10;
// hora vazia = 09:00), interpretados no fuso America/Sao_Paulo. Aceita HH:MM e HH:MM:SS.
// Data/hora invalida => nil (task sem due date).
func eventDueDate(date, tm string) *time.Time {
	date = strings.TrimSpace(date)
	tm = strings.TrimSpace(tm)
	if tm == "" {
		tm = "09:00"
	}
	layout := "2006-01-02 15:04"
	if strings.Count(tm, ":") == 2 {
		layout = "2006-01-02 15:04:05"
	}
	parsed, err := time.ParseInLocation(layout, date+" "+tm, saoPauloLoc)
	if err != nil {
		return nil
	}
	return &parsed
}

// mirrorEventDateTime deriva a (data, hora) do EVENTO a partir do dueDate da task (sentido
// task->evento). CORRIGE o off-by-one de fuso: uma task com data-only (sem hora) nasce meia-noite
// UTC; converter cru para Sao Paulo (-3h) rolava para o DIA ANTERIOR (ex.: 14/07 00:00 UTC ->
// 13/07 21:00). Heuristica do dominio: meia-noite UTC = "sem hora" => usa a DATA em UTC e deixa a
// hora vazia (dia inteiro). Task com hora real (ex.: 12:00 UTC = 09:00 SP) converte para Sao Paulo.
func mirrorEventDateTime(due time.Time) (string, string) {
	u := due.UTC()
	if u.Hour() == 0 && u.Minute() == 0 && u.Second() == 0 {
		return u.Format("2006-01-02"), ""
	}
	sp := due.In(saoPauloLoc)
	return sp.Format("2006-01-02"), sp.Format("15:04")
}

// nonEmptyPtr devolve *string (nil quando vazio) para os campos opcionais uuid do tasks.
func nonEmptyPtr(v string) *string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return &v
}
