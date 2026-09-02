package omnichannel

import (
	"errors"
	"testing"
	"time"
)

func TestNormalizeRolloutInputRequiresPauseReasonAndNormalizesScope(t *testing.T) {
	instanceID := "AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA"
	pause := "  incidente de qualidade  "
	in := RolloutConfigInput{
		Mode: RolloutModePaused, ExpectedRevision: 0, Reason: "  pausa operacional  ",
		AllowedInstanceIDs: []string{instanceID, instanceID}, AutoReplyPercent: 0,
		AllowedHours: RolloutHours{Timezone: "America/Sao_Paulo"},
		ExcludedTags: []string{" VIP ", "vip"}, KillSwitchReason: &pause,
	}
	if err := normalizeRolloutInput(&in); err != nil {
		t.Fatal(err)
	}
	if len(in.AllowedInstanceIDs) != 1 || in.AllowedInstanceIDs[0] != "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa" {
		t.Fatalf("instances=%v", in.AllowedInstanceIDs)
	}
	if len(in.ExcludedTags) != 1 || in.ExcludedTags[0] != "vip" {
		t.Fatalf("tags=%v", in.ExcludedTags)
	}
	if in.KillSwitchReason == nil || *in.KillSwitchReason != "incidente de qualidade" {
		t.Fatalf("kill reason=%v", in.KillSwitchReason)
	}

	missingReason := in
	missingReason.KillSwitchReason = nil
	if err := normalizeRolloutInput(&missingReason); !errors.Is(err, ErrValidation) {
		t.Fatalf("paused sem motivo=%v", err)
	}
}

func TestRolloutHourAndCohortAreDeterministic(t *testing.T) {
	hours := RolloutHours{Timezone: "America/Sao_Paulo", Windows: []RolloutWindow{{
		Days: []int{5}, Start: "09:00", End: "18:00",
	}}}
	inside := time.Date(2026, 8, 28, 15, 0, 0, 0, time.UTC)  // Friday, 12:00 BRT
	outside := time.Date(2026, 8, 29, 15, 0, 0, 0, time.UTC) // Saturday
	if !rolloutHourAllowed(hours, inside) || rolloutHourAllowed(hours, outside) {
		t.Fatalf("allowed inside=%v outside=%v", rolloutHourAllowed(hours, inside), rolloutHourAllowed(hours, outside))
	}
	first := rolloutBucket("account-a", "conversation-a")
	if first < 0 || first > 99 || rolloutBucket("account-a", "conversation-a") != first {
		t.Fatalf("bucket instavel=%d", first)
	}
}
