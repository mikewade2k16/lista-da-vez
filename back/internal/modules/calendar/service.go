package calendar

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Erros de dominio. Fora-do-escopo e nao-encontrado colapsam em ErrNotFound
// (404) para nao vazar existencia de recurso de outra account.
var (
	ErrNotFound          = errors.New("calendar: not found")
	ErrForbidden         = errors.New("calendar: forbidden")
	ErrInvalidDate       = errors.New("calendar: invalid date")
	ErrInvalidTitle      = errors.New("calendar: invalid title")
	ErrInvalidMedia      = errors.New("calendar: invalid media")
	ErrMediaTooLarge     = errors.New("calendar: media too large")
	ErrMediaUnavailable  = errors.New("calendar: media unavailable")
	ErrInvalidMediaRange = errors.New("calendar: invalid media range")
	ErrInvalidClient     = errors.New("calendar: invalid client")
	ErrAINotConfigured   = errors.New("calendar: ai not configured")
	ErrPlanConflict      = errors.New("calendar: plan conflict")
	ErrInvalidStatus     = errors.New("calendar: invalid status")
	// ErrTasksNotConfigured: pediu createTask mas a config C6 nao tem tasks.boardId
	// (integracao desligada). 400 tasks_not_configured — NAO cria evento orfao.
	ErrTasksNotConfigured = errors.New("calendar: tasks integration not configured")
	// ErrVersionConflict: PUT com If-Match divergente da version atual do evento
	// (contrato C12). 409 version_conflict — o front avisa e oferece recarregar.
	ErrVersionConflict = errors.New("calendar: version conflict")
	// ErrInvalidProvider: PUT de secret com provider fora de {gemini,glm,openai}
	// (contrato SEC). 400 invalid_provider.
	ErrInvalidProvider = errors.New("calendar: invalid provider")
	// ErrAIDisabled: dispatch de IA (chat/plano/transcricao) com o kill switch
	// desligado (config ai.enabled=false, contrato PAY). 409 ai_disabled — o Go NEM
	// chama o provider; o front avisa para ligar a IA na aba IA.
	ErrAIDisabled = errors.New("calendar: ai disabled")
	// ErrAIKeyMissing: provider de IA ativo sem chave gravada (nem na conta nem
	// global; resolveAIKey devolveu "", contrato PAY). 409 ai_key_missing — o front
	// avisa para configurar a chave do provider na aba IA (ou pedir as globais ao admin).
	ErrAIKeyMissing = errors.New("calendar: ai key missing")
)

var (
	dateRe  = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	monthRe = regexp.MustCompile(`^\d{4}-\d{2}$`)
	uuidRe  = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	hexRe   = regexp.MustCompile(`^#[0-9a-f]{6}$`)
)

// aiProviders sao os providers de IA aceitos na config (contratos C2/C6/v4). Fora do
// conjunto => cai no default "claude" na sanitizacao. "gemini" entrou no C6 (camada
// OpenAI-compatible do Google AI Studio, free tier); "openai" entrou na Wave 3 (CFG v4).
var aiProviders = map[string]bool{
	"claude": true, "deepseek": true, "qwen": true, "kimi": true, "glm": true,
	"gemini": true, "openai": true, "custom": true,
}

// transcribeProviders sao os providers de transcricao de voz aceitos (CFG v4). Fora do
// conjunto => cai no default "gemini". "local" = Whisper self-hosted (sem key; aceita o
// audio do navegador via ffmpeg interno); "openai" = Whisper hospedado; "gemini" nao
// transcreve o audio webm do navegador (fica so como opcao de config).
var transcribeProviders = map[string]bool{"openai": true, "gemini": true, "local": true}

// chatPositions sao as posicoes validas da janela de chat (CFG v4). Fora => "center".
var chatPositions = map[string]bool{"center": true, "left": true, "right": true, "fullscreen": true}

