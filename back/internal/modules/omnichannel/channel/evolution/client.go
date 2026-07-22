// Package evolution e o 1o adapter REAL do canal (decisao D-A do canonico: Evolution e o
// primeiro provider de verdade; o mock roda sem numero). Implementa channel.Provider +
// channel.SessionManager falando com a Evolution API (integracao WHATSAPP-BAILEYS) por HTTP.
//
// O dominio e o front NUNCA veem este pacote: so o shape canonico do pacote channel. Trocar
// Evolution por WAHA/Meta = outro adapter que implementa a MESMA interface (registry.go).
//
// Independencia (canonico §4): este pacote so fala com a Evolution API configurada por
// instancia. Nao le/escreve schema de outro modulo, nao consulta waha/automation/sistema
// externo. baseURL/apiKey vem da instancia (Credentials); env e so fallback.
package evolution

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// bailenysIntegration e o valor fixo do campo `integration` no createInstance (WhatsApp via
// Baileys — o unico suportado por este adapter; Meta Cloud e outro provider/fase F11).
const baileysIntegration = "WHATSAPP-BAILEYS"

// maxResponseBytes limita o corpo lido das respostas de controle (create/connect/status/
// send). Respostas de controle da Evolution sao pequenas; o teto barra um provider hostil.
const maxResponseBytes = 4 << 20 // 4 MiB

// maxMediaResponseBytes limita o corpo do getBase64FromMediaMessage (a midia vem base64 no
// JSON — bem maior que uma resposta de controle). Teto generoso, mas finito.
const maxMediaResponseBytes = 100 << 20 // 100 MiB

// apiError marca uma resposta HTTP nao-2xx da Evolution. NUNCA carrega o corpo (pode conter
// eco de credencial/telefone — canonico §10): so o status, para o adapter classificar.
type apiError struct {
	StatusCode int
}

func (e *apiError) Error() string {
	return fmt.Sprintf("evolution: resposta http %d", e.StatusCode)
}

// HTTPStatusCode permite classificacao de retry sem expor o body da Evolution.
func (e *apiError) HTTPStatusCode() int { return e.StatusCode }

// client fala com UMA Evolution API (baseURL + apiKey resolvidos por instancia). Stateless:
// criado por chamada a partir das Credentials; compartilha o *http.Client (timeout 30s) do
// adapter singleton. A apiKey NUNCA vai a log nem volta ao cliente.
type client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// do executa uma chamada JSON: header `apikey` (padrao Evolution), content-type json,
// corpo limitado. Non-2xx => *apiError (sem corpo). out nil => resposta ignorada.
func (c *client) do(ctx context.Context, method, path string, body any, out any, limit int64) error {
	var reqBody io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("evolution: falha ao serializar corpo: %w", err)
		}
		reqBody = bytes.NewReader(raw)
	}

	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return fmt.Errorf("evolution: falha ao montar request: %w", err)
	}
	req.Header.Set("apikey", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		// Erro de transporte NAO ecoa a URL com credencial: mensagem generica.
		return fmt.Errorf("evolution: falha de transporte em %s", method)
	}
	defer func() { _ = resp.Body.Close() }()

	// Le sempre com teto, mesmo em erro, para poder drenar/descartar.
	data, readErr := io.ReadAll(io.LimitReader(resp.Body, limit))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &apiError{StatusCode: resp.StatusCode}
	}
	if readErr != nil {
		return fmt.Errorf("evolution: falha ao ler resposta")
	}
	if out == nil || len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		// Corpo fora do contrato: nao ecoar o corpo cru.
		return fmt.Errorf("evolution: resposta fora do contrato JSON")
	}
	return nil
}

// ============================================================================
// Sessao / instancia (client Evolution — SPECS_PORT F3 item 1)
// ============================================================================

// createInstance cria a instancia na Evolution (integracao Baileys, qrcode ligado). Se
// webhookURL != "", ja configura o webhook no mesmo create (auto-reparo — SPECS_PORT F3
// item 2). Instancia ja existente => a Evolution responde non-2xx; o chamador ignora
// (idempotente por nome).
func (c *client) createInstance(ctx context.Context, instanceName, webhookURL string) error {
	body := map[string]any{
		"instanceName": instanceName,
		"integration":  baileysIntegration,
		"qrcode":       true,
	}
	if webhookURL != "" {
		// O create quer o objeto de webhook DIRETO (webhook.url); embutir o envelope
		// {"webhook":...} aqui daria webhook.webhook.url -> 400 "Invalid url".
		body["webhook"] = webhookConfig(webhookURL, c.apiKey)
	}
	return c.do(ctx, http.MethodPost, "/instance/create", body, nil, maxResponseBytes)
}

// connect inicia/renova a sessao: GET /instance/connect/{i}. Devolve o payload cru de QR
// (base64/code), parseado defensivamente pelo adapter.
func (c *client) connect(ctx context.Context, instanceName string) (connectResponse, error) {
	var out connectResponse
	err := c.do(ctx, http.MethodGet, "/instance/connect/"+urlSegment(instanceName), nil, &out, maxResponseBytes)
	return out, err
}

