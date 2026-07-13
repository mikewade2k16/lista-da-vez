package tools

import "errors"

// Sentinelas do modulo tools; o handler HTTP mapeia para 404/400.
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
)
