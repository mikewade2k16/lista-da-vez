package finance

// Structs do modulo finance. JSON camelCase identico ao contrato do front
// (web/layers/finance/types/finances.ts) e ao mock que este modulo substitui.
// Slices sao sempre serializados nao-nil (o front espera array vazio, nunca null).

// Adjustment e um ajuste pontual dentro de uma linha.
type Adjustment struct {
	ID     string  `json:"id"`
	Amount float64 `json:"amount"`
	Note   string  `json:"note"`
	Date   string  `json:"date"`
}

// Line e uma linha de entrada ou saida da planilha.
type Line struct {
	ID               string       `json:"id"`
	Kind             string       `json:"kind,omitempty"`
	Description      string       `json:"description"`
	Category         string       `json:"category"`
	Effective        bool         `json:"effective"`
	EffectiveDate    string       `json:"effectiveDate"`
	Amount           float64      `json:"amount"`
	AdjustmentAmount float64      `json:"adjustmentAmount"`
	Adjustments      []Adjustment `json:"adjustments"`
	FixedAccountID   string       `json:"fixedAccountId"`
	Details          string       `json:"details"`
}

// Summary agrega os totais esperados/efetivos de uma planilha.
type Summary struct {
	ExpectedIn       float64 `json:"expectedIn"`
	EffectiveIn      float64 `json:"effectiveIn"`
	ExpectedOut      float64 `json:"expectedOut"`
	EffectiveOut     float64 `json:"effectiveOut"`
	ExpectedBalance  float64 `json:"expectedBalance"`
	EffectiveBalance float64 `json:"effectiveBalance"`
}

// SheetListItem e a projecao de listagem (sem as linhas).
type SheetListItem struct {
	ID           string  `json:"id"`
	Title        string  `json:"title"`
	Period       string  `json:"period"`
	Status       string  `json:"status"`
	Notes        string  `json:"notes"`
	CoreTenantID string  `json:"coreTenantId"`
	ClientName   string  `json:"clientName"`
	Summary      Summary `json:"summary"`
	Preview      string  `json:"preview"`
	CreatedAt    string  `json:"createdAt"`
	UpdatedAt    string  `json:"updatedAt"`
}

// SheetDetail e a planilha completa (list item + linhas).
type SheetDetail struct {
	SheetListItem
	Entradas []Line `json:"entradas"`
	Saidas   []Line `json:"saidas"`
}

// SheetInput e o corpo de create/update. Campos *string sao opcionais; ausentes
// no create caem no default do service. coreTenantId nunca escapa do escopo da
// account (e apenas filtro/rotulo do cliente).
type SheetInput struct {
	Title        *string `json:"title"`
	Period       *string `json:"period"`
	Status       *string `json:"status"`
	Notes        *string `json:"notes"`
	Entradas     []Line  `json:"entradas"`
	Saidas       []Line  `json:"saidas"`
	CoreTenantID *string `json:"coreTenantId"`
}

// LinePatchInput efetiva/des-efetiva uma linha (PATCH). Campos ausentes ficam
// intactos; des-efetivar (effective=false) limpa effectiveDate.
type LinePatchInput struct {
	Effective     *bool   `json:"effective"`
	EffectiveDate *string `json:"effectiveDate"`
}

// LineMutationData e a resposta do PATCH de linha.
type LineMutationData struct {
	SheetID   string  `json:"sheetId"`
	LineID    string  `json:"lineId"`
	Line      Line    `json:"line"`
	Summary   Summary `json:"summary"`
	Preview   string  `json:"preview"`
	UpdatedAt string  `json:"updatedAt"`
}

// ============================================================================
// Config (categorias, contas fixas, recorrencias)
// ============================================================================

// Category e uma categoria de classificacao de linha.
type Category struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Description string `json:"description"`
}

// FixedAccountMember e um integrante de uma conta fixa (ex.: colaborador na folha).
type FixedAccountMember struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Amount float64 `json:"amount"`
}

// FixedAccount e uma conta fixa reutilizavel (ex.: folha salarial).
type FixedAccount struct {
	ID            string               `json:"id"`
	Name          string               `json:"name"`
	Kind          string               `json:"kind"`
	CategoryID    string               `json:"categoryId"`
	DefaultAmount float64              `json:"defaultAmount"`
	Notes         string               `json:"notes"`
	Members       []FixedAccountMember `json:"members"`
}

// RecurringEntry e uma recorrencia declarada na config.
type RecurringEntry struct {
	SourceCoreTenantID string  `json:"sourceCoreTenantId,omitempty"`
	AdjustmentAmount   float64 `json:"adjustmentAmount"`
	Notes              string  `json:"notes"`
}

// ConfigData e o payload completo de config (round-trip GET/PUT).
type ConfigData struct {
	CoreTenantID     string           `json:"coreTenantId"`
	Categories       []Category       `json:"categories"`
	FixedAccounts    []FixedAccount   `json:"fixedAccounts"`
	RecurringEntries []RecurringEntry `json:"recurringEntries"`
	UpdatedAt        string           `json:"updatedAt"`
}

// ConfigInput e o corpo do PUT /v1/finance/config. coreTenantId opcional (filtro).
type ConfigInput struct {
	CoreTenantID     *string          `json:"coreTenantId"`
	Categories       []Category       `json:"categories"`
	FixedAccounts    []FixedAccount   `json:"fixedAccounts"`
	RecurringEntries []RecurringEntry `json:"recurringEntries"`
}

// RecurringClientStore e uma loja com valor de billing proprio (per_store).
type RecurringClientStore struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Amount float64 `json:"amount"`
}

// RecurringClient e um cliente com mensalidade/billing (read model do painel de
// recorrencias — so platform_admin recebe a lista real).
type RecurringClient struct {
	ID                   string                 `json:"id"`
	CoreTenantID         string                 `json:"coreTenantId"`
	Name                 string                 `json:"name"`
	MonthlyPaymentAmount float64                `json:"monthlyPaymentAmount"`
	PaymentDueDay        string                 `json:"paymentDueDay"`
	BillingMode          string                 `json:"billingMode"`
	Stores               []RecurringClientStore `json:"stores"`
}

// ListFilter parametriza a listagem de planilhas.
type ListFilter struct {
	CoreTenantID string
	Period       string
	Q            string
	Page         int
	Limit        int
}

// ListMeta e o bloco de paginacao da resposta de listagem.
type ListMeta struct {
	Page       int  `json:"page"`
	Limit      int  `json:"limit"`
	Total      int  `json:"total"`
	TotalPages int  `json:"totalPages"`
	HasMore    bool `json:"hasMore"`
}