// fetchInstance consulta o estado da instancia (state + numero pareado) via
// GET /instance/fetchInstances?instanceName={i}. A Evolution varia entre array e objeto
// unico e entre nomes de campo por versao: parse defensivo no adapter.
func (c *client) fetchInstance(ctx context.Context, instanceName string) (json.RawMessage, error) {
	var out json.RawMessage
	err := c.do(ctx, http.MethodGet,
		"/instance/fetchInstances?instanceName="+urlSegment(instanceName), nil, &out, maxResponseBytes)
	return out, err
}

// logout desconecta a instancia: DELETE /instance/logout/{i}.
func (c *client) logout(ctx context.Context, instanceName string) error {
	return c.do(ctx, http.MethodDelete, "/instance/logout/"+urlSegment(instanceName), nil, nil, maxResponseBytes)
}

// setWebhook (re)configura o webhook da instancia: POST /webhook/set/{i}. Usado para
// auto-reparo quando o webhookURL e conhecido. Embute o token da instancia (c.apiKey) nos
// headers para a Evolution devolve-lo em cada webhook (item 2 abaixo).
func (c *client) setWebhook(ctx context.Context, instanceName, webhookURL string) error {
	// O /webhook/set quer o ENVELOPE {"webhook": {...}} (ao contrario do create).
	body := map[string]any{"webhook": webhookConfig(webhookURL, c.apiKey)}
	return c.do(ctx, http.MethodPost, "/webhook/set/"+urlSegment(instanceName),
		body, nil, maxResponseBytes)
}

// ============================================================================
// Envio e midia (F6 consome; a interface fecha aqui)
// ============================================================================

// sendText envia texto: POST /message/sendText/{i}.
func (c *client) sendText(ctx context.Context, instanceName, number, message string, quoted *quotedSend) (sendResponse, error) {
	var out sendResponse
	body := map[string]any{"number": number, "text": message}
	applyQuote(body, quoted)
	err := c.do(ctx, http.MethodPost, "/message/sendText/"+urlSegment(instanceName), body, &out, maxResponseBytes)
	return out, err
}

// sendMedia envia midia (imagem/video/documento/audio): POST /message/sendMedia/{i}.
// mediatype deriva do MessageType canonico; media e a URL (a Evolution baixa) ou base64.
func (c *client) sendMedia(ctx context.Context, instanceName string, m mediaSend) (sendResponse, error) {
	var out sendResponse
	body := map[string]any{
		"number":    m.Number,
		"mediatype": m.MediaType,
		"media":     m.Media,
	}
	if m.MimeType != "" {
		body["mimetype"] = m.MimeType
	}
	if m.FileName != "" {
		body["fileName"] = m.FileName
	}
	if m.Caption != "" {
		body["caption"] = m.Caption
	}
	applyQuote(body, m.Quoted)
	err := c.do(ctx, http.MethodPost, "/message/sendMedia/"+urlSegment(instanceName), body, &out, maxResponseBytes)
	return out, err
}

// sendReaction reage a uma mensagem: POST /message/sendReaction/{i}. O corpo v2 e a KEY da
// mensagem alvo {remoteJid, fromMe, id} + `reaction` (emoji vazio remove a reacao). O id que a
// Evolution atribui a reacao e ignorado — o caller so precisa saber se deu 2xx.
func (c *client) sendReaction(ctx context.Context, instanceName string, r reactionSend) error {
	body := map[string]any{
		"key": map[string]any{
			"remoteJid": r.RemoteJid,
			"fromMe":    r.FromMe,
			"id":        r.MessageID,
		},
		"reaction": r.Reaction,
	}
	return c.do(ctx, http.MethodPost, "/message/sendReaction/"+urlSegment(instanceName), body, nil, maxResponseBytes)
}

// deleteForEveryone apaga uma mensagem para todos: DELETE /chat/deleteMessageForEveryone/{i}
// (rota v2 confirmada no legado: EVOLUTION_DELETE_FOR_ALL_PATH). O corpo e a KEY da mensagem
// {id, remoteJid, fromMe}; participant so vai em grupo (@g.us).
func (c *client) deleteForEveryone(ctx context.Context, instanceName string, k deleteKey) error {
	body := map[string]any{
		"id":        k.MessageID,
		"remoteJid": k.RemoteJid,
		"fromMe":    k.FromMe,
	}
	if strings.TrimSpace(k.Participant) != "" {
		body["participant"] = k.Participant
	}
	return c.do(ctx, http.MethodDelete, "/chat/deleteMessageForEveryone/"+urlSegment(instanceName), body, nil, maxResponseBytes)
}

// getBase64 baixa a midia de uma mensagem recebida:
// POST /chat/getBase64FromMediaMessage/{i}. Resposta traz base64 + mimetype + fileName.
func (c *client) getBase64(ctx context.Context, instanceName, messageID string) (mediaResponse, error) {
	var out mediaResponse
	body := map[string]any{
		"message":      map[string]any{"key": map[string]any{"id": messageID}},
		"convertToMp4": false,
	}
	err := c.do(ctx, http.MethodPost, "/chat/getBase64FromMediaMessage/"+urlSegment(instanceName),
		body, &out, maxMediaResponseBytes)
	return out, err
}

