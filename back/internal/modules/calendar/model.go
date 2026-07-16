package calendar

import (
	"encoding/json"
	"time"
)

// CalendarEvent e um evento de conteudo do calendario (schema calendar.events).
type CalendarEvent struct {
	ID            string
	AccountID     string
	ClientID      *string // null = evento sem cliente
	Date          string  // 'YYYY-MM-DD'
	Time          string
	Type          string
	Title         string
	Status        string
	Priority      string
	ResponsibleID string
	InvolvedIDs   json.RawMessage
	Media         json.RawMessage
	Description   string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	// Version e o contador de revisao do evento (contrato C12), usado no optimistic
	// locking do PUT (If-Match). Escrita bem-sucedida faz version = version + 1. Sempre
	// preenchido pelos scans (eventCols inclui version).
	Version int
	// TaskID e a task vinculada ao evento (contrato C10), resolvida via LEFT JOIN em
	// tasks.task_relations (module='calendar', resource_type='event') no ListEvents/
	// GetEvent. nil = sem vinculo. So e preenchida pelas leituras com o join (scanEventWithTask);
	// nas escritas basicas (scanEvent) fica nil.
	TaskID *string
	// Source e a procedencia do evento (WAVE 5, E1): 'manual' (tela), 'task' (espelho de
	// uma task) ou 'ai' (proposta confirmada do chat). Server-controlled (nunca do body).
	// A guarda anti-loop do espelho so cria/apaga eventos com source='task'.
	Source string
	// LinkedMedia (WAVE 6, cruzamento B): midia ESPELHADA da task vinculada (os videos de
	// ui_metadata.videos), read-only, mantida pelo sync task->evento. jsonb MediaItem[]. O
	// evento nao tem ui_metadata, entao mora em coluna propria (0193). Nunca editada pela UI.
	LinkedMedia json.RawMessage
}

// EventView e a projecao JSON do evento. As chaves batem 1:1 com o tipo
// CalendarEvent do front (web/app/utils/calendar.ts).
type EventView struct {
	ID            string          `json:"id"`
	Date          string          `json:"date"`
	Time          string          `json:"time"`
	ClientID      string          `json:"clientId"`
	Type          string          `json:"type"`
	Title         string          `json:"title"`
	Status        string          `json:"status"`
	Priority      string          `json:"priority"`
	ResponsibleID string          `json:"responsibleId"`
	InvolvedIDs   json.RawMessage `json:"involvedIds"`
	Media         json.RawMessage `json:"media"`
	Description   string          `json:"description"`
	// Version = contador de revisao do evento (contrato C12). O front guarda este numero
	// e o reenvia em If-Match no PUT; divergencia => 409 version_conflict.
	Version int `json:"version"`
	// TaskID = task vinculada via tasks.task_relations (contrato C10); "" = sem vinculo.
	TaskID string `json:"taskId"`
	// Source = procedencia do evento (WAVE 5): manual|task|ai. O front usa 'task' para
	// estilizar o evento-espelho de forma distinta no calendario.
	Source string `json:"source"`
	// LinkedMedia = midia espelhada da task vinculada (WAVE 6 cruzamento B), read-only. O
	// front une na "Midia do post" do evento (com a midia propria + anexos do dia apontados).
	LinkedMedia json.RawMessage `json:"linkedMedia"`
	// TaskWarning e um aviso transitorio SO da resposta de POST /events com createTask:
	// a task falhou mas o evento foi salvo (omitido nas leituras). Nunca persiste.
	TaskWarning string `json:"taskWarning,omitempty"`
}

