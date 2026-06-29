export type AdminUserFieldKey = 'email' | 'displayName' | 'nick' | 'isActive' | 'isPlatformAdmin'

export interface AdminUserItem {
  id: string
  email: string
  displayName: string
  nick: string
  avatarPath: string
  isActive: boolean
  isPlatformAdmin: boolean
  mustChangePassword: boolean
  accountCount: number
  accountNames: string
  // Id do unico cliente (account nao-agencia) ativo deste usuario, ou '' quando
  // ele tem 0 ou mais de 1 cliente. Habilita a edicao inline de "Cliente" na grade
  // (mover o usuario de cliente). Quando '', a celula vira read-only.
  clientAccountId: string
  // True quando o usuario e membro ativo de pelo menos uma conta-agencia
  // (is_agency=true). Vindo do backend (/v1/admin/users). Serve para a grade
  // sinalizar que esse usuario ve todos os clientes/modulos da agencia.
  isAgencyMember: boolean
  createdAt: string
  updatedAt: string
}

export interface AccountMembershipItem {
  accountId: string
  accountSlug: string
  accountName: string
  isActive: boolean
  joinedAt: string
  // Papel coarse do usuario naquela conta (owner/director/marketing/...). isAgency
  // marca a conta-agencia (vinculo de agencia) vs conta-cliente.
  role: string
  isAgency: boolean
}

export interface AdminUserCreateInput {
  email: string
  displayName: string
  nick: string
  isPlatformAdmin: boolean
  temporaryPassword: string
  // Vinculo opcional: cliente (account) e/ou agencia (organization).
  accountId?: string
  organizationId?: string
  // Papel no tenant do cliente (owner/director/marketing) — cria user_tenant_roles
  // legado (necessario para login + /operacao/usuarios). Default owner no backend.
  role?: string
  // Cargo na agencia (agency_owner = acesso total / agency_member = acesso limitado)
  // quando organizationId setado. Default agency_member no backend.
  orgRole?: string
}

export interface AdminUserListResponse {
  users: AdminUserItem[]
  total: number
  page: number
  perPage: number
}

// Cargo na agencia (organization). agency_owner = acesso amplo a todos os
// clientes da org; agency_member = acesso limitado. Espelha core.organization_users.
export type OrgRole = 'agency_owner' | 'agency_member'

// Efeito de um override de permissao por usuario por account. Espelha
// core.user_permission_overrides (allow concede, deny revoga a permissao herdada
// dos papeis). Sem override = herda do papel (nao aparece nesta lista).
export type PermissionEffect = 'allow' | 'deny'

// Override de modulo/pagina de UM usuario numa account especifica. permissionKey
// e a chave da permissao no catalogo (core.permissions, ex.: 'cardapio.orders.view').
export interface UserPermissionOverride {
  permissionKey: string
  effect: PermissionEffect
  // Nota opcional do admin justificando o override (auditoria).
  note?: string
}

// Permissao disponivel para receber override num usuario, vinda do catalogo
// filtrado pelos modulos habilitados na account (backend monta `available`).
// moduleId/scope orientam o agrupamento na UI (por modulo / por workspace).
export interface AvailablePermission {
  key: string
  label: string
  moduleId: string
  scope: string
}

// Resposta de GET/PUT /v1/admin/users/{id}/accounts/{accountId}/overrides.
// overrides = os ativos do usuario; available = catalogo elegivel para edicao.
export interface UserOverridesResponse {
  overrides: UserPermissionOverride[]
  available: AvailablePermission[]
}

// Resumo de um cargo (core.roles) de uma account — clone editavel de template.
// Mesmo shape do RoleSummary do backend (core/model.go). isLocked = cargo de
// sistema nao removivel; isDefault = atribuido por padrao a novos membros.
export interface RoleSummary {
  id: string
  code: string
  label: string
  isLocked: boolean
  isDefault: boolean
  description?: string
}

// Alias semantico: na UI o RoleSummary representa um cargo da conta.
export type AccountRole = RoleSummary

// Detalhe de um cargo: o resumo + a lista de permissionKeys marcadas na matriz.
// Espelha GET /v1/accounts/{accountId}/roles/{roleId}.
export interface RoleDetail {
  role: RoleSummary
  permissions: string[]
}

// Body de POST /v1/accounts/{accountId}/roles (cria cargo customizado).
export interface RoleCreateInput {
  code: string
  label: string
  description?: string
}

// Body de PATCH /v1/accounts/{accountId}/roles/{roleId} (edita label/descricao
// e substitui a matriz de permissoes do cargo).
export interface RoleUpdateInput {
  label: string
  description?: string
  permissions: string[]
}
