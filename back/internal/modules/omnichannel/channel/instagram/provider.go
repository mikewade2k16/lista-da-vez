// Package instagram is the Meta Graph adapter for professional Instagram
// accounts. It only translates/executes provider calls; CRM, moderation,
// windows and outbox state remain in the omnichannel Go module.
package instagram

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel/channel"
)

const providerID = "instagram"

var graphVersionPattern = regexp.MustCompile(`^v[0-9]{2,3}\.0$`)

// ValidateGraphVersion keeps the provider URL segment on an explicit allowlist.
// It is exported so tenant configuration validation can share the adapter rule.
func ValidateGraphVersion(value string) bool {
	return graphVersionPattern.MatchString(strings.TrimSpace(value))
}

type Provider struct {
	envBaseURL string
	http       *http.Client
}

func New(envBaseURL string) *Provider {
	return &Provider{envBaseURL: strings.TrimRight(strings.TrimSpace(envBaseURL), "/"), http: &http.Client{Timeout: 30 * time.Second}}
}
func (p *Provider) ID() string { return providerID }
func (p *Provider) Capabilities() channel.Capabilities {
	return channel.Capabilities{SupportsTemplates: false, SupportsReaction: false, SupportsSticker: false, SupportsGroups: false, MaxMediaBytes: 32 << 20}
}

var _ channel.Provider = (*Provider)(nil)
var _ channel.WebhookChallengeVerifier = (*Provider)(nil)
var _ channel.WebhookInstanceResolver = (*Provider)(nil)
var _ channel.SocialActionProvider = (*Provider)(nil)

type credentialEnvelope struct {
	AccessToken string `json:"accessToken"`
	AppSecret   string `json:"appSecret"`
	VerifyToken string `json:"verifyToken"`
}

func credentialsOf(cred channel.Credentials) (credentialEnvelope, error) {
	raw := strings.TrimSpace(cred.Token)
	if raw == "" {
		return credentialEnvelope{}, errors.New("instagram: credential missing")
	}
	var out credentialEnvelope
	if json.Unmarshal([]byte(raw), &out) != nil {
		out.AccessToken = raw
	}
	if strings.TrimSpace(out.AccessToken) == "" {
		return credentialEnvelope{}, errors.New("instagram: access token missing")
	}
	return out, nil
}
func version(config map[string]string) (string, error) {
	v := strings.TrimSpace(config["graphVersion"])
	if !ValidateGraphVersion(v) {
		return "", errors.New("instagram: graph version invalid")
	}
	return v, nil
}
func (p *Provider) baseURL(config map[string]string) string {
	if v := strings.TrimRight(strings.TrimSpace(config["baseURL"]), "/"); v != "" {
		return v
	}
	if p.envBaseURL != "" {
		return p.envBaseURL
	}
	return "https://graph.facebook.com"
}

func (p *Provider) WebhookInstanceKey(body []byte) string {
	var envelope webhookEnvelope
	if json.Unmarshal(body, &envelope) != nil {
		return ""
	}
	for _, e := range envelope.Entry {
		if strings.TrimSpace(e.ID) != "" {
			return strings.TrimSpace(e.ID)
		}
		for _, m := range e.Messaging {
			if m.Recipient.ID != "" {
				return strings.TrimSpace(m.Recipient.ID)
			}
		}
	}
	return ""
}

func (p *Provider) VerifyWebhook(hdr http.Header, body []byte, cred channel.Credentials) error {
	secrets, err := credentialsOf(cred)
	if err != nil || strings.TrimSpace(secrets.AppSecret) == "" {
		return errors.New("instagram: app secret missing")
	}
	v := strings.TrimSpace(hdr.Get("X-Hub-Signature-256"))
	if !strings.HasPrefix(strings.ToLower(v), "sha256=") {
		return errors.New("instagram: signature missing")
	}
	got, err := hex.DecodeString(strings.TrimSpace(v[len("sha256="):]))
	if err != nil || len(got) != sha256.Size {
		return errors.New("instagram: signature invalid")
	}
	h := hmac.New(sha256.New, []byte(secrets.AppSecret))
	_, _ = h.Write(body)
	if !hmac.Equal(got, h.Sum(nil)) {
		return errors.New("instagram: signature invalid")
	}
	return nil
}

