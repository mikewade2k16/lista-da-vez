// Package meta_whatsapp is the isolated WhatsApp Cloud API adapter. It translates
// Meta webhooks and calls into the canonical omnichannel channel contract; it never
// writes PostgreSQL or sends directly from n8n.
package meta_whatsapp

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel/channel"
)

const providerID = "meta_whatsapp_cloud"

var graphVersionPattern = regexp.MustCompile(`^v[0-9]{2,3}\.0$`)

var (
	errMissingCredential = errors.New("meta whatsapp: credential not configured")
	errInvalidConfig     = errors.New("meta whatsapp: provider config invalid")
	errUnsupportedAction = errors.New("meta whatsapp: action unsupported")
)

// Provider is stateless; account/instance credentials are supplied per call.
type Provider struct {
	envBaseURL string
	http       *http.Client
}

func New(envBaseURL string) *Provider {
	return &Provider{
		envBaseURL: strings.TrimRight(strings.TrimSpace(envBaseURL), "/"),
		http:       &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *Provider) ID() string { return providerID }

func (p *Provider) Capabilities() channel.Capabilities {
	return channel.Capabilities{
		SupportsTemplates: true,
		Requires24hWindow: true,
		SupportsReaction:  false,
		SupportsSticker:   false,
		SupportsGroups:    false,
		MaxMediaBytes:     64 << 20,
	}
}

var _ channel.Provider = (*Provider)(nil)
var _ channel.WebhookChallengeVerifier = (*Provider)(nil)
var _ channel.WebhookInstanceResolver = (*Provider)(nil)
var _ channel.TemplateProvider = (*Provider)(nil)

type credentialEnvelope struct {
	AccessToken string `json:"accessToken"`
	AppSecret   string `json:"appSecret"`
	VerifyToken string `json:"verifyToken"`
}

func credentialsOf(cred channel.Credentials) (credentialEnvelope, error) {
	raw := strings.TrimSpace(cred.Token)
	if raw == "" {
		return credentialEnvelope{}, errMissingCredential
	}
	var out credentialEnvelope
	if json.Unmarshal([]byte(raw), &out) != nil {
		// A plain token is accepted only as an access token. Webhook HMAC and
		// challenge verification still fail closed without their dedicated secrets.
		out.AccessToken = raw
	}
	out.AccessToken = strings.TrimSpace(out.AccessToken)
	out.AppSecret = strings.TrimSpace(out.AppSecret)
	out.VerifyToken = strings.TrimSpace(out.VerifyToken)
	if out.AccessToken == "" {
		return credentialEnvelope{}, errMissingCredential
	}
	return out, nil
}

func graphVersion(config map[string]string) (string, error) {
	v := strings.TrimSpace(config["graphVersion"])
	if !graphVersionPattern.MatchString(v) {
		return "", errInvalidConfig
	}
	return v, nil
}

// ValidateGraphVersion is shared by the configuration service and adapter so a
// tenant cannot persist a version that the client would reject at send time.
func ValidateGraphVersion(value string) bool {
	return graphVersionPattern.MatchString(strings.TrimSpace(value))
}

func phoneNumberID(config map[string]string, fallback string) string {
	if id := strings.TrimSpace(config["phoneNumberId"]); id != "" {
		return id
	}
	return strings.TrimSpace(fallback)
}

func (p *Provider) baseURL(config map[string]string) string {
	if v := strings.TrimRight(strings.TrimSpace(config["baseURL"]), "/"); v != "" {
		return v
	}
	if p != nil && p.envBaseURL != "" {
		return p.envBaseURL
	}
	return "https://graph.facebook.com"
}

// WebhookInstanceKey extracts the public phone_number_id before signature
// verification. It is used only to select the account's matching encrypted
// credential; the HMAC is still checked before parsing/persisting the event.
func (p *Provider) WebhookInstanceKey(body []byte) string {
	var envelope webhookEnvelope
	if json.Unmarshal(body, &envelope) != nil {
		return ""
	}
	for _, entry := range envelope.Entry {
		for _, change := range entry.Changes {
			if key := strings.TrimSpace(change.Value.Metadata.PhoneNumberID); key != "" {
				return key
			}
		}
	}
	return ""
}

// VerifyWebhook authenticates X-Hub-Signature-256 before ParseWebhook runs.
func (p *Provider) VerifyWebhook(hdr http.Header, body []byte, cred channel.Credentials) error {
	secrets, err := credentialsOf(cred)
	if err != nil || secrets.AppSecret == "" {
		return errMissingCredential
	}
	value := strings.TrimSpace(hdr.Get("X-Hub-Signature-256"))
	if !strings.HasPrefix(strings.ToLower(value), "sha256=") {
		return errors.New("meta whatsapp: signature missing")
	}
	got, err := hex.DecodeString(strings.TrimSpace(value[len("sha256="):]))
	if err != nil || len(got) != sha256.Size {
		return errors.New("meta whatsapp: signature invalid")
	}
	h := hmac.New(sha256.New, []byte(secrets.AppSecret))
	_, _ = h.Write(body)
	if !hmac.Equal(got, h.Sum(nil)) {
		return errors.New("meta whatsapp: signature invalid")
	}
	return nil
}

// VerifyWebhookChallenge handles Meta's GET subscription handshake.
func (p *Provider) VerifyWebhookChallenge(query map[string]string, cred channel.Credentials) (string, error) {
	secrets, err := credentialsOf(cred)
	if err != nil || secrets.VerifyToken == "" {
		return "", errMissingCredential
	}
	if strings.TrimSpace(query["hub.mode"]) != "subscribe" ||
		!hmac.Equal([]byte(strings.TrimSpace(query["hub.verify_token"])), []byte(secrets.VerifyToken)) {
		return "", errors.New("meta whatsapp: challenge token invalid")
	}
	challenge := strings.TrimSpace(query["hub.challenge"])
	if challenge == "" || len(challenge) > 512 {
		return "", errors.New("meta whatsapp: challenge invalid")
	}
	return challenge, nil
}

func (p *Provider) SendMessage(ctx context.Context, cred channel.Credentials, out channel.OutboundMessage) (channel.SendResult, error) {
	secrets, err := credentialsOf(cred)
	if err != nil {
		return channel.SendResult{Status: "FAILED"}, err
	}
	version, err := graphVersion(cred.Config)
	if err != nil {
		return channel.SendResult{Status: "FAILED"}, err
	}
	phoneID := phoneNumberID(cred.Config, out.InstanceName)
	if phoneID == "" || strings.TrimSpace(out.ToPhone) == "" {
		return channel.SendResult{Status: "FAILED"}, errInvalidConfig
	}
	payload, err := messagePayload(out)
	if err != nil {
		return channel.SendResult{Status: "FAILED"}, err
	}
	var response struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
	}
	if err := p.doJSON(ctx, cred.Config, secrets.AccessToken, http.MethodPost,
		"/"+version+"/"+phoneID+"/messages", payload, &response); err != nil {
		return channel.SendResult{Status: "FAILED"}, err
	}
	if len(response.Messages) == 0 || strings.TrimSpace(response.Messages[0].ID) == "" {
		return channel.SendResult{Status: "FAILED"}, errors.New("meta whatsapp: provider response missing message id")
	}
	return channel.SendResult{ExternalMessageID: response.Messages[0].ID, Status: "SENT"}, nil
}

