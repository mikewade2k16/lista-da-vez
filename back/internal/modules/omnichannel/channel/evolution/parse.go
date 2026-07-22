package evolution

import (
	"context"
	"crypto/hmac"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel/channel"
)

// webhookTokenHeaders lista, em ordem, os headers onde a Evolution pode mandar o segredo do
// webhook. `x-webhook-token` e o padrao do legado (SPECS_PORT F3 item 3); `apikey` cobre
// setups que reaproveitam a chave mestra no header do webhook.
var webhookTokenHeaders = []string{"X-Webhook-Token", "Apikey"}

// VerifyWebhook autentica a requisicao ANTES do parse: compara, constant-time (hmac.Equal),
// o token do header contra o segredo esperado (credencial da instancia, com fallback no env).
// FAIL-CLOSED: sem segredo esperado OU sem token no header => erro (401 no handler). O erro
// NUNCA carrega o header nem o body (canonico §10).
func (p *Provider) VerifyWebhook(hdr http.Header, _ []byte, cred channel.Credentials) error {
	expected := strings.TrimSpace(p.expectedToken(cred))
	if expected == "" {
		return errors.New("evolution: webhook sem segredo configurado (instancia/env)")
	}
	var provided string
	for _, h := range webhookTokenHeaders {
		if v := strings.TrimSpace(hdr.Get(h)); v != "" {
			provided = v
			break
		}
	}
	if provided == "" {
		return errors.New("evolution: webhook sem token de autenticidade")
	}
	if !hmac.Equal([]byte(provided), []byte(expected)) {
		return errors.New("evolution: token de webhook invalido")
	}
	return nil
}

// ParseWebhook traduz o payload DINAMICO da Evolution em eventos canonicos. Regra de ouro:
// nunca presumir que um campo existe; JSON invalido/ inesperado vira lista vazia ou evento
// ignorado, NUNCA erro com o body (canonico §10). O `event` resolve de
// payload.event ?? payload.type ?? data.event, normalizado ([^a-zA-Z0-9]+ -> _, uppercase).
func (p *Provider) ParseWebhook(_ context.Context, _ http.Header, body []byte) ([]channel.Event, error) {
	var env webhookEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		// Erro SEM body: nunca ecoar o payload cru.
		return nil, errors.New("evolution: payload de webhook nao e JSON valido")
	}

	instance := firstNonEmpty(env.Instance, env.InstanceName)
	eventName := normalizeEvent(firstNonEmpty(env.Event, env.Type, dataEventName(env.Data)))

	switch eventName {
	case "MESSAGES_UPSERT":
		return p.parseUpsert(instance, env.Data), nil
	case "MESSAGES_UPDATE":
		return p.parseUpdate(instance, env.Data), nil
	case "QRCODE_UPDATED":
		return []channel.Event{p.parseQR(instance, env.Data)}, nil
	case "CONNECTION_UPDATE":
		return []channel.Event{p.parseConnection(instance, env.Data)}, nil
	default:
		// Presenca, typing, contatos, etc.: 202 ignored no handler.
		return []channel.Event{{Kind: channel.EventIgnored, InstanceName: instance}}, nil
	}
}

