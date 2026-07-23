package bi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type perolaHTTPClient interface {
	Do(request *http.Request) (*http.Response, error)
}

type perolaCredentials struct {
	CompanyKey  string
	CNPJEmpresa string
	Login       string
	Pass        string
	StaticToken string
}

type perolaClientOptions struct {
	BaseURL        string
	HTTPClient     perolaHTTPClient
	Credentials    perolaCredentials
	TokenTTL       time.Duration
	RequestTimeout time.Duration
	Now            func() time.Time
}

type perolaTokenRefresh struct {
	done  chan struct{}
	token string
	err   error
}

// PerolaClient owns the external API transport and the in-memory JWT lifecycle.
// It never persists credentials or tokens and retries a request at most once
// after an upstream 401.
type PerolaClient struct {
	baseURL        string
	httpClient     perolaHTTPClient
	credentials    perolaCredentials
	tokenTTL       time.Duration
	requestTimeout time.Duration
	now            func() time.Time

	tokenMu        sync.Mutex
	cachedToken    string
	tokenExpiresAt time.Time
	refresh        *perolaTokenRefresh
}

func newPerolaClient(options perolaClientOptions) *PerolaClient {
	baseURL := strings.TrimRight(strings.TrimSpace(options.BaseURL), "/")
	if baseURL == "" {
		baseURL = perolaAPIBase
	}

	httpClient := options.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 0}
	}

	tokenTTL := options.TokenTTL
	if tokenTTL <= 0 {
		tokenTTL = defaultPerolaTokenTTL
	}

	requestTimeout := options.RequestTimeout
	if requestTimeout <= 0 {
		requestTimeout = defaultPerolaRequestTimeout
	}

	now := options.Now
	if now == nil {
		now = time.Now
	}

	options.Credentials.CNPJEmpresa = onlyDigits(options.Credentials.CNPJEmpresa)

	return &PerolaClient{
		baseURL:        baseURL,
		httpClient:     httpClient,
		credentials:    options.Credentials,
		tokenTTL:       tokenTTL,
		requestTimeout: requestTimeout,
		now:            now,
	}
}

