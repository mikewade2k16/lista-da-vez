package core

import "time"

// Role é um cargo efetivo de uma Account — clone editável de um template de cargo
// (catalogo em platform/modules, sincronizado para core.role_templates no boot).
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
