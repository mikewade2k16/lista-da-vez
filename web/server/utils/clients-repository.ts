export type ServerClientStatus = 'active' | 'inactive'
export type ServerClientBillingMode = 'single' | 'per_store'

export interface ServerClientStore {
  id: string
  name: string
  amount: number
}

export interface ServerClientModule {
  code: string
  name: string
  status: 'active' | 'inactive'
}

export interface ServerClient {
  id: number
  coreTenantId: string
  name: string
  status: ServerClientStatus
  userCount: number
  userNicks: string[]
  projectCount: number
  projectSegments: string[]
  billingMode: ServerClientBillingMode
  monthlyPaymentAmount: number
  paymentDueDay: string
  stores: ServerClientStore[]
  modules: ServerClientModule[]
  logo: string
  webhookEnabled: boolean
  webhookKey: string
  contactPhone: string
  contactSite: string
  contactAddress: string
  requireUserStoreLink: boolean
  requireUserRegistration: boolean
}

export interface ClientsListMeta {
  page: number
  limit: number
  total: number
  totalPages: number
  hasMore: boolean
}

export interface ClientListOptions {
  page: number
  limit: number
  q?: string
  status?: string
  viewerUserType?: 'admin' | 'client'
  viewerClientId?: number
}

export interface ClientCreateInput {
  name?: string
  status?: string
}

export interface ClientStoreInput {
  id?: string
  name?: string
  amount?: number | string
}

const globalKey = '__omni_clients_repo__'

interface RepositoryState {
  clients: ServerClient[]
  nextId: number
}

function cloneValue<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T
}

function normalizeSite(value: unknown) {
  const raw = String(value ?? '')
    .trim()
    .slice(0, 255)
  if (!raw) {
    return ''
  }
  if (/^https?:\/\//i.test(raw)) {
    return raw
  }

  return `https://${raw.replace(/^\/+/, '')}`
}

function normalizeShortText(value: unknown, max = 255) {
  return String(value ?? '')
    .replace(/\s+/g, ' ')
    .trim()
    .slice(0, max)
}

function normalizeWebhookEnabled(value: unknown) {
  const normalized = String(value ?? '')
    .trim()
    .toLowerCase()
  return ['1', 'true', 'on', 'enabled', 'active', 'sim'].includes(normalized)
}

function normalizeBooleanLike(value: unknown, fallback = false) {
  if (typeof value === 'boolean') {
    return value
  }

  const normalized = String(value ?? '')
    .trim()
    .toLowerCase()
  if (!normalized) {
    return fallback
  }

  return ['1', 'true', 'on', 'enabled', 'active', 'sim'].includes(normalized)
}

function createWebhookKey(clientId: number) {
  const random = Math.random().toString(36).slice(2, 10)
  return `client_${clientId}_${Date.now().toString(36)}_${random}`
}

function toAmount(value: unknown) {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return Math.max(0, Number(value.toFixed(2)))
  }

  const raw = String(value ?? '').trim()
  if (!raw) {
    return 0
  }

  let normalized = raw
    .replace(/\s+/g, '')
    .replace(/^R\$/i, '')
    .replace(/[^\d,.-]/g, '')

  const hasComma = normalized.includes(',')
  const hasDot = normalized.includes('.')

  if (hasComma && hasDot) {
    if (normalized.lastIndexOf(',') > normalized.lastIndexOf('.')) {
      normalized = normalized.replace(/\./g, '').replace(',', '.')
    } else {
      normalized = normalized.replace(/,/g, '')
    }
  } else if (hasComma) {
    normalized = normalized.replace(/\./g, '').replace(',', '.')
  }

  const parsed = Number(normalized)
  if (!Number.isFinite(parsed)) {
    return 0
  }

  return Math.max(0, Number(parsed.toFixed(2)))
}

