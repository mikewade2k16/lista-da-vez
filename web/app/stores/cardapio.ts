import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { useAuthStore } from '~/stores/auth'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import type {
  Category,
  DeliveryZone,
  Order,
  OrderStatus,
  Product,
  ProductListItem,
  Restaurant,
  RestaurantDomain,
  RestaurantListItem,
  Review,
  ReviewInput,
  SiteLayout,
} from '~/domain/cardapio/types'

// Store do modulo Cardapio Online (painel). Contrato congelado em
// docs/cardapio/PLANO_MODULO_CARDAPIO.md (§4). Os componentes em
// web/app/components/cardapio/* dependem destes nomes (estados/actions).
// X-Account-Id e injetado automaticamente pelo createApiRequest (account ativa).

const ORDERS_PER_PAGE = 20

interface OrdersState {
  items: Order[]
  status: '' | OrderStatus
  page: number
  perPage: number
  total: number
}

function emptyOrders(): OrdersState {
  return { items: [], status: '', page: 1, perPage: ORDERS_PER_PAGE, total: 0 }
}

function asArray<T>(value: unknown, key: string): T[] {
  if (Array.isArray(value)) {
    return value as T[]
  }
  const nested = (value as Record<string, unknown> | null)?.[key]
  return Array.isArray(nested) ? (nested as T[]) : []
}

