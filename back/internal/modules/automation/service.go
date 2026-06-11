package automation

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Service orquestra a persistencia (automation.*) e o proxy da WAHA.
type Service struct {
	store *Store
	waha  *WAHAClient
}

// NewService cria o Service.
func NewService(store *Store, waha *WAHAClient) *Service {
	return &Service{store: store, waha: waha}
}

// Overview retorna o estado do robo + a conexao do WhatsApp (lida da WAHA em
// tempo real, com fallback para o ultimo estado persistido).
func (s *Service) Overview(ctx context.Context, accountID string) (OverviewView, error) {
	a, ch, err := s.store.GetOrCreateDefault(ctx, accountID)
	if err != nil {
		return OverviewView{}, err
	}

	status, phone := s.liveStatus(ctx, ch)
	return OverviewView{
		Automation: toAutomationView(a),
		WhatsApp:   buildWhatsAppView(ch, status, phone),
	}, nil
}

// Connect garante a sessao iniciada e devolve o QR para escanear. Se ja estiver
// conectado (WORKING), nao gera QR.
func (s *Service) Connect(ctx context.Context, accountID string) (ConnectView, error) {
	_, ch, err := s.store.GetOrCreateDefault(ctx, accountID)
	if err != nil {
		return ConnectView{}, err
	}

	if status, phone := s.liveStatus(ctx, ch); status == statusWorking {
		return ConnectView{Status: statusWorking, ConnectedPhone: phone}, nil
	}

	if err := s.waha.Start(ctx, ch.SessionName); err != nil {
		return ConnectView{}, err
	}
	qr, err := s.waha.QR(ctx, ch.SessionName)
	if err != nil {
		return ConnectView{}, err
	}
	_ = s.store.UpdateChannelStatus(ctx, ch.ID, "SCAN_QR_CODE", "")
	return ConnectView{Status: "SCAN_QR_CODE", QR: qr}, nil
}

// Disconnect encerra a sessao do WhatsApp.
func (s *Service) Disconnect(ctx context.Context, accountID string) error {
	_, ch, err := s.store.GetOrCreateDefault(ctx, accountID)
	if err != nil {
		return err
	}
	if err := s.waha.Stop(ctx, ch.SessionName); err != nil {
		return err
	}
	return s.store.UpdateChannelStatus(ctx, ch.ID, "STOPPED", "")
}

// SetEnabled liga/desliga o robo (status active|paused). Em M1 isso so persiste o
// estado; o n8n passa a respeita-lo quando consumir o runtime-config (fase M2).
func (s *Service) SetEnabled(ctx context.Context, accountID string, enabled bool) (AutomationView, error) {
	a, _, err := s.store.GetOrCreateDefault(ctx, accountID)
	if err != nil {
		return AutomationView{}, err
	}
	status := statusPaused
	if enabled {
		status = statusActive
	}
	if err := s.store.UpdateAutomationStatus(ctx, a.ID, status); err != nil {
		return AutomationView{}, err
	}
	a.Status = status
	return toAutomationView(a), nil
}

// Persona retorna a persona ativa da automacao default da account (semeia a default
// Tony/Crow se ainda nao existir). Usado pelo editor do painel.
func (s *Service) Persona(ctx context.Context, accountID string) (PersonaView, error) {
	a, _, err := s.store.GetOrCreateDefault(ctx, accountID)
	if err != nil {
		return PersonaView{}, err
	}
	p, err := s.ensurePersona(ctx, a)
	if err != nil {
		return PersonaView{}, err
	}
	return toPersonaView(p), nil
}

// UpdatePersona salva o comportamento editado no painel (instrucoes + conhecimento).
// Os guardrails sao anexados no runtime, fora deste texto.
func (s *Service) UpdatePersona(ctx context.Context, accountID, name, systemPrompt string) (PersonaView, error) {
	a, _, err := s.store.GetOrCreateDefault(ctx, accountID)
	if err != nil {
		return PersonaView{}, err
	}
	p, err := s.ensurePersona(ctx, a)
	if err != nil {
		return PersonaView{}, err
	}
	if name == "" {
		name = p.Name
	}
	updated, err := s.store.UpdatePersona(ctx, p.ID, name, systemPrompt)
	if err != nil {
		return PersonaView{}, err
	}
	return toPersonaView(updated), nil
}