function normalizeDueDay(value: unknown) {
  const digits = String(value ?? '')
    .replace(/\D/g, '')
    .slice(0, 2)
  if (!digits) {
    return ''
  }

  const day = Number(digits)
  if (!Number.isFinite(day) || day < 1) {
    return ''
  }

  return String(Math.min(day, 31)).padStart(2, '0')
}

function normalizeStoreName(value: unknown) {
  return String(value ?? '')
    .replace(/\s+/g, ' ')
    .trim()
    .slice(0, 120)
}

function normalizeClientName(value: unknown) {
  return String(value ?? '')
    .replace(/\s+/g, ' ')
    .trim()
    .slice(0, 120)
}

function normalizeStatus(value: unknown): ServerClientStatus {
  const normalized = String(value ?? '')
    .trim()
    .toLowerCase()
  if (['inactive', 'false', '0', 'off'].includes(normalized)) {
    return 'inactive'
  }

  return 'active'
}

function normalizeBillingMode(value: unknown): ServerClientBillingMode {
  const normalized = String(value ?? '')
    .trim()
    .toLowerCase()
  if (['per_store', 'por_loja', 'por loja', 'store'].includes(normalized)) {
    return 'per_store'
  }

  return 'single'
}

function normalizeModuleCode(value: unknown) {
  return String(value ?? '')
    .trim()
    .toLowerCase()
    .replace(/\s+/g, '_')
}

function moduleLabelByCode(code: string) {
  const normalized = normalizeModuleCode(code)
  const labels: Record<string, string> = {
    core_panel: 'Core Panel',
    atendimento: 'Atendimento',
    'fila-atendimento': 'Fila de Atendimento',
    indicators: 'Indicadores',
    finance: 'Finance',
    kanban: 'Kanban',
  }

  return labels[normalized] || normalized.replace(/_/g, ' ')
}

function normalizeModules(value: unknown) {
  const source = Array.isArray(value) ? value : String(value ?? '').split(/[\n,;|]+/)
  const dedupe = new Set<string>()
  const modules: ServerClientModule[] = []

  for (const entry of source) {
    const code = normalizeModuleCode(
      typeof entry === 'string'
        ? entry
        : entry && typeof entry === 'object' && 'code' in entry
          ? (entry as { code?: unknown }).code
          : entry,
    )
    if (!code || dedupe.has(code)) {
      continue
    }

    dedupe.add(code)
    modules.push({
      code,
      name: moduleLabelByCode(code),
      status: 'active',
    })
  }

  if (!dedupe.has('core_panel')) {
    modules.unshift({ code: 'core_panel', name: moduleLabelByCode('core_panel'), status: 'active' })
  }

  return modules.slice(0, 24)
}

function buildStoreId(clientId: number, index: number) {
  return `client-${clientId}-store-${Date.now()}-${index}`
}

function normalizeStores(clientId: number, input: ClientStoreInput[]) {
  const unique = new Set<string>()
  const stores: ServerClientStore[] = []

  for (const [index, item] of input.entries()) {
    const name = normalizeStoreName(item.name)
    if (!name) {
      continue
    }

    const dedupeKey = name.toLowerCase()
    if (unique.has(dedupeKey)) {
      continue
    }

    unique.add(dedupeKey)
    stores.push({
      id: String(item.id || buildStoreId(clientId, index)),
      name,
      amount: toAmount(item.amount),
    })
  }

  return stores
}

function toDto(client: ServerClient) {
  const activeModules = client.modules.filter((module) => module.status === 'active')

  return {
    id: client.id,
    coreTenantId: client.coreTenantId,
    name: client.name,
    status: client.status,
    userCount: client.userCount,
    userNicks: client.userNicks.join(', '),
    projectCount: client.projectCount,
    projectSegments: client.projectSegments.join(', '),
    billingMode: client.billingMode,
    monthlyPaymentAmount: client.monthlyPaymentAmount,
    paymentDueDay: client.paymentDueDay,
    stores: cloneValue(client.stores),
    storesCount: client.stores.length,
    modules: cloneValue(activeModules),
    moduleCodes: activeModules.map((module) => module.code),
    logo: client.logo,
    webhookEnabled: client.webhookEnabled,
    webhookKey: client.webhookKey,
    contactPhone: client.contactPhone,
    contactSite: client.contactSite,
    contactAddress: client.contactAddress,
    requireUserStoreLink: client.requireUserStoreLink,
    requireUserRegistration: client.requireUserRegistration,
  }
}

