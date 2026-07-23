package omnichannel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

type mediaAnalysisSource struct {
	MessageID      string
	ConversationID string
	InstanceID     string
	ExternalID     string
	MessageType    string
	MIMEType       string
	FileName       string
	SizeBytes      int64
	ContentHash    string
}

type n8nMediaAnalyzer struct {
	store        *Store
	box          *secretbox.Box
	ai           *AIService
	webhookURL   string
	mediaBaseURL string
	internalKey  string
	client       *http.Client
}

func newN8NMediaAnalyzer(store *Store, box *secretbox.Box, ai *AIService, webhookURL, mediaBaseURL, internalKey string) *n8nMediaAnalyzer {
	if store == nil || box == nil || ai == nil || strings.TrimSpace(webhookURL) == "" || strings.TrimSpace(internalKey) == "" {
		return nil
	}
	return &n8nMediaAnalyzer{
		store: store, box: box, ai: ai, webhookURL: strings.TrimSpace(webhookURL),
		mediaBaseURL: strings.TrimRight(strings.TrimSpace(mediaBaseURL), "/"), internalKey: strings.TrimSpace(internalKey),
		client: &http.Client{Timeout: 2 * time.Minute},
	}
}

type n8nMediaRequest struct {
	SchemaVersion string `json:"schemaVersion"`
	Analysis      struct {
		ID         string `json:"id"`
		MessageID  string `json:"messageId"`
		Kind       string `json:"kind"`
		MIMEType   string `json:"mimeType"`
		FileName   string `json:"fileName"`
		SizeBytes  int64  `json:"sizeBytes"`
		MediaURL   string `json:"mediaUrl"`
		MediaToken string `json:"mediaToken"`
	} `json:"analysis"`
	Execution struct {
		Provider string `json:"provider"`
		Model    string `json:"model"`
		APIKey   string `json:"apiKey"`
	} `json:"execution"`
}

type n8nMediaResponse struct {
	OK         bool            `json:"ok"`
	ErrorCode  string          `json:"errorCode"`
	Result     json.RawMessage `json:"result"`
	ResultText string          `json:"resultText"`
	Usage      struct {
		PromptTokens     int     `json:"promptTokens"`
		CompletionTokens int     `json:"completionTokens"`
		CostUSD          float64 `json:"costUsd"`
	} `json:"usage"`
}

func (a *n8nMediaAnalyzer) AnalyzeMessage(ctx context.Context, accountID, messageID string) error {
	if a == nil {
		return nil
	}
	source, err := a.store.GetMediaAnalysisSource(ctx, accountID, messageID)
	if err != nil {
		return err
	}
	if isWhatsAppGroupExternalID(source.ExternalID) {
		return nil
	}
	agent, ok, err := a.store.ActiveAgentForInstance(ctx, accountID, source.InstanceID)
	if err != nil || !ok || agent.ActiveVersionID == nil {
		return err
	}
	version, err := a.store.GetVersionByID(ctx, accountID, agent.ID, *agent.ActiveVersionID)
	if err != nil {
		return err
	}
	plan, enabled, err := planMediaAnalysis(mediaAnalysisPlanInput{
		MessageID: source.MessageID, ConversationID: source.ConversationID,
		MessageType: source.MessageType, MimeType: source.MIMEType, SizeBytes: source.SizeBytes,
		ContentHash: source.ContentHash, AgentVersionID: version.ID, MediaConfig: version.MediaConfig,
	})
	if err != nil || !enabled {
		return err
	}
	binding, err := mediaCredentialForRole(version.MediaConfig, mediaRoleForKind(plan.Create.Kind))
	if err != nil {
		// Versoes publicadas antes do cofre global nao possuem credentialId.
		// A midia continua disponivel e passa a ser analisada depois que a versao
		// for salva novamente com uma credencial nomeada, sem enfileirar retries.
		if errors.Is(err, ErrValidation) {
			return nil
		}
		return err
	}
	plan.Create.CredentialID = &binding.CredentialID
	plan.Create.ExpiresAt = mediaAnalysisExpiry(version.MediaConfig)
	analysis, _, err := a.store.CreateMediaAnalysis(ctx, accountID, plan.Create)
	if err != nil {
		return err
	}
	if analysis.Status == MediaAnalysisStatusCompleted || analysis.Status == MediaAnalysisStatusBlocked {
		return nil
	}
	if plan.Blocked {
		_, err = a.store.FailMediaAnalysis(ctx, accountID, analysis.ID, MediaAnalysisStatusBlocked, plan.Code)
		return err
	}
	analysis, claimed, err := a.store.ClaimMediaAnalysis(ctx, accountID, analysis.ID)
	if err != nil || !claimed {
		return err
	}
	apiKey, err := a.ai.credentialAPIKey(ctx, accountID, binding.CredentialID, binding.Provider)
	if err != nil {
		return a.fail(ctx, accountID, analysis.ID, "provider_error", err)
	}
	mediaToken, err := IssueMediaStreamToken(a.box, accountID, source.MessageID, analysis.ID, 2*time.Minute)
	if err != nil {
		return a.fail(ctx, accountID, analysis.ID, "content_unavailable", err)
	}
	request := n8nMediaRequest{SchemaVersion: "media.request.v1"}
	request.Analysis.ID = analysis.ID
	request.Analysis.MessageID = source.MessageID
	request.Analysis.Kind = analysis.Kind
	request.Analysis.MIMEType = source.MIMEType
	request.Analysis.FileName = source.FileName
	request.Analysis.SizeBytes = source.SizeBytes
	request.Analysis.MediaURL = a.mediaBaseURL + "/v1/runtime/omnichannel/media/" + source.MessageID
	request.Analysis.MediaToken = mediaToken
	request.Execution.Provider = analysis.Provider
	request.Execution.Model = analysis.Model
	request.Execution.APIKey = apiKey

	started := time.Now()
	response, err := a.call(ctx, request)
	if err != nil {
		return a.fail(ctx, accountID, analysis.ID, mediaAnalysisN8NErrorCode(err), err)
	}
	resultText := strings.TrimSpace(response.ResultText)
	_, err = a.store.CompleteMediaAnalysis(ctx, accountID, analysis.ID, mediaAnalysisComplete{
		ResultText: &resultText, ResultJSON: response.Result,
		PromptTokens: response.Usage.PromptTokens, CompletionTokens: response.Usage.CompletionTokens,
		CostUSD: response.Usage.CostUSD, LatencyMS: int(time.Since(started).Milliseconds()),
	})
	return err
}

