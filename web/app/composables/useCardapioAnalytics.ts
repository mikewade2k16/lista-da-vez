import { ref, watch, type Ref } from 'vue'

import { useCardapioStore } from '~/stores/cardapio'
import { getApiErrorMessage } from '~/utils/api-client'
import {
  ANALYTICS_DEFAULT_LIMIT,
  type AnalyticsClicks,
  type AnalyticsDevices,
  type AnalyticsDwell,
  type AnalyticsDwellDimension,
  type AnalyticsFunnel,
  type AnalyticsGranularity,
  type AnalyticsOverview,
  type AnalyticsPages,
  type AnalyticsRange,
  type AnalyticsSourceDimension,
  type AnalyticsSources,
  type AnalyticsTimeseries,
  type AnalyticsTopProducts,
  type AnalyticsTopProductsMetric,
} from '~/domain/cardapio/analytics'

// Composable de fetch da aba Relatorios (F4). UNICA camada de fetch do dashboard:
// page (CardapioSectionRelatorios) -> este composable -> store.analyticsRequest
// (que herda withScope + X-Account-Id). Os componentes de bloco NAO buscam:
// recebem os refs por prop e emitem retry/update do seu seletor.
//
// Cada bloco tem seu proprio par data/pending/error: um erro de um bloco NUNCA
// derruba os demais (cada fetch isola seu catch). O refresh() dispara todos em
// paralelo (Promise.all sobre promessas que nunca rejeitam). watch([id, range])
// re-busca com debounce ~250ms. Seletores locais (granularidade de horas, metrica
// de top-produtos, dimensao de origem/dwell) re-buscam SO o bloco afetado.

const REFRESH_DEBOUNCE_MS = 250

// Estado de um bloco: o dado (null ate chegar), pending e mensagem de erro.
interface BlockState<T> {
  data: Ref<T | null>
  pending: Ref<boolean>
  error: Ref<string>
}

function createBlock<T>(): BlockState<T> {
  return {
    data: ref(null) as Ref<T | null>,
    pending: ref(false),
    error: ref(''),
  }
}

