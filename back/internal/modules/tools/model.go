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
	IsActive   bool   `json:"isActive"` // desativado: /s/{slug} responde 404
	LastHitAt  string `json:"lastHitAt"`
	CreatedAt  string `json:"createdAt"`
	AccountID  string `json:"accountId"`
	ClientName string `json:"clientName"` // core.accounts.name
}

// ShortLinkInput e o corpo de POST/PATCH /v1/tools/short-links. Todos opcionais
// (ponteiro = ausente permitido; DisallowUnknownFields recusa extras). No POST,
// targetUrl obrigatorio; no PATCH, ausentes ficam intactos. Trocar o slug no
// PATCH exige confirmSlugChange=true (o link antigo para de funcionar).
type ShortLinkInput struct {
	TargetURL         *string `json:"targetUrl"`
	Slug              *string `json:"slug"`
	IsActive          *bool   `json:"isActive"`
	ConfirmSlugChange *bool   `json:"confirmSlugChange"`
	AccountID         *string `json:"accountId"`
}

// ShortLinkPatch sao os campos normalizados para o UPDATE (nil = nao mexe).
type ShortLinkPatch struct {
	Slug      *string
	TargetURL *string
	IsActive  *bool
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
// O slug so e definido no POST: no PATCH, slug diferente do atual e recusado
// (ErrQrSlugImmutable); igual ao atual e aceito como no-op.
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

// QrCodePatch sao os campos normalizados para o UPDATE (nil = nao mexe). Sem
// slug: ele e imutavel depois do create. ExpectFill/ExpectBack, quando
// preenchidos, viram condicao do UPDATE (as cores validadas sao as gravadas).
type QrCodePatch struct {
	TargetURL *string
	FillColor *string
	BackColor *string
	Size      *int
	IsActive  *bool

	ExpectFill string
	ExpectBack string
}

// ListFilter parametriza a listagem (q busca slug/destino/cliente; status so no QR).
type ListFilter struct {
	Q      string
	Status string // "active" | "inactive" | "" (QR e links curtos)
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
