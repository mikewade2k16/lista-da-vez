import type {
  ProductFieldKey,
  ProductItem,
  ProductMutationResponse,
  ProductsListResponse,
} from '~/types/products'

interface UpdateFieldOptions {
  immediate?: boolean
}

interface CreateProductPayload {
  name?: string
  code?: string
  image?: string
}

const UPDATE_DELAY_MS = 340
const DEFAULT_FETCH_LIMIT = 80

function normalizeText(value: unknown, max = 255) {
  return String(value ?? '')
    .replace(/\s+/g, ' ')
    .trim()
    .slice(0, max)
}

function normalizeCode(value: unknown) {
  return normalizeText(value, 50)
}

function normalizeStatus(value: unknown): ProductItem['status'] {
  const normalized = String(value ?? '')
    .trim()
    .toLowerCase()
  return normalized === 'active' ? 'active' : 'desactive'
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

function normalizeNumber(value: unknown, decimals = 2) {
  const parsed = Number(String(value ?? '').replace(',', '.'))
  if (!Number.isFinite(parsed)) {
    return 0
  }

  const safe = Math.max(0, parsed)
  return Number(safe.toFixed(decimals))
}

function splitListText(value: unknown) {
  if (Array.isArray(value)) {
    return value
      .map((item) => normalizeText(item, 120))
      .filter(Boolean)
      .slice(0, 30)
  }

  return String(value ?? '')
    .split(/[\n,;|]+/)
    .map((item) => item.trim())
    .filter(Boolean)
    .slice(0, 30)
}

function joinListText(values: string[]) {
  return values.join(', ')
}

function uniqueSorted(values: string[]) {
  return Array.from(new Set(values.filter(Boolean))).sort((a, b) => a.localeCompare(b, 'pt-BR'))
}

export function useProductsManager() {
  const sessionSimulation = useSessionSimulationStore()
  const { bffFetch } = useBffFetch()
  const realtime = useTenantRealtime()
  realtime.start()

  const products = ref<ProductItem[]>([])
  const campaignOptions = ref<string[]>([])
  const categoryOptions = ref<string[]>([])

  const loading = ref(false)
  const creating = ref(false)
  const deletingId = ref<number | null>(null)
  const errorMessage = ref('')
  const savingMap = ref<Record<string, boolean>>({})

  const pendingFieldTimers = new Map<string, ReturnType<typeof setTimeout>>()

  function keyFor(id: number, field: ProductFieldKey | 'create' | 'delete') {
    return `${id}:${field}`
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

  function syncFilterOptionsFromRows() {
    campaignOptions.value = uniqueSorted(products.value.flatMap((item) => item.campaigns || []))
    categoryOptions.value = uniqueSorted(products.value.flatMap((item) => item.categories || []))
  }

  function replaceProductRow(payload: ProductItem) {
    const index = products.value.findIndex((product) => product.id === payload.id)
    if (index < 0) {
      products.value.unshift(payload)
      syncFilterOptionsFromRows()
      return
    }

    products.value[index] = payload
    syncFilterOptionsFromRows()
  }

  function patchProductLocally(id: number, patch: Partial<ProductItem>) {
    const target = products.value.find((product) => product.id === id)
    if (!target) {
      return
    }

    Object.assign(target, patch)
    syncFilterOptionsFromRows()
  }

  async function fetchProducts() {
    loading.value = true
    errorMessage.value = ''

    try {
      const response = await bffFetch<ProductsListResponse>('/api/admin/products', {
        query: {
          page: 1,
          limit: DEFAULT_FETCH_LIMIT,
          withDeleted: '1',
        },
      })

      products.value = Array.isArray(response.data) ? response.data : []
      campaignOptions.value = Array.isArray(response.filters?.campaigns)
        ? response.filters.campaigns
        : []
      categoryOptions.value = Array.isArray(response.filters?.categories)
        ? response.filters.categories
        : []
      if (campaignOptions.value.length === 0 || categoryOptions.value.length === 0) {
        syncFilterOptionsFromRows()
      }
    } catch {
      errorMessage.value = 'Falha ao carregar produtos.'
    } finally {
      loading.value = false
    }
  }

  async function persistField(id: number, field: ProductFieldKey, value: unknown) {
    const savingKey = keyFor(id, field)
    setSaving(savingKey, true)
    errorMessage.value = ''

    try {
      const response = await bffFetch<ProductMutationResponse>(`/api/admin/products/${id}`, {
        method: 'PATCH',
        body: {
          field,
          value,
        },
      })

      replaceProductRow(response.data)
    } catch {
      errorMessage.value = 'Falha ao salvar alteracao do produto.'
      await fetchProducts()
    } finally {
      setSaving(savingKey, false)
    }
  }

  function queueFieldPersist(
    id: number,
    field: ProductFieldKey,
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
    field: ProductFieldKey,
    value: unknown,
    options: UpdateFieldOptions = {},
  ) {
    if (field === 'name') {
      patchProductLocally(id, { name: normalizeText(value, 100) })
    }

    if (field === 'code') {
      patchProductLocally(id, { code: normalizeCode(value) })
    }

    if (field === 'image') {
      patchProductLocally(id, { image: normalizeText(value, 255) })
    }

    if (field === 'description') {
      patchProductLocally(id, { description: normalizeText(value, 4000) })
    }

    if (field === 'categories' || field === 'categoriesText') {
      const nextList = splitListText(value)
      patchProductLocally(id, {
        categories: nextList,
        categoriesText: joinListText(nextList),
      })
    }

    if (field === 'campaigns' || field === 'campaignsText') {
      const nextList = splitListText(value)
      patchProductLocally(id, {
        campaigns: nextList,
        campaignsText: joinListText(nextList),
      })
    }

    if (field === 'price') {
      patchProductLocally(id, { price: normalizeNumber(value, 2) })
    }

    if (field === 'fator') {
      patchProductLocally(id, { fator: normalizeNumber(value, 2) })
    }

    if (field === 'tipo') {
      patchProductLocally(id, { tipo: normalizeText(value, 50) })
    }

    if (field === 'stock') {
      patchProductLocally(id, { stock: normalizeStock(value) })
    }

    if (field === 'status') {
      patchProductLocally(id, { status: normalizeStatus(value) })
    }

    queueFieldPersist(id, field, value, options)
  }

  async function createProduct(payload?: CreateProductPayload) {
    creating.value = true
    setSaving(keyFor(0, 'create'), true)
    errorMessage.value = ''

    try {
      const response = await bffFetch<ProductMutationResponse>('/api/admin/products', {
        method: 'POST',
        body: {
          name: normalizeText(payload?.name ?? 'Novo Produto', 100),
          code: normalizeCode(payload?.code ?? ''),
          image: normalizeText(payload?.image ?? '', 255),
        },
      })

      products.value.unshift(response.data)
      syncFilterOptionsFromRows()
      return response.data.id
    } catch {
      errorMessage.value = 'Falha ao criar produto.'
      return null
    } finally {
      creating.value = false
      setSaving(keyFor(0, 'create'), false)
    }
  }

  async function deleteProduct(id: number) {
    deletingId.value = id
    setSaving(keyFor(id, 'delete'), true)
    errorMessage.value = ''

    try {
      const response = await bffFetch<ProductMutationResponse>(`/api/admin/products/${id}`, {
        method: 'DELETE',
      })

      replaceProductRow(response.data)
    } catch {
      errorMessage.value = 'Falha ao excluir produto.'
    } finally {
      deletingId.value = null
      setSaving(keyFor(id, 'delete'), false)
    }
  }

  async function uploadImage(id: number, file: File) {
    const savingKey = keyFor(id, 'image')
    setSaving(savingKey, true)
    errorMessage.value = ''

    try {
      const form = new FormData()
      form.append('file', file)

      const response = await bffFetch<ProductMutationResponse>(`/api/admin/products/${id}/image`, {
        method: 'POST',
        body: form,
      })

      replaceProductRow(response.data)
    } catch {
      errorMessage.value = 'Falha ao enviar imagem do produto.'
      await fetchProducts()
    } finally {
      setSaving(savingKey, false)
    }
  }

  onBeforeUnmount(() => {
    for (const timer of pendingFieldTimers.values()) {
      clearTimeout(timer)
    }

    pendingFieldTimers.clear()
  })

  watch(
    () => sessionSimulation.requestContextHash,
    () => {
      void fetchProducts()
    },
  )

  const stopRealtimeSubscription = realtime.subscribeEntity('products', () => {
    void fetchProducts()
  })

  onScopeDispose(() => {
    stopRealtimeSubscription()
  })

  return {
    products,
    campaignOptions,
    categoryOptions,
    loading,
    creating,
    deletingId,
    errorMessage,
    savingMap,
    fetchProducts,
    updateField,
    uploadImage,
    createProduct,
    deleteProduct,
  }
}
