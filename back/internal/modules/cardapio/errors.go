package cardapio

import "errors"

// Erros de dominio do modulo cardapio. O service traduz para HTTP (404 uniforme
// fora do escopo; 400 de validacao; formato {"error":{code,message}}).
var (
	// ErrNotFound: recurso inexistente OU fora do escopo da account (404 uniforme).
	ErrNotFound = errors.New("cardapio: not found")

	// ErrForbidden: accountId pedido fora do permitido para o Principal.
	ErrForbidden = errors.New("cardapio: forbidden")

	// ErrValidation: input invalido (slug/nome vazio, status invalido etc.).
	ErrValidation = errors.New("cardapio: validation")

	// ErrSlugConflict: slug duplicado (restaurante global ou produto/categoria no
	// restaurante).
	ErrSlugConflict = errors.New("cardapio: slug conflict")

	// ErrInvalidMedia: upload invalido (tamanho/mime fora da allowlist).
	ErrInvalidMedia = errors.New("cardapio: invalid media")
)
