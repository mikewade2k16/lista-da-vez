package settings

import (
	"context"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

type OptionItem struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type ProductItem struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Code      string  `json:"code"`
	Category  string  `json:"category"`
	BasePrice float64 `json:"basePrice"`
}

type AppSettings struct {
	MaxConcurrentServices    int     `json:"maxConcurrentServices"`
	TimingFastCloseMinutes   int     `json:"timingFastCloseMinutes"`
	TimingLongServiceMinutes int     `json:"timingLongServiceMinutes"`
	TimingLowSaleAmount      float64 `json:"timingLowSaleAmount"`
	TestModeEnabled          bool    `json:"testModeEnabled"`
	AutoFillFinishModal      bool    `json:"autoFillFinishModal"`
	AlertMinConversionRate   float64 `json:"alertMinConversionRate"`
	AlertMaxQueueJumpRate    float64 `json:"alertMaxQueueJumpRate"`
	AlertMinPaScore          float64 `json:"alertMinPaScore"`
	AlertMinTicketAverage    float64 `json:"alertMinTicketAverage"`
}

type AppSettingsPatch struct {
	MaxConcurrentServices    *int     `json:"maxConcurrentServices,omitempty"`
	TimingFastCloseMinutes   *int     `json:"timingFastCloseMinutes,omitempty"`
	TimingLongServiceMinutes *int     `json:"timingLongServiceMinutes,omitempty"`
	TimingLowSaleAmount      *float64 `json:"timingLowSaleAmount,omitempty"`
	TestModeEnabled          *bool    `json:"testModeEnabled,omitempty"`
	AutoFillFinishModal      *bool    `json:"autoFillFinishModal,omitempty"`
	AlertMinConversionRate   *float64 `json:"alertMinConversionRate,omitempty"`
	AlertMaxQueueJumpRate    *float64 `json:"alertMaxQueueJumpRate,omitempty"`
	AlertMinPaScore          *float64 `json:"alertMinPaScore,omitempty"`
	AlertMinTicketAverage    *float64 `json:"alertMinTicketAverage,omitempty"`
}

