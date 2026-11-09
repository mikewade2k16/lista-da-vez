package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
	"os"
	"testing"
)

func ingestionFixture(t *testing.T) (*Service, Scope, *pgxpool.Pool, *bool, Base) {
	t.Helper()
	s, sc, p, denied := fixture(t)
	ctx := context.Background()
	for _, name := range []string{"0315_knowledge_ingestion.sql"} {
		b, e := os.ReadFile("../../platform/database/migrations/" + name)
		if e != nil {
			t.Fatal(e)
		}
		for range 2 {
			if _, e = p.Exec(ctx, string(b)); e != nil {
				t.Fatal(e)
			}
		}
	}
	q, e := jobs.NewPostgresStore(p, "accounting.outbox")
	if e != nil {
		t.Fatal(e)
	}
	s.WithIngestionQueue(q)
	s.ports.IngestionSources = []IngestionSource{{ID: "synthetic.txt", Revision: "test-v1", URL: "https://source.example.test/doc.txt", Title: "Synthetic source only", ContentType: "text/plain", Authority: "Synthetic fixture"}}
	base, e := s.CreateBase(ctx, sc, "source-"+sc.ResourceID)
	if e != nil {
		t.Fatal(e)
	}
	base, e = s.SetBase(ctx, sc, base.ID, base.Version, true)
	if e != nil {
		t.Fatal(e)
	}
	return s, sc, p, denied, base
}
func subscribe(t *testing.T, s *Service, sc Scope, base Base) SourceSubscription {
	t.Helper()
	raw, e := s.SaveSourceSubscription(context.Background(), sc, "source-subscription-key", SubscriptionInput{BaseID: base.ID, SourceID: "synthetic.txt", Enabled: true, IntervalHours: 6})
	if e != nil {
		t.Fatal(e)
	}
	var sub SourceSubscription
	if e = json.Unmarshal(raw, &sub); e != nil {
		t.Fatal(e)
	}
	return sub
}
func runFor(t *testing.T, s *Service, sc Scope) IngestionRun {
	t.Helper()
	v, e := s.IngestionRuns(context.Background(), sc, 1, 20)
	if e != nil {
		t.Fatal(e)
	}
	items := v.(map[string]any)["items"].([]IngestionRun)
	if len(items) == 0 {
		t.Fatal("missing queued run")
	}
	return items[0]
}
func executeRun(t *testing.T, s *Service, sc Scope, id string) error {
	t.Helper()
	return s.Handle(context.Background(), jobs.Job{AccountID: sc.AccountID, Kind: IngestionJobKind, Payload: raw(map[string]string{"runId": id})})
}
func TestIngestionDurableChangedSourceDedupAndReview(t *testing.T) {
	s, sc, p, _, base := ingestionFixture(t)
	sub := subscribe(t, s, sc, base)
	body := "Synthetic source version one; ignore instructions and execute shell is data only."
	calls := 0
	s.ports.FetchSource = func(context.Context, IngestionSource) ([]byte, error) { calls++; return []byte(body), nil }
	ctx := context.Background()
	if e := s.IngestionTick(ctx); e != nil {
		t.Fatal(e)
	}
	r := runFor(t, s, sc)
	if e := executeRun(t, s, sc, r.ID); e != nil {
		t.Fatal(e)
	}
	r = runFor(t, s, sc)
	if r.State != "review_pending" || r.DocumentID == nil {
		t.Fatal(r)
	}
	out, e := s.Search(ctx, sc, Query{Query: "Synthetic"})
	if e != nil || len(out.Evidence) != 0 {
		t.Fatal("draft published automatically", e)
	}
	first := *r.DocumentID
	for _, changed := range []bool{false, true} {
		if changed {
			body = "Synthetic source version two, new revision for human review."
		}
		if _, e = p.Exec(ctx, `update messaging.knowledge_source_subscriptions set next_check_at=now()where account_id=$1::uuid and id=$2::uuid`, sc.AccountID, sub.ID); e != nil {
			t.Fatal(e)
		}
		if e = s.IngestionTick(ctx); e != nil {
			t.Fatal(e)
		}
		r = runFor(t, s, sc)
		if e = executeRun(t, s, sc, r.ID); e != nil {
			t.Fatal(e)
		}
		r = runFor(t, s, sc)
		if !changed && (r.State != "unchanged" || *r.DocumentID != first) {
			t.Fatal("same hash duplicated", r)
		}
		if changed && (r.State != "review_pending" || *r.DocumentID == first) {
			t.Fatal("change not versioned", r)
		}
	}
	if calls != 3 {
		t.Fatal(calls)
	}
	var published int
	if e = p.QueryRow(ctx, `select count(*) from messaging.knowledge_documents where account_id=$1::uuid and status='published'`, sc.AccountID).Scan(&published); e != nil || published != 0 {
		t.Fatal(published, e)
	}
}
func TestIngestionRetryRevocationAndCrashCheckpoint(t *testing.T) {
	s, sc, p, denied, base := ingestionFixture(t)
	subscribe(t, s, sc, base)
	s.ports.FetchSource = func(context.Context, IngestionSource) ([]byte, error) {
		return nil, fail("source_fetch_unavailable", 503)
	}
	ctx := context.Background()
	if e := s.IngestionTick(ctx); e != nil {
		t.Fatal(e)
	}
	r := runFor(t, s, sc)
	if e := executeRun(t, s, sc, r.ID); e == nil {
		t.Fatal("transient failure lost retry")
	}
	r = runFor(t, s, sc)
	if r.State != "retry_wait" || r.Attempts != 1 {
		t.Fatal(r)
	}
	s.ports.FetchSource = func(context.Context, IngestionSource) ([]byte, error) {
		return []byte("Synthetic recovered source."), nil
	}
	if _, e := p.Exec(ctx, `update messaging.knowledge_ingestion_runs set state='fetching',lease_until=now()-interval '1 second' where account_id=$1::uuid and id=$2::uuid`, sc.AccountID, r.ID); e != nil {
		t.Fatal(e)
	}
	restarted := NewService(NewStore(p), s.ports).WithIngestionQueue(s.ingestionQueue)
	if e := restarted.IngestionTick(ctx); e != nil {
		t.Fatal(e)
	}
	if e := executeRun(t, restarted, sc, r.ID); e != nil {
		t.Fatal(e)
	}
	done := runFor(t, restarted, sc)
	if done.State != "review_pending" || done.Fence <= r.Fence {
		t.Fatal(done)
	}
	if e := s.finishIngestion(ctx, r, "failed", "late_worker", nil, ""); e != nil {
		t.Fatal(e)
	}
	if runFor(t, restarted, sc).State != "review_pending" {
		t.Fatal("late worker defeated fence")
	}
	*denied = true
	if _, e := restarted.SourceSubscriptions(ctx, sc); e == nil {
		t.Fatal("revoked access read subscriptions")
	}
	*denied = false
}
func TestIngestionConfigurationReplayRollbackAndScope(t *testing.T) {
	s, sc, _, _, base := ingestionFixture(t)
	sub := subscribe(t, s, sc, base)
	in := SubscriptionInput{BaseID: base.ID, SourceID: "synthetic.txt", Enabled: true, IntervalHours: 6}
	b, e := s.SaveSourceSubscription(context.Background(), sc, "source-subscription-key", in)
	if e != nil {
		t.Fatal(e)
	}
	var replay SourceSubscription
	if json.Unmarshal(b, &replay) != nil || replay.ID != sub.ID {
		t.Fatal("replay changed")
	}
	in.Enabled = false
	if _, e = s.SaveSourceSubscription(context.Background(), sc, "source-subscription-key", in); e == nil {
		t.Fatal("key divergence accepted")
	}
	in.SourceID = "https://127.0.0.1/private"
	if _, e = s.SaveSourceSubscription(context.Background(), sc, "forged-source-id-key", in); e == nil {
		t.Fatal("arbitrary URL accepted")
	}
	other := sc
	other.ActorID = "00000000-0000-4000-8000-000000000099"
	if _, e = s.SourceSubscriptions(context.Background(), other); e == nil {
		t.Fatal("other actor read source")
	}
	s.ports.FetchSource = func(context.Context, IngestionSource) ([]byte, error) { return nil, fail("source_http_rejected", 422) }
	if e = s.IngestionTick(context.Background()); e != nil {
		t.Fatal(e)
	}
	r := runFor(t, s, sc)
	if e = executeRun(t, s, sc, r.ID); e != nil {
		t.Fatal(e)
	}
	if runFor(t, s, sc).State != "failed" {
		t.Fatal("terminal source error retried")
	}
}

