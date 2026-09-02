package omnichannel

import (
	"testing"
	"time"
)

func TestDeriveOperationalAlertsIsActionableAndPIIFree(t *testing.T) {
	now := time.Date(2026, 8, 28, 18, 0, 0, 0, time.UTC)
	lastPurge := now.Add(-72 * time.Hour)
	alerts := deriveOperationalAlerts(OperationalHealthView{
		Outbox:    OperationalBacklogHealth{Dead: 2, OldestPendingSeconds: 180},
		AI:        OperationalAIHealth{StuckProcessing: 1},
		Provider:  OperationalProviderHealth{MissingCredentials: 1},
		Retention: OperationalRetentionHealth{LastFinishedAt: &lastPurge},
		Bindings:  OperationalBindingHealth{Mismatches: 1},
	}, now)
	if len(alerts) != 6 {
		t.Fatalf("alerts=%d, want 6: %+v", len(alerts), alerts)
	}
	seen := map[string]bool{}
	for _, alert := range alerts {
		if alert.Action == "" || alert.Owner != "omnichannel" || alert.Runbook != omnichannelOperationalRunbook {
			t.Fatalf("alerta sem owner/action/runbook: %+v", alert)
		}
		seen[alert.Code] = true
	}
	for _, code := range []string{"outbox_dead", "outbox_backlog", "ai_dispatch_stuck",
		"provider_credentials_missing", "automation_binding_mismatch", "retention_stale"} {
		if !seen[code] {
			t.Fatalf("alerta %s ausente: %+v", code, alerts)
		}
	}
}
