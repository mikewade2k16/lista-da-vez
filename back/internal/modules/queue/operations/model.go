package operations

import (
	"context"
	"time"
)

type ConsultantProfile struct {
	ID             string
	StoreID        string
	Name           string
	Role           string
	Initials       string
	Color          string
	MonthlyGoal    float64
	CommissionRate float64
	ConversionGoal float64
	AvgTicketGoal  float64
	PAGoal         float64
}

// GoalStats e o atingimento de meta CANONICO do consultor, originado do CRM/ERP
// (`/v1/erp/crm`, campo goalProgress). O snapshot da operacao faz a PONTE: busca
// esse numero server-side (sem passar pelo gate canViewERP) e embute por consultor
// para que TODO operador veja (decisao "todos veem de todos"). Valores em reais.
type GoalStats struct {
	MonthlyGoal     float64 `json:"monthlyGoal"`     // reais
	SoldValue       float64 `json:"soldValue"`       // reais (atingido)
	RemainingToGoal float64 `json:"remainingToGoal"` // reais (falta)
	Progress        float64 `json:"progress"`        // %, pode passar de 100
	HasGoal         bool    `json:"hasGoal"`
}

// GoalProgressProvider entrega o atingimento de meta por consultor do mes, vindo
// do CRM/ERP. A chave do map e o ID do consultor de PERFIL (mesmo `consultant.ID`
// do roster da operacao / `person.id` do front). `month` no formato "YYYY-MM".
// O adapter que cumpre esta interface vive na composition root, para que
// `operations` NAO importe `crm/erp` (sem ciclo de import). E nil-safe: quando o
// provider e nil ou retorna erro, o snapshot degrada com GoalStats=nil.
type GoalProgressProvider interface {
	GoalStatsByConsultant(ctx context.Context, tenantID string, month string) (map[string]GoalStats, error)
}

type QueueEntry struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Role           string     `json:"role"`
	Initials       string     `json:"initials"`
	Color          string     `json:"color"`
	MonthlyGoal    float64    `json:"monthlyGoal,omitempty"`
	CommissionRate float64    `json:"commissionRate,omitempty"`
	QueueJoinedAt  int64      `json:"queueJoinedAt"`
	GoalStats      *GoalStats `json:"goalStats,omitempty"`
}

type SkippedPerson struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ActiveService struct {
	ID                   string          `json:"id"`
	Name                 string          `json:"name"`
	Role                 string          `json:"role"`
	Initials             string          `json:"initials"`
	Color                string          `json:"color"`
	MonthlyGoal          float64         `json:"monthlyGoal,omitempty"`
	CommissionRate       float64         `json:"commissionRate,omitempty"`
	ServiceID            string          `json:"serviceId"`
	ServiceStartedAt     int64           `json:"serviceStartedAt"`
	QueueJoinedAt        int64           `json:"queueJoinedAt"`
	QueueWaitMs          int64           `json:"queueWaitMs"`
	QueuePositionAtStart *int            `json:"queuePositionAtStart,omitempty"`
	StartMode            string          `json:"startMode"`
	SkippedPeople        []SkippedPerson `json:"skippedPeople"`
	ParallelGroupID      string          `json:"parallelGroupId,omitempty"`
	ParallelStartIndex   *int            `json:"parallelStartIndex,omitempty"`
	SiblingServiceIDs    []string        `json:"siblingServiceIds"`
	StartOffsetMs        int64           `json:"startOffsetMs"`
	StoppedAt            int64           `json:"stoppedAt,omitempty"`
	EffectiveFinishedAt  int64           `json:"effectiveFinishedAt,omitempty"`
	StopReason           string          `json:"stopReason,omitempty"`
	// Auto-encerramento (2h): GraceDeadline (>0) = epoch ms ABSOLUTO em que o
	// countdown de 1 min vence; a barra do front encolhe ate ele (comparado contra
	// adjustedNow, nunca Date.now). SnoozedUntil = ate quando o "Continuar" adia.
	GraceDeadline int64      `json:"graceDeadline,omitempty"`
	SnoozedUntil  int64      `json:"snoozedUntil,omitempty"`
	SnoozeCount   int        `json:"snoozeCount,omitempty"`
	GoalStats     *GoalStats `json:"goalStats,omitempty"`
}

