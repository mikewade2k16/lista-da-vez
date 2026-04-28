package erp

import "time"

const (
	DataTypeItem          = "item"
	DataTypeCustomer      = "customer"
	DataTypeEmployee      = "employee"
	DataTypeOrder         = "order"
	DataTypeOrderCanceled = "ordercanceled"

	SyncModeBootstrapMarkdown = "bootstrap_markdown"
	SyncStatusRunning         = "running"
	SyncStatusSucceeded       = "succeeded"
	SyncStatusFailed          = "failed"

	defaultPageSize = 50
	maxPageSize     = 200
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
	SourceDir                  string
	StorageDir                 string
	BootstrapItemFile          string
	BootstrapCustomerFile      string
	BootstrapEmployeeFile      string
	BootstrapOrderFile         string
	BootstrapOrderCanceledFile string
	AllowManualSync            bool
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

type itemConsolidatedBatch struct {
	DataType       string
	StoreCode      string
	StoreCNPJ      string
	SourceFileName string
	BatchDate      string
	ProcessedAt    string
	Rows           []ItemRawRecord
	ChecksumSHA256 string
}

type customerConsolidatedBatch struct {
	DataType       string
	StoreCode      string
	StoreCNPJ      string
	SourceFileName string
	BatchDate      string
	ProcessedAt    string
	Rows           []CustomerRawRecord
	ChecksumSHA256 string
}

type employeeConsolidatedBatch struct {
	DataType       string
	StoreCode      string
	StoreCNPJ      string
	SourceFileName string
	BatchDate      string
	ProcessedAt    string
	Rows           []EmployeeRawRecord
	ChecksumSHA256 string
}

type orderConsolidatedBatch struct {
	DataType       string
	StoreCode      string
	StoreCNPJ      string
	SourceFileName string
	BatchDate      string
	ProcessedAt    string
	Rows           []OrderRawRecord
	ChecksumSHA256 string
}

type ItemRawRecord struct {
	StoreCode         string
	StoreCNPJ         string
	SourceFileName    string
	SourceBatchDate   string
	SourceLineNumber  int
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

type itemBatchImportInput struct {
	RunID      string
	Store      StoreScope
	DataType   string
	Batch      itemConsolidatedBatch
	ImportedAt time.Time
}

type itemBatchImportResult struct {
	Imported  bool
	Rows      int
	FileID    string
	StoreCNPJ string
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
