package omnichannel

import (
	"encoding/json"
	"strings"
)

// mediaAnalysisPlanInput is the small, already-normalized descriptor needed by
// the E3 policy gate. It deliberately carries no bytes, paths, URLs or secrets.
type mediaAnalysisPlanInput struct {
	MessageID      string
	ConversationID string
	MessageType    string
	MimeType       string
	SizeBytes      int64
	ContentHash    string
	AgentVersionID string
	MediaConfig    json.RawMessage
}

type mediaAnalysisPlan struct {
	Create  mediaAnalysisCreate
	Policy  mediaAnalysisPolicy
	Blocked bool
	Code    string
}

// planMediaAnalysis is the deterministic, side-effect-free E3 gate. It is
// intentionally separate from the outbox writer so policy can be tested without
// a model call or a database mutation. The caller must persist/enqueue only when
// Enabled is true and the own multimodal workflow is configured.
func planMediaAnalysis(in mediaAnalysisPlanInput) (mediaAnalysisPlan, bool, error) {
	if strings.TrimSpace(in.ContentHash) == "" {
		return mediaAnalysisPlan{}, false, ErrMediaAnalysisInvalid
	}
	policy, enabled, err := mediaAnalysisPolicyFor(in.MessageType, in.MimeType, in.MediaConfig)
	if err != nil {
		return mediaAnalysisPlan{}, false, err
	}
	if !enabled {
		return mediaAnalysisPlan{}, false, nil
	}
	if in.SizeBytes <= 0 || (policy.MaxBytes > 0 && in.SizeBytes > policy.MaxBytes) {
		return mediaAnalysisPlan{Policy: policy, Blocked: true, Code: "media_too_large"}, true, nil
	}
	create := mediaAnalysisCreate{
		MessageID:      in.MessageID,
		ConversationID: in.ConversationID,
		Kind:           policy.Kind,
		ContentHash:    strings.ToLower(strings.TrimSpace(in.ContentHash)),
		Provider:       strings.TrimSpace(policy.Provider),
		Model:          strings.TrimSpace(policy.Model),
		AgentVersionID: in.AgentVersionID,
	}
	if err := validateMediaAnalysisCreate(create); err != nil {
		return mediaAnalysisPlan{}, false, err
	}
	return mediaAnalysisPlan{Create: create, Policy: policy}, true, nil
}
