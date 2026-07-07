package finance

import "errors"

// Sentinelas usadas pelos stores/service; o handler HTTP mapeia para 404.
var (
	// ErrSheetNotFound: planilha inexistente OU fora da account do request
	// (404, nao 403 — nao vaza existencia cross-tenant).
	ErrSheetNotFound = errors.New("finance: sheet not found")
	// ErrLineNotFound: linha inexistente na planilha alvo (404).
	ErrLineNotFound = errors.New("finance: line not found")
)
