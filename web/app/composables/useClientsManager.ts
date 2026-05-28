import type {
  ClientBillingMode,
  ClientFieldKey,
  ClientItem,
  ClientMutationResponse,
  ClientsListResponse,
  ClientStatus,
  ClientStoreCharge,
} from '~/types/clients'

interface ClientFilters {
  q: string
  status: 'all' | ClientStatus
}

interface UpdateFieldOptions {
  immediate?: boolean
}

interface ClientContactPayload {
  logo: string
  contactPhone: string
  contactSite: string
  contactAddress: string
}

interface CreateClientPayload {
  name?: string
  status?: ClientStatus
  adminName?: string
  adminEmail?: string
  adminPassword?: string
}

const UPDATE_DELAY_MS = 380
const DEFAULT_FETCH_LIMIT = 80
const KNOWN_MODULE_LABELS: Record<string, string> = {
  core_panel: 'Core Panel',
  atendimento: 'Atendimento',
  'fila-atendimento': 'Fila de Atendimento',
  indicators: 'Indicadores',
  finance: 'Finance',
  kanban: 'Kanban',
}

function normalizeModuleCode(value: unknown) {
  return String(value ?? '')
    .trim()
    .toLowerCase()
    .replace(/\s+/g, '_')
}

function normalizeModuleCodes(value: unknown) {
  const source = Array.isArray(value) ? value : String(value ?? '').split(/[\n,;|]+/)

  const dedupe = new Set<string>()
  const output: string[] = []

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
    output.push(code)
  }

  if (!dedupe.has('core_panel')) {
    output.unshift('core_panel')
  }

  return output.slice(0, 24)
}

function moduleLabelByCode(code: string) {
  const normalized = normalizeModuleCode(code)
  if (!normalized) {
    return ''
  }

  return KNOWN_MODULE_LABELS[normalized] || normalized.replace(/_/g, ' ')
}

function parseAmount(value: unknown) {
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

  const parsed = Number(digits)
  if (!Number.isFinite(parsed) || parsed < 1) {
    return ''
  }

  return String(Math.min(parsed, 31)).padStart(2, '0')
}

function normalizeInteger(value: unknown) {
  const parsed = Number.parseInt(String(value ?? '').replace(/\D/g, ''), 10)
  if (!Number.isFinite(parsed)) {
    return 0
  }

  return Math.max(0, parsed)
}

function normalizeListText(value: unknown) {
  return String(value ?? '')
    .split(/[\n,;|]+/)
    .map((item) => item.trim())
    .filter(Boolean)
    .slice(0, 30)
    .join(', ')
}

