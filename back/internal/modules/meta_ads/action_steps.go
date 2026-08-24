package metaads

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type actionStepName string

const (
	actionStepCampaign actionStepName = "campaign"
	actionStepAdSet    actionStepName = "ad_set"
	actionStepCreative actionStepName = "creative"
	actionStepAd       actionStepName = "ad"
)

type actionStep struct {
	ID               string
	AccountID        string
	ProposalID       string
	Step             actionStepName
	RequestHash      string
	Status           ActionStatus
	ExternalEntityID string
	Result           json.RawMessage
	ErrorCode        string
	ErrorMessage     string
	StartedAt        time.Time
	CompletedAt      *time.Time
	UpdatedAt        time.Time
}

type actionStepRepository interface {
	BeginActionStep(context.Context, string, string, actionStepName, string) (actionStep, bool, error)
	CompleteActionStep(context.Context, string, string, actionStepName, string, ActionExecutionOutcome) (actionStep, error)
	ListActionSteps(context.Context, string, string) ([]actionStep, error)
}

const actionStepColumns = `id::text, account_id::text, proposal_id::text, step,
	request_hash, status, external_entity_id, result_snapshot, error_code,
	error_message, started_at, completed_at, updated_at`

func (s *Store) BeginActionStep(
	ctx context.Context,
	accountID, proposalID string,
	step actionStepName,
	requestHash string,
) (actionStep, bool, error) {
	const insert = `insert into meta_ads.action_proposal_steps
		(account_id, proposal_id, step, request_hash, status)
	values ($1::uuid, $2::uuid, $3, $4, 'executing')
	on conflict (account_id, proposal_id, step) do nothing
	returning ` + actionStepColumns
	row, err := scanActionStep(s.pool.QueryRow(ctx, insert, accountID, proposalID, step, requestHash))
	if err == nil {
		return row, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return actionStep{}, false, err
	}
	row, err = scanActionStep(s.pool.QueryRow(ctx, `select `+actionStepColumns+`
		from meta_ads.action_proposal_steps
		where account_id = $1::uuid and proposal_id = $2::uuid and step = $3`,
		accountID, proposalID, step))
	if err != nil {
		return actionStep{}, false, err
	}
	if row.RequestHash != requestHash {
		return actionStep{}, false, ErrActionIdempotencyConflict
	}
	if row.Status != ActionSucceeded {
		return row, false, ErrActionStepUncertain
	}
	return row, false, nil
}

func (s *Store) CompleteActionStep(
	ctx context.Context,
	accountID, proposalID string,
	step actionStepName,
	requestHash string,
	outcome ActionExecutionOutcome,
) (actionStep, error) {
	outcome = normalizeActionOutcome(outcome)
	if outcome.Status != ActionSucceeded && outcome.Status != ActionFailed && outcome.Status != ActionUnknown {
		return actionStep{}, ErrActionValidation
	}
	const update = `update meta_ads.action_proposal_steps
		set status = $5, external_entity_id = $6, result_snapshot = $7,
		    error_code = $8, error_message = $9, completed_at = now(), updated_at = now()
		where account_id = $1::uuid and proposal_id = $2::uuid and step = $3
		  and request_hash = $4 and status = 'executing'
		returning ` + actionStepColumns
	return scanActionStep(s.pool.QueryRow(ctx, update,
		accountID, proposalID, step, requestHash, outcome.Status,
		outcome.ExternalEntityID, outcome.Result, outcome.ErrorCode, outcome.ErrorMessage,
	))
}

func (s *Store) ListActionSteps(
	ctx context.Context,
	accountID, proposalID string,
) ([]actionStep, error) {
	rows, err := s.pool.Query(ctx, `select `+actionStepColumns+`
		from meta_ads.action_proposal_steps
		where account_id = $1::uuid and proposal_id = $2::uuid
		order by started_at, id`, accountID, proposalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]actionStep, 0, 4)
	for rows.Next() {
		step, scanErr := scanActionStep(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, step)
	}
	return out, rows.Err()
}

func scanActionStep(row rowScanner) (actionStep, error) {
	var step actionStep
	err := row.Scan(
		&step.ID, &step.AccountID, &step.ProposalID, &step.Step,
		&step.RequestHash, &step.Status, &step.ExternalEntityID, &step.Result,
		&step.ErrorCode, &step.ErrorMessage, &step.StartedAt, &step.CompletedAt, &step.UpdatedAt,
	)
	if len(step.Result) == 0 {
		step.Result = json.RawMessage(`{}`)
	}
	step.ExternalEntityID = strings.TrimSpace(step.ExternalEntityID)
	return step, err
}