function normalizeViewerClientId(value: unknown) {
  const parsed = Number.parseInt(String(value ?? '').trim(), 10)
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return 0
  }

  return parsed
}

function canAccessClient(
  client: ServerClient,
  options?: { viewerUserType?: 'admin' | 'client'; viewerClientId?: number },
) {
  const viewerUserType = String(options?.viewerUserType ?? 'admin')
    .trim()
    .toLowerCase()
  if (viewerUserType !== 'client') {
    return true
  }

  const viewerClientId = normalizeViewerClientId(options?.viewerClientId)
  if (viewerClientId <= 0) {
    return false
  }

  return client.id === viewerClientId
}

function seedClients(): ServerClient[] {
  return [
    {
      id: 106,
      coreTenantId: '106',
      name: 'crow',
      status: 'active',
      userCount: 4,
      userNicks: ['Antonny', 'Jessica', 'Mike', 'Equipe'],
      projectCount: 0,
      projectSegments: [],
      billingMode: 'single',
      monthlyPaymentAmount: 0,
      paymentDueDay: '',
      stores: [],
      modules: normalizeModules(['core_panel', 'atendimento', 'fila-atendimento']),
      logo: '',
      webhookEnabled: true,
      webhookKey: 'crw_live_8f2be0a1',
      contactPhone: '',
      contactSite: '',
      contactAddress: '',
      requireUserStoreLink: false,
      requireUserRegistration: true,
    },
    {
      id: 105,
      coreTenantId: '105',
      name: 'Perola',
      status: 'active',
      userCount: 1,
      userNicks: ['Cliente Perola'],
      projectCount: 18,
      projectSegments: ['finance', 'marketing', 'sales'],
      billingMode: 'per_store',
      monthlyPaymentAmount: 4700,
      paymentDueDay: '05',
      stores: [
        { id: 'st-1', name: 'Perola Garcia', amount: 1200 },
        { id: 'st-2', name: 'Perola Riomar', amount: 1200 },
        { id: 'st-3', name: 'Perola Jardins', amount: 1100 },
        { id: 'st-4', name: 'Perola Treze', amount: 1200 },
      ],
      modules: normalizeModules(['core_panel', 'atendimento']),
      logo: '',
      webhookEnabled: true,
      webhookKey: 'prl_live_23dce89e',
      contactPhone: '',
      contactSite: '',
      contactAddress: '',
      requireUserStoreLink: true,
      requireUserRegistration: true,
    },
    {
      id: 104,
      coreTenantId: '104',
      name: 'Dr Antonio Tavares',
      status: 'active',
      userCount: 0,
      userNicks: [],
      projectCount: 5,
      projectSegments: ['finance', 'marketing', 'vendas'],
      billingMode: 'single',
      monthlyPaymentAmount: 250,
      paymentDueDay: '10',
      stores: [],
      modules: normalizeModules(['core_panel']),
      logo: '',
      webhookEnabled: false,
      webhookKey: 'drt_live_029ab9d1',
      contactPhone: '',
      contactSite: '',
      contactAddress: '',
      requireUserStoreLink: true,
      requireUserRegistration: false,
    },
    {
      id: 103,
      coreTenantId: '103',
      name: 'Zen as Fuck',
      status: 'active',
      userCount: 0,
      userNicks: [],
      projectCount: 5,
      projectSegments: ['marketing'],
      billingMode: 'single',
      monthlyPaymentAmount: 3500,
      paymentDueDay: '',
      stores: [],
      modules: normalizeModules(['core_panel']),
      logo: '',
      webhookEnabled: true,
      webhookKey: 'znf_live_7bcaa021',
      contactPhone: '',
      contactSite: '',
      contactAddress: '',
      requireUserStoreLink: false,
      requireUserRegistration: false,
    },
    {
      id: 102,
      coreTenantId: '102',
      name: 'Duby',
      status: 'active',
      userCount: 0,
      userNicks: [],
      projectCount: 0,
      projectSegments: [],
      billingMode: 'single',
      monthlyPaymentAmount: 0,
      paymentDueDay: '01',
      stores: [],
      modules: normalizeModules(['core_panel']),
      logo: '',
      webhookEnabled: false,
      webhookKey: 'dby_live_99210ab2',
      contactPhone: '',
      contactSite: '',
      contactAddress: '',
      requireUserStoreLink: false,
      requireUserRegistration: true,
    },
    {
      id: 101,
      coreTenantId: '101',
      name: 'sdfsodifho',
      status: 'active',
      userCount: 0,
      userNicks: [],
      projectCount: 0,
      projectSegments: [],
      billingMode: 'single',
      monthlyPaymentAmount: 0,
      paymentDueDay: '',
      stores: [],
      modules: normalizeModules(['core_panel']),
      logo: '',
      webhookEnabled: false,
      webhookKey: 'sdf_live_5ddaf7aa',
      contactPhone: '',
      contactSite: '',
      contactAddress: '',
      requireUserStoreLink: false,
      requireUserRegistration: false,
    },
  ]
}

