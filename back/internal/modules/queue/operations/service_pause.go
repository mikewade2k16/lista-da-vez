package operations

import (
	"context"
	"strings"
)

func (service *Service) Pause(ctx context.Context, access AccessContext, input PauseCommandInput) (MutationAck, error) {
	return service.pauseLike(ctx, access, input, "pause", pauseKindPause, false)
}

func (service *Service) AssignTask(ctx context.Context, access AccessContext, input AssignTaskCommandInput) (MutationAck, error) {
	return service.pauseLike(ctx, access, PauseCommandInput(input), "assign-task", pauseKindTask, true)
}

func (service *Service) pauseLike(
	ctx context.Context,
	access AccessContext,
	input PauseCommandInput,
	action string,
	kind string,
	rejectIfInService bool,
) (MutationAck, error) {
	resolvedStoreID, _, roster, snapshotState, err := service.loadSnapshot(ctx, access, input.StoreID)
	if err != nil {
		return MutationAck{}, err
	}

	personID := strings.TrimSpace(input.PersonID)
	reason := strings.TrimSpace(input.Reason)
	if reason == "" {
		return MutationAck{}, ErrValidation
	}

	if _, ok := mapRosterByID(roster)[personID]; !ok {
		return MutationAck{}, ErrConsultantNotFound
	}

	if isInService(snapshotState.ActiveServices, personID) {
		if rejectIfInService {
			return MutationAck{}, ErrConsultantBusy
		}

		return service.buildAck(resolvedStoreID, action, personID), nil
	}

	if isPaused(snapshotState.PausedEmployees, personID) {
		return service.buildAck(resolvedStoreID, action, personID), nil
	}

	now := nowUnixMilli()
	snapshotState.WaitingList = filterWaiting(snapshotState.WaitingList, personID)
	snapshotState.PausedEmployees = append(snapshotState.PausedEmployees, PausedStateItem{
		ConsultantID: personID,
		Reason:       reason,
		Kind:         normalizePauseKind(kind),
		StartedAt:    now,
	})
	snapshotState.ConsultantActivitySessions, snapshotState.ConsultantCurrentStatus = applyStatusTransitions(
		snapshotState.ConsultantActivitySessions,
		snapshotState.ConsultantCurrentStatus,
		[]transition{{personID: personID, nextStatus: statusPaused}},
		now,
	)

	return service.persistAndAck(ctx, resolvedStoreID, action, personID, snapshotState, nil, nil)
}
