package erp

import "time"

const (
	DataTypeItem          = "item"
	DataTypeCustomer      = "customer"
	DataTypeEmployee      = "employee"
	DataTypeOrder         = "order"
	DataTypeOrderCanceled = "ordercanceled"

	SyncModeBootstrapMarkdown = "bootstrap_markdown"
	SyncModeCSVFTP            = "csv_ftp"
	SyncStatusRunning         = "running"
	SyncStatusSucceeded       = "succeeded"
	SyncStatusFailed          = "failed"

	SyncTriggeredByManual   = "manual"
	SyncTriggeredByCron     = "cron"
	SyncTriggeredByBackfill = "backfill"

	defaultPageSize              = 50
	maxPageSize                  = 5000
	defaultCSVMaxBytes           = 128 * 1024 * 1024
	defaultManualSyncMaxFiles    = 100
	defaultBackfillMaxFiles      = 1000
	defaultManualSyncMinInterval = 5 * time.Minute
)

var supportedDataTypes = []string{
	DataTypeItem,
	DataTypeCustomer,
	DataTypeEmployee,
	DataTypeOrder,
	DataTypeOrderCanceled,
}

type Options struct {
	Env                        string
	SourceKind                 string
	SourceRecursive            bool
	SourceDir                  string
	StorageDir                 string
	BootstrapItemFile          string
	BootstrapCustomerFile      string
	BootstrapEmployeeFile      string
	BootstrapOrderFile         string
	BootstrapOrderCanceledFile string
	AllowManualSync            bool
	FTPHost                    string
	FTPPort                    int
	FTPUser                    string
	FTPPassword                string
	FTPKeyPath                 string
	FTPRemoteDir               string
	FTPHostKey                 string
	RootStoreCode              string
	SyncAutomaticEnabled       bool
	SyncInterval               time.Duration
	SyncHourUTC                int
	SyncDryRunDefault          bool
	CSVMaxBytes                int64
	ManualSyncMaxFiles         int
	BackfillMaxFiles           int
	ManualSyncMinInterval      time.Duration
}

type StoreScope struct {
	TenantID  string `json:"tenantId"`
	StoreID   string `json:"storeId"`
	StoreCode string `json:"storeCode"`
	StoreName string `json:"storeName"`
	StoreCity string `json:"storeCity,omitempty"`
	StoreCNPJ string `json:"storeCnpj,omitempty"`
}

type SyncRunSummary struct {
	ID            string     `json:"id"`
	DataType      string     `json:"dataType"`
	Mode          string     `json:"mode"`
	Status        string     `json:"status"`
	TriggeredBy   string     `json:"triggeredBy,omitempty"`
	FilesSeen     int        `json:"filesSeen"`
	FilesImported int        `json:"filesImported"`
	FilesSkipped  int        `json:"filesSkipped"`
	RowsRead      int        `json:"rowsRead"`
	RowsImported  int        `json:"rowsImported"`
	SourcePath    string     `json:"sourcePath,omitempty"`
	ErrorMessage  string     `json:"errorMessage,omitempty"`
	StartedAt     time.Time  `json:"startedAt"`
	FinishedAt    *time.Time `json:"finishedAt,omitempty"`
	StoreCNPJ     string     `json:"storeCnpj,omitempty"`
}

type SyncFileSummary struct {
	ID             string    `json:"id"`
	DataType       string    `json:"dataType"`
	SourceName     string    `json:"sourceName"`
	SourceKind     string    `json:"sourceKind"`
	ChecksumSHA256 string    `json:"checksumSha256"`
	RecordCount    int       `json:"recordCount"`
	ImportedAt     time.Time `json:"importedAt"`
	StoreCNPJ      string    `json:"storeCnpj,omitempty"`
}

type StatusResponse struct {
	Store            StoreScope       `json:"store"`
	SupportedTypes   []string         `json:"supportedTypes"`
	FunctionalTypes  []string         `json:"functionalTypes"`
	PlaceholderTypes []string         `json:"placeholderTypes"`
	ProductCurrent   int              `json:"productCurrent"`
	RawItemRows      int              `json:"rawItemRows"`
	TypeStats        []TypeStatus     `json:"typeStats"`
	LastRun          *SyncRunSummary  `json:"lastRun,omitempty"`
	LastImportedFile *SyncFileSummary `json:"lastImportedFile,omitempty"`
}