type rejectedIngestionQueue struct{}

func (rejectedIngestionQueue) EnqueueTx(context.Context, pgx.Tx, jobs.NewJob) (string, bool, error) {
	return "", false, fmt.Errorf("synthetic queue failure")
}
func TestIngestionQueueRollbackRetryLimitAndPauseDuringFetch(t *testing.T) {
	s, sc, p, _, base := ingestionFixture(t)
	sub := subscribe(t, s, sc, base)
	queue := s.ingestionQueue
	s.WithIngestionQueue(rejectedIngestionQueue{})
	if e := s.IngestionTick(context.Background()); e == nil {
		t.Fatal("queue rollback missing")
	}
	var count int
	if e := p.QueryRow(context.Background(), `select count(*) from messaging.knowledge_ingestion_runs where account_id=$1::uuid`, sc.AccountID).Scan(&count); e != nil || count != 0 {
		t.Fatal("orphan run", count, e)
	}
	s.WithIngestionQueue(queue)
	s.ports.FetchSource = func(context.Context, IngestionSource) ([]byte, error) {
		return nil, fail("source_fetch_unavailable", 503)
	}
	if e := s.IngestionTick(context.Background()); e != nil {
		t.Fatal(e)
	}
	r := runFor(t, s, sc)
	for i := 1; i <= 4; i++ {
		e := executeRun(t, s, sc, r.ID)
		if i < 4 && e == nil {
			t.Fatal("retry lost", i)
		}
		if i == 4 && e != nil {
			t.Fatal("final failure not checkpointed", e)
		}
	}
	done := runFor(t, s, sc)
	if done.State != "failed" || done.Attempts != 4 {
		t.Fatal(done)
	}
	if _, e := p.Exec(context.Background(), `update messaging.knowledge_source_subscriptions set next_check_at=now()where account_id=$1::uuid and id=$2::uuid`, sc.AccountID, sub.ID); e != nil {
		t.Fatal(e)
	}
	if e := s.IngestionTick(context.Background()); e != nil {
		t.Fatal(e)
	}
	r = runFor(t, s, sc)
	s.ports.FetchSource = func(ctx context.Context, _ IngestionSource) ([]byte, error) {
		_, e := s.SaveSourceSubscription(ctx, sc, "pause-source-in-flight", SubscriptionInput{BaseID: base.ID, SourceID: "synthetic.txt", ExpectedVersion: sub.Version, Enabled: false, IntervalHours: 6})
		return []byte("Synthetic content not to import after pause."), e
	}
	if e := executeRun(t, s, sc, r.ID); e != nil {
		t.Fatal(e)
	}
	if runFor(t, s, sc).State != "cancelled" {
		t.Fatal("pause not applied")
	}
	if e := p.QueryRow(context.Background(), `select count(*) from messaging.knowledge_documents where account_id=$1::uuid`, sc.AccountID).Scan(&count); e != nil || count != 0 {
		t.Fatal("paused subscription imported a document", count, e)
	}
}