// calendarStore e a fatia da persistencia que o Service consome.
type calendarStore interface {
	ResolveCalendarScope(ctx context.Context, activeAccountID string) (CalendarScope, error)
	ListEvents(ctx context.Context, accountID string, f EventFilter) ([]EventView, error)
	// ListEventsLean projeta eventos reais do mes para o agregado das IAs, incluindo
	// identidade, horario e midia, com teto de linhas (sem N+1).
	ListEventsLean(ctx context.Context, accountID, from, to, clientID string, limit int) ([]AIContextEvent, error)
	// ListEventsLeanForClients projeta os eventos do mes SO dos clientes visiveis (WAVE 4,
	// modo 'all'): client_id = ANY(visiveis) OR NULL. Barra vazamento de evento de cliente
	// fora do escopo do usuario no contexto agregado da IA.
	ListEventsLeanForClients(ctx context.Context, accountID, from, to string, clientIDs []string, limit int) ([]AIContextEvent, error)
	GetEvent(ctx context.Context, id, accountID, clientScopeID string) (CalendarEvent, error)
	CreateEvent(ctx context.Context, accountID string, in EventInput) (CalendarEvent, error)
	// UpdateEvent incrementa version; expectedVersion nao-nil aplica o guard de
	// optimistic locking (C12) e retorna pgx.ErrNoRows quando a version diverge.
	UpdateEvent(ctx context.Context, id, accountID, clientScopeID string, in EventInput, expectedVersion *int) (CalendarEvent, error)
	SetEventLinkedMedia(ctx context.Context, id, accountID string, media []MediaItem) error
	DeleteEvent(ctx context.Context, id, accountID, clientScopeID string) error
	GetNotes(ctx context.Context, accountID, month string) (NoteView, error)
	PutNotes(ctx context.Context, accountID, month, content, updatedBy string) (NoteView, error)
	GetConfig(ctx context.Context, accountID string) (CalendarConfig, error)
	PutConfig(ctx context.Context, accountID string, cfg CalendarConfig) (CalendarConfig, error)
	ListMembers(ctx context.Context, accountID string) ([]Member, error)
	ResolveUserLabel(ctx context.Context, userID string) string
	GetMediaLimits(ctx context.Context) (MediaLimits, error)
	PutMediaLimits(ctx context.Context, limits MediaLimits, updatedBy string) error
	// Perfil estrategico do cliente (Fase 4) — ver profile.go / store_profile.go.
	profileStore
	// Planos de IA (Fase 6) — ver ai_plans.go / store_ai_plans.go.
	aiPlanStore
	// Secrets de IA (Wave 3, SEC) — ver secrets.go / store_secrets.go.
	secretStore
	// Override de IA por cliente (Wave 3.1, SEC+) — ver client_ai.go / store_client_ai.go.
	clientAIStore
	// Chat com memoria + acesso org-aware (Wave 4, D1/D2) — ver chat_store.go / chat_access.go.
	chatConversationStore
}

// Service implementa as regras do modulo calendar. A account ativa vem do Principal;
// quando ela e um cliente, ResolveCalendarScope localiza a agenda da conta-agencia da
// mesma organization e trava toda operacao de evento no client_id da account ativa.
type Service struct {
	store   calendarStore
	storage MediaStorage
	ai      AIDispatchConfig
	chat    chatConfig // webhooks do chat/transcricao de IA (C7/C8); ver chat.go
	// tasksProvider resolve o Service do modulo tasks de forma LAZY (contrato C10;
	// ver task_link.go). nil ou provider que devolve nil = integracao desligada.
	tasksProvider TasksServiceProvider
	// publisher entrega os eventos lean ao canal realtime do calendario (contrato C11;
	// ver publisher.go). Default noopPublisher; app.go injeta o realtimeService.
	publisher Publisher
	// clientScope resolve os clientes visiveis ao principal (contrato D2, Wave 4),
	// reusando a lista permission-scoped de /v1/tenants. Injetado via WithClientScope no
	// Build; nil = sem clientes visiveis (o select do chat fica fechado). Ver chat_access.go.
	clientScope clientScopeLister
	// contentOperationsProvider fornece ao Crow apenas um Brief pronto e somente leitura.
	// O modulo de alertas roda independentemente; o chat nunca dispara nem recalcula regras.
	contentOperationsProvider ContentOperationsBriefProvider
	// assistantRuntimeProvider resolve a configuracao/credencial canonica do
	// Omni Chat somente quando uma request compartilhada informa surface.
	assistantRuntimeProvider AssistantRuntimeProvider
	// assistantExecutionCapabilityProvider repete a leitura da matriz canonica na
	// mesma transacao que confirma cards locais, fechando revogacoes entre /ask e PATCH.
	assistantExecutionCapabilityProvider AssistantExecutionCapabilityProvider
	// metaAssistantContextProvider injeta contexto Meta Ads somente leitura sem
	// dependencia direta entre os modulos.
	metaAssistantContextProvider MetaAssistantContextProvider
	// metaAssistantActionProvider cria a proposta duravel antes de o card ser
	// persistido; o status provider revalida succeeded antes do PATCH accepted.
	metaAssistantActionProvider             MetaAssistantActionProvider
	metaAssistantActionStatusProvider       MetaAssistantActionStatusProvider
	metaAssistantActionBindProvider         MetaAssistantActionBindProvider
	metaAssistantActionConfirmProvider      MetaAssistantActionConfirmProvider
	metaAssistantActionCancelProvider       MetaAssistantActionCancelProvider
	metaAssistantConversationCancelProvider MetaAssistantConversationCancelProvider
	// assistantModuleAvailability repete o gate de core.account_modules para
	// cada capability, inclusive no bypass de RBAC de owner/platform_admin.
	assistantModuleAvailability AssistantModuleAvailabilityProvider
	logger                      *slog.Logger
}

