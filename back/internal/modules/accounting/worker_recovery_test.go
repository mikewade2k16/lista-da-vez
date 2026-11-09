package accounting

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
)

func TestLostLeaseFencingAndReconciliation(t *testing.T) {
	s, sc, p := fixture(t)
	ctx := context.Background()
	c := &fakeConnector{}
	_, b := queued(t, s, sc, c)
	id := field(t, b, "executionId")
	_, old, _, _, lookup, e := s.claimExecution(ctx, sc.AccountID, id)
	if e != nil || lookup || old.ID == "" {
		t.Fatalf("claim: %v", e)
	}
	// A remote effect happened, then the process died before storing its outcome.
	c.accepted = true
	c.effects = 1
	if _, e = p.Exec(ctx, `update accounting.executions set lease_until=now()-interval '1 second' where account_id=$1::uuid and id=$2::uuid`, sc.AccountID, id); e != nil {
		t.Fatal(e)
	}
	if e = s.ReconcileStale(ctx); e != nil {
		t.Fatal(e)
	}
	payload, _ := json.Marshal(map[string]string{"executionId": id})
	if e = s.Handle(ctx, jobs.Job{AccountID: sc.AccountID, Payload: payload}); e != nil {
		t.Fatal(e)
	}
	// Old worker's rejection must not overwrite the reconciled authorization.
	if e = s.finish(ctx, sc, old, Outcome{State: "rejected"}, ""); e != nil {
		t.Fatal(e)
	}
	out, e := s.Read(ctx, sc, "execution", id, "", "", 1, 20)
	if e != nil {
		t.Fatal(e)
	}
	if out.(Execution).State != "authorized" || c.submits != 0 || c.lookups != 1 {
		t.Fatal("recovery resubmitted or old fence overwrote result")
	}
}
func TestRevocationAndExpiryBeforeClaim(t *testing.T) {
	for _, reason := range []string{"authorization", "membership", "expiry"} {
		t.Run(reason, func(t *testing.T) {
			s, sc, p := fixture(t)
			ctx := context.Background()
			c := &fakeConnector{}
			_, b := queued(t, s, sc, c)
			id := field(t, b, "executionId")
			switch reason {
			case "authorization":
				var authorization string
				if e := p.QueryRow(ctx, `select authorization_id::text from accounting.executions where id=$1::uuid`, id).Scan(&authorization); e != nil {
					t.Fatal(e)
				}
				if _, e := s.RevokeAuthorization(ctx, sc, authorization, "revoke-auth-key", Command{ExpectedVersion: 1}); e != nil {
					t.Fatal(e)
				}
			case "membership":
				if _, e := p.Exec(ctx, `update accounting.issuer_members set status='inactive' where account_id=$1::uuid and issuer_id=$2::uuid and user_id=$3::uuid`, sc.AccountID, sc.IssuerID, sc.ActorID); e != nil {
					t.Fatal(e)
				}
			case "expiry":
				if _, e := p.Exec(ctx, `update accounting.submission_authorizations set expires_at=now()-interval '1 minute' where account_id=$1::uuid and issuer_id=$2::uuid`, sc.AccountID, sc.IssuerID); e != nil {
					t.Fatal(e)
				}
			}
			payload, _ := json.Marshal(map[string]string{"executionId": id})
			if e := s.Handle(ctx, jobs.Job{AccountID: sc.AccountID, Payload: payload}); e != nil {
				t.Fatal(e)
			}
			if c.submits != 0 {
				t.Fatal("revoked authority submitted")
			}
		})
	}
}

type artifactMemory struct{ calls int }

