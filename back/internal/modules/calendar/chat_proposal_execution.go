package calendar

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/tasks"
)

var (
	ErrProposalExecutionUnavailable = errors.New("calendar: proposal execution unavailable")
	ErrProposalSnapshotMissing      = errors.New("calendar: proposal snapshot missing")
	ErrProposalCrossModuleEffect    = errors.New("calendar: proposal has cross module effect")
	ErrProposalExecutionUnknown     = errors.New("calendar: proposal execution outcome unknown")
)

type ChatProposalExecutionView struct {
	Status     string `json:"status"`
	CanConfirm bool   `json:"canConfirm"`
	ErrorCode  string `json:"errorCode,omitempty"`
	Message    string `json:"message,omitempty"`
}

type ConfirmChatProposalRequest struct {
	Fields   *ChatProposalFields `json:"fields,omitempty"`
	ClientID string              `json:"clientId,omitempty"`
}

type ConfirmChatProposalResult struct {
	Message  ChatMessageView `json:"message"`
	Resource any             `json:"resource,omitempty"`
}

type chatProposalExecutionCommand struct {
	AccountID        string
	StorageAccountID string
	LockedClientID   string
	ConversationID   string
	MessageID        string
	ProposalID       string
	ConfirmationKey  string
	ActorUserID      string
	ActorLabel       string
	ActorRole        auth.Role
	RequestHash      []byte
	Proposal         StoredProposal
	CapabilityMode   AssistantExecutionCapabilityProvider
}

type chatProposalExecutionResult struct {
	Message       ChatMessage
	Resource      json.RawMessage
	Replayed      bool
	TaskMutation  *tasks.AssistantTaskMutationResult
	EventMutation *chatEventPostCommit
}

type chatEventPostCommit struct {
	Event               CalendarEvent
	LinkedTaskID        string
	PreviousDescription string
	Deleted             bool
}

type chatProposalExecutionStore interface {
	ExecuteChatProposal(ctx context.Context, command chatProposalExecutionCommand) (chatProposalExecutionResult, error)
}

func proposalExecutionViews(proposals []StoredProposal, items []AIContextEvent) []StoredProposal {
	out := append([]StoredProposal(nil), proposals...)
	events := make(map[string]AIContextEvent, len(items))
	for _, item := range items {
		if id := normalizeUUID(item.ID); id != "" {
			events[id] = item
		}
	}
	for index := range out {
		proposal := &out[index]
		if proposal.Kind == "metaAction" {
			continue
		}
		view := &ChatProposalExecutionView{Status: "pending", CanConfirm: proposal.Status == "pending"}
		switch proposal.Kind {
		case "event":
			if proposal.Status != "pending" {
				view.Status = proposal.Status
			} else if proposal.Action != "create" {
				target, exists := events[normalizeUUID(proposal.Fields.TargetID)]
				if !exists || target.Version <= 0 {
					view.Status = "unavailable"
					view.CanConfirm = false
					view.ErrorCode = "proposal_snapshot_missing"
					view.Message = "Gere uma nova proposta: falta o snapshot versionado do evento alvo."
				}
			}
		case "note", "clientProfile":
			if proposal.Status != "pending" {
				view.Status = proposal.Status
			}
		case "task", "taskItem":
			if proposal.Status != "pending" {
				view.Status = proposal.Status
			} else if proposal.Kind == "task" && proposal.Action == "create" && normalizeUUID(proposal.Fields.BoardID) == "" {
				view.Status = "unavailable"
				view.CanConfirm = false
				view.ErrorCode = "tasks_not_configured"
				view.Message = "Configure um board de Tasks nas integracoes do Calendario."
			}
		default:
			view.Status = "unavailable"
			view.CanConfirm = false
			view.ErrorCode = "proposal_kind_unavailable"
			view.Message = "Este tipo de proposta nao possui executor seguro."
		}
		proposal.Execution = view
	}
	return out
}

