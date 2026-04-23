package settings

import (
	"context"
	"time"
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
	Title                       string `json:"title"`
	ProductSeenLabel            string `json:"productSeenLabel"`
	ProductSeenPlaceholder      string `json:"productSeenPlaceholder"`
	ProductClosedLabel          string `json:"productClosedLabel"`
	ProductClosedPlaceholder    string `json:"productClosedPlaceholder"`
	NotesLabel                  string `json:"notesLabel"`
	NotesPlaceholder            string `json:"notesPlaceholder"`
	QueueJumpReasonLabel        string `json:"queueJumpReasonLabel"`
	QueueJumpReasonPlaceholder  string `json:"queueJumpReasonPlaceholder"`
	LossReasonLabel             string `json:"lossReasonLabel"`
	LossReasonPlaceholder       string `json:"lossReasonPlaceholder"`
	CustomerSectionLabel        string `json:"customerSectionLabel"`
	ShowEmailField              bool   `json:"showEmailField"`
	ShowProfessionField         bool   `json:"showProfessionField"`
	ShowNotesField              bool   `json:"showNotesField"`
	VisitReasonSelectionMode    string `json:"visitReasonSelectionMode"`
	VisitReasonDetailMode       string `json:"visitReasonDetailMode"`
	LossReasonSelectionMode     string `json:"lossReasonSelectionMode"`
	LossReasonDetailMode        string `json:"lossReasonDetailMode"`
	CustomerSourceSelectionMode string `json:"customerSourceSelectionMode"`
	CustomerSourceDetailMode    string `json:"customerSourceDetailMode"`
	RequireProduct              bool   `json:"requireProduct"`
	RequireVisitReason          bool   `json:"requireVisitReason"`
	RequireCustomerSource       bool   `json:"requireCustomerSource"`
	RequireCustomerNamePhone    bool   `json:"requireCustomerNamePhone"`
}