// NewService cria o Service. storage pode ser nil quando o modulo roda sem upload.
// publisher nasce no-op (sem realtime) ate WithPublisher injetar o transporte.
func NewService(store calendarStore, storage MediaStorage) *Service {
	return &Service{store: store, storage: storage, publisher: noopPublisher{}}
}

// WithPublisher injeta o transporte realtime (contrato C11); encadeavel no Build.
// provider nil e ignorado (mantem o no-op). Ver publisher.go.
func (s *Service) WithPublisher(p Publisher) *Service {
	if p != nil {
		s.publisher = p
	}
	return s
}

// publishCalendar entrega um evento lean ao canal realtime (no-op quando sem publisher).
func (s *Service) publishCalendar(ctx context.Context, evt RealtimeEvent) {
	if s.publisher == nil {
		return
	}
	s.publisher.PublishCalendarEvent(ctx, evt)
}

// ListEvents devolve os eventos da account na janela pedida.
func (s *Service) ListEvents(ctx context.Context, accountID string, f EventFilter) ([]EventView, error) {
	f.ClientID = normalizeUUID(f.ClientID)
	return s.store.ListEvents(ctx, strings.TrimSpace(accountID), f)
}

// GetEvent devolve um evento dentro do escopo da account.
func (s *Service) GetEvent(ctx context.Context, accountID, id string) (EventView, error) {
	e, err := s.store.GetEvent(ctx, id, strings.TrimSpace(accountID), "")
	if err != nil {
		return EventView{}, mapNotFound(err)
	}
	return e.view(), nil
}

// CreateEvent cria um evento na account. Quando in.CreateTask (contrato C10), tambem
// cria/vincula uma task no board da config C6: a config precisa ter tasks.boardId
// (senao ErrTasksNotConfigured, ANTES de gravar — nao cria evento orfao). A criacao
// da task e best-effort: se falhar DEPOIS do evento salvo, o evento permanece e o erro
// vira aviso em TaskWarning no 201 (nunca desfaz o evento).
func (s *Service) CreateEvent(ctx context.Context, accountID string, in EventInput) (EventView, error) {
	account := strings.TrimSpace(accountID)
	in, err := validateEvent(account, in)
	if err != nil {
		return EventView{}, err
	}
	// Item especial de midia (WAVE 11): o client pede via mediaItem=true e o SERVER seta o
	// source (nunca aceita 'task' do body — anti-loop do espelho preservado).
	if in.IsMediaItem && in.Source == "" {
		in.Source = "media"
	}
	var cfg CalendarConfig
	if in.CreateTask {
		// Pre-condicao da integracao: sem boardId => 400 tasks_not_configured (a config
		// e lida UMA vez e reusada em createLinkedTask, evitando segunda ida ao banco).
		cfg, err = s.store.GetConfig(ctx, account)
		if err != nil {
			return EventView{}, err
		}
		if strings.TrimSpace(cfg.Tasks.BoardID) == "" {
			return EventView{}, ErrTasksNotConfigured
		}
	}
	e, err := s.store.CreateEvent(ctx, account, in)
	if err != nil {
		return EventView{}, err
	}
	view := e.view()
	if in.CreateTask {
		taskID, warning := s.createLinkedTask(ctx, account, cfg, e)
		view.TaskID = taskID
		view.TaskWarning = warning
	}
	s.publishCalendar(ctx, RealtimeEvent{
		Type: realtimeEventCreated, AccountID: account, ClientIDs: []string{eventClientID(e)}, ResourceID: e.ID, Date: e.Date, Version: e.Version,
	})
	return view, nil
}

// UpdateEvent substitui os campos do evento no escopo da account e incrementa a
// version (contrato C12). expectedVersion nao-nil (If-Match) aplica o optimistic
// locking: se a version divergir, o guard nao casa nenhuma linha (pgx.ErrNoRows) e um
// GET escopado desambigua 404 (nao existe/fora do escopo) de 409 (version divergente).
func (s *Service) UpdateEvent(ctx context.Context, accountID, id string, in EventInput, expectedVersion *int) (EventView, error) {
	return s.updateEvent(ctx, accountID, id, "", in, expectedVersion)
}

