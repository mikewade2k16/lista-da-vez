package cardapio

import (
	"encoding/json"
	"time"
)

// DTOs em camelCase EXATO do contrato da API publica (front Nuxt do cardapio).
// Dinheiro sempre inteiro em centavos (...Cents). Campos jsonb chegam/saem como
// json.RawMessage para preservar a forma livre (address/hours/settings/theme/
// gallery/diet/allergens/pairing/tags) sem reescrever o shape do contrato.

// Address e o endereco do restaurante (forma fixa do contrato). number/complement/
// reference (WS-C) sao opcionais (omitempty) e vivem no mesmo jsonb address.
type Address struct {
	Street       string `json:"street"`
	Neighborhood string `json:"neighborhood"`
	City         string `json:"city"`
	State        string `json:"state"`
	Zip          string `json:"zip"`
	Number       string `json:"number,omitempty"`
	Complement   string `json:"complement,omitempty"`
	Reference    string `json:"reference,omitempty"`
}

// HourSpan e uma faixa de horario (dias + horas em texto livre).
type HourSpan struct {
	Days  string `json:"days"`
	Hours string `json:"hours"`
}

// Settings sao as regras comerciais do restaurante (entrega/retirada/local).
type Settings struct {
	DeliveryFeeCents       int64           `json:"deliveryFeeCents"`
	DeliveryEnabled        bool            `json:"deliveryEnabled"`
	PickupEnabled          bool            `json:"pickupEnabled"`
	DineInEnabled          bool            `json:"dineInEnabled"`
	MinOrderCents          int64           `json:"minOrderCents"`
	FreeDeliveryAboveCents int64           `json:"freeDeliveryAboveCents"`
	Payment                PaymentSettings `json:"payment"`
}

// PaymentSettings (WS-B) descreve as formas de pagamento aceitas. E INFORMATIVO:
// sai no menu publico para exibicao, mas NAO entra no checkout (o pedido nao
// escolhe forma de pagamento). jsonb dentro de settings — sem migration.
type PaymentSettings struct {
	Cash   bool        `json:"cash"`
	Debit  PaymentCard `json:"debit"`
	Credit PaymentCard `json:"credit"`
	Pix    bool        `json:"pix"`
	Ticket bool        `json:"ticket"`
	Other  string      `json:"other"`
}

// PaymentCard descreve um meio por cartao (debito/credito): aceito + bandeiras.
type PaymentCard struct {
	Accepted bool     `json:"accepted"`
	Brands   []string `json:"brands"`
}

