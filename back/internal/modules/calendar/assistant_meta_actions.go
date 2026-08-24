package calendar

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

const (
	maxMetaActionProposals = 20
	maxMetaActionNameRunes = 240
	maxMetaActionBudget    = 999_999_999.99
)

var ErrMetaActionNotSucceeded = errors.New("calendar: meta action has not succeeded")

type MetaAssistantActionAdAccount struct {
	ID              string
	Name            string
	Currency        string
	ClientAccountID string
}

type MetaAssistantActionInstagramPost struct {
	ID              string
	Title           string
	IGUserID        string
	PageID          string
	ClientAccountID string
}

type MetaAssistantActionCampaign struct {
	ID             string
	AdAccountID    string
	Name           string
	DailyBudget    *float64
	LifetimeBudget *float64
}

// MetaAssistantActionRequest e o comando interno entregue ao owner Meta Ads.
// Account/user nunca saem do Principal; AllowedAdAccountIDs vem do mesmo contexto
// autoritativo e tenant-scoped usado para responder ao modelo.
type MetaAssistantActionRequest struct {
	AccountID           string
	ActorUserID         string
	ConversationID      string
	MessageID           string
	ProposalIndex       int
	AllowedAdAccountIDs []string
	Action              string
	AdAccountID         string
	Payload             json.RawMessage
}

type MetaAssistantActionResult struct {
	ID                           string
	Action                       string
	AdAccountID                  string
	AdAccountName                string
	Currency                     string
	TargetCampaignID             string
	Summary                      string
	Status                       string
	ExecutionAvailable           bool
	CanConfirm                   bool
	RequiresSpendAcknowledgement bool
	ExpiresAt                    string
	ErrorCode                    string
	ErrorMessage                 string
}

type MetaAssistantActionProvider func(
	ctx context.Context,
	req MetaAssistantActionRequest,
) (MetaAssistantActionResult, error)

type MetaAssistantActionStatusProvider func(
	ctx context.Context,
	accountID, proposalID string,
) (MetaAssistantActionResult, error)

type MetaAssistantActionLifecycleRequest struct {
	AccountID        string
	ActorUserID      string
	ConversationID   string
	MessageID        string
	ActionProposalID string
	IdempotencyKey   string
	AcknowledgeSpend bool
}

type MetaAssistantActionBindProvider func(
	context.Context, MetaAssistantActionLifecycleRequest,
) (MetaAssistantActionResult, error)

type MetaAssistantActionConfirmProvider func(
	context.Context, MetaAssistantActionLifecycleRequest,
) (MetaAssistantActionResult, error)

type MetaAssistantActionCancelProvider func(
	context.Context, MetaAssistantActionLifecycleRequest,
) (MetaAssistantActionResult, error)

type MetaAssistantConversationCancelProvider func(
	context.Context, string, string, string,
) error

type ChatProposalMetaBudget struct {
	Type   string  `json:"type"`
	Amount float64 `json:"amount"`
}

// ChatProposalMetaAction e o shape fechado persistido em fields.metaAction.
// Os campos de entrada sao normalizados antes do provider; nomes/snapshots e o
// actionProposalId sao preenchidos somente com a resposta autoritativa do Meta.
type ChatProposalMetaAction struct {
	Action                       string                  `json:"action"`
	AdAccountID                  string                  `json:"adAccountId"`
	AdAccountName                string                  `json:"adAccountName,omitempty"`
	CampaignID                   string                  `json:"campaignId,omitempty"`
	CampaignName                 string                  `json:"campaignName,omitempty"`
	Currency                     string                  `json:"currency,omitempty"`
	Name                         string                  `json:"name,omitempty"`
	Objective                    string                  `json:"objective,omitempty"`
	SpecialAdCategories          []string                `json:"specialAdCategories,omitempty"`
	Budget                       *ChatProposalMetaBudget `json:"budget,omitempty"`
	InstagramPostID              string                  `json:"instagramPostId,omitempty"`
	InstagramPostTitle           string                  `json:"instagramPostTitle,omitempty"`
	AdSetName                    string                  `json:"adSetName,omitempty"`
	AdName                       string                  `json:"adName,omitempty"`
	Countries                    []string                `json:"countries,omitempty"`
	AgeMin                       int                     `json:"ageMin,omitempty"`
	AgeMax                       int                     `json:"ageMax,omitempty"`
	IGUserID                     string                  `json:"-"`
	PageID                       string                  `json:"-"`
	InstagramClientAccountID     string                  `json:"-"`
	ActionProposalID             string                  `json:"actionProposalId,omitempty"`
	Summary                      string                  `json:"summary,omitempty"`
	ActionStatus                 string                  `json:"actionStatus,omitempty"`
	ExecutionAvailable           bool                    `json:"executionAvailable"`
	CanConfirm                   bool                    `json:"canConfirm"`
	RequiresSpendAcknowledgement bool                    `json:"requiresSpendAcknowledgement"`
	ExpiresAt                    string                  `json:"expiresAt,omitempty"`
	ErrorCode                    string                  `json:"errorCode,omitempty"`
	ErrorMessage                 string                  `json:"errorMessage,omitempty"`
}