func (s *Service) updateEvent(ctx context.Context, accountID, id, clientScopeID string, in EventInput, expectedVersion *int) (EventView, error) {
	account := strings.TrimSpace(accountID)
	in, err := validateEvent(account, in)
	if err != nil {
		return EventView{}, err
	}
	// Descricao antiga + taskId ANTES do update: usados no forward sync para (a) achar a task
	// vinculada e (b) so empurrar a descricao pro corpo da task quando ELA mudou (evita apagar
	// o texto rico da task numa edicao de outro campo do evento). So consulta com tasks ativo.
	oldDesc, taskID, oldClientID := "", "", ""
	if old, gerr := s.store.GetEvent(ctx, id, account, clientScopeID); gerr == nil {
		oldClientID = eventClientID(old)
		if s.tasksSvc() != nil {
			oldDesc = old.Description
			if old.TaskID != nil {
				taskID = strings.TrimSpace(*old.TaskID)
			}
		}
	}
	e, err := s.store.UpdateEvent(ctx, id, account, clientScopeID, in, expectedVersion)
	if err != nil {
		if expectedVersion != nil && errors.Is(err, pgx.ErrNoRows) {
			// Guard nao casou: desambigua 409 (existe, version divergente) de 404 (nao
			// existe/fora do escopo). Um erro transitorio do GET (conexao/ctx) NAO pode
			// virar 404 silencioso — propaga cru para mapear em 500.
			_, getErr := s.store.GetEvent(ctx, id, account, clientScopeID)
			switch {
			case getErr == nil:
				return EventView{}, ErrVersionConflict
			case errors.Is(getErr, pgx.ErrNoRows), errors.Is(getErr, ErrNotFound):
				return EventView{}, ErrNotFound
			default:
				return EventView{}, getErr
			}
		}
		return EventView{}, mapNotFound(err)
	}
	s.publishCalendar(ctx, RealtimeEvent{
		Type: realtimeEventUpdated, AccountID: account, ClientIDs: []string{oldClientID, eventClientID(e)}, ResourceID: e.ID, Date: e.Date, Version: e.Version,
	})
	// WAVE 5 (E4/E5): reflete a edicao do evento na task vinculada (forward sync). O
	// UpdateEvent basico nao traz o taskId; le via join so quando a integracao tasks esta
	// ativa. Terminal do lado tasks (ApplyCalendarSync nao volta pro calendar) — sem loop.
	if s.tasksSvc() != nil && taskID != "" {
		s.syncTaskFromEvent(ctx, account, e, taskID,
			strings.TrimSpace(oldDesc) != strings.TrimSpace(e.Description))
	}
	return e.view(), nil
}

// DeleteEvent remove um evento no escopo da account. Se o evento tem task vinculada (contrato C10):
// archiveTask=false (padrao) remove SO a relation (a task fica); archiveTask=true ARQUIVA a task
// tambem (WAVE 6, politica "excluir os dois", escolhida no modal do front). Best-effort no lado da
// task: falha nao impede a exclusao do evento.
func (s *Service) DeleteEvent(ctx context.Context, accountID, id string, archiveTask bool) error {
	return s.deleteEvent(ctx, accountID, id, "", archiveTask)
}

func (s *Service) deleteEvent(ctx context.Context, accountID, id, clientScopeID string, archiveTask bool) error {
	account := strings.TrimSpace(accountID)
	// Le o evento (com taskId via join) so para desvincular/arquivar e capturar a data do payload
	// realtime; ausencia/erro aqui nao bloqueia a exclusao (o DeleteEvent abaixo aplica o escopo).
	var date, clientID string
	if ev, err := s.store.GetEvent(ctx, id, account, clientScopeID); err == nil {
		date = ev.Date
		clientID = eventClientID(ev)
		if ev.TaskID != nil && strings.TrimSpace(*ev.TaskID) != "" {
			taskID := strings.TrimSpace(*ev.TaskID)
			if archiveTask {
				s.archiveLinkedTask(ctx, account, taskID, ev.ID)
			} else {
				s.unlinkTask(ctx, account, taskID, ev.ID)
			}
		}
	}
	if err := s.store.DeleteEvent(ctx, id, account, clientScopeID); err != nil {
		// Com archiveTask, arquivar a task ja pode ter apagado o evento-espelho (source='task')
		// pelo sync; nesse caso o "nao encontrado" e SUCESSO (o evento sumiu, que era o objetivo).
		if archiveTask && (errors.Is(err, pgx.ErrNoRows) || errors.Is(err, ErrNotFound)) {
			s.publishCalendar(ctx, RealtimeEvent{Type: realtimeEventDeleted, AccountID: account, ClientIDs: []string{clientID}, ResourceID: strings.TrimSpace(id), Date: date})
			return nil
		}
		return mapNotFound(err)
	}
	s.publishCalendar(ctx, RealtimeEvent{
		Type: realtimeEventDeleted, AccountID: account, ClientIDs: []string{clientID}, ResourceID: strings.TrimSpace(id), Date: date,
	})
	return nil
}

// GetNotes devolve a nota do mes ('YYYY-MM').
func (s *Service) GetNotes(ctx context.Context, accountID, month string) (NoteView, error) {
	month = strings.TrimSpace(month)
	if !monthRe.MatchString(month) {
		return NoteView{}, ErrInvalidDate
	}
	return s.store.GetNotes(ctx, strings.TrimSpace(accountID), month)
}

// PutNotes faz upsert da nota do mes.
func (s *Service) PutNotes(ctx context.Context, accountID, month, content, updatedBy string) (NoteView, error) {
	account := strings.TrimSpace(accountID)
	month = strings.TrimSpace(month)
	if !monthRe.MatchString(month) {
		return NoteView{}, ErrInvalidDate
	}
	note, err := s.store.PutNotes(ctx, account, month, content, strings.TrimSpace(updatedBy))
	if err != nil {
		return NoteView{}, err
	}
	s.publishCalendar(ctx, RealtimeEvent{Type: realtimeNoteUpdated, AccountID: account, MonthKey: month})
	return note, nil
}

