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

type OperationCoreSettings struct {
	MaxConcurrentServices              int
	MaxConcurrentServicesPerConsultant int
	TimingFastCloseMinutes             int
	TimingLongServiceMinutes           int
	TimingLowSaleAmount                float64
	ServiceCancelWindowSeconds         int
	TestModeEnabled                    bool
	AutoFillFinishModal                bool
}

type OperationCoreSettingsPatch struct {
	MaxConcurrentServices              *int
	MaxConcurrentServicesPerConsultant *int
	TimingFastCloseMinutes             *int
	TimingLongServiceMinutes           *int
	TimingLowSaleAmount                *float64
	ServiceCancelWindowSeconds         *int
	TestModeEnabled                    *bool
	AutoFillFinishModal                *bool
}

type AlertSettings struct {
	AlertMinConversionRate float64
	AlertMaxQueueJumpRate  float64
	AlertMinPaScore        float64
	AlertMinTicketAverage  float64
}

type AlertSettingsPatch struct {
	AlertMinConversionRate *float64
	AlertMaxQueueJumpRate  *float64
	AlertMinPaScore        *float64
	AlertMinTicketAverage  *float64
}

type AppSettings struct {
	MaxConcurrentServices              int     `json:"maxConcurrentServices"`
	MaxConcurrentServicesPerConsultant int     `json:"maxConcurrentServicesPerConsultant"`
	TimingFastCloseMinutes             int     `json:"timingFastCloseMinutes"`
	TimingLongServiceMinutes           int     `json:"timingLongServiceMinutes"`
	TimingLowSaleAmount                float64 `json:"timingLowSaleAmount"`
	ServiceCancelWindowSeconds         int     `json:"serviceCancelWindowSeconds"`
	TestModeEnabled                    bool    `json:"testModeEnabled"`
	AutoFillFinishModal                bool    `json:"autoFillFinishModal"`
	AlertMinConversionRate             float64 `json:"alertMinConversionRate"`
	AlertMaxQueueJumpRate              float64 `json:"alertMaxQueueJumpRate"`
	AlertMinPaScore                    float64 `json:"alertMinPaScore"`
	AlertMinTicketAverage              float64 `json:"alertMinTicketAverage"`
}

type AppSettingsPatch struct {
	MaxConcurrentServices              *int     `json:"maxConcurrentServices,omitempty"`
	MaxConcurrentServicesPerConsultant *int     `json:"maxConcurrentServicesPerConsultant,omitempty"`
	TimingFastCloseMinutes             *int     `json:"timingFastCloseMinutes,omitempty"`
	TimingLongServiceMinutes           *int     `json:"timingLongServiceMinutes,omitempty"`
	TimingLowSaleAmount                *float64 `json:"timingLowSaleAmount,omitempty"`
	ServiceCancelWindowSeconds         *int     `json:"serviceCancelWindowSeconds,omitempty"`
	TestModeEnabled                    *bool    `json:"testModeEnabled,omitempty"`
	AutoFillFinishModal                *bool    `json:"autoFillFinishModal,omitempty"`
	AlertMinConversionRate             *float64 `json:"alertMinConversionRate,omitempty"`
	AlertMaxQueueJumpRate              *float64 `json:"alertMaxQueueJumpRate,omitempty"`
	AlertMinPaScore                    *float64 `json:"alertMinPaScore,omitempty"`
	AlertMinTicketAverage              *float64 `json:"alertMinTicketAverage,omitempty"`
}

