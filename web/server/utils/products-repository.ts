export type ServerProductStatus = 'active' | 'desactive'

export interface ServerProduct {
  id: number
  clientId: number
  clientName: string
  name: string
  code: string
  image: string
  description: string
  categories: string[]
  campaigns: string[]
  price: number
  fator: number
  tipo: string
  stock: number
  status: ServerProductStatus
  createdAt: string
  updatedAt: string
  deletedAt: string
}

export interface ProductsListMeta {
  page: number
  limit: number
  total: number
  totalPages: number
  hasMore: boolean
}

export interface ProductListOptions {
  page: number
  limit: number
  q?: string
  campaign?: string
  category?: string
  clientId?: string | number
  withDeleted?: string | number
  viewerUserType?: 'admin' | 'client'
  viewerClientId?: number
}

export interface ProductCreateInput {
  name?: string
  code?: string
  image?: string
  clientId?: string | number
  clientName?: string
}

export interface ProductAccessOptions {
  viewerUserType?: 'admin' | 'client'
  viewerClientId?: number
}

const globalKey = '__omni_products_repo__'

const CLIENT_NAMES: Record<number, string> = {
  106: 'crow',
  105: 'Perola',
  104: 'Dr Antonio Tavares',
  103: 'Zen as Fuck',
  102: 'Duby',
  101: 'sdfsodifho',
}

interface RepositoryState {
  products: ServerProduct[]
  nextId: number
}

function nowIso() {
  return new Date().toISOString()
}

function normalizeText(value: unknown, max = 255) {
  return String(value ?? '')
    .replace(/\s+/g, ' ')
    .trim()
    .slice(0, max)
}

function normalizeCode(value: unknown) {
  return normalizeText(value, 50)
}

function normalizeImage(value: unknown) {
  const raw = String(value ?? '').trim()
  if (!raw) {
    return ''
  }

  if (raw.startsWith('data:image/')) {
    return raw
  }

  return raw.replace(/\s+/g, ' ').slice(0, 2048)
}

function normalizeStatus(value: unknown): ServerProductStatus {
  const normalized = String(value ?? '')
    .trim()
    .toLowerCase()
  return normalized === 'active' ? 'active' : 'desactive'
}

function normalizeNumber(value: unknown, decimals = 2) {
  const parsed = Number(String(value ?? '').replace(',', '.'))
  if (!Number.isFinite(parsed)) {
    return 0
  }

  const safe = Math.max(0, parsed)
  return Number(safe.toFixed(decimals))
}

function normalizeStock(value: unknown) {
  const normalized = String(value ?? '')
    .trim()
    .toLowerCase()
  if (['1', 'true', 'on', 'yes', 'sim', 'active'].includes(normalized)) {
    return 1
  }
  if (['0', 'false', 'off', 'no', 'nao', 'desactive', 'inactive'].includes(normalized)) {
    return 0
  }

  const parsed = Number.parseInt(String(value ?? ''), 10)
  if (!Number.isFinite(parsed)) {
    return 0
  }

  return parsed > 0 ? 1 : 0
}

function normalizeList(value: unknown) {
  if (Array.isArray(value)) {
    return value
      .map((item) => normalizeText(item, 120))
      .filter(Boolean)
      .slice(0, 30)
  }

  return normalizeText(value, 3000)
    .split(/[\n,;|]+/)
    .map((item) => normalizeText(item, 120))
    .filter(Boolean)
    .slice(0, 30)
}

function joinList(list: string[]) {
  return list.join(', ')
}

function normalizeSearch(value: unknown) {
  return String(value ?? '')
    .toLowerCase()
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
}

function normalizeClientId(value: unknown) {
  const parsed = Number.parseInt(String(value ?? '').trim(), 10)
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return 0
  }

  return parsed
}

function resolveClientName(clientId: number, explicitName?: unknown) {
  const normalizedName = normalizeText(explicitName, 120)
  if (normalizedName) {
    return normalizedName
  }

  if (clientId > 0 && CLIENT_NAMES[clientId]) {
    return CLIENT_NAMES[clientId]
  }

  if (clientId > 0) {
    return `Cliente #${clientId}`
  }

  return 'Sem cliente'
}

function canAccessProduct(product: ServerProduct, options?: ProductAccessOptions) {
  const viewerType = String(options?.viewerUserType ?? 'admin')
    .trim()
    .toLowerCase()
  if (viewerType !== 'client') {
    return true
  }

  const viewerClientId = normalizeClientId(options?.viewerClientId)
  if (viewerClientId <= 0) {
    return false
  }

  return product.clientId === viewerClientId
}

