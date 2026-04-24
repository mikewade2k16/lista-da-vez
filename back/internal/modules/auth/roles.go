package auth

type Role string
type AccessScope string

const (
	RoleConsultant    Role = "consultant"
	RoleStoreTerminal Role = "store_terminal"
	RoleManager       Role = "manager"
	RoleMarketing     Role = "marketing"
	RoleDirector      Role = "director"
	RoleOwner         Role = "owner"
	RolePlatformAdmin Role = "platform_admin"
)

const (
	ScopeStore    AccessScope = "store"
	ScopeTenant   AccessScope = "tenant"
	ScopePlatform AccessScope = "platform"
)

type RoleDefinition struct {
	ID          Role        `json:"id"`
	Label       string      `json:"label"`
	Description string      `json:"description"`
	Scope       AccessScope `json:"scope"`
	Grants      []string    `json:"grants"`
}

var roleCatalog = []RoleDefinition{
	{
		ID:          RoleConsultant,
		Label:       "Consultor",
		Description: "Acesso operacional basico para trabalhar fila e atendimentos da propria loja.",
		Scope:       ScopeStore,
		Grants:      []string{"queue:read:store", "service:read:store", "service:finish:store"},
	},
	{
		ID:          RoleStoreTerminal,
		Label:       "Acesso da loja",
		Description: "Acesso fixo do computador da loja para tocar a operacao da propria unidade em tempo real.",
		Scope:       ScopeStore,
		Grants:      []string{"queue:*:store", "service:*:store", "realtime:read:store"},
	},
	{
		ID:          RoleManager,
		Label:       "Gerente",
		Description: "Acompanha consultores e operacao da loja sob sua responsabilidade.",
		Scope:       ScopeStore,
		Grants:      []string{"queue:*:store", "consultant:*:store", "report:read:store"},
	},
	{
		ID:          RoleMarketing,
		Label:       "Marketing",
		Description: "Enxerga a operacao integrada de todas as lojas do tenant, com foco em leitura e acompanhamento.",
		Scope:       ScopeTenant,
		Grants:      []string{"operations:read:tenant", "realtime:read:tenant", "report:read:tenant", "campaign:*:tenant"},
	},
	{
		ID:          RoleDirector,
		Label:       "Diretoria",
		Description: "Acompanha a operacao consolidada do tenant com troca de loja e leitura executiva cross-store.",
		Scope:       ScopeTenant,
		Grants:      []string{"operations:read:tenant", "realtime:read:tenant", "report:read:tenant", "analytics:read:tenant"},
	},
	{
		ID:          RoleOwner,
		Label:       "Proprietario",
		Description: "Responsavel pelo cliente/tenant e com visao total das lojas do proprio grupo.",
		Scope:       ScopeTenant,
		Grants:      []string{"tenant:*", "store:*:tenant", "report:*:tenant", "campaign:*:tenant"},
	},
	{
		ID:          RolePlatformAdmin,
		Label:       "Admin da plataforma",
		Description: "Acesso interno do time de produto/dev, incluindo areas de teste e operacao cross-tenant.",
		Scope:       ScopePlatform,
		Grants:      []string{"platform:*", "tenant:*", "store:*", "dev:*"},
	},
}

func RoleCatalog() []RoleDefinition {
	cloned := make([]RoleDefinition, 0, len(roleCatalog))

	for _, definition := range roleCatalog {
		cloned = append(cloned, RoleDefinition{
			ID:          definition.ID,
			Label:       definition.Label,
			Description: definition.Description,
			Scope:       definition.Scope,
			Grants:      append([]string{}, definition.Grants...),
		})
	}

	return cloned
}

func IsValidRole(role Role) bool {
	for _, definition := range roleCatalog {
		if definition.ID == role {
			return true
		}
	}

	return false
}

func ValidateUserScope(user User) error {
	if !IsValidRole(user.Role) {
		return ErrInvalidRoleScope
	}

	switch user.Role {
	case RoleConsultant, RoleStoreTerminal, RoleManager:
		if user.TenantID == "" || len(user.StoreIDs) != 1 {
			return ErrInvalidRoleScope
		}
	case RoleMarketing, RoleDirector, RoleOwner:
		if user.TenantID == "" {
			return ErrInvalidRoleScope
		}
	case RolePlatformAdmin:
		return nil
	}

	return nil
}