func (p *Provider) VerifyWebhookChallenge(query map[string]string, cred channel.Credentials) (string, error) {
	secrets, err := credentialsOf(cred)
	if err != nil || strings.TrimSpace(secrets.VerifyToken) == "" {
		return "", errors.New("instagram: verify token missing")
	}
	if query["hub.mode"] != "subscribe" || !hmac.Equal([]byte(strings.TrimSpace(query["hub.verify_token"])), []byte(secrets.VerifyToken)) {
		return "", errors.New("instagram: challenge invalid")
	}
	challenge := strings.TrimSpace(query["hub.challenge"])
	if challenge == "" || len(challenge) > 512 {
		return "", errors.New("instagram: challenge invalid")
	}
	return challenge, nil
}

func (p *Provider) SendMessage(ctx context.Context, cred channel.Credentials, out channel.OutboundMessage) (channel.SendResult, error) {
	if out.SocialActionKind != "" {
		return p.SendSocialAction(ctx, cred, channel.SocialAction{Kind: out.SocialActionKind, ContentID: out.SocialContentID, Text: out.Content})
	}
	secrets, err := credentialsOf(cred)
	if err != nil {
		return channel.SendResult{Status: "FAILED"}, err
	}
	v, err := version(cred.Config)
	if err != nil {
		return channel.SendResult{Status: "FAILED"}, err
	}
	igID := strings.TrimSpace(cred.Config["igUserId"])
	if igID == "" {
		igID = strings.TrimSpace(out.InstanceName)
	}
	if igID == "" {
		return channel.SendResult{Status: "FAILED"}, errors.New("instagram: account id missing")
	}
	text := strings.TrimSpace(out.Content)
	if text == "" {
		return channel.SendResult{Status: "FAILED"}, errors.New("instagram: message empty")
	}
	path := "/" + v + "/" + igID + "/messages"
	payload := map[string]any{"recipient": map[string]string{"id": strings.TrimSpace(out.ToExternalID)}, "message": map[string]string{"text": text}}
	if out.SocialActionKind == "public_reply" && out.SocialContentID != "" {
		path = "/" + v + "/" + out.SocialContentID + "/replies"
		payload = map[string]any{"message": text}
	}
	if out.SocialActionKind == "private_reply" && out.SocialContentID != "" {
		payload = map[string]any{"recipient": map[string]string{"comment_id": out.SocialContentID}, "message": map[string]string{"text": text}}
	}
	var response struct {
		ID        string `json:"id"`
		MessageID string `json:"message_id"`
		Messages  []struct {
			ID string `json:"id"`
		} `json:"messages"`
	}
	if err := p.doJSON(ctx, cred.Config, secrets.AccessToken, http.MethodPost, path, payload, &response); err != nil {
		return channel.SendResult{Status: "FAILED"}, err
	}
	id := firstNonEmpty(response.ID, response.MessageID)
	if id == "" && len(response.Messages) > 0 {
		id = strings.TrimSpace(response.Messages[0].ID)
	}
	if id == "" {
		return channel.SendResult{Status: "FAILED"}, errors.New("instagram: response missing id")
	}
	return channel.SendResult{ExternalMessageID: id, Status: "SENT"}, nil
}

