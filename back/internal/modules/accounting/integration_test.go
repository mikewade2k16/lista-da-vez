package accounting

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
)

func TestPersistenceIsolationVersionsAndForeignKeys(t *testing.T) {
	s, sc, p := fixture(t)
	ctx := context.Background()
	d := draft(t, s, sc)
	reloaded := NewService(NewStore(p), &testPolicy{}, s.queue)
	if _, e := reloaded.Read(ctx, sc, "document", d.ID, "", "", 1, 20); e != nil {
		t.Fatal(e)
	}
	other := sc
	other.ActorID = "00000000-0000-4000-8000-000000000099"
	_, e := s.Read(ctx, other, "document", d.ID, "", "", 1, 20)
	expectCode(t, e, "not_found")
	other = sc
	other.AccountID = "00000000-0000-4000-8000-000000000099"
	_, e = s.Read(ctx, other, "document", d.ID, "", "", 1, 20)
	expectCode(t, e, "not_found")
	_, e = s.DocumentCommand(ctx, sc, d.ID, "edit", "edit-stale-key", Command{ExpectedVersion: 99, Fields: &d.Fields})
	expectCode(t, e, "version_conflict")
	otherIssuer, e := s.CreateIssuer(ctx, Scope{AccountID: sc.AccountID, ActorID: sc.ActorID}, "second-issuer-key", Command{Profile: &Profile{LegalName: "Other"}})
	if e != nil {
		t.Fatal(e)
	}
	other = sc
	other.IssuerID = field(t, otherIssuer, "id")
	_, e = s.Read(ctx, other, "document", d.ID, "", "", 1, 20)
	expectCode(t, e, "not_found")
	// Composite FK prevents changing only the document's issuer to an unrelated profile/diagnosis.
	if _, e = p.Exec(ctx, `update accounting.documents set issuer_id=$1::uuid where account_id=$2::uuid and id=$3::uuid`, other.IssuerID, sc.AccountID, d.ID); e == nil {
		t.Fatal("cross-issuer foreign key accepted")
	}
	s.policy = &testPolicy{disabled: true}
	_, e = s.Read(ctx, sc, "document", d.ID, "", "", 1, 20)
	expectCode(t, e, "module_disabled")
	s.policy = &testPolicy{deny: "accounting.documents.issue"}
	_, e = s.DocumentCommand(ctx, sc, d.ID, "authorize", "denied-issue-key", Command{ExpectedVersion: 1})
	expectCode(t, e, "permission_denied")
}
func TestConcurrentDedupeAndEditing(t *testing.T) {
	s, sc, p := fixture(t)
	ctx := context.Background()
	d := draft(t, s, sc)
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	ids := make(chan string, 8)
	for range 8 {
		wg.Go(func() {
			b, e := s.DocumentCommand(ctx, sc, d.ID, "edit", "same-edit-key-001", Command{ExpectedVersion: 1, Fields: &d.Fields})
			if e == nil {
				var v Document
				e = json.Unmarshal(b, &v)
				ids <- v.ID
			}
			errs <- e
		})
	}
	wg.Wait()
	close(errs)
	close(ids)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	for id := range ids {
		if id != d.ID {
			t.Fatal("replay changed identity")
		}
	}
	var v int64
	if e := p.QueryRow(ctx, `select version from accounting.documents where account_id=$1::uuid and issuer_id=$2::uuid and id=$3::uuid`, sc.AccountID, sc.IssuerID, d.ID).Scan(&v); e != nil || v != 2 {
		t.Fatalf("version %d: %v", v, e)
	}
	changed := d.Fields
	changed.Amount = ptr("126.00")
	_, e := s.DocumentCommand(ctx, sc, d.ID, "edit", "same-edit-key-001", Command{ExpectedVersion: 1, Fields: &changed})
	expectCode(t, e, "idempotency_conflict")
	successes := 0
	var mu sync.Mutex
	for i := range 2 {
		wg.Go(func() {
			key := []string{"concurrent-edit-a", "concurrent-edit-b"}[i]
			_, e := s.DocumentCommand(ctx, sc, d.ID, "edit", key, Command{ExpectedVersion: 2, Fields: &d.Fields})
			if e == nil {
				mu.Lock()
				successes++
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	if successes != 1 {
		t.Fatalf("wanted one CAS winner, got %d", successes)
	}
}
func TestFiscalFakeOutcomesAndReplay(t *testing.T) {
	for _, mode := range []string{"authorized", "reject", "timeout_before", "timeout_after"} {
		t.Run(mode, func(t *testing.T) {
			s, sc, p := fixture(t)
			ctx := context.Background()
			c := &fakeConnector{mode: mode}
			_, b := queued(t, s, sc, c)
			id := field(t, b, "executionId")
			payload, _ := json.Marshal(map[string]string{"executionId": id})
			job := jobs.Job{AccountID: sc.AccountID, Payload: payload}
			if e := s.Handle(ctx, job); e != nil {
				t.Fatal(e)
			}
			// Simulate process restart: a new domain instance loads all state from PostgreSQL.
			reloaded := NewService(NewStore(p), &testPolicy{}, s.queue).WithConnectors(map[string]Connector{"fake.nfse.v1": c})
			for range 2 {
				if e := reloaded.Handle(ctx, job); e != nil {
					t.Fatal(e)
				}
			}
			out, e := s.Read(ctx, sc, "execution", id, "", "", 1, 20)
			if e != nil {
				t.Fatal(e)
			}
			x := out.(Execution)
			if c.submits != 1 || c.effects > 1 {
				t.Fatalf("duplicate side effect submits=%d effects=%d", c.submits, c.effects)
			}
			switch mode {
			case "reject":
				if x.State != "rejected" {
					t.Fatal(x.State)
				}
			case "timeout_before":
				if x.State != "unknown" || c.lookups < 1 {
					t.Fatal("ambiguous absence incorrectly retried")
				}
			case "timeout_after":
				if x.State != "authorized" || c.lookups != 1 {
					t.Fatal("missing reconciliation")
				}
			default:
				if x.State != "authorized" {
					t.Fatal(x.State)
				}
			}
			if !x.Simulated {
				t.Fatal("fake presented as production")
			}
		})
	}
}
func TestQueueRollbackAndProductionUnavailable(t *testing.T) {
	s, sc, p := fixture(t)
	ctx := context.Background()
	c := &fakeConnector{}
	d, b := queued(t, s, sc, c)
	// A queued command is never sent when the profile is changed before the claim.
	_, e := s.UpdateIssuer(ctx, sc, "profile-update-before-claim", Command{ExpectedVersion: 1, Profile: &Profile{LegalName: "Changed"}})
	if e != nil {
		t.Fatal(e)
	}
	payload, _ := json.Marshal(map[string]string{"executionId": field(t, b, "executionId")})
	if e = s.Handle(ctx, jobs.Job{AccountID: sc.AccountID, Payload: payload}); e != nil {
		t.Fatal(e)
	}
	if c.submits != 0 {
		t.Fatal("stale snapshot submitted")
	}
	_ = d
	// Inject an enqueue failure inside execute's transaction using an otherwise valid setup.
	s2, sc2, p2 := fixture(t)
	c2 := &fakeConnector{}
	d2, b2 := queued(t, s2, sc2, c2)
	// Delete only this synthetic command and receipt to recreate the exact pre-execute state.
	id := field(t, b2, "executionId")
	for _, q := range []string{`delete from accounting.execution_events where execution_id=$1::uuid`, `delete from accounting.outbox where payload->>'executionId'=$1`, `delete from accounting.request_receipts where operation like 'document.%execute' and result->>'executionId'=$1`, `delete from accounting.executions where id=$1::uuid`} {
		if _, e = p2.Exec(ctx, q, id); e != nil {
			t.Fatal(e)
		}
	}
	if _, e = p2.Exec(ctx, `update accounting.documents set version=$2 where id=$1::uuid`, d2.ID, d2.Version); e != nil {
		t.Fatal(e)
	}
	var auth string
	if e = p2.QueryRow(ctx, `select id::text from accounting.submission_authorizations where document_id=$1::uuid`, d2.ID).Scan(&auth); e != nil {
		t.Fatal(e)
	}
	s2.queue = failedQueue{}
	_, e = s2.DocumentCommand(ctx, sc2, d2.ID, "execute", "rollback-execute-key", Command{ExpectedVersion: d2.Version, AuthorizationID: auth, BusinessKey: "rollback-business-key", Operation: "issue"})
	if e == nil {
		t.Fatal("queue failure ignored")
	}
	var count int
	if e = p2.QueryRow(ctx, `select count(*) from accounting.executions where document_id=$1::uuid`, d2.ID).Scan(&count); e != nil || count != 0 {
		t.Fatalf("orphan execution: %d %v", count, e)
	}
	if e = p.QueryRow(ctx, `select count(*) from accounting.outbox where account_id=$1::uuid`, sc.AccountID).Scan(&count); e != nil {
		t.Fatal(e)
	}
	s3, sc3, _ := fixture(t)
	d3 := draft(t, s3, sc3)
	_, e = s3.DocumentCommand(ctx, sc3, d3.ID, "validate", "production-disabled", Command{ExpectedVersion: 1, ConnectorID: "real.nfse.v1", Environment: "production"})
	expectCode(t, e, "coverage_unavailable")
}

func TestExecuteConcurrentReplayCreatesOneCommandAndJob(t *testing.T) {
	s, sc, p := fixture(t)
	ctx := context.Background()
	d, b := queued(t, s, sc, &fakeConnector{})
	id := field(t, b, "executionId")
	var authorization string
	if e := p.QueryRow(ctx, `select authorization_id::text from accounting.executions where id=$1::uuid`, id).Scan(&authorization); e != nil {
		t.Fatal(e)
	}
	in := Command{ExpectedVersion: d.Version, AuthorizationID: authorization, BusinessKey: "synthetic-business-key-001", Operation: "issue"}
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Go(func() {
			out, e := s.DocumentCommand(ctx, sc, d.ID, "execute", "execute-key-001", in)
			if e == nil {
				var x Execution
				e = json.Unmarshal(out, &x)
				if x.ID != id {
					e = failure("replay_changed", 500)
				}
			}
			errs <- e
		})
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	var count int
	if e := p.QueryRow(ctx, `select count(*) from accounting.executions where account_id=$1::uuid and issuer_id=$2::uuid and document_id=$3::uuid`, sc.AccountID, sc.IssuerID, d.ID).Scan(&count); e != nil || count != 1 {
		t.Fatalf("commands %d: %v", count, e)
	}
	if e := p.QueryRow(ctx, `select count(*) from accounting.outbox where account_id=$1::uuid and payload->>'executionId'=$2`, sc.AccountID, id).Scan(&count); e != nil || count != 1 {
		t.Fatalf("jobs %d: %v", count, e)
	}
	in.BusinessKey = "different-business-key"
	_, e := s.DocumentCommand(ctx, sc, d.ID, "execute", "execute-key-001", in)
	expectCode(t, e, "idempotency_conflict")
}

func TestNFEUsesSameFiscalEngine(t *testing.T) {
	s, sc, _ := fixture(t)
	ctx := context.Background()
	c := &fakeConnector{}
	d, b := queuedForKind(t, s, sc, c, "nfe")
	if d.Kind != "nfe" {
		t.Fatal("product draft became service")
	}
	id := field(t, b, "executionId")
	payload, _ := json.Marshal(map[string]string{"executionId": id})
	if e := s.Handle(ctx, jobs.Job{AccountID: sc.AccountID, Payload: payload}); e != nil {
		t.Fatal(e)
	}
	out, e := s.Read(ctx, sc, "execution", id, "", "", 1, 20)
	if e != nil {
		t.Fatal(e)
	}
	x := out.(Execution)
	if x.State != "authorized" || !x.Simulated || c.effects != 1 {
		t.Fatal("product did not share controlled engine")
	}
}

func TestMemberGrantCASRevocationAndIssueCeiling(t *testing.T) {
	s, sc, p := fixture(t)
	ctx := context.Background()
	var user string
	if e := p.QueryRow(ctx, `insert into core.users default values returning id::text`).Scan(&user); e != nil {
		t.Fatal(e)
	}
	b, e := s.SaveMember(ctx, sc, user, "grant-view-key", Command{Grants: []string{"view"}, Status: "active"})
	if e != nil {
		t.Fatal(e)
	}
	other := sc
	other.ActorID = user
	if _, e = s.Read(ctx, other, "issuer", "", "", "", 1, 20); e != nil {
		t.Fatal(e)
	}
	if _, e = s.SaveMember(ctx, sc, user, "grant-stale-key", Command{ExpectedVersion: 99, Grants: []string{"view"}, Status: "inactive"}); e == nil {
		t.Fatal("stale grant update")
	}
	if _, e = s.SaveMember(ctx, sc, user, "revoke-member-key", Command{ExpectedVersion: version(t, b), Grants: []string{}, Status: "inactive"}); e != nil {
		t.Fatal(e)
	}
	_, e = s.Read(ctx, other, "issuer", "", "", "", 1, 20)
	expectCode(t, e, "not_found")
	// Account-level issue alone cannot grant issue without the issuer-level grant.
	if _, e = p.Exec(ctx, `update accounting.issuer_members set grants=array['view','manage'] where account_id=$1::uuid and issuer_id=$2::uuid and user_id=$3::uuid`, sc.AccountID, sc.IssuerID, sc.ActorID); e != nil {
		t.Fatal(e)
	}
	_, e = s.SaveMember(ctx, sc, user, "attempt-issue-escalation", Command{ExpectedVersion: 2, Grants: []string{"view", "issue"}, Status: "active"})
	expectCode(t, e, "permission_denied")
}

type syntheticEvidenceVerifier struct{}

func (syntheticEvidenceVerifier) Verify(_ context.Context, _ Scope, _ string, a Answer, _ *string) error {
	if a.EvidenceRef == nil || *a.EvidenceRef != "synthetic-reviewed-reference" {
		return failure("insufficient_evidence", 422)
	}
	return nil
}
func TestConfirmedDiagnosisRequiresReviewedOwnerEvidence(t *testing.T) {
	s, sc, _ := fixture(t)
	ctx := context.Background()
	in := DiagnosisInput{ExpectedProfileVersion: 1, Intent: "service", CompetenceDate: ptr("2026-11-03"), Answers: map[string]Answer{"taxRegime": {Value: ptr("synthetic"), Confirmation: "confirmed", Origin: ptr("official_evidence"), EvidenceRef: ptr("synthetic-reviewed-reference")}}}
	_, e := s.SaveDiagnosis(ctx, sc, "confirmed-diagnosis-key", in)
	expectCode(t, e, "evidence_verification_unavailable")
	s.WithEvidenceVerifier(syntheticEvidenceVerifier{})
	b, e := s.SaveDiagnosis(ctx, sc, "confirmed-diagnosis-key", in)
	if e != nil {
		t.Fatal(e)
	}
	out, e := s.Read(ctx, sc, "diagnosis", field(t, b, "id"), "", "", 1, 20)
	if e != nil {
		t.Fatal(e)
	}
	var saved struct {
		Answers map[string]Answer `json:"answers"`
	}
	if e = json.Unmarshal(out.(json.RawMessage), &saved); e != nil {
		t.Fatal(e)
	}
	if saved.Answers["taxRegime"].Confirmation != "confirmed" {
		t.Fatal("confirmation lost on reload")
	}
}