func (s *Service) ConfirmChatProposal(
	ctx context.Context,
	accountID, conversationID, messageID, proposalID, confirmationKey string,
	principal auth.Principal,
	req ConfirmChatProposalRequest,
) (ConfirmChatProposalResult, error) {
	confirmationKey = strings.TrimSpace(confirmationKey)
	if !validChatIdempotencyKey(confirmationKey) {
		return ConfirmChatProposalResult{}, ErrIdempotencyKeyRequired
	}
	account := strings.TrimSpace(accountID)
	access, err := s.resolveChatAccess(ctx, principal, account)
	if err != nil {
		return ConfirmChatProposalResult{}, err
	}
	conv, err := s.authorizeConversation(ctx, access, account, conversationID, principal.UserID)
	if err != nil || !access.canAccessSavedScope(conv.ScopeMode, ptrToStr(conv.ScopeClientID)) {
		return ConfirmChatProposalResult{}, ErrNotFound
	}
	capabilities, err := s.resolveConversationCapabilities(ctx, account, conv.EntrySurface, principal)
	if err != nil {
		if isAssistantConversationAccessDenied(err) {
			return ConfirmChatProposalResult{}, ErrNotFound
		}
		return ConfirmChatProposalResult{}, err
	}
	message, err := s.store.GetMessage(ctx, account, conv.ID, strings.TrimSpace(messageID))
	if err != nil {
		return ConfirmChatProposalResult{}, mapNotFound(err)
	}
	proposal, ok := storedProposalByID(message.Proposals, proposalID)
	if !ok || proposal.Kind == "metaAction" {
		return ConfirmChatProposalResult{}, ErrNotFound
	}
	if proposal.Kind == "task" || proposal.Kind == "taskItem" {
		bypass := principal.Role == auth.RoleOwner || principal.Role == auth.RolePlatformAdmin
		allowed := assistantTaskActionAllowed(principal, proposal.Action, bypass)
		if proposal.Kind == "taskItem" {
			allowed = bypass || (principal.PermissionsResolved && containsPermission(principal.Permissions, "tasks.tasks.edit"))
		}
		if assistantCapabilityMode(capabilities, "tasks") != assistantModeWrite || !allowed {
			return ConfirmChatProposalResult{}, ErrForbidden
		}
	} else if assistantCapabilityMode(capabilities, "calendar") != assistantModeWrite || !canManageCalendarProposal(principal) {
		return ConfirmChatProposalResult{}, ErrForbidden
	}
	// Notas mensais pertencem a agenda compartilhada, nao a um client_id. Sem
	// granularidade por cliente no schema, uma conversa client-scoped nao pode
	// alterar a nota global da agencia.
	if proposal.Kind == "note" && (!access.IsAgency || conv.ScopeMode != chatScopeAll) {
		return ConfirmChatProposalResult{}, ErrForbidden
	}
	if proposal.Status != "pending" && proposal.Status != "accepted" {
		return ConfirmChatProposalResult{}, ErrNotFound
	}
	proposal.Fields = overlayEditableProposalFields(proposal.Fields, req.Fields)
	proposal.Fields.TargetID = strings.TrimSpace(storedTargetID(message.Proposals, proposalID))
	proposal.Fields.MetaAction = nil

	selectedClientID := strings.TrimSpace(req.ClientID)
	if selectedClientID != "" {
		selectedClientID = normalizeUUID(selectedClientID)
		if selectedClientID == "" {
			return ConfirmChatProposalResult{}, ErrInvalidClient
		}
		proposal.Fields.ClientID = selectedClientID
	}
	if clean := sanitizeProposal(&ChatProposal{
		Action: proposal.Action, Kind: proposal.Kind, Fields: proposal.Fields,
	}); clean == nil {
		return ConfirmChatProposalResult{}, ErrInvalidProposalStatus
	} else {
		proposal.Action, proposal.Kind, proposal.Fields = clean.Action, clean.Kind, clean.Fields
	}
	if err := validateProposalClient(access, conv, &proposal); err != nil {
		return ConfirmChatProposalResult{}, err
	}
	if proposal.Action != "create" && normalizeUUID(proposal.Fields.TargetID) == "" {
		return ConfirmChatProposalResult{}, ErrProposalSnapshotMissing
	}

	scope, err := s.GetCalendarScope(ctx, account)
	if err != nil {
		return ConfirmChatProposalResult{}, err
	}
	lockedClientID := normalizeUUID(scope.LockedClientID)
	if conv.ScopeMode == chatScopeClient {
		// Agency users can select a client-scoped conversation even though the
		// account-level CalendarScope is not locked. Carry that saved scope into
		// the authoritative target query so a stale/corrupt proposal can never
		// update or delete an event owned by another visible client.
		lockedClientID = normalizeUUID(ptrToStr(conv.ScopeClientID))
		if lockedClientID == "" {
			return ConfirmChatProposalResult{}, ErrNotFound
		}
	}
	requestHash, err := hashJSON(struct {
		AccountID      string             `json:"accountId"`
		ConversationID string             `json:"conversationId"`
		MessageID      string             `json:"messageId"`
		ProposalID     string             `json:"proposalId"`
		Action         string             `json:"action"`
		Kind           string             `json:"kind"`
		Fields         ChatProposalFields `json:"fields"`
	}{account, conv.ID, message.ID, proposal.ID, proposal.Action, proposal.Kind, proposal.Fields})
	if err != nil {
		return ConfirmChatProposalResult{}, err
	}
	executionStore, ok := s.store.(chatProposalExecutionStore)
	if !ok {
		return ConfirmChatProposalResult{}, ErrProposalExecutionUnavailable
	}
	result, err := executionStore.ExecuteChatProposal(ctx, chatProposalExecutionCommand{
		AccountID: account, StorageAccountID: scope.StorageAccountID,
		LockedClientID: lockedClientID, ConversationID: conv.ID,
		MessageID: message.ID, ProposalID: proposal.ID, ConfirmationKey: confirmationKey,
		ActorUserID: principal.UserID, ActorLabel: principalDisplayName(principal),
		ActorRole:   principal.Role,
		RequestHash: requestHash, Proposal: proposal,
		CapabilityMode: s.assistantExecutionCapabilityProvider,
	})
	if err != nil {
		return ConfirmChatProposalResult{}, err
	}
	if !result.Replayed {
		s.publishConfirmedProposal(ctx, scope.StorageAccountID, proposal, result.Resource)
		if result.TaskMutation != nil {
			if taskService := s.tasksSvc(); taskService != nil {
				if taskAccess, accessErr := taskService.ResolveAccessContext(ctx, principal, account); accessErr == nil {
					taskService.FinalizeAssistantTaskMutation(ctx, taskAccess, *result.TaskMutation)
				}
			}
		}
		if mutation := result.EventMutation; mutation != nil && mutation.LinkedTaskID != "" {
			if mutation.Deleted {
				s.unlinkTask(ctx, scope.StorageAccountID, mutation.LinkedTaskID, mutation.Event.ID)
			} else {
				s.syncTaskFromEvent(
					ctx,
					scope.StorageAccountID,
					mutation.Event,
					mutation.LinkedTaskID,
					strings.TrimSpace(mutation.PreviousDescription) != strings.TrimSpace(mutation.Event.Description),
				)
			}
		}
	}
	var resource any
	if len(result.Resource) > 0 && string(result.Resource) != "{}" {
		_ = json.Unmarshal(result.Resource, &resource)
	}
	return ConfirmChatProposalResult{Message: messageViewFrom(result.Message), Resource: resource}, nil
}