type TypeStatus struct {
	DataType         string           `json:"dataType"`
	TotalRows        int              `json:"totalRows"`
	CurrentRows      int              `json:"currentRows,omitempty"`
	RawRows          int              `json:"rawRows,omitempty"`
	LastRun          *SyncRunSummary  `json:"lastRun,omitempty"`
	LastImportedFile *SyncFileSummary `json:"lastImportedFile,omitempty"`
}

type ProductQuery struct {
	TenantID         string `json:"tenantId,omitempty"`
	StoreCode        string `json:"storeCode"`
	IdentifierPrefix string `json:"identifierPrefix,omitempty"`
	Search           string `json:"search,omitempty"`
	Page             int    `json:"page,omitempty"`
	PageSize         int    `json:"pageSize,omitempty"`
	SortBy           string `json:"sortBy,omitempty"`
	SortDir          string `json:"sortDir,omitempty"`
	DateFrom         string `json:"dateFrom,omitempty"`
	DateTo           string `json:"dateTo,omitempty"`
}

type ProductRow struct {
	SKU               string     `json:"sku"`
	Identifier        string     `json:"identifier"`
	Name              string     `json:"name"`
	Description       string     `json:"description,omitempty"`
	SupplierReference string     `json:"supplierReference,omitempty"`
	BrandName         string     `json:"brandName,omitempty"`
	SeasonName        string     `json:"seasonName,omitempty"`
	Category1         string     `json:"category1,omitempty"`
	Category2         string     `json:"category2,omitempty"`
	Category3         string     `json:"category3,omitempty"`
	Size              string     `json:"size,omitempty"`
	Color             string     `json:"color,omitempty"`
	Unit              string     `json:"unit,omitempty"`
	PriceRaw          string     `json:"priceRaw,omitempty"`
	PriceCents        *int64     `json:"priceCents,omitempty"`
	SourceCreatedAt   *time.Time `json:"sourceCreatedAt,omitempty"`
	SourceUpdatedAt   *time.Time `json:"sourceUpdatedAt,omitempty"`
	SourceFileName    string     `json:"sourceFileName,omitempty"`
	SourceBatchDate   string     `json:"sourceBatchDate,omitempty"`
}

type ProductListResponse struct {
	Store            StoreScope   `json:"store"`
	IdentifierPrefix string       `json:"identifierPrefix,omitempty"`
	Search           string       `json:"search,omitempty"`
	Page             int          `json:"page"`
	PageSize         int          `json:"pageSize"`
	Total            int          `json:"total"`
	Items            []ProductRow `json:"items"`
}

type RawRecordsQuery struct {
	TenantID       string `json:"tenantId,omitempty"`
	StoreCode      string `json:"storeCode"`
	DataType       string `json:"dataType"`
	Search         string `json:"search,omitempty"`
	SpecificSearch string `json:"specificSearch,omitempty"`
	Page           int    `json:"page,omitempty"`
	PageSize       int    `json:"pageSize,omitempty"`
	Dedup          bool   `json:"dedup,omitempty"`
	SortBy         string `json:"sortBy,omitempty"`   // coluna do allowlist por dataType
	SortDir        string `json:"sortDir,omitempty"`  // "asc" | "desc"
	DateFrom       string `json:"dateFrom,omitempty"` // YYYY-MM-DD (source_batch_date >=)
	DateTo         string `json:"dateTo,omitempty"`   // YYYY-MM-DD (source_batch_date <=)
}

type RawRecordsListResponse struct {
	Store          StoreScope       `json:"store"`
	DataType       string           `json:"dataType"`
	Search         string           `json:"search,omitempty"`
	SpecificSearch string           `json:"specificSearch,omitempty"`
	Page           int              `json:"page"`
	PageSize       int              `json:"pageSize"`
	Total          int              `json:"total"`
	Items          []map[string]any `json:"items"`
}

