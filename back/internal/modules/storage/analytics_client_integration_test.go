package storage

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestCloudflareUsageClientLive valida, de forma opt-in, o mesmo cliente usado
// pelo status/preflight. Nao grava, lista ou baixa objetos e nunca imprime o token.
func TestCloudflareUsageClientLive(t *testing.T) {
	if os.Getenv("STORAGE_ANALYTICS_LIVE") != "1" {
		t.Skip("set STORAGE_ANALYTICS_LIVE=1 to query Cloudflare account metrics")
	}
	client := NewCloudflareUsageClient(Config{
		AccountID:      os.Getenv("R2_ACCOUNT_ID"),
		AnalyticsToken: os.Getenv("R2_ANALYTICS_API_TOKEN"),
		RequestTimeout: 30 * time.Second,
	})
	now := time.Now().UTC()
	usage, err := client.Usage(context.Background(), billingMonth(now), now)
	if err != nil {
		t.Fatalf("query Cloudflare usage: %v", err)
	}
	if !usage.Available || !usage.Configured || usage.Source != "cloudflare_account" {
		t.Fatalf("unexpected Cloudflare usage state: %+v", usage)
	}
	t.Logf("Cloudflare usage OK: bytes=%d objects=%d classA=%d classB=%d", usage.StoredBytes, usage.ObjectCount, usage.ClassARequests, usage.ClassBRequests)
}