func (e CalendarEvent) view() EventView {
	client := ""
	if e.ClientID != nil {
		client = *e.ClientID
	}
	taskID := ""
	if e.TaskID != nil {
		taskID = *e.TaskID
	}
	source := e.Source
	if source == "" {
		source = "manual"
	}
	return EventView{
		ID:            e.ID,
		Date:          e.Date,
		Time:          e.Time,
		ClientID:      client,
		Type:          e.Type,
		Title:         e.Title,
		Status:        e.Status,
		Priority:      e.Priority,
		ResponsibleID: e.ResponsibleID,
		InvolvedIDs:   normalizeArray(e.InvolvedIDs),
		Media:         normalizeArray(e.Media),
		Description:   e.Description,
		Version:       e.Version,
		TaskID:        taskID,
		Source:        source,
		LinkedMedia:   normalizeArray(e.LinkedMedia),
	}
}

// normalizeArray garante "[]" no lugar de nil (sempre um array no JSON).
func normalizeArray(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("[]")
	}
	return raw
}

// EventInput e o body de POST/PUT de evento (full replace dos campos mutaveis).
type EventInput struct {
	Date          string      `json:"date"`
	Time          string      `json:"time"`
	ClientID      string      `json:"clientId"`
	Type          string      `json:"type"`
	Title         string      `json:"title"`
	Status        string      `json:"status"`
	Priority      string      `json:"priority"`
	ResponsibleID string      `json:"responsibleId"`
	InvolvedIDs   []string    `json:"involvedIds"`
	Media         []MediaItem `json:"media"`
	Description   string      `json:"description"`
	// CreateTask (contrato C10) pede a criacao de uma task vinculada ao criar o evento
	// (POST /events). So vale no POST; requer tasks.boardId na config C6 (senao 400
	// tasks_not_configured). Ignorado no PUT.
	CreateTask bool `json:"createTask"`
	// IsMediaItem (WAVE 11): marca o evento como ITEM ESPECIAL DE MIDIA (upload avulso do dia
	// que vira tarefa: titulo = nome do arquivo; o calendario mostra so a midia, sem titulo).
	// Vira Source='media' server-side no CreateEvent — o client pode pedir 'media', mas NUNCA
	// 'task' (o source do espelho segue server-controlled).
	IsMediaItem bool `json:"mediaItem"`
	// Source e a procedencia do evento (WAVE 5, E1), SERVER-CONTROLLED: `json:"-"` garante
	// que o body nunca define isso. A borda HTTP deixa vazio (-> 'manual' no store); o
	// espelho task->evento seta 'task' internamente (guarda anti-loop, E3).
	Source string `json:"-"`
}

// MediaItem e um anexo (imagem ou video) de um evento. url sempre aponta para
// /uploads/calendar/{accountId}/{arquivo} (nunca URL externa). Persistido como jsonb
// em calendar.events.media (WAVE 13: day_media eliminado — toda midia e de um item).
// PosterURL e a capa do video (opcional; so faz sentido para type "video"),
// capturada no FRONT via canvas e enviada como upload de imagem normal. Passa
// pela MESMA validacao de prefixo interno do url (externo => zerado, ver C1).
type MediaItem struct {
	ID          string `json:"id"`
	URL         string `json:"url"`
	Name        string `json:"name"`
	Type        string `json:"type"` // "image" | "video"
	ContentType string `json:"contentType"`
	SizeBytes   int    `json:"sizeBytes"`
	PosterURL   string `json:"posterUrl,omitempty"`
	// ClientID (WAVE 6): cliente dono do anexo (UUID de core.accounts) OU vazio = sem
	// cliente. Um dia pode ter itens de clientes diferentes; cada upload diz a quem pertence.
	// Derivado do evento quando EventID esta setado. Nao-UUID descartado (vira "").
	ClientID string `json:"clientId,omitempty"`
	// EventID (WAVE 6, W6-4): evento/item do dia a que o anexo pertence (UUID de
	// calendar.events) OU vazio = anexo do dia sem item. Liga o anexo ao item (e, por
	// tabela, a task vinculada). Nao-UUID descartado.
	EventID string `json:"eventId,omitempty"`
}

