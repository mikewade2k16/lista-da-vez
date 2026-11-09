package nfse

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"fmt"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/accounting"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

const testAccount = "00000000-0000-4000-8000-000000000001"
const testIssuer = "00000000-0000-4000-8000-000000000002"
const testDocument = "00000000-0000-4000-8000-000000000003"
const testExecution = "00000000-0000-4000-8000-000000000004"
const testObject = "00000000-0000-4000-8000-000000000005"

func ptr(v string) *string { return &v }
func testDPSID() string {
	return "DPS" + "0000000" + "2" + "00000000000000" + "00001" + "000000000000001"
}
func syntheticDPS(d accounting.Document) []byte {
	return []byte(fmt.Sprintf(`<DPS xmlns="%s" versao="1.01"><infDPS Id="%s"><tpAmb>2</tpAmb><dCompet>%s</dCompet><cLocEmi>0000000</cLocEmi><prest><CNPJ>00000000000000</CNPJ></prest><toma><CPF>00000000000</CPF></toma><serv><cServ><cTribNac>010101</cTribNac><xDescServ>%s</xDescServ></cServ></serv><valores><vServPrest><vServ>%s</vServ></vServPrest></valores></infDPS></DPS>`, Namespace, testDPSID(), *d.Fields.CompetenceDate, *d.Fields.Service.Description, *d.Fields.Amount))
}
func syntheticNFSe(d accounting.Document, key string) []byte {
	dps := syntheticDPS(d)
	return []byte(`<NFSe xmlns="` + Namespace + `" versao="1.01"><infNFSe Id="NFS` + key + `">` + string(dps) + `</infNFSe></NFSe>`)
}
func testCredential(t *testing.T) Credential {
	t.Helper()
	pub, key, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Synthetic CT08, not ICP-Brasil"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}
	cert, e := x509.CreateCertificate(rand.Reader, template, template, pub, key)
	if e != nil {
		t.Fatal(e)
	}
	return Credential{OwnerDocument: "00000000000000", Certificate: tls.Certificate{Certificate: [][]byte{cert}, PrivateKey: key}}
}

type fakeSEFIN struct {
	mu                      sync.Mutex
	mode                    string
	posts, lookups, effects int
	accepted                bool
	doc                     accounting.Document
	key                     string
	dpsID                   string
}