function normalizeBillingMode(value: unknown): ClientBillingMode {
  const normalized = String(value ?? '')
    .trim()
    .toLowerCase()
  return normalized === 'per_store' ? 'per_store' : 'single'
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

function normalizeStatus(value: unknown): ClientStatus {
  const normalized = String(value ?? '')
    .trim()
    .toLowerCase()
  return normalized === 'inactive' ? 'inactive' : 'active'
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

function cloneStores(stores: ClientStoreCharge[]) {
  return stores.map((store) => ({
    id: String(store.id),
    name: String(store.name || '').trim(),
    amount: parseAmount(store.amount),
  }))
}

function normalizeStores(stores: ClientStoreCharge[]) {
  const dedupe = new Set<string>()
  return cloneStores(stores).filter((store) => {
    if (!store.name) {
      return false
    }

    const key = store.name.toLowerCase()
    if (dedupe.has(key)) {
      return false
    }

    dedupe.add(key)
    return true
  })
}

function storesTotal(stores: ClientStoreCharge[]) {
  return Number(stores.reduce((sum, store) => sum + parseAmount(store.amount), 0).toFixed(2))
}

function normalizeClientItem(item: ClientItem): ClientItem {
  const stores = Array.isArray(item.stores) ? normalizeStores(item.stores) : []
  const modules = Array.isArray(item.modules)
    ? item.modules
        .map((module) => {
          const code = normalizeModuleCode(module?.code)
          if (!code) {
            return null
          }

          return {
            code,
            name: String(module?.name ?? '').trim() || moduleLabelByCode(code),
            status:
              String(module?.status ?? 'active')
                .trim()
                .toLowerCase() || 'active',
          }
        })
        .filter((module): module is NonNullable<typeof module> => Boolean(module))
    : []

  const activeModules = modules.filter((module) => module.status === 'active')
  const billingMode = normalizeBillingMode(item.billingMode)

  return {
    ...item,
    coreTenantId: String(item.coreTenantId ?? '').trim() || String(item.id),
    billingMode,
    monthlyPaymentAmount:
      billingMode === 'per_store' ? storesTotal(stores) : parseAmount(item.monthlyPaymentAmount),
    stores,
    storesCount: stores.length,
    requireUserStoreLink: normalizeBooleanLike(item.requireUserStoreLink, true),
    requireUserRegistration: normalizeBooleanLike(item.requireUserRegistration, true),
    modules: activeModules,
    moduleCodes: normalizeModuleCodes(activeModules.map((module) => module.code)),
  }
}

export function useClientsManager() {
  const sessionSimulation = useSessionSimulationStore()
  const { bffFetch } = useBffFetch()
  const { coreToken, hydrate } = useAdminSession()
  const realtime = useTenantRealtime()
  realtime.start()

  const clients = ref<ClientItem[]>([])
  const filters = reactive<ClientFilters>({
    q: '',
    status: 'all',
  })

  const loading = ref(false)
  const creating = ref(false)
  const deletingId = ref<number | null>(null)
  const errorMessage = ref('')
  const savingMap = ref<Record<string, boolean>>({})

  const canResetFilters = computed(() => filters.q.trim() !== '' || filters.status !== 'all')

  const pendingFieldTimers = new Map<string, ReturnType<typeof setTimeout>>()
  let searchTimer: ReturnType<typeof setTimeout> | null = null

  function keyFor(id: number, field: ClientFieldKey | 'stores' | 'delete' | 'create') {
    return `${id}:${field}`
  }

  function isAbortError(error: unknown) {
    return (
      typeof error === 'object' &&
      error !== null &&
      'name' in error &&
      (error as { name?: string }).name === 'AbortError'
    )
  }

  function extractStatusCode(error: unknown) {
    if (typeof error !== 'object' || error === null) {
      return 0
    }

    const statusCode = Number((error as { statusCode?: unknown }).statusCode)
    if (Number.isFinite(statusCode) && statusCode > 0) {
      return statusCode
    }

    const status = Number((error as { status?: unknown }).status)
    if (Number.isFinite(status) && status > 0) {
      return status
    }

    return 0
  }

  function setSaving(key: string, value: boolean) {
    const next = { ...savingMap.value }
    if (value) {
      next[key] = true
    } else {
      delete next[key]
    }

    savingMap.value = next
  }

  function getClientRouteId(id: number) {
    const target = clients.value.find((client) => client.id === id)
    return String(target?.coreTenantId ?? id).trim()
  }

  function rowIsSaving(id: number) {
    const prefix = `${id}:`
    return Object.keys(savingMap.value).some((key) => key.startsWith(prefix))
  }

  function syncSimulationClientOptions() {
    sessionSimulation.replaceClientOptions(
      clients.value.map((client) => ({
        value: Number(client.id),
        label: String(client.name ?? '').trim() || `Cliente #${client.id}`,
        coreTenantId: String(client.coreTenantId ?? '').trim(),
        moduleCodes: normalizeModuleCodes(client.moduleCodes),
      })),
    )
  }

  function replaceClientRow(payload: ClientItem) {
    const normalized = normalizeClientItem(payload)
    const index = clients.value.findIndex((client) => client.id === payload.id)
    if (index < 0) {
      clients.value.unshift(normalized)
      syncSimulationClientOptions()
      return
    }

    clients.value[index] = normalized
    syncSimulationClientOptions()
  }

  function patchClientLocally(id: number, patch: Partial<ClientItem>) {
    const target = clients.value.find((client) => client.id === id)
    if (!target) {
      return
    }

    Object.assign(target, patch)
  }

  async function ensureSessionScopeReady() {
    hydrate()
    sessionSimulation.initialize()

    if (!coreToken.value) {
      return
    }

    if (!sessionSimulation.clientOptionsSynced) {
      await sessionSimulation.refreshClientOptions()
    }
  }

  async function fetchClients() {
    loading.value = true
    errorMessage.value = ''

    try {
      await ensureSessionScopeReady()

      const response = await bffFetch<ClientsListResponse>('/api/admin/clients', {
        query: {
          q: filters.q.trim() || undefined,
          status: filters.status !== 'all' ? filters.status : undefined,
          page: 1,
          limit: DEFAULT_FETCH_LIMIT,
        },
      })

      clients.value = Array.isArray(response.data)
        ? response.data.map((item) => normalizeClientItem(item))
        : []
      syncSimulationClientOptions()
    } catch (error) {
      if (isAbortError(error)) {
        return
      }

      const statusCode = extractStatusCode(error)
      if (statusCode === 401 || statusCode === 403) {
        errorMessage.value = 'Voce nao tem permissao para acessar clientes nesta sessao.'
      } else {
        errorMessage.value = 'Falha ao carregar clientes.'
      }
    } finally {
      loading.value = false
    }
  }

  async function persistField(id: number, field: ClientFieldKey, value: unknown) {
    const savingKey = keyFor(id, field)
    setSaving(savingKey, true)
    errorMessage.value = ''

    try {
      const routeId = getClientRouteId(id)
      if (!routeId) {
        throw new Error('Cliente sem routeId para atualizacao.')
      }

      const response = await bffFetch<ClientMutationResponse>(
        `/api/admin/clients/${encodeURIComponent(routeId)}`,
        {
          method: 'PATCH',
          body: {
            field,
            value,
          },
        },
      )

      replaceClientRow(response.data)
    } catch {
      errorMessage.value = 'Falha ao salvar alteracao do cliente.'
      await fetchClients()
    } finally {
      setSaving(savingKey, false)
    }
  }

  function queueFieldPersist(
    id: number,
    field: ClientFieldKey,
    value: unknown,
    options: UpdateFieldOptions = {},
  ) {
    const timerKey = keyFor(id, field)
    const existingTimer = pendingFieldTimers.get(timerKey)
    if (existingTimer) {
      clearTimeout(existingTimer)
      pendingFieldTimers.delete(timerKey)
    }

    const run = () => {
      pendingFieldTimers.delete(timerKey)
      void persistField(id, field, value)
    }

    if (options.immediate) {
      run()
      return
    }

    const timer = setTimeout(run, UPDATE_DELAY_MS)
    pendingFieldTimers.set(timerKey, timer)
  }

  function updateField(
    id: number,
    field: ClientFieldKey,
    value: unknown,
    options: UpdateFieldOptions = {},
  ) {
    const target = clients.value.find((client) => client.id === id)

    if (field === 'name') {
      patchClientLocally(id, { name: String(value ?? '').slice(0, 120) })
      syncSimulationClientOptions()
    }

    if (field === 'status') {
      patchClientLocally(id, { status: normalizeStatus(value) })
    }

    if (field === 'userCount') {
      patchClientLocally(id, { userCount: normalizeInteger(value) })
    }

    if (field === 'userNicks') {
      patchClientLocally(id, { userNicks: normalizeListText(value) })
    }

    if (field === 'projectCount') {
      patchClientLocally(id, { projectCount: normalizeInteger(value) })
    }

    if (field === 'projectSegments') {
      patchClientLocally(id, { projectSegments: normalizeListText(value) })
    }

    if (field === 'billingMode') {
      const mode = normalizeBillingMode(value)
      const nextPatch: Partial<ClientItem> = {
        billingMode: mode,
      }
      if (mode === 'single') {
        nextPatch.stores = []
      } else {
        nextPatch.monthlyPaymentAmount = storesTotal(target?.stores || [])
      }

      patchClientLocally(id, nextPatch)
    }

    if (field === 'monthlyPaymentAmount') {
      if (target?.billingMode === 'per_store') {
        patchClientLocally(id, { monthlyPaymentAmount: storesTotal(target.stores || []) })
        return
      }

      patchClientLocally(id, { monthlyPaymentAmount: parseAmount(value) })
    }

    if (field === 'paymentDueDay') {
      patchClientLocally(id, { paymentDueDay: normalizeDueDay(value) })
    }

    if (field === 'logo') {
      patchClientLocally(id, {
        logo: String(value ?? '')
          .trim()
          .slice(0, 255),
      })
    }

    if (field === 'webhookEnabled') {
      patchClientLocally(id, { webhookEnabled: normalizeWebhookEnabled(value) })
    }

    if (field === 'contactPhone') {
      patchClientLocally(id, {
        contactPhone: String(value ?? '')
          .trim()
          .slice(0, 60),
      })
    }

    if (field === 'contactSite') {
      patchClientLocally(id, { contactSite: normalizeSite(value) })
    }

    if (field === 'contactAddress') {
      patchClientLocally(id, {
        contactAddress: String(value ?? '')
          .trim()
          .slice(0, 255),
      })
    }

    if (field === 'requireUserStoreLink') {
      patchClientLocally(id, { requireUserStoreLink: normalizeBooleanLike(value, true) })
    }

    if (field === 'requireUserRegistration') {
      patchClientLocally(id, { requireUserRegistration: normalizeBooleanLike(value, true) })
    }

    if (field === 'modules') {
      const moduleCodes = normalizeModuleCodes(value)
      patchClientLocally(id, {
        moduleCodes,
        modules: moduleCodes.map((code) => ({
          code,
          name: moduleLabelByCode(code),
          status: 'active',
        })),
      })
      syncSimulationClientOptions()
    }

    queueFieldPersist(id, field, value, options)
  }

  async function saveContactAndLogo(id: number, payload: ClientContactPayload) {
    const normalizedPayload = {
      logo: String(payload.logo ?? '')
        .trim()
        .slice(0, 255),
      contactPhone: String(payload.contactPhone ?? '')
        .trim()
        .slice(0, 60),
      contactSite: normalizeSite(payload.contactSite),
      contactAddress: String(payload.contactAddress ?? '')
        .trim()
        .slice(0, 255),
    }

    patchClientLocally(id, normalizedPayload)

    const keys: Array<{ field: ClientFieldKey; value: string }> = [
      { field: 'logo', value: normalizedPayload.logo },
      { field: 'contactPhone', value: normalizedPayload.contactPhone },
      { field: 'contactSite', value: normalizedPayload.contactSite },
      { field: 'contactAddress', value: normalizedPayload.contactAddress },
    ]

    await Promise.all(
      keys.map(async ({ field, value }) => {
        await persistField(id, field, value)
      }),
    )
  }

  async function saveWebhookEnabled(id: number, enabled: boolean) {
    await persistField(id, 'webhookEnabled', enabled ? 'true' : 'false')
  }

  async function rotateWebhookKey(id: number) {
    const savingKey = keyFor(id, 'webhookEnabled')
    setSaving(savingKey, true)
    errorMessage.value = ''

    try {
      const routeId = getClientRouteId(id)
      if (!routeId) {
        throw new Error('Cliente sem routeId para webhook.')
      }

      const response = await bffFetch<ClientMutationResponse>(
        `/api/admin/clients/${encodeURIComponent(routeId)}/webhook/rotate`,
        {
          method: 'POST',
        },
      )

      replaceClientRow(response.data)
    } catch {
      errorMessage.value = 'Falha ao gerar nova chave webhook.'
      await fetchClients()
    } finally {
      setSaving(savingKey, false)
    }
  }

  async function saveStores(id: number, stores: ClientStoreCharge[]) {
    const normalizedStores = normalizeStores(stores)
    const savingKey = keyFor(id, 'stores')

    const nextPatch: Partial<ClientItem> = {
      stores: normalizedStores,
      monthlyPaymentAmount: normalizedStores.length ? storesTotal(normalizedStores) : 0,
    }
    if (normalizedStores.length > 0) {
      nextPatch.billingMode = 'per_store'
    }

    patchClientLocally(id, nextPatch)

    setSaving(savingKey, true)
    errorMessage.value = ''

    try {
      const routeId = getClientRouteId(id)
      if (!routeId) {
        throw new Error('Cliente sem routeId para lojas.')
      }

      const response = await bffFetch<ClientMutationResponse>(
        `/api/admin/clients/${encodeURIComponent(routeId)}/stores`,
        {
          method: 'PUT',
          body: {
            stores: normalizedStores,
          },
        },
      )

      replaceClientRow(response.data)
    } catch {
      errorMessage.value = 'Falha ao salvar lojas do cliente.'
      await fetchClients()
    } finally {
      setSaving(savingKey, false)
    }
  }

  async function createClient(payload: CreateClientPayload = {}) {
    creating.value = true
    setSaving(keyFor(0, 'create'), true)
    errorMessage.value = ''

    try {
      const response = await bffFetch<ClientMutationResponse>('/api/admin/clients', {
        method: 'POST',
        body: {
          name: payload.name ?? 'Novo cliente',
          status: payload.status ?? 'active',
          adminName: payload.adminName ?? '',
          adminEmail: payload.adminEmail ?? '',
          adminPassword: payload.adminPassword ?? '',
        },
      })

      clients.value.unshift(normalizeClientItem(response.data))
      syncSimulationClientOptions()
      return response.data.id
    } catch {
      errorMessage.value = 'Falha ao criar cliente.'
      return null
    } finally {
      creating.value = false
      setSaving(keyFor(0, 'create'), false)
    }
  }

  async function deleteClient(id: number) {
    deletingId.value = id
    setSaving(keyFor(id, 'delete'), true)
    errorMessage.value = ''

    try {
      const routeId = getClientRouteId(id)
      if (!routeId) {
        throw new Error('Cliente sem routeId para exclusao.')
      }

      await bffFetch<{ status: 'success' }>(`/api/admin/clients/${encodeURIComponent(routeId)}`, {
        method: 'DELETE',
      })

      clients.value = clients.value.filter((client) => client.id !== id)
      syncSimulationClientOptions()
    } catch {
      errorMessage.value = 'Falha ao excluir cliente.'
    } finally {
      deletingId.value = null
      setSaving(keyFor(id, 'delete'), false)
    }
  }

  function resetFilters() {
    filters.q = ''
    filters.status = 'all'
    void fetchClients()
  }

  watch(
    () => filters.status,
    () => {
      void fetchClients()
    },
  )

  watch(
    () => filters.q,
    () => {
      if (searchTimer) {
        clearTimeout(searchTimer)
      }

      searchTimer = setTimeout(() => {
        void fetchClients()
      }, 280)
    },
  )

  watch(
    () => sessionSimulation.requestContextHash,
    () => {
      void fetchClients()
    },
  )

  const stopRealtimeSubscription = realtime.subscribeEntity('clients', () => {
    void fetchClients()
  })

  onScopeDispose(() => {
    stopRealtimeSubscription()
  })

  onBeforeUnmount(() => {
    if (searchTimer) {
      clearTimeout(searchTimer)
    }

    for (const timer of pendingFieldTimers.values()) {
      clearTimeout(timer)
    }

    pendingFieldTimers.clear()
  })

  return {
    clients,
    filters,
    loading,
    creating,
    deletingId,
    errorMessage,
    savingMap,
    canResetFilters,
    rowIsSaving,
    fetchClients,
    updateField,
    saveContactAndLogo,
    saveWebhookEnabled,
    rotateWebhookKey,
    saveStores,
    createClient,
    deleteClient,
    resetFilters,
  }
}
