package metaads

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type graphActionRequestError struct {
	status int
	cause  error
}

func (e *graphActionRequestError) Error() string {
	if e == nil || e.cause == nil {
		return "meta graph action: request failed"
	}
	return e.cause.Error()
}

func (e *graphActionRequestError) Unwrap() error { return e.cause }

type graphActionMutation struct {
	Success bool   `json:"success"`
	ID      string `json:"id"`
}

type graphActionCreate struct {
	ID string `json:"id"`
}

type graphActionCopy struct {
	CopiedCampaignID string `json:"copied_campaign_id"`
}

type graphActionCampaign struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	ConfiguredStatus string `json:"configured_status"`
	DailyBudget      string `json:"daily_budget"`
	LifetimeBudget   string `json:"lifetime_budget"`
}

func (c *MetaClient) UpdateCampaignAction(
	ctx context.Context,
	token, metaCampaignID string,
	values url.Values,
) (graphActionMutation, error) {
	var result graphActionMutation
	err := c.actionJSON(ctx, http.MethodPost, "/"+metaCampaignID, token, values, &result)
	return result, err
}

func (c *MetaClient) CreateCampaignAction(
	ctx context.Context,
	token, metaAdAccountID string,
	values url.Values,
) (graphActionCreate, error) {
	var result graphActionCreate
	err := c.actionJSON(ctx, http.MethodPost, "/act_"+metaAdAccountID+"/campaigns", token, values, &result)
	return result, err
}

func (c *MetaClient) CopyCampaignAction(
	ctx context.Context,
	token, metaCampaignID string,
	values url.Values,
) (graphActionCopy, error) {
	var result graphActionCopy
	err := c.actionJSON(ctx, http.MethodPost, "/"+metaCampaignID+"/copies", token, values, &result)
	return result, err
}

func (c *MetaClient) CreateAdSetAction(
	ctx context.Context,
	token, metaAdAccountID string,
	values url.Values,
) (graphActionCreate, error) {
	var result graphActionCreate
	err := c.actionJSON(ctx, http.MethodPost, "/act_"+metaAdAccountID+"/adsets", token, values, &result)
	return result, err
}

func (c *MetaClient) CreateAdCreativeAction(
	ctx context.Context,
	token, metaAdAccountID string,
	values url.Values,
) (graphActionCreate, error) {
	var result graphActionCreate
	err := c.actionJSON(ctx, http.MethodPost, "/act_"+metaAdAccountID+"/adcreatives", token, values, &result)
	return result, err
}

func (c *MetaClient) CreateAdAction(
	ctx context.Context,
	token, metaAdAccountID string,
	values url.Values,
) (graphActionCreate, error) {
	var result graphActionCreate
	err := c.actionJSON(ctx, http.MethodPost, "/act_"+metaAdAccountID+"/ads", token, values, &result)
	return result, err
}

func (c *MetaClient) GetCampaignAction(
	ctx context.Context,
	token, metaCampaignID string,
) (graphActionCampaign, error) {
	values := url.Values{}
	values.Set("fields", "id,name,configured_status,daily_budget,lifetime_budget")
	var result graphActionCampaign
	err := c.actionJSON(ctx, http.MethodGet, "/"+metaCampaignID, token, values, &result)
	return result, err
}

// actionJSON usa Authorization Bearer para o token nao entrar na URL. O host e
// a base de configuracao ja usada pelo MetaClient; path e somente ID validado.
func (c *MetaClient) actionJSON(
	ctx context.Context,
	method, path, token string,
	values url.Values,
	destination any,
) error {
	var body io.Reader
	endpoint := c.base + path
	if method == http.MethodGet {
		if encoded := values.Encode(); encoded != "" {
			endpoint += "?" + encoded
		}
	} else {
		body = strings.NewReader(values.Encode())
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, body) //nolint:gosec // host de config confiavel
	if err != nil {
		return &graphActionRequestError{cause: err}
	}
	request.Header.Set("Authorization", "Bearer "+token)
	if method != http.MethodGet {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	response, err := c.http.Do(request) //nolint:gosec // host de config confiavel
	if err != nil {
		return &graphActionRequestError{cause: err}
	}
	defer func() { _ = response.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return &graphActionRequestError{status: response.StatusCode, cause: err}
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return &graphActionRequestError{
			status: response.StatusCode,
			cause:  graphError(response.StatusCode, raw),
		}
	}
	if len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, destination); err != nil {
		return &graphActionRequestError{
			status: response.StatusCode,
			cause:  fmt.Errorf("meta graph action: invalid json: %w", err),
		}
	}
	return nil
}

func classifyGraphActionError(err error, reconciling bool) *ActionExecutorError {
	var requestErr *graphActionRequestError
	if !errors.As(err, &requestErr) {
		return &ActionExecutorError{
			Code: "execution_outcome_unknown", Message: "Nao foi possivel confirmar o resultado da Meta.", Ambiguous: true,
		}
	}
	// Somente um 4xx recebido e decodificado como rejeicao comprova que o
	// efeito nao ocorreu. Falha de rede, 5xx e inclusive erro de leitura/JSON
	// apos 2xx sao ambiguos porque a mutacao ja pode ter sido aplicada.
	if reconciling || !definitiveGraphActionRejection(requestErr.status) {
		return &ActionExecutorError{
			Code: "execution_outcome_unknown", Message: "Nao foi possivel confirmar o resultado da Meta.", Ambiguous: true,
		}
	}
	return &ActionExecutorError{
		Code: "meta_rejected", Message: "A Meta recusou a alteracao. Revise a campanha e as permissoes.",
	}
}

// Uma resposta 4xx normalmente comprova rejeicao antes da mutacao. Excluimos
// os status que podem ser produzidos por timeout, concorrencia intermediaria
// ou throttling, pois nesses casos repetir automaticamente seria inseguro.
func definitiveGraphActionRejection(status int) bool {
	if status < http.StatusBadRequest || status >= http.StatusInternalServerError {
		return false
	}
	switch status {
	case http.StatusRequestTimeout, http.StatusConflict, http.StatusTooEarly, http.StatusTooManyRequests:
		return false
	default:
		return true
	}
}