// ============================================================================
// Config + responsaveis (Fase 2)
// ============================================================================

// GetConfig devolve a config da account (defaults se nao existe).
func (s *Service) GetConfig(ctx context.Context, accountID string) (CalendarConfig, error) {
	return s.store.GetConfig(ctx, strings.TrimSpace(accountID))
}

// PutConfig salva a config da account. Sanitiza TODO o shape C2/C6: ids de responsavel
// para UUID (dedup), weekStartsOn no enum, cores validadas (#rrggbb ou "none"),
// provider de IA no enum, temperature no intervalo 0..1, integracao de tasks
// (boardId/defaultColumnId UUID-ou-vazio) e strings trim. Continua full-replace
// (o store persiste o objeto inteiro em jsonb).
func (s *Service) PutConfig(ctx context.Context, accountID string, cfg CalendarConfig) (CalendarConfig, error) {
	cfg.ResponsibleUserIDs = normalizeResponsibles(cfg.ResponsibleUserIDs)
	cfg.WeekStartsOn = normalizeWeekStart(cfg.WeekStartsOn)
	cfg.ClientColors = sanitizeClientColors(cfg.ClientColors)
	cfg.TypeColors = sanitizeTypeColors(cfg.TypeColors)
	cfg.WhiteLabel = sanitizeWhiteLabel(cfg.WhiteLabel)
	cfg.AI = sanitizeAI(cfg.AI)
	cfg.Tasks = sanitizeTasks(cfg.Tasks)
	cfg.Chat = sanitizeChat(cfg.Chat)
	cfg.Shortcuts = sanitizeShortcuts(cfg.Shortcuts)
	account := strings.TrimSpace(accountID)
	saved, err := s.store.PutConfig(ctx, account, cfg)
	if err != nil {
		return CalendarConfig{}, err
	}
	s.publishCalendar(ctx, RealtimeEvent{Type: realtimeConfigUpdated, AccountID: account})
	return saved, nil
}

// normalizeResponsibles filtra para UUID valido e remove duplicados (ordem preservada).
func normalizeResponsibles(raw []string) []string {
	ids := make([]string, 0, len(raw))
	seen := map[string]bool{}
	for _, r := range raw {
		id := normalizeUUID(r)
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

// normalizeWeekStart aceita apenas sunday|monday; qualquer outro valor cai em sunday.
func normalizeWeekStart(v string) string {
	if strings.ToLower(strings.TrimSpace(v)) == "monday" {
		return "monday"
	}
	return "sunday"
}

// specialShortcutKeys sao os nomes de tecla-base nao-imprimiveis aceitos no mapa de atalhos.
var specialShortcutKeys = map[string]bool{
	"enter": true, "escape": true, "space": true,
	"arrowleft": true, "arrowright": true, "arrowup": true, "arrowdown": true,
}

// shortcutModifierOrder = ordem canonica dos modificadores num combo (front grava igual).
var shortcutModifierOrder = []string{"ctrl", "alt", "shift", "meta"}

// validShortcutBase: tecla-base = 1 caractere a-z/0-9 OU um nome especial.
func validShortcutBase(base string) bool {
	if specialShortcutKeys[base] {
		return true
	}
	return len(base) == 1 && (base[0] >= 'a' && base[0] <= 'z' || base[0] >= '0' && base[0] <= '9')
}

// canonicalShortcut valida e normaliza UM combo ('shift+t', 'ctrl+shift+k', 't'): modificadores
// (subconjunto de ctrl/alt/shift/meta, sem repeticao) reordenados na ordem canonica + a base.
// ok=false => invalido (o chamador cai no default). Vazio ja foi tratado antes (desligado).
func canonicalShortcut(value string) (string, bool) {
	parts := strings.Split(value, "+")
	base := parts[len(parts)-1]
	if !validShortcutBase(base) {
		return "", false
	}
	seen := map[string]bool{}
	for _, mod := range parts[:len(parts)-1] {
		if !isShortcutModifier(mod) || seen[mod] {
			return "", false
		}
		seen[mod] = true
	}
	ordered := make([]string, 0, len(seen)+1)
	for _, mod := range shortcutModifierOrder {
		if seen[mod] {
			ordered = append(ordered, mod)
		}
	}
	ordered = append(ordered, base)
	return strings.Join(ordered, "+"), true
}

func isShortcutModifier(v string) bool {
	for _, mod := range shortcutModifierOrder {
		if v == mod {
			return true
		}
	}
	return false
}

// sanitizeShortcuts valida o mapa de atalhos (WAVE 11): so ACOES conhecidas (whitelist de
// shortcutDefaults); cada valor e um COMBO (modificadores + tecla-base) validado/canonicalizado
// por canonicalShortcut; valor invalido cai no default da acao; vazio explicito = atalho
// DESLIGADO (preservado). Acao ausente = default.
func sanitizeShortcuts(raw map[string]string) map[string]string {
	defaults := shortcutDefaults()
	out := make(map[string]string, len(defaults))
	for action, def := range defaults {
		value, present := raw[action]
		if !present {
			out[action] = def
			continue
		}
		key := strings.ToLower(strings.TrimSpace(value))
		if key == "" {
			out[action] = "" // desligado de proposito
			continue
		}
		if canonical, ok := canonicalShortcut(key); ok {
			out[action] = canonical
		} else {
			out[action] = def
		}
	}
	return out
}

// normalizeColor devolve a cor em minusculas se for #rrggbb ou "none"; senao "".
func normalizeColor(v string) string {
	c := strings.ToLower(strings.TrimSpace(v))
	if c == "none" || hexRe.MatchString(c) {
		return c
	}
	return ""
}

// sanitizeClientColors valida chave (clientId UUID) e valor (#rrggbb ou "none").
// Entradas invalidas sao descartadas; nunca nil (para round-trip estavel do jsonb).
func sanitizeClientColors(in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		id := normalizeUUID(k)
		color := normalizeColor(v)
		if id != "" && color != "" {
			out[id] = color
		}
	}
	return out
}

// sanitizeTypeColors valida o valor (#rrggbb) por tipo de evento; a chave e o tipo
// (string livre, apenas trim). "none" nao faz sentido para tipo => so #rrggbb.
func sanitizeTypeColors(in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		key := strings.TrimSpace(k)
		color := strings.ToLower(strings.TrimSpace(v))
		if key != "" && hexRe.MatchString(color) {
			out[key] = color
		}
	}
	return out
}

