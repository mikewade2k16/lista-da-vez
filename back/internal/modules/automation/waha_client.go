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

// Start garante a sessao iniciada (idempotente). Tenta iniciar a sessao existente
// (POST /api/sessions/{name}/start — caminho que a engine gows aceita); se a sessao
// ainda nao existe (404), cria e inicia. O atalho "criar+iniciar num passo so"
// (POST /api/sessions {start:true}) e' recusado por algumas versoes da WAHA quando a
// sessao ja existe e dava 502 no painel ao clicar Conectar.
func (c *WAHAClient) Start(ctx context.Context, session string) error {
	code, err := c.startSession(ctx, session)
	if err != nil {
		return err
	}
	if code == http.StatusNotFound {
		if err := c.createSession(ctx, session); err != nil {
			return err
		}
		if code, err = c.startSession(ctx, session); err != nil {
			return err
		}
	}
	// 2xx = iniciada; 4xx = provavelmente ja rodando (toleramos); 5xx = erro real.
	if code >= 500 {
		return fmt.Errorf("waha start: http %d", code)
	}
	return nil
}

// startSession dispara POST /api/sessions/{session}/start e devolve o status code.
func (c *WAHAClient) startSession(ctx context.Context, session string) (int, error) {
	resp, err := c.do(ctx, http.MethodPost, "/api/sessions/"+session+"/start", nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode, nil
}

// createSession cria a sessao (POST /api/sessions {name}). 4xx ("ja existe") e' tolerado.
func (c *WAHAClient) createSession(ctx context.Context, session string) error {
	body := strings.NewReader(fmt.Sprintf(`{"name":%q}`, session))
	resp, err := c.do(ctx, http.MethodPost, "/api/sessions", body)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("waha create session: http %d", resp.StatusCode)
	}
	return nil
}

// Logout desconecta a sessao do WhatsApp (POST /api/sessions/{name}/logout),
// liberando o numero pareado para que um novo QR possa ser escaneado. E' o que
// permite "desconectar e conectar outro numero" — um `stop` apenas pausaria mantendo
// o mesmo numero pareado, entao reconectar voltaria o mesmo numero. Tolerante: 4xx
// (sessao ja deslogada/inexistente) nao e' erro; so 5xx falha.
func (c *WAHAClient) Logout(ctx context.Context, session string) error {
	resp, err := c.do(ctx, http.MethodPost, "/api/sessions/"+session+"/logout", nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("waha logout: http %d", resp.StatusCode)
	}
	return nil
}

// Restart reinicia a sessao (POST /api/sessions/{name}/restart), re-armando o engine.
// E' a forma de recuperar uma sessao em estado FAILED: uma sessao FAILED nao gera QR
// (o GET de QR fica em long-poll ate estourar o timeout), e o restart a leva de volta
// a STARTING -> SCAN_QR_CODE. Tolerante: 4xx nao e' erro, so 5xx falha.
func (c *WAHAClient) Restart(ctx context.Context, session string) error {
	resp, err := c.do(ctx, http.MethodPost, "/api/sessions/"+session+"/restart", nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("waha restart: http %d", resp.StatusCode)
	}
	return nil
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

func (c *WAHAClient) do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	// G704 (gosec): falso-positivo. baseURL vem de config confiavel
	// (AUTOMATION_WAHA_INTERNAL_URL, default http://waha:3000) e o session no path e'
	// a sessao fisica ("default" de config ou UUID interno do canal), nunca input do usuario.
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body) //nolint:gosec // host de config confiavel, nao de input
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.http.Do(req) //nolint:gosec // host de config confiavel, nao de input
}

// phoneFromWAHAID extrai o numero de um id tipo "554284138129@c.us".
func phoneFromWAHAID(id string) string {
	if i := strings.IndexByte(id, '@'); i > 0 {
		return id[:i]
	}
	return id
}
