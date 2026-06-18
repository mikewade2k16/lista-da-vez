package erp

type crmStoreAlias struct {
	Slug  string
	Label string
}

type crmStoreTarget struct {
	Slug               string
	Label              string
	Code               string
	Name               string
	StoreType          string
	MonthlyGoalCents   int64
	AvgTicketGoalCents int64
	PAGoal             float64
}

type crmStoreAggregate struct {
	StoreCNPJ         string
	Orders            int
	Units             int64
	SalesCents        int64
	ProductSalesCents int64
}

type crmConsultantAggregate struct {
	ConsultantID      string
	ConsultantName    string
	StoreCNPJ         string
	Orders            int
	Units             int64
	SalesCents        int64
	ProductSalesCents int64
}

type crmConsultantLinkProfile struct {
	ConsultantID   string
	ConsultantName string
	UserID         string
	StoreID        string
	StoreCode      string
	StoreName      string
	EmployeeCode   string
}

type crmConsultantManualLink struct {
	LinkID          string
	ERPEmployeeID   string
	ERPEmployeeName string
	ERPStoreCode    string
	Note            string
	Profile         crmConsultantLinkProfile
}

type crmConsultantResolvedLink struct {
	Profile    crmConsultantLinkProfile
	Status     string
	Confidence float64
	Candidates int
}

type crmOrderAggregate struct {
	ExplicitStoreCNPJ string
	FallbackStoreCNPJ string
	EmployeeID        string
	Units             int64
	SalesCents        int64
	ProductSalesCents int64
}

type crmERPEmployeeLinkCandidate struct {
	ERPEmployeeID   string
	ERPEmployeeName string
	ERPStoreRawCode string
	ERPStoreCode    string
	ERPStoreLabel   string
}

type crmCanceledStoreAggregate struct {
	StoreCNPJ      string
	CanceledOrders int
}

type crmQueueConsultantStat struct {
	PersonID           string
	PersonName         string
	StoreID            string
	StoreCode          string
	StoreName          string
	Attendances        int
	Conversions        int
	QueueCancellations int
}

type crmQueueStoreStat struct {
	StoreID            string
	StoreCode          string
	StoreName          string
	Attendances        int
	Conversions        int
	QueueCancellations int
}

const crmStoreKeyManagementMultiStore = "gerencia-multiloja"

const (
	crmConsultantLinkStatusManual       = "manual"
	crmConsultantLinkStatusEmployeeCode = "employee_code"
	crmConsultantLinkStatusNameExact    = "name_exact"
	crmConsultantLinkStatusAmbiguous    = "ambiguous"
	crmConsultantLinkStatusUnmatched    = "unmatched"
	crmConsultantLinkNoteAutoEmployee   = "system:auto_employee_code"
	crmConsultantLinkNoteAutoName       = "system:auto_name_exact"
)

var crmStoreAliases = map[string]crmStoreAlias{
	"31327524000115": {Slug: "riomar", Label: "Riomar"},
	"12583959000186": {Slug: "riomar", Label: "Riomar"},
	"56173889000163": {Slug: "jardins", Label: "Jardins"},
	"53578278000107": {Slug: "garcia", Label: "Garcia"},
	"43068099000176": {Slug: "treze", Label: "Treze"},
	"43068099000257": {Slug: "treze", Label: "Treze"},
}

var crmSpecialStoreAliases = map[string]crmStoreAlias{
	crmStoreKeyManagementMultiStore: {Slug: crmStoreKeyManagementMultiStore, Label: "Gerencia / Multi-loja"},
}

var crmEmployeeSpecialStoreKeys = map[string]string{
	"16": crmStoreKeyManagementMultiStore,
}

var crmStoreOrder = map[string]int{
	"riomar":                        0,
	"jardins":                       1,
	"garcia":                        2,
	"treze":                         3,
	crmStoreKeyManagementMultiStore: 4,
}
