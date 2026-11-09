package nfse

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/accounting"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
)

type motorPolicy struct{}

func (motorPolicy) Check(context.Context, accounting.Scope, string) error { return nil }
func (motorPolicy) Member(context.Context, string, string) (bool, error)  { return true, nil }

type storedSynthetic struct {
	account, issuer string
	raw             []byte
}
type syntheticStorage struct {
	pool    *pgxpool.Pool
	mu      sync.Mutex
	objects map[string]storedSynthetic
	keys    map[string]string
	fail    bool
}

func (s *syntheticStorage) PutPrivate(ctx context.Context, scope accounting.Scope, execution string, a accounting.Artifact) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail {
		return "", errors.New("synthetic storage unavailable")
	}
	key := scope.AccountID + ":" + scope.IssuerID + ":" + execution + ":" + a.Kind + ":" + a.SHA256
	if id := s.keys[key]; id != "" {
		return id, nil
	}
	var id string
	if e := s.pool.QueryRow(ctx, `select gen_random_uuid()::text`).Scan(&id); e != nil {
		return "", e
	}
	s.keys[key] = id
	s.objects[id] = storedSynthetic{scope.AccountID, scope.IssuerID, append([]byte{}, a.Content...)}
	return id, nil
}
func (s *syntheticStorage) OpenPrivate(_ context.Context, sc accounting.Scope, id string) (io.ReadCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.objects[id]
	if !ok || value.account != sc.AccountID || value.issuer != sc.IssuerID {
		return nil, errors.New("synthetic not found")
	}
	return io.NopCloser(bytes.NewReader(append([]byte{}, value.raw...))), nil
}
func motorPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	endpoint := os.Getenv("NFSE_TEST_DATABASE_URL")
	if endpoint == "" {
		t.Skip("requires disposable accounting_test NFSE database")
	}
	p, e := pgxpool.New(context.Background(), endpoint)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(p.Close)
	if !strings.HasPrefix(p.Config().ConnConfig.Database, "accounting_test") {
		t.Fatal("refusing non-disposable database")
	}
	if _, e = p.Exec(context.Background(), `create schema if not exists core;create table if not exists core.accounts(id uuid primary key default gen_random_uuid(),name text not null default '');create table if not exists core.users(id uuid primary key default gen_random_uuid())`); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile("../../../../platform/database/migrations/0310_accounting_foundation.sql")
	if e != nil {
		t.Fatal(e)
	}
	for range 2 {
		if _, e = p.Exec(context.Background(), string(b)); e != nil {
			t.Fatal(e)
		}
	}
	if _, e = p.Exec(context.Background(), `create table if not exists accounting.ct08_fixture_manifests(account_id uuid not null,issuer_id uuid not null,document_id uuid not null,content_version bigint not null,manifest jsonb not null,primary key(account_id,issuer_id,document_id,content_version))`); e != nil {
		t.Fatal(e)
	}
	return p
}
func field(t *testing.T, raw []byte, name string) string {
	t.Helper()
	var value map[string]json.RawMessage
	if e := json.Unmarshal(raw, &value); e != nil {
		t.Fatal(e)
	}
	var out string
	if e := json.Unmarshal(value[name], &out); e != nil {
		t.Fatal(e)
	}
	return out
}
func version(t *testing.T, raw []byte) int64 {
	t.Helper()
	var value struct {
		Version int64 `json:"version"`
	}
	if e := json.Unmarshal(raw, &value); e != nil {
		t.Fatal(e)
	}
	return value.Version
}
func motorFixture(t *testing.T, mode string) (*accounting.Service, accounting.Scope, accounting.Document, *Connector, *fakeSEFIN, *syntheticStorage, *pgxpool.Pool) {
	t.Helper()
	p := motorPool(t)
	ctx := context.Background()
	sc := accounting.Scope{}
	if e := p.QueryRow(ctx, `insert into core.accounts default values returning id::text`).Scan(&sc.AccountID); e != nil {
		t.Fatal(e)
	}
	if e := p.QueryRow(ctx, `insert into core.users default values returning id::text`).Scan(&sc.ActorID); e != nil {
		t.Fatal(e)
	}
	queue, e := jobs.NewPostgresStore(p, "accounting.outbox")
	if e != nil {
		t.Fatal(e)
	}
	storage := &syntheticStorage{pool: p, objects: map[string]storedSynthetic{}, keys: map[string]string{}}
	service := accounting.NewService(accounting.NewStore(p), motorPolicy{}, queue).WithArtifacts(storage).WithArtifactReader(storage)
	raw, e := service.CreateIssuer(ctx, sc, "ct08-create-issuer", accounting.Command{EnableIssue: true, Profile: &accounting.Profile{LegalName: "Synthetic CT08", Document: ptr("00000000000000"), TaxRegime: ptr("SYNTHETIC - not a fiscal classification"), MunicipalityCode: ptr("0000000"), State: ptr("ZZ")}})
	if e != nil {
		t.Fatal(e)
	}
	sc.IssuerID = field(t, raw, "id")
	raw, e = service.SaveDiagnosis(ctx, sc, "ct08-diagnosis", accounting.DiagnosisInput{ExpectedProfileVersion: 1, Intent: "service", CompetenceDate: ptr("2026-11-04"), Answers: map[string]accounting.Answer{}})
	if e != nil {
		t.Fatal(e)
	}
	c, _, sub, remote := fixtureConnector(t, mode)
	raw, e = service.Prepare(ctx, sc, "ct08-prepare", accounting.PrepareInput{ExpectedProfileVersion: 1, ExpectedDiagnosisVersion: 1, DiagnosisID: field(t, raw, "id"), Kind: "nfse", Fields: sub.Document.Fields})
	if e != nil {
		t.Fatal(e)
	}
	value, e := service.Read(ctx, sc, "document", field(t, raw, "id"), "", "", 1, 20)
	if e != nil {
		t.Fatal(e)
	}
	doc := value.(accounting.Document)
	remote.doc = doc
	signed := syntheticDPS(doc)
	config, _ := c.ports.Config(ctx, sc)
	config.AccountID = sc.AccountID
	config.IssuerID = sc.IssuerID
	c.ports.Config = func(context.Context, accounting.Scope) (Config, error) { return config, nil }
	manifest := Manifest{AccountID: sc.AccountID, IssuerID: sc.IssuerID, DocumentID: doc.ID, ConfigVersion: config.Version, SchemaVersion: config.SchemaVersion, ContentVersion: doc.ContentVersion, ProfileVersion: doc.ProfileVersion, DPSID: testDPSID(), SHA256: sha(signed), StorageObjectID: testObject, FieldsDigest: FieldsDigest(doc), SchemaReviewed: true, SignatureReviewed: true}
	metadata, e := json.Marshal(manifest)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = p.Exec(ctx, `insert into accounting.ct08_fixture_manifests(account_id,issuer_id,document_id,content_version,manifest)values($1::uuid,$2::uuid,$3::uuid,$4,$5::jsonb)`, sc.AccountID, sc.IssuerID, doc.ID, doc.ContentVersion, metadata); e != nil {
		t.Fatal(e)
	}
	c.ports.Manifest = func(ctx context.Context, scope accounting.Scope, d accounting.Document) (Manifest, error) {
		var raw []byte
		e := p.QueryRow(ctx, `select manifest from accounting.ct08_fixture_manifests where account_id=$1::uuid and issuer_id=$2::uuid and document_id=$3::uuid and content_version=$4`, scope.AccountID, scope.IssuerID, d.ID, d.ContentVersion).Scan(&raw)
		if e != nil {
			return Manifest{}, e
		}
		var m Manifest
		e = json.Unmarshal(raw, &m)
		return m, e
	}
	c.ports.ReadSignedXML = func(_ context.Context, scope accounting.Scope, m Manifest) ([]byte, error) {
		if m.AccountID != scope.AccountID || m.IssuerID != scope.IssuerID {
			return nil, errors.New("scope mismatch")
		}
		return append([]byte{}, signed...), nil
	}
	service.WithConnectors(map[string]accounting.Connector{ID: c})
	return service, sc, doc, c, remote, storage, p
}
func queueDocument(t *testing.T, s *accounting.Service, sc accounting.Scope, d accounting.Document) ([]byte, accounting.Command) {
	t.Helper()
	ctx := context.Background()
	raw, e := s.DocumentCommand(ctx, sc, d.ID, "validate", "ct08-validate", accounting.Command{ExpectedVersion: d.Version, ConnectorID: ID, Environment: "fake_local"})
	if e != nil {
		t.Fatal(e)
	}
	value, e := s.Read(ctx, sc, "review", d.ID, ID, "fake_local", 1, 20)
	if e != nil {
		t.Fatal(e)
	}
	review := value.(accounting.Review)
	raw, e = s.DocumentCommand(ctx, sc, d.ID, "review", "ct08-review", accounting.Command{ExpectedVersion: version(t, raw), ExpectedProfileVersion: d.ProfileVersion, ConnectorID: ID, Environment: "fake_local", SnapshotHash: review.Hash, Confirmed: true})
	if e != nil {
		t.Fatal(e)
	}
	raw, e = s.DocumentCommand(ctx, sc, d.ID, "authorize", "ct08-authorize", accounting.Command{ExpectedVersion: version(t, raw), ReviewID: field(t, raw, "reviewId"), ConnectorID: ID, Environment: "fake_local", SnapshotHash: review.Hash, Confirmed: true})
	if e != nil {
		t.Fatal(e)
	}
	command := accounting.Command{ExpectedVersion: version(t, raw), AuthorizationID: field(t, raw, "authorizationId"), BusinessKey: "ct08-synthetic-business", Operation: "issue"}
	raw, e = s.DocumentCommand(ctx, sc, d.ID, "execute", "ct08-execute", command)
	if e != nil {
		t.Fatal(e)
	}
	return raw, command
}
func TestMotorLocalSEFINReconciliationAndPrivateArtifacts(t *testing.T) {
	for _, mode := range []string{"ok", "reject", "before", "after", "lost"} {
		t.Run(mode, func(t *testing.T) {
			s, sc, d, c, remote, storage, p := motorFixture(t, mode)
			raw, command := queueDocument(t, s, sc, d)
			execution := field(t, raw, "executionId")
			payload, _ := json.Marshal(map[string]string{"executionId": execution})
			job := jobs.Job{AccountID: sc.AccountID, Payload: payload}
			// Same command replay and duplicate job cannot create another POST/effect.
			replay, e := s.DocumentCommand(context.Background(), sc, d.ID, "execute", "ct08-execute", command)
			if e != nil || field(t, raw, "executionId") != field(t, replay, "executionId") {
				t.Fatal("execute replay changed", e)
			}
			var wg sync.WaitGroup
			errs := make(chan error, 2)
			for range 2 {
				wg.Go(func() { errs <- s.Handle(context.Background(), job) })
			}
			wg.Wait()
			for range 2 {
				if e = <-errs; e != nil {
					t.Fatal(e)
				}
			}
			// Reconstruct connector/service against persisted DB manifest and durable state.
			queue, e := jobs.NewPostgresStore(p, "accounting.outbox")
			if e != nil {
				t.Fatal(e)
			}
			restarted := New(c.ports)
			restarted.simulated = true
			restarted.transport = c.transport
			resumed := accounting.NewService(accounting.NewStore(p), motorPolicy{}, queue).WithConnectors(map[string]accounting.Connector{ID: restarted}).WithArtifacts(storage).WithArtifactReader(storage)
			if e = resumed.Handle(context.Background(), job); e != nil {
				t.Fatal(e)
			}
			value, e := resumed.Read(context.Background(), sc, "execution", execution, "", "", 1, 20)
			if e != nil {
				t.Fatal(e)
			}
			state := value.(accounting.Execution)
			expected := "authorized"
			if mode == "reject" {
				expected = "rejected"
			}
			if mode == "before" {
				expected = "unknown"
			}
			if state.State != expected || !state.Simulated {
				t.Fatal("wrong persisted/simulated state", state.State, state.Simulated)
			}
			remote.mu.Lock()
			posts, effects := remote.posts, remote.effects
			remote.mu.Unlock()
			if posts != 1 || effects > 1 {
				t.Fatal("duplicate HTTP submission", posts, effects)
			}
			if expected == "authorized" {
				var artifact string
				if e = p.QueryRow(context.Background(), `select id::text from accounting.artifacts where account_id=$1::uuid and issuer_id=$2::uuid and execution_id=$3::uuid and kind='xml'`, sc.AccountID, sc.IssuerID, execution).Scan(&artifact); e != nil {
					t.Fatal(e)
				}
				downloaded, e := resumed.DownloadArtifact(context.Background(), sc, execution, artifact)
				if e != nil || !bytes.Equal(downloaded.Content, syntheticNFSe(d, remote.key)) || sha(downloaded.Content) != downloaded.SHA256 {
					t.Fatal("private artifact download integrity", e)
				}
				other := sc
				other.IssuerID = testIssuer
				if _, e = resumed.DownloadArtifact(context.Background(), other, execution, artifact); e == nil {
					t.Fatal("cross issuer artifact leak")
				}
				other = sc
				other.AccountID = testAccount
				if _, e = resumed.DownloadArtifact(context.Background(), other, execution, artifact); e == nil {
					t.Fatal("cross account artifact leak")
				}
				if _, e = storage.OpenPrivate(context.Background(), other, testObject); e == nil {
					t.Fatal("storage scope bypass")
				}
			}
		})
	}
}

