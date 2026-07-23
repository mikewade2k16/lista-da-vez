package socialpublishing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const maxGraphResponseBytes = 1 << 20

type InstagramProvider interface {
	ValidateToken(ctx context.Context, accessToken string) (InstagramProfile, error)
	CreateImageContainer(
		ctx context.Context,
		accessToken, igUserID, mediaURL, caption, altText string,
	) (string, error)
	PublishContainer(ctx context.Context, accessToken, igUserID, creationID string) (string, error)
	FetchPermalink(ctx context.Context, accessToken, mediaID string) (string, error)
	FetchMediaInsights(ctx context.Context, accessToken, mediaID string) (Analytics, error)
}

type InstagramGraphProvider struct {
	baseURL string
	client  *http.Client
}

func NewInstagramGraphProvider(graphBase string, clients ...*http.Client) *InstagramGraphProvider {
	graphBase = strings.TrimRight(strings.TrimSpace(graphBase), "/")
	if graphBase == "" {
		graphBase = DefaultGraphURL
	}
	client := &http.Client{Timeout: 20 * time.Second}
	if len(clients) > 0 && clients[0] != nil {
		client = clients[0]
	}
	return &InstagramGraphProvider{baseURL: graphBase, client: client}
}

type ProviderError struct {
	StatusCode int
	Code       int
}

func (e *ProviderError) Error() string {
	if e.Code != 0 {
		return fmt.Sprintf("instagram graph: status %d code %d", e.StatusCode, e.Code)
	}
	return fmt.Sprintf("instagram graph: status %d", e.StatusCode)
}

func (p *InstagramGraphProvider) ValidateToken(
	ctx context.Context,
	accessToken string,
) (InstagramProfile, error) {
	query := url.Values{}
	query.Set("fields", "user_id,username,account_type,media_count")
	var response struct {
		UserID      string `json:"user_id"`
		ID          string `json:"id"`
		Username    string `json:"username"`
		AccountType string `json:"account_type"`
		MediaCount  int64  `json:"media_count"`
	}
	if err := p.do(ctx, http.MethodGet, "/me", accessToken, query, &response); err != nil {
		return InstagramProfile{}, err
	}
	userID := strings.TrimSpace(response.UserID)
	if userID == "" {
		userID = strings.TrimSpace(response.ID)
	}
	accountType := strings.ToUpper(strings.TrimSpace(response.AccountType))
	if userID == "" || strings.TrimSpace(response.Username) == "" || !professionalAccountType(accountType) {
		return InstagramProfile{}, ErrInvalidToken
	}
	return InstagramProfile{
		UserID:      userID,
		Username:    strings.TrimSpace(response.Username),
		AccountType: accountType,
		MediaCount:  response.MediaCount,
	}, nil
}

func (p *InstagramGraphProvider) CreateImageContainer(
	ctx context.Context,
	accessToken, igUserID, mediaURL, caption, altText string,
) (string, error) {
	form := url.Values{}
	form.Set("image_url", mediaURL)
	if caption != "" {
		form.Set("caption", caption)
	}
	if altText != "" {
		form.Set("alt_text", altText)
	}
	var response struct {
		ID string `json:"id"`
	}
	if err := p.do(ctx, http.MethodPost, "/"+url.PathEscape(igUserID)+"/media", accessToken, form, &response); err != nil {
		return "", err
	}
	if strings.TrimSpace(response.ID) == "" {
		return "", ErrProviderUnavailable
	}
	return response.ID, nil
}

func (p *InstagramGraphProvider) PublishContainer(
	ctx context.Context,
	accessToken, igUserID, creationID string,
) (string, error) {
	form := url.Values{}
	form.Set("creation_id", creationID)
	var response struct {
		ID string `json:"id"`
	}
	path := "/" + url.PathEscape(igUserID) + "/media_publish"
	if err := p.do(ctx, http.MethodPost, path, accessToken, form, &response); err != nil {
		return "", err
	}
	if strings.TrimSpace(response.ID) == "" {
		return "", ErrProviderUnavailable
	}
	return response.ID, nil
}

func (p *InstagramGraphProvider) FetchPermalink(
	ctx context.Context,
	accessToken, mediaID string,
) (string, error) {
	query := url.Values{}
	query.Set("fields", "permalink")
	var response struct {
		Permalink string `json:"permalink"`
	}
	if err := p.do(
		ctx,
		http.MethodGet,
		"/"+url.PathEscape(mediaID),
		accessToken,
		query,
		&response,
	); err != nil {
		return "", err
	}
	return strings.TrimSpace(response.Permalink), nil
}

