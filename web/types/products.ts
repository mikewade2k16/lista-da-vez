export type ProductStatus = 'active' | 'desactive'

export type ProductFieldKey =
  | 'name'
  | 'code'
  | 'categories'
  | 'categoriesText'
  | 'campaigns'
  | 'campaignsText'
  | 'image'
  | 'price'
  | 'fator'
  | 'tipo'
  | 'stock'
  | 'status'
  | 'description'

export interface ProductItem {
  id: number
  clientId: number
  clientName: string
  name: string
  code: string
  image: string
  description: string
  categories: string[]
  categoriesText: string
  campaigns: string[]
  campaignsText: string
  price: number
  fator: number
  tipo: string
  stock: number
  status: ProductStatus
  createdAt: string
  updatedAt: string
  deletedAt: string
  isDeleted: boolean
}

export interface ProductsListMeta {
  page: number
  limit: number
  total: number
  totalPages: number
  hasMore: boolean
}

export interface ProductsListFilters {
  campaigns: string[]
  categories: string[]
}

export interface ProductsListResponse {
  status: 'success'
  data: ProductItem[]
  meta: ProductsListMeta
  filters: ProductsListFilters
}

export interface ProductMutationResponse {
  status: 'success'
  data: ProductItem
}