function getState(): RepositoryState {
  const globalRef = globalThis as typeof globalThis & { [globalKey]?: RepositoryState }
  if (!globalRef[globalKey]) {
    const seeded = seedClients().sort((a, b) => b.id - a.id)
    globalRef[globalKey] = {
      clients: seeded,
      nextId: Math.max(...seeded.map((item) => item.id)) + 1,
    }
  }

  return globalRef[globalKey] as RepositoryState
}

export function listClients(options: ClientListOptions) {
  const state = getState()
  const page = Number.isFinite(options.page) && options.page > 0 ? options.page : 1
  const limit =
    Number.isFinite(options.limit) && options.limit > 0 ? Math.min(options.limit, 100) : 20
  const q = String(options.q ?? '')
    .trim()
    .toLowerCase()
  const statusFilter = String(options.status ?? '')
    .trim()
    .toLowerCase()

  const filtered = state.clients.filter((client) => {
    if (!canAccessClient(client, options)) {
      return false
    }
    if (statusFilter && statusFilter !== 'all' && client.status !== statusFilter) {
      return false
    }
    if (!q) {
      return true
    }

    const haystack = [
      client.name,
      client.userNicks.join(' '),
      client.projectSegments.join(' '),
      client.contactPhone,
      client.contactSite,
      client.contactAddress,
      client.modules.map((module) => module.name).join(' '),
    ]
      .join(' ')
      .toLowerCase()

    return haystack.includes(q)
  })

  const total = filtered.length
  const totalPages = Math.max(1, Math.ceil(total / limit))
  const safePage = Math.min(page, totalPages)
  const start = (safePage - 1) * limit
  const items = filtered.slice(start, start + limit).map(toDto)

  return {
    items,
    meta: {
      page: safePage,
      limit,
      total,
      totalPages,
      hasMore: safePage < totalPages,
    } satisfies ClientsListMeta,
  }
}

export function createClient(input: ClientCreateInput = {}) {
  const state = getState()
  const id = state.nextId++

  const created: ServerClient = {
    id,
    coreTenantId: String(id),
    name: normalizeClientName(input.name) || `Novo cliente ${id}`,
    status: normalizeStatus(input.status),
    userCount: 0,
    userNicks: [],
    projectCount: 0,
    projectSegments: [],
    billingMode: 'single',
    monthlyPaymentAmount: 0,
    paymentDueDay: '',
    stores: [],
    modules: normalizeModules(['core_panel']),
    logo: '',
    webhookEnabled: false,
    webhookKey: createWebhookKey(id),
    contactPhone: '',
    contactSite: '',
    contactAddress: '',
    requireUserStoreLink: false,
    requireUserRegistration: true,
  }

  state.clients.unshift(created)
  return toDto(created)
}