// sanitizeWhiteLabel faz trim das strings e valida a cor primaria (#rrggbb ou vazio).
func sanitizeWhiteLabel(w WhiteLabelConfig) WhiteLabelConfig {
	w.LogoURL = strings.TrimSpace(w.LogoURL)
	w.Title = strings.TrimSpace(w.Title)
	color := strings.ToLower(strings.TrimSpace(w.PrimaryColor))
	if hexRe.MatchString(color) {
		w.PrimaryColor = color
	} else {
		w.PrimaryColor = ""
	}
	return w
}

// sanitizeAI valida o provider (enum, default claude), faz trim das strings e prende
// a temperature no intervalo 0..1. Wave 3 (CFG v4): sanitiza tambem o provider de
// transcricao (enum, default gemini) e faz trim do transcribeModel. Enabled/UseGlobalKeys
// sao bool (passam direto). As CHAVES nunca vivem aqui — moram nos secrets.
func sanitizeAI(ai AIConfig) AIConfig {
	ai.Provider = strings.ToLower(strings.TrimSpace(ai.Provider))
	if !aiProviders[ai.Provider] {
		ai.Provider = "claude"
	}
	ai.Model = strings.TrimSpace(ai.Model)
	ai.BaseURL = strings.TrimSpace(ai.BaseURL)
	ai.SystemPrompt = strings.TrimSpace(ai.SystemPrompt)
	ai.TranscribeProvider = strings.ToLower(strings.TrimSpace(ai.TranscribeProvider))
	if !transcribeProviders[ai.TranscribeProvider] {
		ai.TranscribeProvider = "gemini"
	}
	ai.TranscribeModel = strings.TrimSpace(ai.TranscribeModel)
	switch {
	case ai.Temperature < 0:
		ai.Temperature = 0
	case ai.Temperature > 1:
		ai.Temperature = 1
	}
	// Wave 3.1 (CFG+): scopeMode no enum (case-insensitive, canonico general|perClient,
	// default general); disabledClientIds = UUIDs validos dedup (reusa normalizeClientIDs).
	switch strings.ToLower(strings.TrimSpace(ai.ScopeMode)) {
	case "perclient":
		ai.ScopeMode = scopeModePerClient
	default:
		ai.ScopeMode = scopeModeGeneral
	}
	ai.DisabledClientIDs = normalizeClientIDs(ai.DisabledClientIDs)
	return ai
}

// sanitizeChat valida a posicao da janela de chat (enum, default center) e prende
// width/height no intervalo 0..2000 (0 = default da posicao no front). CFG v4.
func sanitizeChat(c ChatConfig) ChatConfig {
	c.Position = strings.ToLower(strings.TrimSpace(c.Position))
	if !chatPositions[c.Position] {
		c.Position = "center"
	}
	c.Width = clampDim(c.Width)
	c.Height = clampDim(c.Height)
	return c
}

// clampDim prende uma dimensao (px) no intervalo 0..2000.
func clampDim(v int) int {
	switch {
	case v < 0:
		return 0
	case v > 2000:
		return 2000
	}
	return v
}

