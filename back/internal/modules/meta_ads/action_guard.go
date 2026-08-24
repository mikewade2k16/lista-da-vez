package metaads

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const actionGuardSnapshotVersion = 1

type actionGuardSnapshot struct {
	ConnectionID       string
	ConnectionRevision string
	AdAccount          AdAccount
	Policy             *ActionPolicy
	Campaign           *Campaign
	AdAccountHash      string
	PolicyHash         string
	CampaignHash       string
	Hash               string
}

type actionAdAccountGuardDocument struct {
	ID              string `json:"id"`
	MetaAdAccountID string `json:"metaAdAccountId"`
	ClientAccountID string `json:"clientAccountId"`
	Name            string `json:"name"`
	Currency        string `json:"currency"`
	Status          string `json:"status"`
	IsCurrent       bool   `json:"isCurrent"`
	UpdatedAt       string `json:"updatedAt"`
}

type actionPolicyGuardDocument struct {
	Configured             bool   `json:"configured"`
	ID                     string `json:"id"`
	UpdatedAt              string `json:"updatedAt"`
	Currency               string `json:"currency"`
	MaxDailyBudgetMinor    *int64 `json:"maxDailyBudgetMinor"`
	MaxLifetimeBudgetMinor *int64 `json:"maxLifetimeBudgetMinor"`
	AllowCreate            bool   `json:"allowCreate"`
	AllowDuplicate         bool   `json:"allowDuplicate"`
	AllowResume            bool   `json:"allowResume"`
}

type actionCampaignGuardDocument struct {
	ID                  string `json:"id"`
	MetaCampaignID      string `json:"metaCampaignId"`
	Name                string `json:"name"`
	Objective           string `json:"objective"`
	Status              string `json:"status"`
	DailyBudgetMinor    *int64 `json:"dailyBudgetMinor"`
	LifetimeBudgetMinor *int64 `json:"lifetimeBudgetMinor"`
	IsCurrent           bool   `json:"isCurrent"`
	SyncedAt            string `json:"syncedAt"`
}

type actionGuardDocument struct {
	Version            int    `json:"version"`
	ViewerAccountID    string `json:"viewerAccountId"`
	ResourceAccountID  string `json:"resourceAccountId"`
	RequestHash        string `json:"requestHash"`
	ConnectionID       string `json:"connectionId"`
	ConnectionRevision string `json:"connectionRevision"`
	AdAccountHash      string `json:"adAccountHash"`
	PolicyHash         string `json:"policyHash"`
	CampaignHash       string `json:"campaignHash"`
}

func captureActionGuardSnapshot(
	ctx context.Context,
	tx pgx.Tx,
	viewerAccountID, resourceAccountID, adAccountID, targetCampaignID, requestHash string,
) (actionGuardSnapshot, error) {
	const adAccountQuery = `select connection.id::text, connection.revision::text,
		aa.id::text, aa.account_id::text, aa.connection_id::text,
		aa.meta_ad_account_id, aa.client_account_id::text, aa.name, aa.currency,
		aa.status, aa.is_current, aa.created_at, aa.updated_at
		from meta_ads.ad_accounts aa
		join meta_ads.connections connection
		  on connection.id = aa.connection_id and connection.account_id = aa.account_id
		where aa.account_id = $1::uuid and aa.id = $2::uuid
		  and connection.status = 'active'
		  and (connection.token_expires_at is null or connection.token_expires_at > now())
		  and aa.is_current
		  and ($3::uuid = aa.account_id or aa.client_account_id = $3::uuid)
		for update of connection, aa`
	var snapshot actionGuardSnapshot
	err := tx.QueryRow(ctx, adAccountQuery,
		resourceAccountID, adAccountID, viewerAccountID,
	).Scan(
		&snapshot.ConnectionID, &snapshot.ConnectionRevision,
		&snapshot.AdAccount.ID, &snapshot.AdAccount.AccountID,
		&snapshot.AdAccount.ConnectionID, &snapshot.AdAccount.MetaAdAccountID,
		&snapshot.AdAccount.ClientAccountID, &snapshot.AdAccount.Name,
		&snapshot.AdAccount.Currency, &snapshot.AdAccount.Status,
		&snapshot.AdAccount.IsCurrent, &snapshot.AdAccount.CreatedAt,
		&snapshot.AdAccount.UpdatedAt,
	)
	if err != nil {
		return actionGuardSnapshot{}, err
	}

	policy, err := scanActionPolicy(tx.QueryRow(ctx, `select id::text, account_id::text,
		ad_account_id::text, currency, max_daily_budget, max_lifetime_budget,
		allow_create, allow_duplicate, allow_resume, updated_by_user_id::text,
		created_at, updated_at
		from meta_ads.action_policies
		where account_id = $1::uuid and ad_account_id = $2::uuid
		for update`, resourceAccountID, adAccountID))
	if err == nil {
		snapshot.Policy = &policy
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return actionGuardSnapshot{}, err
	}

	if targetCampaignID != "" {
		campaign, campaignErr := scanCampaign(tx.QueryRow(ctx, `select campaign.id,
			campaign.account_id, campaign.ad_account_id, campaign.meta_campaign_id,
			campaign.name, campaign.objective, campaign.status, campaign.daily_budget,
			campaign.lifetime_budget, campaign.is_current, campaign.synced_at
			from meta_ads.campaigns campaign
			where campaign.account_id = $1::uuid and campaign.ad_account_id = $2::uuid
			  and campaign.id = $3::uuid and campaign.is_current
			for update`, resourceAccountID, adAccountID, targetCampaignID))
		if campaignErr != nil {
			return actionGuardSnapshot{}, campaignErr
		}
		snapshot.Campaign = &campaign
	}

	if err := snapshot.calculateHashes(viewerAccountID, resourceAccountID, requestHash); err != nil {
		return actionGuardSnapshot{}, err
	}
	return snapshot, nil
}

