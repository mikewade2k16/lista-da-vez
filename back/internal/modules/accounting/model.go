package accounting

import (
	"context"
	"encoding/json"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
)

type Scope struct{ AccountID, ActorID, IssuerID, CorrelationID string }
type Error struct {
	Code   string
	Status int
}

func (e *Error) Error() string              { return e.Code }
func failure(code string, status int) error { return &Error{code, status} }

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func validID(id string) bool { return uuidPattern.MatchString(id) }

type Profile struct {
	LegalName             string   `json:"legalName"`
	Document              *string  `json:"document"`
	IssuerType            *string  `json:"issuerType"`
	TaxRegime             *string  `json:"taxRegime"`
	Municipality          *string  `json:"municipality"`
	MunicipalityCode      *string  `json:"municipalityCode"`
	State                 *string  `json:"state"`
	MunicipalRegistration *string  `json:"municipalRegistration"`
	StateRegistration     *string  `json:"stateRegistration"`
	Address               *Address `json:"address,omitempty"`
}
type Address struct {
	Street           string `json:"street"`
	Number           string `json:"number"`
	District         string `json:"district"`
	PostalCode       string `json:"postalCode"`
	MunicipalityCode string `json:"municipalityCode"`
	State            string `json:"state"`
}
type Answer struct {
	Value        *string `json:"value"`
	Confirmation string  `json:"confirmation"`
	Origin       *string `json:"origin"`
	EvidenceRef  *string `json:"evidenceRef"`
}
type DiagnosisInput struct {
	DiagnosisID            *string           `json:"diagnosisId"`
	ExpectedVersion        int64             `json:"expectedVersion"`
	ExpectedProfileVersion int64             `json:"expectedProfileVersion"`
	Intent                 string            `json:"intent"`
	CompetenceDate         *string           `json:"competenceDate"`
	Answers                map[string]Answer `json:"answers"`
}
type PersonalSource struct {
	Owner                     string   `json:"owner"`
	DraftID                   string   `json:"draftId"`
	ExpectedDraftUpdatedAt    string   `json:"expectedDraftUpdatedAt"`
	ExpectedProfileUpdatedAt  string   `json:"expectedProfileUpdatedAt"`
	TemplateID                *string  `json:"templateId"`
	ExpectedTemplateUpdatedAt *string  `json:"expectedTemplateUpdatedAt"`
	SelectedFields            []string `json:"selectedFields"`
	ConfirmShareWithIssuer    bool     `json:"confirmShareWithIssuer"`
}
type PersonalSnapshot struct {
	Service   *ServiceFields  `json:"service"`
	Reference json.RawMessage `json:"reference"`
}
type PersonalReader interface {
	ReadPersonalSource(context.Context, Scope, PersonalSource) (PersonalSnapshot, error)
}
type Party struct {
	StateRegistration *string  `json:"stateRegistration,omitempty"`
	Name              *string  `json:"name"`
	Document          *string  `json:"document"`
	Address           *Address `json:"address"`
}
type ServiceFields struct {
	Description *string `json:"description"`
	ServiceCode *string `json:"serviceCode"`
	CNAE        *string `json:"cnae"`
	NBS         *string `json:"nbs"`
	ISSRate     *string `json:"issRate"`
	ISSWithheld *bool   `json:"issWithheld"`
	Location    *string `json:"location"`
}
type Item struct {
	Description string  `json:"description"`
	Unit        string  `json:"unit"`
	Quantity    string  `json:"quantity"`
	UnitPrice   string  `json:"unitPrice"`
	Discount    *string `json:"discount"`
	Addition    *string `json:"addition"`
	NCM         *string `json:"ncm"`
	CFOP        *string `json:"cfop"`
	CST         *string `json:"cst"`
	CSOSN       *string `json:"csosn"`
	Origin      *string `json:"origin"`
}
type Operation struct {
	Nature           string `json:"nature"`
	Purpose          string `json:"purpose"`
	OriginState      string `json:"originState"`
	DestinationState string `json:"destinationState"`
}
type Transport struct {
	Mode          *string `json:"mode"`
	Carrier       *Party  `json:"carrier"`
	FreightAmount *string `json:"freightAmount"`
}
type Payment struct {
	Method *string `json:"method"`
	Amount *string `json:"amount"`
}
type Fields struct {
	Transport      *Transport     `json:"transport,omitempty"`
	Payment        *Payment       `json:"payment,omitempty"`
	CompetenceDate *string        `json:"competenceDate"`
	Amount         *string        `json:"amount"`
	Service        *ServiceFields `json:"service,omitempty"`
	Customer       *Party         `json:"customer,omitempty"`
	Items          []Item         `json:"items,omitempty"`
	Operation      *Operation     `json:"operation,omitempty"`
	Recipient      *Party         `json:"recipient,omitempty"`
}
type PrepareInput struct {
	ExpectedVersion          int64           `json:"expectedVersion"`
	DiagnosisID              string          `json:"diagnosisId"`
	ExpectedDiagnosisVersion int64           `json:"expectedDiagnosisVersion"`
	ExpectedProfileVersion   int64           `json:"expectedProfileVersion"`
	Kind                     string          `json:"kind"`
	Source                   *PersonalSource `json:"source"`
	Fields                   Fields          `json:"fields"`
}
type Command struct {
	EnableIssue            bool            `json:"enableIssue,omitempty"`
	Source                 *PersonalSource `json:"source,omitempty"`
	ExpectedVersion        int64           `json:"expectedVersion"`
	ExpectedProfileVersion int64           `json:"expectedProfileVersion,omitempty"`
	Profile                *Profile        `json:"profile,omitempty"`
	Intent                 string          `json:"intent,omitempty"`
	Fields                 *Fields         `json:"fields,omitempty"`
	SnapshotHash           string          `json:"snapshotHash,omitempty"`
	Confirmed              bool            `json:"confirmed,omitempty"`
	ReviewID               string          `json:"reviewId,omitempty"`
	AuthorizationID        string          `json:"authorizationId,omitempty"`
	BusinessKey            string          `json:"businessKey,omitempty"`
	Operation              string          `json:"operation,omitempty"`
	ConnectorID            string          `json:"connectorId,omitempty"`
	Environment            string          `json:"environment,omitempty"`
	Grants                 []string        `json:"grants,omitempty"`
	Status                 string          `json:"status,omitempty"`
}
type Document struct {
	ContentVersion       int64           `json:"contentVersion"`
	ValidatedConnector   string          `json:"validatedConnector"`
	ValidatedEnvironment string          `json:"validatedEnvironment"`
	ID                   string          `json:"id"`
	IssuerID             string          `json:"issuerId"`
	Version              int64           `json:"version"`
	ProfileVersion       int64           `json:"profileVersion"`
	DiagnosisID          string          `json:"diagnosisId"`
	DiagnosisVersion     int64           `json:"diagnosisVersion"`
	Kind                 string          `json:"kind"`
	State                string          `json:"state"`
	Fields               Fields          `json:"fields"`
	Source               json.RawMessage `json:"source"`
}
type Review struct {
	Document          Document `json:"document"`
	Profile           Profile  `json:"profile"`
	Hash              string   `json:"snapshotHash"`
	Pending           []string `json:"pending"`
	ConnectorID       string   `json:"connectorId"`
	Environment       string   `json:"environment"`
	EmissionAvailable bool     `json:"emissionAvailable"`
}
type Execution struct {
	DocumentState    string            `json:"documentState"`
	Artifacts        []json.RawMessage `json:"artifacts"`
	ID               string            `json:"executionId"`
	JobID            *string           `json:"jobId"`
	IssuerID         string            `json:"issuerId"`
	DocumentID       string            `json:"documentId"`
	ActorID          string            `json:"-"`
	AuthorizationID  string            `json:"-"`
	State            string            `json:"state"`
	Version          int64             `json:"version"`
	ConnectorID      string            `json:"connectorId"`
	Environment      string            `json:"environment"`
	RemoteIdentity   string            `json:"-"`
	RemoteReference  *string           `json:"remoteReference"`
	Protocol         *string           `json:"protocol"`
	SafeErrorCode    *string           `json:"safeErrorCode"`
	ArtifactsPending bool              `json:"artifactsPending"`
	Simulated        bool              `json:"simulated"`
	Fence            int64             `json:"-"`
	LeaseUntil       *time.Time        `json:"-"`
}
type Capability struct {
	CanSubmit, CanLookup bool
	Kind, Environment    string
}
type Submission struct {
	ExecutionID, RemoteIdentity, Environment, SnapshotHash string
	Document                                               Document
}
type Outcome struct {
	State                     string
	RemoteReference, Protocol *string
}
type Artifact struct {
	Kind, SHA256 string
	Content      []byte
}
type Connector interface {
	Capability(context.Context, Scope) (Capability, error)
	Validate(context.Context, Scope, Submission) ([]string, error)
	Submit(context.Context, Scope, Submission) (Outcome, error)
	Lookup(context.Context, Scope, Submission) (Outcome, error)
	FetchArtifacts(context.Context, Scope, Submission) ([]Artifact, error)
}
type ArtifactStore interface {
	PutPrivate(context.Context, Scope, string, Artifact) (string, error)
}
type Queue interface {
	EnqueueTx(context.Context, pgx.Tx, jobs.NewJob) (string, bool, error)
}
type Policy interface {
	Check(context.Context, Scope, string) error
	Member(context.Context, string, string) (bool, error)
}

// EvidenceVerifier is supplied by the reviewed knowledge owner, never the client.
type EvidenceVerifier interface {
	Verify(context.Context, Scope, string, Answer, *string) error
}