type RunsQuery struct {
	TenantID  string `json:"tenantId,omitempty"`
	StoreCode string `json:"storeCode"`
	DataType  string `json:"dataType,omitempty"`
	Page      int    `json:"page,omitempty"`
	PageSize  int    `json:"pageSize,omitempty"`
}

type SyncRunsListResponse struct {
	Store    StoreScope       `json:"store"`
	DataType string           `json:"dataType,omitempty"`
	Page     int              `json:"page"`
	PageSize int              `json:"pageSize"`
	Total    int              `json:"total"`
	Items    []SyncRunSummary `json:"items"`
}

type SyncAutomationSummary struct {
	Enabled       bool   `json:"enabled"`
	Interval      string `json:"interval,omitempty"`
	HourUTC       int    `json:"hourUtc,omitempty"`
	DryRunDefault bool   `json:"dryRunDefault"`
}

type SyncCoverageTotals struct {
	TotalFiles    int `json:"totalFiles"`
	ImportedFiles int `json:"importedFiles"`
	PendingFiles  int `json:"pendingFiles"`
}

type SyncCoverageEntitySummary struct {
	DataType         string `json:"dataType"`
	RemoteFilesTotal int    `json:"remoteFilesTotal"`
	ImportedFiles    int    `json:"importedFiles"`
	PendingFiles     int    `json:"pendingFiles"`
	RowsInBank       int    `json:"rowsInBank"`
	SearchableRows   int    `json:"searchableRows"`
	CurrentRows      int    `json:"currentRows,omitempty"`
}

type SyncCoverageFileSummary struct {
	SourceName    string     `json:"sourceName"`
	DataType      string     `json:"dataType"`
	DataReference time.Time  `json:"dataReference"`
	ModTime       *time.Time `json:"modTime,omitempty"`
	SizeBytes     int64      `json:"sizeBytes"`
	Imported      bool       `json:"imported"`
	Status        string     `json:"status"`
	RecordCount   int        `json:"recordCount,omitempty"`
	ImportedAt    *time.Time `json:"importedAt,omitempty"`
	SourceKind    string     `json:"sourceKind,omitempty"`
}

type SyncOverviewResponse struct {
	Store            StoreScope                  `json:"store"`
	SourceKind       string                      `json:"sourceKind"`
	SourcePath       string                      `json:"sourcePath,omitempty"`
	Automatic        SyncAutomationSummary       `json:"automatic"`
	Totals           SyncCoverageTotals          `json:"totals"`
	FullyImported    bool                        `json:"fullyImported"`
	Entities         []SyncCoverageEntitySummary `json:"entities"`
	MissingFiles     []SyncCoverageFileSummary   `json:"missingFiles"`
	AgentDocPath     string                      `json:"agentDocPath,omitempty"`
	AgentDocURL      string                      `json:"agentDocUrl,omitempty"`
	LastRun          *SyncRunSummary             `json:"lastRun,omitempty"`
	LastImportedFile *SyncFileSummary            `json:"lastImportedFile,omitempty"`
}

type CRMOverviewQuery struct {
	TenantID        string    `json:"tenantId,omitempty"`
	StoreCode       string    `json:"storeCode,omitempty"`
	DateFrom        time.Time `json:"dateFrom,omitempty"`
	DateTo          time.Time `json:"dateTo,omitempty"`
	DateFromHasTime bool      `json:"-"`
	DateToHasTime   bool      `json:"-"`
}

type CRMSummary struct {
	Orders               int     `json:"orders"`
	Units                int64   `json:"units"`
	SalesCents           int64   `json:"salesCents"`
	TicketAverageCents   int64   `json:"ticketAverageCents"`
	ValuePerProductCents int64   `json:"valuePerProductCents"`
	PAScore              float64 `json:"paScore"`
	MonthlyGoalCents     int64   `json:"monthlyGoalCents"`
	GoalProgress         float64 `json:"goalProgress"`
	RemainingToGoalCents int64   `json:"remainingToGoalCents"`
	UnmappedSalesCents   int64   `json:"unmappedSalesCents,omitempty"`
	ERPCancellations     int     `json:"erpCancellations,omitempty"`
	ERPCancellationRate  float64 `json:"erpCancellationRate,omitempty"`
}