type ModalConfig struct {
	Title                           string `json:"title"`
	ProductSeenLabel                string `json:"productSeenLabel"`
	ProductSeenPlaceholder          string `json:"productSeenPlaceholder"`
	ProductClosedLabel              string `json:"productClosedLabel"`
	ProductClosedPlaceholder        string `json:"productClosedPlaceholder"`
	NotesLabel                      string `json:"notesLabel"`
	NotesPlaceholder                string `json:"notesPlaceholder"`
	QueueJumpReasonLabel            string `json:"queueJumpReasonLabel"`
	QueueJumpReasonPlaceholder      string `json:"queueJumpReasonPlaceholder"`
	LossReasonLabel                 string `json:"lossReasonLabel"`
	LossReasonPlaceholder           string `json:"lossReasonPlaceholder"`
	CustomerSectionLabel            string `json:"customerSectionLabel"`
	ShowCustomerNameField           bool   `json:"showCustomerNameField"`
	ShowCustomerPhoneField          bool   `json:"showCustomerPhoneField"`
	ShowEmailField                  bool   `json:"showEmailField"`
	ShowProfessionField             bool   `json:"showProfessionField"`
	ShowNotesField                  bool   `json:"showNotesField"`
	ShowProductSeenField            bool   `json:"showProductSeenField"`
	ShowProductSeenNotesField       bool   `json:"showProductSeenNotesField"`
	ShowProductClosedField          bool   `json:"showProductClosedField"`
	ShowVisitReasonField            bool   `json:"showVisitReasonField"`
	ShowCustomerSourceField         bool   `json:"showCustomerSourceField"`
	ShowExistingCustomerField       bool   `json:"showExistingCustomerField"`
	ShowQueueJumpReasonField        bool   `json:"showQueueJumpReasonField"`
	ShowLossReasonField             bool   `json:"showLossReasonField"`
	AllowProductSeenNone            bool   `json:"allowProductSeenNone"`
	VisitReasonSelectionMode        string `json:"visitReasonSelectionMode"`
	VisitReasonDetailMode           string `json:"visitReasonDetailMode"`
	LossReasonSelectionMode         string `json:"lossReasonSelectionMode"`
	LossReasonDetailMode            string `json:"lossReasonDetailMode"`
	CustomerSourceSelectionMode     string `json:"customerSourceSelectionMode"`
	CustomerSourceDetailMode        string `json:"customerSourceDetailMode"`
	RequireCustomerNameField        bool   `json:"requireCustomerNameField"`
	RequireCustomerPhoneField       bool   `json:"requireCustomerPhoneField"`
	RequireEmailField               bool   `json:"requireEmailField"`
	RequireProfessionField          bool   `json:"requireProfessionField"`
	RequireNotesField               bool   `json:"requireNotesField"`
	RequireProduct                  bool   `json:"requireProduct"`
	RequireProductSeenField         bool   `json:"requireProductSeenField"`
	RequireProductSeenNotesField    bool   `json:"requireProductSeenNotesField"`
	RequireProductClosedField       bool   `json:"requireProductClosedField"`
	RequireVisitReason              bool   `json:"requireVisitReason"`
	RequireCustomerSource           bool   `json:"requireCustomerSource"`
	RequireCustomerNamePhone        bool   `json:"requireCustomerNamePhone"`
	RequireProductSeenNotesWhenNone bool   `json:"requireProductSeenNotesWhenNone"`
	ProductSeenNotesMinChars        int    `json:"productSeenNotesMinChars"`
	RequireQueueJumpReasonField     bool   `json:"requireQueueJumpReasonField"`
	RequireLossReasonField          bool   `json:"requireLossReasonField"`
}