func (s *artifactMemory) PutPrivate(context.Context, Scope, string, Artifact) (string, error) {
	s.calls++
	return "00000000-0000-4000-8000-000000000111", nil
}
func TestArtifactsRecoveryNeverResubmits(t *testing.T) {
	s, sc, _ := fixture(t)
	ctx := context.Background()
	c := &fakeConnector{}
	_, b := queued(t, s, sc, c)
	id := field(t, b, "executionId")
	payload, _ := json.Marshal(map[string]string{"executionId": id})
	if e := s.Handle(ctx, jobs.Job{AccountID: sc.AccountID, Payload: payload}); e != nil {
		t.Fatal(e)
	}
	out, e := s.Read(ctx, sc, "execution", id, "", "", 1, 20)
	if e != nil || !out.(Execution).ArtifactsPending {
		t.Fatal("pending artifacts hidden")
	}
	storage := &artifactMemory{}
	s.WithArtifacts(storage)
	if e = s.Handle(ctx, jobs.Job{AccountID: sc.AccountID, Payload: payload}); e != nil {
		t.Fatal(e)
	}
	out, e = s.Read(ctx, sc, "execution", id, "", "", 1, 20)
	if e != nil {
		t.Fatal(e)
	}
	if out.(Execution).ArtifactsPending || storage.calls != 1 || c.submits != 1 {
		t.Fatal("artifact recovery repeated submission")
	}
}
func TestSameAccountEmittersAndDeleteInvisible(t *testing.T) {
	s, sc, p := fixture(t)
	ctx := context.Background()
	d := draft(t, s, sc)
	var user string
	if e := p.QueryRow(ctx, `insert into core.users default values returning id::text`).Scan(&user); e != nil {
		t.Fatal(e)
	}
	other := Scope{AccountID: sc.AccountID, ActorID: user}
	b, e := s.CreateIssuer(ctx, other, "other-operator-issuer", Command{Profile: &Profile{LegalName: "Second operator"}})
	if e != nil {
		t.Fatal(e)
	}
	other.IssuerID = field(t, b, "id")
	other.IssuerID = sc.IssuerID
	_, e = s.Delete(ctx, other, d.ID, "delete-invisible-key", Command{ExpectedVersion: 1})
	expectCode(t, e, "not_found")
	_, e = s.Delete(ctx, sc, d.ID, "delete-visible-key", Command{ExpectedVersion: 1})
	if e != nil {
		t.Fatal(e)
	}
	_, e = s.Read(ctx, sc, "document", d.ID, "", "", 1, 20)
	expectCode(t, e, "not_found")
}

func TestEditingIdenticalValuesStillInvalidatesReviewedSnapshot(t *testing.T) {
	s, sc, _ := fixture(t)
	ctx := context.Background()
	s.WithConnectors(map[string]Connector{"fake.nfse.v1": &fakeConnector{}})
	d := draft(t, s, sc)
	b, e := s.DocumentCommand(ctx, sc, d.ID, "validate", "same-values-validate1", Command{ExpectedVersion: 1, ConnectorID: "fake.nfse.v1", Environment: "fake_local"})
	if e != nil {
		t.Fatal(e)
	}
	out, e := s.Read(ctx, sc, "review", d.ID, "fake.nfse.v1", "fake_local", 1, 20)
	if e != nil {
		t.Fatal(e)
	}
	oldHash := out.(Review).Hash
	b, e = s.DocumentCommand(ctx, sc, d.ID, "review", "same-values-review1", Command{ExpectedVersion: version(t, b), ExpectedProfileVersion: 1, SnapshotHash: oldHash, Confirmed: true})
	if e != nil {
		t.Fatal(e)
	}
	b, e = s.DocumentCommand(ctx, sc, d.ID, "edit", "same-values-edit", Command{ExpectedVersion: version(t, b), Fields: &d.Fields})
	if e != nil {
		t.Fatal(e)
	}
	b, e = s.DocumentCommand(ctx, sc, d.ID, "validate", "same-values-validate2", Command{ExpectedVersion: version(t, b), ConnectorID: "fake.nfse.v1", Environment: "fake_local"})
	if e != nil {
		t.Fatal(e)
	}
	out, e = s.Read(ctx, sc, "review", d.ID, "fake.nfse.v1", "fake_local", 1, 20)
	if e != nil {
		t.Fatal(e)
	}
	newHash := out.(Review).Hash
	if oldHash == newHash {
		t.Fatal("content revision absent from fingerprint")
	}
	_, e = s.DocumentCommand(ctx, sc, d.ID, "review", "same-values-stale-review", Command{ExpectedVersion: version(t, b), ExpectedProfileVersion: 1, SnapshotHash: oldHash, Confirmed: true})
	expectCode(t, e, "snapshot_stale")
	if _, e = s.DocumentCommand(ctx, sc, d.ID, "review", "same-values-review2", Command{ExpectedVersion: version(t, b), ExpectedProfileVersion: 1, SnapshotHash: newHash, Confirmed: true}); e != nil {
		t.Fatal(e)
	}
}

func TestAuthorizationOutsideIssuerReturnsNotFound(t *testing.T) {
	s, sc, _ := fixture(t)
	_, e := s.RevokeAuthorization(context.Background(), sc, "00000000-0000-4000-8000-000000000099", "revoke-invisible-key", Command{ExpectedVersion: 1})
	expectCode(t, e, "not_found")
}
