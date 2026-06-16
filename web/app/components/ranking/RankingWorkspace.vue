<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRankingDetailsDrawer } from '~/composables/useRankingDetailsDrawer'
import { getMetricValue, normalizeSearch, useRankingData } from '~/composables/useRankingData'
import RankingDetailsDrawer from '~/components/ranking/RankingDetailsDrawer.vue'
import RankingFilters from '~/components/ranking/RankingFilters.vue'
import RankingLeaderboardCard from '~/components/ranking/RankingLeaderboardCard.vue'
import RankingPodium from '~/components/ranking/RankingPodium.vue'

const FILTER_ALL = 'all'
const METRIC_OPTIONS = [
  { value: 'score360', label: 'Score 360' },
  { value: 'soldValue', label: 'Valor vendido' },
  { value: 'conversionRate', label: 'Conversao' },
  { value: 'ticketAverage', label: 'Ticket medio' },
  { value: 'paScore', label: 'P.A.' },
  { value: 'qualityScore', label: 'Qualidade' },
]

const props = withDefaults(
  defineProps<{
    report?: object | null
    pending?: boolean
    errorMessage?: string
    integratedScope?: boolean
    dateFrom?: string
    dateTo?: string
  }>(),
  {
    report: null,
    pending: false,
    errorMessage: '',
    integratedScope: false,
    dateFrom: '',
    dateTo: '',
  },
)

const emit = defineEmits<{
  'update:dateFrom': [value: string]
  'update:dateTo': [value: string]
  applyPeriod: []
  setCurrentMonth: []
  setPreviousMonth: []
}>()

const drawer = useRankingDetailsDrawer()
const metric = ref('score360')
const searchTerm = ref('')
const storeFilter = ref(FILTER_ALL)

const reportData = computed(() => props.report as Record<string, unknown> | null | undefined)
const monthlyRows = computed(() => (reportData.value?.monthlyRows as unknown[]) || [])
const dailyRows = computed(() => (reportData.value?.dailyRows as unknown[]) || [])
const alerts = computed(() => (reportData.value?.alerts as unknown[]) || [])

const { enrichedMonthly, monthlyStoreRows, variationFor, storeVariationFor } = useRankingData(
  monthlyRows,
  dailyRows,
)

function normalizeText(v: unknown) {
  return String(v || '').trim()
}

function matchesSearch(term: string, candidates: unknown[]) {
  if (!term) return true
  return candidates.some((c) => normalizeSearch(c).includes(term))
}

const storeOptions = computed(() => {
  const stores = new Map<string, { value: string; label: string }>()
  enrichedMonthly.value.forEach((row) => {
    const id = normalizeText(row.storeId)
    const name = normalizeText(row.storeName)
    if (id && name && !stores.has(id)) stores.set(id, { value: id, label: name })
  })
  return [
    { value: FILTER_ALL, label: 'Todas as lojas' },
    ...[...stores.values()].sort((a, b) => a.label.localeCompare(b.label)),
  ]
})

const singleStoreMode = computed(() => props.integratedScope && storeFilter.value !== FILTER_ALL)
const selectedStoreLabel = computed(() =>
  storeFilter.value === FILTER_ALL
    ? 'Todas as lojas'
    : storeOptions.value.find((o) => o.value === storeFilter.value)?.label || 'Loja selecionada',
)
const normalizedSearchTerm = computed(() => normalizeSearch(searchTerm.value))

const filteredConsultantRows = computed(() =>
  enrichedMonthly.value.filter((row) => {
    if (
      props.integratedScope &&
      storeFilter.value !== FILTER_ALL &&
      row.storeId !== storeFilter.value
    )
      return false
    return matchesSearch(normalizedSearchTerm.value, [row.consultantName, row.storeName])
  }),
)

function sortRows<T extends Record<string, unknown>>(rows: T[], key: string) {
  return [...rows].sort((l, r) => {
    const diff = getMetricValue(r, key) - getMetricValue(l, key)
    return diff !== 0
      ? diff
      : String(l.consultantName || '').localeCompare(String(r.consultantName || ''))
  })
}