export function useCardapioAnalytics(restaurantId: Ref<string>, range: Ref<AnalyticsRange>) {
  const store = useCardapioStore()

  // Seletores locais dos blocos com mais de uma visao. Mudam -> re-busca so o bloco.
  const hoursGranularity =
    ref<Extract<AnalyticsGranularity, 'hour_of_day' | 'weekday_hour'>>('hour_of_day')
  const topProductsMetric = ref<AnalyticsTopProductsMetric>('orders')
  const sourcesDimension = ref<AnalyticsSourceDimension>('utm_source')
  const dwellDimension = ref<AnalyticsDwellDimension>('page')

  // Um bloco por endpoint do contrato (timeseries aparece 2x: tendencia por dia e
  // horarios — sao buscas independentes com granularidades diferentes).
  const overview = createBlock<AnalyticsOverview>()
  const trend = createBlock<AnalyticsTimeseries>()
  const hours = createBlock<AnalyticsTimeseries>()
  const funnel = createBlock<AnalyticsFunnel>()
  const topProducts = createBlock<AnalyticsTopProducts>()
  const sources = createBlock<AnalyticsSources>()
  const devices = createBlock<AnalyticsDevices>()
  const pages = createBlock<AnalyticsPages>()
  const dwell = createBlock<AnalyticsDwell>()
  const clicks = createBlock<AnalyticsClicks>()

  // Parametros comuns a todo bloco (from/to). tz fica no default do servidor.
  function baseParams(): Record<string, string> {
    return { from: range.value.from, to: range.value.to }
  }

  // Executa um fetch isolado: sem id valido vira no-op; seta pending/erro do bloco
  // e NUNCA rejeita (retorna mesmo em erro) para nao derrubar o Promise.all.
  async function runBlock<T>(
    block: BlockState<T>,
    fetcher: (id: string) => Promise<T>,
  ): Promise<void> {
    const id = String(restaurantId.value || '').trim()
    if (!id) {
      return
    }
    block.pending.value = true
    block.error.value = ''
    try {
      block.data.value = await fetcher(id)
    } catch (caught) {
      block.error.value = getApiErrorMessage(caught, 'Nao foi possivel carregar este bloco.')
    } finally {
      block.pending.value = false
    }
  }

  // --- Refresh por bloco (cada um mapeia 1 endpoint do contrato) ---

  function refreshOverview() {
    return runBlock(overview, (id) =>
      store.analyticsRequest<AnalyticsOverview>(id, 'overview', baseParams()),
    )
  }

  function refreshTrend() {
    return runBlock(trend, (id) =>
      store.analyticsRequest<AnalyticsTimeseries>(id, 'timeseries', {
        ...baseParams(),
        granularity: 'day',
      }),
    )
  }

  function refreshHours() {
    return runBlock(hours, (id) =>
      store.analyticsRequest<AnalyticsTimeseries>(id, 'timeseries', {
        ...baseParams(),
        granularity: hoursGranularity.value,
      }),
    )
  }

  function refreshFunnel() {
    return runBlock(funnel, (id) =>
      store.analyticsRequest<AnalyticsFunnel>(id, 'funnel', baseParams()),
    )
  }

  function refreshTopProducts() {
    return runBlock(topProducts, (id) =>
      store.analyticsRequest<AnalyticsTopProducts>(id, 'top-products', {
        ...baseParams(),
        metric: topProductsMetric.value,
        limit: ANALYTICS_DEFAULT_LIMIT,
      }),
    )
  }

  function refreshSources() {
    return runBlock(sources, (id) =>
      store.analyticsRequest<AnalyticsSources>(id, 'sources', {
        ...baseParams(),
        dimension: sourcesDimension.value,
        limit: ANALYTICS_DEFAULT_LIMIT,
      }),
    )
  }

  function refreshDevices() {
    return runBlock(devices, (id) =>
      store.analyticsRequest<AnalyticsDevices>(id, 'devices', baseParams()),
    )
  }

  function refreshPages() {
    return runBlock(pages, (id) =>
      store.analyticsRequest<AnalyticsPages>(id, 'pages', {
        ...baseParams(),
        limit: ANALYTICS_DEFAULT_LIMIT,
      }),
    )
  }

  function refreshDwell() {
    return runBlock(dwell, (id) =>
      store.analyticsRequest<AnalyticsDwell>(id, 'dwell', {
        ...baseParams(),
        dimension: dwellDimension.value,
        limit: ANALYTICS_DEFAULT_LIMIT,
      }),
    )
  }

  function refreshClicks() {
    return runBlock(clicks, (id) =>
      store.analyticsRequest<AnalyticsClicks>(id, 'clicks', {
        ...baseParams(),
        limit: ANALYTICS_DEFAULT_LIMIT,
      }),
    )
  }

  // Dispara TODOS os blocos em paralelo. Promise.all nao derruba porque runBlock
  // engole o erro em cada bloco (so seta block.error). Retorna quando todos terminam.
  function refresh(): Promise<void> {
    return Promise.all([
      refreshOverview(),
      refreshTrend(),
      refreshHours(),
      refreshFunnel(),
      refreshTopProducts(),
      refreshSources(),
      refreshDevices(),
      refreshPages(),
      refreshDwell(),
      refreshClicks(),
    ]).then(() => undefined)
  }

  // watch com debounce: trocar restaurante ou periodo re-busca tudo, mas espera
  // ~250ms para nao disparar 10 requests por tecla/arraste do date picker.
  let debounceTimer: ReturnType<typeof setTimeout> | null = null
  watch(
    [restaurantId, () => range.value.from, () => range.value.to],
    () => {
      if (debounceTimer) {
        clearTimeout(debounceTimer)
      }
      debounceTimer = setTimeout(() => {
        void refresh()
      }, REFRESH_DEBOUNCE_MS)
    },
    { immediate: true },
  )

  // Seletores locais re-buscam SO o bloco afetado (sem debounce: clique unico).
  watch(hoursGranularity, () => void refreshHours())
  watch(topProductsMetric, () => void refreshTopProducts())
  watch(sourcesDimension, () => void refreshSources())
  watch(dwellDimension, () => void refreshDwell())

  return {
    // Seletores locais (v-model dos blocos com toggle/select).
    hoursGranularity,
    topProductsMetric,
    sourcesDimension,
    dwellDimension,
    // Blocos (data/pending/error).
    overview,
    trend,
    hours,
    funnel,
    topProducts,
    sources,
    devices,
    pages,
    dwell,
    clicks,
    // Refresh global + por bloco (o @retry de cada bloco chama o seu).
    refresh,
    refreshOverview,
    refreshTrend,
    refreshHours,
    refreshFunnel,
    refreshTopProducts,
    refreshSources,
    refreshDevices,
    refreshPages,
    refreshDwell,
    refreshClicks,
  }
}

export type UseCardapioAnalytics = ReturnType<typeof useCardapioAnalytics>