func (f *fakeSEFIN) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/nfse") {
		f.posts++
		var body struct {
			XML string `json:"dpsXmlGZipB64"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			w.WriteHeader(400)
			return
		}
		raw, e := expandXML(body.XML)
		if e != nil || sha(raw) != sha(syntheticDPS(f.doc)) {
			w.WriteHeader(400)
			return
		}
		switch f.mode {
		case "reject":
			w.WriteHeader(422)
			_, _ = io.WriteString(w, `{"erros":[{"Codigo":"SYNTHETIC_REJECTION","Descricao":"Never persist raw provider error"}]}`)
			return
		case "before":
			w.WriteHeader(503)
			_, _ = io.WriteString(w, `{}`)
			return
		}
		if !f.accepted {
			f.accepted = true
			f.effects++
		}
		switch f.mode {
		case "after":
			w.WriteHeader(503)
			_, _ = io.WriteString(w, `{}`)
			return
		case "lost":
			_, _ = io.WriteString(w, `{"partial":`)
			return
		case "empty200":
			_, _ = io.WriteString(w, `{}`)
			return
		}
		f.writeNFSe(w)
		return
	}
	if r.Method == "GET" && strings.Contains(r.URL.Path, "/dps/") {
		f.lookups++
		if !f.accepted || !strings.HasSuffix(r.URL.Path, "/"+f.dpsID) {
			w.WriteHeader(404)
			_, _ = io.WriteString(w, `{}`)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"chaveAcesso": f.key})
		return
	}
	if r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/nfse/"+f.key) && f.accepted {
		f.writeNFSe(w)
		return
	}
	w.WriteHeader(404)
}
func (f *fakeSEFIN) writeNFSe(w http.ResponseWriter) {
	xml := syntheticNFSe(f.doc, f.key)
	if f.mode == "wrong_key" {
		xml = syntheticNFSe(f.doc, strings.Repeat("2", 50))
	}
	encoded, _ := compressXML(xml)
	_ = json.NewEncoder(w).Encode(map[string]string{"idDps": f.dpsID, "chaveAcesso": f.key, "nfseXmlGZipB64": encoded})
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// No production test constructor exists. Only this _test helper activates fake_local.
func fixtureConnector(t *testing.T, mode string) (*Connector, accounting.Scope, accounting.Submission, *fakeSEFIN) {
	t.Helper()
	sc := accounting.Scope{AccountID: testAccount, IssuerID: testIssuer, ActorID: testAccount}
	d := accounting.Document{ID: testDocument, IssuerID: testIssuer, Kind: "nfse", ContentVersion: 1, ProfileVersion: 1, State: "submitting", Fields: accounting.Fields{CompetenceDate: ptr("2026-11-04"), Amount: ptr("125.00"), Service: &accounting.ServiceFields{Description: ptr("Synthetic service"), ServiceCode: ptr("010101")}, Customer: &accounting.Party{Name: ptr("Synthetic customer"), Document: ptr("00000000000")}}}
	credential := testCredential(t)
	raw := syntheticDPS(d)
	manifest := Manifest{AccountID: sc.AccountID, IssuerID: sc.IssuerID, DocumentID: d.ID, ConfigVersion: "synthetic-v1", SchemaVersion: "1.01", ContentVersion: 1, ProfileVersion: 1, DPSID: testDPSID(), SHA256: sha(raw), StorageObjectID: testObject, FieldsDigest: FieldsDigest(d), SchemaReviewed: true, SignatureReviewed: true}
	cfg := Config{AccountID: sc.AccountID, IssuerID: sc.IssuerID, Version: "synthetic-v1", Environment: "fake_local", IssuerDocument: "00000000000000", MunicipalityCode: "0000000", SchemaVersion: "1.01", CredentialRef: "synthetic-vault-ref", CoverageRef: "synthetic-only", TaxRegimeRef: "synthetic-only", WireContractRef: "synthetic-wire-not-qualified", Enabled: true, HomologationAuthorized: true, Homologated: true}
	c := New(Ports{Config: func(context.Context, accounting.Scope) (Config, error) { return cfg, nil }, Credential: func(context.Context, accounting.Scope, string) (Credential, error) { return credential, nil }, Manifest: func(context.Context, accounting.Scope, accounting.Document) (Manifest, error) { return manifest, nil }, ReadSignedXML: func(context.Context, accounting.Scope, Manifest) ([]byte, error) { return raw, nil }, ValidateSchema: func(_ context.Context, _, _ string, xml []byte) error { _, e := inspectXML(xml); return e }, VerifySignature: func(context.Context, string, []byte) error { return nil }})
	c.simulated = true
	fake := &fakeSEFIN{mode: mode, doc: d, key: strings.Repeat("1", 50), dpsID: manifest.DPSID}
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	target, e := url.Parse(server.URL)
	if e != nil {
		t.Fatal(e)
	}
	c.transport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "sefin.producaorestrita.nfse.gov.br" {
			t.Fatal("unexpected outbound host")
		}
		clone := r.Clone(r.Context())
		endpoint := *r.URL
		endpoint.Scheme = target.Scheme
		endpoint.Host = target.Host
		clone.URL = &endpoint
		clone.Host = target.Host
		return server.Client().Transport.RoundTrip(clone)
	})
	return c, sc, accounting.Submission{ExecutionID: testExecution, RemoteIdentity: testExecution, Environment: "fake_local", SnapshotHash: strings.Repeat("a", 64), Document: d}, fake
}
func TestSEFINLocalProtocolOutcomesAndReconciliation(t *testing.T) {
	for _, mode := range []string{"ok", "reject", "before", "after", "lost", "empty200", "wrong_key"} {
		t.Run(mode, func(t *testing.T) {
			c, sc, sub, f := fixtureConnector(t, mode)
			out, e := c.Submit(context.Background(), sc, sub)
			switch mode {
			case "ok":
				if e != nil || out.State != "authorized" {
					t.Fatal("authorized response not validated", e, out)
				}
			case "reject":
				if e != nil || out.State != "rejected" {
					t.Fatal("business rejection not parsed", e, out)
				}
			default:
				if out.State != "unknown" {
					t.Fatal("uncertain response became definitive", e, out)
				}
			}
			if mode == "after" || mode == "lost" || mode == "empty200" {
				out, e = c.Lookup(context.Background(), sc, sub)
				if e != nil || out.State != "authorized" {
					t.Fatal("lost response not recovered", e, out)
				}
			}
			if mode == "before" {
				out, _ = c.Lookup(context.Background(), sc, sub)
				if out.State != "unknown" {
					t.Fatal("absence treated as safe resubmit")
				}
			}
			if mode == "ok" {
				artifacts, e := c.FetchArtifacts(context.Background(), sc, sub)
				if e != nil || len(artifacts) != 2 || artifacts[0].Kind != "xml" || sha(artifacts[0].Content) != artifacts[0].SHA256 {
					t.Fatal("artifact integrity", e)
				}
			}
			if f.posts != 1 || f.effects > 1 {
				t.Fatal("unexpected retry")
			}
		})
	}
}

func TestSEFINBlocksUnqualifiedConfigAndChangedDocument(t *testing.T) {
	cases := []struct {
		name   string
		change func(*Connector, *accounting.Submission)
	}{
		{"configuration_missing", func(c *Connector, _ *accounting.Submission) { c.ports.Config = nil }},
		{"certificate_missing", func(c *Connector, _ *accounting.Submission) { c.ports.Credential = nil }},
		{"certificate_expired", func(c *Connector, _ *accounting.Submission) {
			c.now = func() time.Time { return time.Now().Add(2 * time.Hour) }
		}},
		{"validator_missing", func(c *Connector, _ *accounting.Submission) { c.ports.ValidateSchema = nil }},
		{"signature_missing", func(c *Connector, _ *accounting.Submission) { c.ports.VerifySignature = nil }},
		{"schema_invalid", func(c *Connector, _ *accounting.Submission) {
			c.ports.ValidateSchema = func(context.Context, string, string, []byte) error { return failure("synthetic_invalid") }
		}},
		{"signature_invalid", func(c *Connector, _ *accounting.Submission) {
			c.ports.VerifySignature = func(context.Context, string, []byte) error { return failure("synthetic_invalid") }
		}},
		{"draft_changed", func(_ *Connector, s *accounting.Submission) { s.Document.ContentVersion++ }},
		{"profile_changed", func(_ *Connector, s *accounting.Submission) { s.Document.ProfileVersion++ }},
		{"amount_changed", func(_ *Connector, s *accounting.Submission) { s.Document.Fields.Amount = ptr("126.00") }},
		{"unclaimed_motor", func(_ *Connector, s *accounting.Submission) { s.Document.State = "reviewed" }},
		{"identity_not_persisted", func(_ *Connector, s *accounting.Submission) { s.RemoteIdentity = "" }},
		{"environment_mismatch", func(_ *Connector, s *accounting.Submission) { s.Environment = "production" }},
		{"corrupted_storage", func(c *Connector, _ *accounting.Submission) {
			c.ports.ReadSignedXML = func(context.Context, accounting.Scope, Manifest) ([]byte, error) { return []byte("corrupted"), nil }
		}},
		{"no_live_simulation", func(c *Connector, _ *accounting.Submission) { c.simulated = false }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, sc, sub, f := fixtureConnector(t, "ok")
			tc.change(c, &sub)
			out, e := c.Submit(context.Background(), sc, sub)
			if e == nil || out.State != "unknown" || f.posts != 0 {
				t.Fatal("unsafe submission", out, e, f.posts)
			}
		})
	}
	c, sc, sub, f := fixtureConnector(t, "ok")
	sc.AccountID = "00000000-0000-4000-8000-000000000099"
	if _, e := c.Submit(context.Background(), sc, sub); e == nil || f.posts != 0 {
		t.Fatal("cross-account submission")
	}
	c, sc, sub, f = fixtureConnector(t, "ok")
	sc.IssuerID = "00000000-0000-4000-8000-000000000099"
	if _, e := c.Submit(context.Background(), sc, sub); e == nil || f.posts != 0 {
		t.Fatal("cross-issuer submission")
	}
}
func TestXMLDefensesAndMissingFields(t *testing.T) {
	c, sc, sub, f := fixtureConnector(t, "ok")
	sub.Document.Fields.Service.ServiceCode = nil
	pending, e := c.Validate(context.Background(), sc, sub)
	if e != nil || len(pending) == 0 || pending[0] != "service.serviceCode" || f.posts != 0 {
		t.Fatal("missing code was inferred", pending, e)
	}
	valid := syntheticDPS(f.doc)
	for _, raw := range [][]byte{[]byte(`<!DOCTYPE DPS [<!ENTITY remote SYSTEM "file:///etc/passwd">]>` + string(valid)), []byte(strings.Replace(string(valid), Namespace, "https://fake.invalid", 1)), []byte(strings.Replace(string(valid), "<vServ>125.00</vServ>", "<vServ>125.00</vServ><vServ>125.00</vServ>", 1)), []byte(string(valid) + string(valid)), []byte("not xml")} {
		if _, e = inspectXML(raw); e == nil {
			t.Fatal("unsafe XML accepted")
		}
	}
	encoded, e := compressXML([]byte(strings.Repeat("A", MaxXMLBytes+1)))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = expandXML(encoded); e == nil {
		t.Fatal("gzip bomb accepted")
	}
	encoded, e = compressXML(valid)
	if e != nil {
		t.Fatal(e)
	}
	decoded, e := expandXML(encoded)
	if e != nil || sha(decoded) != sha(valid) {
		t.Fatal("original changed in compression", e)
	}
}
func TestUnavailableAdapterDoesNotEnableProduct(t *testing.T) {
	cap, e := New(Ports{}).Capability(context.Background(), accounting.Scope{AccountID: testAccount, IssuerID: testIssuer})
	if e != nil || cap.CanSubmit || cap.CanLookup || cap.Kind != "nfse" {
		t.Fatal("unconfigured connector enabled", cap, e)
	}
}

func TestTransportTimeoutBeforeAndAfterAcceptanceNeverRetriesPOST(t *testing.T) {
	for _, after := range []bool{false, true} {
		t.Run(fmt.Sprint(after), func(t *testing.T) {
			c, sc, sub, f := fixtureConnector(t, "ok")
			original := c.transport
			c.transport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				if r.Method != "POST" {
					return original.RoundTrip(r)
				}
				if after {
					response, e := original.RoundTrip(r)
					if e != nil {
						return nil, e
					}
					_ = response.Body.Close()
				}
				return nil, context.DeadlineExceeded
			})
			out, e := c.Submit(context.Background(), sc, sub)
			if e == nil || out.State != "unknown" {
				t.Fatal("timeout definitive", out, e)
			}
			out, e = c.Lookup(context.Background(), sc, sub)
			if after {
				if e != nil || out.State != "authorized" || f.posts != 1 || f.effects != 1 {
					t.Fatal("acceptance not recovered", out, e)
				}
			} else if out.State != "unknown" || f.posts != 0 || f.effects != 0 {
				t.Fatal("before timeout resubmitted", out, e)
			}
		})
	}
}

func TestHomologationPermissionAndProductionAreSeparate(t *testing.T) {
	for _, tc := range []struct {
		name, env                                     string
		permission, homologated, production, expected bool
	}{
		{"homologation_without_authorization", "homologation", false, false, false, false},
		{"homologation_explicitly_authorized", "homologation", true, false, false, true},
		{"production_without_homologation", "production", true, false, true, false},
		{"production_without_approval", "production", true, true, false, false},
		{"production_requires_both", "production", false, true, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, sc, _, remote := fixtureConnector(t, "ok")
			cfg, e := c.ports.Config(context.Background(), sc)
			if e != nil {
				t.Fatal(e)
			}
			cfg.Environment = tc.env
			cfg.HomologationAuthorized = tc.permission
			cfg.Homologated = tc.homologated
			cfg.ProductionApproved = tc.production
			c.ports.Config = func(context.Context, accounting.Scope) (Config, error) { return cfg, nil }
			cap, e := c.Capability(context.Background(), sc)
			if e != nil || cap.CanSubmit != tc.expected || remote.posts != 0 {
				t.Fatal("environment policy mismatch", cap, e)
			}
		})
	}
}

func TestCertificateMaterialCannotBeSerialized(t *testing.T) {
	raw, e := json.Marshal(testCredential(t))
	if e != nil || string(raw) != "{}" {
		t.Fatal("certificate material exposed by serialization")
	}
}
