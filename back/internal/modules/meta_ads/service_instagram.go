package metaads

import (
	"context"
	"strings"
)

// Limites do feed do Instagram para o bridge do runner.
const (
	instagramMediaDefaultLimit = 5  // limit ausente/invalido no bridge
	instagramMediaMaxLimit     = 20 // teto do contrato (clamp)
)

// InstagramAccountView e uma conta de Instagram Business para o bridge do runner.
// Tags JSON CONGELADAS com o contrato do runner (NAO alterar).
type InstagramAccountView struct {
	IGUserID string `json:"igUserId"`
	Username string `json:"username"`
	PageID   string `json:"pageId"`
	PageName string `json:"pageName"`
}

// InstagramMediaView e uma postagem do feed para o bridge do runner. Tags JSON
// CONGELADAS com o contrato (NAO alterar). Campos ausentes na Graph viram string
// vazia (ex.: thumbnailUrl so existe para VIDEO).
type InstagramMediaView struct {
	ID           string `json:"id"`
	Caption      string `json:"caption"`
	MediaType    string `json:"mediaType"`
	MediaURL     string `json:"mediaUrl"`
	ThumbnailURL string `json:"thumbnailUrl"`
	Permalink    string `json:"permalink"`
	Timestamp    string `json:"timestamp"`
}

// InstagramAccounts lista as contas de Instagram Business acessiveis pela conexao
// Meta da account. Sem conexao ativa -> ErrNotConnected (handler -> 404). O token
// e decifrado sob demanda (mesmo caminho do sync) e nunca logado.
func (s *Service) InstagramAccounts(ctx context.Context, accountID string) ([]InstagramAccountView, error) {
	token, err := s.connectionToken(ctx, accountID)
	if err != nil {
		return nil, err
	}
	pages, err := s.client.ListPagesWithInstagram(ctx, token)
	if err != nil {
		return nil, err
	}
	views := make([]InstagramAccountView, 0, len(pages))
	for _, p := range pages {
		views = append(views, InstagramAccountView{
			IGUserID: p.IGUserID,
			Username: p.IGUsername,
			PageID:   p.PageID,
			PageName: p.PageName,
		})
	}
	return views, nil
}

// InstagramMedia lista as postagens recentes de uma conta de Instagram Business.
// igUserID vazio = primeira conta IG disponivel da conexao. Sem conexao ->
// ErrNotConnected (404). limit fora de 1..20 -> clamp.
func (s *Service) InstagramMedia(ctx context.Context, accountID, igUserID string, limit int) ([]InstagramMediaView, error) {
	token, err := s.connectionToken(ctx, accountID)
	if err != nil {
		return nil, err
	}

	igUserID = strings.TrimSpace(igUserID)
	if igUserID == "" {
		pages, pErr := s.client.ListPagesWithInstagram(ctx, token)
		if pErr != nil {
			return nil, pErr
		}
		if len(pages) == 0 {
			// Conexao viva mas sem nenhuma conta IG vinculada: lista vazia (nao erro).
			return []InstagramMediaView{}, nil
		}
		igUserID = pages[0].IGUserID
	}

	media, err := s.client.ListInstagramMedia(ctx, token, igUserID, clampMediaLimit(limit))
	if err != nil {
		return nil, err
	}
	views := make([]InstagramMediaView, 0, len(media))
	for _, m := range media {
		// GraphInstagramMedia e InstagramMediaView tem os mesmos campos/ordem; a
		// diferenca e so nas tags json (snake_case da Graph -> camelCase do
		// contrato). A conversao de tipo copia os valores ignorando as tags.
		views = append(views, InstagramMediaView(m))
	}
	return views, nil
}

// connectionToken garante conexao ativa e devolve o token decifrado. Sem conexao
// -> ErrNotConnected (handler -> 404 not_connected). Mesmo caminho que o sync usa.
func (s *Service) connectionToken(ctx context.Context, accountID string) (string, error) {
	if _, err := s.store.GetConnection(ctx, accountID); err != nil {
		if noRows(err) {
			return "", ErrNotConnected
		}
		return "", err
	}
	return s.store.GetDecryptedToken(ctx, accountID)
}

// clampMediaLimit prende o limit do feed em [1, 20] (default 5 quando <= 0).
func clampMediaLimit(limit int) int {
	switch {
	case limit <= 0:
		return instagramMediaDefaultLimit
	case limit > instagramMediaMaxLimit:
		return instagramMediaMaxLimit
	default:
		return limit
	}
}
