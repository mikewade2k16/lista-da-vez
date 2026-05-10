package core

import "time"

// RoleTemplate espelha core.role_templates. Declarado pelos módulos via
// RoleTemplateDef e populado pelo SyncCatalog no boot.
type RoleTemplate struct {
	ID          string
	ModuleID    string
	Label       string
	Description string
	IsSystem    bool
	IsLocked    bool
	SortOrder   int
}

// Role é um cargo efetivo de uma Account — clone editável de RoleTemplate.
// Vive em core.roles e pertence exclusivamente à Account (não ao catálogo).
type Role struct {
	ID                   string
	AccountID            string
	ClonedFromTemplateID string
	Code                 string
	Label                string
	Description          string
	IsDefault            bool
	IsLocked             bool
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

func (r Role) ToSummary() RoleSummary {
	return RoleSummary{
		ID:          r.ID,
		Code:        r.Code,
		Label:       r.Label,
		IsLocked:    r.IsLocked,
		IsDefault:   r.IsDefault,
		Description: r.Description,
	}
}
