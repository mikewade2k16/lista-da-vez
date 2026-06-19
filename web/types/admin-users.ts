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