func TestMotorArtifactRecoveryNeverResubmitsAndChangedProfileCancels(t *testing.T) {
	t.Run("storage_failure_then_recovery", func(t *testing.T) {
		s, sc, d, _, remote, storage, p := motorFixture(t, "ok")
		raw, _ := queueDocument(t, s, sc, d)
		execution := field(t, raw, "executionId")
		job := jobs.Job{AccountID: sc.AccountID, Payload: []byte(`{"executionId":"` + execution + `"}`)}
		storage.fail = true
		if e := s.Handle(context.Background(), job); e != nil {
			t.Fatal(e)
		}
		value, e := s.Read(context.Background(), sc, "execution", execution, "", "", 1, 20)
		if e != nil {
			t.Fatal(e)
		}
		state := value.(accounting.Execution)
		if state.State != "authorized" || !state.ArtifactsPending {
			t.Fatal("storage failure lost fiscal state")
		}
		storage.fail = false
		if e = s.Handle(context.Background(), job); e != nil {
			t.Fatal(e)
		}
		value, e = s.Read(context.Background(), sc, "execution", execution, "", "", 1, 20)
		if e != nil {
			t.Fatal(e)
		}
		state = value.(accounting.Execution)
		if state.ArtifactsPending || remote.posts != 1 {
			t.Fatal("artifact retry resubmitted")
		}
		var object, artifact string
		if e = p.QueryRow(context.Background(), `select id::text,storage_object_id::text from accounting.artifacts where account_id=$1::uuid and issuer_id=$2::uuid and execution_id=$3::uuid and kind='xml'`, sc.AccountID, sc.IssuerID, execution).Scan(&artifact, &object); e != nil {
			t.Fatal(e)
		}
		storage.mu.Lock()
		corrupt := storage.objects[object]
		corrupt.raw = []byte("corrupt")
		storage.objects[object] = corrupt
		storage.mu.Unlock()
		downloaded, e := s.DownloadArtifact(context.Background(), sc, execution, artifact)
		if e == nil || len(downloaded.Content) > 0 {
			t.Fatal("corrupt original exposed")
		}
	})
	t.Run("profile_changed_after_authorization", func(t *testing.T) {
		s, sc, d, _, remote, _, _ := motorFixture(t, "ok")
		raw, _ := queueDocument(t, s, sc, d)
		execution := field(t, raw, "executionId")
		if _, e := s.UpdateIssuer(context.Background(), sc, "ct08-profile-changed", accounting.Command{ExpectedVersion: 1, Profile: &accounting.Profile{LegalName: "Synthetic changed"}}); e != nil {
			t.Fatal(e)
		}
		if e := s.Handle(context.Background(), jobs.Job{AccountID: sc.AccountID, Payload: []byte(`{"executionId":"` + execution + `"}`)}); e != nil {
			t.Fatal(e)
		}
		value, e := s.Read(context.Background(), sc, "execution", execution, "", "", 1, 20)
		if e != nil {
			t.Fatal(e)
		}
		if value.(accounting.Execution).State != "cancelled" || remote.posts != 0 {
			t.Fatal("changed profile was submitted")
		}
	})
}
