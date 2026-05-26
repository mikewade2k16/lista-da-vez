package roadmap

import "time"

const (
	PermRoadmapView   = "roadmap.view"
	PermRoadmapManage = "roadmap.manage"

	StatusPending    = "pending"
	StatusInProgress = "in_progress"
	StatusBeta       = "beta"
	StatusDone       = "done"

	PriorityP0 = "P0"
	PriorityP1 = "P1"
	PriorityP2 = "P2"
	PriorityP3 = "P3"

	CategoryFrontend                = "frontend"
	CategoryBackend                 = "backend"
	CategoryBanco                   = "banco"
	CategoryLinguagens              = "linguagens"
	CategoryDeploy                  = "deploy"
	CategoryPadroesGerais           = "padroes-gerais"
	ModuleCategoryAtendimento       = "atendimento"
	ModuleCategoryTools             = "tools"
	ModuleCategoryOperacaoComercial = "operacao-comercial"
	ModuleCategoryIndicadores       = "indicadores"
	ModuleCategoryManage            = "manage"
)

type AccessContext struct {
	UserID          string
	AccountID       string
	IsPlatformAdmin bool
	Permissions     map[string]struct{}
}

func (access AccessContext) Has(permission string) bool {
	if access.IsPlatformAdmin {
		return true
	}
	_, ok := access.Permissions[permission]
	return ok
}

type ModuleRecord struct {
	ID          string    `json:"id"`
	SourceID    string    `json:"sourceId"`
	AccountID   *string   `json:"accountId,omitempty"`
	IsGlobal    bool      `json:"isGlobal"`
	Label       string    `json:"label"`
	Route       string    `json:"route"`
	Status      string    `json:"status"`
	Priority    string    `json:"priority"`
	Category    string    `json:"category,omitempty"`
	Description string    `json:"description"`
	Scope       []string  `json:"scope"`
	DependsOn   []string  `json:"dependsOn"`
	SortOrder   int       `json:"sortOrder"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type Rule struct {
	ID          string    `json:"id"`
	SourceID    string    `json:"sourceId"`
	AccountID   *string   `json:"accountId,omitempty"`
	IsGlobal    bool      `json:"isGlobal"`
	Category    string    `json:"category"`
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	Why         string    `json:"why"`
	AppliesWhen string    `json:"appliesWhen"`
	SortOrder   int       `json:"sortOrder"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type UpsertModuleInput struct {
	SourceID    string
	Label       string
	Route       string
	Status      string
	Priority    string
	Category    string
	Description string
	Scope       []string
	DependsOn   []string
	SortOrder   int
}

type UpsertRuleInput struct {
	SourceID    string
	Category    string
	Title       string
	Body        string
	Why         string
	AppliesWhen string
	SortOrder   int
}

type DashboardTask struct {
	ID                string  `json:"id"`
	Title             string  `json:"title"`
	Status            *string `json:"status,omitempty"`
	Priority          string  `json:"priority"`
	Archived          bool    `json:"archived"`
	BoardID           string  `json:"boardId"`
	ColumnID          *string `json:"columnId,omitempty"`
	ResponsibleUserID *string `json:"responsibleUserId,omitempty"`
}

type DashboardCounts struct {
	Total      int `json:"total"`
	Idea       int `json:"idea"`
	Planning   int `json:"planning"`
	InProgress int `json:"inProgress"`
	Done       int `json:"done"`
}

type DashboardModule struct {
	Module ModuleRecord    `json:"module"`
	Tasks  []DashboardTask `json:"tasks"`
	Counts DashboardCounts `json:"counts"`
}
