package accounting

import (
	"context"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/knowledge"
	"net/http"
	"sync"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/events"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/modules"
)

type Module struct {
	obligationFinance ObligationFinance
	artifactReader    ArtifactReader
	knowledgePorts    knowledge.Ports

	evidence EvidenceVerifier
	service  *Service
	personal PersonalReader
	storage  ArtifactStore
	policy   Policy
}

func (m *Module) WithKnowledgePorts(p knowledge.Ports) *Module { m.knowledgePorts = p; return m }
func (m *Module) WithArtifactReader(reader ArtifactReader) *Module {
	m.artifactReader = reader
	return m
}
func (m *Module) WithObligationFinance(f ObligationFinance) *Module {
	m.obligationFinance = f
	return m
}
func New(policy Policy) *Module                                   { return &Module{policy: policy} }
func (m *Module) WithPersonalReader(p PersonalReader) *Module     { m.personal = p; return m }
func (m *Module) WithArtifacts(p ArtifactStore) *Module           { m.storage = p; return m }
func (m *Module) WithEvidenceVerifier(v EvidenceVerifier) *Module { m.evidence = v; return m }
func (m *Module) ID() string                                      { return "accounting" }
func (m *Module) Metadata() modules.Metadata {
	return modules.Metadata{SchemaName: "accounting", Label: "Contábil", Description: "Diagnóstico e documentos fiscais", SortOrder: 51, OptionalModules: []string{"finance"}}
}
func (m *Module) Permissions() []modules.PermissionDef {
	keys := []string{"issuers.view", "issuers.manage", "documents.view", "documents.manage", "documents.review", "documents.issue", "knowledge.view", "knowledge.manage", "obligations.view", "obligations.manage", "obligations.review"}
	out := make([]modules.PermissionDef, 0, len(keys))
	for _, k := range keys {
		out = append(out, modules.PermissionDef{Key: "accounting." + k, Label: k, Scope: "account"})
	}
	return out
}
func (m *Module) RoleTemplates() []modules.RoleTemplateDef {
	view := []string{"accounting.issuers.view", "accounting.documents.view", "accounting.knowledge.view", "accounting.obligations.view"}
	operator := append(append([]string{}, view...), "accounting.issuers.manage", "accounting.documents.manage", "accounting.documents.review", "accounting.obligations.manage", "accounting.obligations.review")
	return []modules.RoleTemplateDef{{ID: "accounting.viewer", Label: "Leitor Contábil", Permissions: view}, {ID: "accounting.operator", Label: "Operador Contábil", Permissions: operator}, {ID: "accounting.issuer", Label: "Emissor Contábil", Permissions: append(append([]string{}, operator...), "accounting.documents.issue")}}
}
func (m *Module) Build(deps modules.Dependencies) (modules.Handle, error) {
	queue, e := jobs.NewPostgresStore(deps.Pool, "accounting.outbox")
	if e != nil {
		return nil, e
	}
	m.service = NewService(NewStore(deps.Pool), m.policy, queue).WithPersonalReader(m.personal).WithArtifacts(m.storage).WithArtifactReader(m.artifactReader).WithEvidenceVerifier(m.evidence).WithObligationFinance(m.obligationFinance)
	ports := m.knowledgePorts
	ports.Authorize = m.service.AuthorizeKnowledge
	m.service.knowledge = knowledge.NewService(knowledge.NewStore(deps.Pool), ports).WithIngestionQueue(queue)
	worker := jobs.New(queue, jobs.Config{Logger: deps.Logger})
	worker.Register("accounting.issue.v1", m.service)
	worker.Register(knowledge.IngestionJobKind, m.service.knowledge)
	ctx, cancel := context.WithCancel(context.Background())
	worker.Start(ctx)
	h := &handle{service: m.service, auth: deps.AuthMiddleware, worker: worker, cancel: cancel}
	h.wg.Go(func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := m.service.knowledge.IngestionTick(ctx); err != nil && deps.Logger != nil {
					deps.Logger.Warn("knowledge_ingestion_tick_unavailable")
				}
				if err := m.service.ReconcileStale(ctx); err != nil && deps.Logger != nil {
					deps.Logger.Warn("accounting_reconciliation_unavailable")
				}
			}
		}
	})
	return h, nil
}

type handle struct {
	wg      sync.WaitGroup
	service *Service
	auth    *auth.Middleware
	worker  *jobs.Worker
	cancel  context.CancelFunc
}

func (h *handle) ID() string { return "accounting" }
func (h *handle) RegisterRoutes(mux *http.ServeMux) {
	RegisterRoutes(mux, h.service, h.auth)
	RegisterKnowledgeRoutes(mux, h.service.knowledge, h.auth)
}
func (h *handle) RegisterEventHandlers(_ events.Bus) {}
func (h *handle) Close() error                       { h.cancel(); h.wg.Wait(); return h.worker.Close() }

func (m *Module) Service() *Service { return m.service }