func (a *n8nMediaAnalyzer) call(ctx context.Context, payload n8nMediaRequest) (n8nMediaResponse, error) {
	if validateBrainHTTPURL(a.webhookURL) != nil || a.mediaBaseURL == "" {
		return n8nMediaResponse{}, errors.New("media analysis: n8n configuration invalid")
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return n8nMediaResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.webhookURL, bytes.NewReader(raw))
	if err != nil {
		return n8nMediaResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Omni-Internal-Token", a.internalKey)
	resp, err := a.client.Do(req)
	if err != nil {
		return n8nMediaResponse{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return n8nMediaResponse{}, &jobs.StatusError{StatusCode: resp.StatusCode, Err: errors.New("media analysis: n8n rejected request")}
	}
	var out n8nMediaResponse
	if json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&out) != nil || !out.OK || len(out.Result) == 0 {
		return n8nMediaResponse{}, errors.New("media analysis: invalid n8n response")
	}
	return out, nil
}

func (a *n8nMediaAnalyzer) fail(ctx context.Context, accountID, analysisID, code string, cause error) error {
	_, _ = a.store.FailMediaAnalysis(ctx, accountID, analysisID, MediaAnalysisStatusFailed, code)
	return cause
}

func mediaAnalysisN8NErrorCode(err error) string {
	var statusErr *jobs.StatusError
	if errors.As(err, &statusErr) && statusErr.StatusCode == http.StatusRequestTimeout {
		return "timeout"
	}
	return "provider_error"
}

func mediaRoleForKind(kind string) string {
	switch kind {
	case MediaAnalysisKindTranscription:
		return "audio"
	case MediaAnalysisKindVision:
		return "image"
	case MediaAnalysisKindVideo:
		return "video"
	default:
		return "document"
	}
}

func mediaCredentialForRole(config json.RawMessage, role string) (mediaCredentialBinding, error) {
	bindings, err := mediaCredentialBindings(config)
	if err != nil {
		return mediaCredentialBinding{}, err
	}
	for _, binding := range bindings {
		if binding.Role == role {
			return binding, nil
		}
	}
	return mediaCredentialBinding{}, ErrValidation
}

func mediaAnalysisExpiry(config json.RawMessage) *time.Time {
	var root struct {
		RetentionDays int `json:"retentionDays"`
	}
	_ = json.Unmarshal(config, &root)
	if root.RetentionDays <= 0 {
		return nil
	}
	expires := time.Now().UTC().Add(time.Duration(root.RetentionDays) * 24 * time.Hour)
	return &expires
}
