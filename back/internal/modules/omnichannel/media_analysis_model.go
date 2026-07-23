package omnichannel

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"
)

const (
	MediaAnalysisKindTranscription = "transcription"
	MediaAnalysisKindVision        = "vision"
	MediaAnalysisKindVideo         = "video_summary"
	MediaAnalysisKindDocument      = "document_text"
	MediaAnalysisStatusQueued      = "queued"
	MediaAnalysisStatusProcessing  = "processing"
	MediaAnalysisStatusCompleted   = "completed"
	MediaAnalysisStatusFailed      = "failed"
	MediaAnalysisStatusBlocked     = "blocked"
)

var ErrMediaAnalysisInvalid = errors.New("omnichannel: invalid media analysis")

type mediaAnalysisRow struct {
	ID               string
	AccountID        string
	MessageID        string
	ConversationID   string
	Kind             string
	ContentHash      string
	Status           string
	Provider         string
	Model            string
	AgentVersionID   string
	CredentialID     *string
	ResultText       *string
	ResultJSON       json.RawMessage
	PromptTokens     int
	CompletionTokens int
	CostUSD          float64
	LatencyMS        int
	Attempts         int
	LastError        string
	CreatedAt        time.Time
	CompletedAt      *time.Time
	ExpiresAt        *time.Time
}

type mediaAnalysisCreate struct {
	MessageID      string
	ConversationID string
	Kind           string
	ContentHash    string
	Provider       string
	Model          string
	AgentVersionID string
	CredentialID   *string
	ExpiresAt      *time.Time
}

type mediaAnalysisComplete struct {
	ResultText       *string
	ResultJSON       json.RawMessage
	PromptTokens     int
	CompletionTokens int
	CostUSD          float64
	LatencyMS        int
}

type MediaAnalysisView struct {
	ID               string          `json:"id"`
	MessageID        string          `json:"messageId"`
	ConversationID   string          `json:"conversationId"`
	AnalysisKind     string          `json:"analysisKind"`
	Status           string          `json:"status"`
	Provider         string          `json:"provider"`
	Model            string          `json:"model"`
	AgentVersionID   string          `json:"agentVersionId"`
	ResultText       *string         `json:"resultText"`
	ResultJSON       json.RawMessage `json:"resultJson"`
	PromptTokens     int             `json:"promptTokens"`
	CompletionTokens int             `json:"completionTokens"`
	CostUSD          float64         `json:"costUsd"`
	LatencyMS        int             `json:"latencyMs"`
	Attempts         int             `json:"attempts"`
	LastError        string          `json:"lastError"`
	CreatedAt        time.Time       `json:"createdAt"`
	CompletedAt      *time.Time      `json:"completedAt"`
	ExpiresAt        *time.Time      `json:"expiresAt"`
}

func mediaAnalysisView(row mediaAnalysisRow) MediaAnalysisView {
	result := row.ResultJSON
	if len(result) == 0 || string(result) == "null" {
		result = json.RawMessage(`{}`)
	}
	return MediaAnalysisView{
		ID: row.ID, MessageID: row.MessageID, ConversationID: row.ConversationID,
		AnalysisKind: row.Kind, Status: row.Status, Provider: row.Provider, Model: row.Model,
		AgentVersionID: row.AgentVersionID, ResultText: row.ResultText, ResultJSON: result,
		PromptTokens: row.PromptTokens, CompletionTokens: row.CompletionTokens, CostUSD: row.CostUSD,
		LatencyMS: row.LatencyMS, Attempts: row.Attempts, LastError: row.LastError,
		CreatedAt: row.CreatedAt, CompletedAt: row.CompletedAt, ExpiresAt: row.ExpiresAt,
	}
}

const mediaAnalysisColumns = `id::text, account_id::text, message_id::text, conversation_id::text,
	analysis_kind, content_hash, status, provider, model, agent_version_id::text, result_text,
	result_json, prompt_tokens, completion_tokens, cost_usd::float8, latency_ms, attempts,
	last_error, created_at, completed_at, expires_at, credential_id::text`

func scanMediaAnalysis(row rowScanner) (mediaAnalysisRow, error) {
	var out mediaAnalysisRow
	err := row.Scan(&out.ID, &out.AccountID, &out.MessageID, &out.ConversationID, &out.Kind,
		&out.ContentHash, &out.Status, &out.Provider, &out.Model, &out.AgentVersionID,
		&out.ResultText, &out.ResultJSON, &out.PromptTokens, &out.CompletionTokens, &out.CostUSD,
		&out.LatencyMS, &out.Attempts, &out.LastError, &out.CreatedAt, &out.CompletedAt, &out.ExpiresAt,
		&out.CredentialID)
	return out, err
}

func validateMediaAnalysisCreate(in mediaAnalysisCreate) error {
	if !omnichannelUUIDPattern.MatchString(strings.TrimSpace(in.MessageID)) ||
		!omnichannelUUIDPattern.MatchString(strings.TrimSpace(in.ConversationID)) ||
		!omnichannelUUIDPattern.MatchString(strings.TrimSpace(in.AgentVersionID)) {
		return ErrMediaAnalysisInvalid
	}
	switch in.Kind {
	case MediaAnalysisKindTranscription, MediaAnalysisKindVision, MediaAnalysisKindVideo, MediaAnalysisKindDocument:
	default:
		return ErrMediaAnalysisInvalid
	}
	if len(in.ContentHash) != 64 || strings.Trim(in.ContentHash, "0123456789abcdef") != "" {
		return ErrMediaAnalysisInvalid
	}
	if len([]rune(in.Provider)) > 120 || len([]rune(in.Model)) > 120 {
		return ErrMediaAnalysisInvalid
	}
	return nil
}