type PausedEmployee struct {
	PersonID  string `json:"personId"`
	Reason    string `json:"reason"`
	Kind      string `json:"kind,omitempty"`
	StartedAt int64  `json:"startedAt"`
}

type ConsultantSession struct {
	PersonID   string `json:"personId"`
	Status     string `json:"status"`
	StartedAt  int64  `json:"startedAt"`
	EndedAt    int64  `json:"endedAt"`
	DurationMs int64  `json:"durationMs"`
	// Reason/Kind so sao preenchidos quando a sessao fechada e de pausa
	// (status=paused): preservam o motivo e o tipo (pause/assignment) da pausa
	// para metrificacao, ja que operation_paused_consultants e apagado no resume.
	Reason string `json:"reason,omitempty"`
	Kind   string `json:"kind,omitempty"`
}

type ConsultantStatus struct {
	Status    string `json:"status"`
	StartedAt int64  `json:"startedAt"`
}

type ProductEntry struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Code     string  `json:"code"`
	Price    float64 `json:"price"`
	IsCustom bool    `json:"isCustom,omitempty"`
}

type ServiceHistoryEntry struct {
	ServiceID                  string            `json:"serviceId"`
	StoreID                    string            `json:"storeId"`
	StoreName                  string            `json:"storeName"`
	PersonID                   string            `json:"personId"`
	PersonName                 string            `json:"personName"`
	StartedAt                  int64             `json:"startedAt"`
	FinishedAt                 int64             `json:"finishedAt"`
	DurationMs                 int64             `json:"durationMs"`
	FinishOutcome              string            `json:"finishOutcome"`
	StartMode                  string            `json:"startMode"`
	QueuePositionAtStart       *int              `json:"queuePositionAtStart,omitempty"`
	QueueWaitMs                int64             `json:"queueWaitMs"`
	SkippedPeople              []SkippedPerson   `json:"skippedPeople"`
	SkippedCount               int               `json:"skippedCount"`
	ParallelGroupID            string            `json:"parallelGroupId,omitempty"`
	ParallelStartIndex         *int              `json:"parallelStartIndex,omitempty"`
	SiblingServiceIDs          []string          `json:"siblingServiceIds"`
	StartOffsetMs              int64             `json:"startOffsetMs"`
	IsWindowService            bool              `json:"isWindowService"`
	IsGift                     bool              `json:"isGift"`
	ProductSeen                string            `json:"productSeen"`
	ProductClosed              string            `json:"productClosed"`
	PurchaseCode               string            `json:"purchaseCode"`
	ProductDetails             string            `json:"productDetails"`
	ProductsSeen               []ProductEntry    `json:"productsSeen"`
	ProductsClosed             []ProductEntry    `json:"productsClosed"`
	ProductsNotFound           []ProductEntry    `json:"productsNotFound"`
	ProductsSeenNone           bool              `json:"productsSeenNone"`
	VisitReasonsNotInformed    bool              `json:"visitReasonsNotInformed"`
	CustomerSourcesNotInformed bool              `json:"customerSourcesNotInformed"`
	CustomerName               string            `json:"customerName"`
	CustomerPhone              string            `json:"customerPhone"`
	CustomerEmail              string            `json:"customerEmail"`
	IsExistingCustomer         bool              `json:"isExistingCustomer"`
	VisitReasons               []string          `json:"visitReasons"`
	VisitReasonDetails         map[string]string `json:"visitReasonDetails"`
	CustomerSources            []string          `json:"customerSources"`
	CustomerSourceDetails      map[string]string `json:"customerSourceDetails"`
	LossReasons                []string          `json:"lossReasons"`
	LossReasonDetails          map[string]string `json:"lossReasonDetails"`
	LossReasonID               string            `json:"lossReasonId"`
	LossReason                 string            `json:"lossReason"`
	SaleAmount                 float64           `json:"saleAmount"`
	CustomerProfession         string            `json:"customerProfession"`
	QueueJumpReason            string            `json:"queueJumpReason"`
	CancelReason               string            `json:"cancelReason"`
	StopReason                 string            `json:"stopReason"`
	Notes                      string            `json:"notes"`
	CampaignMatches            []CampaignMatch   `json:"campaignMatches"`
	CampaignBonusTotal         float64           `json:"campaignBonusTotal"`
	// Auto-encerramento (2h): CloseReason='auto' e ValidationStatus='pending' quando
	// o sweep fechou; o gerente valida (validated + outcome real) ou cancela
	// (cancelled + CancelReason). ValidatedBy/At = auditoria. SnoozeCount = adiamentos.
	CloseReason      string `json:"closeReason,omitempty"`
	ValidationStatus string `json:"validationStatus,omitempty"`
	ValidatedBy      string `json:"validatedBy,omitempty"`
	ValidatedAt      int64  `json:"validatedAt,omitempty"`
	SnoozeCount      int    `json:"snoozeCount,omitempty"`
	// ValidationReason: justificativa (obrigatoria) registrada pela gestao ao
	// encerrar uma pendencia — por que o consultor nao encerrou na hora. Base das
	// metricas de cobranca por consultor/gerente/loja.
	ValidationReason string `json:"validationReason,omitempty"`
}