// sanitizeTasks valida a integracao com o modulo tasks (contrato C6 + WAVE 5): boardId e
// defaultColumnId sao UUID valido ou vazio (valor nao-UUID e descartado -> "").
// boardId vazio = integracao desligada (evento nao cria/vincula task). WAVE 5 (E1/E5):
// mirrorTasks (bool, passa direto); defaultEventType (trim; vazio = fallback "post" na
// criacao do espelho); statusColumnMap com columnId UUID valido, eventStatus nao-vazio e
// dedup por eventStatus (mantem a 1a ocorrencia). Backend permissivo no status (o enum
// valido vive no front, como no resto do modulo).
func sanitizeTasks(t TasksConfig) TasksConfig {
	t.BoardID = normalizeUUID(t.BoardID)
	t.DefaultColumnID = normalizeUUID(t.DefaultColumnID)
	t.DefaultEventType = strings.TrimSpace(t.DefaultEventType)
	t.StatusColumnMap = sanitizeStatusColumnMap(t.StatusColumnMap)
	return t
}

// sanitizeStatusColumnMap limpa o mapa status<->coluna (WAVE 5, E5): descarta entradas
// sem eventStatus ou com columnId nao-UUID e deduplica por eventStatus (1a vence). Sempre
// devolve slice nao-nil (JSON estavel []).
func sanitizeStatusColumnMap(in []StatusColumnMapEntry) []StatusColumnMapEntry {
	out := make([]StatusColumnMapEntry, 0, len(in))
	seen := map[string]bool{}
	for _, e := range in {
		status := strings.TrimSpace(e.EventStatus)
		col := normalizeUUID(e.ColumnID)
		if status == "" || col == "" || seen[status] {
			continue
		}
		seen[status] = true
		out = append(out, StatusColumnMapEntry{EventStatus: status, ColumnID: col})
	}
	return out
}

// ListMembers lista os usuarios da account (candidatos a responsavel).
func (s *Service) ListMembers(ctx context.Context, accountID string) ([]Member, error) {
	return s.store.ListMembers(ctx, strings.TrimSpace(accountID))
}

// ListHolidays devolve os feriados/datas comemorativas na janela [from, to]
// (inclusive), filtrados pelos conjuntos ligados na config da conta. Read-only:
// as datas sao calculadas/seed em codigo (sem tabela). from/to malformados ->
// ErrInvalidDate.
func (s *Service) ListHolidays(ctx context.Context, accountID, from, to string) ([]Holiday, error) {
	from = strings.TrimSpace(from)
	to = strings.TrimSpace(to)
	if !dateRe.MatchString(from) || !dateRe.MatchString(to) {
		return nil, ErrInvalidDate
	}
	cfg, err := s.store.GetConfig(ctx, strings.TrimSpace(accountID))
	if err != nil {
		return nil, err
	}
	return HolidaysInRange(from, to, cfg), nil
}

// ListResponsibles devolve os responsaveis ativos: subconjunto configurado ou,
// se vazio, todos os membros da account.
func (s *Service) ListResponsibles(ctx context.Context, accountID string) ([]Member, error) {
	account := strings.TrimSpace(accountID)
	cfg, err := s.store.GetConfig(ctx, account)
	if err != nil {
		return nil, err
	}
	members, err := s.store.ListMembers(ctx, account)
	if err != nil {
		return nil, err
	}
	if len(cfg.ResponsibleUserIDs) == 0 {
		return members, nil
	}
	allowed := make(map[string]bool, len(cfg.ResponsibleUserIDs))
	for _, id := range cfg.ResponsibleUserIDs {
		allowed[id] = true
	}
	out := make([]Member, 0, len(members))
	for _, m := range members {
		if allowed[m.ID] {
			out = append(out, m)
		}
	}
	return out, nil
}

// ============================================================================
// Anexos / midia (Fase 3)
// ============================================================================

// GetMediaLimits devolve os tetos de upload (globais da plataforma; default se
// nao configurado).
func (s *Service) GetMediaLimits(ctx context.Context) (MediaLimits, error) {
	legacy, err := s.store.GetMediaLimits(ctx)
	if err != nil {
		return MediaLimits{}, err
	}
	storage, ok := s.storage.(MediaLimitStorage)
	if !ok {
		return legacy, nil
	}
	managed, err := storage.Limits(ctx)
	if err != nil {
		return MediaLimits{}, err
	}
	return MediaLimits{
		ImageMaxBytes:           min(legacy.ImageMaxBytes, managed.ImageMaxBytes),
		VideoMaxBytes:           min(legacy.VideoMaxBytes, managed.VideoMaxBytes),
		R2UploadsEnabled:        managed.R2UploadsEnabled,
		MultipartThresholdBytes: managed.MultipartThresholdBytes,
	}, nil
}

