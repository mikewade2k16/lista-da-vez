package metaads

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

const (
	maxActionPayloadBytes = 32 << 10
	maxActionNameRunes    = 240
	maxActionBudget       = 999_999_999.99
)

var actionIdempotencyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{7,159}$`)
var metaAdsNumericIDPattern = regexp.MustCompile(`^[0-9]{5,64}$`)
var metaAdsCountryPattern = regexp.MustCompile(`^[A-Z]{2}$`)

type campaignBudgetPayload struct {
	Type   string  `json:"type"`
	Amount float64 `json:"amount"`
}

type createCampaignActionPayload struct {
	Name                string                 `json:"name"`
	Objective           string                 `json:"objective"`
	SpecialAdCategories []string               `json:"specialAdCategories"`
	Budget              *campaignBudgetPayload `json:"budget,omitempty"`
	Status              string                 `json:"status"`
}

type duplicateCampaignActionPayload struct {
	CampaignID string `json:"campaignId"`
	Name       string `json:"name"`
	Status     string `json:"status"`
}

type updateCampaignActionPayload struct {
	CampaignID string                 `json:"campaignId"`
	Name       string                 `json:"name,omitempty"`
	Budget     *campaignBudgetPayload `json:"budget,omitempty"`
}

type targetCampaignActionPayload struct {
	CampaignID string `json:"campaignId"`
}

type promoteInstagramPostActionPayload struct {
	Name            string                 `json:"name"`
	InstagramPostID string                 `json:"instagramPostId"`
	IGUserID        string                 `json:"igUserId"`
	PageID          string                 `json:"pageId"`
	ClientAccountID string                 `json:"clientAccountId,omitempty"`
	AdSetName       string                 `json:"adSetName"`
	AdName          string                 `json:"adName"`
	Budget          *campaignBudgetPayload `json:"budget"`
	Countries       []string               `json:"countries"`
	AgeMin          int                    `json:"ageMin"`
	AgeMax          int                    `json:"ageMax"`
	Status          string                 `json:"status"`
}

type normalizedActionPayload struct {
	Raw              json.RawMessage
	TargetCampaignID string
	Name             string
	Budget           *campaignBudgetPayload
}

func normalizeActionPayload(action ActionKind, raw json.RawMessage) (normalizedActionPayload, error) {
	if len(raw) == 0 || len(raw) > maxActionPayloadBytes || string(raw) == "null" {
		return normalizedActionPayload{}, ErrActionValidation
	}
	switch action {
	case ActionCreateCampaign:
		var payload createCampaignActionPayload
		if err := decodeStrictActionJSON(raw, &payload); err != nil {
			return normalizedActionPayload{}, ErrActionValidation
		}
		payload.Name = normalizeActionName(payload.Name)
		payload.Objective = strings.ToUpper(strings.TrimSpace(payload.Objective))
		categories, err := normalizeSpecialAdCategories(payload.SpecialAdCategories)
		if payload.Name == "" || !validCampaignObjective(payload.Objective) || err != nil {
			return normalizedActionPayload{}, ErrActionValidation
		}
		payload.SpecialAdCategories = categories
		payload.Status = "PAUSED"
		if err := normalizeCampaignBudget(payload.Budget); err != nil {
			return normalizedActionPayload{}, err
		}
		canonical, err := json.Marshal(payload)
		return normalizedActionPayload{Raw: canonical, Name: payload.Name, Budget: payload.Budget}, err

	case ActionDuplicateCampaign:
		var payload duplicateCampaignActionPayload
		if err := decodeStrictActionJSON(raw, &payload); err != nil {
			return normalizedActionPayload{}, ErrActionValidation
		}
		payload.CampaignID = strings.TrimSpace(payload.CampaignID)
		payload.Name = normalizeActionName(payload.Name)
		payload.Status = "PAUSED"
		if !metaAdsUUIDRe.MatchString(payload.CampaignID) || payload.Name == "" {
			return normalizedActionPayload{}, ErrActionValidation
		}
		canonical, err := json.Marshal(payload)
		return normalizedActionPayload{
			Raw: canonical, TargetCampaignID: payload.CampaignID, Name: payload.Name,
		}, err

	case ActionUpdateCampaign:
		var payload updateCampaignActionPayload
		if err := decodeStrictActionJSON(raw, &payload); err != nil {
			return normalizedActionPayload{}, ErrActionValidation
		}
		payload.CampaignID = strings.TrimSpace(payload.CampaignID)
		payload.Name = normalizeActionName(payload.Name)
		if !metaAdsUUIDRe.MatchString(payload.CampaignID) || (payload.Name == "" && payload.Budget == nil) {
			return normalizedActionPayload{}, ErrActionValidation
		}
		if err := normalizeCampaignBudget(payload.Budget); err != nil {
			return normalizedActionPayload{}, err
		}
		canonical, err := json.Marshal(payload)
		return normalizedActionPayload{
			Raw: canonical, TargetCampaignID: payload.CampaignID, Name: payload.Name, Budget: payload.Budget,
		}, err

	case ActionPauseCampaign, ActionResumeCampaign:
		var payload targetCampaignActionPayload
		if err := decodeStrictActionJSON(raw, &payload); err != nil {
			return normalizedActionPayload{}, ErrActionValidation
		}
		payload.CampaignID = strings.TrimSpace(payload.CampaignID)
		if !metaAdsUUIDRe.MatchString(payload.CampaignID) {
			return normalizedActionPayload{}, ErrActionValidation
		}
		canonical, err := json.Marshal(payload)
		return normalizedActionPayload{Raw: canonical, TargetCampaignID: payload.CampaignID}, err

	case ActionPromoteInstagramPost:
		var payload promoteInstagramPostActionPayload
		if err := decodeStrictActionJSON(raw, &payload); err != nil {
			return normalizedActionPayload{}, ErrActionValidation
		}
		payload.Name = normalizeActionName(payload.Name)
		payload.AdSetName = normalizeActionName(payload.AdSetName)
		payload.AdName = normalizeActionName(payload.AdName)
		payload.InstagramPostID = strings.TrimSpace(payload.InstagramPostID)
		payload.IGUserID = strings.TrimSpace(payload.IGUserID)
		payload.PageID = strings.TrimSpace(payload.PageID)
		payload.ClientAccountID = strings.TrimSpace(payload.ClientAccountID)
		if payload.AdSetName == "" && payload.Name != "" {
			payload.AdSetName = payload.Name + " - conjunto"
		}
		if payload.AdName == "" && payload.Name != "" {
			payload.AdName = payload.Name + " - anuncio"
		}
		if len(payload.Countries) == 0 {
			payload.Countries = []string{"BR"}
		}
		countries, err := normalizeActionCountries(payload.Countries)
		if payload.AgeMin == 0 {
			payload.AgeMin = 18
		}
		if payload.AgeMax == 0 {
			payload.AgeMax = 65
		}
		if payload.Name == "" || payload.AdSetName == "" || payload.AdName == "" ||
			!metaAdsNumericIDPattern.MatchString(payload.InstagramPostID) ||
			!metaAdsNumericIDPattern.MatchString(payload.IGUserID) ||
			!metaAdsNumericIDPattern.MatchString(payload.PageID) ||
			(payload.ClientAccountID != "" && !metaAdsUUIDRe.MatchString(payload.ClientAccountID)) ||
			payload.Budget == nil || err != nil || payload.AgeMin < 18 || payload.AgeMax > 65 ||
			payload.AgeMin > payload.AgeMax {
			return normalizedActionPayload{}, ErrActionValidation
		}
		if err := normalizeCampaignBudget(payload.Budget); err != nil || payload.Budget.Type != "daily" {
			return normalizedActionPayload{}, ErrActionValidation
		}
		payload.Countries = countries
		payload.Status = "PAUSED"
		canonical, err := json.Marshal(payload)
		return normalizedActionPayload{Raw: canonical, Name: payload.Name, Budget: payload.Budget}, err
	default:
		return normalizedActionPayload{}, ErrActionValidation
	}
}

func normalizeActionCountries(values []string) ([]string, error) {
	if len(values) == 0 || len(values) > 10 {
		return nil, ErrActionValidation
	}
	out := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.ToUpper(strings.TrimSpace(raw))
		if !metaAdsCountryPattern.MatchString(value) {
			return nil, ErrActionValidation
		}
		if !slices.Contains(out, value) {
			out = append(out, value)
		}
	}
	return out, nil
}

func decodeStrictActionJSON(raw []byte, dst any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return ErrActionValidation
	}
	return nil
}

func normalizeActionName(value string) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if value == "" || !utf8.ValidString(value) || utf8.RuneCountInString(value) > maxActionNameRunes {
		return ""
	}
	return value
}

func validCampaignObjective(value string) bool {
	switch value {
	case "OUTCOME_APP_PROMOTION", "OUTCOME_AWARENESS", "OUTCOME_ENGAGEMENT",
		"OUTCOME_LEADS", "OUTCOME_SALES", "OUTCOME_TRAFFIC":
		return true
	default:
		return false
	}
}

func normalizeSpecialAdCategories(values []string) ([]string, error) {
	if values == nil {
		values = []string{}
	}
	if len(values) > 6 {
		return nil, ErrActionValidation
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToUpper(strings.TrimSpace(value))
		switch value {
		case "CREDIT", "EMPLOYMENT", "HOUSING", "ISSUES_ELECTIONS_POLITICS",
			"ONLINE_GAMBLING_AND_GAMING", "NONE":
		default:
			return nil, ErrActionValidation
		}
		if !slices.Contains(out, value) {
			out = append(out, value)
		}
	}
	if slices.Contains(out, "NONE") && len(out) != 1 {
		return nil, ErrActionValidation
	}
	return out, nil
}

func normalizeCampaignBudget(budget *campaignBudgetPayload) error {
	if budget == nil {
		return nil
	}
	budget.Type = strings.ToLower(strings.TrimSpace(budget.Type))
	if budget.Type != "daily" && budget.Type != "lifetime" {
		return ErrActionValidation
	}
	if math.IsNaN(budget.Amount) || math.IsInf(budget.Amount, 0) || budget.Amount <= 0 || budget.Amount > maxActionBudget {
		return ErrActionValidation
	}
	minor := math.Round(budget.Amount * 100)
	if math.Abs(budget.Amount*100-minor) > 1e-7 {
		return ErrActionValidation
	}
	budget.Amount = minor / 100
	return nil
}

func normalizeActionPolicyInput(input ActionPolicyInput) (ActionPolicyInput, error) {
	normalize := func(value *float64) (*float64, error) {
		if value == nil {
			return nil, nil
		}
		if math.IsNaN(*value) || math.IsInf(*value, 0) || *value <= 0 || *value > maxActionBudget {
			return nil, ErrActionValidation
		}
		minor := math.Round(*value * 100)
		if math.Abs(*value*100-minor) > 1e-7 {
			return nil, ErrActionValidation
		}
		rounded := minor / 100
		return &rounded, nil
	}
	var err error
	input.MaxDailyBudget, err = normalize(input.MaxDailyBudget)
	if err != nil {
		return ActionPolicyInput{}, err
	}
	input.MaxLifetimeBudget, err = normalize(input.MaxLifetimeBudget)
	if err != nil {
		return ActionPolicyInput{}, err
	}
	if (input.AllowDuplicate || input.AllowResume) && input.MaxDailyBudget == nil && input.MaxLifetimeBudget == nil {
		return ActionPolicyInput{}, ErrActionValidation
	}
	return input, nil
}

func validateBudgetAgainstPolicy(budget *campaignBudgetPayload, policy *ActionPolicy) error {
	if budget == nil {
		return nil
	}
	if policy == nil {
		return ErrActionPolicyRequired
	}
	capValue := policy.MaxDailyBudget
	if budget.Type == "lifetime" {
		capValue = policy.MaxLifetimeBudget
	}
	if capValue == nil {
		return ErrActionPolicyRequired
	}
	if budgetMinorUnits(budget.Amount) > budgetMinorUnits(*capValue) {
		return ErrActionBudgetCapExceeded
	}
	return nil
}

func validateActionBudgetCurrency(
	action ActionKind,
	payload normalizedActionPayload,
	currency string,
) error {
	if (action == ActionCreateCampaign || action == ActionUpdateCampaign || action == ActionPromoteInstagramPost) && payload.Budget != nil &&
		strings.ToUpper(strings.TrimSpace(currency)) != "BRL" {
		return ErrActionCurrencyUnsupported
	}
	return nil
}

func budgetMinorUnits(value float64) int64 {
	return int64(math.Round(value * 100))
}

func actionRequestHash(action ActionKind, adAccountID string, payload json.RawMessage) string {
	canonical := fmt.Sprintf("%s\n%s\n%s", action, strings.TrimSpace(adAccountID), payload)
	digest := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(digest[:])
}

func validActionIdempotencyKey(value string) bool {
	return actionIdempotencyPattern.MatchString(strings.TrimSpace(value))
}

func sanitizeActionErrorText(value string) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if utf8.RuneCountInString(value) <= 500 {
		return value
	}
	runes := []rune(value)
	return string(runes[:500])
}
