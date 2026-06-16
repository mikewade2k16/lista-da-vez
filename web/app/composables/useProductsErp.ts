import type { Ref } from 'vue'
import type {
  ErpUnmatchedItem,
  ErpUnmatchedResponse,
  ProductErpMatchResult,
  ProductItem,
} from '~/types/products'
import { getApiErrorMessage } from '~/utils/api-client'

// Dependencias compartilhadas com useProductsManager (mesma instancia de
// apiRequest/account/lista de produtos). Extraido para manter o manager enxuto.
interface ProductsErpDeps {
  apiRequest: (path: string, options?: Record<string, unknown>) => Promise<unknown>
  accountHeaders: () => Record<string, string>
  errorMessage: Ref<string>
  products: Ref<ProductItem[]>
  total: Ref<number>
  normalizeProduct: (raw: Record<string, unknown>) => ProductItem
  absolutizeImage: (url: string) => string
  fetchProducts: () => Promise<void>
}

// Cruzamento produto<->ERP: botao "Cruzar com ERP" (recalcula links) + aba
// "Produtos do ERP (fora do site)" (paginada server-side) + "Puxar pro site".
export function useProductsErp(deps: ProductsErpDeps) {
  const erpMatching = ref(false)
  const erpUnmatched = ref<ErpUnmatchedItem[]>([])
  const erpUnmatchedTotal = ref(0)
  const erpUnmatchedPage = ref(1)
  const erpUnmatchedPerPage = ref(50)
  const erpUnmatchedQuery = ref('')
  const erpUnmatchedLoading = ref(false)
  const erpCreatingSku = ref<string | null>(null)

  // Recalcula os vinculos produto<->ERP (POST /v1/admin/products/erp-match) e
  // refaz a lista para refletir os novos erpSynced. Retorna { matched, products }.
  async function erpMatch(): Promise<ProductErpMatchResult | null> {
    erpMatching.value = true
    deps.errorMessage.value = ''
    try {
      const resp = await deps.apiRequest('/v1/admin/products/erp-match', {
        method: 'POST',
        headers: deps.accountHeaders(),
      })
      const raw = (resp ?? {}) as Record<string, unknown>
      const result: ProductErpMatchResult = {
        matched: Number(raw.matched ?? 0) || 0,
        products: Number(raw.products ?? 0) || 0,
      }
      await deps.fetchProducts()
      return result
    } catch (e) {
      deps.errorMessage.value = getApiErrorMessage(e, 'Falha ao cruzar produtos com o ERP.')
      return null
    } finally {
      erpMatching.value = false
    }
  }

  // Lista paginada (server-side) dos itens do ERP que NAO existem no site.
  async function loadErpUnmatched(opts?: { page?: number; perPage?: number; q?: string }) {
    if (typeof opts?.q === 'string') erpUnmatchedQuery.value = opts.q
    if (typeof opts?.perPage === 'number') erpUnmatchedPerPage.value = opts.perPage
    erpUnmatchedPage.value = Math.max(
      1,
      Math.floor(Number(opts?.page ?? erpUnmatchedPage.value) || 1),
    )

    erpUnmatchedLoading.value = true
    deps.errorMessage.value = ''
    try {
      const query = new URLSearchParams()
      query.set('page', String(erpUnmatchedPage.value))
      query.set('perPage', String(erpUnmatchedPerPage.value))
      const q = erpUnmatchedQuery.value.trim()
      if (q) query.set('q', q)
      const resp = (await deps.apiRequest(`/v1/admin/products/erp-unmatched?${query.toString()}`, {
        method: 'GET',
        headers: deps.accountHeaders(),
      })) as Partial<ErpUnmatchedResponse> | null
      const items = Array.isArray(resp?.items) ? resp.items : []
      erpUnmatched.value = items.map((item) => ({
        sku: String((item as Record<string, unknown>).sku ?? ''),
        name: String((item as Record<string, unknown>).name ?? ''),
        description: String((item as Record<string, unknown>).description ?? ''),
      }))
      erpUnmatchedTotal.value = Number(resp?.total ?? erpUnmatched.value.length) || 0
    } catch (e) {
      deps.errorMessage.value = getApiErrorMessage(e, 'Falha ao carregar itens do ERP.')
    } finally {
      erpUnmatchedLoading.value = false
    }
  }

  // Cria um produto no site a partir de um SKU do ERP
  // (POST /v1/admin/products/from-erp). Some da lista do ERP e passa a aparecer
  // na aba principal de produtos.
  async function createFromErp(sku: string): Promise<boolean> {
    const target = String(sku ?? '').trim()
    if (!target) return false
    erpCreatingSku.value = target
    deps.errorMessage.value = ''
    try {
      const resp = await deps.apiRequest('/v1/admin/products/from-erp', {
        method: 'POST',
        body: { sku: target },
        headers: deps.accountHeaders(),
      })
      const created = deps.normalizeProduct(resp as Record<string, unknown>)
      created.image = deps.absolutizeImage(created.image)
      deps.products.value.unshift(created)
      // O sku puxado some do erp-unmatched.
      erpUnmatched.value = erpUnmatched.value.filter((item) => item.sku !== target)
      erpUnmatchedTotal.value = Math.max(0, erpUnmatchedTotal.value - 1)
      deps.total.value += 1
      return true
    } catch (e) {
      deps.errorMessage.value = getApiErrorMessage(e, 'Falha ao puxar o item do ERP.')
      return false
    } finally {
      erpCreatingSku.value = null
    }
  }

  return {
    erpMatching,
    erpUnmatched,
    erpUnmatchedTotal,
    erpUnmatchedPage,
    erpUnmatchedPerPage,
    erpUnmatchedQuery,
    erpUnmatchedLoading,
    erpCreatingSku,
    erpMatch,
    loadErpUnmatched,
    createFromErp,
  }
}
