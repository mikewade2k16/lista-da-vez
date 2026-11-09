package nfse

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/accounting"
	"strings"
)

// FieldsDigest is not an authorization. The motor still owns review/snapshot/grants.
func FieldsDigest(d accounting.Document) string {
	raw, _ := json.Marshal(struct {
		Fields                         accounting.Fields
		ContentVersion, ProfileVersion int64
	}{d.Fields, d.ContentVersion, d.ProfileVersion})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func (c *Connector) configuration(ctx context.Context, sc accounting.Scope) (Config, Credential, error) {
	var cfg Config
	var credential Credential
	if !uuidPattern.MatchString(sc.AccountID) || !uuidPattern.MatchString(sc.IssuerID) || c.ports.Config == nil || c.ports.Credential == nil {
		return cfg, credential, failure("configuration_missing")
	}
	value, e := c.ports.Config(ctx, sc)
	if e != nil {
		return cfg, credential, failure("configuration_unavailable")
	}
	cfg = value
	if cfg.AccountID != sc.AccountID || cfg.IssuerID != sc.IssuerID {
		return cfg, credential, failure("not_found")
	}
	if !cfg.Enabled || cfg.Version == "" || cfg.SchemaVersion != "1.01" || !federalDocument.MatchString(cfg.IssuerDocument) || !digits7.MatchString(cfg.MunicipalityCode) || cfg.CredentialRef == "" || cfg.CoverageRef == "" || cfg.WireContractRef == "" || cfg.TaxRegimeRef == "" {
		return cfg, credential, failure("coverage_unavailable")
	}
	environment := cfg.Environment
	if c.simulated && environment == "fake_local" {
		environment = "homologation"
	}
	if _, e = baseURL(environment); e != nil {
		return cfg, credential, e
	}
	credential, e = c.ports.Credential(ctx, sc, cfg.CredentialRef)
	if e != nil {
		return cfg, credential, failure("certificate_unavailable")
	}
	if credential.OwnerDocument != cfg.IssuerDocument || len(credential.Certificate.Certificate) == 0 || credential.Certificate.PrivateKey == nil {
		return cfg, credential, failure("certificate_unavailable")
	}
	leaf, e := x509.ParseCertificate(credential.Certificate.Certificate[0])
	if e != nil {
		return cfg, credential, failure("certificate_invalid")
	}
	now := c.now()
	if now.Before(leaf.NotBefore) || !now.Before(leaf.NotAfter) {
		return cfg, credential, failure("certificate_expired")
	}
	credential.Certificate.Leaf = leaf
	return cfg, credential, nil
}
func (c *Connector) Capability(ctx context.Context, sc accounting.Scope) (accounting.Capability, error) {
	out := accounting.Capability{Kind: "nfse"}
	cfg, _, e := c.configuration(ctx, sc)
	out.Environment = cfg.Environment
	if e != nil {
		return out, nil
	}
	ready := c.ports.Manifest != nil && c.ports.ReadSignedXML != nil && c.ports.ValidateSchema != nil && c.ports.VerifySignature != nil
	out.CanLookup = ready
	out.CanSubmit = ready && ((cfg.Environment == "production" && cfg.Homologated && cfg.ProductionApproved) || (cfg.Environment != "production" && cfg.HomologationAuthorized))
	return out, nil
}
func (c *Connector) prepared(ctx context.Context, sc accounting.Scope, cfg Config, d accounting.Document) (Manifest, error) {
	var m Manifest
	if d.Kind != "nfse" || d.IssuerID != sc.IssuerID || !uuidPattern.MatchString(d.ID) || d.ContentVersion < 1 || c.ports.Manifest == nil {
		return m, failure("document_scope_invalid")
	}
	m, e := c.ports.Manifest(ctx, sc, d)
	if e != nil {
		return m, failure("prepared_dps_missing")
	}
	if m.AccountID != sc.AccountID || m.IssuerID != sc.IssuerID || m.DocumentID != d.ID {
		return m, failure("not_found")
	}
	if m.ContentVersion != d.ContentVersion || m.ProfileVersion != d.ProfileVersion || m.ConfigVersion != cfg.Version || m.SchemaVersion != cfg.SchemaVersion || m.FieldsDigest != FieldsDigest(d) {
		return m, failure("prepared_dps_stale")
	}
	if !m.SchemaReviewed || !m.SignatureReviewed || !checksumPattern.MatchString(m.SHA256) || !uuidPattern.MatchString(m.StorageObjectID) || !dpsIDPattern.MatchString(m.DPSID) {
		return m, failure("prepared_dps_unreviewed")
	}
	// Identity must be derived from confirmed issuer+municipality and persisted series/number.
	doc := cfg.IssuerDocument
	kind := "2"
	if len(doc) == 11 {
		kind = "1"
		doc = "000" + doc
	}
	if !strings.HasPrefix(m.DPSID, "DPS"+cfg.MunicipalityCode+kind+doc) {
		return m, failure("dps_identity_mismatch")
	}
	return m, nil
}
func (c *Connector) Validate(ctx context.Context, sc accounting.Scope, sub accounting.Submission) ([]string, error) {
	cfg, _, e := c.configuration(ctx, sc)
	if e != nil {
		return []string{safeCode(e)}, nil
	}
	if cfg.Environment != sub.Environment {
		return []string{"environment_mismatch"}, nil
	}
	pending := []string{}
	f := sub.Document.Fields
	if f.Service == nil || f.Service.Description == nil || strings.TrimSpace(*f.Service.Description) == "" {
		pending = append(pending, "service.description")
	}
	if f.Service == nil || f.Service.ServiceCode == nil || strings.TrimSpace(*f.Service.ServiceCode) == "" {
		pending = append(pending, "service.serviceCode")
	}
	if f.Customer == nil || f.Customer.Document == nil || !federalDocument.MatchString(*f.Customer.Document) {
		pending = append(pending, "customer.document")
	}
	if f.CompetenceDate == nil {
		pending = append(pending, "competenceDate")
	}
	if f.Amount == nil || !xmlMoney.MatchString(*f.Amount) || *f.Amount == "0.00" {
		pending = append(pending, "positiveAmount")
	}
	if len(pending) > 0 {
		return pending, nil
	}
	if _, e = c.prepared(ctx, sc, cfg, sub.Document); e != nil {
		return []string{safeCode(e)}, nil
	}
	return []string{}, nil
}
