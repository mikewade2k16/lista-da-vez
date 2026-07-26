package customerdata

import (
	"context"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

const permCapabilitiesManage = "customer_data.capabilities.manage"

var capabilityCatalog = []string{
	CapabilityCore,
	CapabilityIdentity,
	CapabilityMatchingMerge,
	CapabilityOffline,
	CapabilitySegmentation,
	"segment_exports",
}

var writerCatalog = []string{
	WriterRelationship,
	WriterIdentity,
	WriterNote,
	WriterConsent,
	WriterMerge,
	WriterSegment,
}

func (s *Service) GetControlState(
	ctx context.Context,
	principal auth.Principal,
	clientAccountID string,
) (ControlStateView, error) {
	scope, err := s.authorizedScope(ctx, principal, clientAccountID, permCapabilitiesManage)
	if err != nil {
		return ControlStateView{}, err
	}
	capabilityRows, err := s.repo.ListCapabilityStates(ctx, scope)
	if err != nil {
		return ControlStateView{}, err
	}
	writerRows, err := s.repo.ListWriterStates(ctx, scope)
	if err != nil {
		return ControlStateView{}, err
	}
	capabilities := make(map[string]CapabilityState, len(capabilityRows))
	for _, item := range capabilityRows {
		capabilities[item.CapabilityKey] = item
	}
	writers := make(map[string]WriterState, len(writerRows))
	for _, item := range writerRows {
		writers[item.EntityKey] = item
	}
	out := ControlStateView{ClientAccountID: scope.ClientAccountID}
	for _, key := range capabilityCatalog {
		item, ok := capabilities[key]
		if !ok {
			item = CapabilityState{CapabilityKey: key, Mode: CapabilityOff}
		}
		out.Capabilities = append(out.Capabilities, item)
	}
	for _, key := range writerCatalog {
		item, ok := writers[key]
		if !ok {
			item = WriterState{EntityKey: key, Mode: WriterLegacy}
		}
		out.Writers = append(out.Writers, item)
	}
	return out, nil
}

func (s *Service) SetCapabilityState(
	ctx context.Context,
	principal auth.Principal,
	clientAccountID, capability string,
	input CapabilityStateInput,
) (CapabilityState, bool, error) {
	scope, err := s.authorizedScope(ctx, principal, clientAccountID, permCapabilitiesManage)
	if err != nil {
		return CapabilityState{}, false, err
	}
	if !contains(capabilityCatalog, capability) {
		return CapabilityState{}, false, invalid("capabilityKey", "unsupported")
	}
	if input.Mode != CapabilityOff && input.Mode != CapabilityShadow && input.Mode != CapabilityOn {
		return CapabilityState{}, false, invalid("mode", "unsupported")
	}
	if input.ExpectedRevision < 0 || len(strings.TrimSpace(input.IdempotencyKey)) < 8 ||
		strings.TrimSpace(input.Reason) == "" || len(input.Reason) > 1000 {
		return CapabilityState{}, false, invalid("controlState", "invalid_precondition")
	}
	current, err := s.repo.GetCapabilityState(ctx, scope, capability)
	if err != nil {
		return CapabilityState{}, false, err
	}
	if current.Mode == CapabilityOff && input.Mode == CapabilityOn && capability != CapabilityCore {
		return CapabilityState{}, false, ErrConflict
	}
	return s.repo.SetCapabilityState(ctx, scope, capability, input)
}

func (s *Service) SetWriterState(
	ctx context.Context,
	principal auth.Principal,
	clientAccountID, entity string,
	input WriterStateInput,
) (WriterState, bool, error) {
	scope, err := s.authorizedScope(ctx, principal, clientAccountID, permCapabilitiesManage)
	if err != nil {
		return WriterState{}, false, err
	}
	if !contains(writerCatalog, entity) {
		return WriterState{}, false, invalid("entityKey", "unsupported")
	}
	if input.Mode != WriterLegacy && input.Mode != WriterShadow && input.Mode != WriterNew {
		return WriterState{}, false, invalid("mode", "unsupported")
	}
	if input.ExpectedRevision < 0 || len(strings.TrimSpace(input.IdempotencyKey)) < 8 ||
		strings.TrimSpace(input.Reason) == "" || len(input.Reason) > 1000 {
		return WriterState{}, false, invalid("controlState", "invalid_precondition")
	}
	current, err := s.repo.GetWriterState(ctx, scope, entity)
	if err != nil {
		return WriterState{}, false, err
	}
	if !validWriterTransition(current.Mode, input.Mode) {
		return WriterState{}, false, ErrConflict
	}
	if input.Mode == WriterNew {
		if input.SourceChecksum == nil || input.TargetChecksum == nil ||
			strings.TrimSpace(*input.SourceChecksum) == "" ||
			*input.SourceChecksum != *input.TargetChecksum {
			return WriterState{}, false, invalid("checksums", "required_and_must_match")
		}
		capability := writerCapability(entity)
		mode, err := s.repo.CapabilityMode(ctx, scope, capability)
		if err != nil {
			return WriterState{}, false, err
		}
		if mode != CapabilityOn {
			return WriterState{}, false, ErrCapabilityDisabled
		}
	}
	return s.repo.SetWriterState(ctx, scope, entity, input)
}

func validWriterTransition(from, to WriterMode) bool {
	if from == to {
		return true
	}
	switch from {
	case WriterLegacy:
		return to == WriterShadow
	case WriterShadow:
		return to == WriterLegacy || to == WriterNew
	case WriterNew:
		return to == WriterShadow
	default:
		return false
	}
}

func writerCapability(entity string) string {
	switch entity {
	case WriterIdentity:
		return CapabilityIdentity
	case WriterMerge:
		return CapabilityMatchingMerge
	case WriterSegment:
		return CapabilitySegmentation
	default:
		return CapabilityCore
	}
}