type ModalConfigPatch struct {
	Title                       *string `json:"title,omitempty"`
	ProductSeenLabel            *string `json:"productSeenLabel,omitempty"`
	ProductSeenPlaceholder      *string `json:"productSeenPlaceholder,omitempty"`
	ProductClosedLabel          *string `json:"productClosedLabel,omitempty"`
	ProductClosedPlaceholder    *string `json:"productClosedPlaceholder,omitempty"`
	NotesLabel                  *string `json:"notesLabel,omitempty"`
	NotesPlaceholder            *string `json:"notesPlaceholder,omitempty"`
	QueueJumpReasonLabel        *string `json:"queueJumpReasonLabel,omitempty"`
	QueueJumpReasonPlaceholder  *string `json:"queueJumpReasonPlaceholder,omitempty"`
	LossReasonLabel             *string `json:"lossReasonLabel,omitempty"`
	LossReasonPlaceholder       *string `json:"lossReasonPlaceholder,omitempty"`
	CustomerSectionLabel        *string `json:"customerSectionLabel,omitempty"`
	ShowEmailField              *bool   `json:"showEmailField,omitempty"`
	ShowProfessionField         *bool   `json:"showProfessionField,omitempty"`
	ShowNotesField              *bool   `json:"showNotesField,omitempty"`
	VisitReasonSelectionMode    *string `json:"visitReasonSelectionMode,omitempty"`
	VisitReasonDetailMode       *string `json:"visitReasonDetailMode,omitempty"`
	LossReasonSelectionMode     *string `json:"lossReasonSelectionMode,omitempty"`
	LossReasonDetailMode        *string `json:"lossReasonDetailMode,omitempty"`
	CustomerSourceSelectionMode *string `json:"customerSourceSelectionMode,omitempty"`
	CustomerSourceDetailMode    *string `json:"customerSourceDetailMode,omitempty"`
	RequireProduct              *bool   `json:"requireProduct,omitempty"`
	RequireVisitReason          *bool   `json:"requireVisitReason,omitempty"`
	RequireCustomerSource       *bool   `json:"requireCustomerSource,omitempty"`
	RequireCustomerNamePhone    *bool   `json:"requireCustomerNamePhone,omitempty"`
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

type Bundle struct {
	StoreID                     string              `json:"storeId"`
	OperationTemplates          []OperationTemplate `json:"operationTemplates,omitempty"`
	SelectedOperationTemplateID string              `json:"selectedOperationTemplateId"`
	Settings                    AppSettings         `json:"settings"`
	ModalConfig                 ModalConfig         `json:"modalConfig"`
	VisitReasonOptions          []OptionItem        `json:"visitReasonOptions"`
	CustomerSourceOptions       []OptionItem        `json:"customerSourceOptions"`
	QueueJumpReasonOptions      []OptionItem        `json:"queueJumpReasonOptions"`
	LossReasonOptions           []OptionItem        `json:"lossReasonOptions"`
	ProfessionOptions           []OptionItem        `json:"professionOptions"`
	ProductCatalog              []ProductItem       `json:"productCatalog"`
}

type OperationSectionInput struct {
	StoreID                     string            `json:"storeId"`
	SelectedOperationTemplateID *string           `json:"selectedOperationTemplateId,omitempty"`
	Settings                    *AppSettingsPatch `json:"settings,omitempty"`
}

type ModalSectionInput struct {
	StoreID     string            `json:"storeId"`
	ModalConfig *ModalConfigPatch `json:"modalConfig,omitempty"`
}

type OptionSectionInput struct {
	StoreID string       `json:"storeId"`
	Items   []OptionItem `json:"items"`
}

type OptionItemInput struct {
	StoreID string     `json:"storeId"`
	Item    OptionItem `json:"item"`
}

type OptionItemPatchInput struct {
	StoreID string `json:"storeId"`
	Label   string `json:"label"`
}

type ProductSectionInput struct {
	StoreID string        `json:"storeId"`
	Items   []ProductItem `json:"items"`
}

type ProductItemInput struct {
	StoreID string      `json:"storeId"`
	Item    ProductItem `json:"item"`
}

type ProductItemPatchInput struct {
	StoreID   string  `json:"storeId"`
	Name      string  `json:"name"`
	Code      string  `json:"code"`
	Category  string  `json:"category"`
	BasePrice float64 `json:"basePrice"`
}

type MutationAck struct {
	OK      bool      `json:"ok"`
	StoreID string    `json:"storeId"`
	SavedAt time.Time `json:"savedAt"`
}

type Record struct {
	StoreID                     string
	SelectedOperationTemplateID string
	Settings                    AppSettings
	ModalConfig                 ModalConfig
	VisitReasonOptions          []OptionItem
	CustomerSourceOptions       []OptionItem
	QueueJumpReasonOptions      []OptionItem
	LossReasonOptions           []OptionItem
	ProfessionOptions           []OptionItem
	ProductCatalog              []ProductItem
	CreatedAt                   time.Time
	UpdatedAt                   time.Time
}

type Repository interface {
	StoreExists(ctx context.Context, storeID string) (bool, error)
	GetByStore(ctx context.Context, storeID string) (Record, bool, error)
	Upsert(ctx context.Context, record Record) (Record, error)
	UpsertConfig(ctx context.Context, record Record) (Record, error)
	ReplaceOptionGroup(ctx context.Context, storeID string, kind string, options []OptionItem) (time.Time, error)
	UpsertOption(ctx context.Context, storeID string, kind string, option OptionItem) (time.Time, error)
	DeleteOption(ctx context.Context, storeID string, kind string, optionID string) (time.Time, error)
	ReplaceProducts(ctx context.Context, storeID string, products []ProductItem) (time.Time, error)
	UpsertProduct(ctx context.Context, storeID string, product ProductItem) (time.Time, error)
	DeleteProduct(ctx context.Context, storeID string, productID string) (time.Time, error)
}
