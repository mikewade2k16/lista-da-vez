package omnichannel

import (
	"encoding/json"
	"strings"
)

type mediaAnalysisPolicy struct {
	Kind           string
	Provider       string
	Model          string
	MaxBytes       int64
	MaxSeconds     int
	MaxPages       int
	AllowedMIME    []string
	IncludeInReply bool
}

func mediaAnalysisPolicyFor(messageType, mime string, raw json.RawMessage) (mediaAnalysisPolicy, bool, error) {
	config, err := normalizeMediaConfig(raw)
	if err != nil {
		return mediaAnalysisPolicy{}, false, err
	}
	var root map[string]json.RawMessage
	if json.Unmarshal(config, &root) != nil {
		return mediaAnalysisPolicy{}, false, ErrMediaAnalysisInvalid
	}
	kind, sectionName := "", ""
	switch strings.ToUpper(strings.TrimSpace(messageType)) {
	case "AUDIO":
		kind, sectionName = MediaAnalysisKindTranscription, "audio"
	case "IMAGE":
		kind, sectionName = MediaAnalysisKindVision, "image"
	case "VIDEO":
		kind, sectionName = MediaAnalysisKindVideo, "video"
	case "DOCUMENT":
		kind, sectionName = MediaAnalysisKindDocument, "document"
	default:
		return mediaAnalysisPolicy{}, false, nil
	}
	sectionRaw, ok := root[sectionName]
	if !ok {
		return mediaAnalysisPolicy{}, false, nil
	}
	var section map[string]json.RawMessage
	if json.Unmarshal(sectionRaw, &section) != nil {
		return mediaAnalysisPolicy{}, false, ErrMediaAnalysisInvalid
	}
	var enabled bool
	if json.Unmarshal(section["enabled"], &enabled) != nil || !enabled {
		return mediaAnalysisPolicy{}, false, nil
	}
	var provider, model string
	_ = json.Unmarshal(section["provider"], &provider)
	_ = json.Unmarshal(section["model"], &model)
	if provider == "" || model == "" {
		return mediaAnalysisPolicy{}, false, ErrMediaAnalysisInvalid
	}
	policy := mediaAnalysisPolicy{Kind: kind, Provider: provider, Model: model, MaxBytes: 60 << 20, MaxSeconds: 600, MaxPages: 20}
	_ = json.Unmarshal(root["includeInReply"], &policy.IncludeInReply)
	switch kind {
	case MediaAnalysisKindTranscription:
		if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(mime)), "audio/") {
			return mediaAnalysisPolicy{}, false, ErrMediaAnalysisInvalid
		}
		_ = json.Unmarshal(section["maxSeconds"], &policy.MaxSeconds)
	case MediaAnalysisKindVision:
		if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(mime)), "image/") {
			return mediaAnalysisPolicy{}, false, ErrMediaAnalysisInvalid
		}
		_ = json.Unmarshal(section["maxBytes"], &policy.MaxBytes)
	case MediaAnalysisKindVideo:
		if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(mime)), "video/") {
			return mediaAnalysisPolicy{}, false, ErrMediaAnalysisInvalid
		}
		_ = json.Unmarshal(section["maxBytes"], &policy.MaxBytes)
	case MediaAnalysisKindDocument:
		_ = json.Unmarshal(section["maxPages"], &policy.MaxPages)
		_ = json.Unmarshal(section["allowedMime"], &policy.AllowedMIME)
		if len(policy.AllowedMIME) > 0 && !containsFold(policy.AllowedMIME, strings.ToLower(strings.TrimSpace(mime))) {
			return mediaAnalysisPolicy{}, false, ErrMediaAnalysisInvalid
		}
	}
	return policy, true, nil
}

func containsFold(values []string, wanted string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), wanted) {
			return true
		}
	}
	return false
}
