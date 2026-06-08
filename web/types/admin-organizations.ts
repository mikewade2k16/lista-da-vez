export type AdminOrganizationFieldKey = 'name' | 'slug' | 'isActive'

export interface AdminOrganizationItem {
  id: string
  slug: string
  name: string
  isActive: boolean
  accountCount: number
  accountNames: string
  createdAt: string
  updatedAt: string
}

export interface AdminOrganizationCreateInput {
  slug: string
  name: string
}

export interface AdminOrganizationListResponse {
  organizations: AdminOrganizationItem[]
  total: number
  page: number
  perPage: number
}