// parseUpsert traduz MESSAGES_UPSERT em message_received. `data` pode ser um objeto unico ou
// um array (batch): trata os dois. Mensagens `fromMe` (nosso proprio envio ecoado) sao
// ignoradas — nao re-ingerir o outbound como inbound.
func (p *Provider) parseUpsert(instance string, data json.RawMessage) []channel.Event {
	msgs := decodeMessages(data)
	out := make([]channel.Event, 0, len(msgs))
	for i := range msgs {
		m := &msgs[i]
		id := strings.TrimSpace(m.Key.ID)
		if id == "" {
			continue
		}
		body := decodeMessageBody(m.Message)
		msgType, content, media := interpretBody(body)
		reply := replyReferenceFromBody(body)
		// fromMe = enviada PELO celular pareado (ou eco do proprio envio da plataforma).
		// NAO descartamos mais: o ingest grava como OUTBOUND e dedupa pelo external_message_id
		// (o eco de um envio da plataforma cai no dedup). O pushName do fromMe e o NOSSO nome —
		// nao usar como nome do contato (o contato e o RemoteJid, que nao muda).
		contactName := strings.TrimSpace(m.PushName)
		if m.Key.FromMe {
			contactName = ""
		}
		out = append(out, channel.Event{
			Kind: channel.EventMessageReceived,
			// Compoe com o escopo da instancia para nao colidir entre contas (armadilha 2).
			ExternalEventID: instance + ":msg:" + id,
			InstanceName:    instance,
			OccurredAt:      occurredAt(m.MessageTimestamp),
			Message: &channel.InboundMessage{
				ExternalMessageID: id,
				Channel:           "WHATSAPP",
				ContactExternalID: strings.TrimSpace(m.Key.RemoteJid),
				ContactPhone:      numberFromJid(m.Key.RemoteJid),
				ContactName:       contactName,
				FromMe:            m.Key.FromMe,
				MessageType:       msgType,
				Content:           content,
				MediaMimeType:     media.MimeType,
				MediaFileName:     media.FileName,
				MediaCaption:      media.Caption,
				Reply:             reply,
			},
		})
	}
	if len(out) == 0 {
		return []channel.Event{{Kind: channel.EventIgnored, InstanceName: instance}}
	}
	return out
}

// parseUpdate traduz MESSAGES_UPDATE. Um update de ACK vira message_status
// (SENT/FAILED). Uma delecao (protocolMessage REVOKE) NAO tem tipo canonico no shape da
// interface — vira ignored (documentado: a delecao inbound entra numa fase que a modele).
func (p *Provider) parseUpdate(instance string, data json.RawMessage) []channel.Event {
	msgs := decodeMessages(data)
	out := make([]channel.Event, 0, len(msgs))
	for i := range msgs {
		m := &msgs[i]
		id := strings.TrimSpace(m.Key.ID)
		if id == "" {
			continue
		}
		status := mapAckStatus(m.Status)
		if status == "" {
			// Sem status de ACK reconhecido (ex.: delecao/edicao): ignora nesta fase.
			out = append(out, channel.Event{Kind: channel.EventIgnored, InstanceName: instance})
			continue
		}
		out = append(out, channel.Event{
			Kind:            channel.EventMessageStatus,
			ExternalEventID: instance + ":upd:" + id + ":" + status,
			InstanceName:    instance,
			OccurredAt:      occurredAt(m.MessageTimestamp),
			Status: &channel.StatusUpdate{
				ExternalMessageID: id,
				Status:            status,
				ErrorCode:         safeProviderErrorCode(m.ErrorCode, m.StatusReason),
			},
		})
	}
	if len(out) == 0 {
		return []channel.Event{{Kind: channel.EventIgnored, InstanceName: instance}}
	}
	return out
}

// parseQR traduz QRCODE_UPDATED: o QR normalizado (data URL) para o cache de sessao. F4 nao
// persiste esse evento (o domInio so processa message_received) — e o seam para a F5.
func (p *Provider) parseQR(instance string, data json.RawMessage) channel.Event {
	var d struct {
		QRCode  *qrCodeBlock `json:"qrcode"`
		Base64  string       `json:"base64"`
		Code    string       `json:"code"`
		Message string       `json:"message"`
	}
	_ = json.Unmarshal(data, &d)
	qr := normalizeQR(qrFrom(connectResponse{Base64: d.Base64, Code: d.Code, QRCode: d.QRCode}))
	return channel.Event{
		Kind:            channel.EventQRUpdated,
		ExternalEventID: instance + ":qr:" + strconv.FormatInt(time.Now().UnixNano(), 10),
		InstanceName:    instance,
		OccurredAt:      time.Now().UTC(),
		Session:         &channel.SessionState{Connected: false, QRCode: qr},
	}
}

