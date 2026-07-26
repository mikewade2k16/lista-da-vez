package socialpublishing

import (
	"context"
	"testing"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
)

func TestOutboxLanesUseDistinctPlatformJobsTables(t *testing.T) {
	if publishOutboxTable != "social_publishing.outbox" {
		t.Fatalf("publish table = %q", publishOutboxTable)
	}
	if analyticsOutboxTable != "social_publishing.analytics_outbox" {
		t.Fatalf("analytics table = %q", analyticsOutboxTable)
	}
	if publishOutboxTable == analyticsOutboxTable {
		t.Fatal("publish and analytics must never share the same outbox table")
	}
}

func TestPublishWorkerPoolUsesSingleJobClaimsAndUniqueIDs(t *testing.T) {
	configs := publishWorkerConfigs(nil)
	if len(configs) != publishWorkerPoolSize || len(configs) < 2 {
		t.Fatalf("publish worker configs = %d, want %d", len(configs), publishWorkerPoolSize)
	}
	seen := make(map[string]struct{}, len(configs))
	for _, config := range configs {
		if config.Batch != 1 {
			t.Fatalf("worker %q batch = %d, want 1", config.WorkerID, config.Batch)
		}
		if config.WorkerID == "" {
			t.Fatal("publish worker ID must not be empty")
		}
		if _, exists := seen[config.WorkerID]; exists {
			t.Fatalf("duplicate publish worker ID %q", config.WorkerID)
		}
		seen[config.WorkerID] = struct{}{}
	}
}

func TestLegacyPublishLaneCanDrainAnalyticsJobs(t *testing.T) {
	publish := jobs.HandlerFunc(func(context.Context, jobs.Job) error { return nil })
	analytics := jobs.HandlerFunc(func(context.Context, jobs.Job) error { return nil })
	handlers := publishLaneHandlers(publish, analytics)

	if handlers[PublishJobKind] == nil {
		t.Fatalf("legacy lane missing %q handler", PublishJobKind)
	}
	if handlers[AnalyticsJobKind] == nil {
		t.Fatalf("legacy lane missing compatibility handler %q", AnalyticsJobKind)
	}
}

func TestAnalyticsWorkerTimeoutCoversSequentialInsightsRequests(t *testing.T) {
	config := analyticsWorkerConfig(nil)
	if config.Batch != 1 {
		t.Fatalf("analytics batch = %d, want 1 to avoid unleased claimed jobs", config.Batch)
	}
	if config.JobTimeout != analyticsJobTimeout {
		t.Fatalf("analytics timeout = %s, want %s", config.JobTimeout, analyticsJobTimeout)
	}
	if config.JobTimeout <= 140*time.Second {
		t.Fatalf("analytics timeout = %s, must exceed seven 20s provider calls", config.JobTimeout)
	}
}