type CRMStoreMetric struct {
	StoreSlug            string       `json:"storeSlug"`
	StoreLabel           string       `json:"storeLabel"`
	StoreCode            string       `json:"storeCode,omitempty"`
	StoreName            string       `json:"storeName,omitempty"`
	StoreCNPJs           []string     `json:"storeCnpjs,omitempty"`
	Mapped               bool         `json:"mapped"`
	Orders               int          `json:"orders"`
	Units                int64        `json:"units"`
	SalesCents           int64        `json:"salesCents"`
	TicketAverageCents   int64        `json:"ticketAverageCents"`
	ValuePerProductCents int64        `json:"valuePerProductCents"`
	PAScore              float64      `json:"paScore"`
	MonthlyGoalCents     int64        `json:"monthlyGoalCents"`
	AvgTicketGoalCents   int64        `json:"avgTicketGoalCents"`
	PAGoal               float64      `json:"paGoal"`
	GoalProgress         float64      `json:"goalProgress"`
	RemainingToGoalCents int64        `json:"remainingToGoalCents"`
	ERPCancellations     int          `json:"erpCancellations,omitempty"`
	ERPCancellationRate  float64      `json:"erpCancellationRate,omitempty"`
	StoreType            string       `json:"storeType"`
	ManagerPayout        *PayoutStore `json:"managerPayout,omitempty"`
	SupportPayout        *PayoutStore `json:"supportPayout,omitempty"`

	// Progresso EFETIVO da loja usado no gate de comissao (em reais). A meta cai na
	// soma das metas dos consultores quando a loja nao tem meta propria cadastrada,
	// para o display bater EXATAMENTE com o numero usado no calculo (evita o
	// "GoalProgress" cru, que usava so a meta da loja e estourava o %).
	StoreSold     float64 `json:"storeSold"`
	StoreGoal     float64 `json:"storeGoal"`
	StoreProgress float64 `json:"storeProgress"`

	// Flags de gap (aviso acionavel inline): de onde veio a meta da loja e quais
	// configs estao faltando. Derivados dos insumos ja carregados em applyCRMPayouts
	// (sem recalcular). "own" = loja tem monthly_goal proprio; "consultant-sum" =
	// caiu na soma das metas dos consultores; "none" = sem meta alguma.
	StoreGoalSource      string `json:"storeGoalSource"`
	MissingStoreGoal     bool   `json:"missingStoreGoal"`
	MissingTicketGoal    bool   `json:"missingTicketGoal"`
	MissingPaGoal        bool   `json:"missingPaGoal"`
	SplitConsultantCount int    `json:"splitConsultantCount"`
}

// PayoutConsultant e o payout por consultor embutido em byConsultant.
type PayoutConsultant struct {
	Amount         float64 `json:"amount"`
	RatePercent    float64 `json:"ratePercent"`
	Base           float64 `json:"base"`
	Group          string  `json:"group"`
	RuleLabel      string  `json:"ruleLabel"`
	PenaltyApplied float64 `json:"penaltyApplied"`
}

// PayoutStore e o payout de gerente/caixa resolvido por loja (store_type ja decidido).
type PayoutStore struct {
	Amount      float64 `json:"amount"`
	RatePercent float64 `json:"ratePercent"`
	RuleLabel   string  `json:"ruleLabel"`
}