export function deleteClientById(id: number) {
  const state = getState()
  const index = state.clients.findIndex((client) => client.id === id)
  if (index < 0) {
    return false
  }

  state.clients.splice(index, 1)
  return true
}

export function updateClientField(
  id: number,
  field: string,
  value: unknown,
  options?: { viewerUserType?: 'admin' | 'client'; viewerClientId?: number },
) {
  const state = getState()
  const target = state.clients.find((client) => client.id === id)
  if (!target || !canAccessClient(target, options)) {
    return null
  }

  if (field === 'name') target.name = normalizeClientName(value) || target.name
  if (field === 'status') target.status = normalizeStatus(value)
  if (field === 'billingMode') {
    target.billingMode = normalizeBillingMode(value)
    if (target.billingMode === 'single') {
      target.stores = []
    }
  }
  if (field === 'monthlyPaymentAmount') target.monthlyPaymentAmount = toAmount(value)
  if (field === 'paymentDueDay') target.paymentDueDay = normalizeDueDay(value)
  if (field === 'userCount') {
    const parsed = Number(value)
    target.userCount = Number.isFinite(parsed) ? Math.max(0, Math.round(parsed)) : target.userCount
  }
  if (field === 'projectCount') {
    const parsed = Number(value)
    target.projectCount = Number.isFinite(parsed)
      ? Math.max(0, Math.round(parsed))
      : target.projectCount
  }
  if (field === 'userNicks') {
    target.userNicks = String(value ?? '')
      .split(/[\n,;|]+/)
      .map((item) => item.trim())
      .filter(Boolean)
      .slice(0, 30)
  }
  if (field === 'projectSegments') {
    target.projectSegments = String(value ?? '')
      .split(/[\n,;|]+/)
      .map((item) => item.trim())
      .filter(Boolean)
      .slice(0, 30)
  }
  if (field === 'logo') target.logo = normalizeShortText(value, 255)
  if (field === 'webhookEnabled') target.webhookEnabled = normalizeWebhookEnabled(value)
  if (field === 'contactPhone') target.contactPhone = normalizeShortText(value, 60)
  if (field === 'contactSite') target.contactSite = normalizeSite(value)
  if (field === 'contactAddress') target.contactAddress = normalizeShortText(value, 255)
  if (field === 'requireUserStoreLink')
    target.requireUserStoreLink = normalizeBooleanLike(value, target.requireUserStoreLink)
  if (field === 'requireUserRegistration')
    target.requireUserRegistration = normalizeBooleanLike(value, target.requireUserRegistration)
  if (field === 'modules') target.modules = normalizeModules(value)

  return toDto(target)
}

export function replaceClientStores(
  id: number,
  storesInput: ClientStoreInput[],
  options?: { viewerUserType?: 'admin' | 'client'; viewerClientId?: number },
) {
  const state = getState()
  const target = state.clients.find((client) => client.id === id)
  if (!target || !canAccessClient(target, options)) {
    return null
  }

  const normalized = normalizeStores(id, storesInput)
  target.stores = normalized

  if (normalized.length > 0) {
    target.billingMode = 'per_store'
    target.monthlyPaymentAmount = Number(
      normalized.reduce((sum, store) => sum + store.amount, 0).toFixed(2),
    )
  } else {
    target.monthlyPaymentAmount = 0
  }

  return toDto(target)
}

export function rotateClientWebhookKey(
  id: number,
  options?: { viewerUserType?: 'admin' | 'client'; viewerClientId?: number },
) {
  const state = getState()
  const target = state.clients.find((client) => client.id === id)
  if (!target || !canAccessClient(target, options)) {
    return null
  }

  target.webhookKey = createWebhookKey(id)
  return toDto(target)
}
