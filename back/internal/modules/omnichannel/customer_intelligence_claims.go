package omnichannel

import (
	"math"
	"strconv"
	"strings"
)

const (
	maxCustomerIntelligenceClaims       = 100
	maxCustomerIntelligenceEvidenceRefs = 100
)

func validCustomerIntelligenceClaimEnvelope(
	runs []CustomerIntelligenceProcessRunRef,
	claims []CustomerIntelligenceAcceptedClaimRef,
) bool {
	if len(claims) == 0 {
		return true
	}
	if len(claims) > maxCustomerIntelligenceClaims || len(runs) == 0 || len(runs) > 32 {
		return false
	}
	runByID := make(map[string]CustomerIntelligenceProcessRunRef, len(runs))
	for _, run := range runs {
		requiredUUIDs := []string{
			run.RunID, run.ProcessDefinitionID, run.ProcessConfigVersionID,
			run.PromptBindingID, run.PlatformPromptVersionID,
			run.ProcessPromptVersionID, run.AgentVersionID, run.ModelID,
			run.ContextSnapshotID,
		}
		for _, value := range requiredUUIDs {
			if !omnichannelUUIDPattern.MatchString(strings.TrimSpace(value)) {
				return false
			}
		}
		for _, value := range []string{run.AgencyPromptVersionID, run.ClientPromptVersionID} {
			if value != "" && !omnichannelUUIDPattern.MatchString(strings.TrimSpace(value)) {
				return false
			}
		}
		if strings.TrimSpace(run.ProcessKey) == "" ||
			len(run.ProcessKey) > 160 ||
			run.Status != "succeeded" ||
			run.ExecutionMode != "active" ||
			strings.TrimSpace(run.OutputSchemaVersion) == "" ||
			len(run.OutputSchemaVersion) > 160 {
			return false
		}
		if _, duplicate := runByID[run.RunID]; duplicate {
			return false
		}
		runByID[run.RunID] = run
	}
	seen := make(map[string]bool, len(claims))
	for _, claim := range claims {
		run, exists := runByID[claim.RuntimeRunID]
		key := claim.RuntimeRunID + ":" + strconv.Itoa(claim.Ordinal)
		if !exists || seen[key] ||
			claim.Ordinal < 0 || claim.Ordinal >= maxCustomerIntelligenceClaims ||
			strings.TrimSpace(claim.FactKey) == "" || len(claim.FactKey) > 160 ||
			strings.TrimSpace(claim.ValueType) == "" || len(claim.ValueType) > 40 ||
			claim.Confidence < 0 || claim.Confidence > 1 ||
			math.IsNaN(claim.Confidence) || math.IsInf(claim.Confidence, 0) ||
			len(claim.EvidenceObservationIDs) > maxCustomerIntelligenceEvidenceRefs ||
			claim.ProcessKey != run.ProcessKey ||
			claim.PromptBindingID != run.PromptBindingID ||
			claim.OutputSchemaVersion != run.OutputSchemaVersion {
			return false
		}
		for _, observationID := range claim.EvidenceObservationIDs {
			if !omnichannelUUIDPattern.MatchString(strings.TrimSpace(observationID)) {
				return false
			}
		}
		seen[key] = true
	}
	return true
}