type CRMConsultantMetric struct {
	ConsultantID          string            `json:"consultantId"`
	ConsultantName        string            `json:"consultantName"`
	ERPEmployeeID         string            `json:"erpEmployeeId,omitempty"`
	ProfileConsultantID   string            `json:"profileConsultantId,omitempty"`
	ProfileConsultantName string            `json:"profileConsultantName,omitempty"`
	ProfileUserID         string            `json:"profileUserId,omitempty"`
	ProfileStoreID        string            `json:"profileStoreId,omitempty"`
	ProfileStoreCode      string            `json:"profileStoreCode,omitempty"`
	ProfileStoreName      string            `json:"profileStoreName,omitempty"`
	LinkStatus            string            `json:"linkStatus,omitempty"`
	LinkConfidence        float64           `json:"linkConfidence,omitempty"`
	LinkCandidates        int               `json:"linkCandidates,omitempty"`
	StoreSlug             string            `json:"storeSlug"`
	StoreLabel            string            `json:"storeLabel"`
	StoreCNPJ             string            `json:"storeCnpj,omitempty"`
	Mapped                bool              `json:"mapped"`
	Orders                int               `json:"orders"`
	Units                 int64             `json:"units"`
	SalesCents            int64             `json:"salesCents"`
	TicketAverageCents    int64             `json:"ticketAverageCents"`
	ValuePerProductCents  int64             `json:"valuePerProductCents"`
	PAScore               float64           `json:"paScore"`
	MonthlyGoalCents      int64             `json:"monthlyGoalCents"`
	AvgTicketGoalCents    int64             `json:"avgTicketGoalCents"`
	PAGoal                float64           `json:"paGoal"`
	GoalProgress          float64           `json:"goalProgress"`
	Payout                *PayoutConsultant `json:"payout,omitempty"`

	// Flags de gap (aviso acionavel inline): de onde veio a meta mensal do consultor
	// e quais metas estao faltando. Derivados dos insumos ja carregados em
	// applyConsultantPayout (sem recalcular). "own" = consultor tem meta mensal
	// propria; "store-split" = herdou a meta da loja dividida entre N consultores;
	// "none" = sem meta alguma. Ticket/PA "missing" considera tambem a heranca da loja.
	GoalSource         string `json:"goalSource"`
	MissingMonthlyGoal bool   `json:"missingMonthlyGoal"`
	MissingTicketGoal  bool   `json:"missingTicketGoal"`
	MissingPaGoal      bool   `json:"missingPaGoal"`
}

// QueueConsultantStats contem indicadores de atendimento da fila por consultor.
// O merge com CRMConsultantMetric usa o vinculo resolvido e fallback por nome.
type QueueConsultantStats struct {
	PersonID              string  `json:"personId"`
	PersonName            string  `json:"personName"`
	StoreID               string  `json:"storeId"`
	StoreSlug             string  `json:"storeSlug,omitempty"`
	StoreLabel            string  `json:"storeLabel,omitempty"`
	Attendances           int     `json:"attendances"`
	Conversions           int     `json:"conversions"`
	ConversionRate        float64 `json:"conversionRate"`
	QueueCancellations    int     `json:"queueCancellations"`
	QueueCancellationRate float64 `json:"queueCancellationRate"`
}

type QueueStoreStats struct {
	StoreID               string  `json:"storeId"`
	StoreSlug             string  `json:"storeSlug,omitempty"`
	StoreLabel            string  `json:"storeLabel,omitempty"`
	Attendances           int     `json:"attendances"`
	Conversions           int     `json:"conversions"`
	ConversionRate        float64 `json:"conversionRate"`
	QueueCancellations    int     `json:"queueCancellations"`
	QueueCancellationRate float64 `json:"queueCancellationRate"`
}

type QueueStats struct {
	TotalAttendances   int                    `json:"totalAttendances"`
	TotalConversions   int                    `json:"totalConversions"`
	TotalCancellations int                    `json:"totalCancellations"`
	ConversionRate     float64                `json:"conversionRate"`
	CancellationRate   float64                `json:"cancellationRate"`
	ByStore            []QueueStoreStats      `json:"byStore,omitempty"`
	ByConsultant       []QueueConsultantStats `json:"byConsultant,omitempty"`
}

type CRMOverviewResponse struct {
	Store       StoreScope            `json:"store"`
	DateFrom    string                `json:"dateFrom"`
	DateTo      string                `json:"dateTo"`
	Summary     CRMSummary            `json:"summary"`
	Stores      []CRMStoreMetric      `json:"stores"`
	Consultants []CRMConsultantMetric `json:"consultants"`
	QueueStats  *QueueStats           `json:"queueStats,omitempty"`
	Warnings    []string              `json:"warnings,omitempty"`
}

