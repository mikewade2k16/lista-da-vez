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
}

export interface AdminUserListResponse {
  users: AdminUserItem[]
  total: number
  page: number
  perPage: number
}
