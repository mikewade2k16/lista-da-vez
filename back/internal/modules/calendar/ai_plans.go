package calendar

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"
)

// Status possiveis de um plano de IA (contrato C4). O ciclo de vida e:
// pending -> done | error (via callback do n8n); done -> applied (via painel).
const (
	planStatusPending = "pending"
	planStatusDone    = "done"
	planStatusError   = "error"
	planStatusApplied = "applied"
)

// planContentTypes sao os tipos de postagem aceitos nos dias do plano; fora do
// conjunto cai em "post" na normalizacao do content.
var planContentTypes = map[string]bool{"post": true, "story": true, "reels": true}

// AIPlan e a linha persistida (schema calendar.ai_plans). account_id = dona do
// calendario (Principal). Content so e preenchido quando status = done/applied.
type AIPlan struct {
	ID        string
	AccountID string
	Month     string
	ClientIDs []string
	Status    string
	Provider  string
	Model     string
	Content   AIPlanContent
	Error     string
	CreatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// AIPlanContent e o conteudo gerado pela IA (contrato C4.content, espelhado no
// n8n e no front). summary + pilares + ideias por cliente/dia.
type AIPlanContent struct {
	Summary string         `json:"summary"`
	Pillars []AIPlanPillar `json:"pillars"`
	Clients []AIPlanClient `json:"clients"`
}

// AIPlanPillar e um pilar de conteudo (nome + proporcao + racional).
type AIPlanPillar struct {
	Name       string `json:"name"`
	Proportion string `json:"proportion"`
	Rationale  string `json:"rationale"`
}

// AIPlanClient agrupa a estrategia e as ideias de um cliente no plano.
type AIPlanClient struct {
	ClientID   string      `json:"clientId"`
	ClientName string      `json:"clientName"`
	Strategy   string      `json:"strategy"`
	Days       []AIPlanDay `json:"days"`
}

// AIPlanDay e uma ideia de postagem para um dia (data + tipo + ideia + copy).
type AIPlanDay struct {
	Date string `json:"date"`
	Type string `json:"type"` // post|story|reels
	Idea string `json:"idea"`
	Copy string `json:"copy"`
}

// AIPlanView e a projecao JSON completa do plano (chaves batem com o front).
type AIPlanView struct {
	ID        string        `json:"id"`
	Month     string        `json:"month"`
	ClientIDs []string      `json:"clientIds"`
	Status    string        `json:"status"`
	Provider  string        `json:"provider"`
	Model     string        `json:"model"`
	Content   AIPlanContent `json:"content"`
	Error     string        `json:"error,omitempty"`
	CreatedAt time.Time     `json:"createdAt"`
	UpdatedAt time.Time     `json:"updatedAt"`
}

// AIPlanIndexItem e a linha lean da listagem de planos (sem o content).
type AIPlanIndexItem struct {
	ID        string    `json:"id"`
	Month     string    `json:"month"`
	ClientIDs []string  `json:"clientIds"`
	Status    string    `json:"status"`
	Provider  string    `json:"provider"`
	Model     string    `json:"model"`
	CreatedAt time.Time `json:"createdAt"`
}

// AIPlanRequest e o body do POST /ai/plan (mes + clientes escolhidos).
type AIPlanRequest struct {
	Month     string   `json:"month"`
	ClientIDs []string `json:"clientIds"`
}

// AIPlanCallback e o body do callback publico do n8n (contrato C4). status
// transiciona o plano a partir de pending; content preenchido quando done.
type AIPlanCallback struct {
	Status  string        `json:"status"`
	Content AIPlanContent `json:"content"`
	Error   string        `json:"error"`
}

func (p AIPlan) view() AIPlanView {
	ids := p.ClientIDs
	if ids == nil {
		ids = []string{}
	}
	return AIPlanView{
		ID:        p.ID,
		Month:     p.Month,
		ClientIDs: ids,
		Status:    p.Status,
		Provider:  p.Provider,
		Model:     p.Model,
		Content:   normalizePlanContent(p.Content),
		Error:     p.Error,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
}

// WithAI injeta a config de disparo do plano de IA (C5) e o logger; encadeavel no
// Build. WebhookURL vazio => ErrAINotConfigured; ServiceToken vazio => callback 503.
func (s *Service) WithAI(cfg AIDispatchConfig, logger *slog.Logger) *Service {
	s.ai = cfg
	s.logger = logger
	return s
}

// aiPlanStore e a fatia da persistencia que o Service de planos consome.
type aiPlanStore interface {
	CreateAIPlan(ctx context.Context, p AIPlan) (AIPlan, error)
	GetAIPlan(ctx context.Context, accountID, id string) (AIPlan, error)
	// GetAIPlanByID le um plano so pelo id (sem escopo de account). Usado pelo
	// callback publico, que nao tem contexto de conta (o dono e a linha).
	GetAIPlanByID(ctx context.Context, id string) (AIPlan, error)
	ListAIPlans(ctx context.Context, accountID, month string) ([]AIPlanIndexItem, error)
	// SetAIPlanResult transiciona pending -> done|error com o content/erro; so
	// altera se o status atual for pending (ErrNoRows caso contrario/inexistente).
	SetAIPlanResult(ctx context.Context, accountID, id, status string, content AIPlanContent, planErr string) (AIPlan, error)
	// MarkAIPlanApplied transiciona done -> applied; so altera se status = done.
	MarkAIPlanApplied(ctx context.Context, accountID, id string) (AIPlan, error)
	DeleteAIPlan(ctx context.Context, accountID, id string) error
	// planContext monta os insumos do payload C5 (nomes/perfis/feriados/nota).
	planContext(ctx context.Context, accountID, month string, clientIDs []string) (planContext, error)
}

// CreateAIPlan valida o pedido, cria a linha pending e dispara o n8n em goroutine.
// Sem webhook configurado => ErrAINotConfigured (503). O accountID vem do Principal
// (nunca do body); clientIds sao filtrados para UUID valido.
func (s *Service) CreateAIPlan(ctx context.Context, accountID string, req AIPlanRequest, createdBy string) (AIPlanView, error) {
	account := strings.TrimSpace(accountID)
	month := strings.TrimSpace(req.Month)
	if !monthRe.MatchString(month) {
		return AIPlanView{}, ErrInvalidDate
	}
	ids := normalizeClientIDs(req.ClientIDs)
	if len(ids) == 0 {
		return AIPlanView{}, ErrInvalidClient
	}
	if strings.TrimSpace(s.ai.WebhookURL) == "" {
		return AIPlanView{}, ErrAINotConfigured
	}
	cfg, err := s.store.GetConfig(ctx, account)
	if err != nil {
		return AIPlanView{}, err
	}
	// Kill switch + KEY CRUA do provider (SPEC-B2), resolvidos SINCRONO (como o
	// ErrAINotConfigured acima) para o usuario receber 409 ai_disabled/ai_key_missing
	// ANTES de criar a linha pending — evita plano orfao preso em pending sem key. O plano
	// usa o config GERAL da conta (WAVE 3.1: disabledClientIds filtra clientes no dispatch).
	apiKey, err := s.resolveDispatchKey(ctx, account, cfg.AI.Enabled, cfg.AI.Provider)
	if err != nil {
		return AIPlanView{}, err
	}
	plan := AIPlan{
		AccountID: account,
		Month:     month,
		ClientIDs: ids,
		Status:    planStatusPending,
		Provider:  cfg.AI.Provider,
		Model:     cfg.AI.Model,
		CreatedBy: strings.TrimSpace(createdBy),
	}
	saved, err := s.store.CreateAIPlan(ctx, plan)
	if err != nil {
		return AIPlanView{}, err
	}
	// Dispara o n8n desacoplado da request: contexto proprio (a request pode
	// terminar antes) e marca error via callback interno se o dispatch falhar. A key
	// crua ja resolvida vai no payload C5 (ai.apiKey), nunca logada.
	s.dispatchPlan(saved, cfg, apiKey)
	return saved.view(), nil
}

// GetAIPlan devolve o plano completo (com content) no escopo da account.
func (s *Service) GetAIPlan(ctx context.Context, accountID, id string) (AIPlanView, error) {
	p, err := s.store.GetAIPlan(ctx, strings.TrimSpace(accountID), strings.TrimSpace(id))
	if err != nil {
		return AIPlanView{}, mapNotFound(err)
	}
	return p.view(), nil
}

// ListAIPlans devolve o indice lean dos planos da account (opcional por mes).
func (s *Service) ListAIPlans(ctx context.Context, accountID, month string) ([]AIPlanIndexItem, error) {
	month = strings.TrimSpace(month)
	if month != "" && !monthRe.MatchString(month) {
		return nil, ErrInvalidDate
	}
	return s.store.ListAIPlans(ctx, strings.TrimSpace(accountID), month)
}

// MarkAIPlanApplied marca o plano como applied (so se estiver done). Plano em
// outro estado => ErrPlanConflict (409); fora do escopo => ErrNotFound (404). O
// GET previo (escopado por account) separa o 404 do 409 — o UPDATE guardado por
// status sozinho nao distingue "nao existe" de "status errado".
func (s *Service) MarkAIPlanApplied(ctx context.Context, accountID, id string) (AIPlanView, error) {
	account := strings.TrimSpace(accountID)
	id = strings.TrimSpace(id)
	current, err := s.store.GetAIPlan(ctx, account, id)
	if err != nil {
		return AIPlanView{}, mapNotFound(err)
	}
	if current.Status != planStatusDone {
		return AIPlanView{}, ErrPlanConflict
	}
	p, err := s.store.MarkAIPlanApplied(ctx, account, id)
	if err != nil {
		return AIPlanView{}, mapNotFound(err)
	}
	return p.view(), nil
}

// DeleteAIPlan remove um plano no escopo da account.
func (s *Service) DeleteAIPlan(ctx context.Context, accountID, id string) error {
	if err := s.store.DeleteAIPlan(ctx, strings.TrimSpace(accountID), strings.TrimSpace(id)); err != nil {
		return mapNotFound(err)
	}
	return nil
}

// ApplyPlanResult e o ponto de entrada do callback publico (n8n -> api). O
// accountID NAO vem do JWT (callback sem auth): resolvemos o plano so pelo id e o
// account dono e o gravado na linha. So transiciona a partir de pending (plano ja
// done/applied => ErrPlanConflict 409; inexistente => ErrNotFound 404). status
// invalido => ErrInvalidStatus. O GET previo separa o 404 do 409.
func (s *Service) ApplyPlanResult(ctx context.Context, id string, cb AIPlanCallback) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return ErrNotFound
	}
	status := strings.ToLower(strings.TrimSpace(cb.Status))
	if status != planStatusDone && status != planStatusError {
		return ErrInvalidStatus
	}
	current, err := s.store.GetAIPlanByID(ctx, id)
	if err != nil {
		return mapNotFound(err)
	}
	if current.Status != planStatusPending {
		return ErrPlanConflict
	}
	content := AIPlanContent{}
	if status == planStatusDone {
		content = normalizePlanContent(cb.Content)
	}
	// accountID vazio => o store resolve so pelo id (callback sem escopo de conta).
	if _, err = s.store.SetAIPlanResult(ctx, "", id, status, content, strings.TrimSpace(cb.Error)); err != nil {
		return mapNotFound(err)
	}
	// Realtime (C11): avisa o painel que o plano transicionou (o front encerra o polling
	// do modal de IA). accountID sai da linha (current), nao do callback sem escopo.
	s.publishCalendar(ctx, RealtimeEvent{
		Type: realtimePlanUpdated, AccountID: current.AccountID, ResourceID: id, Status: status,
	})
	return nil
}

