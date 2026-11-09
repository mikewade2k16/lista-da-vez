package accounting

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
)

type testPolicy struct {
	disabled bool
	deny     string
}

func (p *testPolicy) Check(_ context.Context, _ Scope, perm string) error {
	if p.disabled {
		return failure("module_disabled", 403)
	}
	if p.deny == perm {
		return failure("permission_denied", 403)
	}
	return nil
}
func (p *testPolicy) Member(context.Context, string, string) (bool, error) { return true, nil }
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("ACCOUNTING_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("ACCOUNTING_TEST_DATABASE_URL requires disposable accounting_test database")
	}
	p, e := pgxpool.New(context.Background(), url)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(p.Close)
	if !strings.HasPrefix(p.Config().ConnConfig.Database, "accounting_test") {
		t.Fatal("refusing non-disposable database")
	}
	_, e = p.Exec(context.Background(), `create schema if not exists core;create table if not exists core.accounts(id uuid primary key default gen_random_uuid());create table if not exists core.users(id uuid primary key default gen_random_uuid())`)
	if e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile("../../platform/database/migrations/0310_accounting_foundation.sql")
	if e != nil {
		t.Fatal(e)
	}
	for range 2 {
		if _, e = p.Exec(context.Background(), string(b)); e != nil {
			t.Fatal(e)
		}
	}
	return p
}
func fixture(t *testing.T) (*Service, Scope, *pgxpool.Pool) {
	t.Helper()
	p := testPool(t)
	ctx := context.Background()
	sc := Scope{}
	if e := p.QueryRow(ctx, `insert into core.accounts default values returning id::text`).Scan(&sc.AccountID); e != nil {
		t.Fatal(e)
	}
	if e := p.QueryRow(ctx, `insert into core.users default values returning id::text`).Scan(&sc.ActorID); e != nil {
		t.Fatal(e)
	}
	q, e := jobs.NewPostgresStore(p, "accounting.outbox")
	if e != nil {
		t.Fatal(e)
	}
	s := NewService(NewStore(p), &testPolicy{}, q)
	data, e := s.CreateIssuer(ctx, sc, "issuer-key-001", Command{EnableIssue: true, Profile: &Profile{LegalName: "Synthetic", Document: ptr("SYNTHETIC"), MunicipalityCode: ptr("0000000"), State: ptr("ZZ"), TaxRegime: ptr("synthetic")}})
	if e != nil {
		t.Fatal(e)
	}
	sc.IssuerID = field(t, data, "id")
	return s, sc, p
}
func ptr(s string) *string { return &s }
func field(t *testing.T, b []byte, key string) string {
	t.Helper()
	var m map[string]any
	if e := json.Unmarshal(b, &m); e != nil {
		t.Fatal(e)
	}
	v, ok := m[key].(string)
	if !ok {
		t.Fatalf("missing field %s: %s", key, b)
	}
	return v
}
func version(t *testing.T, b []byte) int64 {
	t.Helper()
	var m struct {
		Version int64 `json:"version"`
	}
	if e := json.Unmarshal(b, &m); e != nil {
		t.Fatal(e)
	}
	return m.Version
}
func expectCode(t *testing.T, e error, code string) {
	t.Helper()
	var d *Error
	if !errors.As(e, &d) || d.Code != code {
		t.Fatalf("wanted %s, got %v", code, e)
	}
}
func draft(t *testing.T, s *Service, sc Scope) Document { return draftForKind(t, s, sc, "nfse") }
func draftForKind(t *testing.T, s *Service, sc Scope, kind string) Document {
	t.Helper()
	ctx := context.Background()
	intent := "service"
	if kind == "nfe" {
		intent = "product"
	}
	b, e := s.SaveDiagnosis(ctx, sc, "diagnosis-key-001", DiagnosisInput{ExpectedProfileVersion: 1, Intent: intent, CompetenceDate: ptr("2026-11-03"), Answers: map[string]Answer{}})
	if e != nil {
		t.Fatal(e)
	}
	fields := Fields{CompetenceDate: ptr("2026-11-03"), Amount: ptr("125.00"), Service: &ServiceFields{Description: ptr("Synthetic service")}, Customer: &Party{Name: ptr("Synthetic recipient"), Document: ptr("SYNTHETIC")}}
	if kind == "nfe" {
		fields.Service = nil
		fields.Customer = nil
		fields.Items = []Item{{Description: "Synthetic item", Unit: "synthetic", Quantity: "1", UnitPrice: "125.00", NCM: ptr("synthetic"), CFOP: ptr("synthetic"), CST: ptr("synthetic")}}
		fields.Operation = &Operation{Nature: "synthetic", Purpose: "synthetic", OriginState: "ZZ", DestinationState: "ZZ"}
		fields.Recipient = &Party{Name: ptr("Synthetic recipient"), Document: ptr("SYNTHETIC")}
	}
	b, e = s.Prepare(ctx, sc, "prepare-key-001", PrepareInput{DiagnosisID: field(t, b, "id"), ExpectedDiagnosisVersion: 1, ExpectedProfileVersion: 1, Kind: kind, Fields: fields})
	if e != nil {
		t.Fatal(e)
	}
	out, e := s.Read(ctx, sc, "document", field(t, b, "id"), "", "", 1, 20)
	if e != nil {
		t.Fatal(e)
	}
	return out.(Document)
}