type ModalConfig struct {
	Title                                 string `json:"title"`
	FinishFlowMode                        string `json:"finishFlowMode"`
	ProductSeenLabel                      string `json:"productSeenLabel"`
	ProductSeenPlaceholder                string `json:"productSeenPlaceholder"`
	ProductClosedLabel                    string `json:"productClosedLabel"`
	ProductClosedPlaceholder              string `json:"productClosedPlaceholder"`
	PurchaseCodeLabel                     string `json:"purchaseCodeLabel"`
	PurchaseCodePlaceholder               string `json:"purchaseCodePlaceholder"`
	NotesLabel                            string `json:"notesLabel"`
	NotesPlaceholder                      string `json:"notesPlaceholder"`
	QueueJumpReasonLabel                  string `json:"queueJumpReasonLabel"`
	QueueJumpReasonPlaceholder            string `json:"queueJumpReasonPlaceholder"`
	LossReasonLabel                       string `json:"lossReasonLabel"`
	LossReasonPlaceholder                 string `json:"lossReasonPlaceholder"`
	CustomerSectionLabel                  string `json:"customerSectionLabel"`
	CustomerNameLabel                     string `json:"customerNameLabel"`
	CustomerPhoneLabel                    string `json:"customerPhoneLabel"`
	CustomerEmailLabel                    string `json:"customerEmailLabel"`
	CustomerProfessionLabel               string `json:"customerProfessionLabel"`
	ExistingCustomerLabel                 string `json:"existingCustomerLabel"`
	ProductSeenNotesLabel                 string `json:"productSeenNotesLabel"`
	ProductSeenNotesPlaceholder           string `json:"productSeenNotesPlaceholder"`
	VisitReasonLabel                      string `json:"visitReasonLabel"`
	CustomerSourceLabel                   string `json:"customerSourceLabel"`
	CancelReasonLabel                     string `json:"cancelReasonLabel"`
	CancelReasonPlaceholder               string `json:"cancelReasonPlaceholder"`
	CancelReasonOtherLabel                string `json:"cancelReasonOtherLabel"`
	CancelReasonOtherPlaceholder          string `json:"cancelReasonOtherPlaceholder"`
	StopReasonLabel                       string `json:"stopReasonLabel"`
	StopReasonPlaceholder                 string `json:"stopReasonPlaceholder"`
	StopReasonOtherLabel                  string `json:"stopReasonOtherLabel"`
	StopReasonOtherPlaceholder            string `json:"stopReasonOtherPlaceholder"`
	ShowCustomerNameField                 bool   `json:"showCustomerNameField"`
	ShowCustomerPhoneField                bool   `json:"showCustomerPhoneField"`
	ShowEmailField                        bool   `json:"showEmailField"`
	ShowProfessionField                   bool   `json:"showProfessionField"`
	ShowNotesField                        bool   `json:"showNotesField"`
	ShowProductSeenField                  bool   `json:"showProductSeenField"`
	ShowProductSeenNotesField             bool   `json:"showProductSeenNotesField"`
	ShowProductClosedField                bool   `json:"showProductClosedField"`
	ShowPurchaseCodeField                 bool   `json:"showPurchaseCodeField"`
	ShowVisitReasonField                  bool   `json:"showVisitReasonField"`
	ShowCustomerSourceField               bool   `json:"showCustomerSourceField"`
	ShowExistingCustomerField             bool   `json:"showExistingCustomerField"`
	ShowQueueJumpReasonField              bool   `json:"showQueueJumpReasonField"`
	ShowLossReasonField                   bool   `json:"showLossReasonField"`
	ShowCancelReasonField                 bool   `json:"showCancelReasonField"`
	ShowStopReasonField                   bool   `json:"showStopReasonField"`
	AllowProductSeenNone                  bool   `json:"allowProductSeenNone"`
	VisitReasonSelectionMode              string `json:"visitReasonSelectionMode"`
	VisitReasonDetailMode                 string `json:"visitReasonDetailMode"`
	LossReasonSelectionMode               string `json:"lossReasonSelectionMode"`
	LossReasonDetailMode                  string `json:"lossReasonDetailMode"`
	CustomerSourceSelectionMode           string `json:"customerSourceSelectionMode"`
	CustomerSourceDetailMode              string `json:"customerSourceDetailMode"`
	CancelReasonInputMode                 string `json:"cancelReasonInputMode"`
	StopReasonInputMode                   string `json:"stopReasonInputMode"`
	RequireCustomerNameField              bool   `json:"requireCustomerNameField"`
	RequireCustomerPhoneField             bool   `json:"requireCustomerPhoneField"`
	RequireEmailField                     bool   `json:"requireEmailField"`
	RequireProfessionField                bool   `json:"requireProfessionField"`
	RequireNotesField                     bool   `json:"requireNotesField"`
	RequireProduct                        bool   `json:"requireProduct"`
	RequireProductSeenField               bool   `json:"requireProductSeenField"`
	RequireProductSeenNotesField          bool   `json:"requireProductSeenNotesField"`
	RequireProductClosedField             bool   `json:"requireProductClosedField"`
	RequirePurchaseCodeField              bool   `json:"requirePurchaseCodeField"`
	RequireVisitReason                    bool   `json:"requireVisitReason"`
	RequireCustomerSource                 bool   `json:"requireCustomerSource"`
	RequireCustomerNamePhone              bool   `json:"requireCustomerNamePhone"`
	RequireCustomerNameJustification      bool   `json:"requireCustomerNameJustification"`
	CustomerNameJustificationMinChars     int    `json:"customerNameJustificationMinChars"`
	RequireCustomerPhoneJustification     bool   `json:"requireCustomerPhoneJustification"`
	CustomerPhoneJustificationMinChars    int    `json:"customerPhoneJustificationMinChars"`
	RequireEmailJustification             bool   `json:"requireEmailJustification"`
	EmailJustificationMinChars            int    `json:"emailJustificationMinChars"`
	RequireProfessionJustification        bool   `json:"requireProfessionJustification"`
	ProfessionJustificationMinChars       int    `json:"professionJustificationMinChars"`
	RequireExistingCustomerJustification  bool   `json:"requireExistingCustomerJustification"`
	ExistingCustomerJustificationMinChars int    `json:"existingCustomerJustificationMinChars"`
	RequireNotesJustification             bool   `json:"requireNotesJustification"`
	NotesJustificationMinChars            int    `json:"notesJustificationMinChars"`
	RequireProductSeenJustification       bool   `json:"requireProductSeenJustification"`
	ProductSeenJustificationMinChars      int    `json:"productSeenJustificationMinChars"`
	RequireProductSeenNotesJustification  bool   `json:"requireProductSeenNotesJustification"`
	ProductSeenNotesJustificationMinChars int    `json:"productSeenNotesJustificationMinChars"`
	RequireProductClosedJustification     bool   `json:"requireProductClosedJustification"`
	ProductClosedJustificationMinChars    int    `json:"productClosedJustificationMinChars"`
	RequirePurchaseCodeJustification      bool   `json:"requirePurchaseCodeJustification"`
	PurchaseCodeJustificationMinChars     int    `json:"purchaseCodeJustificationMinChars"`
	RequireVisitReasonJustification       bool   `json:"requireVisitReasonJustification"`
	VisitReasonJustificationMinChars      int    `json:"visitReasonJustificationMinChars"`
	RequireCustomerSourceJustification    bool   `json:"requireCustomerSourceJustification"`
	CustomerSourceJustificationMinChars   int    `json:"customerSourceJustificationMinChars"`
	RequireProductSeenNotesWhenNone       bool   `json:"requireProductSeenNotesWhenNone"`
	ProductSeenNotesMinChars              int    `json:"productSeenNotesMinChars"`
	RequireQueueJumpReasonJustification   bool   `json:"requireQueueJumpReasonJustification"`
	QueueJumpReasonJustificationMinChars  int    `json:"queueJumpReasonJustificationMinChars"`
	RequireLossReasonJustification        bool   `json:"requireLossReasonJustification"`
	LossReasonJustificationMinChars       int    `json:"lossReasonJustificationMinChars"`
	RequireQueueJumpReasonField           bool   `json:"requireQueueJumpReasonField"`
	RequireLossReasonField                bool   `json:"requireLossReasonField"`
	RequireCancelReasonField              bool   `json:"requireCancelReasonField"`
	RequireStopReasonField                bool   `json:"requireStopReasonField"`
}

