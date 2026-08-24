package calendar

import (
	"context"
	"strings"
	"time"
)

// Tetos das projecoes lean do agregado de contexto (contrato C9/C7). Mantem o
// payload enxuto para nao estourar a janela de contexto das IAs.
const (
	maxContextEvents = 100
	maxContextPlans  = 10
	maxContextTasks  = 100
	// Espelha o teto autoritativo de Tasks para que nenhum item persistido fique
	// impossivel de localizar/alterar pelo Crow apenas por sua posicao no checklist.
	maxContextTaskItems = 200
	// maxContextMediaPerEvent mantem o prompt e o snapshot do chat limitados sem esconder
	// que o evento possui midia; o card mostra as primeiras e sinaliza o restante recebido.
	maxContextMediaPerEvent = 6
	// maxContextClients e o teto de clientes no agregado multi-cliente do chat
	// (contrato D4, scope 'all'): quando a agencia enxerga muitos clientes, so os
	// primeiros N entram — mantem o payload token-bounded.
	maxContextClients = 30
	// maxBrandVoiceRunes trunca o brandVoice no resumo lean de cada cliente do
	// agregado multi-cliente (o perfil completo estouraria a janela com muitos clientes).
	maxBrandVoiceRunes = 280
)

// AIContext e o agregado de contexto compartilhado entre as IAs do calendario
// (contrato C9): dados que qualquer bot (chat do calendario, WhatsApp, Omni Chat)
// usa para responder sobre o mes de um cliente. As chaves batem 1:1 com o C9.
//
// REGRA DE UNIFICACAO (C7): o bloco "context" do chat = ESTE agregado SEM o campo
// Account. Os dois sao montados pela MESMA funcao (BuildAIContext) para nao
// divergir — o chat (B6) monta seu payload a partir deste struct, apenas omitindo
// Account. Nunca duplicar a montagem.
type AIContext struct {
	Account    AIContextAccount `json:"account"`
	Client     *planClient      `json:"client"` // null quando sem clientId
	Month      string           `json:"month"`
	Holidays   []Holiday        `json:"holidays"`
	MonthNotes string           `json:"monthNotes"`
	Events     []AIContextEvent `json:"events"`
	Plans      []AIContextPlan  `json:"plans"`
}

// AIContextAccount identifica a account dona do calendario no agregado.
type AIContextAccount struct {
	ID string `json:"id"`
}

// AIContextEvent e a projecao segura de um evento enviada ao chat. Alem dos campos
// textuais, leva IDs e midias reais para o LLM referenciar; o backend usa esses IDs para
// montar os cards ricos, portanto uma URL inventada pelo modelo nunca chega ao front.
type AIContextEvent struct {
	ID            string      `json:"id"`
	Version       int         `json:"version,omitempty"`
	Date          string      `json:"date"`
	Time          string      `json:"time"`
	Type          string      `json:"type"`
	Title         string      `json:"title"`
	Status        string      `json:"status"`
	Priority      string      `json:"priority"`
	ResponsibleID string      `json:"responsibleId,omitempty"`
	InvolvedIDs   []string    `json:"involvedIds,omitempty"`
	ClientID      string      `json:"clientId"`
	ClientName    string      `json:"clientName"`
	Description   string      `json:"description,omitempty"`
	TaskID        string      `json:"taskId,omitempty"`
	Media         []MediaItem `json:"media"`
}

// AIContextTask e a projecao segura das tasks do board configurado do calendario. O
// chat usa estes IDs reais para propor update/delete de task sem inventar targetId.
type AIContextTask struct {
	ID            string              `json:"id"`
	BoardID       string              `json:"boardId"`
	ColumnID      string              `json:"columnId,omitempty"`
	Title         string              `json:"title"`
	Status        string              `json:"status,omitempty"`
	Priority      string              `json:"priority"`
	DueDate       string              `json:"dueDate,omitempty"`
	StartDate     string              `json:"startDate,omitempty"`
	DueEndDate    string              `json:"dueEndDate,omitempty"`
	ResponsibleID string              `json:"responsibleId,omitempty"`
	InvolvedIDs   []string            `json:"involvedIds,omitempty"`
	ClientID      string              `json:"clientId,omitempty"`
	ClientName    string              `json:"clientName,omitempty"`
	Type          string              `json:"type,omitempty"`
	Description   string              `json:"description,omitempty"`
	Archived      bool                `json:"archived,omitempty"`
	Version       int                 `json:"version,omitempty"`
	Items         []AIContextTaskItem `json:"items,omitempty"`
}