func storedProposalByID(proposals []StoredProposal, proposalID string) (StoredProposal, bool) {
	proposalID = strings.TrimSpace(proposalID)
	for _, proposal := range proposals {
		if proposal.ID == proposalID {
			return proposal, true
		}
	}
	return StoredProposal{}, false
}

func storedTargetID(proposals []StoredProposal, proposalID string) string {
	proposal, ok := storedProposalByID(proposals, proposalID)
	if !ok {
		return ""
	}
	return proposal.Fields.TargetID
}

func canManageCalendarProposal(principal auth.Principal) bool {
	if principal.Role == auth.RoleOwner || principal.Role == auth.RolePlatformAdmin {
		return true
	}
	return principal.PermissionsResolved && containsPermission(principal.Permissions, "calendar.manage")
}

func validateProposalClient(access ChatAccess, conv ChatConversation, proposal *StoredProposal) error {
	clientID := normalizeUUID(proposal.Fields.ClientID)
	lockedClientID := ""
	if conv.ScopeMode == chatScopeClient {
		lockedClientID = normalizeUUID(ptrToStr(conv.ScopeClientID))
	}
	if lockedClientID != "" {
		if clientID != "" && clientID != lockedClientID {
			return ErrInvalidClient
		}
		clientID = lockedClientID
		proposal.Fields.ClientID = lockedClientID
	}
	if proposal.Kind == "clientProfile" && clientID == "" {
		return ErrInvalidClient
	}
	if clientID == "" {
		return nil
	}
	for _, visible := range access.VisibleClientIDs {
		if normalizeUUID(visible) == clientID {
			return nil
		}
	}
	return ErrInvalidClient
}