func (p *Provider) DownloadMedia(ctx context.Context, cred channel.Credentials, ref channel.MediaRef) (io.ReadCloser, channel.MediaMeta, error) {
	secrets, err := credentialsOf(cred)
	if err != nil {
		return nil, channel.MediaMeta{}, err
	}
	version, err := graphVersion(cred.Config)
	if err != nil {
		return nil, channel.MediaMeta{}, err
	}
	mediaID := strings.TrimSpace(ref.MediaURL)
	if mediaID == "" {
		return nil, channel.MediaMeta{}, errors.New("meta whatsapp: media id missing")
	}
	var info struct {
		URL      string `json:"url"`
		MimeType string `json:"mime_type"`
		FileSize int64  `json:"file_size"`
		FileName string `json:"file_name"`
	}
	if err := p.doJSON(ctx, cred.Config, secrets.AccessToken, http.MethodGet,
		"/"+version+"/"+mediaID, nil, &info); err != nil {
		return nil, channel.MediaMeta{}, err
	}
	if strings.TrimSpace(info.URL) == "" {
		return nil, channel.MediaMeta{}, errors.New("meta whatsapp: media URL missing")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, info.URL, nil)
	if err != nil {
		return nil, channel.MediaMeta{}, err
	}
	request.Header.Set("Authorization", "Bearer "+secrets.AccessToken)
	client := p.http
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, channel.MediaMeta{}, errors.New("meta whatsapp: media download failed")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_ = response.Body.Close()
		return nil, channel.MediaMeta{}, &httpError{status: response.StatusCode}
	}
	if info.FileSize > 64<<20 {
		_ = response.Body.Close()
		return nil, channel.MediaMeta{}, errors.New("meta whatsapp: media too large")
	}
	return &limitedReadCloser{Reader: io.LimitReader(response.Body, 64<<20+1), closer: response.Body}, channel.MediaMeta{
		MimeType: firstNonEmpty(info.MimeType, response.Header.Get("Content-Type")), FileName: info.FileName, SizeBytes: info.FileSize,
	}, nil
}

