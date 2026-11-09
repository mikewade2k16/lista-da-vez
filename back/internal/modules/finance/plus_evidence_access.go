package finance

import (
	"context"
	"encoding/json"
)

func (s *PlusService) ValidateOutput(ctx context.Context, sc PlusScope, raw json.RawMessage) error {
	var payload struct {
		Evidence []struct {
			ChunkID string `json:"chunkId"`
		} `json:"evidence"`
		Messages []struct {
			Envelope json.RawMessage `json:"plusEnvelope"`
		} `json:"messages"`
	}
	if json.Unmarshal(raw, &payload) != nil {
		return plusError("invalid_tool_output", 502)
	}
	ids := []string{}
	for _, v := range payload.Evidence {
		ids = append(ids, v.ChunkID)
	}
	for _, message := range payload.Messages {
		if len(message.Envelope) == 0 || string(message.Envelope) == "null" {
			continue
		}
		var envelope struct {
			Evidence []struct {
				ChunkID string `json:"chunkId"`
			} `json:"evidence"`
		}
		if json.Unmarshal(message.Envelope, &envelope) != nil {
			return plusError("invalid_tool_output", 502)
		}
		for _, v := range envelope.Evidence {
			ids = append(ids, v.ChunkID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	if s.ports.ValidateEvidence == nil {
		return plusError("capability_unavailable", 503)
	}
	return s.ports.ValidateEvidence(ctx, sc, ids)
}