func validateMediaAnalysisResult(in mediaAnalysisComplete) error {
	if in.PromptTokens < 0 || in.CompletionTokens < 0 || in.CostUSD < 0 || in.LatencyMS < 0 || in.LatencyMS > 10*60*1000 {
		return ErrMediaAnalysisInvalid
	}
	if len(in.ResultJSON) == 0 || !json.Valid(in.ResultJSON) || string(in.ResultJSON) == "null" {
		return ErrMediaAnalysisInvalid
	}
	var object map[string]any
	if json.Unmarshal(in.ResultJSON, &object) != nil || object == nil {
		return ErrMediaAnalysisInvalid
	}
	if in.ResultText != nil && len([]rune(*in.ResultText)) > 20000 {
		return ErrMediaAnalysisInvalid
	}
	return nil
}

func validateMediaAnalysisShape(kind string, raw json.RawMessage) error {
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil || object == nil {
		return ErrMediaAnalysisInvalid
	}
	var allowed, required map[string]struct{}
	switch kind {
	case MediaAnalysisKindTranscription:
		allowed = map[string]struct{}{"text": {}, "language": {}, "durationSeconds": {}, "segments": {}, "confidence": {}}
		required = map[string]struct{}{"text": {}, "language": {}, "durationSeconds": {}}
	case MediaAnalysisKindVision:
		allowed = map[string]struct{}{"summary": {}, "visibleText": {}, "objects": {}, "safetyFlags": {}, "confidence": {}}
		required = map[string]struct{}{"summary": {}, "visibleText": {}, "objects": {}, "safetyFlags": {}}
	case MediaAnalysisKindVideo:
		allowed = map[string]struct{}{"summary": {}, "visibleText": {}, "scenes": {}, "safetyFlags": {}, "durationSeconds": {}, "confidence": {}}
		required = map[string]struct{}{"summary": {}, "visibleText": {}, "scenes": {}, "safetyFlags": {}}
	case MediaAnalysisKindDocument:
		allowed = map[string]struct{}{"summary": {}, "extractedText": {}, "pageCount": {}, "truncated": {}, "warnings": {}}
		required = map[string]struct{}{"summary": {}, "extractedText": {}, "pageCount": {}, "truncated": {}, "warnings": {}}
	default:
		return ErrMediaAnalysisInvalid
	}
	for key := range object {
		if _, ok := allowed[key]; !ok {
			return ErrMediaAnalysisInvalid
		}
	}
	for key := range required {
		if _, ok := object[key]; !ok {
			return ErrMediaAnalysisInvalid
		}
	}
	for _, key := range []string{"summary", "text", "language", "visibleText", "extractedText"} {
		if value, ok := object[key]; ok {
			var text string
			if json.Unmarshal(value, &text) != nil || len([]rune(text)) > 50000 {
				return ErrMediaAnalysisInvalid
			}
		}
	}
	if value, ok := object["durationSeconds"]; ok && !validAnalysisNumber(value, 0, 600) {
		return ErrMediaAnalysisInvalid
	}
	if value, ok := object["pageCount"]; ok && !validAnalysisInteger(value, 0, 20) {
		return ErrMediaAnalysisInvalid
	}
	if value, ok := object["confidence"]; ok && !validAnalysisNumber(value, 0, 1) {
		return ErrMediaAnalysisInvalid
	}
	if value, ok := object["truncated"]; ok {
		var b bool
		if json.Unmarshal(value, &b) != nil {
			return ErrMediaAnalysisInvalid
		}
	}
	if value, ok := object["objects"]; ok && !validAnalysisStringArray(value, 200, 160) {
		return ErrMediaAnalysisInvalid
	}
	if value, ok := object["scenes"]; ok && !validAnalysisStringArray(value, 200, 500) {
		return ErrMediaAnalysisInvalid
	}
	if value, ok := object["safetyFlags"]; ok && !validAnalysisStringArray(value, 100, 160) {
		return ErrMediaAnalysisInvalid
	}
	if value, ok := object["warnings"]; ok && !validAnalysisStringArray(value, 100, 500) {
		return ErrMediaAnalysisInvalid
	}
	return nil
}

func validAnalysisNumber(raw json.RawMessage, min, max float64) bool {
	var value float64
	return json.Unmarshal(raw, &value) == nil && value >= min && value <= max
}

func validAnalysisInteger(raw json.RawMessage, min, max float64) bool {
	var value float64
	return json.Unmarshal(raw, &value) == nil && math.Trunc(value) == value && value >= min && value <= max
}

func validAnalysisStringArray(raw json.RawMessage, maxItems, maxLength int) bool {
	var values []string
	if json.Unmarshal(raw, &values) != nil || len(values) > maxItems {
		return false
	}
	for _, value := range values {
		if len([]rune(value)) == 0 || len([]rune(value)) > maxLength {
			return false
		}
	}
	return true
}
