package calendar

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/tasks"
)

// prepareTaskProposalExecutionFields snapshots the configured board into task
// creates before the card is persisted. The model and browser cannot choose or
// override this technical field.
func (s *Service) prepareTaskProposalExecutionFields(ctx context.Context, accountID string, proposals []ChatProposal) error {
	needsBoard := false
	for _, proposal := range proposals {
		if proposal.Kind == "task" && proposal.Action == "create" {
			needsBoard = true
			break
		}
	}
	if !needsBoard {
		return nil
	}
	config, err := s.store.GetConfig(ctx, strings.TrimSpace(accountID))
	if err != nil {
		return err
	}
	boardID := normalizeUUID(config.Tasks.BoardID)
	for index := range proposals {
		if proposals[index].Kind == "task" && proposals[index].Action == "create" {
			proposals[index].Fields.BoardID = boardID
		}
	}
	return nil
}

func executeTaskProposalTx(
	ctx context.Context,
	tx pgx.Tx,
	command chatProposalExecutionCommand,
	expectedVersion *int,
	before json.RawMessage,
	beforeHash []byte,
) (tasks.AssistantTaskMutationResult, error) {
	fields := command.Proposal.Fields
	clientID := normalizeUUID(fields.ClientID)
	if err := validateEventPeopleTx(ctx, tx, command.AccountID, fields.ResponsibleID, fields.InvolvedIDs); err != nil {
		return tasks.AssistantTaskMutationResult{}, err
	}
	contentHTML := strings.TrimSpace(fields.ContentHTML)
	if contentHTML == "" {
		contentHTML = descToHTML(fields.Description)
	}
	input := tasks.AssistantTaskMutationInput{
		AccountID: command.AccountID, ActorUserID: command.ActorUserID,
		Action: command.Proposal.Action, Kind: command.Proposal.Kind,
		TargetID: normalizeUUID(fields.TargetID), BoardID: normalizeUUID(fields.BoardID),
		Title: fields.Title, ContentHTML: contentHTML,
		Status: fields.Status, Priority: fields.Priority, DueDate: firstNonEmpty(fields.DueDate, fields.Date),
		StartDate: fields.StartDate, DueEndDate: fields.DueEndDate, Time: fields.Time,
		ColumnID: normalizeUUID(fields.ColumnID), ResponsibleUserID: normalizeUUID(fields.ResponsibleID),
		ClientAccountID: clientID, ClientName: fields.ClientName, Type: fields.Type,
		InvolvedIDs: append([]string(nil), fields.InvolvedIDs...), Archived: fields.Archived,
		ExpectedVersion: expectedVersion, BeforeSnapshot: before, BeforeHash: beforeHash,
		DeterministicItemID: "crow:" + command.MessageID + ":" + command.ProposalID,
	}
	if fields.TaskItem != nil {
		input.Item = &tasks.AssistantTaskItemInput{
			ID: fields.TaskItem.ID, Title: fields.TaskItem.Title, Status: fields.TaskItem.Status,
			StatusDate: fields.TaskItem.StatusDate, Completed: fields.TaskItem.Completed,
			CompletedDate: fields.TaskItem.CompletedDate,
		}
	}
	outcome, err := tasks.ExecuteAssistantTaskMutationTx(ctx, tx, input)
	if err == nil {
		return outcome, nil
	}
	switch {
	case errors.Is(err, tasks.ErrAssistantSnapshotMissing):
		return tasks.AssistantTaskMutationResult{}, ErrProposalSnapshotMissing
	case errors.Is(err, tasks.ErrAssistantSnapshotStale), errors.Is(err, tasks.ErrVersionConflict):
		return tasks.AssistantTaskMutationResult{}, ErrVersionConflict
	case errors.Is(err, tasks.ErrTaskNotFound), errors.Is(err, tasks.ErrBoardNotFound):
		return tasks.AssistantTaskMutationResult{}, ErrNotFound
	case errors.Is(err, tasks.ErrValidation):
		return tasks.AssistantTaskMutationResult{}, ErrInvalidProposalStatus
	default:
		return tasks.AssistantTaskMutationResult{}, err
	}
}
