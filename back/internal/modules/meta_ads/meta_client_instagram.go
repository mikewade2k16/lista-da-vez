package metaads

import (
	"context"
	"net/url"
	"strconv"
)

// igPaginationMaxPages limita quantas paginas de /me/accounts o bridge percorre
// ao descobrir paginas com Instagram Business (evita varredura ilimitada).
const igPaginationMaxPages = 3

// GraphInstagramPage e uma pagina do Facebook (em /me/accounts) com a conta de
// Instagram Business vinculada, quando houver. Paginas sem instagram_business_account
// NAO entram no resultado de ListPagesWithInstagram.
type GraphInstagramPage struct {
	PageID     string
	PageName   string
	IGUserID   string
	IGUsername string
}

// GraphInstagramMedia e uma postagem do feed do Instagram Business
// (/{ig-user-id}/media). Campos opcionais da Graph (caption, media_url,
// thumbnail_url) podem vir ausentes — tratados como string vazia.
type GraphInstagramMedia struct {
	ID           string `json:"id"`
	Caption      string `json:"caption"`
	MediaType    string `json:"media_type"`
	MediaURL     string `json:"media_url"`
	ThumbnailURL string `json:"thumbnail_url"`
	Permalink    string `json:"permalink"`
	Timestamp    string `json:"timestamp"`
}

// rawPageAccount e o shape bruto de uma pagina em /me/accounts. O campo
// instagram_business_account so existe quando a pagina tem IG Business vinculado.
type rawPageAccount struct {
	ID                       string `json:"id"`
	Name                     string `json:"name"`
	InstagramBusinessAccount *struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	} `json:"instagram_business_account"`
}

// pagedAccounts e o envelope de /me/accounts com o cursor de paginacao. Pagina
// pelo cursor "after" (re-request no mesmo path/base) em vez do link absoluto
// paging.next — esse link ja inclui o prefixo de versao e duplicaria o base.
type pagedAccounts struct {
	Data   []rawPageAccount `json:"data"`
	Paging struct {
		Cursors struct {
			After string `json:"after"`
		} `json:"cursors"`
	} `json:"paging"`
}

// ListPagesWithInstagram lista as paginas do Facebook acessiveis pelo token que
// possuem uma conta de Instagram Business vinculada. Segue o cursor "after" ate
// igPaginationMaxPages. O token nunca aparece em log/erro (getJSON o passa
// internamente e graphError so ecoa a mensagem da Graph).
func (c *MetaClient) ListPagesWithInstagram(ctx context.Context, token string) ([]GraphInstagramPage, error) {
	var pages []GraphInstagramPage
	after := ""

	for page := 0; page < igPaginationMaxPages; page++ {
		q := url.Values{}
		q.Set("fields", "id,name,instagram_business_account{id,username}")
		q.Set("limit", "50")
		if after != "" {
			q.Set("after", after)
		}

		var out pagedAccounts
		if err := c.getJSON(ctx, "/me/accounts", token, q, &out); err != nil {
			return nil, err
		}
		for _, p := range out.Data {
			if p.InstagramBusinessAccount == nil || p.InstagramBusinessAccount.ID == "" {
				continue
			}
			pages = append(pages, GraphInstagramPage{
				PageID:     p.ID,
				PageName:   p.Name,
				IGUserID:   p.InstagramBusinessAccount.ID,
				IGUsername: p.InstagramBusinessAccount.Username,
			})
		}
		after = out.Paging.Cursors.After
		if after == "" {
			break
		}
	}
	return pages, nil
}

// ListInstagramMedia lista as postagens recentes do feed de uma conta de
// Instagram Business. limit ja vem validado (1..20) do service.
func (c *MetaClient) ListInstagramMedia(ctx context.Context, token, igUserID string, limit int) ([]GraphInstagramMedia, error) {
	q := url.Values{}
	q.Set("fields", "id,caption,media_type,media_url,thumbnail_url,permalink,timestamp")
	q.Set("limit", strconv.Itoa(limit))

	var out struct {
		Data []GraphInstagramMedia `json:"data"`
	}
	if err := c.getJSON(ctx, "/"+igUserID+"/media", token, q, &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}
