import type {
  ProductCreateInput,
  ProductFieldKey,
  ProductItem,
  ProductStatus,
} from '~/types/products'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

const PATCH_DELAY_MS = 380

const FIELD_TO_PATCH: Record<ProductFieldKey, string> = {
  name: 'name',
  code: 'code',
  description: 'description',
  image: 'image',
  categories: 'categories',
  campaigns: 'campaigns',
  price: 'price',
  fator: 'fator',
  tipo: 'tipo',
  stock: 'stock',
  status: 'status',
}

function normalizeProduct(raw: Record<string, unknown>): ProductItem {
  const status = String(raw.status ?? 'active')
  const valid: ProductStatus = status === 'inactive' ? 'inactive' : 'active'
  return {
    id: String(raw.id ?? ''),
    accountId: String(raw.accountId ?? ''),
    sourceId: String(raw.sourceId ?? ''),
    sourceLabel: String(raw.sourceLabel ?? ''),
    name: String(raw.name ?? ''),
    code: String(raw.code ?? ''),
    description: String(raw.description ?? ''),
    image: String(raw.image ?? ''),
    categories: Array.isArray(raw.categories) ? (raw.categories as string[]) : [],
    campaigns: Array.isArray(raw.campaigns) ? (raw.campaigns as string[]) : [],
    price: Number(raw.price ?? 0) || 0,
    fator: Number(raw.fator ?? 1) || 1,
    tipo: String(raw.tipo ?? ''),
    stock: Number(raw.stock ?? 0) || 0,
    status: valid,
    createdAt: String(raw.createdAt ?? ''),
    updatedAt: String(raw.updatedAt ?? ''),
  }
}

export function useProductsManager() {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  function accountHeaders() {
    return auth.activeTenantId ? { 'X-Account-Id': auth.activeTenantId } : {}
  }

  const products = ref<ProductItem[]>([])
  const filters = reactive({
    q: '',
    status: '' as '' | ProductStatus,
    category: '',
  })
  const loading = ref(false)
  const creating = ref(false)
  const deletingId = ref<string | null>(null)
  const errorMessage = ref('')
  const savingMap = ref<Record<string, boolean>>({})
  const canResetFilters = computed(() => Boolean(filters.q || filters.status || filters.category))

  const pendingTimers = new Map<string, ReturnType<typeof setTimeout>>()

  function setSaving(key: string, value: boolean) {
    const next = { ...savingMap.value }
    if (value) next[key] = true
    else delete next[key]
    savingMap.value = next
  }

  function rowIsSaving(id: string) {
    return Object.keys(savingMap.value).some((k) => k.startsWith(`${id}:`))
  }

  function applyPatch(id: string, raw: Record<string, unknown>) {
    const idx = products.value.findIndex((p) => p.id === id)
    if (idx >= 0) products.value[idx] = normalizeProduct(raw)
  }

  function patchLocal(id: string, field: ProductFieldKey, value: unknown) {
    const idx = products.value.findIndex((p) => p.id === id)
    if (idx < 0) return
    ;(products.value[idx] as Record<string, unknown>)[field] = value
  }

  async function fetchProducts() {
    loading.value = true
    errorMessage.value = ''
    try {
      const query = new URLSearchParams()
      if (filters.q) query.set('q', filters.q)
      if (filters.status) query.set('status', filters.status)
      if (filters.category) query.set('category', filters.category)
      const path = `/v1/admin/products${query.toString() ? `?${query.toString()}` : ''}`
      const resp = await apiRequest(path, { headers: accountHeaders() })
      products.value = (resp.products as Record<string, unknown>[]).map(normalizeProduct)
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao carregar produtos.')
    } finally {
      loading.value = false
    }
  }

  async function persistPatch(id: string, fieldKey: string, patch: Record<string, unknown>) {
    const key = `${id}:${fieldKey}`
    setSaving(key, true)
    try {
      const resp = await apiRequest(`/v1/admin/products/${encodeURIComponent(id)}`, {
        method: 'PATCH',
        body: patch,
        headers: accountHeaders(),
      })
      applyPatch(id, resp as Record<string, unknown>)
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao salvar.')
    } finally {
      setSaving(key, false)
    }
  }

  function updateField(
    id: string,
    field: ProductFieldKey,
    value: unknown,
    opts?: { immediate?: boolean },
  ) {
    patchLocal(id, field, value)
    const backendField = FIELD_TO_PATCH[field]
    if (!backendField) return

    const patch = { [backendField]: value }
    const timerKey = `${id}:${field}`

    if (pendingTimers.has(timerKey)) clearTimeout(pendingTimers.get(timerKey)!)

    if (opts?.immediate) {
      void persistPatch(id, field, patch)
      return
    }

    pendingTimers.set(
      timerKey,
      setTimeout(() => {
        pendingTimers.delete(timerKey)
        void persistPatch(id, field, patch)
      }, PATCH_DELAY_MS),
    )
  }

  async function createProduct(input: ProductCreateInput): Promise<string | null> {
    creating.value = true
    errorMessage.value = ''
    try {
      const resp = await apiRequest('/v1/admin/products', {
        method: 'POST',
        body: input,
        headers: accountHeaders(),
      })
      const created = normalizeProduct(resp as Record<string, unknown>)
      products.value.unshift(created)
      return created.id
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao criar produto.')
      return null
    } finally {
      creating.value = false
    }
  }

  async function deleteProduct(id: string) {
    deletingId.value = id
    setSaving(`${id}:delete`, true)
    errorMessage.value = ''
    try {
      await apiRequest(`/v1/admin/products/${encodeURIComponent(id)}`, {
        method: 'DELETE',
        headers: accountHeaders(),
      })
      products.value = products.value.filter((p) => p.id !== id)
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao excluir produto.')
    } finally {
      deletingId.value = null
      setSaving(`${id}:delete`, false)
    }
  }

  function resetFilters() {
    filters.q = ''
    filters.status = ''
    filters.category = ''
  }

  onBeforeUnmount(() => {
    for (const timer of pendingTimers.values()) clearTimeout(timer)
    pendingTimers.clear()
  })

  return {
    products,
    filters,
    loading,
    creating,
    deletingId,
    errorMessage,
    savingMap,
    canResetFilters,
    rowIsSaving,
    fetchProducts,
    updateField,
    createProduct,
    deleteProduct,
    resetFilters,
  }
}
