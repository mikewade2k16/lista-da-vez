package knowledge

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
	"time"
)

const IngestionJobKind = "knowledge.ingest.v1"

type IngestionSource struct {
	ID          string `json:"id"`
	Revision    string `json:"revision"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	ContentType string `json:"contentType"`
	Authority   string `json:"authority"`
}
type SubscriptionInput struct {
	BaseID          string `json:"baseId"`
	SourceID        string `json:"sourceId"`
	ExpectedVersion int64  `json:"expectedVersion"`
	Enabled         bool   `json:"enabled"`
	IntervalHours   int    `json:"intervalHours"`
}
type SourceSubscription struct {
	ID             string    `json:"id"`
	BaseID         string    `json:"baseId"`
	SourceID       string    `json:"sourceId"`
	SourceRevision string    `json:"sourceRevision"`
	Version        int64     `json:"version"`
	Enabled        bool      `json:"enabled"`
	IntervalHours  int       `json:"intervalHours"`
	NextCheckAt    time.Time `json:"nextCheckAt"`
	LastChecksum   *string   `json:"lastChecksum"`
	LastDocumentID *string   `json:"lastDocumentId"`
	Scope          Scope     `json:"-"`
}
type IngestionRun struct {
	Recoveries          int       `json:"recoveries"`
	ID                  string    `json:"id"`
	SubscriptionID      string    `json:"subscriptionId"`
	SubscriptionVersion int64     `json:"subscriptionVersion"`
	BaseID              string    `json:"baseId"`
	SourceID            string    `json:"sourceId"`
	SourceRevision      string    `json:"sourceRevision"`
	State               string    `json:"state"`
	Attempts            int       `json:"attempts"`
	Fence               int64     `json:"-"`
	DocumentID          *string   `json:"documentId"`
	Checksum            *string   `json:"checksum"`
	ErrorCode           *string   `json:"errorCode"`
	CreatedAt           time.Time `json:"createdAt"`
	Scope               Scope     `json:"-"`
}
type IngestionQueue interface {
	EnqueueTx(context.Context, pgx.Tx, jobs.NewJob) (string, bool, error)
}

func (s *Service) WithIngestionQueue(q IngestionQueue) *Service { s.ingestionQueue = q; return s }
func OfficialIngestionSources() []IngestionSource {
	return []IngestionSource{{ID: "nfse.contribuintes.api", Revision: "2026-11-05.1", Title: "Manual de contribuintes — API NFS-e (fonte monitorada)", URL: "https://www.gov.br/nfse/pt-br/biblioteca/documentacao-tecnica/documentacao-atual/manual-contribuintes-emissor-publico-api-sistema-nacional-nfs-e-v1-2-out2025.pdf", ContentType: "application/pdf", Authority: "Portal nacional NFS-e"}}
}

type ingestionImportGuard struct {
	RunID               string
	Fence               int64
	SubscriptionVersion int64
}
