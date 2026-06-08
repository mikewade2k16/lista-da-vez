export type ProductStatus = 'active' | 'inactive'

export type ProductFieldKey =
  | 'name'
  | 'code'
  | 'description'
  | 'image'
  | 'categories'
  | 'campaigns'
  | 'price'
  | 'fator'
  | 'tipo'
  | 'stock'
  | 'status'

export interface ProductItem {
  id: string
  accountId: string
  sourceId: string
  sourceLabel: string
  name: string
  code: string
  description: string
  image: string
  categories: string[]
  campaigns: string[]
  price: number
  fator: number
  tipo: string
  stock: number
  status: ProductStatus
  createdAt: string
  updatedAt: string
}

export interface ProductCreateInput {
  name: string
  code: string
  description: string
  image: string
  categories: string[]
  campaigns: string[]
  price: number
  fator: number
  tipo: string
  stock: number
}

export interface ProductsListResponse {
  products: ProductItem[]
  total: number
  page: number
  perPage: number
}