const sortedSingleStoreRows = computed(() => sortRows(filteredConsultantRows.value, metric.value))
const sortedStoreRows = computed(() => {
  if (!props.integratedScope || singleStoreMode.value) return []
  const ids = new Set(
    filteredConsultantRows.value.map((r) => normalizeText(r.storeId)).filter(Boolean),
  )
  return sortRows(
    monthlyStoreRows.value.filter((r) => ids.has(normalizeText(r.storeId))),
    metric.value,
  )
})

function buildPodiumRows(
  rows: Array<Record<string, unknown>>,
  subtitleFn: ((r: Record<string, unknown>) => string) | null,
) {
  return rows.slice(0, 3).map((row) => ({
    key: String(row.rowKey || ''),
    name: String(row.consultantName || ''),
    subtitle: subtitleFn ? subtitleFn(row) : undefined,
    metricValue: getMetricValue(row, metric.value),
    score360: Number(row.score360 || 0),
    soldValue: Number(row.soldValue || 0),
  }))
}

function buildLeaderboardRows(
  rows: Array<Record<string, unknown>>,
  subtitleFn: ((r: Record<string, unknown>) => string) | null = null,
  isStore = false,
) {
  return rows.slice(3).map((row, i) => ({
    ...row,
    position: i + 4,
    subtitle: subtitleFn ? subtitleFn(row) : undefined,
    variation: isStore
      ? storeVariationFor(String(row.rowKey || ''), Number(row.score360 || 0))
      : variationFor(String(row.rowKey || ''), Number(row.score360 || 0)),
  }))
}

const singleStorePodiumRows = computed(() => buildPodiumRows(sortedSingleStoreRows.value, null))
const singleStoreLeaderboardRows = computed(() => buildLeaderboardRows(sortedSingleStoreRows.value))

const groupOrderMap = computed(
  () => new Map(sortedStoreRows.value.map((r, i) => [normalizeText(r.storeId), i])),
)
const groupedConsultantRows = computed(() => {
  if (!props.integratedScope || singleStoreMode.value) return []
  const groups = new Map<
    string,
    { storeId: string; storeName: string; rows: typeof enrichedMonthly.value }
  >()
  filteredConsultantRows.value.forEach((row) => {
    const id = normalizeText(row.storeId)
    const current = groups.get(id) || {
      storeId: id,
      storeName: normalizeText(row.storeName) || 'Loja sem nome',
      rows: [],
    }
    current.rows.push(row)
    groups.set(id, current)
  })
  return [...groups.values()]
    .map((g) => {
      const rows = sortRows(g.rows, metric.value)
      return {
        ...g,
        rows,
        podiumRows: buildPodiumRows(rows, null),
        leaderboardRows: buildLeaderboardRows(rows),
      }
    })
    .sort(
      (l, r) =>
        (groupOrderMap.value.get(l.storeId) ?? Number.MAX_SAFE_INTEGER) -
        (groupOrderMap.value.get(r.storeId) ?? Number.MAX_SAFE_INTEGER),
    )
})

const storePodiumRows = computed(() =>
  buildPodiumRows(sortedStoreRows.value, (r) => `${r.consultantCount || 0} consultor(es)`),
)
const storeLeaderboardRows = computed(() =>
  buildLeaderboardRows(
    sortedStoreRows.value,
    (r) => `${r.consultantCount || 0} consultor(es)`,
    true,
  ),
)

const selectedDrawerRow = computed(() => {
  const key = drawer.currentRowKey.value
  if (!key) return null
  return (
    (filteredConsultantRows.value.find((r) => r.rowKey === key) as Record<string, unknown>) || null
  )
})

const maxSold = computed(() =>
  Math.max(...filteredConsultantRows.value.map((r) => Number(r.soldValue || 0)), 1),
)
const maxPa = computed(() =>
  Math.max(...filteredConsultantRows.value.map((r) => Number(r.paScore || 0)), 0.01),
)
const selectedLegacyRows = computed(
  () => sortRows(filteredConsultantRows.value, metric.value) as Array<Record<string, unknown>>,
)
const currentMetricLabel = computed(
  () => METRIC_OPTIONS.find((o) => o.value === metric.value)?.label || 'Score 360',
)