function toDto(product: ServerProduct) {
  return {
    id: product.id,
    clientId: product.clientId,
    clientName: product.clientName,
    name: product.name,
    code: product.code,
    image: product.image,
    description: product.description,
    categories: [...product.categories],
    categoriesText: joinList(product.categories),
    campaigns: [...product.campaigns],
    campaignsText: joinList(product.campaigns),
    price: product.price,
    fator: product.fator,
    tipo: product.tipo,
    stock: product.stock,
    status: product.status,
    createdAt: product.createdAt,
    updatedAt: product.updatedAt,
    deletedAt: product.deletedAt,
    isDeleted: Boolean(product.deletedAt),
  }
}

function seedProducts(): ServerProduct[] {
  return [
    {
      id: 360588,
      clientId: 105,
      clientName: 'Perola',
      name: 'joia3',
      code: 'joia34578',
      image:
        'https://images.unsplash.com/photo-1617038220319-276d3cfab638?auto=format&fit=crop&w=320&q=80',
      description: 'Colecao valentine premium.',
      categories: ['anel', 'ouro'],
      campaigns: ['valentines_2026'],
      price: 350.0,
      fator: 1.15,
      tipo: 'anel',
      stock: 1,
      status: 'active',
      createdAt: '2026-02-10T10:00:00.000Z',
      updatedAt: '2026-02-14T08:00:00.000Z',
      deletedAt: '',
    },
    {
      id: 360561,
      clientId: 105,
      clientName: 'Perola',
      name: 'joia2',
      code: 'joia2',
      image:
        'https://images.unsplash.com/photo-1515562141207-7a88fb7ce338?auto=format&fit=crop&w=320&q=80',
      description: 'Produto de vitrine.',
      categories: ['brinco'],
      campaigns: ['valentines_2026'],
      price: 220.0,
      fator: 1.1,
      tipo: 'brinco',
      stock: 1,
      status: 'active',
      createdAt: '2026-02-11T10:00:00.000Z',
      updatedAt: '2026-02-14T08:30:00.000Z',
      deletedAt: '',
    },
    {
      id: 360582,
      clientId: 104,
      clientName: 'Dr Antonio Tavares',
      name: 'joia1',
      code: 'joia1',
      image:
        'https://images.unsplash.com/photo-1605100804763-247f67b3557e?auto=format&fit=crop&w=320&q=80',
      description: 'Produto para campanha principal.',
      categories: ['colar'],
      campaigns: ['valentines_2026', 'site_home'],
      price: 180.0,
      fator: 1.05,
      tipo: 'colar',
      stock: 1,
      status: 'active',
      createdAt: '2026-02-12T10:00:00.000Z',
      updatedAt: '2026-02-14T09:00:00.000Z',
      deletedAt: '',
    },
    {
      id: 361062,
      clientId: 106,
      clientName: 'crow',
      name: '360582_360561_360588',
      code: '360582_360561_360588',
      image:
        'https://images.unsplash.com/photo-1535632066927-ab7c9ab60908?auto=format&fit=crop&w=320&q=80',
      description: 'Kit combinado de campanha.',
      categories: ['kit'],
      campaigns: ['valentines_2026'],
      price: 650.0,
      fator: 1.22,
      tipo: 'kit',
      stock: 1,
      status: 'active',
      createdAt: '2026-02-13T10:00:00.000Z',
      updatedAt: '2026-02-14T09:15:00.000Z',
      deletedAt: '',
    },
    {
      id: 361500,
      clientId: 106,
      clientName: 'crow',
      name: 'produto arquivado',
      code: 'archived-01',
      image: '',
      description: 'Registro deletado para teste.',
      categories: ['arquivo'],
      campaigns: ['legacy'],
      price: 10,
      fator: 1,
      tipo: 'outro',
      stock: 0,
      status: 'desactive',
      createdAt: '2026-01-01T10:00:00.000Z',
      updatedAt: '2026-01-10T10:00:00.000Z',
      deletedAt: '2026-02-01T10:00:00.000Z',
    },
  ]
}

function getState(): RepositoryState {
  const globalRef = globalThis as typeof globalThis & { [globalKey]?: RepositoryState }
  if (!globalRef[globalKey]) {
    const seeded = seedProducts().sort((a, b) => b.id - a.id)
    globalRef[globalKey] = {
      products: seeded,
      nextId: Math.max(...seeded.map((item) => item.id)) + 1,
    }
  }

  return globalRef[globalKey] as RepositoryState
}