// parseConnection traduz CONNECTION_UPDATE em session_status (connected + numero pareado).
func (p *Provider) parseConnection(instance string, data json.RawMessage) channel.Event {
	var d struct {
		State    string `json:"state"`
		Status   string `json:"status"`
		OwnerJid string `json:"ownerJid"`
		Owner    string `json:"owner"`
		Number   string `json:"number"`
	}
	_ = json.Unmarshal(data, &d)
	connected := isConnectedState(d.State, d.Status)
	return channel.Event{
		Kind:            channel.EventSessionStatus,
		ExternalEventID: instance + ":conn:" + strconv.FormatInt(time.Now().UnixNano(), 10),
		InstanceName:    instance,
		OccurredAt:      time.Now().UTC(),
		Session: &channel.SessionState{
			Connected:   connected,
			PhoneNumber: numberFromJid(firstNonEmpty(d.OwnerJid, d.Owner, d.Number)),
		},
	}
}

// ============================================================================
// Shapes do webhook (parse defensivo)
// ============================================================================

type webhookEnvelope struct {
	Event        string          `json:"event"`
	Type         string          `json:"type"`
	Instance     string          `json:"instance"`
	InstanceName string          `json:"instanceName"`
	Data         json.RawMessage `json:"data"`
}

// dataEventName recupera o event aninhado em data.event (algumas versoes so o poem la).
func dataEventName(data json.RawMessage) string {
	if len(data) == 0 {
		return ""
	}
	var d struct {
		Event string `json:"event"`
	}
	_ = json.Unmarshal(data, &d)
	return d.Event
}

// messageData e o shape de uma mensagem no MESSAGES_UPSERT/UPDATE.
type messageData struct {
	Key struct {
		RemoteJid string `json:"remoteJid"`
		FromMe    bool   `json:"fromMe"`
		ID        string `json:"id"`
	} `json:"key"`
	PushName         string          `json:"pushName"`
	Message          json.RawMessage `json:"message"`
	MessageType      string          `json:"messageType"`
	MessageTimestamp flexInt         `json:"messageTimestamp"`
	Status           string          `json:"status"`
	ErrorCode        json.RawMessage `json:"errorCode"`
	StatusReason     json.RawMessage `json:"statusReason"`
}

// decodeMessages aceita `data` como objeto unico OU array (batch) OU {messages:[...]}.
func decodeMessages(data json.RawMessage) []messageData {
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil
	}
	// Array direto.
	var arr []messageData
	if err := json.Unmarshal(data, &arr); err == nil && len(arr) > 0 {
		return arr
	}
	// Envelope {messages:[...]}.
	var wrap struct {
		Messages []messageData `json:"messages"`
	}
	if err := json.Unmarshal(data, &wrap); err == nil && len(wrap.Messages) > 0 {
		return wrap.Messages
	}
	// Objeto unico.
	var one messageData
	if err := json.Unmarshal(data, &one); err == nil {
		return []messageData{one}
	}
	return nil
}

// messageBody cobre os tipos de conteudo do WhatsApp que traduzimos.
type messageBody struct {
	Conversation string `json:"conversation"`
	ExtendedText *struct {
		Text        string       `json:"text"`
		ContextInfo *contextInfo `json:"contextInfo"`
	} `json:"extendedTextMessage"`
	ImageMessage    *mediaContent `json:"imageMessage"`
	VideoMessage    *mediaContent `json:"videoMessage"`
	AudioMessage    *mediaContent `json:"audioMessage"`
	DocumentMessage *mediaContent `json:"documentMessage"`
	StickerMessage  *mediaContent `json:"stickerMessage"`
	ProtocolMessage *struct {
		Type string `json:"type"`
	} `json:"protocolMessage"`
}

type mediaContent struct {
	Caption     string       `json:"caption"`
	MimeType    string       `json:"mimetype"`
	FileName    string       `json:"fileName"`
	URL         string       `json:"url"`
	ContextInfo *contextInfo `json:"contextInfo"`
}

// contextInfo e o shape de quote do Baileys/Evolution. quotedMessage usa o mesmo
// envelope de conteudo de uma mensagem comum; manter RawMessage evita acoplamento a
// campos do provider que nao pertencem ao dominio.
type contextInfo struct {
	StanzaID      string          `json:"stanzaId"`
	Participant   string          `json:"participant"`
	QuotedMessage json.RawMessage `json:"quotedMessage"`
}

func decodeMessageBody(raw json.RawMessage) messageBody {
	var b messageBody
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &b)
	}
	return b
}