type CampaignMatch struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	BonusAmount float64 `json:"bonusAmount"`
}

// RosterMember e a projecao ENXUTA do consultor exposta dentro do snapshot da
// operacao. So carrega o que a faixa de consultores precisa para operar a fila
// (id/nome/iniciais/cor/papel); NUNCA inclui meta, comissao ou e-mail de acesso.
// Assim qualquer papel que pode ler a operacao (consultor/terminal/gerente)
// recebe a faixa sem precisar da permissao de gestao de consultores
// (`/v1/consultants`), que continua restrita.
type RosterMember struct {
	ID       string `json:"id"`
	StoreID  string `json:"storeId,omitempty"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Initials string `json:"initials"`
	Color    string `json:"color"`
}

type Snapshot struct {
	StoreID                    string                      `json:"storeId"`
	Roster                     []RosterMember              `json:"roster"`
	WaitingList                []QueueEntry                `json:"waitingList"`
	ActiveServices             []ActiveService             `json:"activeServices"`
	PausedEmployees            []PausedEmployee            `json:"pausedEmployees"`
	ConsultantActivitySessions []ConsultantSession         `json:"consultantActivitySessions"`
	ConsultantCurrentStatus    map[string]ConsultantStatus `json:"consultantCurrentStatus"`
	ServiceHistory             []ServiceHistoryEntry       `json:"serviceHistory"`
	// PendingValidations sao os atendimentos auto-encerrados (close_reason='auto')
	// aguardando o gerente validar/cancelar. Derivado do historico
	// (validation_status='pending'); array proprio porque o servico ja saiu de
	// operation_active_services no auto-close.
	PendingValidations []PendingValidation `json:"pendingValidations"`
	// ServerTime e o relogio do servidor no momento da resposta. O front usa para
	// re-sincronizar o serverClockOffsetMs a cada leitura ao vivo (nao so no ack de
	// mutacao), evitando que a sessao que apenas observa drifte o timer ate o reload.
	ServerTime time.Time `json:"serverTime"`
}

// PendingValidation e um atendimento encerrado automaticamente (2h) aguardando
// validacao do gerente. O cronometro esta PARADO: DurationMs e fixo (fechamento
// menos inicio), sem contador ao vivo no front.
type PendingValidation struct {
	ServiceID    string `json:"serviceId"`
	StoreID      string `json:"storeId"`
	StoreName    string `json:"storeName,omitempty"`
	PersonID     string `json:"personId"`
	PersonName   string `json:"personName"`
	StartedAt    int64  `json:"startedAt"`
	FinishedAt   int64  `json:"finishedAt"`
	AutoClosedAt int64  `json:"autoClosedAt"`
	DurationMs   int64  `json:"durationMs"`
	SnoozeCount  int    `json:"snoozeCount"`
}

type SnapshotState struct {
	StoreID                    string
	WaitingList                []QueueStateItem
	ActiveServices             []ActiveServiceState
	PausedEmployees            []PausedStateItem
	ConsultantActivitySessions []ConsultantSession
	ConsultantCurrentStatus    map[string]ConsultantStatus
	ServiceHistory             []ServiceHistoryEntry
}

type QueueStateItem struct {
	ConsultantID  string
	QueueJoinedAt int64
}

type ActiveServiceState struct {
	ConsultantID         string
	ServiceID            string
	ServiceStartedAt     int64
	QueueJoinedAt        int64
	QueueWaitMs          int64
	QueuePositionAtStart *int
	StartMode            string
	SkippedPeople        []SkippedPerson
	ParallelGroupID      string
	ParallelStartIndex   *int
	SiblingServiceIDs    []string
	StartOffsetMs        int64
	StoppedAt            int64
	StopReason           string
	// Auto-encerramento (2h): estado corrente do countdown/adiamento persistido em
	// queue.operation_active_services. GraceDeadline = epoch ms absoluto do vencimento
	// da barra (0 = sem countdown). SnoozedUntil = ate quando o "Continuar" adia.
	GraceDeadline int64
	SnoozedUntil  int64
	SnoozeCount   int
}

type PausedStateItem struct {
	ConsultantID string
	Reason       string
	Kind         string
	StartedAt    int64
}

type OperationOverviewStore struct {
	StoreID        string `json:"storeId"`
	AccountID      string `json:"accountId"`
	StoreName      string `json:"storeName"`
	StoreCode      string `json:"storeCode,omitempty"`
	City           string `json:"city,omitempty"`
	WaitingCount   int    `json:"waitingCount"`
	ActiveCount    int    `json:"activeCount"`
	PausedCount    int    `json:"pausedCount"`
	AvailableCount int    `json:"availableCount"`
}

type OperationOverviewPerson struct {
	StoreID              string          `json:"storeId"`
	StoreName            string          `json:"storeName"`
	StoreCode            string          `json:"storeCode,omitempty"`
	PersonID             string          `json:"personId"`
	Name                 string          `json:"name"`
	Role                 string          `json:"role"`
	Initials             string          `json:"initials"`
	Color                string          `json:"color"`
	MonthlyGoal          float64         `json:"monthlyGoal,omitempty"`
	CommissionRate       float64         `json:"commissionRate,omitempty"`
	Status               string          `json:"status"`
	StatusStartedAt      int64           `json:"statusStartedAt"`
	QueueJoinedAt        int64           `json:"queueJoinedAt,omitempty"`
	QueuePosition        int             `json:"queuePosition,omitempty"`
	ServiceID            string          `json:"serviceId,omitempty"`
	ServiceStartedAt     int64           `json:"serviceStartedAt,omitempty"`
	QueueWaitMs          int64           `json:"queueWaitMs,omitempty"`
	QueuePositionAtStart *int            `json:"queuePositionAtStart,omitempty"`
	StartMode            string          `json:"startMode,omitempty"`
	SkippedPeople        []SkippedPerson `json:"skippedPeople,omitempty"`
	ParallelGroupID      string          `json:"parallelGroupId,omitempty"`
	ParallelStartIndex   *int            `json:"parallelStartIndex,omitempty"`
	SiblingServiceIDs    []string        `json:"siblingServiceIds,omitempty"`
	StartOffsetMs        int64           `json:"startOffsetMs,omitempty"`
	StoppedAt            int64           `json:"stoppedAt,omitempty"`
	EffectiveFinishedAt  int64           `json:"effectiveFinishedAt,omitempty"`
	StopReason           string          `json:"stopReason,omitempty"`
	PauseReason          string          `json:"pauseReason,omitempty"`
	PauseKind            string          `json:"pauseKind,omitempty"`
	GoalStats            *GoalStats      `json:"goalStats,omitempty"`
}

type OperationOverview struct {
	Scope                string                    `json:"scope"`
	Stores               []OperationOverviewStore  `json:"stores"`
	WaitingList          []OperationOverviewPerson `json:"waitingList"`
	ActiveServices       []OperationOverviewPerson `json:"activeServices"`
	PausedEmployees      []OperationOverviewPerson `json:"pausedEmployees"`
	AvailableConsultants []OperationOverviewPerson `json:"availableConsultants"`
	// PendingValidations: auto-encerramentos (2h) aguardando validacao da gestao,
	// AGREGADOS de todas as lojas acessiveis (a caixa de Pendencias funciona tambem
	// na visao "Todas as lojas").
	PendingValidations []PendingValidation `json:"pendingValidations"`
	// ServerTime: relogio do servidor na resposta, para o front re-sincronizar o
	// serverClockOffsetMs a cada refresh ao vivo (ver Snapshot.ServerTime).
	ServerTime time.Time `json:"serverTime"`
}

type PersistInput struct {
	StoreID          string
	WaitingList      []QueueStateItem
	ActiveServices   []ActiveServiceState
	PausedEmployees  []PausedStateItem
	CurrentStatus    map[string]ConsultantStatus
	AppendedSessions []ConsultantSession
	AppendedHistory  []ServiceHistoryEntry
}

type QueueCommandInput struct {
	StoreID  string `json:"storeId"`
	PersonID string `json:"personId"`
}

type PauseCommandInput struct {
	StoreID  string `json:"storeId"`
	PersonID string `json:"personId"`
	Reason   string `json:"reason"`
}

type AssignTaskCommandInput struct {
	StoreID  string `json:"storeId"`
	PersonID string `json:"personId"`
	Reason   string `json:"reason"`
}

type StartCommandInput struct {
	StoreID  string `json:"storeId"`
	PersonID string `json:"personId"`
}

type StartParallelCommandInput struct {
	StoreID  string `json:"storeId"`
	PersonID string `json:"personId"`
}

type FinishCommandInput struct {
	StoreID                    string            `json:"storeId"`
	ServiceID                  string            `json:"serviceId"`
	PersonID                   string            `json:"personId"`
	Action                     string            `json:"action"`
	Outcome                    string            `json:"outcome"`
	IsWindowService            bool              `json:"isWindowService"`
	IsGift                     bool              `json:"isGift"`
	ProductSeen                string            `json:"productSeen"`
	ProductClosed              string            `json:"productClosed"`
	PurchaseCode               string            `json:"purchaseCode"`
	ProductDetails             string            `json:"productDetails"`
	ProductsSeen               []ProductEntry    `json:"productsSeen"`
	ProductsClosed             []ProductEntry    `json:"productsClosed"`
	ProductsNotFound           []ProductEntry    `json:"productsNotFound"`
	ProductsSeenNone           bool              `json:"productsSeenNone"`
	VisitReasonsNotInformed    bool              `json:"visitReasonsNotInformed"`
	CustomerSourcesNotInformed bool              `json:"customerSourcesNotInformed"`
	CustomerName               string            `json:"customerName"`
	CustomerPhone              string            `json:"customerPhone"`
	CustomerEmail              string            `json:"customerEmail"`
	IsExistingCustomer         bool              `json:"isExistingCustomer"`
	VisitReasons               []string          `json:"visitReasons"`
	VisitReasonDetails         map[string]string `json:"visitReasonDetails"`
	CustomerSources            []string          `json:"customerSources"`
	CustomerSourceDetails      map[string]string `json:"customerSourceDetails"`
	LossReasons                []string          `json:"lossReasons"`
	LossReasonDetails          map[string]string `json:"lossReasonDetails"`
	LossReasonID               string            `json:"lossReasonId"`
	LossReason                 string            `json:"lossReason"`
	SaleAmount                 float64           `json:"saleAmount"`
	CustomerProfession         string            `json:"customerProfession"`
	QueueJumpReason            string            `json:"queueJumpReason"`
	CancelReason               string            `json:"cancelReason"`
	StopReason                 string            `json:"stopReason"`
	Notes                      string            `json:"notes"`
	CampaignMatches            []CampaignMatch   `json:"campaignMatches"`
	CampaignBonusTotal         float64           `json:"campaignBonusTotal"`
	// ValidationReason so e usada pelo POST /v1/operations/validate (encerramento de
	// pendencia pela gestao): justificativa OBRIGATORIA de por que o consultor nao
	// encerrou na hora. Ignorada pelo /finish normal.
	ValidationReason string `json:"validationReason"`
}

// KeepOpenCommandInput e o "Continuar atendimento": adia o auto-encerramento (2h)
// por mais uma janela de re-pergunta (snooze) e limpa o countdown corrente.
type KeepOpenCommandInput struct {
	StoreID   string `json:"storeId"`
	ServiceID string `json:"serviceId"`
}

// CancelMetricCommandInput cancela a metrica de uma pendencia auto-encerrada
// (fora da metrica, preservada para auditoria). Motivo obrigatorio.
type CancelMetricCommandInput struct {
	StoreID   string `json:"storeId"`
	ServiceID string `json:"serviceId"`
	Reason    string `json:"reason"`
}

type Repository interface {
	StoreExists(ctx context.Context, storeID string) (bool, error)
	StoreExistsIncludingArchived(ctx context.Context, storeID string) (bool, error)
	GetStoreName(ctx context.Context, storeID string) (string, error)
	// GetStoreTenantID devolve o tenant_id (= id da account) dono da loja. Usado para
	// resolver o escopo do ERP quando o principal nao traz account/tenant (ex.:
	// platform_admin em rota que so usa RequireAuth, sem X-Account-Id).
	GetStoreTenantID(ctx context.Context, storeID string) (string, error)
	GetMaxConcurrentServices(ctx context.Context, storeID string) (int, error)
	GetMaxConcurrentServicesPerConsultant(ctx context.Context, storeID string) (int, error)
	ListStoresWithActiveServices(ctx context.Context) ([]string, error)
	ListStoresWithActiveServicesByTenant(ctx context.Context, tenantID string) ([]string, error)
	ListRoster(ctx context.Context, storeID string) ([]ConsultantProfile, error)
	// EffectiveMonthlyGoalByConsultant resolve a meta mensal CANONICA por consultor
	// (queue.operation_goal_targets) para o mes informado: meta individual quando
	// existe, senao a meta da loja como fallback. Chave = consultant.ID (mesmo id do
	// roster/snapshot). Independente do ERP, entao funciona ate em loja sem ERP.
	EffectiveMonthlyGoalByConsultant(ctx context.Context, storeIDs []string, month time.Time) (map[string]float64, error)
	LoadSnapshot(ctx context.Context, storeID string) (SnapshotState, error)
	Persist(ctx context.Context, input PersistInput) error
	// ValidateAutoClose promove uma pendencia (validation_status='pending') a validada,
	// gravando o desfecho real + dados do modal de fechamento + auditoria, preservando
	// os campos imutaveis de tempo. ErrPendingNotFound quando nao ha linha pendente.
	ValidateAutoClose(ctx context.Context, storeID string, entry ServiceHistoryEntry, validatedBy string, validatedAt int64) error
	// CancelAutoClose marca a pendencia como cancelled (fora da metrica, preservada
	// para auditoria) com motivo. ErrPendingNotFound quando nao ha linha pendente.
	CancelAutoClose(ctx context.Context, storeID string, serviceID string, cancelReason string, validatedBy string, validatedAt int64) error
}

type PublishedEvent struct {
	StoreID  string
	Action   string
	PersonID string
	SavedAt  time.Time
}

type EventPublisher interface {
	PublishOperationEvent(ctx context.Context, event PublishedEvent)
}

type MutationAck struct {
	OK        bool      `json:"ok"`
	StoreID   string    `json:"storeId"`
	SavedAt   time.Time `json:"savedAt"`
	Action    string    `json:"action,omitempty"`
	PersonID  string    `json:"personId,omitempty"`
	ServiceID string    `json:"serviceId,omitempty"`
}