// normalizeClientIDs filtra para UUID valido e remove duplicados (ordem preservada).
func normalizeClientIDs(raw []string) []string {
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

// normalizePlanContent garante shape estavel do content (arrays nunca nil, tipo
// de dia no enum post|story|reels). Aplicado na leitura e no callback.
func normalizePlanContent(c AIPlanContent) AIPlanContent {
	if c.Pillars == nil {
		c.Pillars = []AIPlanPillar{}
	}
	if c.Clients == nil {
		c.Clients = []AIPlanClient{}
	}
	for i := range c.Clients {
		if c.Clients[i].Days == nil {
			c.Clients[i].Days = []AIPlanDay{}
		}
		for j := range c.Clients[i].Days {
			t := strings.ToLower(strings.TrimSpace(c.Clients[i].Days[j].Type))
			if !planContentTypes[t] {
				t = "post"
			}
			c.Clients[i].Days[j].Type = t
		}
	}
	return c
}

// marshalContent serializa o content para jsonb (nunca nil).
func marshalContent(c AIPlanContent) []byte {
	b, err := json.Marshal(normalizePlanContent(c))
	if err != nil {
		return []byte("{}")
	}
	return b
}

// decodeContent desserializa o content do jsonb; falha/nulo -> struct zero.
func decodeContent(raw json.RawMessage) AIPlanContent {
	var c AIPlanContent
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &c)
	}
	return normalizePlanContent(c)
}
