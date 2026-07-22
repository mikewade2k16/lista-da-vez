package evolution

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel/channel"
)

// transportFunc intercepta a request ANTES da rede (sem discar): exercita o caminho real do
// client sem Evolution no ar (padrao de platform/llm/client_test.go).
type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// providerWith monta um Provider (env baseURL/apiKey) cujo transporte e o handler dado. O
// ultimo request interceptado fica em *captured para assercao (white-box: mesmo pacote).
func providerWith(captured **http.Request, fn func(*http.Request) (*http.Response, error)) *Provider {
	p := New("http://evo.local", "env-key")
	p.http = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		if captured != nil {
			*captured = r
		}
		return fn(r)
	})}
	return p
}

func jsonResp(status int, body string) (*http.Response, error) {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}, nil
}

// ============================================================================
// VerifyWebhook — constant-time, fail-closed
// ============================================================================

func TestVerifyWebhook(t *testing.T) {
	p := New("", "")
	cred := channel.Credentials{Token: "s3cr3t"}

	t.Run("token correto passa", func(t *testing.T) {
		h := http.Header{}
		h.Set("X-Webhook-Token", "s3cr3t")
		if err := p.VerifyWebhook(h, nil, cred); err != nil {
			t.Fatalf("esperava sucesso, veio %v", err)
		}
	})

	t.Run("apikey header como fallback", func(t *testing.T) {
		h := http.Header{}
		h.Set("apikey", "s3cr3t")
		if err := p.VerifyWebhook(h, nil, cred); err != nil {
			t.Fatalf("esperava sucesso via apikey, veio %v", err)
		}
	})

	t.Run("token errado falha", func(t *testing.T) {
		h := http.Header{}
		h.Set("X-Webhook-Token", "errado")
		if err := p.VerifyWebhook(h, nil, cred); err == nil {
			t.Fatal("esperava erro para token invalido")
		}
	})

	t.Run("sem token no header falha", func(t *testing.T) {
		if err := p.VerifyWebhook(http.Header{}, nil, cred); err == nil {
			t.Fatal("esperava erro para header ausente")
		}
	})

	t.Run("fail-closed sem segredo configurado", func(t *testing.T) {
		h := http.Header{}
		h.Set("X-Webhook-Token", "qualquer")
		if err := p.VerifyWebhook(h, nil, channel.Credentials{}); err == nil {
			t.Fatal("esperava erro quando nao ha segredo esperado (instancia/env)")
		}
	})

	t.Run("fallback no env apiKey", func(t *testing.T) {
		pe := New("", "env-secret")
		h := http.Header{}
		h.Set("X-Webhook-Token", "env-secret")
		if err := pe.VerifyWebhook(h, nil, channel.Credentials{}); err != nil {
			t.Fatalf("esperava sucesso via env, veio %v", err)
		}
	})
}

// ============================================================================
// ParseWebhook — payload dinamico, defensivo
// ============================================================================

