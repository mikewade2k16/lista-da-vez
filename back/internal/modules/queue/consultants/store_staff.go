package consultants

import (
	"context"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

// StoreStaffMember e a projecao crua do membro de loja que NAO opera a fila,
// montada a partir do modelo core RBAC.
type StoreStaffMember struct {
	UserID       string
	Name         string
	RoleCode     string
	RoleTemplate string
	RoleLabel    string
	StoreID      string
	StoreName    string
}

// StoreStaffView e o shape enxuto consumido pelo front: cards de staff exibidos
// ao lado dos consultores, apenas para mostrar quanto recebem pela meta da loja.
type StoreStaffView struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	RoleLabel string `json:"roleLabel"`
	StoreID   string `json:"storeId"`
	StoreName string `json:"storeName"`
}

// queueConsultantRoleCodes lista os codes core.roles que representam o consultor
// de fila — esses ficam de fora do roster de staff porque ja vem de
// /v1/consultants. Cobre os codes queue.* e os legados normalizados.
func queueConsultantRoleCodes() []string {
	return []string{"queue.consultant", "consultant", "core.member", "queue.marketing", "marketing"}
}

// queueConsultantTemplateCodes lista os cloned_from_template_id que indicam
// consultor de fila. Roles customizados clonados do template queue.consultant
// tambem operam a fila e por isso sao excluidos.
func queueConsultantTemplateCodes() []string {
	return []string{"queue.consultant"}
}

// ListStoreStaff devolve os membros das lojas acessiveis ao principal que NAO
// sao consultores de fila. Quando storeID e informado, filtra dentro do escopo
// permitido (membership validada); loja fora do escopo retorna ErrStoreNotFound
// (404). O escopo NUNCA vem do client: parte de principal.StoreIDs (ou das lojas
// do tenant ativo, no caso de platform_admin).
func (service *Service) ListStoreStaff(ctx context.Context, principal auth.Principal, storeID string) ([]StoreStaffView, error) {
	if !canViewConsultants(principal) {
		return nil, ErrForbidden
	}

	accessibleStoreIDs, err := service.resolveAccessibleStoreIDs(ctx, principal)
	if err != nil {
		return nil, err
	}

	trimmedStoreID := strings.TrimSpace(storeID)
	if trimmedStoreID != "" {
		// storeId da query e filtro DENTRO do permitido: valida membership
		// contra o Principal. Fora do escopo -> 404.
		if err := service.ensureStoreAccess(ctx, principal, trimmedStoreID); err != nil {
			return nil, err
		}
		accessibleStoreIDs = []string{trimmedStoreID}
	}

	if len(accessibleStoreIDs) == 0 {
		return []StoreStaffView{}, nil
	}

	members, err := service.repository.ListStoreStaff(ctx, accessibleStoreIDs)
	if err != nil {
		return nil, err
	}

	views := make([]StoreStaffView, 0, len(members))
	for _, member := range members {
		role := normalizeStaffRole(member.RoleCode, member.RoleTemplate)
		views = append(views, StoreStaffView{
			ID:        member.UserID,
			Name:      member.Name,
			Role:      role,
			RoleLabel: staffRoleLabel(role, member.RoleLabel),
			StoreID:   member.StoreID,
			StoreName: member.StoreName,
		})
	}

	return views, nil
}

// resolveAccessibleStoreIDs devolve as lojas que o principal pode enxergar.
// Para papeis nao-platform o escopo ja chega resolvido em principal.StoreIDs.
// platform_admin nao carrega lojas no token: resolvemos pelas lojas ativas do
// tenant do principal (se houver). Sem tenant resolvido -> escopo vazio.
func (service *Service) resolveAccessibleStoreIDs(ctx context.Context, principal auth.Principal) ([]string, error) {
	if principal.Role == auth.RolePlatformAdmin {
		tenantID := strings.TrimSpace(principal.AccountID)
		if tenantID == "" {
			tenantID = strings.TrimSpace(principal.TenantID)
		}
		if tenantID == "" {
			return []string{}, nil
		}
		return service.repository.ListAccessibleStoreIDsForTenant(ctx, tenantID)
	}

	storeIDs := make([]string, 0, len(principal.StoreIDs))
	for _, storeID := range principal.StoreIDs {
		trimmed := strings.TrimSpace(storeID)
		if trimmed != "" {
			storeIDs = append(storeIDs, trimmed)
		}
	}
	return storeIDs, nil
}

// normalizeStaffRole converte o code/template do core role numa categoria
// estavel consumida pelo front para agrupar o recebimento da meta:
//   - gerente            -> "manager"
//   - terminal/caixa     -> "cashier"
//   - demais store-scoped -> categoria derivada do code (ex.: "auxiliar")
//
// O consultor de fila ja foi excluido na query, entao nunca chega aqui.
func normalizeStaffRole(roleCode string, roleTemplate string) string {
	code := strings.ToLower(strings.TrimSpace(roleCode))
	switch code {
	case "queue.manager", "manager":
		return "manager"
	case "queue.store_terminal", "store_terminal":
		return "cashier"
	case "queue.owner", "owner", "core.owner":
		return "manager"
	case "queue.director", "director", "core.admin", "queue.supervisor":
		return "manager"
	}

	// Roles customizados: usa o sufixo do code como categoria (queue.caixa ->
	// "caixa"). Cai aqui apenas para papeis fora do seed padrao.
	if idx := strings.LastIndex(code, "."); idx >= 0 && idx+1 < len(code) {
		suffix := strings.TrimSpace(code[idx+1:])
		if suffix != "" {
			return suffix
		}
	}
	if code != "" {
		return code
	}

	// Fallback ultimo recurso: categoria derivada do template.
	if strings.EqualFold(strings.TrimSpace(roleTemplate), "queue.supervisor") {
		return "manager"
	}
	return "support"
}

// staffRoleLabel devolve o rotulo pt-BR. Prefere o label do proprio role no
// banco (core.roles.label); cai num mapa estavel quando o label vier vazio.
func staffRoleLabel(role string, rawLabel string) string {
	trimmed := strings.TrimSpace(rawLabel)
	if trimmed != "" {
		return trimmed
	}

	switch role {
	case "manager":
		return "Gerente"
	case "cashier":
		return "Caixa"
	case "assistant", "auxiliar":
		return "Auxiliar"
	case "support":
		return "Apoio"
	default:
		if role == "" {
			return "Equipe"
		}
		runes := []rune(role)
		return strings.ToUpper(string(runes[0])) + string(runes[1:])
	}
}
