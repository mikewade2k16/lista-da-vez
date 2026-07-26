package omnichannel

import (
	"context"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

func (s *ChannelClientBindingService) Exceptions(
	ctx context.Context,
	accountID string,
	p auth.Principal,
) (ChannelClientBindingExceptionsResponse, error) {
	if err := s.requireManage(ctx, accountID, p); err != nil {
		return ChannelClientBindingExceptionsResponse{}, err
	}
	items, err := s.repo.ListChannelClientBindingExceptions(ctx, accountID)
	if err != nil {
		return ChannelClientBindingExceptionsResponse{}, err
	}
	return ChannelClientBindingExceptionsResponse{Items: items}, nil
}

func (s *ChannelClientBindingService) GetPolicy(
	ctx context.Context,
	accountID string,
	p auth.Principal,
) (ChannelClientBindingPolicyView, error) {
	if err := s.requireManage(ctx, accountID, p); err != nil {
		return ChannelClientBindingPolicyView{}, err
	}
	return s.repo.GetChannelClientBindingPolicy(ctx, accountID)
}

func (s *ChannelClientBindingService) UpdatePolicy(
	ctx context.Context,
	accountID string,
	p auth.Principal,
	in ChannelClientBindingPolicyInput,
) (ChannelClientBindingPolicyView, error) {
	if err := s.requireManage(ctx, accountID, p); err != nil {
		return ChannelClientBindingPolicyView{}, err
	}
	in.ChannelBindingMode = strings.ToLower(strings.TrimSpace(in.ChannelBindingMode))
	in.CustomerIntelligenceMode = strings.ToLower(strings.TrimSpace(in.CustomerIntelligenceMode))
	in.CustomerIntelligenceFailurePolicy = strings.ToLower(
		strings.TrimSpace(in.CustomerIntelligenceFailurePolicy),
	)
	if in.CustomerIntelligenceFailurePolicy == "" {
		in.CustomerIntelligenceFailurePolicy = "retry_then_handoff"
	}
	if (in.ChannelBindingMode != "legacy" &&
		in.ChannelBindingMode != "shadow" &&
		in.ChannelBindingMode != "enforced") ||
		(in.CustomerIntelligenceMode != "off" &&
			in.CustomerIntelligenceMode != "shadow" &&
			in.CustomerIntelligenceMode != "on") ||
		(in.CustomerIntelligenceFailurePolicy != "legacy_fallback" &&
			in.CustomerIntelligenceFailurePolicy != "retry_then_handoff" &&
			in.CustomerIntelligenceFailurePolicy != "immediate_handoff") ||
		in.ExpectedRevision < 1 {
		return ChannelClientBindingPolicyView{}, ErrValidation
	}
	return s.repo.UpdateChannelClientBindingPolicy(ctx, accountID, in)
}

func (s *ChannelClientBindingService) ResolveException(
	ctx context.Context,
	accountID string,
	p auth.Principal,
	in ResolveChannelClientBindingExceptionInput,
) (ChannelClientBindingView, error) {
	effectiveAt := in.EffectiveAt
	if effectiveAt.IsZero() {
		effectiveAt = s.now().UTC()
	}
	return s.Create(ctx, accountID, p, CreateChannelClientBindingInput{
		ClientAccountID:   in.ClientAccountID,
		Channel:           in.Channel,
		ChannelResourceID: in.ChannelResourceID,
		EffectiveFrom:     &effectiveAt,
		Reason:            in.Reason,
		IdempotencyKey:    in.IdempotencyKey,
	})
}

func (s *ChannelClientBindingService) CreateRepairPreview(
	ctx context.Context,
	accountID string,
	p auth.Principal,
	in ChannelClientBindingRepairPreviewInput,
) (ChannelClientBindingRepairJobView, error) {
	if err := s.requireManage(ctx, accountID, p); err != nil {
		return ChannelClientBindingRepairJobView{}, err
	}
	if !omnichannelUUIDPattern.MatchString(strings.TrimSpace(in.BindingID)) ||
		!in.ConfirmNoRetroactiveMove {
		return ChannelClientBindingRepairJobView{}, ErrValidation
	}
	binding, err := s.repo.GetChannelClientBinding(ctx, accountID, in.BindingID)
	if err != nil {
		return ChannelClientBindingRepairJobView{}, err
	}
	if _, err = s.accessibleClient(ctx, accountID, p, binding.ClientAccountID); err != nil {
		return ChannelClientBindingRepairJobView{}, err
	}
	reason, key, watermark, err := normalizeBindingMutation(in.Reason, in.IdempotencyKey, in.Watermark)
	if err != nil {
		return ChannelClientBindingRepairJobView{}, err
	}
	in.Reason, in.IdempotencyKey, in.Watermark = reason, key, watermark
	requestHash := channelBindingRequestHash("repair-preview", map[string]any{
		"bindingId":     in.BindingID,
		"watermark":     watermark.Format(time.RFC3339Nano),
		"includeClosed": in.IncludeClosed,
		"reason":        reason,
	})
	return s.repo.CreateChannelClientBindingRepairPreview(ctx, accountID, p, in, requestHash)
}

func (s *ChannelClientBindingService) ApplyRepair(
	ctx context.Context,
	accountID string,
	p auth.Principal,
	in ChannelClientBindingRepairApplyInput,
) (ChannelClientBindingRepairJobView, error) {
	if err := s.requireManage(ctx, accountID, p); err != nil {
		return ChannelClientBindingRepairJobView{}, err
	}
	if !in.Confirm ||
		!omnichannelUUIDPattern.MatchString(strings.TrimSpace(in.PreviewID)) ||
		len(strings.TrimSpace(in.PreviewChecksum)) != 64 {
		return ChannelClientBindingRepairJobView{}, ErrValidation
	}
	preview, err := s.repo.GetChannelClientBindingRepairJob(ctx, accountID, in.PreviewID)
	if err != nil {
		return ChannelClientBindingRepairJobView{}, err
	}
	binding, err := s.repo.GetChannelClientBinding(ctx, accountID, preview.BindingID)
	if err != nil {
		return ChannelClientBindingRepairJobView{}, err
	}
	if _, err = s.accessibleClient(ctx, accountID, p, binding.ClientAccountID); err != nil {
		return ChannelClientBindingRepairJobView{}, err
	}
	reason, key, _, err := normalizeBindingMutation(in.Reason, in.IdempotencyKey, s.now().UTC())
	if err != nil {
		return ChannelClientBindingRepairJobView{}, err
	}
	in.Reason, in.IdempotencyKey = reason, key
	requestHash := channelBindingRequestHash("repair-apply", map[string]any{
		"previewId":       in.PreviewID,
		"previewChecksum": in.PreviewChecksum,
		"reason":          reason,
	})
	return s.repo.ApplyChannelClientBindingRepair(ctx, accountID, p, in, requestHash)
}

func (s *ChannelClientBindingService) GetRepairJob(
	ctx context.Context,
	accountID, jobID string,
	p auth.Principal,
) (ChannelClientBindingRepairJobView, error) {
	if err := s.requireManage(ctx, accountID, p); err != nil {
		return ChannelClientBindingRepairJobView{}, err
	}
	if !omnichannelUUIDPattern.MatchString(strings.TrimSpace(jobID)) {
		return ChannelClientBindingRepairJobView{}, ErrValidation
	}
	job, err := s.repo.GetChannelClientBindingRepairJob(ctx, accountID, jobID)
	if err != nil {
		return ChannelClientBindingRepairJobView{}, err
	}
	if _, err = s.accessibleClient(ctx, accountID, p, job.ClientAccountID); err != nil {
		return ChannelClientBindingRepairJobView{}, err
	}
	return job, nil
}
