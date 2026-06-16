import type {
  ProductCreateInput,
  ProductFacets,
  ProductFieldKey,
  ProductItem,
  ProductSourceMode,
  ProductStatus,
  ProductSyncResult,
} from '~/types/products'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import { useCoreAccountStore } from '../../layers/core/stores/account'
import { useProductsErp } from './useProductsErp'

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
  const stock = Number(raw.stock ?? 0) || 0
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
    stock,
    // Switch "Tem estoque" deriva do estoque numerico (>0 = visivel/em estoque).
    // O backend nao tem coluna hasStock; ao alternar, traduzimos para stock 0/1.
    hasStock: stock > 0,
    status: valid,
    createdAt: String(raw.createdAt ?? ''),
    updatedAt: String(raw.updatedAt ?? ''),
    // Cruzamento com o ERP — info adicional (nao confundir com name/description).
    erpSynced: raw.erpSynced === true,
    erpName: String(raw.erpName ?? ''),
    erpDescription: String(raw.erpDescription ?? ''),
  }
}

export function useProductsManager() {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const coreAccount = useCoreAccountStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  // Imagens cacheadas pelo sync ficam em /uploads/... (servido pela API, porta
  // 9091). No painel (3003) o caminho relativo apontaria pro front errado, entao
  // prefixamos o apiBase. URLs absolutas (origem externa) passam direto.
  const apiBase = String((runtimeConfig.public as Record<string, unknown>).apiBase || '').replace(
    /\/$/,
    '',
  )
  function absolutizeImage(url: string): string {
    return apiBase && url.startsWith('/uploads/') ? apiBase + url : url
  }

  // Cliente selecionado (admin). Vazio = usa a account ativa. Cada cliente pode
  // ter integracao/produtos diferentes, entao o admin escolhe qual ver/sincronizar.
  const selectedAccountId = ref('')

  // Base = account selecionada (filtro), senao a account GLOBAL (mesma que o
  // api-client injeta no X-Account-Id) — alinhar evita o mismatch em que a sync
  // gravava numa account e a listagem buscava noutra.
  function effectiveAccountId() {
    return String(
      selectedAccountId.value || coreAccount.activeAccountId || auth.activeTenantId || '',
    ).trim()
  }

  function accountHeaders() {
    const id = effectiveAccountId()
    return id ? { 'X-Account-Id': id } : {}
  }

  const products = ref<ProductItem[]>([])
  const total = ref(0)
  const filters = reactive({
    q: '',
    status: '' as '' | ProductStatus,
    category: '',
    campaign: '',
  })
  // Modo de listagem:
  // - 'paged' (DEFAULT): paginacao server-side com filtros (q/status/category/
  //            campaign) aplicados no backend; troca de pagina/filtro refaz o
  //            fetch. So 50 linhas por vez = render rapido (a API responde em
  //            ~30ms; o gargalo era montar ~826 linhas com celulas editaveis
  //            complexas de uma vez, levando ~1min ao recarregar).
  // - 'all'  : carrega o catalogo INTEIRO (perPage alto) e filtra client-side.
  //            Os dropdowns de categoria/campanha veem tudo nesse modo. Opcao
  //            pesada (render de tudo) — deixar so quando o usuario escolher.
  const mode = ref<'all' | 'paged'>('paged')
  const page = ref(1)
  const perPage = ref(50)
  const ALL_PER_PAGE = 5000
  const loading = ref(false)
  const creating = ref(false)
  const syncing = ref(false)
  // Fonte de produtos (toggle Local XAMPP / Online). 'online' e o default ate o
  // GET /v1/admin/products/source responder com o base_url real da account.
  const sourceMode = ref<ProductSourceMode>('online')
  const sourceLoading = ref(false)
  const deletingId = ref<string | null>(null)
  const errorMessage = ref('')
  const savingMap = ref<Record<string, boolean>>({})
  const canResetFilters = computed(() =>
    Boolean(filters.q || filters.status || filters.category || filters.campaign),
  )

  // Facets (categorias/campanhas/tipos distintos da account), carregados uma vez
  // e reusados nos selects de Categorias/Campanhas. Independem da paginacao —
  // mesmo endpoint do editor de bio (escopo por X-Account-Id).
  const facets = ref<ProductFacets>({ categories: [], campaigns: [], tipos: [] })
  const facetsLoaded = ref(false)
  const facetsLoading = ref(false)

  async function loadFacets(force = false): Promise<ProductFacets> {
    if (facetsLoaded.value && !force) return facets.value
    facetsLoading.value = true
    try {
      const resp = (await apiRequest('/v1/bio/sources/site_products/facets', {
        method: 'GET',
        headers: accountHeaders(),
      })) as Partial<ProductFacets> | null
      facets.value = {
        categories: Array.isArray(resp?.categories) ? resp.categories : [],
        campaigns: Array.isArray(resp?.campaigns) ? resp.campaigns : [],
        tipos: Array.isArray(resp?.tipos) ? resp.tipos : [],
      }
      facetsLoaded.value = true
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao carregar filtros de produtos.')
    } finally {
      facetsLoading.value = false
    }
    return facets.value
  }

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
      if (mode.value === 'paged') {
        // Paginacao server-side: filtros (q/status/category/campaign) vao para o
        // backend e so a pagina pedida volta. Mais novos primeiro (created_at desc).
        if (filters.q) query.set('q', filters.q)
        if (filters.status) query.set('status', filters.status)
        if (filters.category) query.set('category', filters.category)
        if (filters.campaign) query.set('campaign', filters.campaign)
        query.set('page', String(page.value))
        query.set('perPage', String(perPage.value))
      } else {
        // Modo 'all': carrega o catalogo INTEIRO de uma vez. Os dropdowns de
        // categoria/campanha e a filtragem sao client-side e precisam ver todos os
        // produtos. Imagens sao locais (cache do sync), entao listar tudo nao
        // dispara requests para a origem.
        query.set('perPage', String(ALL_PER_PAGE))
      }
      const path = `/v1/admin/products?${query.toString()}`
      const resp = await apiRequest(path, { headers: accountHeaders() })
      products.value = (resp.products as Record<string, unknown>[]).map((raw) => {
        const product = normalizeProduct(raw)
        product.image = absolutizeImage(product.image)
        return product
      })
      total.value = Number((resp as Record<string, unknown>).total ?? products.value.length)
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao carregar produtos.')
    } finally {
      loading.value = false
    }
  }

  // Troca de pagina (modo paginado) — refaz o fetch server-side.
  function setPage(next: number) {
    const target = Math.max(1, Math.floor(Number(next) || 1))
    if (target === page.value) return
    page.value = target
    void fetchProducts()
  }

  // Alterna entre 'all' (client-side) e 'paged' (server-side). Reseta para a
  // pagina 1 e recarrega.
  function setMode(next: 'all' | 'paged') {
    if (next === mode.value) return
    mode.value = next
    page.value = 1
    void fetchProducts()
  }

  // Aplica filtros do workspace e, no modo paginado, refaz o fetch server-side
  // (resetando para a pagina 1). No modo 'all' a filtragem e client-side, entao
  // so atualiza o estado (o workspace ja filtra em memoria).
  function applyServerFilters(next: {
    q?: string
    status?: '' | ProductStatus
    category?: string
    campaign?: string
  }) {
    filters.q = next.q ?? ''
    filters.status = next.status ?? ''
    filters.category = next.category ?? ''
    filters.campaign = next.campaign ?? ''
    if (mode.value !== 'paged') return
    page.value = 1
    void fetchProducts()
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

  // Upload de imagem do produto (multipart, campo `file`). NAO setamos
  // Content-Type manual: o browser poe o boundary do multipart. Atualiza a linha
  // com a image retornada (absolutizada para apontar pra API).
  async function uploadProductImage(id: string, file: File): Promise<boolean> {
    const key = `${id}:image`
    setSaving(key, true)
    errorMessage.value = ''
    try {
      const form = new FormData()
      form.append('file', file)
      const resp = await apiRequest(`/v1/admin/products/${encodeURIComponent(id)}/image`, {
        method: 'POST',
        body: form,
        headers: accountHeaders(),
      })
      applyPatch(id, resp as Record<string, unknown>)
      const idx = products.value.findIndex((p) => p.id === id)
      if (idx >= 0) products.value[idx].image = absolutizeImage(products.value[idx].image)
      return true
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao enviar a imagem.')
      return false
    } finally {
      setSaving(key, false)
    }
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

  async function syncProducts(): Promise<ProductSyncResult | null> {
    syncing.value = true
    errorMessage.value = ''
    try {
      const accountId = effectiveAccountId()
      const query = accountId ? `?accountId=${encodeURIComponent(accountId)}` : ''
      const resp = await apiRequest(`/v1/admin/products/sync${query}`, {
        method: 'POST',
        headers: accountHeaders(),
      })
      const raw = (resp ?? {}) as Record<string, unknown>
      const result: ProductSyncResult = {
        inserted: Number(raw.inserted ?? 0) || 0,
        updated: Number(raw.updated ?? 0) || 0,
        skipped: Number(raw.skipped ?? 0) || 0,
      }
      await fetchProducts()
      return result
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao sincronizar produtos.')
      return null
    } finally {
      syncing.value = false
    }
  }

  // Le a fonte de produtos atual da account (GET) e reflete no toggle. Mode
  // 'custom' (base_url fora das 2 conhecidas) e tratado como leitura: o USelect
  // do painel so oferece 'local'/'online', entao mostra a opcao mais proxima sem
  // sobrescrever a fonte.
  async function loadSource(): Promise<ProductSourceMode | null> {
    sourceLoading.value = true
    try {
      const resp = (await apiRequest('/v1/admin/products/source', {
        method: 'GET',
        headers: accountHeaders(),
      })) as { mode?: string } | null
      const mode = resp?.mode === 'local' ? 'local' : resp?.mode === 'custom' ? 'custom' : 'online'
      sourceMode.value = mode
      return mode
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao carregar a fonte de produtos.')
      return null
    } finally {
      sourceLoading.value = false
    }
  }

  // Troca a fonte (PATCH local|online). NAO re-sincroniza aqui: o caller decide
  // (o workspace dispara syncProducts() apos o toast). Devolve o mode aplicado.
  async function setSourceMode(mode: 'local' | 'online'): Promise<ProductSourceMode | null> {
    sourceLoading.value = true
    errorMessage.value = ''
    try {
      const resp = (await apiRequest('/v1/admin/products/source', {
        method: 'PATCH',
        body: { mode },
        headers: accountHeaders(),
      })) as { mode?: string } | null
      const applied = resp?.mode === 'local' ? 'local' : 'online'
      sourceMode.value = applied
      return applied
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao trocar a fonte de produtos.')
      return null
    } finally {
      sourceLoading.value = false
    }
  }

  // Cruzamento produto<->ERP (botao "Cruzar com ERP" + aba de itens do ERP fora
  // do site). Estado e acoes vivem em useProductsErp, compartilhando a mesma
  // lista/account deste manager.
  const erp = useProductsErp({
    apiRequest,
    accountHeaders,
    errorMessage,
    products,
    total,
    normalizeProduct,
    absolutizeImage,
    fetchProducts,
  })

  function resetFilters() {
    filters.q = ''
    filters.status = ''
    filters.category = ''
    filters.campaign = ''
    if (mode.value === 'paged') {
      page.value = 1
      void fetchProducts()
    }
  }

  onBeforeUnmount(() => {
    for (const timer of pendingTimers.values()) clearTimeout(timer)
    pendingTimers.clear()
  })

  return {
    products,
    total,
    filters,
    mode,
    page,
    perPage,
    facets,
    facetsLoading,
    selectedAccountId,
    loading,
    creating,
    syncing,
    sourceMode,
    sourceLoading,
    deletingId,
    errorMessage,
    savingMap,
    canResetFilters,
    rowIsSaving,
    fetchProducts,
    loadFacets,
    setPage,
    setMode,
    applyServerFilters,
    updateField,
    uploadProductImage,
    createProduct,
    deleteProduct,
    syncProducts,
    loadSource,
    setSourceMode,
    resetFilters,
    // ERP (cruzamento + aba de itens do ERP fora do site)
    ...erp,
  }
}