// FetchMediaInsights consulta cada metrica separadamente. A Graph rejeita o
// request inteiro quando uma metrica nao se aplica ao tipo de midia; chamadas
// individuais preservam as metricas suportadas sem esconder falha de token/rede.
func (p *InstagramGraphProvider) FetchMediaInsights(
	ctx context.Context,
	accessToken, mediaID string,
) (Analytics, error) {
	result := Analytics{PostID: "", CapturedAt: time.Now().UTC()}
	metrics := []string{"views", "reach", "total_interactions", "likes", "comments", "saved", "shares"}
	totalSeen := false
	for _, metric := range metrics {
		value, available, err := p.fetchMetric(ctx, accessToken, mediaID, metric)
		if err != nil {
			return Analytics{}, err
		}
		if !available {
			continue
		}
		switch metric {
		case "views":
			result.Views = value
		case "reach":
			result.Reach = value
		case "total_interactions":
			result.TotalInteractions = value
			totalSeen = true
		case "likes":
			result.Likes = value
		case "comments":
			result.Comments = value
		case "saved":
			result.Saved = value
		case "shares":
			result.Shares = value
		}
	}
	if !totalSeen {
		result.TotalInteractions = result.Likes + result.Comments + result.Saved + result.Shares
	}
	return result, nil
}

func (p *InstagramGraphProvider) fetchMetric(
	ctx context.Context,
	accessToken, mediaID, metric string,
) (int64, bool, error) {
	query := url.Values{}
	query.Set("metric", metric)
	var response struct {
		Data []struct {
			Name   string `json:"name"`
			Values []struct {
				Value json.RawMessage `json:"value"`
			} `json:"values"`
			TotalValue struct {
				Value json.RawMessage `json:"value"`
			} `json:"total_value"`
		} `json:"data"`
	}
	err := p.do(ctx, http.MethodGet, "/"+url.PathEscape(mediaID)+"/insights", accessToken, query, &response)
	if err != nil {
		var providerErr *ProviderError
		if errors.As(err, &providerErr) && providerErr.StatusCode == http.StatusBadRequest &&
			(providerErr.Code == 100 || providerErr.Code == 2108006) {
			return 0, false, nil
		}
		return 0, false, err
	}
	if len(response.Data) == 0 {
		return 0, false, nil
	}
	raw := response.Data[0].TotalValue.Value
	if len(response.Data[0].Values) > 0 {
		raw = response.Data[0].Values[len(response.Data[0].Values)-1].Value
	}
	value, ok := metricInt(raw)
	return value, ok, nil
}

func (p *InstagramGraphProvider) do(
	ctx context.Context,
	method, path, accessToken string,
	values url.Values,
	dst any,
) error {
	var body io.Reader
	endpoint := p.baseURL + path
	if method == http.MethodGet {
		if encoded := values.Encode(); encoded != "" {
			endpoint += "?" + encoded
		}
	} else {
		body = bytes.NewBufferString(values.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return fmt.Errorf("%w: criar request", ErrProviderUnavailable)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	if method != http.MethodGet {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	resp, err := p.client.Do(req) //nolint:gosec // base URL vem de configuracao confiavel
	if err != nil {
		return fmt.Errorf("%w: transporte", ErrProviderUnavailable)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxGraphResponseBytes+1))
	if err != nil || len(raw) > maxGraphResponseBytes {
		return fmt.Errorf("%w: resposta invalida", ErrProviderUnavailable)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return decodeProviderError(resp.StatusCode, raw)
	}
	if dst == nil || len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("%w: resposta invalida", ErrProviderUnavailable)
	}
	return nil
}

func decodeProviderError(status int, raw []byte) error {
	var envelope struct {
		Error struct {
			Code int `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(raw, &envelope)
	return &ProviderError{StatusCode: status, Code: envelope.Error.Code}
}

func professionalAccountType(value string) bool {
	switch value {
	case "BUSINESS", "CREATOR", "MEDIA_CREATOR":
		return true
	default:
		return false
	}
}

func metricInt(raw json.RawMessage) (int64, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, false
	}
	var number json.Number
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&number); err == nil {
		if integer, parseErr := strconv.ParseInt(number.String(), 10, 64); parseErr == nil {
			return integer, true
		}
		if decimal, parseErr := strconv.ParseFloat(number.String(), 64); parseErr == nil {
			return int64(decimal), true
		}
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		value, parseErr := strconv.ParseInt(text, 10, 64)
		return value, parseErr == nil
	}
	return 0, false
}