// ConsultantGoalStat e o atingimento de meta CANONICO de um consultor de PERFIL,
// derivado de CRMConsultantMetric. Usado pela ponte server-side que enriquece o
// snapshot da Operacao (queue/operations) com a meta vinda do CRM, sem expor o
// payload inteiro do /v1/erp/crm (que e gestao-only). Valores em reais.
type ConsultantGoalStat struct {
	MonthlyGoal     float64
	SoldValue       float64
	RemainingToGoal float64
	Progress        float64
	HasGoal         bool
}

type ConsultantERPLinkEmployeeRow struct {
	ERPEmployeeID        string  `json:"erpEmployeeId"`
	ERPEmployeeName      string  `json:"erpEmployeeName"`
	ERPStoreCode         string  `json:"erpStoreCode,omitempty"`
	ERPStoreLabel        string  `json:"erpStoreLabel,omitempty"`
	ERPStoreRawCode      string  `json:"erpStoreRawCode,omitempty"`
	LinkID               string  `json:"linkId,omitempty"`
	LinkedConsultantID   string  `json:"linkedConsultantId,omitempty"`
	LinkedConsultantName string  `json:"linkedConsultantName,omitempty"`
	LinkedStoreID        string  `json:"linkedStoreId,omitempty"`
	LinkedStoreName      string  `json:"linkedStoreName,omitempty"`
	LinkStatus           string  `json:"linkStatus"`
	LinkConfidence       float64 `json:"linkConfidence,omitempty"`
	LinkCandidates       int     `json:"linkCandidates,omitempty"`
	Note                 string  `json:"note,omitempty"`
}

type ConsultantERPLinkConsultantOption struct {
	ConsultantID   string `json:"consultantId"`
	ConsultantName string `json:"consultantName"`
	StoreID        string `json:"storeId"`
	StoreCode      string `json:"storeCode,omitempty"`
	StoreName      string `json:"storeName"`
	EmployeeCode   string `json:"employeeCode,omitempty"`
}

type ConsultantERPLinksResponse struct {
	Store       StoreScope                          `json:"store"`
	Employees   []ConsultantERPLinkEmployeeRow      `json:"employees"`
	Consultants []ConsultantERPLinkConsultantOption `json:"consultants"`
}

type ConsultantERPLinkUpsertInput struct {
	TenantID        string   `json:"tenantId,omitempty"`
	StoreCode       string   `json:"storeCode,omitempty"`
	ERPStoreCode    string   `json:"erpStoreCode,omitempty"`
	ERPEmployeeID   string   `json:"erpEmployeeId"`
	ERPEmployeeName string   `json:"erpEmployeeName,omitempty"`
	ConsultantID    string   `json:"consultantId"`
	EmployeeIDs     []string `json:"employeeIds,omitempty"`
	Note            string   `json:"note,omitempty"`
}

type ConsultantERPLinkDeleteInput struct {
	TenantID    string   `json:"tenantId,omitempty"`
	StoreCode   string   `json:"storeCode,omitempty"`
	EmployeeIDs []string `json:"employeeIds,omitempty"`
	LinkID      string   `json:"linkId"`
}

type ItemBootstrapInput struct {
	TenantID   string `json:"tenantId,omitempty"`
	StoreCode  string `json:"storeCode"`
	SourcePath string `json:"sourcePath,omitempty"`
}

type BootstrapInput struct {
	TenantID   string `json:"tenantId,omitempty"`
	StoreCode  string `json:"storeCode"`
	DataType   string `json:"dataType"`
	SourcePath string `json:"sourcePath,omitempty"`
}

type ItemBootstrapResult struct {
	OK            bool       `json:"ok"`
	RunID         string     `json:"runId"`
	Store         StoreScope `json:"store"`
	DataType      string     `json:"dataType"`
	SourcePath    string     `json:"sourcePath"`
	FilesSeen     int        `json:"filesSeen"`
	FilesImported int        `json:"filesImported"`
	FilesSkipped  int        `json:"filesSkipped"`
	RowsRead      int        `json:"rowsRead"`
	RowsImported  int        `json:"rowsImported"`
	StartedAt     time.Time  `json:"startedAt"`
	FinishedAt    time.Time  `json:"finishedAt"`
	StoreCNPJ     string     `json:"storeCnpj,omitempty"`
}