export function listProducts(options: ProductListOptions) {
  const state = getState()
  const page = Number.isFinite(options.page) && options.page > 0 ? options.page : 1
  const limit =
    Number.isFinite(options.limit) && options.limit > 0 ? Math.min(options.limit, 300) : 100
  const q = normalizeSearch(options.q)
  const campaign = normalizeSearch(options.campaign)
  const category = normalizeSearch(options.category)
  const clientId = normalizeClientId(options.clientId)
  const withDeleted = ['1', 'true', 'on', 'yes', 'sim'].includes(
    String(options.withDeleted ?? '')
      .trim()
      .toLowerCase(),
  )

  const filtered = state.products.filter((product) => {
    if (!canAccessProduct(product, options)) {
      return false
    }
    if (clientId > 0 && product.clientId !== clientId) {
      return false
    }
    if (!withDeleted && product.deletedAt) {
      return false
    }
    if (campaign && !product.campaigns.some((item) => normalizeSearch(item) === campaign)) {
      return false
    }
    if (category && !product.categories.some((item) => normalizeSearch(item) === category)) {
      return false
    }
    if (!q) {
      return true
    }

    const haystack = normalizeSearch(
      [
        product.name,
        product.code,
        product.description,
        product.clientName,
        product.categories.join(' '),
        product.campaigns.join(' '),
      ].join(' '),
    )

    return haystack.includes(q)
  })

  const total = filtered.length
  const totalPages = Math.max(1, Math.ceil(total / limit))
  const safePage = Math.min(page, totalPages)
  const start = (safePage - 1) * limit
  const items = filtered.slice(start, start + limit).map(toDto)

  const allowedProducts = state.products.filter((product) => canAccessProduct(product, options))
  const campaigns = Array.from(new Set(allowedProducts.flatMap((item) => item.campaigns))).sort(
    (a, b) => a.localeCompare(b, 'pt-BR'),
  )
  const categories = Array.from(new Set(allowedProducts.flatMap((item) => item.categories))).sort(
    (a, b) => a.localeCompare(b, 'pt-BR'),
  )

  return {
    items,
    meta: {
      page: safePage,
      limit,
      total,
      totalPages,
      hasMore: safePage < totalPages,
    } satisfies ProductsListMeta,
    filters: {
      campaigns,
      categories,
    },
  }
}

export function createProduct(input: ProductCreateInput = {}, options?: ProductAccessOptions) {
  const state = getState()
  const id = state.nextId++
  const now = nowIso()
  const viewerType = String(options?.viewerUserType ?? 'admin')
    .trim()
    .toLowerCase()
  const viewerClientId = normalizeClientId(options?.viewerClientId)
  const requestedClientId = normalizeClientId(input.clientId)
  const resolvedClientId =
    viewerType === 'client'
      ? viewerClientId > 0
        ? viewerClientId
        : 106
      : requestedClientId > 0
        ? requestedClientId
        : viewerClientId > 0
          ? viewerClientId
          : 106

  const created: ServerProduct = {
    id,
    clientId: resolvedClientId,
    clientName: resolveClientName(resolvedClientId, input.clientName),
    name: normalizeText(input.name, 100) || `Novo Produto ${id}`,
    code: normalizeCode(input.code) || `NEW-${id}`,
    image: normalizeImage(input.image),
    description: '',
    categories: [],
    campaigns: [],
    price: 0,
    fator: 0,
    tipo: '',
    stock: 0,
    status: 'active',
    createdAt: now,
    updatedAt: now,
    deletedAt: '',
  }

  state.products.unshift(created)
  return toDto(created)
}

export function updateProductField(
  id: number,
  field: string,
  value: unknown,
  options?: ProductAccessOptions,
) {
  const state = getState()
  const target = state.products.find((product) => product.id === id)
  if (!target || !canAccessProduct(target, options)) {
    return null
  }

  if (field === 'name') target.name = normalizeText(value, 100)
  if (field === 'code') target.code = normalizeCode(value)
  if (field === 'image') target.image = normalizeImage(value)
  if (field === 'description') target.description = normalizeText(value, 4000)
  if (field === 'categoriesText' || field === 'categories') target.categories = normalizeList(value)
  if (field === 'campaignsText' || field === 'campaigns') target.campaigns = normalizeList(value)
  if (field === 'price') target.price = normalizeNumber(value, 2)
  if (field === 'fator') target.fator = normalizeNumber(value, 2)
  if (field === 'tipo') target.tipo = normalizeText(value, 50)
  if (field === 'stock') target.stock = normalizeStock(value)
  if (field === 'status') target.status = normalizeStatus(value)

  target.updatedAt = nowIso()
  return toDto(target)
}

export function softDeleteProductById(id: number, options?: ProductAccessOptions) {
  const state = getState()
  const target = state.products.find((product) => product.id === id)
  if (!target || !canAccessProduct(target, options)) {
    return null
  }

  target.deletedAt = nowIso()
  target.updatedAt = nowIso()
  target.status = 'desactive'
  target.stock = 0

  return toDto(target)
}