watch(
  () => props.integratedScope,
  (v) => {
    if (!v) storeFilter.value = FILTER_ALL
  },
  { immediate: true },
)
watch(storeOptions, (opts) => {
  if (storeFilter.value !== FILTER_ALL && !opts.some((o) => o.value === storeFilter.value))
    storeFilter.value = FILTER_ALL
})

function openDrawer(rowKey: string) {
  drawer.open(rowKey)
}
function noop() {}
</script>

<template>
  <section class="admin-panel" data-testid="ranking-panel">
    <header class="admin-panel__header">
      <h2 class="admin-panel__title">Ranking</h2>
      <p class="admin-panel__text">
        {{
          integratedScope
            ? 'Comparativo consolidado das lojas e consultores do tenant ativo.'
            : 'Comparativo de consultores da loja ativa por Score 360 e metricas core.'
        }}
      </p>
    </header>

    <article v-if="errorMessage" class="insight-card">
      <p class="settings-card__text">{{ errorMessage }}</p>
    </article>
    <article v-else-if="pending && !monthlyRows.length" class="insight-card">
      <p class="settings-card__text">Carregando ranking...</p>
    </article>

    <template v-else>
      <RankingFilters
        :date-from="dateFrom"
        :date-to="dateTo"
        :search-term="searchTerm"
        :store-filter="storeFilter"
        :metric="metric"
        :store-options="storeOptions"
        :metric-options="METRIC_OPTIONS"
        :integrated-scope="integratedScope"
        :pending="pending"
        @update:date-from="emit('update:dateFrom', $event)"
        @update:date-to="emit('update:dateTo', $event)"
        @update:search-term="searchTerm = $event"
        @update:store-filter="storeFilter = $event"
        @update:metric="metric = $event"
        @apply-period="emit('applyPeriod')"
        @set-current-month="emit('setCurrentMonth')"
        @set-previous-month="emit('setPreviousMonth')"
      />

      <template v-if="integratedScope && !singleStoreMode">
        <section class="ranking-workspace__section ranking-workspace__section--static">
          <header class="ranking-workspace__section-header">
            <div>
              <h3 class="ranking-workspace__section-title">Ranking de lojas</h3>
              <p class="ranking-workspace__section-text">
                Consolidado por loja usando {{ currentMetricLabel.toLowerCase() }}.
              </p>
            </div>
            <span class="insight-tag">{{ sortedStoreRows.length }} lojas</span>
          </header>
          <div v-if="sortedStoreRows.length">
            <div class="ranking-workspace__store-ranking">
              <RankingPodium :rows="storePodiumRows" :metric="metric" @select="noop" />
              <div v-if="storeLeaderboardRows.length" class="ranking-workspace__leaderboard">
                <RankingLeaderboardCard
                  v-for="row in storeLeaderboardRows"
                  :key="String(row.rowKey)"
                  :row-key="String(row.rowKey)"
                  :position="Number(row.position)"
                  :name="String(row.consultantName)"
                  :subtitle="row.subtitle"
                  :metric="metric"
                  :metric-value="getMetricValue(row, metric)"
                  :variation="typeof row.variation === 'number' ? row.variation : null"
                  @select="noop"
                />
              </div>
            </div>
          </div>
          <div v-else class="player-grid__empty">
            Nenhuma loja encontrada para os filtros atuais.
          </div>
        </section>

        <div v-if="groupedConsultantRows.length" class="ranking-workspace__groups">
          <section
            v-for="group in groupedConsultantRows"
            :key="group.storeId"
            class="ranking-workspace__section"
          >
            <header class="ranking-workspace__section-header">
              <div>
                <h3 class="ranking-workspace__section-title">{{ group.storeName }}</h3>
                <p class="ranking-workspace__section-text">
                  {{ group.rows.length }} consultor(es) no recorte atual.
                </p>
              </div>
            </header>
            <RankingPodium :rows="group.podiumRows" :metric="metric" @select="openDrawer" />
            <div v-if="group.leaderboardRows.length" class="ranking-workspace__leaderboard">
              <RankingLeaderboardCard
                v-for="row in group.leaderboardRows"
                :key="String(row.rowKey)"
                :row-key="String(row.rowKey)"
                :position="Number(row.position)"
                :name="String(row.consultantName)"
                :metric="metric"
                :metric-value="getMetricValue(row, metric)"
                :variation="typeof row.variation === 'number' ? row.variation : null"
                @select="openDrawer"
              />
            </div>
          </section>
        </div>
        <div v-else class="player-grid__empty">
          Nenhum consultor encontrado para os filtros atuais.
        </div>
      </template>

      <template v-else>
        <section class="ranking-workspace__section">
          <header class="ranking-workspace__section-header">
            <div>
              <h3 class="ranking-workspace__section-title">
                {{
                  integratedScope ? `Ranking de ${selectedStoreLabel}` : 'Ranking de consultores'
                }}
              </h3>
              <p class="ranking-workspace__section-text">
                {{ currentMetricLabel }} no recorte selecionado.
              </p>
            </div>
            <span class="insight-tag">{{ sortedSingleStoreRows.length }} consultores</span>
          </header>
          <div v-if="sortedSingleStoreRows.length">
            <RankingPodium :rows="singleStorePodiumRows" :metric="metric" @select="openDrawer" />
            <div v-if="singleStoreLeaderboardRows.length" class="ranking-workspace__leaderboard">
              <RankingLeaderboardCard
                v-for="row in singleStoreLeaderboardRows"
                :key="String(row.rowKey)"
                :row-key="String(row.rowKey)"
                :position="Number(row.position)"
                :name="String(row.consultantName)"
                :subtitle="integratedScope ? undefined : String(row.storeName || '')"
                :metric="metric"
                :metric-value="getMetricValue(row, metric)"
                :variation="typeof row.variation === 'number' ? row.variation : null"
                @select="openDrawer"
              />
            </div>
          </div>
          <div v-else class="player-grid__empty">
            Nenhum consultor encontrado para os filtros atuais.
          </div>
        </section>
        <div
          v-if="alerts.length && !integratedScope"
          class="ranking-workspace__alerts-hint"
          data-testid="ranking-alerts-hint"
        >
          <span>{{ alerts.length }} alerta{{ alerts.length > 1 ? 's' : '' }} de desempenho</span>
          <span class="ranking-workspace__alerts-hint-text">
            Abra o detalhe de um consultor para ver os alertas associados.
          </span>
        </div>
      </template>
    </template>

    <RankingDetailsDrawer
      :row="selectedDrawerRow"
      :alerts="alerts"
      :max-sold="maxSold"
      :max-pa="maxPa"
      :legacy-rows="selectedLegacyRows"
    />
  </section>