type BootstrapResult struct {
	OK            bool       `json:"ok"`
	RunID         string     `json:"runId"`
	Store         StoreScope `json:"store"`
	DataType      string     `json:"dataType"`
	SourcePath    string     `json:"sourcePath"`
	FilesSeen     int        `json:"filesSeen"`
	FilesImported int        `json:"filesImported"`
	FilesSkipped  int        `json:"filesSkipped"`
	RowsRead      int        `json:"rowsRead"`
	RowsImported  int        `json:"rowsImported"`
	StartedAt     time.Time  `json:"startedAt"`
	FinishedAt    time.Time  `json:"finishedAt"`
	StoreCNPJ     string     `json:"storeCnpj,omitempty"`
}

type IngestInput struct {
	TenantID    string `json:"tenantId,omitempty"`
	StoreCode   string `json:"storeCode"`
	DataType    string `json:"dataType,omitempty"`
	DryRun      bool   `json:"dryRun,omitempty"`
	MaxFiles    int    `json:"maxFiles,omitempty"`
	TriggeredBy string `json:"triggeredBy,omitempty"`
}

type FileFailure struct {
	SourceName string `json:"sourceName"`
	Message    string `json:"message"`
}

type IngestResult struct {
	OK            bool          `json:"ok"`
	RunID         string        `json:"runId,omitempty"`
	RunIDs        []string      `json:"runIds,omitempty"`
	Store         StoreScope    `json:"store"`
	DataType      string        `json:"dataType,omitempty"`
	DataTypes     []string      `json:"dataTypes,omitempty"`
	DryRun        bool          `json:"dryRun"`
	FilesSeen     int           `json:"filesSeen"`
	FilesImported int           `json:"filesImported"`
	FilesSkipped  int           `json:"filesSkipped"`
	FilesFailed   []FileFailure `json:"filesFailed,omitempty"`
	RowsRead      int           `json:"rowsRead"`
	RowsImported  int           `json:"rowsImported"`
	StartedAt     time.Time     `json:"startedAt"`
	FinishedAt    time.Time     `json:"finishedAt"`
	Duration      string        `json:"duration"`
	StoreCNPJ     string        `json:"storeCnpj,omitempty"`
}

type itemConsolidatedBatch struct {
	DataType            string
	StoreCode           string
	StoreCNPJ           string
	SourceFileName      string
	SourcePath          string
	SourceKind          string
	BatchDate           string
	SourceExtractedAt   *time.Time
	SourceDataReference *time.Time
	SourceSizeBytes     int64
	ErrorMessage        string
	ProcessedAt         string
	Rows                []ItemRawRecord
	ChecksumSHA256      string
}

type customerConsolidatedBatch struct {
	DataType            string
	StoreCode           string
	StoreCNPJ           string
	SourceFileName      string
	SourcePath          string
	SourceKind          string
	BatchDate           string
	SourceExtractedAt   *time.Time
	SourceDataReference *time.Time
	SourceSizeBytes     int64
	ErrorMessage        string
	ProcessedAt         string
	Rows                []CustomerRawRecord
	ChecksumSHA256      string
}

type employeeConsolidatedBatch struct {
	DataType            string
	StoreCode           string
	StoreCNPJ           string
	SourceFileName      string
	SourcePath          string
	SourceKind          string
	BatchDate           string
	SourceExtractedAt   *time.Time
	SourceDataReference *time.Time
	SourceSizeBytes     int64
	ErrorMessage        string
	ProcessedAt         string
	Rows                []EmployeeRawRecord
	ChecksumSHA256      string
}

type orderConsolidatedBatch struct {
	DataType            string
	StoreCode           string
	StoreCNPJ           string
	SourceFileName      string
	SourcePath          string
	SourceKind          string
	BatchDate           string
	SourceExtractedAt   *time.Time
	SourceDataReference *time.Time
	SourceSizeBytes     int64
	ErrorMessage        string
	ProcessedAt         string
	Rows                []OrderRawRecord
	ChecksumSHA256      string
}