type ModalConfigPatch struct {
	Title                           *string `json:"title,omitempty"`
	ProductSeenLabel                *string `json:"productSeenLabel,omitempty"`
	ProductSeenPlaceholder          *string `json:"productSeenPlaceholder,omitempty"`
	ProductClosedLabel              *string `json:"productClosedLabel,omitempty"`
	ProductClosedPlaceholder        *string `json:"productClosedPlaceholder,omitempty"`
	NotesLabel                      *string `json:"notesLabel,omitempty"`
	NotesPlaceholder                *string `json:"notesPlaceholder,omitempty"`
	QueueJumpReasonLabel            *string `json:"queueJumpReasonLabel,omitempty"`
	QueueJumpReasonPlaceholder      *string `json:"queueJumpReasonPlaceholder,omitempty"`
	LossReasonLabel                 *string `json:"lossReasonLabel,omitempty"`
	LossReasonPlaceholder           *string `json:"lossReasonPlaceholder,omitempty"`
	CustomerSectionLabel            *string `json:"customerSectionLabel,omitempty"`
	ShowCustomerNameField           *bool   `json:"showCustomerNameField,omitempty"`
	ShowCustomerPhoneField          *bool   `json:"showCustomerPhoneField,omitempty"`
	ShowEmailField                  *bool   `json:"showEmailField,omitempty"`
	ShowProfessionField             *bool   `json:"showProfessionField,omitempty"`
	ShowNotesField                  *bool   `json:"showNotesField,omitempty"`
	ShowProductSeenField            *bool   `json:"showProductSeenField,omitempty"`
	ShowProductSeenNotesField       *bool   `json:"showProductSeenNotesField,omitempty"`
	ShowProductClosedField          *bool   `json:"showProductClosedField,omitempty"`
	ShowVisitReasonField            *bool   `json:"showVisitReasonField,omitempty"`
	ShowCustomerSourceField         *bool   `json:"showCustomerSourceField,omitempty"`
	ShowExistingCustomerField       *bool   `json:"showExistingCustomerField,omitempty"`
	ShowQueueJumpReasonField        *bool   `json:"showQueueJumpReasonField,omitempty"`
	ShowLossReasonField             *bool   `json:"showLossReasonField,omitempty"`
	AllowProductSeenNone            *bool   `json:"allowProductSeenNone,omitempty"`
	VisitReasonSelectionMode        *string `json:"visitReasonSelectionMode,omitempty"`
	VisitReasonDetailMode           *string `json:"visitReasonDetailMode,omitempty"`
	LossReasonSelectionMode         *string `json:"lossReasonSelectionMode,omitempty"`
	LossReasonDetailMode            *string `json:"lossReasonDetailMode,omitempty"`
	CustomerSourceSelectionMode     *string `json:"customerSourceSelectionMode,omitempty"`
	CustomerSourceDetailMode        *string `json:"customerSourceDetailMode,omitempty"`
	RequireCustomerNameField        *bool   `json:"requireCustomerNameField,omitempty"`
	RequireCustomerPhoneField       *bool   `json:"requireCustomerPhoneField,omitempty"`
	RequireEmailField               *bool   `json:"requireEmailField,omitempty"`
	RequireProfessionField          *bool   `json:"requireProfessionField,omitempty"`
	RequireNotesField               *bool   `json:"requireNotesField,omitempty"`
	RequireProduct                  *bool   `json:"requireProduct,omitempty"`
	RequireProductSeenField         *bool   `json:"requireProductSeenField,omitempty"`
	RequireProductSeenNotesField    *bool   `json:"requireProductSeenNotesField,omitempty"`
	RequireProductClosedField       *bool   `json:"requireProductClosedField,omitempty"`
	RequireVisitReason              *bool   `json:"requireVisitReason,omitempty"`
	RequireCustomerSource           *bool   `json:"requireCustomerSource,omitempty"`
	RequireCustomerNamePhone        *bool   `json:"requireCustomerNamePhone,omitempty"`
	RequireProductSeenNotesWhenNone *bool   `json:"requireProductSeenNotesWhenNone,omitempty"`
	ProductSeenNotesMinChars        *int    `json:"productSeenNotesMinChars,omitempty"`
	RequireQueueJumpReasonField     *bool   `json:"requireQueueJumpReasonField,omitempty"`
	RequireLossReasonField          *bool   `json:"requireLossReasonField,omitempty"`
}

type OperationTemplate struct {
	ID                    string       `json:"id"`
	Label                 string       `json:"label"`
	Description           string       `json:"description"`
	Settings              AppSettings  `json:"settings"`
	ModalConfig           ModalConfig  `json:"modalConfig"`
	VisitReasonOptions    []OptionItem `json:"visitReasonOptions"`
	CustomerSourceOptions []OptionItem `json:"customerSourceOptions"`
}

// Bundle representa a configuracao operacional de um tenant.
// As configuracoes deixaram de ser por loja: o campo TenantID identifica o tenant
// dono do bundle e a configuracao retornada vale para todas as lojas dele.
type Bundle struct {
	TenantID                    string              `json:"tenantId"`
	OperationTemplates          []OperationTemplate `json:"operationTemplates,omitempty"`
	SelectedOperationTemplateID string              `json:"selectedOperationTemplateId"`
	Settings                    AppSettings         `json:"settings"`
	ModalConfig                 ModalConfig         `json:"modalConfig"`
	VisitReasonOptions          []OptionItem        `json:"visitReasonOptions"`
	CustomerSourceOptions       []OptionItem        `json:"customerSourceOptions"`
	PauseReasonOptions          []OptionItem        `json:"pauseReasonOptions"`
	QueueJumpReasonOptions      []OptionItem        `json:"queueJumpReasonOptions"`
	LossReasonOptions           []OptionItem        `json:"lossReasonOptions"`
	ProfessionOptions           []OptionItem        `json:"professionOptions"`
	ProductCatalog              []ProductItem       `json:"productCatalog"`
}

