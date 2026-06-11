package automation

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// WAHAClient fala com a API interna da WAHA (rede do compose, http://waha:3000).
// Nunca exposto ao cliente: o painel passa pela API Go, que faz o proxy escopado.
type WAHAClient struct {
	baseURL string
	http    *http.Client
}

// NewWAHAClient cria o cliente. baseURL ex.: "http://waha:3000".
func NewWAHAClient(baseURL string) *WAHAClient {
	return &WAHAClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

type wahaSession struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Me     *struct {
		ID       string `json:"id"`
		PushName string `json:"pushName"`
	} `json:"me"`
}

// Status retorna o estado da sessao e o numero conectado. Sessao inexistente
// (404) vira "STOPPED" sem erro.
func (c *WAHAClient) Status(ctx context.Context, session string) (status string, phone string, err error) {
	resp, err := c.do(ctx, http.MethodGet, "/api/sessions/"+session, nil)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return "STOPPED", "", nil
	case resp.StatusCode != http.StatusOK:
		return "", "", fmt.Errorf("waha status: http %d", resp.StatusCode)
	}

	var s wahaSession
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&s); err != nil {
		return "", "", err
	}
	if s.Me != nil {
		phone = phoneFromWAHAID(s.Me.ID)
	}
	return s.Status, phone, nil
}

// Start garante a sessao criada e iniciada. Idempotente e tolerante: se a sessao
// ja existe, tenta iniciar o recurso existente.
func (c *WAHAClient) Start(ctx context.Context, session string) error {
	body := strings.NewReader(fmt.Sprintf(`{"name":%q,"start":true}`, session))
	resp, err := c.do(ctx, http.MethodPost, "/api/sessions", body)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusUnprocessableEntity, resp.StatusCode == http.StatusConflict:
		return c.post(ctx, "/api/sessions/"+session+"/start")
	case resp.StatusCode >= 300:
		return fmt.Errorf("waha start: http %d", resp.StatusCode)
	default:
		return nil
	}
}

// Stop encerra a sessao.
func (c *WAHAClient) Stop(ctx context.Context, session string) error {
	return c.post(ctx, "/api/sessions/"+session+"/stop")
}

// QR busca o QR code (PNG) e devolve um data URL base64 pronto para <img src>.
func (c *WAHAClient) QR(ctx context.Context, session string) (string, error) {
	resp, err := c.do(ctx, http.MethodGet, "/api/"+session+"/auth/qr?format=image", nil)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("waha qr: http %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	if err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(raw), nil
}

func (c *WAHAClient) post(ctx context.Context, path string) error {
	resp, err := c.do(ctx, http.MethodPost, path, nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("waha %s: http %d", path, resp.StatusCode)
	}
	return nil
}

func (c *WAHAClient) do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.http.Do(req)
}

// phoneFromWAHAID extrai o numero de um id tipo "554284138129@c.us".
func phoneFromWAHAID(id string) string {
	if i := strings.IndexByte(id, '@'); i > 0 {
		return id[:i]
	}
	return id
}