// ============================================================================
// Shapes de resposta (parse defensivo — a Evolution varia por versao)
// ============================================================================

// connectResponse cobre as variacoes conhecidas do GET /instance/connect: o QR pode vir em
// `base64`/`code` na raiz OU aninhado em `qrcode`.
type connectResponse struct {
	Base64      string        `json:"base64"`
	Code        string        `json:"code"`
	PairingCode string        `json:"pairingCode"`
	QRCode      *qrCodeBlock  `json:"qrcode"`
	Instance    *instanceInfo `json:"instance"`
}

type qrCodeBlock struct {
	Base64      string `json:"base64"`
	Code        string `json:"code"`
	PairingCode string `json:"pairingCode"`
}

type instanceInfo struct {
	InstanceName string `json:"instanceName"`
	State        string `json:"state"`
	Status       string `json:"status"`
	Owner        string `json:"owner"`
	OwnerJid     string `json:"ownerJid"`
	ProfileName  string `json:"profileName"`
	Number       string `json:"number"`
}

// sendResponse cobre onde a Evolution devolve o id da mensagem enviada.
type sendResponse struct {
	Key       *messageKey `json:"key"`
	MessageID string      `json:"messageId"`
	ID        string      `json:"id"`
	Status    string      `json:"status"`
}

type messageKey struct {
	ID string `json:"id"`
}

// externalID resolve o id da mensagem enviada entre as variacoes.
func (r sendResponse) externalID() string {
	if r.Key != nil && strings.TrimSpace(r.Key.ID) != "" {
		return strings.TrimSpace(r.Key.ID)
	}
	if strings.TrimSpace(r.MessageID) != "" {
		return strings.TrimSpace(r.MessageID)
	}
	return strings.TrimSpace(r.ID)
}

// mediaResponse e a resposta do getBase64FromMediaMessage.
type mediaResponse struct {
	Base64   string `json:"base64"`
	MimeType string `json:"mimetype"`
	FileName string `json:"fileName"`
}

// mediaSend e o input tipado do sendMedia.
type mediaSend struct {
	Number    string
	MediaType string // image | video | document | audio
	Media     string // URL ou base64
	MimeType  string
	FileName  string
	Caption   string
	Quoted    *quotedSend
}

type quotedSend struct {
	ExternalMessageID string
	Content           string
}

func applyQuote(body map[string]any, quoted *quotedSend) {
	if quoted == nil || strings.TrimSpace(quoted.ExternalMessageID) == "" {
		return
	}
	body["quoted"] = map[string]any{
		"key":     map[string]any{"id": strings.TrimSpace(quoted.ExternalMessageID)},
		"message": map[string]any{"conversation": strings.TrimSpace(quoted.Content)},
	}
}

// reactionSend e o input tipado do sendReaction (KEY da mensagem alvo + emoji).
type reactionSend struct {
	RemoteJid string
	FromMe    bool
	MessageID string
	Reaction  string // vazio = remover a reacao
}

// deleteKey e a KEY da mensagem alvo do deleteForEveryone. Participant so vai em grupo.
type deleteKey struct {
	MessageID   string
	RemoteJid   string
	FromMe      bool
	Participant string
}

// webhookConfig monta o OBJETO de config do webhook v2 (enabled/url/headers/eventos), SEM o
// envelope `{"webhook":...}`. Os dois consumidores diferem no envelope, e por isso ele nao
// mora aqui:
//   - createInstance embute este objeto direto em body["webhook"] (o create quer
//     webhook.url — nao webhook.webhook.url; o envelope duplo devolve 400 "Invalid url").
//   - setWebhook o envolve em {"webhook": webhookConfig(...)} (o /webhook/set quer o envelope).
//
// webhookBase64 liga o envio da midia em base64 no evento (o adapter baixa via getBase64
// quando precisar; aqui e best-effort de config).
//
// headers: a Evolution v2 devolve estes headers em CADA POST de webhook. Embutimos o token
// (mesmo valor que o VerifyWebhook espera — c.apiKey == expectedToken(cred)) no header
// `apikey` para o inbound autenticar (parse.go aceita `X-Webhook-Token` OU `Apikey`). SEM
// isto a Evolution nao devolve token e todo webhook inbound volta 401. Token vazio => nao
// adianta setar header (o VerifyWebhook e fail-closed); omitimos.
func webhookConfig(url, token string) map[string]any {
	cfg := map[string]any{
		"enabled":         true,
		"url":             url,
		"webhookByEvents": false,
		"webhookBase64":   true,
		"events": []string{
			"MESSAGES_UPSERT",
			"MESSAGES_UPDATE",
			"QRCODE_UPDATED",
			"CONNECTION_UPDATE",
		},
	}
	if strings.TrimSpace(token) != "" {
		cfg["headers"] = map[string]any{"apikey": token}
	}
	return cfg
}