func (snapshot *actionGuardSnapshot) calculateHashes(
	viewerAccountID, resourceAccountID, requestHash string,
) error {
	adAccountDocument := actionAdAccountGuardDocument{
		ID: snapshot.AdAccount.ID, MetaAdAccountID: snapshot.AdAccount.MetaAdAccountID,
		ClientAccountID: optionalActionString(snapshot.AdAccount.ClientAccountID),
		Name:            strings.TrimSpace(snapshot.AdAccount.Name),
		Currency:        strings.ToUpper(strings.TrimSpace(snapshot.AdAccount.Currency)),
		Status:          strings.ToUpper(strings.TrimSpace(snapshot.AdAccount.Status)),
		IsCurrent:       snapshot.AdAccount.IsCurrent,
		UpdatedAt:       snapshot.AdAccount.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	policyDocument := actionPolicyGuardDocument{}
	if snapshot.Policy != nil {
		policyDocument = actionPolicyGuardDocument{
			Configured: true, ID: snapshot.Policy.ID,
			UpdatedAt:              snapshot.Policy.UpdatedAt.UTC().Format(time.RFC3339Nano),
			Currency:               strings.ToUpper(strings.TrimSpace(snapshot.Policy.Currency)),
			MaxDailyBudgetMinor:    actionMinorPointer(snapshot.Policy.MaxDailyBudget),
			MaxLifetimeBudgetMinor: actionMinorPointer(snapshot.Policy.MaxLifetimeBudget),
			AllowCreate:            snapshot.Policy.AllowCreate,
			AllowDuplicate:         snapshot.Policy.AllowDuplicate,
			AllowResume:            snapshot.Policy.AllowResume,
		}
	}
	var campaignDocument *actionCampaignGuardDocument
	if snapshot.Campaign != nil {
		campaignDocument = &actionCampaignGuardDocument{
			ID: snapshot.Campaign.ID, MetaCampaignID: snapshot.Campaign.MetaCampaignID,
			Name:                strings.TrimSpace(snapshot.Campaign.Name),
			Objective:           strings.ToUpper(strings.TrimSpace(snapshot.Campaign.Objective)),
			Status:              strings.ToUpper(strings.TrimSpace(snapshot.Campaign.Status)),
			DailyBudgetMinor:    actionMinorPointer(snapshot.Campaign.DailyBudget),
			LifetimeBudgetMinor: actionMinorPointer(snapshot.Campaign.LifetimeBudget),
			IsCurrent:           snapshot.Campaign.IsCurrent,
			SyncedAt:            snapshot.Campaign.SyncedAt.UTC().Format(time.RFC3339Nano),
		}
	}
	var err error
	snapshot.AdAccountHash, err = hashActionGuardDocument(adAccountDocument)
	if err != nil {
		return err
	}
	snapshot.PolicyHash, err = hashActionGuardDocument(policyDocument)
	if err != nil {
		return err
	}
	snapshot.CampaignHash, err = hashActionGuardDocument(campaignDocument)
	if err != nil {
		return err
	}
	snapshot.Hash, err = hashActionGuardDocument(actionGuardDocument{
		Version:         actionGuardSnapshotVersion,
		ViewerAccountID: viewerAccountID, ResourceAccountID: resourceAccountID,
		RequestHash: requestHash, ConnectionID: snapshot.ConnectionID,
		ConnectionRevision: snapshot.ConnectionRevision,
		AdAccountHash:      snapshot.AdAccountHash, PolicyHash: snapshot.PolicyHash,
		CampaignHash: snapshot.CampaignHash,
	})
	return err
}

func hashActionGuardDocument(value any) (string, error) {
	canonical, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:]), nil
}

func actionMinorPointer(value *float64) *int64 {
	if value == nil {
		return nil
	}
	minor := budgetMinorUnits(*value)
	return &minor
}

func optionalActionString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func actionGuardMatchesProposal(snapshot actionGuardSnapshot, proposal ActionProposal) bool {
	if proposal.GuardSnapshotVersion != actionGuardSnapshotVersion ||
		proposal.ConnectionIDSnapshot == nil || proposal.ConnectionRevisionSnapshot == nil ||
		proposal.GuardSnapshotHash == "" {
		return false
	}
	return snapshot.Hash == proposal.GuardSnapshotHash &&
		snapshot.ConnectionID == *proposal.ConnectionIDSnapshot &&
		snapshot.ConnectionRevision == *proposal.ConnectionRevisionSnapshot &&
		snapshot.AdAccountHash == proposal.AdAccountHashSnapshot &&
		snapshot.PolicyHash == proposal.PolicyHashSnapshot &&
		snapshot.CampaignHash == proposal.CampaignHashSnapshot
}