// RuntimeConfig e o que o n8n consome a cada execucao: resolve a sessao -> automacao,
// monta o systemMessage (persona ativa + knowledge docs + guardrails) e diz se o robo
// esta ligado. Semeia a persona default (Tony/Crow) na primeira vez.
func (s *Service) RuntimeConfig(ctx context.Context, session string) (RuntimeConfigView, error) {
	ch, err := s.store.GetChannelBySession(ctx, session)
	if err != nil {
		return RuntimeConfigView{}, err
	}
	a, err := s.store.GetAutomationByID(ctx, ch.AutomationID)
	if err != nil {
		return RuntimeConfigView{}, err
	}
	persona, err := s.ensurePersona(ctx, a)
	if err != nil {
		return RuntimeConfigView{}, err
	}
	docs, err := s.store.ListKnowledgeDocs(ctx, a.ID)
	if err != nil {
		return RuntimeConfigView{}, err
	}
	docViews := make([]KnowledgeDocView, len(docs))
	for i, d := range docs {
		docViews[i] = toKnowledgeDocView(d)
	}
	return RuntimeConfigView{
		Enabled:       a.Status == statusActive,
		SystemMessage: buildSystemMessage(persona.SystemPrompt, docs),
		Persona:       persona.SystemPrompt,
		Guardrails:    defaultGuardrails,
		Docs:          docViews,
	}, nil
}

// KnowledgeDocs lista os documentos de conhecimento da automacao default da account.
func (s *Service) KnowledgeDocs(ctx context.Context, accountID string) ([]KnowledgeDocView, error) {
	a, _, err := s.store.GetOrCreateDefault(ctx, accountID)
	if err != nil {
		return nil, err
	}
	docs, err := s.store.ListKnowledgeDocs(ctx, a.ID)
	if err != nil {
		return nil, err
	}
	views := make([]KnowledgeDocView, len(docs))
	for i, d := range docs {
		views[i] = toKnowledgeDocView(d)
	}
	return views, nil
}

// CreateKnowledgeDoc cria um documento de conhecimento para a automacao default.
// sort_order inicial = numero de documentos ja existentes (append).
func (s *Service) CreateKnowledgeDoc(ctx context.Context, accountID, title, body string) (KnowledgeDocView, error) {
	a, _, err := s.store.GetOrCreateDefault(ctx, accountID)
	if err != nil {
		return KnowledgeDocView{}, err
	}
	existing, err := s.store.ListKnowledgeDocs(ctx, a.ID)
	if err != nil {
		return KnowledgeDocView{}, err
	}
	d, err := s.store.CreateKnowledgeDoc(ctx, a.ID, a.AccountID, title, body, len(existing))
	if err != nil {
		return KnowledgeDocView{}, err
	}
	return toKnowledgeDocView(d), nil
}

// UpdateKnowledgeDoc edita um documento e zera a memoria longa de todos os
// contatos da automacao (info desatualizada nao deve persistir no longMem).
func (s *Service) UpdateKnowledgeDoc(ctx context.Context, accountID, docID, title, body string, sortOrder int, enabled bool) (KnowledgeDocView, error) {
	a, _, err := s.store.GetOrCreateDefault(ctx, accountID)
	if err != nil {
		return KnowledgeDocView{}, err
	}
	d, err := s.store.UpdateKnowledgeDoc(ctx, docID, a.ID, title, body, sortOrder, enabled)
	if err != nil {
		return KnowledgeDocView{}, err
	}
	if err := s.store.ClearLongMemory(ctx, a.ID); err != nil {
		slog.WarnContext(ctx, "falha ao limpar long_memory apos edicao de doc", "automation_id", a.ID, "err", err)
	}
	return toKnowledgeDocView(d), nil
}

// DeleteKnowledgeDoc remove um documento e zera a memoria longa de todos os
// contatos da automacao.
func (s *Service) DeleteKnowledgeDoc(ctx context.Context, accountID, docID string) error {
	a, _, err := s.store.GetOrCreateDefault(ctx, accountID)
	if err != nil {
		return err
	}
	if err := s.store.DeleteKnowledgeDoc(ctx, docID, a.ID); err != nil {
		return err
	}
	if err := s.store.ClearLongMemory(ctx, a.ID); err != nil {
		slog.WarnContext(ctx, "falha ao limpar long_memory apos exclusao de doc", "automation_id", a.ID, "err", err)
	}
	return nil
}