func TestParseWebhook_MessagesUpsert(t *testing.T) {
	p := New("", "")
	body := `{
		"event": "messages.upsert",
		"instance": "omni-main",
		"data": {
			"key": {"remoteJid": "5511988887777@s.whatsapp.net", "fromMe": false, "id": "3EB0ABC"},
			"pushName": "Maria",
			"message": {"conversation": "ola mundo"},
			"messageTimestamp": 1700000000
		}
	}`
	events, err := p.ParseWebhook(context.Background(), nil, []byte(body))
	if err != nil {
		t.Fatalf("ParseWebhook: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("esperava 1 evento, veio %d", len(events))
	}
	ev := events[0]
	if ev.Kind != channel.EventMessageReceived {
		t.Errorf("Kind = %q", ev.Kind)
	}
	if ev.ExternalEventID != "omni-main:msg:3EB0ABC" {
		t.Errorf("ExternalEventID = %q (deve compor instancia+id — armadilha 2)", ev.ExternalEventID)
	}
	if ev.InstanceName != "omni-main" {
		t.Errorf("InstanceName = %q", ev.InstanceName)
	}
	if ev.OccurredAt.Unix() != 1700000000 {
		t.Errorf("OccurredAt = %v (deve ser o timestamp do provider, nao now)", ev.OccurredAt)
	}
	if ev.Message == nil {
		t.Fatal("Message nil")
	}
	if ev.Message.Content != "ola mundo" || ev.Message.MessageType != "TEXT" {
		t.Errorf("conteudo = %q tipo = %q", ev.Message.Content, ev.Message.MessageType)
	}
	if ev.Message.ContactPhone != "5511988887777" {
		t.Errorf("ContactPhone = %q (deve extrair do JID)", ev.Message.ContactPhone)
	}
	if ev.Message.ContactName != "Maria" {
		t.Errorf("ContactName = %q", ev.Message.ContactName)
	}
}

func TestParseWebhook_FromMeAsOutbound(t *testing.T) {
	p := New("", "")
	body := `{"event":"messages.upsert","instance":"i","data":{"key":{"fromMe":true,"id":"X","remoteJid":"5511988887777@s.whatsapp.net"},"message":{"conversation":"eco"}}}`
	events, err := p.ParseWebhook(context.Background(), nil, []byte(body))
	if err != nil {
		t.Fatalf("ParseWebhook: %v", err)
	}
	// fromMe NAO e mais ignorado: vira message_received com FromMe=true. O ingest grava OUTBOUND
	// (mensagem enviada pelo aparelho pareado) e dedupa pelo external id o eco do proprio envio.
	if len(events) != 1 || events[0].Kind != channel.EventMessageReceived {
		t.Fatalf("mensagem fromMe deveria virar message_received, veio %+v", events)
	}
	if events[0].Message == nil || !events[0].Message.FromMe {
		t.Fatalf("Message.FromMe deveria ser true, veio %+v", events[0].Message)
	}
	if events[0].Message.Content != "eco" {
		t.Errorf("Content = %q (deve decodificar a mensagem)", events[0].Message.Content)
	}
	// O pushName do fromMe e o NOSSO nome — nao vira nome do contato.
	if events[0].Message.ContactName != "" {
		t.Errorf("ContactName = %q (fromMe nao deve setar nome do contato)", events[0].Message.ContactName)
	}
}

func TestParseWebhook_MediaTypes(t *testing.T) {
	p := New("", "")
	cases := []struct {
		name     string
		message  string
		wantType string
		wantCap  string
	}{
		{"imagem", `{"imageMessage":{"caption":"foto","mimetype":"image/jpeg"}}`, "IMAGE", "foto"},
		{"audio", `{"audioMessage":{"mimetype":"audio/ogg"}}`, "AUDIO", ""},
		{"documento", `{"documentMessage":{"caption":"pdf","fileName":"a.pdf"}}`, "DOCUMENT", "pdf"},
		{"texto estendido", `{"extendedTextMessage":{"text":"link"}}`, "TEXT", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"event":"MESSAGES_UPSERT","instance":"i","data":{"key":{"id":"M1"},"message":` + tc.message + `}}`
			events, err := p.ParseWebhook(context.Background(), nil, []byte(body))
			if err != nil {
				t.Fatalf("ParseWebhook: %v", err)
			}
			if len(events) != 1 || events[0].Message == nil {
				t.Fatalf("esperava 1 evento com mensagem, veio %+v", events)
			}
			if events[0].Message.MessageType != tc.wantType {
				t.Errorf("MessageType = %q, queria %q", events[0].Message.MessageType, tc.wantType)
			}
			if events[0].Message.MediaCaption != tc.wantCap {
				t.Errorf("MediaCaption = %q, queria %q", events[0].Message.MediaCaption, tc.wantCap)
			}
		})
	}
}

func TestParseWebhook_EventMapping(t *testing.T) {
	p := New("", "")
	cases := []struct {
		name string
		body string
		want channel.EventKind
	}{
		{"qrcode", `{"event":"qrcode.updated","instance":"i","data":{"base64":"AAAA"}}`, channel.EventQRUpdated},
		{"connection", `{"event":"connection.update","instance":"i","data":{"state":"open","ownerJid":"5511@s.whatsapp.net"}}`, channel.EventSessionStatus},
		{"desconhecido", `{"event":"presence.update","instance":"i","data":{}}`, channel.EventIgnored},
		{"ack update", `{"event":"messages.update","instance":"i","data":{"key":{"id":"M1"},"status":"DELIVERY_ACK"}}`, channel.EventMessageStatus},
		{"update sem ack", `{"event":"messages.update","instance":"i","data":{"key":{"id":"M1"},"message":{"protocolMessage":{"type":"REVOKE"}}}}`, channel.EventIgnored},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			events, err := p.ParseWebhook(context.Background(), nil, []byte(tc.body))
			if err != nil {
				t.Fatalf("ParseWebhook: %v", err)
			}
			if len(events) != 1 || events[0].Kind != tc.want {
				t.Fatalf("Kind = %+v, queria %q", events, tc.want)
			}
		})
	}
}

func TestParseWebhook_QRNormalizedToDataURL(t *testing.T) {
	p := New("", "")
	body := `{"event":"qrcode.updated","instance":"i","data":{"base64":"AAAABBBB"}}`
	events, _ := p.ParseWebhook(context.Background(), nil, []byte(body))
	if len(events) != 1 || events[0].Session == nil {
		t.Fatalf("esperava evento de QR com Session, veio %+v", events)
	}
	if events[0].Session.QRCode != "data:image/png;base64,AAAABBBB" {
		t.Errorf("QRCode = %q (deve virar data URL)", events[0].Session.QRCode)
	}
}

func TestParseWebhook_InvalidJSONNoBodyLeak(t *testing.T) {
	p := New("", "")
	_, err := p.ParseWebhook(context.Background(), nil, []byte(`{nao-json`))
	if err == nil {
		t.Fatal("esperava erro para JSON invalido")
	}
	if strings.Contains(err.Error(), "nao-json") {
		t.Fatalf("o body vazou no erro: %v", err)
	}
}

// ============================================================================
// SendMessage / DownloadMedia / Connect via transporte falso
// ============================================================================

func TestSendMessage_Text(t *testing.T) {
	var got *http.Request
	p := providerWith(&got, func(_ *http.Request) (*http.Response, error) {
		return jsonResp(http.StatusCreated, `{"key":{"id":"WAMID-123"}}`)
	})
	cred := channel.Credentials{Token: "inst-key"}
	res, err := p.SendMessage(context.Background(), cred, channel.OutboundMessage{
		InstanceName: "omni-main", ToPhone: "5511999998888", MessageType: "TEXT", Content: "oi",
	})
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if res.ExternalMessageID != "WAMID-123" || res.Status != "SENT" {
		t.Errorf("resultado = %+v", res)
	}
	if !strings.HasSuffix(got.URL.Path, "/message/sendText/omni-main") {
		t.Errorf("path = %q", got.URL.Path)
	}
	if got.Header.Get("apikey") != "inst-key" {
		t.Errorf("apikey = %q (deve ser a credencial da instancia, nao o env)", got.Header.Get("apikey"))
	}
}

func TestSendMessage_QuotedContractForTextAndMedia(t *testing.T) {
	tests := []struct {
		name        string
		messageType string
		mediaURL    string
		wantPath    string
	}{
		{name: "text", messageType: "TEXT", wantPath: "/message/sendText/omni-main"},
		{name: "image", messageType: "IMAGE", mediaURL: "https://cdn.example.test/image.jpg", wantPath: "/message/sendMedia/omni-main"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var body []byte
			p := providerWith(nil, func(r *http.Request) (*http.Response, error) {
				body, _ = io.ReadAll(r.Body)
				if r.URL.Path != tc.wantPath {
					t.Errorf("path = %q, want %q", r.URL.Path, tc.wantPath)
				}
				return jsonResp(http.StatusCreated, `{"key":{"id":"WAMID-QUOTED"}}`)
			})
			_, err := p.SendMessage(context.Background(), channel.Credentials{Token: "k"}, channel.OutboundMessage{
				InstanceName: "omni-main",
				ToPhone:      "5511999998888",
				MessageType:  tc.messageType,
				Content:      "resposta nova",
				MediaURL:     tc.mediaURL,
				MediaMimeType: func() string {
					if tc.messageType == "IMAGE" {
						return "image/jpeg"
					}
					return ""
				}(),
				Reply: &channel.ReplyReference{
					ExternalMessageID: "WAMID-ORIGINAL",
					Content:           "mensagem original",
					MessageType:       "TEXT",
				},
			})
			if err != nil {
				t.Fatalf("SendMessage: %v", err)
			}
			var payload struct {
				Quoted struct {
					Key struct {
						ID string `json:"id"`
					} `json:"key"`
					Message struct {
						Conversation string `json:"conversation"`
					} `json:"message"`
				} `json:"quoted"`
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("body fora do contrato: %v", err)
			}
			if payload.Quoted.Key.ID != "WAMID-ORIGINAL" || payload.Quoted.Message.Conversation != "mensagem original" {
				t.Fatalf("quoted = %+v", payload.Quoted)
			}
		})
	}
}

func TestSendMessage_ConfigBaseURLOverridesEnv(t *testing.T) {
	var got *http.Request
	p := providerWith(&got, func(_ *http.Request) (*http.Response, error) {
		return jsonResp(http.StatusOK, `{"key":{"id":"X"}}`)
	})
	cred := channel.Credentials{Token: "k", Config: map[string]string{"baseURL": "http://custom-host:9000"}}
	_, err := p.SendMessage(context.Background(), cred, channel.OutboundMessage{
		InstanceName: "i", ToPhone: "551199", MessageType: "TEXT", Content: "x",
	})
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if got.URL.Host != "custom-host:9000" {
		t.Errorf("host = %q (provider_config.baseURL deve vencer o env)", got.URL.Host)
	}
}

func TestSendMessage_NoBaseURLIsActionableError(t *testing.T) {
	p := New("", "") // sem env, sem config
	_, err := p.SendMessage(context.Background(), channel.Credentials{}, channel.OutboundMessage{
		InstanceName: "i", ToPhone: "551199", MessageType: "TEXT", Content: "x",
	})
	if err == nil || !strings.Contains(err.Error(), "baseURL") {
		t.Fatalf("esperava erro acionavel citando baseURL, veio %v", err)
	}
}

func TestSendMessage_HTTPErrorNeverLeaksKey(t *testing.T) {
	p := providerWith(nil, func(_ *http.Request) (*http.Response, error) {
		return jsonResp(http.StatusInternalServerError, `erro com apikey super-secreta-xyz`)
	})
	cred := channel.Credentials{Token: "super-secreta-xyz"}
	_, err := p.SendMessage(context.Background(), cred, channel.OutboundMessage{
		InstanceName: "i", ToPhone: "551199", MessageType: "TEXT", Content: "x",
	})
	if err == nil {
		t.Fatal("esperava erro")
	}
	if strings.Contains(err.Error(), "super-secreta-xyz") {
		t.Fatalf("a chave/body vazou no erro: %v", err)
	}
}

func TestConnect_ReturnsQRDataURL(t *testing.T) {
	p := providerWith(nil, func(r *http.Request) (*http.Response, error) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/instance/create"):
			// Instancia ja existe: non-2xx e ignorado (idempotente por nome).
			return jsonResp(http.StatusForbidden, `{"message":"already in use"}`)
		case strings.Contains(r.URL.Path, "/instance/connect/"):
			return jsonResp(http.StatusOK, `{"base64":"QRDATA"}`)
		default:
			return jsonResp(http.StatusNotFound, `{}`)
		}
	})
	state, err := p.Connect(context.Background(), channel.Credentials{Token: "k"}, "omni-main")
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if state.Connected {
		t.Error("nao deveria estar conectado (so QR)")
	}
	if state.QRCode != "data:image/png;base64,QRDATA" {
		t.Errorf("QRCode = %q", state.QRCode)
	}
}

// TestConnect_SetsWebhookWithTokenHeader prova os fixes do BUG 1 + BUG 2:
//   - BUG 1: o webhook (no create E no set) embute `headers.apikey` = token da instancia — o
//     MESMO valor que o VerifyWebhook compara. Sem esse header a Evolution nao devolve token
//     e o inbound volta 401.
//   - BUG 2: o corpo do create traz `webhook.url` DIRETO (nao `webhook.webhook.url`). O
//     envelope duplo devolvia 400 "Invalid url" -> instancia nunca criada -> setWebhook 500.
func TestConnect_SetsWebhookWithTokenHeader(t *testing.T) {
	var createBody, setBody []byte
	p := providerWith(nil, func(r *http.Request) (*http.Response, error) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/instance/create"):
			createBody, _ = io.ReadAll(r.Body)
			return jsonResp(http.StatusForbidden, `{}`) // ja existe: idempotente
		case strings.Contains(r.URL.Path, "/webhook/set/"):
			setBody, _ = io.ReadAll(r.Body)
			return jsonResp(http.StatusOK, `{}`)
		case strings.Contains(r.URL.Path, "/instance/connect/"):
			return jsonResp(http.StatusOK, `{"base64":"QRDATA"}`)
		default:
			return jsonResp(http.StatusNotFound, `{}`)
		}
	})
	cred := channel.Credentials{Token: "inst-key", Config: map[string]string{
		"webhookUrl": "http://api:8080/v1/webhooks/omnichannel/evolution/crow",
	}}
	if _, err := p.Connect(context.Background(), cred, "omni-main"); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	// create: o webhook e um objeto FLAT (sem envelope duplo). `webhook.url` deve existir.
	var create struct {
		Webhook struct {
			URL     string            `json:"url"`
			Headers map[string]string `json:"headers"`
			Webhook json.RawMessage   `json:"webhook"` // NAO deve existir (double-wrap)
		} `json:"webhook"`
	}
	if err := json.Unmarshal(createBody, &create); err != nil {
		t.Fatalf("create body fora do contrato: %v", err)
	}
	if create.Webhook.URL != "http://api:8080/v1/webhooks/omnichannel/evolution/crow" {
		t.Errorf("create webhook.url = %q (double-wrap daria vazio + 400 Invalid url)", create.Webhook.URL)
	}
	if len(create.Webhook.Webhook) != 0 {
		t.Errorf("create webhook NAO deve ter envelope aninhado, veio %s", create.Webhook.Webhook)
	}
	if create.Webhook.Headers["apikey"] != "inst-key" {
		t.Errorf("create webhook.headers.apikey = %q", create.Webhook.Headers["apikey"])
	}

	// set: o /webhook/set quer o ENVELOPE {"webhook": {...}}.
	if len(setBody) == 0 {
		t.Fatal("setWebhook nao foi chamado")
	}
	var set struct {
		Webhook struct {
			URL     string            `json:"url"`
			Headers map[string]string `json:"headers"`
			Enabled bool              `json:"enabled"`
		} `json:"webhook"`
	}
	if err := json.Unmarshal(setBody, &set); err != nil {
		t.Fatalf("set body fora do contrato: %v", err)
	}
	if set.Webhook.Headers["apikey"] != "inst-key" {
		t.Errorf("set webhook.headers.apikey = %q (deve casar com o VerifyWebhook)", set.Webhook.Headers["apikey"])
	}
	if set.Webhook.URL != "http://api:8080/v1/webhooks/omnichannel/evolution/crow" || !set.Webhook.Enabled {
		t.Errorf("set webhook = %+v", set.Webhook)
	}
}

// TestWebhookConfig_HeadersToggle: com token embute headers.apikey; sem token OMITE headers
// (o VerifyWebhook e fail-closed). E NUNCA envolve em {"webhook":...} (isso e responsabilidade
// do setWebhook; no create o envelope daria 400).
func TestConnect_SetWebhookFailureDoesNotLogURLOrToken(t *testing.T) {
	var logs bytes.Buffer
	p := providerWith(nil, func(r *http.Request) (*http.Response, error) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/instance/create"):
			return jsonResp(http.StatusForbidden, `{}`)
		case strings.Contains(r.URL.Path, "/webhook/set/"):
			return jsonResp(http.StatusInternalServerError, `{}`)
		case strings.Contains(r.URL.Path, "/instance/connect/"):
			return jsonResp(http.StatusOK, `{"base64":"QRDATA"}`)
		default:
			return jsonResp(http.StatusNotFound, `{}`)
		}
	})
	p.logger = slog.New(slog.NewTextHandler(&logs, nil))

	const (
		providerToken = "inst-key-that-must-not-leak"
		webhookURL    = "https://api.example.test/webhook?token=signed-secret"
	)
	cred := channel.Credentials{Token: providerToken, Config: map[string]string{
		configKeyWebhookURL: webhookURL,
	}}
	if _, err := p.Connect(context.Background(), cred, "omni-main"); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	got := logs.String()
	if !strings.Contains(got, "evolution_set_webhook_failed") {
		t.Fatalf("log esperado ausente: %q", got)
	}
	for _, secret := range []string{webhookURL, providerToken, "signed-secret"} {
		if strings.Contains(got, secret) {
			t.Fatalf("log vazou dado sensivel %q: %q", secret, got)
		}
	}
}

func TestWebhookConfig_HeadersToggle(t *testing.T) {
	with := webhookConfig("http://x/y", "tok")
	if with["url"] != "http://x/y" {
		t.Errorf("url = %v", with["url"])
	}
	if _, nested := with["webhook"]; nested {
		t.Error("webhookConfig nao deve ter o envelope 'webhook' (double-wrap = 400 no create)")
	}
	hdr, ok := with["headers"].(map[string]any)
	if !ok || hdr["apikey"] != "tok" {
		t.Errorf("com token: headers.apikey deveria ser tok, veio %+v", with["headers"])
	}
	without := webhookConfig("http://x/y", "")
	if _, ok := without["headers"]; ok {
		t.Error("sem token: nao deveria setar headers")
	}
}

func TestConnect_AlreadyPaired(t *testing.T) {
	p := providerWith(nil, func(r *http.Request) (*http.Response, error) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/instance/create"):
			return jsonResp(http.StatusForbidden, `{}`)
		case strings.Contains(r.URL.Path, "/instance/connect/"):
			return jsonResp(http.StatusOK, `{"instance":{"state":"open","ownerJid":"5511977776666@s.whatsapp.net"}}`)
		default:
			return jsonResp(http.StatusNotFound, `{}`)
		}
	})
	state, err := p.Connect(context.Background(), channel.Credentials{Token: "k"}, "omni-main")
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if !state.Connected || state.PhoneNumber != "5511977776666" {
		t.Errorf("state = %+v (deveria estar conectado com o numero pareado)", state)
	}
}

func TestStatus_ParsesFetchInstances(t *testing.T) {
	p := providerWith(nil, func(_ *http.Request) (*http.Response, error) {
		return jsonResp(http.StatusOK, `[{"name":"omni-main","connectionStatus":"open","ownerJid":"5511900001111@s.whatsapp.net"}]`)
	})
	state, err := p.Status(context.Background(), channel.Credentials{Token: "k"}, "omni-main")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !state.Connected || state.PhoneNumber != "5511900001111" {
		t.Errorf("state = %+v", state)
	}
}

func TestDownloadMedia_DecodesBase64(t *testing.T) {
	raw := []byte("conteudo binario da midia")
	encoded := base64.StdEncoding.EncodeToString(raw)
	var got *http.Request
	p := providerWith(&got, func(_ *http.Request) (*http.Response, error) {
		return jsonResp(http.StatusOK, `{"base64":"`+encoded+`","mimetype":"image/png","fileName":"foto.png"}`)
	})
	rc, meta, err := p.DownloadMedia(context.Background(), channel.Credentials{Token: "k"},
		channel.MediaRef{InstanceName: "omni-main", ExternalMessageID: "M1"})
	if err != nil {
		t.Fatalf("DownloadMedia: %v", err)
	}
	defer func() { _ = rc.Close() }()
	out, _ := io.ReadAll(rc)
	if string(out) != string(raw) {
		t.Errorf("bytes = %q, queria %q", out, raw)
	}
	if meta.MimeType != "image/png" || meta.FileName != "foto.png" || meta.SizeBytes != int64(len(raw)) {
		t.Errorf("meta = %+v", meta)
	}
	if !strings.Contains(got.URL.Path, "/chat/getBase64FromMediaMessage/omni-main") {
		t.Errorf("path = %q", got.URL.Path)
	}
}

// ============================================================================
// SendReaction / DeleteForAll (F7 — acoes sincronas) via transporte falso
// ============================================================================

func TestSendReaction_PayloadAndHeader(t *testing.T) {
	var got *http.Request
	var body []byte
	p := providerWith(&got, func(r *http.Request) (*http.Response, error) {
		body, _ = io.ReadAll(r.Body)
		return jsonResp(http.StatusOK, `{}`)
	})
	cred := channel.Credentials{Token: "inst-key"}
	err := p.SendReaction(context.Background(), cred, channel.ReactionInput{
		InstanceName:      "omni-main",
		RemoteJID:         "5511999998888@s.whatsapp.net",
		ExternalMessageID: "WAMID-9",
		FromMe:            true,
		Emoji:             "🔥",
	})
	if err != nil {
		t.Fatalf("SendReaction: %v", err)
	}
	if got.Method != http.MethodPost || !strings.HasSuffix(got.URL.Path, "/message/sendReaction/omni-main") {
		t.Errorf("metodo/path = %s %q", got.Method, got.URL.Path)
	}
	if got.Header.Get("apikey") != "inst-key" {
		t.Errorf("apikey = %q (deve ser a credencial da instancia)", got.Header.Get("apikey"))
	}
	var payload struct {
		Key struct {
			RemoteJid string `json:"remoteJid"`
			FromMe    bool   `json:"fromMe"`
			ID        string `json:"id"`
		} `json:"key"`
		Reaction string `json:"reaction"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("payload fora do contrato: %v", err)
	}
	if payload.Key.RemoteJid != "5511999998888@s.whatsapp.net" || !payload.Key.FromMe || payload.Key.ID != "WAMID-9" {
		t.Errorf("key = %+v (deve ser {remoteJid,fromMe,id} da mensagem alvo)", payload.Key)
	}
	if payload.Reaction != "🔥" {
		t.Errorf("reaction = %q", payload.Reaction)
	}
}

func TestSendReaction_EmptyEmojiRemoves(t *testing.T) {
	var body []byte
	p := providerWith(nil, func(r *http.Request) (*http.Response, error) {
		body, _ = io.ReadAll(r.Body)
		return jsonResp(http.StatusOK, `{}`)
	})
	err := p.SendReaction(context.Background(), channel.Credentials{Token: "k"}, channel.ReactionInput{
		InstanceName: "i", RemoteJID: "551199@s.whatsapp.net", ExternalMessageID: "M1", Emoji: "",
	})
	if err != nil {
		t.Fatalf("SendReaction: %v", err)
	}
	var payload struct {
		Reaction *string `json:"reaction"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("payload: %v", err)
	}
	if payload.Reaction == nil || *payload.Reaction != "" {
		t.Errorf("emoji vazio deve mandar reaction=\"\" (remover), veio %v", payload.Reaction)
	}
}

func TestSendReaction_MissingFieldsAreActionable(t *testing.T) {
	p := providerWith(nil, func(_ *http.Request) (*http.Response, error) {
		return jsonResp(http.StatusOK, `{}`)
	})
	if err := p.SendReaction(context.Background(), channel.Credentials{Token: "k"}, channel.ReactionInput{
		InstanceName: "i", RemoteJID: "551199@s.whatsapp.net", ExternalMessageID: "",
	}); err == nil {
		t.Error("esperava erro para id da mensagem vazio")
	}
	if err := p.SendReaction(context.Background(), channel.Credentials{Token: "k"}, channel.ReactionInput{
		InstanceName: "i", RemoteJID: "", ExternalMessageID: "M1",
	}); err == nil {
		t.Error("esperava erro para remoteJid vazio")
	}
}

func TestSendReaction_HTTPErrorNeverLeaksKey(t *testing.T) {
	p := providerWith(nil, func(_ *http.Request) (*http.Response, error) {
		return jsonResp(http.StatusBadGateway, `erro com apikey super-secreta-xyz e 5511@telefone`)
	})
	cred := channel.Credentials{Token: "super-secreta-xyz"}
	err := p.SendReaction(context.Background(), cred, channel.ReactionInput{
		InstanceName: "i", RemoteJID: "551199@s.whatsapp.net", ExternalMessageID: "M1", Emoji: "👍",
	})
	if err == nil {
		t.Fatal("esperava erro")
	}
	if strings.Contains(err.Error(), "super-secreta-xyz") || strings.Contains(err.Error(), "telefone") {
		t.Fatalf("a chave/corpo vazou no erro: %v", err)
	}
}

func TestDeleteForAll_PayloadAndMethod(t *testing.T) {
	var got *http.Request
	var body []byte
	p := providerWith(&got, func(r *http.Request) (*http.Response, error) {
		body, _ = io.ReadAll(r.Body)
		return jsonResp(http.StatusOK, `{}`)
	})
	err := p.DeleteForAll(context.Background(), channel.Credentials{Token: "inst-key"}, channel.DeleteInput{
		InstanceName:      "omni-main",
		RemoteJID:         "5511999998888@s.whatsapp.net",
		ExternalMessageID: "WAMID-DEL",
		FromMe:            true,
	})
	if err != nil {
		t.Fatalf("DeleteForAll: %v", err)
	}
	if got.Method != http.MethodDelete || !strings.HasSuffix(got.URL.Path, "/chat/deleteMessageForEveryone/omni-main") {
		t.Errorf("metodo/path = %s %q (v2: DELETE /chat/deleteMessageForEveryone)", got.Method, got.URL.Path)
	}
	if got.Header.Get("apikey") != "inst-key" {
		t.Errorf("apikey = %q", got.Header.Get("apikey"))
	}
	var payload struct {
		ID          string `json:"id"`
		RemoteJid   string `json:"remoteJid"`
		FromMe      bool   `json:"fromMe"`
		Participant string `json:"participant"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("payload fora do contrato: %v", err)
	}
	if payload.ID != "WAMID-DEL" || payload.RemoteJid != "5511999998888@s.whatsapp.net" || !payload.FromMe {
		t.Errorf("payload = %+v (deve ser a KEY {id,remoteJid,fromMe})", payload)
	}
	if payload.Participant != "" {
		t.Errorf("chat 1:1 nao deve levar participant, veio %q", payload.Participant)
	}
}

func TestDeleteForAll_GroupCarriesParticipant(t *testing.T) {
	var body []byte
	p := providerWith(nil, func(r *http.Request) (*http.Response, error) {
		body, _ = io.ReadAll(r.Body)
		return jsonResp(http.StatusOK, `{}`)
	})
	err := p.DeleteForAll(context.Background(), channel.Credentials{Token: "k"}, channel.DeleteInput{
		InstanceName:      "i",
		RemoteJID:         "120363000000000000@g.us",
		ExternalMessageID: "M1",
		FromMe:            true,
		ParticipantJID:    "5511999998888@s.whatsapp.net",
	})
	if err != nil {
		t.Fatalf("DeleteForAll: %v", err)
	}
	var payload struct {
		Participant string `json:"participant"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("payload: %v", err)
	}
	if payload.Participant != "5511999998888@s.whatsapp.net" {
		t.Errorf("grupo (@g.us) deve levar participant, veio %q", payload.Participant)
	}
}

func TestDeleteForAll_HTTPErrorNeverLeaksKey(t *testing.T) {
	p := providerWith(nil, func(_ *http.Request) (*http.Response, error) {
		return jsonResp(http.StatusInternalServerError, `falhou com apikey chave-secreta-123`)
	})
	err := p.DeleteForAll(context.Background(), channel.Credentials{Token: "chave-secreta-123"}, channel.DeleteInput{
		InstanceName: "i", RemoteJID: "551199@s.whatsapp.net", ExternalMessageID: "M1", FromMe: true,
	})
	if err == nil {
		t.Fatal("esperava erro")
	}
	if strings.Contains(err.Error(), "chave-secreta-123") {
		t.Fatalf("a chave/corpo vazou no erro: %v", err)
	}
}

func TestCapabilities_EvolutionHasNoTemplates(t *testing.T) {
	c := New("", "").Capabilities()
	if c.SupportsTemplates || c.Requires24hWindow {
		t.Errorf("Evolution nao tem template nem janela 24h (isso e Meta Cloud): %+v", c)
	}
	if !c.SupportsGroups || c.MaxMediaBytes <= 0 {
		t.Errorf("capabilities inesperadas: %+v", c)
	}
}

func TestID(t *testing.T) {
	if New("", "").ID() != "evolution" {
		t.Fatalf("ID = %q", New("", "").ID())
	}
}
