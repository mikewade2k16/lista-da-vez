package tools

import "errors"

// Sentinelas do modulo tools; o handler HTTP mapeia para 404/400/409.
var (
	// ErrShortLinkNotFound: link inexistente OU fora da account do request
	// (404, nao 403 — nao vaza existencia cross-tenant).
	ErrShortLinkNotFound = errors.New("tools: short link not found")
	// ErrQrCodeNotFound: QR inexistente OU fora da account do request (404).
	ErrQrCodeNotFound = errors.New("tools: qr code not found")
	// ErrInvalidTargetURL: URL de destino ausente/invalida (400).
	ErrInvalidTargetURL = errors.New("tools: invalid target url")
	// ErrAccountRequired: create sem account resolvida (admin em platform view
	// que nao escolheu conta dona) — 400.
	ErrAccountRequired = errors.New("tools: account required")
	// ErrInvalidSlug: slug vazio/sem caractere valido numa edicao (400). No
	// create, slug vazio continua gerando um codigo aleatorio.
	ErrInvalidSlug = errors.New("tools: invalid slug")
	// ErrQrSlugImmutable: o slug de um QR nao muda depois de criado (409). O QR
	// impresso codifica /q/{slug}; trocar o slug quebraria todos os impressos.
	ErrQrSlugImmutable = errors.New("tools: qr slug is immutable")
	// ErrSlugChangeNotConfirmed: renomear um link curto derruba o link antigo,
	// entao o PATCH exige confirmSlugChange=true (409).
	ErrSlugChangeNotConfirmed = errors.New("tools: slug change requires confirmation")
	// ErrInvalidColor: cor fora do formato #rrggbb (400). Nunca cai num default
	// silencioso.
	ErrInvalidColor = errors.New("tools: invalid color")
	// ErrLowContrast: cores que o leitor de QR nao consegue distinguir (400).
	ErrLowContrast = errors.New("tools: low qr contrast")
	// ErrConcurrentChange: as cores mudaram entre a leitura e a gravacao (409).
	ErrConcurrentChange = errors.New("tools: concurrent change")
)
