package tools

// Structs do modulo tools. JSON camelCase = contrato do front. Slices/strings
// sempre nao-nil (o front espera valor, nunca null).

// ShortLinkItem e o DTO de um link curto.
type ShortLinkItem struct {
	ID         string `json:"id"`
	Slug       string `json:"slug"`
	TargetURL  string `json:"targetUrl"`
	ShortURL   string `json:"shortUrl"` // {publicBase}/s/{slug}
	Hits       int64  `json:"hits"`
	CreatedAt  string `json:"createdAt"`
	AccountID  string `json:"accountId"`
	ClientName string `json:"clientName"` // core.accounts.name
}

// ShortLinkInput e o corpo de POST/PATCH /v1/tools/short-links. Todos opcionais
// (ponteiro = ausente permitido; DisallowUnknownFields recusa extras). No POST,
// targetUrl obrigatorio; no PATCH, ausentes ficam intactos.
type ShortLinkInput struct {
	TargetURL *string `json:"targetUrl"`
	Slug      *string `json:"slug"`
	AccountID *string `json:"accountId"`
}

// ShortLinkPatch sao os campos normalizados para o UPDATE (nil = nao mexe).
type ShortLinkPatch struct {
	Slug      *string
	TargetURL *string
}

// QrCodeItem e o DTO de um QR code. qrUrl e o que o QR codifica (redirect
// rastreado); a imagem PNG e gerada no cliente a partir dele + cores + size.
type QrCodeItem struct {
	ID            string `json:"id"`
	Slug          string `json:"slug"`
	TargetURL     string `json:"targetUrl"`
	QrURL         string `json:"qrUrl"` // {publicBase}/q/{slug}
	FillColor     string `json:"fillColor"`
	BackColor     string `json:"backColor"`
	Size          int    `json:"size"`
	IsActive      bool   `json:"isActive"`
	ScanCount     int64  `json:"scanCount"`
	LastScannedAt string `json:"lastScannedAt"`
	CreatedAt     string `json:"createdAt"`
	AccountID     string `json:"accountId"`
	ClientName    string `json:"clientName"`
}

// QrCodeInput e o corpo de POST/PATCH /v1/tools/qr-codes. No POST, ausentes caem
// no default; no PATCH, ausentes ficam intactos. targetUrl obrigatorio no POST.
type QrCodeInput struct {
	TargetURL *string `json:"targetUrl"`
	Slug      *string `json:"slug"`
	FillColor *string `json:"fillColor"`
	BackColor *string `json:"backColor"`
	Size      *int    `json:"size"`
	IsActive  *bool   `json:"isActive"`
	AccountID *string `json:"accountId"`
}

// QrCodeRecord sao os campos normalizados para o INSERT.
type QrCodeRecord struct {
	Slug      string
	TargetURL string
	FillColor string
	BackColor string
	Size      int
	IsActive  bool
}

// QrCodePatch sao os campos normalizados para o UPDATE (nil = nao mexe).
type QrCodePatch struct {
	Slug      *string
	TargetURL *string
	FillColor *string
	BackColor *string
	Size      *int
	IsActive  *bool
}

// ListFilter parametriza a listagem (q busca slug/destino/cliente; status so no QR).
type ListFilter struct {
	Q      string
	Status string // qr: "active" | "inactive" | ""
	Page   int
	Limit  int
}

// ListMeta e o bloco de paginacao da resposta de listagem.
type ListMeta struct {
	Page       int  `json:"page"`
	Limit      int  `json:"limit"`
	Total      int  `json:"total"`
	TotalPages int  `json:"totalPages"`
	HasMore    bool `json:"hasMore"`
}