func (client *PerolaClient) Login(ctx context.Context, credentials perolaCredentials) (PerolaProxyResponse, error) {
	credentials.CNPJEmpresa = onlyDigits(credentials.CNPJEmpresa)
	if strings.TrimSpace(credentials.CompanyKey) == "" ||
		strings.TrimSpace(credentials.CNPJEmpresa) == "" ||
		strings.TrimSpace(credentials.Login) == "" ||
		credentials.Pass == "" {
		return PerolaProxyResponse{}, ErrConfiguration
	}

	body, err := json.Marshal(map[string]string{
		"login": credentials.Login,
		"pass":  credentials.Pass,
	})
	if err != nil {
		return PerolaProxyResponse{}, fmt.Errorf("%w: %v", ErrValidation, err)
	}

	requestCtx, cancel := client.requestContext(ctx)
	defer cancel()

	request, err := http.NewRequestWithContext(
		requestCtx,
		http.MethodPost,
		client.baseURL+"/sessoes",
		bytes.NewReader(body),
	)
	if err != nil {
		return PerolaProxyResponse{}, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("dsCompanyKey", credentials.CompanyKey)
	request.Header.Set("dsCnpjEmpresa", credentials.CNPJEmpresa)

	response, err := client.do(request)
	if err != nil {
		return PerolaProxyResponse{}, err
	}
	response.Token = extractBearerToken(response.Body)
	return response, nil
}

func (client *PerolaClient) Find(ctx context.Context, endpoint string, body []byte) (PerolaProxyResponse, error) {
	token, err := client.EnsureToken(ctx, false)
	if err != nil {
		return PerolaProxyResponse{}, err
	}

	response, err := client.FindWithToken(ctx, endpoint, body, client.credentials, token)
	if err != nil || response.UpstreamStatus != http.StatusUnauthorized || !client.hasLoginCredentials() {
		return response, err
	}

	refreshedToken, err := client.RefreshAfterUnauthorized(ctx, token)
	if err != nil {
		return PerolaProxyResponse{}, err
	}
	return client.FindWithToken(ctx, endpoint, body, client.credentials, refreshedToken)
}

func (client *PerolaClient) FindWithToken(
	ctx context.Context,
	endpoint string,
	body []byte,
	credentials perolaCredentials,
	token string,
) (PerolaProxyResponse, error) {
	credentials.CNPJEmpresa = onlyDigits(credentials.CNPJEmpresa)
	token = normalizeBearerToken(token)
	if strings.TrimSpace(credentials.CompanyKey) == "" ||
		strings.TrimSpace(credentials.CNPJEmpresa) == "" ||
		token == "" {
		return PerolaProxyResponse{}, ErrConfiguration
	}

	requestCtx, cancel := client.requestContext(ctx)
	defer cancel()

	request, err := http.NewRequestWithContext(
		requestCtx,
		http.MethodPost,
		client.baseURL+endpoint,
		bytes.NewReader(body),
	)
	if err != nil {
		return PerolaProxyResponse{}, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("dsCompanyKey", credentials.CompanyKey)
	request.Header.Set("dsCnpjEmpresa", credentials.CNPJEmpresa)
	request.Header.Set("Authorization", "Bearer "+token)

	return client.do(request)
}

func (client *PerolaClient) EnsureToken(ctx context.Context, forceRefresh bool) (string, error) {
	client.tokenMu.Lock()
	if !forceRefresh && client.cachedTokenValidLocked() {
		token := client.cachedToken
		client.tokenMu.Unlock()
		return token, nil
	}
	if activeRefresh := client.refresh; activeRefresh != nil {
		client.tokenMu.Unlock()
		select {
		case <-activeRefresh.done:
			return activeRefresh.token, activeRefresh.err
		case <-ctx.Done():
			return "", fmt.Errorf("%w: %v", ErrUpstream, ctx.Err())
		}
	}

	if !client.hasLoginCredentials() {
		client.tokenMu.Unlock()
		token := normalizeBearerToken(client.credentials.StaticToken)
		if token == "" {
			return "", ErrConfiguration
		}
		return token, nil
	}

	activeRefresh := &perolaTokenRefresh{done: make(chan struct{})}
	client.refresh = activeRefresh
	client.tokenMu.Unlock()

	response, err := client.Login(ctx, client.credentials)
	token := normalizeBearerToken(response.Token)
	if err == nil && (!response.OK || token == "") {
		err = ErrUpstream
	}

	client.tokenMu.Lock()
	if err == nil {
		client.cachedToken = token
		client.tokenExpiresAt = client.now().Add(client.tokenTTL)
	}
	activeRefresh.token = token
	activeRefresh.err = err
	client.refresh = nil
	close(activeRefresh.done)
	client.tokenMu.Unlock()

	return token, err
}

func (client *PerolaClient) RefreshAfterUnauthorized(ctx context.Context, rejectedToken string) (string, error) {
	rejectedToken = normalizeBearerToken(rejectedToken)

	client.tokenMu.Lock()
	if client.cachedTokenValidLocked() && client.cachedToken != rejectedToken {
		token := client.cachedToken
		client.tokenMu.Unlock()
		return token, nil
	}
	client.tokenMu.Unlock()

	return client.EnsureToken(ctx, true)
}

func (client *PerolaClient) InvalidateToken() {
	client.tokenMu.Lock()
	defer client.tokenMu.Unlock()
	client.cachedToken = ""
	client.tokenExpiresAt = time.Time{}
}

func (client *PerolaClient) cachedTokenValidLocked() bool {
	return client.cachedToken != "" && client.now().Before(client.tokenExpiresAt)
}

func (client *PerolaClient) hasLoginCredentials() bool {
	return strings.TrimSpace(client.credentials.CompanyKey) != "" &&
		strings.TrimSpace(client.credentials.CNPJEmpresa) != "" &&
		strings.TrimSpace(client.credentials.Login) != "" &&
		client.credentials.Pass != ""
}

func (client *PerolaClient) requestContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, client.requestTimeout)
}

func (client *PerolaClient) do(request *http.Request) (PerolaProxyResponse, error) {
	startedAt := client.now()
	response, err := client.httpClient.Do(request)
	if err != nil {
		return PerolaProxyResponse{}, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	return parsePerolaHTTPResponse(request, response, startedAt, client.now)
}

func parsePerolaHTTPResponse(
	request *http.Request,
	response *http.Response,
	startedAt time.Time,
	now func() time.Time,
) (PerolaProxyResponse, error) {
	rawBody, err := io.ReadAll(io.LimitReader(response.Body, 5<<20))
	if err != nil {
		return PerolaProxyResponse{}, fmt.Errorf("%w: %v", ErrUpstream, err)
	}

	body, rawText := parseUpstreamBody(response.Header.Get("Content-Type"), rawBody)
	return PerolaProxyResponse{
		OK:                 response.StatusCode >= 200 && response.StatusCode < 300,
		UpstreamStatus:     response.StatusCode,
		UpstreamStatusText: response.Status,
		UpstreamURL:        request.URL.String(),
		DurationMs:         now().Sub(startedAt).Milliseconds(),
		Headers:            selectedHeaders(response.Header),
		Body:               body,
		RawBody:            rawText,
	}, nil
}
