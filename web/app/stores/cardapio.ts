import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { useAuthStore } from '~/stores/auth'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import type {
  Category,
  Order,
  OrderStatus,
  Product,
  ProductListItem,
  Restaurant,
  RestaurantDomain,
  RestaurantListItem,
  Review,
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

  // Lista de restaurantes (projecao lean).
  const restaurants = ref<RestaurantListItem[]>([])
  const listPending = ref(false)
  const listError = ref('')

  // Restaurante ativo (editor) + catalogo.
  const restaurant = ref<Restaurant | null>(null)
  const categories = ref<Category[]>([])
  const products = ref<ProductListItem[]>([])
  const domains = ref<RestaurantDomain[]>([])
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
    orders.value = emptyOrders()
    detailError.value = ''
    ordersError.value = ''
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
      listError.value = getApiErrorMessage(caught, 'Nao foi possivel carregar os cardapios.')
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
        message: getApiErrorMessage(caught, 'Nao foi possivel criar o cardapio.'),
      }
    }
  }

  // --- Restaurante ativo (editor) ---

  async function loadRestaurant(id: string) {
    detailPending.value = true
    detailError.value = ''
    try {
      const [restaurantResponse, categoriesResponse, productsResponse, domainsResponse] =
        await Promise.all([
          apiRequest(`/v1/cardapio/restaurants/${encodeURIComponent(id)}`),
          apiRequest(`/v1/cardapio/restaurants/${encodeURIComponent(id)}/categories`),
          apiRequest(`/v1/cardapio/restaurants/${encodeURIComponent(id)}/products`),
          apiRequest(`/v1/cardapio/restaurants/${encodeURIComponent(id)}/domains`),
        ])
      restaurant.value = restaurantResponse as Restaurant
      categories.value = asArray<Category>(categoriesResponse, 'categories')
      products.value = asArray<ProductListItem>(productsResponse, 'products')
      domains.value = asArray<RestaurantDomain>(domainsResponse, 'domains')
    } catch (caught) {
      detailError.value = getApiErrorMessage(caught, 'Nao foi possivel carregar o cardapio.')
    } finally {
      detailPending.value = false
    }
  }

  async function patchRestaurant(id: string, body: Record<string, unknown>) {
    const response = (await apiRequest(`/v1/cardapio/restaurants/${encodeURIComponent(id)}`, {
      method: 'PATCH',
      body,
    })) as Restaurant
    restaurant.value = response
    return response
  }

  async function deleteRestaurant(id: string) {
    await apiRequest(`/v1/cardapio/restaurants/${encodeURIComponent(id)}`, { method: 'DELETE' })
    restaurants.value = restaurants.value.filter((item) => item.id !== id)
  }

  // --- Categorias ---

  async function reloadCategories(id: string) {
    const response = await apiRequest(
      `/v1/cardapio/restaurants/${encodeURIComponent(id)}/categories`,
    )
    categories.value = asArray<Category>(response, 'categories')
  }

  async function createCategory(id: string, body: Record<string, unknown>) {
    const response = (await apiRequest(
      `/v1/cardapio/restaurants/${encodeURIComponent(id)}/categories`,
      { method: 'POST', body },
    )) as Category
    await reloadCategories(id)
    return response
  }

  async function patchCategory(categoryId: string, body: Record<string, unknown>) {
    const response = (await apiRequest(
      `/v1/cardapio/categories/${encodeURIComponent(categoryId)}`,
      { method: 'PATCH', body },
    )) as Category
    if (restaurantId.value) {
      await reloadCategories(restaurantId.value)
    }
    return response
  }

  async function deleteCategory(categoryId: string) {
    await apiRequest(`/v1/cardapio/categories/${encodeURIComponent(categoryId)}`, {
      method: 'DELETE',
    })
    if (restaurantId.value) {
      await reloadCategories(restaurantId.value)
    }
  }

  // --- Produtos ---

  async function reloadProducts(id: string) {
    const response = await apiRequest(`/v1/cardapio/restaurants/${encodeURIComponent(id)}/products`)
    products.value = asArray<ProductListItem>(response, 'products')
  }

  async function loadProduct(productId: string): Promise<Product> {
    const response = (await apiRequest(
      `/v1/cardapio/products/${encodeURIComponent(productId)}`,
    )) as Product
    return response
  }

  async function createProduct(id: string, body: Record<string, unknown>) {
    const response = (await apiRequest(
      `/v1/cardapio/restaurants/${encodeURIComponent(id)}/products`,
      { method: 'POST', body },
    )) as Product
    await reloadProducts(id)
    return response
  }

  async function patchProduct(productId: string, body: Record<string, unknown>) {
    const response = (await apiRequest(`/v1/cardapio/products/${encodeURIComponent(productId)}`, {
      method: 'PATCH',
      body,
    })) as Product
    if (restaurantId.value) {
      await reloadProducts(restaurantId.value)
    }
    return response
  }

  async function deleteProduct(productId: string) {
    await apiRequest(`/v1/cardapio/products/${encodeURIComponent(productId)}`, { method: 'DELETE' })
    if (restaurantId.value) {
      await reloadProducts(restaurantId.value)
    }
  }

  // --- Avaliacoes (por produto) ---

  async function loadReviews(productId: string): Promise<Review[]> {
    const response = await apiRequest(
      `/v1/cardapio/products/${encodeURIComponent(productId)}/reviews`,
    )
    return asArray<Review>(response, 'reviews')
  }

  async function createReview(productId: string, body: Record<string, unknown>) {
    return (await apiRequest(`/v1/cardapio/products/${encodeURIComponent(productId)}/reviews`, {
      method: 'POST',
      body,
    })) as Review
  }

  async function patchReview(reviewId: string, body: Record<string, unknown>) {
    return (await apiRequest(`/v1/cardapio/reviews/${encodeURIComponent(reviewId)}`, {
      method: 'PATCH',
      body,
    })) as Review
  }

  async function deleteReview(reviewId: string) {
    await apiRequest(`/v1/cardapio/reviews/${encodeURIComponent(reviewId)}`, { method: 'DELETE' })
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
        `/v1/cardapio/restaurants/${encodeURIComponent(id)}/orders?${search.toString()}`,
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
    const response = (await apiRequest(`/v1/cardapio/orders/${encodeURIComponent(orderId)}`, {
      method: 'PATCH',
      body: { status },
    })) as Order
    orders.value = {
      ...orders.value,
      items: orders.value.items.map((order) => (order.id === orderId ? response : order)),
    }
    return response
  }

  // --- Dominios ---

  async function reloadDomains(id: string) {
    const response = await apiRequest(`/v1/cardapio/restaurants/${encodeURIComponent(id)}/domains`)
    domains.value = asArray<RestaurantDomain>(response, 'domains')
  }

  async function createDomain(id: string, host: string, isPrimary: boolean) {
    const response = (await apiRequest(
      `/v1/cardapio/restaurants/${encodeURIComponent(id)}/domains`,
      { method: 'POST', body: { host, isPrimary } },
    )) as RestaurantDomain
    await reloadDomains(id)
    return response
  }

  async function deleteDomain(host: string) {
    await apiRequest(`/v1/cardapio/domains?host=${encodeURIComponent(host)}`, { method: 'DELETE' })
    if (restaurantId.value) {
      await reloadDomains(restaurantId.value)
    }
  }

  // --- Upload de midia ---

  async function uploadMedia(id: string, file: File): Promise<string> {
    const form = new FormData()
    form.append('file', file)
    const response = (await apiRequest(`/v1/cardapio/restaurants/${encodeURIComponent(id)}/media`, {
      method: 'POST',
      body: form,
    })) as { url?: string }
    return String(response?.url ?? '')
  }

  return {
    restaurants,
    listPending,
    listError,
    restaurant,
    categories,
    products,
    domains,
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
    patchReview,
    deleteReview,
    loadOrders,
    updateOrderStatus,
    reloadDomains,
    createDomain,
    deleteDomain,
    uploadMedia,
  }
})