// SaveMediaLimits persiste os tetos de upload (so platform_admin, gate no HTTP).
// Valores <= 0 sao rejeitados (evita travar o upload por config invalida).
func (s *Service) SaveMediaLimits(ctx context.Context, limits MediaLimits, updatedBy string) (MediaLimits, error) {
	if limits.ImageMaxBytes <= 0 || limits.VideoMaxBytes <= 0 {
		return MediaLimits{}, ErrInvalidMedia
	}
	if err := s.store.PutMediaLimits(ctx, limits, strings.TrimSpace(updatedBy)); err != nil {
		return MediaLimits{}, err
	}
	return limits, nil
}

// SaveMedia valida (mime + tamanho contra os limites da plataforma) e grava o
// anexo, devolvendo o MediaItem para o front anexar a um evento ou dia.
func (s *Service) SaveMedia(ctx context.Context, accountID, actorID, idempotencyKey, fileName, contentType string, content []byte) (MediaItem, error) {
	if s.storage == nil {
		return MediaItem{}, ErrInvalidMedia
	}
	limits, err := s.GetMediaLimits(ctx)
	if err != nil {
		return MediaItem{}, err
	}
	return s.storage.Save(ctx, strings.TrimSpace(accountID), strings.TrimSpace(actorID), strings.TrimSpace(idempotencyKey), fileName, contentType, content, limits)
}

func (s *Service) SaveMediaStream(ctx context.Context, accountID, actorID, idempotencyKey, fileName, contentType string, sizeBytes int64, content io.Reader) (MediaItem, error) {
	storage, ok := s.storage.(MediaStreamStorage)
	if !ok {
		return MediaItem{}, ErrMediaUnavailable
	}
	limits, err := s.GetMediaLimits(ctx)
	if err != nil {
		return MediaItem{}, err
	}
	return storage.SaveStream(ctx, strings.TrimSpace(accountID), strings.TrimSpace(actorID), strings.TrimSpace(idempotencyKey), fileName, contentType, sizeBytes, content, limits)
}

func (s *Service) StatMedia(ctx context.Context, accountID, objectID string) (MediaContent, error) {
	storage, ok := s.storage.(MediaContentStorage)
	if !ok {
		return MediaContent{}, ErrMediaUnavailable
	}
	return storage.Stat(ctx, strings.TrimSpace(accountID), strings.TrimSpace(objectID))
}

func (s *Service) OpenMedia(ctx context.Context, accountID, objectID, byteRange string) (MediaContent, error) {
	storage, ok := s.storage.(MediaContentStorage)
	if !ok {
		return MediaContent{}, ErrMediaUnavailable
	}
	return storage.Open(ctx, strings.TrimSpace(accountID), strings.TrimSpace(objectID), strings.TrimSpace(byteRange))
}

// WAVE 13: "anexos do dia" (calendar.day_media) foi ELIMINADO — toda midia do calendario
// pertence a um ITEM (evento), em calendar.events.media. O upload/reorder/remove agora e por
// evento (PUT /events/{id} full-replace do campo media). A migration 0199 consolidou os anexos
// vinculados no evento e transformou os orfaos em itens source='media'. Sem ListDayMedia/
// PutDayMedia/pushDayMediaToTasks: o espelho task<-evento (calendarMedia) usa so events.media
// (eventMediaForTask) e roda no proprio update do evento (syncTaskFromEvent).

// ============================================================================
// Helpers
// ============================================================================

// validateEvent valida os campos minimos e devolve o input normalizado (defaults
// + trims + client_id descartado se nao for UUID valido, evitando erro de cast).
// accountID e o dono do calendario (do Principal, nunca do body): amarra o prefixo
// da midia a conta em normalizeMedia.
func validateEvent(accountID string, in EventInput) (EventInput, error) {
	in.Date = strings.TrimSpace(in.Date)
	if !dateRe.MatchString(in.Date) {
		return in, ErrInvalidDate
	}
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		return in, ErrInvalidTitle
	}
	in.Time = strings.TrimSpace(in.Time)
	in.ClientID = normalizeUUID(in.ClientID)
	in.Type = firstNonEmpty(strings.TrimSpace(in.Type), "post")
	in.Status = firstNonEmpty(strings.TrimSpace(in.Status), "planejado")
	in.Priority = firstNonEmpty(strings.TrimSpace(in.Priority), "media")
	in.ResponsibleID = strings.TrimSpace(in.ResponsibleID)
	if in.InvolvedIDs == nil {
		in.InvolvedIDs = []string{}
	}
	in.Media = normalizeMedia(accountID, in.Media)
	return in, nil
}

// normalizeUUID devolve o UUID em minusculas se valido; senao "" (sem cliente/
// sem filtro), evitando erro de cast ::uuid no banco com ids nao-UUID (ex.: os
// clientes de demonstracao do front mock).
func normalizeUUID(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	if uuidRe.MatchString(id) {
		return id
	}
	return ""
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// mapNotFound colapsa pgx.ErrNoRows em ErrNotFound (404).
func mapNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