</template>

<style scoped>
.ranking-workspace__groups {
  display: grid;
  gap: 1rem;
}
.ranking-workspace__section {
  display: grid;
  gap: 0.85rem;
}
.ranking-workspace__section--static {
  padding-bottom: 0.15rem;
}
.ranking-workspace__section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}
.ranking-workspace__section-title {
  margin: 0;
  font-size: 1rem;
  color: rgb(var(--text) / 0.96);
}
.ranking-workspace__section-text {
  margin: 0.2rem 0 0;
  font-size: 0.78rem;
  color: rgb(var(--muted) / 0.92);
}
.ranking-workspace__store-ranking {
  pointer-events: none;
  opacity: 0.96;
}
.ranking-workspace__leaderboard {
  display: grid;
  gap: 0.55rem;
}
.player-grid__empty {
  padding: 2rem;
  text-align: center;
  color: rgb(var(--muted) / 0.92);
}
.ranking-workspace__alerts-hint {
  display: grid;
  gap: 0.25rem;
  padding: 0.7rem 0.9rem;
  border-radius: 0.7rem;
  border: 1px solid rgb(var(--danger) / 0.32);
  background: rgb(var(--danger) / 0.08);
  color: rgb(var(--text) / 0.92);
  font-size: 0.82rem;
}
.ranking-workspace__alerts-hint-text {
  color: rgb(var(--muted) / 0.92);
  font-size: 0.74rem;
}
@media (max-width: 720px) {
  .ranking-workspace__section-header {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
