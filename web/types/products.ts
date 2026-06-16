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
  // Derivado de stock > 0 (UI). Switch "Tem estoque" traduz para stock 0/1 no PATCH.
  hasStock: boolean
  status: ProductStatus
  createdAt: string
  updatedAt: string
  // Cruzamento com o ERP (POST /v1/admin/products/erp-match). erpSynced indica se
  // o produto esta vinculado a um item do ERP; erpName/erpDescription sao os dados
  // do ERP (informacao ADICIONAL — nao confundir com name/description do produto).
  erpSynced: boolean
  erpName: string
  erpDescription: string
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

export interface ProductSyncResult {
  inserted: number
  updated: number
  skipped: number
}

// Fonte de produtos (GET/PATCH /v1/admin/products/source). 'local' = XAMPP via
// host.docker.internal; 'online' = API publica do site; 'custom' = base_url fora
// das duas conhecidas (so leitura). O toggle do painel grava 'local'|'online'.
export type ProductSourceMode = 'local' | 'online' | 'custom'

export interface ProductSourceView {
  mode: ProductSourceMode
  baseUrl: string
}

// Facets distintos da account (GET /v1/bio/sources/site_products/facets) —
// popula os selects de Categorias/Campanhas/Tipo. Lista completa, independe de
// paginacao.
export interface ProductFacets {
  categories: string[]
  campaigns: string[]
  tipos: string[]
}

// Resultado do cruzamento produto<->ERP (POST /v1/admin/products/erp-match).
export interface ProductErpMatchResult {
  matched: number
  products: number
}

// Item do ERP que ainda NAO existe no site
// (GET /v1/admin/products/erp-unmatched). "Puxar pro site" cria o produto a
// partir dele (POST /v1/admin/products/from-erp).
export interface ErpUnmatchedItem {
  sku: string
  name: string
  description: string
}

export interface ErpUnmatchedResponse {
  items: ErpUnmatchedItem[]
  total: number
  page: number
  perPage: number
}