// MediaLimits sao os tetos de upload definidos NA PLATAFORMA (global, editavel por
// platform_admin) — core.platform_settings, chave 'media_limits'.
type MediaLimits struct {
	ImageMaxBytes int64 `json:"imageMaxBytes"`
	VideoMaxBytes int64 `json:"videoMaxBytes"`
}

// defaultMediaLimits: imagem 10MB, video 300MB (o "atualmente 300mb").
func defaultMediaLimits() MediaLimits {
	return MediaLimits{ImageMaxBytes: 10 * 1024 * 1024, VideoMaxBytes: 300 * 1024 * 1024}
}

// EventFilter sao os filtros da listagem: janela de datas (inclusive) + cliente.
type EventFilter struct {
	From     string // 'YYYY-MM-DD'
	To       string // 'YYYY-MM-DD'
	ClientID string
}

// NoteView e a nota de um mes (calendar.notes).
type NoteView struct {
	Month     string    `json:"month"`
	Content   string    `json:"content"`
	UpdatedBy string    `json:"updatedBy,omitempty"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// NoteInput e o body do PUT de notas.
type NoteInput struct {
	Content string `json:"content"`
}

// ============================================================================
// Config por conta (Fase 2)
// ============================================================================

// HolidayConfig liga/desliga cada conjunto de feriados/datas comemorativas.
type HolidayConfig struct {
	BrNational bool `json:"brNational"`
	Sergipe    bool `json:"sergipe"`
	Aracaju    bool `json:"aracaju"`
	LuxuryIntl bool `json:"luxuryIntl"`
}

// WhiteLabelConfig personaliza a marca da visao do calendario (logo/titulo/cor).
// Tudo vazio = sem white-label (usa a marca padrao do painel).
type WhiteLabelConfig struct {
	LogoURL      string `json:"logoUrl"`
	Title        string `json:"title"`
	PrimaryColor string `json:"primaryColor"`
}

// AIConfig e a config da IA do calendario (plano/chat/transcricao). As CHAVES de API
// NUNCA vivem aqui — moram nos secrets (calendar.ai_secrets por conta ou a chave global
// da plataforma), resolvidas server-side. baseUrl vazio = default do provider (mapa no
// n8n e no front). systemPrompt vazio = prompt default do workflow.
// Wave 3 (CFG v4): Enabled = kill switch da IA; UseGlobalKeys = true usa as chaves
// GLOBAIS da plataforma, false usa as chaves DESTA conta; TranscribeProvider/Model =
// transcricao de voz (openai|gemini; local fica p/ depois).
type AIConfig struct {
	Enabled            bool    `json:"enabled"`       // kill switch da IA do calendario
	UseGlobalKeys      bool    `json:"useGlobalKeys"` // true = chaves da plataforma; false = chaves da conta
	Provider           string  `json:"provider"`      // gemini|glm|openai|claude|deepseek|qwen|kimi|custom (C6)
	Model              string  `json:"model"`
	BaseURL            string  `json:"baseUrl"`
	SystemPrompt       string  `json:"systemPrompt"`
	Temperature        float64 `json:"temperature"`        // clamp 0..1
	TranscribeProvider string  `json:"transcribeProvider"` // openai|gemini (local fica p/ depois)
	TranscribeModel    string  `json:"transcribeModel"`    // vazio = default do provider
	// Wave 3.1 (CFG+): escopo da IA por cliente. ScopeMode = general (uma config p/
	// todos) | perClient (config individual por cliente). DisabledClientIDs = clientes
	// com a IA DESLIGADA (excecoes; vale nos DOIS modos). O override de comportamento
	// por cliente mora em calendar.client_profiles.ai_config (ver ClientAIOverride).
	ScopeMode         string   `json:"scopeMode"`         // general | perClient
	DisabledClientIDs []string `json:"disabledClientIds"` // clientes com a IA desligada
}

// ClientAIOverride e o override de COMPORTAMENTO da IA de UM cliente (WAVE 3.1, SEC+),
// persistido em calendar.client_profiles.ai_config. Enabled/Temperature sao ponteiros
// para distinguir "nao setado" (nil = herda a config geral) de zero (false/0.0). Os
// demais campos vazios tambem herdam. A API KEY NUNCA vive aqui: o override so muda o
// comportamento (provider/model/baseUrl/systemPrompt/temperature), nunca a credencial —
// a key resolve pelo provider EFETIVO no nivel conta/global (resolveAIKey da 3.0).
type ClientAIOverride struct {
	Enabled      *bool    `json:"enabled"`
	Provider     string   `json:"provider"`
	Model        string   `json:"model"`
	BaseURL      string   `json:"baseUrl"`
	SystemPrompt string   `json:"systemPrompt"`
	Temperature  *float64 `json:"temperature"`
}

// ChatConfig e o layout da janela de chat do calendario (por conta, CFG v4). Position
// dirige a largura: center = area interna do calendario; left = painel esquerdo; right
// = modal direito. Width/Height em px (0 = default da posicao; clamp 0..2000).
type ChatConfig struct {
	Position string `json:"position"` // center | left | right
	Width    int    `json:"width"`    // px; 0 = default da posicao
	Height   int    `json:"height"`   // px; 0 = default da posicao
}

// TasksConfig liga a integracao calendario<->tasks (contrato C6 + WAVE 5). BoardID/
// DefaultColumnID vazios = integracao DESLIGADA (evento nao cria/vincula task).
// Ambos sao UUID-ou-vazio; valores nao-UUID sao descartados na sanitizacao.
// WAVE 5 (E1): MirrorTasks liga o espelho task->evento (default true); DefaultEventType
// e o tipo do evento-espelho nascido de task; StatusColumnMap mapeia status de evento <->
// coluna do board nos dois sentidos (E5). MirrorTasks/StatusColumnMap so tem efeito com
// BoardID configurado.
type TasksConfig struct {
	BoardID          string                 `json:"boardId"`
	DefaultColumnID  string                 `json:"defaultColumnId"`
	MirrorTasks      bool                   `json:"mirrorTasks"`
	DefaultEventType string                 `json:"defaultEventType"`
	StatusColumnMap  []StatusColumnMapEntry `json:"statusColumnMap"`
}

// StatusColumnMapEntry mapeia UM status de evento a UMA coluna do board (WAVE 5, E5). O
// mapa vale nos dois sentidos: evento muda status -> task vai pra coluna; task muda de
// coluna -> evento ganha o status. eventStatus fora do conjunto valido / columnId nao-UUID
// sao descartados na sanitizacao.
type StatusColumnMapEntry struct {
	EventStatus string `json:"eventStatus"`
	ColumnID    string `json:"columnId"`
}

// CalendarConfig e a config do calendario por account (jsonb em calendar.config).
// ResponsibleUserIDs vazio = todos os membros da conta aparecem como responsaveis.
// WeekStartsOn: "sunday" (default) | "monday". ClientColors: { [clientId]:
// "#rrggbb" | "none" } — vazio = paleta automatica. TypeColors: { [tipo]:
// "#rrggbb" } — vazio = cor do cliente. Contrato C2 (SPEC-B3).
type CalendarConfig struct {
	ResponsibleUserIDs []string          `json:"responsibleUserIds"`
	Holidays           HolidayConfig     `json:"holidays"`
	WeekStartsOn       string            `json:"weekStartsOn"`
	ClientColors       map[string]string `json:"clientColors"`
	TypeColors         map[string]string `json:"typeColors"`
	WhiteLabel         WhiteLabelConfig  `json:"whiteLabel"`
	AI                 AIConfig          `json:"ai"`
	Tasks              TasksConfig       `json:"tasks"` // integracao com tasks (C6)
	Chat               ChatConfig        `json:"chat"`  // layout da janela de chat (CFG v4)
	// Shortcuts (WAVE 11) = mapa de atalhos de teclado configuravel pelo painel:
	// { acao: combo }. Acoes conhecidas em shortcutDefaults(); combo = modificadores
	// opcionais (ctrl/alt/shift/meta, ordem canonica) + tecla-base (1 caractere a-z/0-9
	// ou nome especial enter/escape/space/arrow*), ex.: 'shift+t', 'ctrl+shift+k', 't'.
	// Vazio = atalho DESLIGADO. Chave ausente no jsonb = default (merge no unmarshal).
	Shortcuts map[string]string `json:"shortcuts"`
}

// shortcutDefaults sao as acoes de atalho conhecidas + a tecla default de cada uma
// (WAVE 11). chat* valem com a janela do assistente; cal* valem na pagina do calendario.
func shortcutDefaults() map[string]string {
	return map[string]string{
		"chatOpen":        "c",          // abrir/fechar o assistente
		"chatRecordStart": "a",          // iniciar gravacao (janela aberta)
		"chatRecordStop":  "enter",      // parar gravacao (gravando)
		"chatClose":       "escape",     // fechar a janela da IA (mesmo sem foco)
		"calToday":        "t",          // botao Hoje
		"calMonthView":    "m",          // visao mensal
		"calWeekView":     "w",          // visao semanal
		"calNewItem":      "n",          // + Novo
		"calNotesSidebar": "s",          // recolher/mostrar as anotacoes
		"calSpans":        "b",          // mostrar/ocultar as barras multi-dia
		"calPrev":         "arrowleft",  // mes/semana anterior
		"calNext":         "arrowright", // proximo mes/semana
	}
}

// defaultConfig e o estado inicial: todos os conjuntos de feriados ligados,
// nenhum responsavel filtrado (= todos os membros), semana comecando domingo,
// sem cores custom nem white-label, IA no provider claude e integracao com tasks
// desligada (Tasks vazio). Wave 3 (CFG v4): IA nasce LIGADA (enabled) usando as
// chaves GLOBAIS da plataforma (useGlobalKeys), transcricao no gemini e a janela
// de chat centralizada. Preenche TODOS os campos dos contratos C2/C6/v4 para que
// linha antiga (config sem as secoes novas) ganhe os campos ao ler (unmarshal por
// cima destes defaults no store; struct de valor com chave ausente ou null vira
// no-op, entao os campos novos ficam com estes defaults).
func defaultConfig() CalendarConfig {
	return CalendarConfig{
		ResponsibleUserIDs: []string{},
		Holidays:           HolidayConfig{BrNational: true, Sergipe: true, Aracaju: true, LuxuryIntl: true},
		WeekStartsOn:       "sunday",
		ClientColors:       map[string]string{},
		TypeColors:         map[string]string{},
		WhiteLabel:         WhiteLabelConfig{},
		AI: AIConfig{
			// Default gemini (tem slot de chave em secretProviders); claude/deepseek/etc nao
			// tem slot na Wave 3, entao nasceriam em ai_key_missing sem onde gravar a chave.
			// Wave 3.1: nasce no modo geral (uma config p/ todos), sem clientes desativados.
			Enabled: true, UseGlobalKeys: true, Provider: "gemini", Model: "gemini-2.5-flash",
			Temperature: 0.7, TranscribeProvider: "local",
			ScopeMode: scopeModeGeneral, DisabledClientIDs: []string{},
		},
		// WAVE 5: espelho task->evento LIGADO por padrao (decisao do dono 2026-07-05); so
		// tem efeito quando ha board configurado. StatusColumnMap vazio = sem sync de status.
		Tasks: TasksConfig{MirrorTasks: true, StatusColumnMap: []StatusColumnMapEntry{}},
		Chat:  ChatConfig{Position: "center"},
		// WAVE 11: atalhos de teclado com os defaults do produto (editaveis pelo painel).
		Shortcuts: shortcutDefaults(),
	}
}

// Member e um usuario da conta (candidato/atual a responsavel).
type Member struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