type fakeConnector struct {
	mu                        sync.Mutex
	mode                      string
	kind                      string
	submits, lookups, effects int
	accepted                  bool
}

func (f *fakeConnector) Capability(context.Context, Scope) (Capability, error) {
	kind := f.kind
	if kind == "" {
		kind = "nfse"
	}
	return Capability{CanSubmit: true, CanLookup: true, Kind: kind, Environment: "fake_local"}, nil
}
func (f *fakeConnector) Validate(context.Context, Scope, Submission) ([]string, error) {
	return nil, nil
}
func (f *fakeConnector) Submit(context.Context, Scope, Submission) (Outcome, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.submits++
	switch f.mode {
	case "reject":
		return Outcome{State: "rejected"}, nil
	case "timeout_before":
		return Outcome{}, context.DeadlineExceeded
	case "timeout_after":
		f.effects++
		f.accepted = true
		return Outcome{}, context.DeadlineExceeded
	default:
		f.effects++
		f.accepted = true
		return Outcome{State: "authorized", RemoteReference: ptr("synthetic-ref"), Protocol: ptr("synthetic-protocol")}, nil
	}
}
func (f *fakeConnector) Lookup(context.Context, Scope, Submission) (Outcome, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lookups++
	if f.accepted {
		return Outcome{State: "authorized", RemoteReference: ptr("synthetic-ref"), Protocol: ptr("synthetic-protocol")}, nil
	}
	return Outcome{State: "not_submitted"}, nil
}
func (f *fakeConnector) FetchArtifacts(context.Context, Scope, Submission) ([]Artifact, error) {
	return []Artifact{{Kind: "xml", Content: []byte("<synthetic/>")}}, nil
}
func queued(t *testing.T, s *Service, sc Scope, c *fakeConnector) (Document, json.RawMessage) {
	return queuedForKind(t, s, sc, c, "nfse")
}
func queuedForKind(t *testing.T, s *Service, sc Scope, c *fakeConnector, kind string) (Document, json.RawMessage) {
	t.Helper()
	ctx := context.Background()
	connector := "fake." + kind + ".v1"
	c.kind = kind
	s.WithConnectors(map[string]Connector{connector: c})
	d := draftForKind(t, s, sc, kind)
	b, e := s.DocumentCommand(ctx, sc, d.ID, "validate", "validate-key-001", Command{ExpectedVersion: 1, ConnectorID: connector, Environment: "fake_local"})
	if e != nil {
		t.Fatal(e)
	}
	d.Version = version(t, b)
	out, e := s.Read(ctx, sc, "review", d.ID, connector, "fake_local", 1, 20)
	if e != nil {
		t.Fatal(e)
	}
	r := out.(Review)
	b, e = s.DocumentCommand(ctx, sc, d.ID, "review", "review-key-001", Command{ExpectedVersion: d.Version, ExpectedProfileVersion: 1, ConnectorID: connector, Environment: "fake_local", SnapshotHash: r.Hash, Confirmed: true})
	if e != nil {
		t.Fatal(e)
	}
	b, e = s.DocumentCommand(ctx, sc, d.ID, "authorize", "authorize-key-001", Command{ExpectedVersion: version(t, b), ReviewID: field(t, b, "reviewId"), ConnectorID: connector, Environment: "fake_local", SnapshotHash: r.Hash, Confirmed: true})
	if e != nil {
		t.Fatal(e)
	}
	d.Version = version(t, b)
	in := Command{ExpectedVersion: d.Version, AuthorizationID: field(t, b, "authorizationId"), BusinessKey: "synthetic-business-key-001", Operation: "issue"}
	b, e = s.DocumentCommand(ctx, sc, d.ID, "execute", "execute-key-001", in)
	if e != nil {
		t.Fatal(e)
	}
	return d, b
}

type failedQueue struct{}

func (failedQueue) EnqueueTx(context.Context, pgx.Tx, jobs.NewJob) (string, bool, error) {
	return "", false, errors.New("synthetic queue failure")
}