type metaCreateCampaignPayload struct {
	Name                string                  `json:"name"`
	Objective           string                  `json:"objective"`
	SpecialAdCategories []string                `json:"specialAdCategories"`
	Budget              *ChatProposalMetaBudget `json:"budget,omitempty"`
	Status              string                  `json:"status"`
}

type metaDuplicateCampaignPayload struct {
	CampaignID string `json:"campaignId"`
	Name       string `json:"name"`
	Status     string `json:"status"`
}

type metaUpdateCampaignPayload struct {
	CampaignID string                  `json:"campaignId"`
	Name       string                  `json:"name,omitempty"`
	Budget     *ChatProposalMetaBudget `json:"budget,omitempty"`
}

type metaTargetCampaignPayload struct {
	CampaignID string `json:"campaignId"`
}

type metaPromoteInstagramPostPayload struct {
	Name            string                  `json:"name"`
	InstagramPostID string                  `json:"instagramPostId"`
	IGUserID        string                  `json:"igUserId"`
	PageID          string                  `json:"pageId"`
	ClientAccountID string                  `json:"clientAccountId,omitempty"`
	AdSetName       string                  `json:"adSetName"`
	AdName          string                  `json:"adName"`
	Budget          *ChatProposalMetaBudget `json:"budget"`
	Countries       []string                `json:"countries"`
	AgeMin          int                     `json:"ageMin"`
	AgeMax          int                     `json:"ageMax"`
	Status          string                  `json:"status"`
}