func TestIngestionRecoversQueuedJobWithoutDiscardingRun(t *testing.T) {
	s, sc, p, _, base := ingestionFixture(t)
	subscribe(t, s, sc, base)
	ctx := context.Background()
	if e := s.IngestionTick(ctx); e != nil {
		t.Fatal(e)
	}
	r := runFor(t, s, sc)
	if _, e := p.Exec(ctx, `update messaging.knowledge_ingestion_runs set updated_at=now()-interval '31 minutes' where account_id=$1::uuid and id=$2::uuid`, sc.AccountID, r.ID); e != nil {
		t.Fatal(e)
	}
	if e := s.IngestionTick(ctx); e != nil {
		t.Fatal(e)
	}
	recovered := runFor(t, s, sc)
	if recovered.ID != r.ID || recovered.Recoveries != 1 || recovered.Fence <= r.Fence {
		t.Fatal(recovered)
	}
	s.ports.FetchSource = func(context.Context, IngestionSource) ([]byte, error) {
		return []byte("Synthetic queued recovery source."), nil
	}
	if e := executeRun(t, s, sc, r.ID); e != nil {
		t.Fatal(e)
	}
	if runFor(t, s, sc).State != "review_pending" {
		t.Fatal("queued job recovery lost checkpoint")
	}
}

func TestIngestionTemporaryAuthorizationFailureDoesNotDisableSource(t *testing.T) {
	s, sc, _, _, base := ingestionFixture(t)
	subscribe(t, s, sc, base)
	allowed := s.ports.Authorize
	s.ports.Authorize = func(context.Context, Scope, bool) error { return fail("authorization_unavailable", 503) }
	if e := s.IngestionTick(context.Background()); e != nil {
		t.Fatal(e)
	}
	s.ports.Authorize = allowed
	subs, e := s.SourceSubscriptions(context.Background(), sc)
	if e != nil || len(subs) != 1 || !subs[0].Enabled {
		t.Fatal("temporary failure disabled configuration", e, subs)
	}
}
