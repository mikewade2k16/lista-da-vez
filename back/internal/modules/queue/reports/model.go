package reports

type Filters struct {
	TenantID              string   `json:"tenantId,omitempty"`
	StoreID               string   `json:"storeId"`
	DateFrom              string   `json:"dateFrom,omitempty"`
	DateTo                string   `json:"dateTo,omitempty"`
	ConsultantIDs         []string `json:"consultantIds,omitempty"`
	Outcomes              []string `json:"outcomes,omitempty"`
	SourceIDs             []string `json:"sourceIds,omitempty"`
	VisitReasonIDs        []string `json:"visitReasonIds,omitempty"`
	StartModes            []string `json:"startModes,omitempty"`
	ExistingCustomerModes []string `json:"existingCustomerModes,omitempty"`
	CompletionLevels      []string `json:"completionLevels,omitempty"`
	CampaignIDs           []string `json:"campaignIds,omitempty"`
	MinSaleAmount         *float64 `json:"minSaleAmount,omitempty"`
	MaxSaleAmount         *float64 `json:"maxSaleAmount,omitempty"`
	Search                string   `json:"search,omitempty"`
	Page                  int      `json:"page,omitempty"`
	PageSize              int      `json:"pageSize,omitempty"`
}

type repositoryFilters struct {
	FinishedAtFrom     *int64
	FinishedAtTo       *int64
	ConsultantIDs      []string
	Outcomes           []string
	StartModes         []string
	IsExistingCustomer *bool
	MinSaleAmount      *float64
	MaxSaleAmount      *float64
}

type StoreLiveCounts struct {
	StoreID     string
	Consultants int
	QueueCount  int
	ActiveCount int
	PausedCount int
}

type OverviewResponse struct {
	StoreID   string          `json:"storeId"`
	Filters   Filters         `json:"filters"`
	Metrics   Metrics         `json:"metrics"`
	Quality   QualityOverview `json:"quality"`
	ChartData ChartData       `json:"chartData"`
}

type ResultsResponse struct {
	StoreID  string      `json:"storeId"`
	Filters  Filters     `json:"filters"`
	Page     int         `json:"page"`
	PageSize int         `json:"pageSize"`
	Total    int         `json:"total"`
	Rows     []ResultRow `json:"rows"`
}

type RecentServicesResponse struct {
	StoreID  string      `json:"storeId"`
	Filters  Filters     `json:"filters"`
	Page     int         `json:"page"`
	PageSize int         `json:"pageSize"`
	Total    int         `json:"total"`
	Items    []ResultRow `json:"items"`
}

type MultiStoreOverviewResponse struct {
	TenantID string                  `json:"tenantId"`
	Filters  Filters                 `json:"filters"`
	Summary  MultiStoreSummary       `json:"summary"`
	Stores   []MultiStoreOverviewRow `json:"stores"`
}

type MultiStoreSummary struct {
	ActiveStores        int     `json:"activeStores"`
	TotalAttendances    int     `json:"totalAttendances"`
	TotalSoldValue      float64 `json:"totalSoldValue"`
	TotalQueue          int     `json:"totalQueue"`
	TotalActiveServices int     `json:"totalActiveServices"`
	AverageHealthScore  float64 `json:"averageHealthScore"`
}

type MultiStoreOverviewRow struct {
	StoreID            string  `json:"storeId"`
	StoreName          string  `json:"storeName"`
	StoreCode          string  `json:"storeCode"`
	StoreCity          string  `json:"storeCity"`
	Consultants        int     `json:"consultants"`
	QueueCount         int     `json:"queueCount"`
	ActiveCount        int     `json:"activeCount"`
	PausedCount        int     `json:"pausedCount"`
	Attendances        int     `json:"attendances"`
	ConversionRate     float64 `json:"conversionRate"`
	SoldValue          float64 `json:"soldValue"`
	TicketAverage      float64 `json:"ticketAverage"`
	PAScore            float64 `json:"paScore"`
	AverageQueueWaitMs float64 `json:"averageQueueWaitMs"`
	QueueJumpRate      float64 `json:"queueJumpRate"`
	HealthScore        float64 `json:"healthScore"`
	MonthlyGoal        float64 `json:"monthlyGoal"`
	WeeklyGoal         float64 `json:"weeklyGoal"`
	AvgTicketGoal      float64 `json:"avgTicketGoal"`
	ConversionGoal     float64 `json:"conversionGoal"`
	PAGoal             float64 `json:"paGoal"`
	DefaultTemplateID  string  `json:"defaultTemplateId"`
}