export const useCardapioStore = defineStore('cardapio', () => {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  // Escopo de account do EDITOR. Quando platform_admin abre um restaurante de
  // OUTRO cliente, o accountId precisa ir na query — o backend prioriza o query
  // `accountId` sobre o header X-Account-Id (account ativa). Vazio = usa o
  // contexto (nao-admin, ou admin operando na propria agencia/account ativa).
  const scopeAccountId = ref('')

  function withScope(path: string): string {
    return withScopeFor(path, scopeAccountId.value)
  }

  // Igual ao withScope, mas com o accountId explicito da chamada. Usado pela
  // LISTA: ali o scopeAccountId esta vazio (e do editor), entao a edicao/exclusao
  // inline de um restaurante de OUTRA account (platform_admin) precisa passar o
  // accountId da propria linha — senao cai na account ativa (X-Account-Id) e da
  // 404. Vazio = sem query, usa o contexto (account ativa).
  function withScopeFor(path: string, accountId: string): string {
    const scoped = String(accountId || '').trim()
    if (!scoped) {
      return path
    }
    const sep = path.includes('?') ? '&' : '?'
    return `${path}${sep}accountId=${encodeURIComponent(scoped)}`
  }

  // Lista de restaurantes (projecao lean).
  const restaurants = ref<RestaurantListItem[]>([])
  const listPending = ref(false)
  const listError = ref('')

  // Restaurante ativo (editor) + catalogo.
  const restaurant = ref<Restaurant | null>(null)
  const categories = ref<Category[]>([])
  const products = ref<ProductListItem[]>([])
  const domains = ref<RestaurantDomain[]>([])
  const zones = ref<DeliveryZone[]>([])
  const detailPending = ref(false)
  const detailError = ref('')

  // Pedidos do restaurante ativo.
  const orders = ref<OrdersState>(emptyOrders())
  const ordersPending = ref(false)
  const ordersError = ref('')

  const restaurantId = computed(() => restaurant.value?.id ?? '')
  const primaryDomain = computed(
    () => domains.value.find((domain) => domain.isPrimary)?.host ?? domains.value[0]?.host ?? '',
  )

  function resetActive() {
    restaurant.value = null
    categories.value = []
    products.value = []
    domains.value = []
    zones.value = []
    orders.value = emptyOrders()
    detailError.value = ''
    ordersError.value = ''
    scopeAccountId.value = ''
  }

  // --- Lista de restaurantes ---

  async function loadRestaurants(params: { accountId?: string; q?: string } = {}) {
    listPending.value = true
    listError.value = ''
    try {
      const search = new URLSearchParams()
      if (params.accountId) {
        search.set('accountId', params.accountId)
      }
      if (params.q) {
        search.set('q', params.q)
      }
      const query = search.toString()
      const response = await apiRequest(`/v1/cardapio/restaurants${query ? `?${query}` : ''}`)
      restaurants.value = asArray<RestaurantListItem>(response, 'restaurants')
    } catch (caught) {
      listError.value = getApiErrorMessage(caught, 'Nao foi possivel carregar os estabelecimentos.')
    } finally {
      listPending.value = false
    }
  }

  async function createRestaurant(payload: { accountId?: string; slug: string; name: string }) {
    try {
      const body: Record<string, string> = {
        slug: String(payload.slug || '').trim(),
        name: String(payload.name || '').trim(),
      }
      if (payload.accountId) {
        body.accountId = payload.accountId
      }
      const response = (await apiRequest('/v1/cardapio/restaurants', {
        method: 'POST',
        body,
      })) as Restaurant
      return { ok: true as const, restaurant: response }
    } catch (caught) {
      return {
        ok: false as const,
        message: getApiErrorMessage(caught, 'Nao foi possivel criar o estabelecimento.'),
      }
    }
  }

  // --- Restaurante ativo (editor) ---

  // accountId: account do restaurante (vem da listagem ou da query da rota). So
  // platform_admin abrindo restaurante de outro cliente precisa informar; demais
  // casos passam vazio e usam o X-Account-Id do contexto.
  async function loadRestaurant(id: string, accountId = '') {
    scopeAccountId.value = String(accountId || '').trim()
    detailPending.value = true
    detailError.value = ''
    try {
      const base = `/v1/cardapio/restaurants/${encodeURIComponent(id)}`
      const [
        restaurantResponse,
        categoriesResponse,
        productsResponse,
        domainsResponse,
        zonesResponse,
      ] = await Promise.all([
        apiRequest(withScope(base)),
        apiRequest(withScope(`${base}/categories`)),
        apiRequest(withScope(`${base}/products`)),
        apiRequest(withScope(`${base}/domains`)),
        apiRequest(withScope(`${base}/delivery-zones`)),
      ])
      restaurant.value = restaurantResponse as Restaurant
      categories.value = asArray<Category>(categoriesResponse, 'categories')
      products.value = asArray<ProductListItem>(productsResponse, 'products')
      domains.value = asArray<RestaurantDomain>(domainsResponse, 'domains')
      zones.value = asArray<DeliveryZone>(zonesResponse, 'deliveryZones')
    } catch (caught) {
      detailError.value = getApiErrorMessage(caught, 'Nao foi possivel carregar o estabelecimento.')
    } finally {
      detailPending.value = false
    }
  }

  async function patchRestaurant(id: string, body: Record<string, unknown>) {
    const response = (await apiRequest(
      withScope(`/v1/cardapio/restaurants/${encodeURIComponent(id)}`),
      {
        method: 'PATCH',
        body,
      },
    )) as Restaurant
    restaurant.value = response
    return response
  }

  async function deleteRestaurant(id: string) {
    await apiRequest(withScope(`/v1/cardapio/restaurants/${encodeURIComponent(id)}`), {
      method: 'DELETE',
    })
    restaurants.value = restaurants.value.filter((item) => item.id !== id)
  }

  // Versoes "scoped" para a LISTA: recebem o accountId da propria linha e o
  // anexam na query daquela chamada (sem tocar no scopeAccountId do editor).
  // Resolvem o isolamento multi-tenant da edicao/exclusao inline quando o admin
  // mexe num restaurante de outra account. Atualizam a projecao lean da lista.
  async function patchRestaurantScoped(
    id: string,
    accountId: string,
    body: Record<string, unknown>,
  ) {
    const response = (await apiRequest(
      withScopeFor(`/v1/cardapio/restaurants/${encodeURIComponent(id)}`, accountId),
      { method: 'PATCH', body },
    )) as Restaurant
    restaurants.value = restaurants.value.map((item) =>
      item.id === id
        ? { ...item, name: response.name, slug: response.slug, isActive: response.isActive }
        : item,
    )
    return response
  }

  async function deleteRestaurantScoped(id: string, accountId: string) {
    await apiRequest(
      withScopeFor(`/v1/cardapio/restaurants/${encodeURIComponent(id)}`, accountId),
      { method: 'DELETE' },
    )
    restaurants.value = restaurants.value.filter((item) => item.id !== id)
  }

  // Duplica um restaurante a partir da LISTA (so platform_admin; o back nega
  // demais papeis). O accountId e o da PROPRIA linha (vazio = account ativa) e vai
  // na query igual as demais acoes scoped da lista — o admin pode duplicar um
  // restaurante de OUTRA account, e o scopeAccountId (do editor) esta vazio aqui.
  // O novo restaurante nasce inativo e na MESMA account do source. Re-le a lista
  // lean do back (a resposta e um Restaurant full, sem accountName/primaryDomain
  // da projecao lean) para refletir o item novo. Retorna o restaurante criado.
  async function duplicateRestaurant(
    id: string,
    payload: { name: string; slug: string },
    accountId = '',
  ): Promise<Restaurant> {
    const response = (await apiRequest(
      withScopeFor(`/v1/cardapio/restaurants/${encodeURIComponent(id)}/duplicate`, accountId),
      {
        method: 'POST',
        body: {
          name: String(payload.name || '').trim(),
          slug: String(payload.slug || '').trim(),
        },
      },
    )) as Restaurant
    await loadRestaurants(accountId ? { accountId } : {})
    return response
  }

  // Define o dominio primario de um restaurante a partir da LISTA (edicao inline).
  // host vazio => NO-OP (nao apaga dominio inline; remover e so na aba Dominios).
  // host preenchido => se ja existe um primario na linha, DELETE o antigo e POST
  // o novo como primario; senao so POST. Usa o accountId da linha (multi-tenant).
  // Atualiza o primaryDomain da projecao lean. Retorna o host normalizado salvo.
  async function setPrimaryDomain(restaurantId: string, accountId: string, host: string) {
    const normalized = String(host || '')
      .trim()
      .toLowerCase()
    if (!normalized) {
      return ''
    }
    const current = restaurants.value.find((item) => item.id === restaurantId)
    const previous = String(current?.primaryDomain || '').trim()
    if (previous && previous !== normalized) {
      await apiRequest(
        withScopeFor(`/v1/cardapio/domains?host=${encodeURIComponent(previous)}`, accountId),
        { method: 'DELETE' },
      )
    }
    await apiRequest(
      withScopeFor(
        `/v1/cardapio/restaurants/${encodeURIComponent(restaurantId)}/domains`,
        accountId,
      ),
      { method: 'POST', body: { host: normalized, isPrimary: true } },
    )
    restaurants.value = restaurants.value.map((item) =>
      item.id === restaurantId ? { ...item, primaryDomain: normalized } : item,
    )
    return normalized
  }

  // --- Categorias ---

  async function reloadCategories(id: string) {
    const response = await apiRequest(
      withScope(`/v1/cardapio/restaurants/${encodeURIComponent(id)}/categories`),
    )
    categories.value = asArray<Category>(response, 'categories')
  }

  async function createCategory(id: string, body: Record<string, unknown>) {
    const response = (await apiRequest(
      withScope(`/v1/cardapio/restaurants/${encodeURIComponent(id)}/categories`),
      { method: 'POST', body },
    )) as Category
    await reloadCategories(id)
    return response
  }

  async function patchCategory(categoryId: string, body: Record<string, unknown>) {
    const response = (await apiRequest(
      withScope(`/v1/cardapio/categories/${encodeURIComponent(categoryId)}`),
      { method: 'PATCH', body },
    )) as Category
    if (restaurantId.value) {
      await reloadCategories(restaurantId.value)
    }
    return response
  }

  async function deleteCategory(categoryId: string) {
    await apiRequest(withScope(`/v1/cardapio/categories/${encodeURIComponent(categoryId)}`), {
      method: 'DELETE',
    })
    if (restaurantId.value) {
      await reloadCategories(restaurantId.value)
    }
  }

  // --- Produtos ---

  async function reloadProducts(id: string) {
    const response = await apiRequest(
      withScope(`/v1/cardapio/restaurants/${encodeURIComponent(id)}/products`),
    )
    products.value = asArray<ProductListItem>(response, 'products')
  }

  async function loadProduct(productId: string): Promise<Product> {
    const response = (await apiRequest(
      withScope(`/v1/cardapio/products/${encodeURIComponent(productId)}`),
    )) as Product
    return response
  }

  async function createProduct(id: string, body: Record<string, unknown>) {
    const response = (await apiRequest(
      withScope(`/v1/cardapio/restaurants/${encodeURIComponent(id)}/products`),
      { method: 'POST', body },
    )) as Product
    await reloadProducts(id)
    return response
  }

  async function patchProduct(productId: string, body: Record<string, unknown>) {
    const response = (await apiRequest(
      withScope(`/v1/cardapio/products/${encodeURIComponent(productId)}`),
      { method: 'PATCH', body },
    )) as Product
    if (restaurantId.value) {
      await reloadProducts(restaurantId.value)
    }
    return response
  }

  async function deleteProduct(productId: string) {
    await apiRequest(withScope(`/v1/cardapio/products/${encodeURIComponent(productId)}`), {
      method: 'DELETE',
    })
    if (restaurantId.value) {
      await reloadProducts(restaurantId.value)
    }
  }

  // --- Avaliacoes ---
  // As reviews NAO ficam no state da store: cada secao busca a sua lista (por
  // produto OU do estabelecimento) e a guarda localmente. As actions abaixo so
  // fazem a chamada e devolvem o resultado (mesmo padrao de loadProduct).
  // ReviewInput e full-replace: PATCH manda o objeto COMPLETO + o campo alterado.

  async function loadReviews(productId: string): Promise<Review[]> {
    const response = await apiRequest(
      withScope(`/v1/cardapio/products/${encodeURIComponent(productId)}/reviews`),
    )
    return asArray<Review>(response, 'reviews')
  }

  async function createReview(productId: string, input: ReviewInput) {
    return (await apiRequest(
      withScope(`/v1/cardapio/products/${encodeURIComponent(productId)}/reviews`),
      { method: 'POST', body: { ...input } },
    )) as Review
  }

  // Reviews do ESTABELECIMENTO: productId null OU showOnEstablishment=true. O back
  // forca product_id NULL no POST (review propria do estabelecimento), entao o
  // input nao carrega productId. Mesma forma de guardar das reviews de produto:
  // a action so devolve a lista; quem chama a guarda.
  async function loadEstablishmentReviews(restaurantId: string): Promise<Review[]> {
    const response = await apiRequest(
      withScope(`/v1/cardapio/restaurants/${encodeURIComponent(restaurantId)}/reviews`),
    )
    return asArray<Review>(response, 'reviews')
  }

  async function createEstablishmentReview(restaurantId: string, input: ReviewInput) {
    return (await apiRequest(
      withScope(`/v1/cardapio/restaurants/${encodeURIComponent(restaurantId)}/reviews`),
      { method: 'POST', body: { ...input } },
    )) as Review
  }

  async function patchReview(reviewId: string, input: ReviewInput) {
    return (await apiRequest(withScope(`/v1/cardapio/reviews/${encodeURIComponent(reviewId)}`), {
      method: 'PATCH',
      body: { ...input },
    })) as Review
  }

  // Conveniencia: vira a flag showOnEstablishment de uma review SEM zerar o resto.
  // ReviewInput e full-replace, e nao ha GET de review individual; por isso recebe
  // a review COMPLETA (a secao ja tem o objeto em maos) e reenvia todos os campos
  // com a flag trocada. Espelha o patchReview cheio do toggle de destaque.
  async function setReviewOnEstablishment(review: Review, value: boolean) {
    return patchReview(review.id, {
      authorName: review.authorName,
      authorLevel: review.authorLevel,
      rating: review.rating,
      body: review.body,
      isHighlight: review.isHighlight,
      showOnEstablishment: value,
      dateLabel: review.dateLabel,
      sortOrder: review.sortOrder,
    })
  }

  async function deleteReview(reviewId: string) {
    await apiRequest(withScope(`/v1/cardapio/reviews/${encodeURIComponent(reviewId)}`), {
      method: 'DELETE',
    })
  }

  // --- Pedidos ---

  async function loadOrders(
    id: string,
    options: { status?: '' | OrderStatus; page?: number } = {},
  ) {
    const nextStatus = options.status ?? orders.value.status
    const nextPage = options.page ?? orders.value.page
    ordersPending.value = true
    ordersError.value = ''
    try {
      const search = new URLSearchParams()
      if (nextStatus) {
        search.set('status', nextStatus)
      }
      search.set('page', String(nextPage))
      search.set('perPage', String(ORDERS_PER_PAGE))
      const response = (await apiRequest(
        withScope(`/v1/cardapio/restaurants/${encodeURIComponent(id)}/orders?${search.toString()}`),
      )) as { orders?: Order[]; total?: number; page?: number } | Order[]

      const items = asArray<Order>(response, 'orders')
      const total = Array.isArray(response) ? items.length : Number(response?.total ?? items.length)
      orders.value = {
        items,
        status: nextStatus,
        page: nextPage,
        perPage: ORDERS_PER_PAGE,
        total,
      }
    } catch (caught) {
      ordersError.value = getApiErrorMessage(caught, 'Nao foi possivel carregar os pedidos.')
    } finally {
      ordersPending.value = false
    }
  }

  async function updateOrderStatus(orderId: string, status: OrderStatus) {
    const response = (await apiRequest(
      withScope(`/v1/cardapio/orders/${encodeURIComponent(orderId)}`),
      {
        method: 'PATCH',
        body: { status },
      },
    )) as Order
    orders.value = {
      ...orders.value,
      items: orders.value.items.map((order) => (order.id === orderId ? response : order)),
    }
    return response
  }

  // --- Dominios ---

  async function reloadDomains(id: string) {
    const response = await apiRequest(
      withScope(`/v1/cardapio/restaurants/${encodeURIComponent(id)}/domains`),
    )
    domains.value = asArray<RestaurantDomain>(response, 'domains')
  }

  async function createDomain(id: string, host: string, isPrimary: boolean) {
    const response = (await apiRequest(
      withScope(`/v1/cardapio/restaurants/${encodeURIComponent(id)}/domains`),
      { method: 'POST', body: { host, isPrimary } },
    )) as RestaurantDomain
    await reloadDomains(id)
    return response
  }

  async function deleteDomain(host: string) {
    await apiRequest(withScope(`/v1/cardapio/domains?host=${encodeURIComponent(host)}`), {
      method: 'DELETE',
    })
    if (restaurantId.value) {
      await reloadDomains(restaurantId.value)
    }
  }

  // --- Zonas de entrega (bairro + frete) ---

  async function reloadZones(id: string) {
    const response = await apiRequest(
      withScope(`/v1/cardapio/restaurants/${encodeURIComponent(id)}/delivery-zones`),
    )
    zones.value = asArray<DeliveryZone>(response, 'deliveryZones')
  }

  async function createZone(id: string, body: Record<string, unknown>) {
    const response = (await apiRequest(
      withScope(`/v1/cardapio/restaurants/${encodeURIComponent(id)}/delivery-zones`),
      { method: 'POST', body },
    )) as DeliveryZone
    await reloadZones(id)
    return response
  }

  // PATCH de zona e PARCIAL (pointer-based no back): pode mandar so {isActive}.
  async function patchZone(zoneId: string, body: Record<string, unknown>) {
    const response = (await apiRequest(
      withScope(`/v1/cardapio/delivery-zones/${encodeURIComponent(zoneId)}`),
      { method: 'PATCH', body },
    )) as DeliveryZone
    if (restaurantId.value) {
      await reloadZones(restaurantId.value)
    }
    return response
  }

  async function deleteZone(zoneId: string) {
    await apiRequest(withScope(`/v1/cardapio/delivery-zones/${encodeURIComponent(zoneId)}`), {
      method: 'DELETE',
    })
    if (restaurantId.value) {
      await reloadZones(restaurantId.value)
    }
  }

  // --- Upload de midia ---

  async function uploadMedia(id: string, file: File): Promise<string> {
    const form = new FormData()
    form.append('file', file)
    const response = (await apiRequest(
      withScope(`/v1/cardapio/restaurants/${encodeURIComponent(id)}/media`),
      {
        method: 'POST',
        body: form,
      },
    )) as { url?: string }
    return String(response?.url ?? '')
  }

  // --- Site Builder (layout do Studio do TAVOLA) ---

  // Normaliza a resposta {layout, version} do back. Layout ausente => documento
  // vazio canonico; version ausente => 0 (rascunho novo).
  function asLayoutResult(value: unknown): { layout: SiteLayout; version: number } {
    const source = (value ?? {}) as { layout?: SiteLayout | null; version?: number }
    const layout: SiteLayout =
      source.layout && typeof source.layout === 'object' ? source.layout : { pages: {} }
    const version = Number.isFinite(source.version) ? Number(source.version) : 0
    return { layout, version }
  }

  // GET do rascunho do layout. Vazio = { pages: {} } e version 0. Usa o escopo do
  // editor (withScope anexa ?accountId= para platform_admin de outra account).
  async function loadLayout(id: string): Promise<{ layout: SiteLayout; version: number }> {
    const response = await apiRequest(
      withScope(`/v1/cardapio/restaurants/${encodeURIComponent(id)}/layout`),
    )
    return asLayoutResult(response)
  }

  // PUT do rascunho. Manda If-Match com a versao conhecida quando > 0 (controle de
  // concorrencia otimista; 412 se outro editor salvou no meio). O back tolera
  // ausencia do header. Retorna a nova {layout, version}.
  async function putDraftLayout(
    id: string,
    layout: SiteLayout,
    version = 0,
  ): Promise<{ layout: SiteLayout; version: number }> {
    const headers: Record<string, string> = {}
    if (version > 0) {
      headers['If-Match'] = String(version)
    }
    const response = await apiRequest(
      withScope(`/v1/cardapio/restaurants/${encodeURIComponent(id)}/layout`),
      { method: 'PUT', body: layout, headers },
    )
    return asLayoutResult(response)
  }

  // POST publica o rascunho atual (vira a versao publica do site). Retorna a
  // {layout, version} publicada.
  async function publishLayout(id: string): Promise<{ layout: SiteLayout; version: number }> {
    const response = await apiRequest(
      withScope(`/v1/cardapio/restaurants/${encodeURIComponent(id)}/layout/publish`),
      { method: 'POST' },
    )
    return asLayoutResult(response)
  }

  return {
    restaurants,
    listPending,
    listError,
    restaurant,
    categories,
    products,
    domains,
    zones,
    detailPending,
    detailError,
    orders,
    ordersPending,
    ordersError,
    restaurantId,
    primaryDomain,
    resetActive,
    loadRestaurants,
    createRestaurant,
    loadRestaurant,
    patchRestaurant,
    deleteRestaurant,
    patchRestaurantScoped,
    deleteRestaurantScoped,
    duplicateRestaurant,
    setPrimaryDomain,
    reloadCategories,
    createCategory,
    patchCategory,
    deleteCategory,
    reloadProducts,
    loadProduct,
    createProduct,
    patchProduct,
    deleteProduct,
    loadReviews,
    createReview,
    loadEstablishmentReviews,
    createEstablishmentReview,
    patchReview,
    setReviewOnEstablishment,
    deleteReview,
    loadOrders,
    updateOrderStatus,
    reloadDomains,
    createDomain,
    deleteDomain,
    reloadZones,
    createZone,
    patchZone,
    deleteZone,
    uploadMedia,
    loadLayout,
    putDraftLayout,
    publishLayout,
  }
})