func (p *Provider) SendReaction(context.Context, channel.Credentials, channel.ReactionInput) error {
	return errUnsupportedAction
}

func (p *Provider) ListTemplates(ctx context.Context, cred channel.Credentials) ([]channel.Template, error) {
	secrets, err := credentialsOf(cred)
	if err != nil {
		return nil, err
	}
	version, err := graphVersion(cred.Config)
	if err != nil {
		return nil, err
	}
	wabaID := strings.TrimSpace(cred.Config["wabaId"])
	if wabaID == "" {
		return nil, errInvalidConfig
	}
	var response struct {
		Data []struct {
			ID         string          `json:"id"`
			Name       string          `json:"name"`
			Language   string          `json:"language"`
			Category   string          `json:"category"`
			Status     string          `json:"status"`
			Quality    string          `json:"quality_score"`
			Components json.RawMessage `json:"components"`
		} `json:"data"`
	}
	if err := p.doJSON(ctx, cred.Config, secrets.AccessToken, http.MethodGet,
		"/"+version+"/"+wabaID+"/message_templates?limit=100", nil, &response); err != nil {
		return nil, err
	}
	out := make([]channel.Template, 0, len(response.Data))
	for _, item := range response.Data {
		components := item.Components
		if len(components) == 0 {
			components = json.RawMessage(`[]`)
		}
		out = append(out, channel.Template{ExternalID: item.ID, Name: item.Name, Language: item.Language,
			Category: item.Category, Status: item.Status, Quality: item.Quality, Components: components})
	}
	return out, nil
}

func (p *Provider) DeleteForAll(context.Context, channel.Credentials, channel.DeleteInput) error {
	return errUnsupportedAction
}

type limitedReadCloser struct {
	io.Reader
	closer io.Closer
}

func (r *limitedReadCloser) Close() error { return r.closer.Close() }

type httpError struct{ status int }

func (e *httpError) Error() string       { return fmt.Sprintf("meta whatsapp: upstream status %d", e.status) }
func (e *httpError) HTTPStatusCode() int { return e.status }

func (p *Provider) doJSON(ctx context.Context, config map[string]string, token, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	request, err := http.NewRequestWithContext(ctx, method, p.baseURL(config)+path, reader)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	client := p.http
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return errors.New("meta whatsapp: upstream unavailable")
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return &httpError{status: response.StatusCode}
	}
	if out == nil {
		return nil
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	if err := decoder.Decode(out); err != nil {
		return errors.New("meta whatsapp: invalid provider response")
	}
	return nil
}

func messagePayload(out channel.OutboundMessage) (map[string]any, error) {
	payload := map[string]any{"messaging_product": "whatsapp", "to": strings.TrimSpace(out.ToPhone)}
	if out.Reply != nil && strings.TrimSpace(out.Reply.ExternalMessageID) != "" {
		payload["context"] = map[string]any{"message_id": strings.TrimSpace(out.Reply.ExternalMessageID)}
	}
	switch strings.ToUpper(strings.TrimSpace(out.MessageType)) {
	case "", "TEXT":
		payload["type"] = "text"
		payload["text"] = map[string]any{"body": out.Content, "preview_url": false}
	case "TEMPLATE":
		if strings.TrimSpace(out.TemplateName) == "" || strings.TrimSpace(out.TemplateLanguage) == "" {
			return nil, errors.New("meta whatsapp: template name/language missing")
		}
		parameters := make([]map[string]string, 0, len(out.TemplateParameters))
		for _, parameter := range out.TemplateParameters {
			parameters = append(parameters, map[string]string{"type": "text", "text": parameter})
		}
		payload["type"] = "template"
		payload["template"] = map[string]any{"name": out.TemplateName, "language": map[string]string{"code": out.TemplateLanguage}, "components": []any{map[string]any{"type": "body", "parameters": parameters}}}
	case "IMAGE", "VIDEO", "AUDIO", "DOCUMENT":
		kind := strings.ToLower(strings.TrimSpace(out.MessageType))
		media := map[string]any{}
		if strings.HasPrefix(out.MediaURL, "http://") || strings.HasPrefix(out.MediaURL, "https://") {
			media["link"] = out.MediaURL
		} else {
			media["id"] = out.MediaURL
		}
		if out.MediaCaption != "" && kind != "audio" {
			media["caption"] = out.MediaCaption
		}
		if kind == "document" && out.MediaFileName != "" {
			media["filename"] = out.MediaFileName
		}
		payload["type"], payload[kind] = kind, media
	default:
		return nil, errors.New("meta whatsapp: message type unsupported")
	}
	return payload, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func timestamp(value string) time.Time {
	seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || seconds <= 0 {
		return time.Now().UTC()
	}
	return time.Unix(seconds, 0).UTC()
}