type Metrics struct {
	TotalAttendances   int     `json:"totalAttendances"`
	Conversions        int     `json:"conversions"`
	NonConversions     int     `json:"nonConversions"`
	ConversionRate     float64 `json:"conversionRate"`
	SoldValue          float64 `json:"soldValue"`
	AverageTicket      float64 `json:"averageTicket"`
	AverageDurationMs  float64 `json:"averageDurationMs"`
	AverageQueueWaitMs float64 `json:"averageQueueWaitMs"`
	QueueJumpRate      float64 `json:"queueJumpRate"`
	CampaignBonusTotal float64 `json:"campaignBonusTotal"`
}

type QualityOverview struct {
	CompleteCount   int                    `json:"completeCount"`
	ExcellentCount  int                    `json:"excellentCount"`
	IncompleteCount int                    `json:"incompleteCount"`
	NotesCount      int                    `json:"notesCount"`
	CompleteRate    float64                `json:"completeRate"`
	ExcellentRate   float64                `json:"excellentRate"`
	IncompleteRate  float64                `json:"incompleteRate"`
	NotesRate       float64                `json:"notesRate"`
	ByConsultant    []ConsultantQualityRow `json:"byConsultant"`
}

type ConsultantQualityRow struct {
	ConsultantID      string  `json:"consultantId"`
	ConsultantName    string  `json:"consultantName"`
	TotalAttendances  int     `json:"totalAttendances"`
	CompleteCount     int     `json:"completeCount"`
	ExcellentCount    int     `json:"excellentCount"`
	IncompleteCount   int     `json:"incompleteCount"`
	NotesCount        int     `json:"notesCount"`
	CompleteRate      float64 `json:"completeRate"`
	ExcellentRate     float64 `json:"excellentRate"`
	IncompleteRate    float64 `json:"incompleteRate"`
	NotesRate         float64 `json:"notesRate"`
	QualityLevelKey   string  `json:"qualityLevelKey"`
	QualityLevelLabel string  `json:"qualityLevelLabel"`
}

type ChartData struct {
	OutcomeCounts      OutcomeCounts      `json:"outcomeCounts"`
	HourlyData         []HourlyDataPoint  `json:"hourlyData"`
	ConsultantAgg      []ConsultantAggRow `json:"consultantAgg"`
	TopProductsClosed  []CountRow         `json:"topProductsClosed"`
	TopVisitReasons    []CountRow         `json:"topVisitReasons"`
	TopCustomerSources []CountRow         `json:"topCustomerSources"`
}

type OutcomeCounts struct {
	Compra    int `json:"compra"`
	Reserva   int `json:"reserva"`
	NaoCompra int `json:"nao-compra"`
}

type HourlyDataPoint struct {
	Hour        string  `json:"hour"`
	Attendances int     `json:"attendances"`
	Conversions int     `json:"conversions"`
	SaleAmount  float64 `json:"saleAmount"`
}

type ConsultantAggRow struct {
	ConsultantID   string  `json:"consultantId"`
	ConsultantName string  `json:"consultantName"`
	Attendances    int     `json:"attendances"`
	Conversions    int     `json:"conversions"`
	SaleAmount     float64 `json:"saleAmount"`
}

type CountRow struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

type ResultRow struct {
	ServiceID          string   `json:"serviceId"`
	StoreID            string   `json:"storeId"`
	StoreName          string   `json:"storeName"`
	ConsultantID       string   `json:"consultantId"`
	ConsultantName     string   `json:"consultantName"`
	StartedAt          int64    `json:"startedAt"`
	FinishedAt         int64    `json:"finishedAt"`
	DurationMs         int64    `json:"durationMs"`
	QueueWaitMs        int64    `json:"queueWaitMs"`
	Outcome            string   `json:"outcome"`
	StartMode          string   `json:"startMode"`
	SaleAmount         float64  `json:"saleAmount"`
	IsWindowService    bool     `json:"isWindowService"`
	IsGift             bool     `json:"isGift"`
	IsExistingCustomer bool     `json:"isExistingCustomer"`
	CustomerName       string   `json:"customerName"`
	CustomerPhone      string   `json:"customerPhone"`
	CustomerEmail      string   `json:"customerEmail"`
	CustomerProfession string   `json:"customerProfession"`
	ProductSeen        string   `json:"productSeen"`
	ProductClosed      string   `json:"productClosed"`
	ProductDetails     string   `json:"productDetails"`
	VisitReasons       []string `json:"visitReasons"`
	CustomerSources    []string `json:"customerSources"`
	CampaignNames      []string `json:"campaignNames"`
	QueueJumpReason    string   `json:"queueJumpReason"`
	Notes              string   `json:"notes"`
	HasNotes           bool     `json:"hasNotes"`
	CompletionLevel    string   `json:"completionLevel"`
	CompletionRate     float64  `json:"completionRate"`
	CampaignBonusTotal float64  `json:"campaignBonusTotal"`
}