func sanitizeMetaActionIntent(value *ChatProposalMetaAction) bool {
	if value == nil {
		return false
	}
	value.Action = strings.ToLower(strings.TrimSpace(value.Action))
	value.AdAccountID = trimMetaResourcePrefix(value.AdAccountID, "meta_ad_account:")
	value.CampaignID = trimMetaResourcePrefix(value.CampaignID, "meta_campaign:")
	value.Name = normalizeMetaActionName(value.Name)
	value.InstagramPostID = trimMetaResourcePrefix(value.InstagramPostID, "instagram_post:")
	value.InstagramPostTitle = ""
	value.AdSetName = normalizeMetaActionName(value.AdSetName)
	value.AdName = normalizeMetaActionName(value.AdName)
	value.IGUserID = ""
	value.PageID = ""
	value.InstagramClientAccountID = ""
	value.Objective = strings.ToUpper(strings.TrimSpace(value.Objective))
	value.ActionProposalID = ""
	value.AdAccountName = ""
	value.CampaignName = ""
	value.Currency = ""
	value.Summary = ""
	value.ActionStatus = ""
	value.ExecutionAvailable = false
	value.CanConfirm = false
	value.RequiresSpendAcknowledgement = false
	value.ExpiresAt = ""
	value.ErrorCode = ""
	value.ErrorMessage = ""
	if !sanitizeMetaBudget(value.Budget) {
		return false
	}
	switch value.Action {
	case "create_campaign":
		categories, ok := sanitizeMetaSpecialCategories(value.SpecialAdCategories)
		if !ok || !uuidRe.MatchString(value.AdAccountID) || value.Name == "" ||
			!validMetaCampaignObjective(value.Objective) {
			return false
		}
		value.SpecialAdCategories = categories
		value.CampaignID = ""
	case "duplicate_campaign":
		if !uuidRe.MatchString(value.CampaignID) || value.Name == "" {
			return false
		}
		value.Objective = ""
		value.SpecialAdCategories = nil
		value.Budget = nil
	case "update_campaign":
		if !uuidRe.MatchString(value.CampaignID) || (value.Name == "" && value.Budget == nil) {
			return false
		}
		value.Objective = ""
		value.SpecialAdCategories = nil
	case "pause_campaign", "resume_campaign":
		if !uuidRe.MatchString(value.CampaignID) {
			return false
		}
		value.Name = ""
		value.Objective = ""
		value.SpecialAdCategories = nil
		value.Budget = nil
		value.InstagramPostID = ""
		value.AdSetName = ""
		value.AdName = ""
		value.Countries = nil
		value.AgeMin = 0
		value.AgeMax = 0
	case "promote_instagram_post":
		if !uuidRe.MatchString(value.AdAccountID) || value.Name == "" ||
			!assistantResourceSuffixRe.MatchString(value.InstagramPostID) || value.Budget == nil ||
			value.Budget.Type != "daily" {
			return false
		}
		if value.AdSetName == "" {
			value.AdSetName = normalizeMetaActionName(value.Name + " - conjunto")
		}
		if value.AdName == "" {
			value.AdName = normalizeMetaActionName(value.Name + " - anuncio")
		}
		if len(value.Countries) == 0 {
			value.Countries = []string{"BR"}
		}
		if !sanitizeMetaCountries(&value.Countries) {
			return false
		}
		if value.AgeMin == 0 {
			value.AgeMin = 18
		}
		if value.AgeMax == 0 {
			value.AgeMax = 65
		}
		if value.AgeMin < 18 || value.AgeMax > 65 || value.AgeMin > value.AgeMax {
			return false
		}
		value.CampaignID = ""
		value.Objective = ""
		value.SpecialAdCategories = nil
	default:
		return false
	}
	return true
}

func sanitizeMetaCountries(values *[]string) bool {
	if values == nil || len(*values) == 0 || len(*values) > 10 {
		return false
	}
	out := make([]string, 0, len(*values))
	seen := make(map[string]bool, len(*values))
	for _, raw := range *values {
		value := strings.ToUpper(strings.TrimSpace(raw))
		if len(value) != 2 || value[0] < 'A' || value[0] > 'Z' || value[1] < 'A' || value[1] > 'Z' {
			return false
		}
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	*values = out
	return true
}

func trimMetaResourcePrefix(value, prefix string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, prefix) {
		return strings.TrimPrefix(value, prefix)
	}
	return value
}

func normalizeMetaActionName(value string) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if value == "" || !utf8.ValidString(value) || utf8.RuneCountInString(value) > maxMetaActionNameRunes {
		return ""
	}
	return value
}

func sanitizeMetaBudget(value *ChatProposalMetaBudget) bool {
	if value == nil {
		return true
	}
	value.Type = strings.ToLower(strings.TrimSpace(value.Type))
	if value.Type != "daily" && value.Type != "lifetime" {
		return false
	}
	if math.IsNaN(value.Amount) || math.IsInf(value.Amount, 0) || value.Amount <= 0 || value.Amount > maxMetaActionBudget {
		return false
	}
	minor := math.Round(value.Amount * 100)
	if math.Abs(value.Amount*100-minor) > 1e-7 {
		return false
	}
	value.Amount = minor / 100
	return true
}