// Inputs continuam aceitando o legado StoreID (json:"storeId") porque a UI atual
// envia esse campo. O service ignora o valor e resolve o tenant pelo principal,
// garantindo que mudancas em config nunca sejam gravadas no escopo de uma loja.
type OperationSectionInput struct {
	StoreID                     string            `json:"storeId,omitempty"`
	TenantID                    string            `json:"tenantId,omitempty"`
	SelectedOperationTemplateID *string           `json:"selectedOperationTemplateId,omitempty"`
	Settings                    *AppSettingsPatch `json:"settings,omitempty"`
}

type ModalSectionInput struct {
	StoreID     string            `json:"storeId,omitempty"`
	TenantID    string            `json:"tenantId,omitempty"`
	ModalConfig *ModalConfigPatch `json:"modalConfig,omitempty"`
}

type OptionSectionInput struct {
	StoreID  string       `json:"storeId,omitempty"`
	TenantID string       `json:"tenantId,omitempty"`
	Items    []OptionItem `json:"items"`
}

type OptionItemInput struct {
	StoreID  string     `json:"storeId,omitempty"`
	TenantID string     `json:"tenantId,omitempty"`
	Item     OptionItem `json:"item"`
}

type OptionItemPatchInput struct {
	StoreID  string `json:"storeId,omitempty"`
	TenantID string `json:"tenantId,omitempty"`
	Label    string `json:"label"`
}

type ProductSectionInput struct {
	StoreID  string        `json:"storeId,omitempty"`
	TenantID string        `json:"tenantId,omitempty"`
	Items    []ProductItem `json:"items"`
}

type ProductItemInput struct {
	StoreID  string      `json:"storeId,omitempty"`
	TenantID string      `json:"tenantId,omitempty"`
	Item     ProductItem `json:"item"`
}

type ProductItemPatchInput struct {
	StoreID   string  `json:"storeId,omitempty"`
	TenantID  string  `json:"tenantId,omitempty"`
	Name      string  `json:"name"`
	Code      string  `json:"code"`
	Category  string  `json:"category"`
	BasePrice float64 `json:"basePrice"`
}

type MutationAck struct {
	OK       bool      `json:"ok"`
	TenantID string    `json:"tenantId"`
	SavedAt  time.Time `json:"savedAt"`
}

type Record struct {
	TenantID                    string
	SelectedOperationTemplateID string
	Settings                    AppSettings
	ModalConfig                 ModalConfig
	VisitReasonOptions          []OptionItem
	CustomerSourceOptions       []OptionItem
	PauseReasonOptions          []OptionItem
	QueueJumpReasonOptions      []OptionItem
	LossReasonOptions           []OptionItem
	ProfessionOptions           []OptionItem
	ProductCatalog              []ProductItem
	CreatedAt                   time.Time
	UpdatedAt                   time.Time
}

type Repository interface {
	TenantExists(ctx context.Context, tenantID string) (bool, error)
	ResolveDefaultTenantID(ctx context.Context, principal auth.Principal) (string, error)
	GetByTenant(ctx context.Context, tenantID string) (Record, bool, error)
	Upsert(ctx context.Context, record Record) (Record, error)
	UpsertConfig(ctx context.Context, record Record) (Record, error)
	ReplaceOptionGroup(ctx context.Context, tenantID string, kind string, options []OptionItem) (time.Time, error)
	UpsertOption(ctx context.Context, tenantID string, kind string, option OptionItem) (time.Time, error)
	DeleteOption(ctx context.Context, tenantID string, kind string, optionID string) (time.Time, error)
	ReplaceProducts(ctx context.Context, tenantID string, products []ProductItem) (time.Time, error)
	UpsertProduct(ctx context.Context, tenantID string, product ProductItem) (time.Time, error)
	DeleteProduct(ctx context.Context, tenantID string, productID string) (time.Time, error)
}
