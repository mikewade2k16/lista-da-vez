package metaads

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxOAuthResponseBytes = 1 << 20

type oauthTokenExchanger interface {
	ExchangeCode(ctx context.Context, appID, appSecret, redirectURI, code string) (OAuthAccessToken, error)
	ListPermissions(ctx context.Context, token string) ([]OAuthPermission, error)
}

// MetaOAuthClient implementa as trocas server-side do Facebook Login e a
// verificacao dos grants reais. Segredos seguem no body form-urlencoded ou no
// header Bearer (nunca na URL) e a resposta nao sai desta camada.
type MetaOAuthClient struct {
	base string
	http *http.Client
	now  func() time.Time
}

func NewMetaOAuthClient(graphBase string) *MetaOAuthClient {
	return &MetaOAuthClient{
		base: strings.TrimRight(graphBase, "/"),
		http: &http.Client{Timeout: 15 * time.Second},
		now:  time.Now,
	}
}

func (c *MetaOAuthClient) ExchangeCode(
	ctx context.Context,
	appID string,
	appSecret string,
	redirectURI string,
	code string,
) (OAuthAccessToken, error) {
	short, err := c.exchange(ctx, url.Values{
		"client_id":     {appID},
		"client_secret": {appSecret},
		"redirect_uri":  {redirectURI},
		"code":          {code},
	})
	if err != nil {
		return OAuthAccessToken{}, err
	}

	// Facebook Login emite normalmente um user token curto. A segunda troca o
	// converte para longa duracao usando somente credenciais server-side.
	longLived, err := c.exchange(ctx, url.Values{
		"grant_type":        {"fb_exchange_token"},
		"client_id":         {appID},
		"client_secret":     {appSecret},
		"fb_exchange_token": {short.AccessToken},
	})
	if err != nil {
		return OAuthAccessToken{}, err
	}

	expiresAt := time.Time{}
	if longLived.ExpiresIn > 0 {
		expiresAt = c.now().UTC().Add(time.Duration(longLived.ExpiresIn) * time.Second)
	}
	return OAuthAccessToken{Value: longLived.AccessToken, ExpiresAt: expiresAt}, nil
}

// ListPermissions consulta os grants reais do user token. O token segue
// exclusivamente no header Authorization e nunca integra URL, query ou erro.
func (c *MetaOAuthClient) ListPermissions(ctx context.Context, token string) ([]OAuthPermission, error) {
	return listMetaPermissions(ctx, c.base, c.http, token)
}

// listMetaPermissions e compartilhado pelo callback OAuth e pelo token manual.
// A resposta livre da Meta nunca e propagada: somente um status HTTP generico
// atravessa esta fronteira.
func listMetaPermissions(
	ctx context.Context,
	base string,
	client *http.Client,
	token string,
) ([]OAuthPermission, error) {
	endpoint := strings.TrimRight(base, "/") + "/me/permissions"
	// G704: a base vem exclusivamente de configuracao server-side. O path e
	// fixo e nenhum dado do callback participa da URL.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("meta graph: criar consulta de permissoes")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req) //nolint:gosec // host de config confiavel, nao de input
	if err != nil {
		return nil, fmt.Errorf("meta graph: consultar permissoes")
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxOAuthResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("meta graph: ler permissoes")
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		// Nao reutilize graphError aqui: a mensagem livre do provider poderia
		// refletir dados sensiveis. O status e suficiente para o mapeamento HTTP.
		return nil, fmt.Errorf("meta graph: permissions http %d", resp.StatusCode)
	}

	var payload struct {
		Data []struct {
			Permission string `json:"permission"`
			Status     string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("meta graph: resposta de permissoes invalida")
	}
	permissions := make([]OAuthPermission, 0, len(payload.Data))
	for _, item := range payload.Data {
		permissions = append(permissions, OAuthPermission{
			Name:   strings.TrimSpace(item.Permission),
			Status: strings.TrimSpace(item.Status),
		})
	}
	return permissions, nil
}

type oauthTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
}

func (c *MetaOAuthClient) exchange(ctx context.Context, form url.Values) (oauthTokenResponse, error) {
	endpoint := c.base + "/oauth/access_token"
	// G704: a base vem exclusivamente de configuracao do servidor, nunca do
	// callback ou do usuario. Segredos ficam no body e nao na URL.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode())) //nolint:gosec
	if err != nil {
		return oauthTokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req) //nolint:gosec // host de config confiavel, nao de input
	if err != nil {
		return oauthTokenResponse{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxOAuthResponseBytes))
	if err != nil {
		return oauthTokenResponse{}, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return oauthTokenResponse{}, graphError(resp.StatusCode, raw)
	}

	var token oauthTokenResponse
	if err := json.Unmarshal(raw, &token); err != nil {
		return oauthTokenResponse{}, fmt.Errorf("meta oauth: resposta invalida")
	}
	if strings.TrimSpace(token.AccessToken) == "" {
		return oauthTokenResponse{}, fmt.Errorf("meta oauth: token ausente")
	}
	return token, nil
}