type ItemRawRecord struct {
	StoreCode         string
	StoreCNPJ         string
	SourceFileName    string
	SourceBatchDate   string
	SourceLineNumber  int
	RawValues         []string
	RawPayload        map[string]string
	SKU               string
	Name              string
	Description       string
	SupplierReference string
	BrandName         string
	SeasonName        string
	Category1         string
	Category2         string
	Category3         string
	Size              string
	Color             string
	Unit              string
	PriceRaw          string
	PriceCents        *int64
	Identifier        string
	CreatedAtRaw      string
	UpdatedAtRaw      string
	CreatedAt         *time.Time
	UpdatedAt         *time.Time
}

type CustomerRawRecord struct {
	StoreCode        string
	StoreCNPJ        string
	SourceFileName   string
	SourceBatchDate  string
	SourceLineNumber int
	RawValues        []string
	RawPayload       map[string]string
	Name             string
	Nickname         string
	CPF              string
	Email            string
	Phone            string
	Mobile           string
	Gender           string
	BirthdayRaw      string
	Street           string
	Number           string
	Complement       string
	Neighborhood     string
	City             string
	UF               string
	Country          string
	Zipcode          string
	EmployeeID       string
	StoreIDRaw       string
	RegisteredAtRaw  string
	OriginalID       string
	Identifier       string
	Tags             string
}

type EmployeeRawRecord struct {
	StoreCode        string
	StoreCNPJ        string
	SourceFileName   string
	SourceBatchDate  string
	SourceLineNumber int
	RawValues        []string
	RawPayload       map[string]string
	Name             string
	StoreIDRaw       string
	OriginalID       string
	Street           string
	Complement       string
	City             string
	UF               string
	Zipcode          string
	IsActiveRaw      string
}

type OrderRawRecord struct {
	StoreCode           string
	StoreCNPJ           string
	SourceFileName      string
	SourceBatchDate     string
	SourceLineNumber    int
	RawValues           []string
	RawPayload          map[string]string
	OrderID             string
	Identifier          string
	StoreIDRaw          string
	CustomerID          string
	OrderDateRaw        string
	OrderDate           *time.Time
	TotalAmountRaw      string
	TotalAmountCents    *int64
	ProductReturnRaw    string
	ProductReturnCents  *int64
	SKU                 string
	AmountRaw           string
	AmountCents         *int64
	QuantityRaw         string
	Quantity            *int64
	EmployeeID          string
	PaymentType         string
	TotalExclusionRaw   string
	TotalExclusionCents *int64
	TotalDebitRaw       string
	TotalDebitCents     *int64
}

type RecordsStatsQuery struct {
	TenantID       string `json:"tenantId,omitempty"`
	StoreCode      string `json:"storeCode,omitempty"`
	DataType       string `json:"dataType"`
	Search         string `json:"search,omitempty"`
	SpecificSearch string `json:"specificSearch,omitempty"`
	DateFrom       string `json:"dateFrom,omitempty"`
	DateTo         string `json:"dateTo,omitempty"`
}

type RecordsStatsResponse struct {
	DataType         string  `json:"dataType"`
	OrderCount       int64   `json:"orderCount"`
	TotalAmountCents int64   `json:"totalAmountCents"`
	AvgAmountCents   int64   `json:"avgAmountCents"`
	TotalItems       int64   `json:"totalItems"`
	PA               float64 `json:"pa"`
	CustomerCount    int64   `json:"customerCount"`
}

type itemBatchImportInput struct {
	RunID      string
	Store      StoreScope
	DataType   string
	Batch      itemConsolidatedBatch
	ImportedAt time.Time
}

type itemBatchImportResult struct {
	Imported      bool
	Rows          int
	FileID        string
	StoreCNPJ     string
	RefreshedRows int
}

type customerBatchImportInput struct {
	RunID      string
	Store      StoreScope
	DataType   string
	Batch      customerConsolidatedBatch
	ImportedAt time.Time
}

type employeeBatchImportInput struct {
	RunID      string
	Store      StoreScope
	DataType   string
	Batch      employeeConsolidatedBatch
	ImportedAt time.Time
}

type orderBatchImportInput struct {
	RunID      string
	Store      StoreScope
	DataType   string
	Batch      orderConsolidatedBatch
	ImportedAt time.Time
}
