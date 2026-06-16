package automation

import "time"

// Automation e o "robo" — uma automacao de WhatsApp/IA de uma account.
// Entidade central do modulo (N por account; V0 opera 1 default por account).
type Automation struct {
	ID        string
	AccountID string
	Type      string
	Name      string
	Slug      string
	Status    string // draft | active | paused (active = ligado)
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Channel e a conexao do canal (WAHA session = 1 numero). provider pluggable.
type Channel struct {
	ID             string
	AutomationID   string
	AccountID      string
	Provider       string
	SessionName    string
	Status         string
	ConnectedPhone *string
	UpdatedAt      time.Time
}

// Persona e o comportamento da automacao (instrucoes + conhecimento). 1 ativa por
// automacao. system_prompt e o texto da persona; os guardrails sao anexados no runtime.
type Persona struct {
	ID           string
	AutomationID string
	AccountID    string
	Name         string
	SystemPrompt string
	IsActive     bool
}

// PersonaView e a projecao da persona para o painel (editor de comportamento).
type PersonaView struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	SystemPrompt string `json:"systemPrompt"`
}

func toPersonaView(p Persona) PersonaView {
	return PersonaView{ID: p.ID, Name: p.Name, SystemPrompt: p.SystemPrompt}
}

// RuntimeConfigView e o que o n8n consome a cada execucao (auth por service token).
// systemMessage = montagem completa (Opcao A / fallback).
// persona + guardrails + docs = partes separadas para o n8n montar dinamicamente (Opcao B).
// models = modelo escolhido por funcao (chat/vision/audio/classifier) com as flags do
// MODELOS.md, para o n8n selecionar o no certo e os params (Responses API / temperature).
type RuntimeConfigView struct {
	Enabled       bool                  `json:"enabled"`
	SystemMessage string                `json:"systemMessage"`
	Persona       string                `json:"persona"`
	Guardrails    string                `json:"guardrails"`
	Docs          []KnowledgeDocView    `json:"docs"`
	Models        []AutomationModelView `json:"models"`
}

// OverviewView e a projecao do painel: estado do robo + conexao do WhatsApp.
type OverviewView struct {
	Automation AutomationView `json:"automation"`
	WhatsApp   WhatsAppView   `json:"whatsapp"`
}

// AutomationView e a projecao lean do robo para o front.
type AutomationView struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Slug    string `json:"slug"`
	Type    string `json:"type"`
	Status  string `json:"status"`  // draft | active | paused
	Enabled bool   `json:"enabled"` // status == active
}

// WhatsAppView e o estado da conexao (lido da WAHA em tempo real).
type WhatsAppView struct {
	Provider       string `json:"provider"`
	SessionName    string `json:"sessionName"`
	Status         string `json:"status"` // STOPPED | STARTING | SCAN_QR_CODE | WORKING | FAILED
	Connected      bool   `json:"connected"`
	ConnectedPhone string `json:"connectedPhone,omitempty"`
}

// ConnectView retorna o QR (data URL base64) quando precisa escanear.
type ConnectView struct {
	Status         string `json:"status"`
	QR             string `json:"qr,omitempty"` // data:image/png;base64,...
	ConnectedPhone string `json:"connectedPhone,omitempty"`
}

func toAutomationView(a Automation) AutomationView {
	return AutomationView{
		ID:      a.ID,
		Name:    a.Name,
		Slug:    a.Slug,
		Type:    a.Type,
		Status:  a.Status,
		Enabled: a.Status == statusActive,
	}
}

// KnowledgeDoc e um documento de conhecimento da automacao. O runtime-config
// concatena os habilitados (em sort_order) no systemMessage apos as instrucoes
// da persona e antes dos guardrails. RAG (pgvector, P8) entra quando o volume
// for grande demais para caber no contexto.
type KnowledgeDoc struct {
	ID           string
	AutomationID string
	AccountID    string
	Title        string
	Body         string
	SortOrder    int
	Enabled      bool
}

// KnowledgeDocView e a projecao do documento para o painel.
type KnowledgeDocView struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	SortOrder int    `json:"sortOrder"`
	Enabled   bool   `json:"enabled"`
}

func toKnowledgeDocView(d KnowledgeDoc) KnowledgeDocView {
	return KnowledgeDocView{ID: d.ID, Title: d.Title, Body: d.Body, SortOrder: d.SortOrder, Enabled: d.Enabled}
}

// Contact armazena o contexto de conversa de um cliente (chatId) no Postgres,
// em vez do staticData do n8n. Quando um doc de conhecimento e deletado ou
// atualizado o Go zera long_memory de todos os contatos da automacao.
// PausedUntil (M4) marca handover humano: enquanto > now(), o bot fica em silencio.
type Contact struct {
	ID           string
	AutomationID string
	AccountID    string
	ChatID       string
	Seg          int
	LastMsg      string
	LastMsgTs    int64
	LongMemory   string
	PausedUntil  *time.Time
}

// ContactMemoryView e o contrato de leitura/escrita entre o n8n e o runtime.
// paused/pausedUntil (M4) dizem ao n8n se deve ficar em silencio (handover humano).
type ContactMemoryView struct {
	Seg         int    `json:"seg"`
	LastMsg     string `json:"lastMsg"`
	LastMsgTs   int64  `json:"ts"`
	LongMemory  string `json:"longMem"`
	Paused      bool   `json:"paused"`
	PausedUntil string `json:"pausedUntil"`
}

func toContactMemoryView(c Contact) ContactMemoryView {
	v := ContactMemoryView{Seg: c.Seg, LastMsg: c.LastMsg, LastMsgTs: c.LastMsgTs, LongMemory: c.LongMemory}
	if c.PausedUntil != nil {
		v.PausedUntil = c.PausedUntil.UTC().Format(time.RFC3339)
		v.Paused = c.PausedUntil.After(time.Now())
	}
	return v
}

// ContextPreviewView e o que o painel mostra na secao "Contexto do bot":
// a estrutura completa que sera montada no systemMessage (instrucoes + docs + guardrails).
// Retorna cada bloco separado para que o front mostre de onde cada parte vem.
type ContextPreviewView struct {
	PersonaName   string             `json:"personaName"`
	Instructions  string             `json:"instructions"`
	KnowledgeDocs []KnowledgeDocView `json:"knowledgeDocs"`
	Guardrails    string             `json:"guardrails"`
	SystemMessage string             `json:"systemMessage"`
}

const (
	statusDraft  = "draft"
	statusActive = "active"
	statusPaused = "paused"

	// defaultSlug / defaultSession: V0 opera 1 automacao "Tony" por account,
	// na sessao WAHA "default" (WAHA Core suporta 1 sessao). Multi-numero = P11.
	defaultSlug    = "tony"
	defaultName    = "Tony"
	defaultType    = "atendimento"
	defaultSession = "default"
	providerWAHA   = "waha"
)
