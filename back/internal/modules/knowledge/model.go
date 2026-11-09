package knowledge

import (
	"context"
	"encoding/json"
	"regexp"
)

type Scope struct{ AccountID, ActorID, ResourceID string }
type Error struct {
	Code   string
	Status int
}

func (e *Error) Error() string           { return e.Code }
func fail(code string, status int) error { return &Error{code, status} }

var idPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type Metadata struct {
	Authority      string  `json:"authority"`
	Jurisdiction   string  `json:"jurisdiction"`
	Regime         string  `json:"regime"`
	Classification string  `json:"classification"`
	PublishedOn    *string `json:"publishedOn"`
	ValidFrom      *string `json:"validFrom"`
	ValidTo        *string `json:"validTo"`
	ValiditySource string  `json:"validitySource"`
}
type Import struct {
	ingestion   *ingestionImportGuard
	BaseID      string   `json:"baseId"`
	Title       string   `json:"title"`
	SourceRef   string   `json:"sourceRef"`
	FileName    string   `json:"fileName"`
	ContentType string   `json:"contentType"`
	Content     []byte   `json:"content"`
	Metadata    Metadata `json:"metadata"`
}
type Review struct {
	ExpectedVersion int64    `json:"expectedVersion"`
	Metadata        Metadata `json:"metadata"`
	Confirmed       bool     `json:"confirmed"`
}
type Base struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	Version int64  `json:"version"`
}
type Document struct {
	ID          string   `json:"id"`
	BaseID      string   `json:"baseId"`
	Title       string   `json:"title"`
	SourceRef   string   `json:"sourceRef"`
	Checksum    string   `json:"checksum"`
	Status      string   `json:"status"`
	Version     int      `json:"version"`
	LockVersion int64    `json:"lockVersion"`
	Metadata    Metadata `json:"metadata"`
	ReviewedBy  *string  `json:"reviewedBy"`
	ChunkCount  int      `json:"chunkCount"`
}
type Query struct {
	ContextConfirmed bool   `json:"-"`
	Query            string `json:"query"`
	CompetenceDate   string `json:"competenceDate"`
	Jurisdiction     string `json:"jurisdiction"`
	Regime           string `json:"regime"`
	TopK             int    `json:"topK"`
	Page             int    `json:"page"`
}
type Evidence struct {
	DocumentID          string               `json:"documentId"`
	Version             int                  `json:"version"`
	ChunkID             string               `json:"chunkId"`
	Title               string               `json:"title"`
	Excerpt             string               `json:"excerpt"`
	SourceRef           string               `json:"sourceRef"`
	Score               float64              `json:"score"`
	Location            json.RawMessage      `json:"location"`
	Metadata            Metadata             `json:"metadata"`
	Applicable          bool                 `json:"applicable"`
	ApplicabilityReason string               `json:"applicabilityReason"`
	Review              *PublicReviewSummary `json:"review,omitempty"`
}
type Result struct {
	Truncated         bool       `json:"truncated"`
	Status            string     `json:"status"`
	Evidence          []Evidence `json:"evidence"`
	PotentialConflict bool       `json:"potentialConflict"`
	Message           string     `json:"message"`
}
type Ports struct {
	IngestionSources []IngestionSource
	FetchSource      func(context.Context, IngestionSource) ([]byte, error)
	Authorize        func(context.Context, Scope, bool) error
	PutOriginal      func(context.Context, Scope, string, Import) (string, error)
	ExtractPDF       func(context.Context, []byte) (string, error)
	// Curadoria pública (CT-15 fase C): download do catálogo e PDF longo por página.
	FetchPublic      func(context.Context, PublicCatalogEntry) ([]byte, string, error)
	ExtractPublicPDF func(context.Context, []byte) ([]string, error)
}
type Service struct {
	ingestionQueue IngestionQueue
	store          *Store
	ports          Ports
}

func NewService(s *Store, p Ports) *Service { return &Service{store: s, ports: p} }
func (s *Service) authorize(ctx context.Context, sc Scope, write bool) error {
	if !idPattern.MatchString(sc.AccountID) || !idPattern.MatchString(sc.ActorID) || !idPattern.MatchString(sc.ResourceID) {
		return fail("not_found", 404)
	}
	if s.ports.Authorize == nil {
		return fail("capability_unavailable", 503)
	}
	return s.ports.Authorize(ctx, sc, write)
}