func (p *Provider) SendSocialAction(ctx context.Context, cred channel.Credentials, action channel.SocialAction) (channel.SendResult, error) {
	secrets, err := credentialsOf(cred)
	if err != nil {
		return channel.SendResult{Status: "FAILED"}, err
	}
	v, err := version(cred.Config)
	if err != nil {
		return channel.SendResult{Status: "FAILED"}, err
	}
	id := strings.TrimSpace(action.ContentID)
	text := strings.TrimSpace(action.Text)
	if id == "" || (action.Kind != "hide" && action.Kind != "ignore" && text == "") {
		return channel.SendResult{Status: "FAILED"}, errors.New("instagram: action payload invalid")
	}
	path := "/" + v + "/" + id + "/replies"
	payload := any(map[string]string{"message": text})
	switch action.Kind {
	case "private_reply":
		path = "/" + v + "/" + strings.TrimSpace(cred.Config["igUserId"]) + "/messages"
		payload = map[string]any{"recipient": map[string]string{"comment_id": id}, "message": map[string]string{"text": text}}
	case "hide":
		path = "/" + v + "/" + id
		payload = map[string]any{"hide": true}
	case "ignore":
		return channel.SendResult{Status: "SENT", ExternalMessageID: id}, nil
	case "public_reply":
	default:
		return channel.SendResult{Status: "FAILED"}, errors.New("instagram: action unsupported")
	}
	var response struct {
		ID string `json:"id"`
	}
	if err := p.doJSON(ctx, cred.Config, secrets.AccessToken, http.MethodPost, path, payload, &response); err != nil {
		return channel.SendResult{Status: "FAILED"}, err
	}
	return channel.SendResult{Status: "SENT", ExternalMessageID: firstNonEmpty(response.ID, id)}, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (p *Provider) DownloadMedia(ctx context.Context, cred channel.Credentials, ref channel.MediaRef) (io.ReadCloser, channel.MediaMeta, error) {
	secrets, err := credentialsOf(cred)
	if err != nil {
		return nil, channel.MediaMeta{}, err
	}
	rawURL := strings.TrimSpace(ref.MediaURL)
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || !allowedMediaHost(u.Hostname()) {
		return nil, channel.MediaMeta{}, errors.New("instagram: media URL not allowed")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, channel.MediaMeta{}, err
	}
	req.Header.Set("Authorization", "Bearer "+secrets.AccessToken)
	client := p.http
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, channel.MediaMeta{}, errors.New("instagram: media download failed")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = resp.Body.Close()
		return nil, channel.MediaMeta{}, &httpError{status: resp.StatusCode}
	}
	if resp.ContentLength > 32<<20 {
		_ = resp.Body.Close()
		return nil, channel.MediaMeta{}, errors.New("instagram: media too large")
	}
	return &limitedReadCloser{Reader: io.LimitReader(resp.Body, 32<<20+1), closer: resp.Body}, channel.MediaMeta{
		MimeType: resp.Header.Get("Content-Type"), SizeBytes: resp.ContentLength,
	}, nil
}

func allowedMediaHost(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	for _, suffix := range []string{"instagram.com", "facebook.com", "fbcdn.net", "cdninstagram.com"} {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return true
		}
	}
	return false
}
func (p *Provider) SendReaction(context.Context, channel.Credentials, channel.ReactionInput) error {
	return errors.New("instagram: reaction unsupported")
}
func (p *Provider) DeleteForAll(context.Context, channel.Credentials, channel.DeleteInput) error {
	return errors.New("instagram: delete unsupported")
}

func (p *Provider) doJSON(ctx context.Context, config map[string]string, token, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, p.baseURL(config)+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := p.http
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return errors.New("instagram: upstream unavailable")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New("instagram: upstream request rejected")
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out); err != nil {
		return errors.New("instagram: invalid provider response")
	}
	return nil
}

type limitedReadCloser struct {
	io.Reader
	closer io.Closer
}

func (r *limitedReadCloser) Close() error { return r.closer.Close() }

type httpError struct{ status int }

func (e *httpError) Error() string       { return "instagram: upstream request rejected" }
func (e *httpError) HTTPStatusCode() int { return e.status }