// AIContextTaskItem e a projecao defensiva de ui_metadata.checklist usada pelo Crow.
type AIContextTaskItem struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Completed     bool   `json:"completed"`
	Status        string `json:"status,omitempty"`
	StatusDate    string `json:"statusDate,omitempty"`
	CompletedDate string `json:"completedDate,omitempty"`
}

// AIContextPlan e a projecao lean de um plano de IA (contrato C9/C7): sem o
// content gerado, so o cabecalho para a IA saber que planos existem.
type AIContextPlan struct {
	ID       string `json:"id"`
	Month    string `json:"month"`
	Status   string `json:"status"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// BuildAIContext monta o agregado de contexto compartilhado (C9) no escopo da
// account: cliente em foco (opcional, com nome + perfil C3), feriados do mes,
// nota do mes, eventos lean do mes e planos lean recentes.
//
// Reusa as queries de store_ai_context.go: planContext ja resolve nome/perfil do
// cliente (UMA query cada, sem N+1), feriados (HolidaysInRange sobre a config) e a
// nota do mes. Aqui somam-se apenas a projecao lean de eventos e de planos.
//
// accountID vem SEMPRE do Principal / token de servico (nunca do body). clientID e
// month sao opcionais: month vazio = mes corrente; clientID vazio = sem cliente
// em foco (Client == nil, eventos do mes inteiro). clientID nao-UUID e descartado
// (vira "" -> sem cliente), como no resto do modulo. O isolamento multi-tenant esta
// no filtro por account_id de todas as queries (defesa em profundidade): um
// clientID forjado de outra conta volta sem nome/perfil (loadAccountNames/loadProfiles
// amarram ao universo desta account), sem vazar dado.
func (s *Service) BuildAIContext(ctx context.Context, accountID, clientID, month string) (AIContext, error) {
	account := strings.TrimSpace(accountID)
	clientID = normalizeUUID(clientID)
	month = strings.TrimSpace(month)
	if month == "" {
		month = time.Now().Format("2006-01")
	}
	if !monthRe.MatchString(month) {
		return AIContext{}, ErrInvalidDate
	}

	clientIDs := []string{}
	if clientID != "" {
		clientIDs = []string{clientID}
	}
	// planContext monta cliente(s) + feriados + nota do mes reusando o mesmo codigo
	// do disparo de plano (sem N+1). Com clientIDs vazio: Clients vazio (Client nil),
	// mas feriados e nota do mes continuam vindo.
	pc, err := s.store.planContext(ctx, account, month, clientIDs)
	if err != nil {
		return AIContext{}, err
	}

	from, to, err := monthBounds(month)
	if err != nil {
		return AIContext{}, err
	}
	// Eventos do mes filtrados pelo cliente em foco (quando informado): espelha o
	// contexto da tela (cliente filtrado + mes) e mantem o payload lean.
	events, err := s.store.ListEventsLean(ctx, account, from, to, clientID, maxContextEvents)
	if err != nil {
		return AIContext{}, err
	}
	clientNames := map[string]string{}
	for _, c := range pc.Clients {
		clientNames[c.ID] = c.Name
	}
	setContextClientNames(events, clientNames)
	plans, err := s.leanPlans(ctx, account)
	if err != nil {
		return AIContext{}, err
	}

	return AIContext{
		Account:    AIContextAccount{ID: account},
		Client:     firstClient(pc.Clients),
		Month:      month,
		Holidays:   pc.Holidays,
		MonthNotes: pc.MonthNote,
		Events:     events,
		Plans:      plans,
	}, nil
}

// AIContextClientLean e a projecao ENXUTA de um cliente no agregado multi-cliente do
// chat (contrato D4, scope 'all'): so id/nome/segmento + um resumo do brandVoice. Os
// campos pesados do perfil (positioning/description/history/objectives/extra) ficam de
// fora — com muitos clientes eles estourariam a janela de contexto da IA. ProfileMissing
// (WAVE 7) leva SO os NOMES dos campos estaveis ainda vazios (token-cheap) para a IA saber
// o que pedir/insistir mesmo no escopo 'all', onde o perfil completo nao viaja.
type AIContextClientLean struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Segment        string   `json:"segment"`
	BrandVoice     string   `json:"brandVoice"`
	ProfileMissing []string `json:"profileMissing,omitempty"`
}

// missingProfileFields devolve as CHAVES dos campos estaveis do perfil ainda vazios (contrato C3),
// na ordem C3, para a IA saber o que pedir e insistir (WAVE 7). So os estaveis (o extra e detalhe;
// no escopo 'client' o perfil completo ja viaja e a IA ve os vazios do extra tambem).
func missingProfileFields(p planProfile) []string {
	pairs := []struct{ key, val string }{
		{"segment", p.Segment},
		{"positioning", p.Positioning},
		{"description", p.Description},
		{"history", p.History},
		{"siteUrl", p.SiteURL},
		{"instagram", p.Instagram},
		{"address", p.Address},
		{"objectives", p.Objectives},
		{"brandVoice", p.BrandVoice},
	}
	missing := make([]string, 0, len(pairs))
	for _, f := range pairs {
		if strings.TrimSpace(f.val) == "" {
			missing = append(missing, f.key)
		}
	}
	return missing
}

// AIContextAll e o agregado LEAN multi-cliente do chat em escopo 'all' (contrato D4):
// um resumo de cada cliente visivel (teto maxContextClients) + feriados/eventos/nota do
// mes de TODOS os clientes. E usado SO pelo chat (nao ha runtime 'all'); por isso ja sai
// SEM o campo account (o chat omite account, regra de unificacao C7). Scope marca 'all'
// para a IA saber que o contexto e multi-cliente (diferente do bloco client do C7).
type AIContextAll struct {
	Scope   string                `json:"scope"` // sempre "all"
	Month   string                `json:"month"`
	Clients []AIContextClientLean `json:"clients"`
	// Client (WAVE 16): perfil COMPLETO do cliente CITADO na pergunta, hidratado sob demanda
	// (appendNamedClientProfile). No escopo 'all' os clientes vao ENXUTOS (nome/segmento/tom +
	// campos vazios) para nao estourar o contexto com muitos clientes — mas "traz os dados do
	// cliente X" precisa do perfil inteiro. Quando setado, o workflow renderiza "Cliente em
	// foco" com todos os campos (mesmo bloco do escopo 'client'), sem trocar o escopo nem o
	// workflow. nil = ninguem citado (contexto segue leve). MESMO json:"client" do escopo single.
	Client     *planClient      `json:"client,omitempty"`
	Holidays   []Holiday        `json:"holidays"`
	MonthNotes string           `json:"monthNotes"`
	Events     []AIContextEvent `json:"events"`
	Tasks      []AIContextTask  `json:"tasks,omitempty"`
	// People (WAVE 12): pessoas da equipe (id+nome) para a IA resolver responsavel/
	// envolvidos por NOME (mesma fonte do GET /responsibles), sem exigir ID do usuario.
	People            []Member              `json:"people,omitempty"`
	ContentOperations any                   `json:"contentOperations,omitempty"`
	Capabilities      []AssistantCapability `json:"capabilities,omitempty"`
	MetaAds           any                   `json:"metaAds,omitempty"`
	Resources         []AssistantResource   `json:"resources,omitempty"`
}

// BuildAIContextAll monta o agregado LEAN multi-cliente (contrato D4) para o chat em
// escopo 'all': resumo de cada cliente VISIVEL (teto maxContextClients) + feriados, nota
// e eventos do mes de TODOS os clientes (teto maxContextEvents). visibleClientIDs vem do
// acesso resolvido server-side (nunca do body): so entram clientes que o usuario pode ver.
//
// Reusa planContext (nomes + perfis em UMA query cada, sem N+1) e ListEventsLean (sem
// cliente => todos), a MESMA fonte do BuildAIContext single. O isolamento multi-tenant
// esta no filtro por account_id de todas as queries; um clientID forjado nem chega aqui
// (a lista ja vem permission-scoped). Token-bounded: teto de clientes/eventos + brandVoice
// truncado.
func (s *Service) BuildAIContextAll(ctx context.Context, accountID string, visibleClientIDs []string, month string) (AIContextAll, error) {
	account := strings.TrimSpace(accountID)
	month = strings.TrimSpace(month)
	if month == "" {
		month = time.Now().Format("2006-01")
	}
	if !monthRe.MatchString(month) {
		return AIContextAll{}, ErrInvalidDate
	}
	clientIDs := capClientIDs(visibleClientIDs, maxContextClients)
	// planContext resolve nome + perfil de cada cliente (UMA query cada) + feriados e nota
	// do mes. Com clientIDs vazio: Clients vazio, mas feriados/nota continuam vindo.
	pc, err := s.store.planContext(ctx, account, month, clientIDs)
	if err != nil {
		return AIContextAll{}, err
	}
	clients := make([]AIContextClientLean, 0, len(pc.Clients))
	for _, c := range pc.Clients {
		clients = append(clients, AIContextClientLean{
			ID:             c.ID,
			Name:           c.Name,
			Segment:        c.Profile.Segment,
			BrandVoice:     truncateRunes(c.Profile.BrandVoice, maxBrandVoiceRunes),
			ProfileMissing: missingProfileFields(c.Profile),
		})
	}
	from, to, err := monthBounds(month)
	if err != nil {
		return AIContextAll{}, err
	}
	// Eventos lean do mes SO dos clientes VISIVEIS (WAVE 4: fecha vazamento de evento de
	// cliente fora do escopo do usuario no modo 'all') + eventos gerais sem cliente. Teto
	// maxContextEvents. clientIDs ja veio capado/normalizado acima.
	events, err := s.store.ListEventsLeanForClients(ctx, account, from, to, clientIDs, maxContextEvents)
	if err != nil {
		return AIContextAll{}, err
	}
	clientNames := make(map[string]string, len(pc.Clients))
	for _, c := range pc.Clients {
		clientNames[c.ID] = c.Name
	}
	setContextClientNames(events, clientNames)
	return AIContextAll{
		Scope:      chatScopeAll,
		Month:      month,
		Clients:    clients,
		Holidays:   pc.Holidays,
		MonthNotes: pc.MonthNote,
		Events:     events,
	}, nil
}

// fillLeanClientNames completa o nome dos clientes lean do contexto multi-cliente a partir
// da lista AUTORIZADA de clientes visiveis (mesma fonte do select de escopo). loadAccountNames
// so nomeia cliente com evento/perfil; sem isto um cliente visivel SEM evento/perfil viajava
// com name vazio e a IA nao conseguia cita-lo (bug: "faltou Duby/Bari"). Nao afrouxa a trava de
// enumeracao: esses nomes ja sao exibidos no select de escopo (permission-scoped). So preenche
// o que veio vazio (nome do banco, quando existe, vence).
func fillLeanClientNames(clients []AIContextClientLean, nameByID map[string]string) {
	if len(clients) == 0 || len(nameByID) == 0 {
		return
	}
	for i := range clients {
		if strings.TrimSpace(clients[i].Name) != "" {
			continue
		}
		if name := strings.TrimSpace(nameByID[clients[i].ID]); name != "" {
			clients[i].Name = name
		}
	}
}

// setContextClientNames preenche o nome do cliente em cada evento do contexto da IA.
// WAVE 13: a midia (propria + espelhada da task) ja vem da query de eventos
// (scanAIContextEvent une events.media + linked_media); nao ha mais day_media a unir aqui.
func setContextClientNames(events []AIContextEvent, clientNames map[string]string) {
	for i := range events {
		events[i].ClientName = clientNames[events[i].ClientID]
	}
}

func appendContextMedia(items []MediaItem, candidate MediaItem) []MediaItem {
	if strings.TrimSpace(candidate.URL) == "" {
		return items
	}
	for _, item := range items {
		if (candidate.ID != "" && item.ID == candidate.ID) || item.URL == candidate.URL {
			return items
		}
	}
	if len(items) >= maxContextMediaPerEvent {
		return items
	}
	return append(items, candidate)
}

// capClientIDs normaliza (UUID) e limita a lista de clientes ao teto, preservando a
// ordem e removendo duplicados (token-bounded, contrato D4). max <= 0 = sem teto.
func capClientIDs(ids []string, max int) []string {
	out := make([]string, 0, len(ids))
	seen := map[string]bool{}
	for _, raw := range ids {
		id := normalizeUUID(raw)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
		if max > 0 && len(out) >= max {
			break
		}
	}
	return out
}

// truncateRunes corta uma string em `max` runes (respeita multibyte) e sinaliza o corte
// com reticencias, mantendo o payload token-bounded. max <= 0 = sem corte.
func truncateRunes(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 {
		return s
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "..."
}

// leanPlans devolve os planos mais recentes da account (todos os meses) na
// projecao lean do contrato C9/C7, com teto de maxContextPlans. Reusa o indice
// lean ListAIPlans (ja sem o content), ordenado por created_at desc no store.
func (s *Service) leanPlans(ctx context.Context, accountID string) ([]AIContextPlan, error) {
	items, err := s.store.ListAIPlans(ctx, accountID, "")
	if err != nil {
		return nil, err
	}
	out := make([]AIContextPlan, 0, maxContextPlans)
	for _, it := range items {
		if len(out) >= maxContextPlans {
			break
		}
		out = append(out, AIContextPlan{
			ID:       it.ID,
			Month:    it.Month,
			Status:   it.Status,
			Provider: it.Provider,
			Model:    it.Model,
		})
	}
	return out, nil
}

// firstClient devolve o primeiro cliente do planContext como ponteiro (null no
// JSON quando nao ha cliente em foco). planClient ja e o shape do cliente do C9/C7
// (id + nome + perfil C3 sem clientId), reusado para nao duplicar tipo.
func firstClient(clients []planClient) *planClient {
	if len(clients) == 0 {
		return nil
	}
	c := clients[0]
	return &c
}