type ModalConfigPatch struct {
	Title                                 *string `json:"title,omitempty"`
	FinishFlowMode                        *string `json:"finishFlowMode,omitempty"`
	ProductSeenLabel                      *string `json:"productSeenLabel,omitempty"`
	ProductSeenPlaceholder                *string `json:"productSeenPlaceholder,omitempty"`
	ProductClosedLabel                    *string `json:"productClosedLabel,omitempty"`
	ProductClosedPlaceholder              *string `json:"productClosedPlaceholder,omitempty"`
	PurchaseCodeLabel                     *string `json:"purchaseCodeLabel,omitempty"`
	PurchaseCodePlaceholder               *string `json:"purchaseCodePlaceholder,omitempty"`
	NotesLabel                            *string `json:"notesLabel,omitempty"`
	NotesPlaceholder                      *string `json:"notesPlaceholder,omitempty"`
	QueueJumpReasonLabel                  *string `json:"queueJumpReasonLabel,omitempty"`
	QueueJumpReasonPlaceholder            *string `json:"queueJumpReasonPlaceholder,omitempty"`
	LossReasonLabel                       *string `json:"lossReasonLabel,omitempty"`
	LossReasonPlaceholder                 *string `json:"lossReasonPlaceholder,omitempty"`
	CustomerSectionLabel                  *string `json:"customerSectionLabel,omitempty"`
	CustomerNameLabel                     *string `json:"customerNameLabel,omitempty"`
	CustomerPhoneLabel                    *string `json:"customerPhoneLabel,omitempty"`
	CustomerEmailLabel                    *string `json:"customerEmailLabel,omitempty"`
	CustomerProfessionLabel               *string `json:"customerProfessionLabel,omitempty"`
	ExistingCustomerLabel                 *string `json:"existingCustomerLabel,omitempty"`
	ProductSeenNotesLabel                 *string `json:"productSeenNotesLabel,omitempty"`
	ProductSeenNotesPlaceholder           *string `json:"productSeenNotesPlaceholder,omitempty"`
	VisitReasonLabel                      *string `json:"visitReasonLabel,omitempty"`
	CustomerSourceLabel                   *string `json:"customerSourceLabel,omitempty"`
	CancelReasonLabel                     *string `json:"cancelReasonLabel,omitempty"`
	CancelReasonPlaceholder               *string `json:"cancelReasonPlaceholder,omitempty"`
	CancelReasonOtherLabel                *string `json:"cancelReasonOtherLabel,omitempty"`
	CancelReasonOtherPlaceholder          *string `json:"cancelReasonOtherPlaceholder,omitempty"`
	StopReasonLabel                       *string `json:"stopReasonLabel,omitempty"`
	StopReasonPlaceholder                 *string `json:"stopReasonPlaceholder,omitempty"`
	StopReasonOtherLabel                  *string `json:"stopReasonOtherLabel,omitempty"`
	StopReasonOtherPlaceholder            *string `json:"stopReasonOtherPlaceholder,omitempty"`
	ShowCustomerNameField                 *bool   `json:"showCustomerNameField,omitempty"`
	ShowCustomerPhoneField                *bool   `json:"showCustomerPhoneField,omitempty"`
	ShowEmailField                        *bool   `json:"showEmailField,omitempty"`
	ShowProfessionField                   *bool   `json:"showProfessionField,omitempty"`
	ShowNotesField                        *bool   `json:"showNotesField,omitempty"`
	ShowProductSeenField                  *bool   `json:"showProductSeenField,omitempty"`
	ShowProductSeenNotesField             *bool   `json:"showProductSeenNotesField,omitempty"`
	ShowProductClosedField                *bool   `json:"showProductClosedField,omitempty"`
	ShowPurchaseCodeField                 *bool   `json:"showPurchaseCodeField,omitempty"`
	ShowVisitReasonField                  *bool   `json:"showVisitReasonField,omitempty"`
	ShowCustomerSourceField               *bool   `json:"showCustomerSourceField,omitempty"`
	ShowExistingCustomerField             *bool   `json:"showExistingCustomerField,omitempty"`
	ShowQueueJumpReasonField              *bool   `json:"showQueueJumpReasonField,omitempty"`
	ShowLossReasonField                   *bool   `json:"showLossReasonField,omitempty"`
	ShowCancelReasonField                 *bool   `json:"showCancelReasonField,omitempty"`
	ShowStopReasonField                   *bool   `json:"showStopReasonField,omitempty"`
	AllowProductSeenNone                  *bool   `json:"allowProductSeenNone,omitempty"`
	VisitReasonSelectionMode              *string `json:"visitReasonSelectionMode,omitempty"`
	VisitReasonDetailMode                 *string `json:"visitReasonDetailMode,omitempty"`
	LossReasonSelectionMode               *string `json:"lossReasonSelectionMode,omitempty"`
	LossReasonDetailMode                  *string `json:"lossReasonDetailMode,omitempty"`
	CustomerSourceSelectionMode           *string `json:"customerSourceSelectionMode,omitempty"`
	CustomerSourceDetailMode              *string `json:"customerSourceDetailMode,omitempty"`
	CancelReasonInputMode                 *string `json:"cancelReasonInputMode,omitempty"`
	StopReasonInputMode                   *string `json:"stopReasonInputMode,omitempty"`
	RequireCustomerNameField              *bool   `json:"requireCustomerNameField,omitempty"`
	RequireCustomerPhoneField             *bool   `json:"requireCustomerPhoneField,omitempty"`
	RequireEmailField                     *bool   `json:"requireEmailField,omitempty"`
	RequireProfessionField                *bool   `json:"requireProfessionField,omitempty"`
	RequireNotesField                     *bool   `json:"requireNotesField,omitempty"`
	RequireProduct                        *bool   `json:"requireProduct,omitempty"`
	RequireProductSeenField               *bool   `json:"requireProductSeenField,omitempty"`
	RequireProductSeenNotesField          *bool   `json:"requireProductSeenNotesField,omitempty"`
	RequireProductClosedField             *bool   `json:"requireProductClosedField,omitempty"`
	RequirePurchaseCodeField              *bool   `json:"requirePurchaseCodeField,omitempty"`
	RequireVisitReason                    *bool   `json:"requireVisitReason,omitempty"`
	RequireCustomerSource                 *bool   `json:"requireCustomerSource,omitempty"`
	RequireCustomerNamePhone              *bool   `json:"requireCustomerNamePhone,omitempty"`
	RequireCustomerNameJustification      *bool   `json:"requireCustomerNameJustification,omitempty"`
	CustomerNameJustificationMinChars     *int    `json:"customerNameJustificationMinChars,omitempty"`
	RequireCustomerPhoneJustification     *bool   `json:"requireCustomerPhoneJustification,omitempty"`
	CustomerPhoneJustificationMinChars    *int    `json:"customerPhoneJustificationMinChars,omitempty"`
	RequireEmailJustification             *bool   `json:"requireEmailJustification,omitempty"`
	EmailJustificationMinChars            *int    `json:"emailJustificationMinChars,omitempty"`
	RequireProfessionJustification        *bool   `json:"requireProfessionJustification,omitempty"`
	ProfessionJustificationMinChars       *int    `json:"professionJustificationMinChars,omitempty"`
	RequireExistingCustomerJustification  *bool   `json:"requireExistingCustomerJustification,omitempty"`
	ExistingCustomerJustificationMinChars *int    `json:"existingCustomerJustificationMinChars,omitempty"`
	RequireNotesJustification             *bool   `json:"requireNotesJustification,omitempty"`
	NotesJustificationMinChars            *int    `json:"notesJustificationMinChars,omitempty"`
	RequireProductSeenJustification       *bool   `json:"requireProductSeenJustification,omitempty"`
	ProductSeenJustificationMinChars      *int    `json:"productSeenJustificationMinChars,omitempty"`
	RequireProductSeenNotesJustification  *bool   `json:"requireProductSeenNotesJustification,omitempty"`
	ProductSeenNotesJustificationMinChars *int    `json:"productSeenNotesJustificationMinChars,omitempty"`
	RequireProductClosedJustification     *bool   `json:"requireProductClosedJustification,omitempty"`
	ProductClosedJustificationMinChars    *int    `json:"productClosedJustificationMinChars,omitempty"`
	RequirePurchaseCodeJustification      *bool   `json:"requirePurchaseCodeJustification,omitempty"`
	PurchaseCodeJustificationMinChars     *int    `json:"purchaseCodeJustificationMinChars,omitempty"`
	RequireVisitReasonJustification       *bool   `json:"requireVisitReasonJustification,omitempty"`
	VisitReasonJustificationMinChars      *int    `json:"visitReasonJustificationMinChars,omitempty"`
	RequireCustomerSourceJustification    *bool   `json:"requireCustomerSourceJustification,omitempty"`
	CustomerSourceJustificationMinChars   *int    `json:"customerSourceJustificationMinChars,omitempty"`
	RequireProductSeenNotesWhenNone       *bool   `json:"requireProductSeenNotesWhenNone,omitempty"`
	ProductSeenNotesMinChars              *int    `json:"productSeenNotesMinChars,omitempty"`
	RequireQueueJumpReasonJustification   *bool   `json:"requireQueueJumpReasonJustification,omitempty"`
	QueueJumpReasonJustificationMinChars  *int    `json:"queueJumpReasonJustificationMinChars,omitempty"`
	RequireLossReasonJustification        *bool   `json:"requireLossReasonJustification,omitempty"`
	LossReasonJustificationMinChars       *int    `json:"lossReasonJustificationMinChars,omitempty"`
	RequireQueueJumpReasonField           *bool   `json:"requireQueueJumpReasonField,omitempty"`
	RequireLossReasonField                *bool   `json:"requireLossReasonField,omitempty"`
	RequireCancelReasonField              *bool   `json:"requireCancelReasonField,omitempty"`
	RequireStopReasonField                *bool   `json:"requireStopReasonField,omitempty"`
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
	CancelReasonOptions         []OptionItem        `json:"cancelReasonOptions"`
	StopReasonOptions           []OptionItem        `json:"stopReasonOptions"`
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

type OperationTemplateApplyInput struct {
	StoreID    string `json:"storeId,omitempty"`
	TenantID   string `json:"tenantId,omitempty"`
	TemplateID string `json:"templateId,omitempty"`
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

type OperationSectionRecord struct {
	TenantID                    string
	SelectedOperationTemplateID string
	CoreSettings                OperationCoreSettings
	AlertSettings               AlertSettings
	CreatedAt                   time.Time
	UpdatedAt                   time.Time
}

type ModalSectionRecord struct {
	TenantID                    string
	SelectedOperationTemplateID string
	ModalConfig                 ModalConfig
	CreatedAt                   time.Time
	UpdatedAt                   time.Time
}

type OperationTemplateApplyRecord struct {
	TenantID              string
	OperationSection      OperationSectionRecord
	ModalSection          ModalSectionRecord
	VisitReasonOptions    []OptionItem
	CustomerSourceOptions []OptionItem
}

type Record struct {
	TenantID                    string
	SelectedOperationTemplateID string
	Settings                    AppSettings
	ModalConfig                 ModalConfig
	VisitReasonOptions          []OptionItem
	CustomerSourceOptions       []OptionItem
	PauseReasonOptions          []OptionItem
	CancelReasonOptions         []OptionItem
	StopReasonOptions           []OptionItem
	QueueJumpReasonOptions      []OptionItem
	LossReasonOptions           []OptionItem
	ProfessionOptions           []OptionItem
	ProductCatalog              []ProductItem
	CreatedAt                   time.Time
	UpdatedAt                   time.Time
}

type Repository interface {
	TenantExists(ctx context.Context, tenantID string) (bool, error)
	CanAccessTenant(ctx context.Context, principal auth.Principal, tenantID string) (bool, error)
	ResolveDefaultTenantID(ctx context.Context, principal auth.Principal) (string, error)
	GetByTenant(ctx context.Context, tenantID string) (Record, bool, error)
	GetOperationSection(ctx context.Context, tenantID string) (OperationSectionRecord, bool, error)
	GetModalSection(ctx context.Context, tenantID string) (ModalSectionRecord, bool, error)
	GetOptionGroup(ctx context.Context, tenantID string, kind string) ([]OptionItem, error)
	GetProductCatalog(ctx context.Context, tenantID string) ([]ProductItem, error)
	Upsert(ctx context.Context, record Record) (Record, error)
	UpsertOperationSection(ctx context.Context, section OperationSectionRecord) (OperationSectionRecord, error)
	UpsertModalSection(ctx context.Context, section ModalSectionRecord) (ModalSectionRecord, error)
	ApplyOperationTemplate(ctx context.Context, record OperationTemplateApplyRecord) (time.Time, error)
	ReplaceOptionGroup(ctx context.Context, tenantID string, kind string, options []OptionItem) (time.Time, error)
	UpsertOption(ctx context.Context, tenantID string, kind string, option OptionItem) (time.Time, error)
	DeleteOption(ctx context.Context, tenantID string, kind string, optionID string) (time.Time, error)
	ReplaceProducts(ctx context.Context, tenantID string, products []ProductItem) (time.Time, error)
	UpsertProduct(ctx context.Context, tenantID string, product ProductItem) (time.Time, error)
	DeleteProduct(ctx context.Context, tenantID string, productID string) (time.Time, error)
}
