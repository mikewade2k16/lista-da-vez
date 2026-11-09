// Package nfse implements the guarded national SEFIN transport adapter. It is
// deliberately not registered in production: pilot coverage, vault certificate,
// official schema/signature validators and homologation are required first.
package nfse

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/accounting"
	"net/http"
	"regexp"
	"time"
)

const ID = "nfse.sefin_nacional.v1"
const Namespace = "http://www.sped.fazenda.gov.br/nfse"
const MaxXMLBytes = 1024 * 1024
const maxResponseBytes = 2 * 1024 * 1024

var digits7 = regexp.MustCompile(`^[0-9]{7}$`)
var federalDocument = regexp.MustCompile(`^([0-9]{11}|[0-9]{14})$`)
var dpsIDPattern = regexp.MustCompile(`^DPS[0-9]{42}$`)
var accessKeyPattern = regexp.MustCompile(`^[0-9]{50}$`)
var checksumPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// Config is read only from an authoritative, scoped server adapter. No URL,
// password, private certificate or enablement is accepted from chat/doc payload.
type Config struct {
	AccountID, IssuerID, Version, Environment                        string
	IssuerDocument, MunicipalityCode, SchemaVersion                  string
	CredentialRef, CoverageRef, WireContractRef, TaxRegimeRef        string
	Enabled, HomologationAuthorized, Homologated, ProductionApproved bool
}

// Manifest is a durable preparation result, stored BEFORE authorization/queueing.
// It contains no XML or secret. Reader must be local, immutable and account-scoped.
// FieldsDigest binds the existing document fields/profile/content versions.
type Manifest struct {
	AccountID, IssuerID, DocumentID, ConfigVersion, SchemaVersion string
	ContentVersion, ProfileVersion                                int64
	DPSID, SHA256, StorageObjectID, FieldsDigest                  string
	SchemaReviewed, SignatureReviewed                             bool
}
type Credential struct {
	Certificate   tls.Certificate `json:"-"`
	Roots         *x509.CertPool  `json:"-"`
	OwnerDocument string          `json:"-"`
}
type Ports struct {
	Config        func(context.Context, accounting.Scope) (Config, error)
	Manifest      func(context.Context, accounting.Scope, accounting.Document) (Manifest, error)
	Credential    func(context.Context, accounting.Scope, string) (Credential, error)
	ReadSignedXML func(context.Context, accounting.Scope, Manifest) ([]byte, error)
	// Validation is offline. Validators must enforce the reviewed official version
	// and signature, with no external entity resolution/network access.
	ValidateSchema  func(context.Context, string, string, []byte) error
	VerifySignature func(context.Context, string, []byte) error
}
type Connector struct {
	ports     Ports
	transport http.RoundTripper
	now       func() time.Time
	simulated bool
}

func New(ports Ports) *Connector { return &Connector{ports: ports, now: time.Now} }

var _ accounting.Connector = (*Connector)(nil)

type Error struct{ Code string }

func (e *Error) Error() string  { return "nfse: " + e.Code }
func failure(code string) error { return &Error{Code: code} }
func safeCode(e error) string {
	var v *Error
	if errors.As(e, &v) {
		return v.Code
	}
	return "capability_unavailable"
}
func baseURL(environment string) (string, error) {
	switch environment {
	case "homologation":
		return "https://sefin.producaorestrita.nfse.gov.br/API/SefinNacional", nil
	case "production":
		return "https://sefin.nfse.gov.br/SefinNacional", nil
	default:
		return "", failure("environment_unavailable")
	}
}