func sanitizeMetaSpecialCategories(values []string) ([]string, bool) {
	if len(values) > 6 {
		return nil, false
	}
	allowed := map[string]bool{
		"CREDIT": true, "EMPLOYMENT": true, "HOUSING": true,
		"ISSUES_ELECTIONS_POLITICS": true, "ONLINE_GAMBLING_AND_GAMING": true, "NONE": true,
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, raw := range values {
		value := strings.ToUpper(strings.TrimSpace(raw))
		if !allowed[value] {
			return nil, false
		}
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	if seen["NONE"] && len(out) != 1 {
		return nil, false
	}
	return out, true
}

func validMetaCampaignObjective(value string) bool {
	switch value {
	case "OUTCOME_APP_PROMOTION", "OUTCOME_AWARENESS", "OUTCOME_ENGAGEMENT",
		"OUTCOME_LEADS", "OUTCOME_SALES", "OUTCOME_TRAFFIC":
		return true
	default:
		return false
	}
}

func prepareMetaActionProposals(
	ctx context.Context,
	accountID string,
	principal auth.Principal,
	conversationID, messageID string,
	proposals []ChatProposal,
	metaContext MetaAssistantContextResult,
	provider MetaAssistantActionProvider,
) ([]ChatProposal, int, error) {
	adAccounts := make(map[string]MetaAssistantActionAdAccount, len(metaContext.ActionAdAccounts))
	allowedAdAccountIDs := make([]string, 0, len(metaContext.ActionAdAccounts))
	for _, account := range metaContext.ActionAdAccounts {
		account.ID = strings.TrimSpace(account.ID)
		if !uuidRe.MatchString(account.ID) {
			continue
		}
		account.Name = assistantResourceText(account.Name, 300)
		account.Currency = strings.ToUpper(strings.TrimSpace(account.Currency))
		if len(account.Currency) != 3 {
			continue
		}
		adAccounts[account.ID] = account
		allowedAdAccountIDs = append(allowedAdAccountIDs, account.ID)
	}
	campaigns := make(map[string]MetaAssistantActionCampaign, len(metaContext.ActionCampaigns))
	for _, campaign := range metaContext.ActionCampaigns {
		campaign.ID = strings.TrimSpace(campaign.ID)
		campaign.AdAccountID = strings.TrimSpace(campaign.AdAccountID)
		if !uuidRe.MatchString(campaign.ID) || adAccounts[campaign.AdAccountID].ID == "" {
			continue
		}
		campaign.Name = assistantResourceText(campaign.Name, 300)
		campaigns[campaign.ID] = campaign
	}
	instagramPosts := make(map[string]MetaAssistantActionInstagramPost, len(metaContext.ActionInstagramPosts))
	for _, post := range metaContext.ActionInstagramPosts {
		post.ID = trimMetaResourcePrefix(post.ID, "instagram_post:")
		post.IGUserID = strings.TrimSpace(post.IGUserID)
		post.PageID = strings.TrimSpace(post.PageID)
		post.ClientAccountID = strings.TrimSpace(post.ClientAccountID)
		post.Title = assistantResourceText(post.Title, 180)
		if !assistantResourceSuffixRe.MatchString(post.ID) ||
			!assistantResourceSuffixRe.MatchString(post.IGUserID) ||
			!assistantResourceSuffixRe.MatchString(post.PageID) {
			continue
		}
		instagramPosts[post.ID] = post
	}
	out := make([]ChatProposal, 0, len(proposals))
	seenMetaRequests := make(map[string]struct{}, len(proposals))
	dropped := 0
	metaIndex := 0
	for _, proposal := range proposals {
		if proposal.Kind != "metaAction" {
			out = append(out, proposal)
			continue
		}
		if metaIndex >= maxMetaActionProposals || provider == nil {
			dropped++
			continue
		}
		intent := proposal.Fields.MetaAction
		adAccount, campaign, ok := resolveMetaActionTargets(intent, adAccounts, campaigns)
		if !ok {
			dropped++
			continue
		}
		if intent.Action == "promote_instagram_post" {
			post, exists := instagramPosts[intent.InstagramPostID]
			if !exists {
				dropped++
				continue
			}
			intent.InstagramPostID = post.ID
			intent.InstagramPostTitle = post.Title
			intent.IGUserID = post.IGUserID
			intent.PageID = post.PageID
			intent.InstagramClientAccountID = post.ClientAccountID
		}
		payload, err := metaActionPayload(intent)
		if err != nil {
			dropped++
			continue
		}
		requestHash := metaAssistantActionRequestHash(intent.Action, adAccount.ID, payload)
		if _, duplicate := seenMetaRequests[requestHash]; duplicate {
			dropped++
			continue
		}
		seenMetaRequests[requestHash] = struct{}{}
		intent.AdAccountID = adAccount.ID
		intent.AdAccountName = adAccount.Name
		intent.Currency = adAccount.Currency
		if campaign.ID != "" {
			intent.CampaignID = campaign.ID
			intent.CampaignName = campaign.Name
		}
		result, err := provider(ctx, MetaAssistantActionRequest{
			AccountID: accountID, ActorUserID: principal.UserID,
			ConversationID: conversationID, MessageID: messageID, ProposalIndex: metaIndex,
			AllowedAdAccountIDs: append([]string(nil), allowedAdAccountIDs...),
			Action:              intent.Action, AdAccountID: adAccount.ID, Payload: payload,
		})
		if err != nil {
			return nil, dropped, err
		}
		applyMetaActionResult(intent, result)
		proposal.Action = "create"
		proposal.Fields.MetaAction = intent
		out = append(out, proposal)
		metaIndex++
	}
	return out, dropped, nil
}

func metaAssistantActionRequestHash(action, adAccountID string, payload json.RawMessage) string {
	canonical := fmt.Sprintf("%s\n%s\n%s", action, strings.TrimSpace(adAccountID), payload)
	digest := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(digest[:])
}

func resolveMetaActionTargets(
	intent *ChatProposalMetaAction,
	adAccounts map[string]MetaAssistantActionAdAccount,
	campaigns map[string]MetaAssistantActionCampaign,
) (MetaAssistantActionAdAccount, MetaAssistantActionCampaign, bool) {
	if intent == nil {
		return MetaAssistantActionAdAccount{}, MetaAssistantActionCampaign{}, false
	}
	if intent.Action == "create_campaign" || intent.Action == "promote_instagram_post" {
		account, ok := adAccounts[intent.AdAccountID]
		return account, MetaAssistantActionCampaign{}, ok
	}
	campaign, ok := campaigns[intent.CampaignID]
	if !ok {
		return MetaAssistantActionAdAccount{}, MetaAssistantActionCampaign{}, false
	}
	account, ok := adAccounts[campaign.AdAccountID]
	return account, campaign, ok
}

func metaActionPayload(intent *ChatProposalMetaAction) (json.RawMessage, error) {
	var value any
	switch intent.Action {
	case "create_campaign":
		value = metaCreateCampaignPayload{
			Name: intent.Name, Objective: intent.Objective,
			SpecialAdCategories: append([]string(nil), intent.SpecialAdCategories...),
			Budget:              intent.Budget, Status: "PAUSED",
		}
	case "duplicate_campaign":
		value = metaDuplicateCampaignPayload{CampaignID: intent.CampaignID, Name: intent.Name, Status: "PAUSED"}
	case "update_campaign":
		value = metaUpdateCampaignPayload{CampaignID: intent.CampaignID, Name: intent.Name, Budget: intent.Budget}
	case "pause_campaign", "resume_campaign":
		value = metaTargetCampaignPayload{CampaignID: intent.CampaignID}
	case "promote_instagram_post":
		value = metaPromoteInstagramPostPayload{
			Name: intent.Name, InstagramPostID: intent.InstagramPostID,
			IGUserID: intent.IGUserID, PageID: intent.PageID,
			ClientAccountID: intent.InstagramClientAccountID,
			AdSetName:       intent.AdSetName, AdName: intent.AdName, Budget: intent.Budget,
			Countries: append([]string(nil), intent.Countries...),
			AgeMin:    intent.AgeMin, AgeMax: intent.AgeMax, Status: "PAUSED",
		}
	default:
		return nil, ErrInvalidProposalStatus
	}
	return json.Marshal(value)
}

func applyMetaActionResult(intent *ChatProposalMetaAction, result MetaAssistantActionResult) {
	intent.ActionProposalID = strings.TrimSpace(result.ID)
	if !uuidRe.MatchString(intent.ActionProposalID) {
		intent.ActionProposalID = ""
	}
	intent.Summary = assistantResourceText(result.Summary, 1000)
	intent.ActionStatus = normalizeMetaActionStatus(result.Status)
	intent.ExecutionAvailable = result.ExecutionAvailable
	intent.CanConfirm = result.CanConfirm && result.ExecutionAvailable && intent.ActionStatus == "pending"
	intent.RequiresSpendAcknowledgement = result.RequiresSpendAcknowledgement
	intent.ExpiresAt = strings.TrimSpace(result.ExpiresAt)
	intent.ErrorCode = assistantResourceText(result.ErrorCode, 100)
	intent.ErrorMessage = assistantResourceText(result.ErrorMessage, 500)
	if intent.ActionStatus == "" {
		intent.ActionStatus = "pending"
	}
	if !intent.ExecutionAvailable && intent.ErrorMessage == "" {
		intent.ErrorCode = "action_unavailable"
		intent.ErrorMessage = "Esta acao ainda nao possui executor Graph seguro ou as escritas estao desabilitadas."
	}
	if intent.Summary == "" {
		intent.Summary = fallbackMetaActionSummary(intent)
	}
}

func bindPersistedMetaActionProposals(
	ctx context.Context,
	accountID string,
	principal auth.Principal,
	conversationID, messageID string,
	proposals []StoredProposal,
	bindProvider MetaAssistantActionBindProvider,
	cancelProvider MetaAssistantActionCancelProvider,
) ([]StoredProposal, error) {
	bound := append([]StoredProposal(nil), proposals...)
	metaIndexes := make([]int, 0)
	for index := range bound {
		meta := bound[index].Fields.MetaAction
		if bound[index].Kind != "metaAction" || meta == nil ||
			!uuidRe.MatchString(strings.TrimSpace(meta.ActionProposalID)) {
			continue
		}
		metaIndexes = append(metaIndexes, index)
	}
	if len(metaIndexes) == 0 {
		return bound, nil
	}
	if bindProvider == nil {
		return bound, ErrMetaActionNotSucceeded
	}
	for _, index := range metaIndexes {
		meta := bound[index].Fields.MetaAction
		result, err := bindProvider(ctx, MetaAssistantActionLifecycleRequest{
			AccountID: accountID, ActorUserID: principal.UserID,
			ConversationID: conversationID, MessageID: messageID,
			ActionProposalID: strings.TrimSpace(meta.ActionProposalID),
		})
		if err != nil {
			persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			cancelPersistedMetaActionsAfterBindFailure(
				persistCtx, accountID, principal, conversationID, messageID,
				bound, cancelProvider,
			)
			cancel()
			return bound, err
		}
		applyMetaActionResult(meta, result)
	}
	return bound, nil
}

func hydrateMetaActionProposals(
	ctx context.Context,
	accountID string,
	messages []ChatMessage,
	provider MetaAssistantActionStatusProvider,
) ([]ChatMessage, error) {
	hydrated := append([]ChatMessage(nil), messages...)
	cache := make(map[string]MetaAssistantActionResult)
	for messageIndex := range hydrated {
		proposals := append([]StoredProposal(nil), hydrated[messageIndex].Proposals...)
		hydrated[messageIndex].Proposals = proposals
		for proposalIndex := range proposals {
			proposal := &proposals[proposalIndex]
			meta := proposal.Fields.MetaAction
			if proposal.Status != "pending" || proposal.Kind != "metaAction" || meta == nil {
				continue
			}
			proposalID := strings.TrimSpace(meta.ActionProposalID)
			if !uuidRe.MatchString(proposalID) {
				meta.CanConfirm = false
				continue
			}
			if provider == nil {
				meta.CanConfirm = false
				meta.ErrorCode = "action_status_unavailable"
				meta.ErrorMessage = "Nao foi possivel validar o estado atual desta acao Meta Ads."
				continue
			}
			result, ok := cache[proposalID]
			if !ok {
				var err error
				result, err = provider(ctx, accountID, proposalID)
				if err != nil {
					meta.CanConfirm = false
					meta.ExecutionAvailable = false
					meta.ErrorCode = "action_status_unavailable"
					meta.ErrorMessage = "Nao foi possivel validar o estado atual desta acao Meta Ads. Atualize a conversa para tentar novamente."
					continue
				}
				cache[proposalID] = result
			}
			applyMetaActionResult(meta, result)
		}
	}
	return hydrated, nil
}

func cancelPersistedMetaActionsAfterBindFailure(
	ctx context.Context,
	accountID string,
	principal auth.Principal,
	conversationID, messageID string,
	proposals []StoredProposal,
	cancelProvider MetaAssistantActionCancelProvider,
) {
	if cancelProvider == nil {
		return
	}
	for _, proposal := range proposals {
		meta := proposal.Fields.MetaAction
		if proposal.Kind != "metaAction" || meta == nil ||
			!uuidRe.MatchString(strings.TrimSpace(meta.ActionProposalID)) {
			continue
		}
		_, _ = cancelProvider(ctx, MetaAssistantActionLifecycleRequest{
			AccountID: accountID, ActorUserID: principal.UserID,
			ConversationID: conversationID, MessageID: messageID,
			ActionProposalID: strings.TrimSpace(meta.ActionProposalID),
			IdempotencyKey:   "assistant-bind-failure:" + strings.TrimSpace(meta.ActionProposalID),
		})
	}
}

func normalizeMetaActionStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "pending", "executing", "succeeded", "failed", "unknown", "cancelled", "expired":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func fallbackMetaActionSummary(intent *ChatProposalMetaAction) string {
	switch intent.Action {
	case "create_campaign":
		return fmt.Sprintf("Criar a campanha %q pausada em %s.", intent.Name, intent.AdAccountName)
	case "duplicate_campaign":
		return fmt.Sprintf("Duplicar %q como %q, mantendo a copia pausada.", intent.CampaignName, intent.Name)
	case "update_campaign":
		return fmt.Sprintf("Atualizar a campanha %q.", intent.CampaignName)
	case "pause_campaign":
		return fmt.Sprintf("Pausar a campanha %q.", intent.CampaignName)
	case "resume_campaign":
		return fmt.Sprintf("Ativar a campanha %q e retomar a veiculacao.", intent.CampaignName)
	case "promote_instagram_post":
		return fmt.Sprintf(
			"Promover %q criando campanha, conjunto e anuncio pausados em %s.",
			intent.InstagramPostTitle, intent.AdAccountName,
		)
	default:
		return "Proposta Meta Ads."
	}
}

func newAssistantMessageID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(raw[:])
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" +
		encoded[16:20] + "-" + encoded[20:32], nil
}

func metaActionProposalFromMessage(message ChatMessage, proposalID string) (*StoredProposal, bool) {
	for index := range message.Proposals {
		proposal := &message.Proposals[index]
		if proposal.ID == proposalID {
			return proposal, proposal.Kind == "metaAction" && proposal.Fields.MetaAction != nil
		}
	}
	return nil, false
}

func canManageMetaActions(principal auth.Principal) bool {
	if principal.Role == auth.RoleOwner || principal.Role == auth.RolePlatformAdmin {
		return true
	}
	return principal.PermissionsResolved && containsPermission(principal.Permissions, "meta_ads.manage")
}

// ValidateMetaAssistantActionSource e a porta fail-closed usada pelo owner Meta
// imediatamente antes de bind/confirm. Ela revalida tenant, conversa viva, escopo,
// capability write, RBAC atual e o vinculo exato message/card/proposal.
func (s *Service) ValidateMetaAssistantActionSource(
	ctx context.Context,
	accountID, conversationID, messageID, actionProposalID string,
) error {
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok || strings.TrimSpace(principal.AccountID) != strings.TrimSpace(accountID) ||
		!canManageMetaActions(principal) {
		return ErrForbidden
	}
	access, err := s.resolveChatAccess(ctx, principal, strings.TrimSpace(accountID))
	if err != nil {
		return err
	}
	conv, err := s.authorizeConversation(
		ctx, access, strings.TrimSpace(accountID), strings.TrimSpace(conversationID), principal.UserID,
	)
	if err != nil || !access.canAccessSavedScope(conv.ScopeMode, ptrToStr(conv.ScopeClientID)) {
		return ErrNotFound
	}
	capabilities, err := s.resolveConversationCapabilities(ctx, accountID, conv.EntrySurface, principal)
	if err != nil {
		return err
	}
	if assistantCapabilityMode(capabilities, "meta_ads") != assistantModeWrite {
		return ErrForbidden
	}
	message, err := s.store.GetMessage(
		ctx, strings.TrimSpace(accountID), conv.ID, strings.TrimSpace(messageID),
	)
	if err != nil {
		return mapNotFound(err)
	}
	for _, proposal := range message.Proposals {
		if proposal.Status != "pending" || proposal.Kind != "metaAction" ||
			proposal.Fields.MetaAction == nil {
			continue
		}
		if strings.TrimSpace(proposal.Fields.MetaAction.ActionProposalID) == strings.TrimSpace(actionProposalID) {
			return nil
		}
	}
	return ErrNotFound
}