// Restaurant e o DTO completo do restaurante.
type Restaurant struct {
	ID          string          `json:"id"`
	Slug        string          `json:"slug"`
	Name        string          `json:"name"`
	Tagline     string          `json:"tagline"`
	Description string          `json:"description"`
	LogoURL     string          `json:"logoUrl"`
	BannerURL   string          `json:"bannerUrl"`
	WhatsApp    string          `json:"whatsapp"`
	Phone       string          `json:"phone"`
	Email       string          `json:"email"`
	Instagram   string          `json:"instagram"`
	Address     Address         `json:"address"`
	Hours       []HourSpan      `json:"hours"`
	Settings    Settings        `json:"settings"`
	Theme       json.RawMessage `json:"theme"`
	// WS-C: campos faltantes (paridade lojatop) + estatisticas.
	Segment           string    `json:"segment"`
	Facebook          string    `json:"facebook"`
	Youtube           string    `json:"youtube"`
	GoogleAnalyticsID string    `json:"googleAnalyticsId"`
	FacebookPixelID   string    `json:"facebookPixelId"`
	CustomHeadHTML    string    `json:"customHeadHtml"`
	IsActive          bool      `json:"isActive"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

// RestaurantLean e a projecao enxuta da listagem do painel.
type RestaurantLean struct {
	ID            string    `json:"id"`
	AccountID     string    `json:"accountId"`
	AccountName   string    `json:"accountName"`
	Slug          string    `json:"slug"`
	Name          string    `json:"name"`
	IsActive      bool      `json:"isActive"`
	PrimaryDomain string    `json:"primaryDomain"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// Category e o DTO de categoria. imageUrl (WS-F) e foto representativa opcional;
// productCount (WS-F) e derivado no menu publico (omitempty: 0 => ausente, o
// front deriva localmente) e NAO tem coluna.
type Category struct {
	ID           string    `json:"id"`
	RestaurantID string    `json:"restaurantId"`
	Slug         string    `json:"slug"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	ImageURL     string    `json:"imageUrl"`
	SortOrder    int       `json:"sortOrder"`
	IsActive     bool      `json:"isActive"`
	CreatedAt    time.Time `json:"createdAt"`
	ProductCount int       `json:"productCount,omitempty"`
}

// Variation e uma opcao mutuamente exclusiva do produto.
type Variation struct {
	ID              string `json:"id"`
	ProductID       string `json:"productId"`
	Name            string `json:"name"`
	PriceDeltaCents int64  `json:"priceDeltaCents"`
	SortOrder       int    `json:"sortOrder"`
}

// Addon e um adicional cumulativo do produto.
type Addon struct {
	ID         string `json:"id"`
	ProductID  string `json:"productId"`
	Name       string `json:"name"`
	PriceCents int64  `json:"priceCents"`
	SortOrder  int    `json:"sortOrder"`
}

// Product e o DTO completo do prato.
type Product struct {
	ID           string  `json:"id"`
	RestaurantID string  `json:"restaurantId"`
	CategoryID   *string `json:"categoryId"`
	Slug         string  `json:"slug"`
	Name         string  `json:"name"`
	ShortDesc    string  `json:"shortDesc"`
	Description  string  `json:"description"`
	Body         string  `json:"body"`
	PriceCents   int64   `json:"priceCents"`
	// CompareAtPriceCents (WS-F): preco "cheio" para exibicao riscada (promocao).
	// omitempty => 0 = sem preco riscado. Nunca e usado como preco real.
	CompareAtPriceCents int64           `json:"compareAtPriceCents,omitempty"`
	ImageURL            string          `json:"imageUrl"`
	Gallery             []string        `json:"gallery"`
	Weight              string          `json:"weight"`
	CookTime            string          `json:"cookTime"`
	Diet                []string        `json:"diet"`
	Allergens           []string        `json:"allergens"`
	Pairing             json.RawMessage `json:"pairing"`
	Tags                []string        `json:"tags"`
	IsAvailable         bool            `json:"isAvailable"`
	IsFeatured          bool            `json:"isFeatured"`
	SortOrder           int             `json:"sortOrder"`
	Rating              *float64        `json:"rating"`
	ReviewCount         int             `json:"reviewCount"`
	SoldCount           int             `json:"soldCount"`
	CreatedAt           time.Time       `json:"createdAt"`
	UpdatedAt           time.Time       `json:"updatedAt"`
	Variations          []Variation     `json:"variations"`
	Addons              []Addon         `json:"addons"`
}

// ProductLean e a projecao enxuta da listagem de produtos do painel.
type ProductLean struct {
	ID          string  `json:"id"`
	CategoryID  *string `json:"categoryId"`
	Slug        string  `json:"slug"`
	Name        string  `json:"name"`
	PriceCents  int64   `json:"priceCents"`
	ImageURL    string  `json:"imageUrl"`
	IsAvailable bool    `json:"isAvailable"`
	IsFeatured  bool    `json:"isFeatured"`
	SortOrder   int     `json:"sortOrder"`
}

// Review e o DTO de avaliacao curada.
type Review struct {
	ID           string    `json:"id"`
	RestaurantID string    `json:"restaurantId"`
	ProductID    string    `json:"productId"`
	AuthorName   string    `json:"authorName"`
	AuthorLevel  string    `json:"authorLevel"`
	Rating       int       `json:"rating"`
	Body         string    `json:"body"`
	IsHighlight  bool      `json:"isHighlight"`
	DateLabel    string    `json:"dateLabel"`
	SortOrder    int       `json:"sortOrder"`
	CreatedAt    time.Time `json:"createdAt"`
}

// Domain e o DTO de dominio custom de um restaurante.
type Domain struct {
	Host         string    `json:"host"`
	RestaurantID string    `json:"restaurantId"`
	IsPrimary    bool      `json:"isPrimary"`
	CreatedAt    time.Time `json:"createdAt"`
}

// EventView e o DTO cru de um evento de telemetria (listagem do painel).
type EventView struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	SessionID string          `json:"sessionId"`
	Context   json.RawMessage `json:"context"`
	CreatedAt time.Time       `json:"createdAt"`
}

// DeliveryZone (WS-A) e um bairro com valor de entrega. centavos int64.
type DeliveryZone struct {
	ID           string `json:"id"`
	RestaurantID string `json:"restaurantId"`
	Name         string `json:"name"`
	FeeCents     int64  `json:"feeCents"`
	IsActive     bool   `json:"isActive"`
	SortOrder    int    `json:"sortOrder"`
}

// PublicMenu e a resposta de GET /v1/public/restaurants/{slug}.
type PublicMenu struct {
	Restaurant    Restaurant     `json:"restaurant"`
	Categories    []Category     `json:"categories"`
	Products      []Product      `json:"products"`
	DeliveryZones []DeliveryZone `json:"deliveryZones"`
}

// PublicProduct e a resposta de GET .../products/{productSlug}.
type PublicProduct struct {
	Restaurant Restaurant `json:"restaurant"`
	Product    Product    `json:"product"`
	Reviews    []Review   `json:"reviews"`
}

// ============================================================================
// Requests do painel
// ============================================================================

// CreateRestaurantInput cria um restaurante (nome + slug; accountId opcional, so
// admin escolhe; nao-admin usa a account do contexto).
type CreateRestaurantInput struct {
	AccountID string `json:"accountId"`
	Slug      string `json:"slug"`
	Name      string `json:"name"`
}

// UpdateRestaurantInput cobre os campos editaveis do restaurante. Ponteiros
// permitem PATCH parcial (campo ausente => preserva o valor atual).
// AccountID move o restaurante para outra conta (so platform_admin; espelha bio):
// nil/vazio/conta atual => nao move; o handler zera o campo para nao-admin.
type UpdateRestaurantInput struct {
	AccountID   *string          `json:"accountId"`
	Name        *string          `json:"name"`
	Tagline     *string          `json:"tagline"`
	Description *string          `json:"description"`
	LogoURL     *string          `json:"logoUrl"`
	BannerURL   *string          `json:"bannerUrl"`
	WhatsApp    *string          `json:"whatsapp"`
	Phone       *string          `json:"phone"`
	Email       *string          `json:"email"`
	Instagram   *string          `json:"instagram"`
	Address     *Address         `json:"address"`
	Hours       *[]HourSpan      `json:"hours"`
	Settings    *Settings        `json:"settings"`
	Theme       *json.RawMessage `json:"theme"`
	// WS-C: campos faltantes (paridade lojatop) + estatisticas.
	Segment           *string `json:"segment"`
	Facebook          *string `json:"facebook"`
	Youtube           *string `json:"youtube"`
	GoogleAnalyticsID *string `json:"googleAnalyticsId"`
	FacebookPixelID   *string `json:"facebookPixelId"`
	CustomHeadHTML    *string `json:"customHeadHtml"`
	IsActive          *bool   `json:"isActive"`
}

// CategoryInput cria/edita categoria (full-replace: o painel manda o body
// completo, incluindo imageUrl — sem ele, zera a foto).
type CategoryInput struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ImageURL    string `json:"imageUrl"`
	SortOrder   int    `json:"sortOrder"`
	IsActive    bool   `json:"isActive"`
}

// VariationInput e um item da lista replace-all de variacoes no PATCH do produto.
type VariationInput struct {
	Name            string `json:"name"`
	PriceDeltaCents int64  `json:"priceDeltaCents"`
	SortOrder       int    `json:"sortOrder"`
}

// AddonInput e um item da lista replace-all de adicionais no PATCH do produto.
type AddonInput struct {
	Name       string `json:"name"`
	PriceCents int64  `json:"priceCents"`
	SortOrder  int    `json:"sortOrder"`
}

// ProductInput cria/edita produto. variations/addons (quando nao-nil) substituem
// todas as opcoes existentes (replace-all transacional).
type ProductInput struct {
	CategoryID          *string          `json:"categoryId"`
	Slug                string           `json:"slug"`
	Name                string           `json:"name"`
	ShortDesc           string           `json:"shortDesc"`
	Description         string           `json:"description"`
	Body                string           `json:"body"`
	PriceCents          int64            `json:"priceCents"`
	CompareAtPriceCents int64            `json:"compareAtPriceCents"`
	ImageURL            string           `json:"imageUrl"`
	Gallery             []string         `json:"gallery"`
	Weight              string           `json:"weight"`
	CookTime            string           `json:"cookTime"`
	Diet                []string         `json:"diet"`
	Allergens           []string         `json:"allergens"`
	Pairing             json.RawMessage  `json:"pairing"`
	Tags                []string         `json:"tags"`
	IsAvailable         bool             `json:"isAvailable"`
	IsFeatured          bool             `json:"isFeatured"`
	SortOrder           int              `json:"sortOrder"`
	Variations          []VariationInput `json:"variations"`
	Addons              []AddonInput     `json:"addons"`
}

// ReviewInput cria/edita avaliacao.
type ReviewInput struct {
	ProductID   string `json:"productId"`
	AuthorName  string `json:"authorName"`
	AuthorLevel string `json:"authorLevel"`
	Rating      int    `json:"rating"`
	Body        string `json:"body"`
	IsHighlight bool   `json:"isHighlight"`
	DateLabel   string `json:"dateLabel"`
	SortOrder   int    `json:"sortOrder"`
}

// DomainInput cria um dominio custom.
type DomainInput struct {
	Host      string `json:"host"`
	IsPrimary bool   `json:"isPrimary"`
}

// DeliveryZoneInput cria uma zona de entrega (WS-A).
type DeliveryZoneInput struct {
	Name      string `json:"name"`
	FeeCents  int64  `json:"feeCents"`
	IsActive  bool   `json:"isActive"`
	SortOrder int    `json:"sortOrder"`
}

// UpdateDeliveryZoneInput e o PATCH parcial de uma zona (WS-A). Ponteiros => so
// os campos enviados mudam (toggle de is_active nao precisa do body inteiro).
type UpdateDeliveryZoneInput struct {
	Name      *string `json:"name"`
	FeeCents  *int64  `json:"feeCents"`
	IsActive  *bool   `json:"isActive"`
	SortOrder *int    `json:"sortOrder"`
}