// GetMemory retorna a memoria de conversa de um contato (consumido pelo n8n).
// Retorna defaults zerados se o contato ainda nao existe.
func (s *Service) GetMemory(ctx context.Context, session, chatID string) (ContactMemoryView, error) {
	ch, err := s.store.GetChannelBySession(ctx, session)
	if err != nil {
		return ContactMemoryView{}, err
	}
	c, err := s.store.GetContact(ctx, ch.AutomationID, chatID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ContactMemoryView{}, nil
	}
	if err != nil {
		return ContactMemoryView{}, err
	}
	return toContactMemoryView(c), nil
}

// SaveMemory persiste o estado de conversa de um contato (chamado pelo n8n).
// longMem vazio = preserva o valor existente no banco (partial update de seg/ts).
func (s *Service) SaveMemory(ctx context.Context, session, chatID string, seg int, lastMsg string, ts int64, longMem string) error {
	ch, err := s.store.GetChannelBySession(ctx, session)
	if err != nil {
		return err
	}
	return s.store.UpsertContact(ctx, ch.AutomationID, chatID, seg, lastMsg, ts, longMem)
}

// ContextPreview monta a previa do systemMessage completo para exibicao no painel.
// Nao requer sessao — usa a automacao default da account. Cada bloco e retornado
// separado para que o front mostre de onde cada parte vem.
func (s *Service) ContextPreview(ctx context.Context, accountID string) (ContextPreviewView, error) {
	a, _, err := s.store.GetOrCreateDefault(ctx, accountID)
	if err != nil {
		return ContextPreviewView{}, err
	}
	persona, err := s.ensurePersona(ctx, a)
	if err != nil {
		return ContextPreviewView{}, err
	}
	docs, err := s.store.ListKnowledgeDocs(ctx, a.ID)
	if err != nil {
		return ContextPreviewView{}, err
	}
	docViews := make([]KnowledgeDocView, len(docs))
	for i, d := range docs {
		docViews[i] = toKnowledgeDocView(d)
	}
	return ContextPreviewView{
		PersonaName:   persona.Name,
		Instructions:  persona.SystemPrompt,
		KnowledgeDocs: docViews,
		Guardrails:    defaultGuardrails,
		SystemMessage: buildSystemMessage(persona.SystemPrompt, docs),
	}, nil
}

// buildSystemMessage monta o systemMessage completo:
// instrucoes da persona + knowledge docs habilitados (em sort_order) + guardrails.
func buildSystemMessage(instructions string, docs []KnowledgeDoc) string {
	var sb strings.Builder
	sb.WriteString(instructions)
	for _, d := range docs {
		if !d.Enabled {
			continue
		}
		sb.WriteString("\n\n---\n\n")
		if d.Title != "" {
			sb.WriteString("## ")
			sb.WriteString(d.Title)
			sb.WriteString("\n\n")
		}
		sb.WriteString(d.Body)
	}
	sb.WriteString("\n\n")
	sb.WriteString(defaultGuardrails)
	return sb.String()
}

// ensurePersona retorna a persona ativa; se nao houver, cria a default (Tony/Crow).
func (s *Service) ensurePersona(ctx context.Context, a Automation) (Persona, error) {
	p, err := s.store.GetActivePersona(ctx, a.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return s.store.CreatePersona(ctx, a.ID, a.AccountID, defaultName, defaultPersona, true)
	}
	return p, err
}

// liveStatus le o estado na WAHA e persiste; em erro, cai no ultimo persistido.
func (s *Service) liveStatus(ctx context.Context, ch Channel) (status, phone string) {
	status, phone, err := s.waha.Status(ctx, ch.SessionName)
	if err != nil {
		current := ch.Status
		if ch.ConnectedPhone != nil {
			return current, *ch.ConnectedPhone
		}
		return current, ""
	}
	_ = s.store.UpdateChannelStatus(ctx, ch.ID, status, phone)
	return status, phone
}

func buildWhatsAppView(ch Channel, status, phone string) WhatsAppView {
	return WhatsAppView{
		Provider:       ch.Provider,
		SessionName:    ch.SessionName,
		Status:         status,
		Connected:      status == statusWorking,
		ConnectedPhone: phone,
	}
}

const statusWorking = "WORKING"