func overlayEditableProposalFields(base ChatProposalFields, edited *ChatProposalFields) ChatProposalFields {
	if edited == nil {
		return base
	}
	if value := strings.TrimSpace(edited.Title); value != "" {
		base.Title = value
	}
	if value := strings.TrimSpace(edited.Date); value != "" {
		base.Date = value
	}
	if value := strings.TrimSpace(edited.Time); value != "" {
		base.Time = value
	}
	if value := strings.TrimSpace(edited.Type); value != "" {
		base.Type = value
	}
	if value := strings.TrimSpace(edited.Status); value != "" {
		base.Status = value
	}
	if value := strings.TrimSpace(edited.Priority); value != "" {
		base.Priority = value
	}
	if value := strings.TrimSpace(edited.ResponsibleID); value != "" {
		base.ResponsibleID = value
	}
	if edited.InvolvedIDs != nil {
		base.InvolvedIDs = append([]string(nil), edited.InvolvedIDs...)
	}
	if value := strings.TrimSpace(edited.Description); value != "" {
		base.Description = value
	}
	if value := strings.TrimSpace(edited.ContentHTML); value != "" {
		base.ContentHTML = value
	}
	if value := strings.TrimSpace(edited.ClientID); value != "" {
		base.ClientID = value
	}
	if value := strings.TrimSpace(edited.ClientName); value != "" {
		base.ClientName = value
	}
	if edited.Note != nil {
		copy := *edited.Note
		base.Note = &copy
	}
	if edited.Profile != nil {
		copy := *edited.Profile
		base.Profile = &copy
	}
	return base
}

func (s *Service) publishConfirmedProposal(
	ctx context.Context,
	storageAccountID string,
	proposal StoredProposal,
	resource json.RawMessage,
) {
	switch proposal.Kind {
	case "event":
		var event EventView
		if json.Unmarshal(resource, &event) != nil {
			return
		}
		eventType := realtimeEventUpdated
		switch proposal.Action {
		case "create":
			eventType = realtimeEventCreated
		case "delete":
			eventType = realtimeEventDeleted
		}
		s.publishCalendar(ctx, RealtimeEvent{Type: eventType, AccountID: storageAccountID,
			ClientIDs: []string{event.ClientID}, ResourceID: event.ID, Date: event.Date, Version: event.Version})
	case "note":
		if proposal.Fields.Note != nil {
			s.publishCalendar(ctx, RealtimeEvent{Type: realtimeNoteUpdated, AccountID: storageAccountID,
				MonthKey: proposal.Fields.Note.Month})
		}
	case "clientProfile":
		s.publishCalendar(ctx, RealtimeEvent{Type: realtimeClientProfileUpdated, AccountID: storageAccountID,
			ResourceID: proposal.Fields.ClientID})
	}
}