// interpretBody deriva (tipo canonico, conteudo textual, midia) do corpo do WhatsApp. A URL
// crua da midia NAO e usada aqui (vem cifrada; a rehidratacao e via DownloadMedia na F6): so
// mimetype/fileName/caption entram no dominio.
func interpretBody(b messageBody) (msgType, content string, media mediaContent) {
	switch {
	case b.Conversation != "":
		return "TEXT", b.Conversation, mediaContent{}
	case b.ExtendedText != nil && b.ExtendedText.Text != "":
		return "TEXT", b.ExtendedText.Text, mediaContent{}
	case b.ImageMessage != nil:
		return "IMAGE", b.ImageMessage.Caption, *b.ImageMessage
	case b.StickerMessage != nil:
		return "IMAGE", "", *b.StickerMessage
	case b.VideoMessage != nil:
		return "VIDEO", b.VideoMessage.Caption, *b.VideoMessage
	case b.AudioMessage != nil:
		return "AUDIO", "", *b.AudioMessage
	case b.DocumentMessage != nil:
		return "DOCUMENT", b.DocumentMessage.Caption, *b.DocumentMessage
	default:
		return "TEXT", "", mediaContent{}
	}
}

// replyReferenceFromBody extrai a referencia de quote sem aplicar regra de tenant ou
// persistencia. Um contextInfo sem stanzaId nao e navegavel e por isso nao vira reply.
func replyReferenceFromBody(b messageBody) *channel.ReplyReference {
	ctx := contextInfoFromBody(b)
	if ctx == nil || strings.TrimSpace(ctx.StanzaID) == "" {
		return nil
	}
	quoted := decodeMessageBody(ctx.QuotedMessage)
	messageType, content, media := interpretBody(quoted)
	if strings.TrimSpace(content) == "" {
		content = strings.TrimSpace(media.Caption)
	}
	return &channel.ReplyReference{
		ExternalMessageID: strings.TrimSpace(ctx.StanzaID),
		ParticipantID:     strings.TrimSpace(ctx.Participant),
		Content:           strings.TrimSpace(content),
		MessageType:       messageType,
	}
}

func contextInfoFromBody(b messageBody) *contextInfo {
	switch {
	case b.ExtendedText != nil && b.ExtendedText.ContextInfo != nil:
		return b.ExtendedText.ContextInfo
	case b.ImageMessage != nil && b.ImageMessage.ContextInfo != nil:
		return b.ImageMessage.ContextInfo
	case b.VideoMessage != nil && b.VideoMessage.ContextInfo != nil:
		return b.VideoMessage.ContextInfo
	case b.AudioMessage != nil && b.AudioMessage.ContextInfo != nil:
		return b.AudioMessage.ContextInfo
	case b.DocumentMessage != nil && b.DocumentMessage.ContextInfo != nil:
		return b.DocumentMessage.ContextInfo
	case b.StickerMessage != nil && b.StickerMessage.ContextInfo != nil:
		return b.StickerMessage.ContextInfo
	default:
		return nil
	}
}

// ============================================================================
// Helpers de parse
// ============================================================================

// flexInt decodifica messageTimestamp seja numero ou string, sem nunca falhar (defensivo).
type flexInt int64

func (f *flexInt) UnmarshalJSON(b []byte) error {
	s := strings.Trim(strings.TrimSpace(string(b)), `"`)
	if s == "" || s == "null" {
		return nil
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		*f = flexInt(n)
	}
	return nil
}

// occurredAt converte o messageTimestamp (unix segundos) do provider; 0 => now (canonico:
// nunca now() quando ha timestamp do provider — armadilha 5).
func occurredAt(ts flexInt) time.Time {
	if ts > 0 {
		return time.Unix(int64(ts), 0).UTC()
	}
	return time.Now().UTC()
}

// mapAckStatus traduz o status de ACK da Evolution para o vocabulario canonico E1.
// Vazio => nao reconhecido (o chamador ignora).
func mapAckStatus(raw string) string {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "SERVER_ACK", "SENT":
		return "SENT"
	case "DELIVERY_ACK", "DELIVERED":
		return "DELIVERED"
	case "READ", "PLAYED", "READ_ACK", "PLAYED_ACK":
		return "READ"
	case "ERROR", "FAILED":
		return "FAILED"
	case "DELETED", "REVOKED":
		return "DELETED"
	default:
		return ""
	}
}

// safeProviderErrorCode aceita apenas um token curto do provider. Objetos, mensagens e
// qualquer valor potencialmente contendo PII sao descartados.
func safeProviderErrorCode(values ...json.RawMessage) string {
	for _, raw := range values {
		trimmed := strings.TrimSpace(string(raw))
		if trimmed == "" || trimmed == "null" {
			continue
		}
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			// Numeros simples sao codigos aceitaveis; objetos/arrays nao sao.
			if _, err := strconv.ParseInt(strings.Trim(trimmed, `"`), 10, 64); err != nil {
				continue
			}
			text = strings.Trim(trimmed, `"`)
		}
		text = strings.ToUpper(strings.TrimSpace(text))
		if text == "" || len(text) > 64 {
			continue
		}
		valid := true
		for _, r := range text {
			if (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' && r != '-' {
				valid = false
				break
			}
		}
		if valid {
			return text
		}
	}
	return ""
}

// normalizeEvent aplica [^a-zA-Z0-9]+ -> _ e uppercase (ex.: "messages.upsert" ->
// "MESSAGES_UPSERT"), com trim das bordas.
func normalizeEvent(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var b strings.Builder
	prevUnderscore := false
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevUnderscore = false
			continue
		}
		if !prevUnderscore {
			b.WriteByte('_')
			prevUnderscore = true
		}
	}
	return strings.ToUpper(strings.Trim(b.String(), "_"))
}

// qrFrom extrai o QR da resposta de connect/qrcode, preferindo a imagem base64.
func qrFrom(r connectResponse) string {
	if r.Base64 != "" {
		return r.Base64
	}
	if r.QRCode != nil && r.QRCode.Base64 != "" {
		return r.QRCode.Base64
	}
	if r.Code != "" {
		return r.Code
	}
	if r.QRCode != nil && r.QRCode.Code != "" {
		return r.QRCode.Code
	}
	return ""
}

// normalizeQR normaliza o QR para data URL (o painel renderiza uma <img>). Ja em data URL =>
// intacto; base64 cru => prefixa como PNG.
func normalizeQR(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if strings.HasPrefix(v, "data:") {
		return v
	}
	return "data:image/png;base64," + v
}

// parseInstanceInfo decodifica o retorno do fetchInstances, que varia entre array e objeto,
// com/sem o wrapper `instance`.
func parseInstanceInfo(raw json.RawMessage) instanceInfo {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return instanceInfo{}
	}
	var arr []fetchInstanceItem
	if err := json.Unmarshal(raw, &arr); err == nil && len(arr) > 0 {
		return arr[0].normalized()
	}
	var one fetchInstanceItem
	if err := json.Unmarshal(raw, &one); err == nil {
		return one.normalized()
	}
	return instanceInfo{}
}

// fetchInstanceItem cobre as variacoes de campo do fetchInstances entre versoes.
type fetchInstanceItem struct {
	InstanceName     string             `json:"instanceName"`
	Name             string             `json:"name"`
	State            string             `json:"state"`
	Status           string             `json:"status"`
	ConnectionStatus string             `json:"connectionStatus"`
	Owner            string             `json:"owner"`
	OwnerJid         string             `json:"ownerJid"`
	Number           string             `json:"number"`
	Instance         *fetchInstanceItem `json:"instance"`
}

func (it fetchInstanceItem) normalized() instanceInfo {
	if it.Instance != nil {
		it = *it.Instance
	}
	return instanceInfo{
		InstanceName: firstNonEmpty(it.InstanceName, it.Name),
		State:        firstNonEmpty(it.State, it.ConnectionStatus),
		Status:       it.Status,
		Owner:        it.Owner,
		OwnerJid:     it.OwnerJid,
		Number:       it.Number,
	}
}

// urlSegment escapa o nome da instancia para uso seguro no path.
func urlSegment(s string) string {
	return url.PathEscape(strings.TrimSpace(s))
}
