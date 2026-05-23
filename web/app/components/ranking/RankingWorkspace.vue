<script setup>
import { computed, ref, watch } from 'vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import { computeScore360, useGamificationConfig } from '~/composables/useGamificationConfig'
import { useRankingDetailsDrawer } from '~/composables/useRankingDetailsDrawer'
import RankingDetailsDrawer from '~/components/ranking/RankingDetailsDrawer.vue'
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

const props = defineProps({
  report: {
    type: Object,
    default: null,
  },
  pending: {
    type: Boolean,
    default: false,
  },
  errorMessage: {
    type: String,
    default: '',
  },
  integratedScope: {
    type: Boolean,
    default: false,
  },
})

const { scoreWeights } = useGamificationConfig()
const drawer = useRankingDetailsDrawer()

const metric = ref('score360')
const searchTerm = ref('')
const storeFilter = ref(FILTER_ALL)

const monthlyRows = computed(() => props.report?.monthlyRows || [])
const dailyRows = computed(() => props.report?.dailyRows || [])
const alerts = computed(() => props.report?.alerts || [])

const monthlyConsultantMaxSold = computed(() =>
  Math.max(...monthlyRows.value.map((row) => Number(row.soldValue || 0)), 1),
)
const monthlyConsultantMaxPa = computed(() =>
  Math.max(...monthlyRows.value.map((row) => Number(row.paScore || 0)), 0.01),
)

function buildRowKey(row) {
  return `${String(row.storeId || '').trim()}:${String(row.consultantId || '').trim()}`
}

function normalizeText(value) {
  return String(value || '').trim()
}

function normalizeSearch(value) {
  return normalizeText(value)
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .toLowerCase()
}

function matchesSearch(searchValue, candidates) {
  if (!searchValue) {
    return true
  }

  return candidates.some((candidate) => normalizeSearch(candidate).includes(searchValue))
}

const enrichedMonthly = computed(() =>
  monthlyRows.value.map((row) => {
    const score360 = computeScore360(
      {
        conversionRate: Number(row.conversionRate || 0),
        soldValue: Number(row.soldValue || 0),
        qualityScore: Number(row.qualityScore || 0),
        paScore: Number(row.paScore || 0),
        queueJumpServices: Number(row.queueJumpServices || 0),
        attendances: Number(row.attendances || 0),
      },
      {
        maxSold: monthlyConsultantMaxSold.value,
        maxPa: monthlyConsultantMaxPa.value,
        weights: scoreWeights.value,
      },
    )
    return {
      ...row,
      rowKey: buildRowKey(row),
      consultantName: normalizeText(row.consultantName) || 'Consultor sem nome',
      storeId: normalizeText(row.storeId),
      storeName: normalizeText(row.storeName) || 'Loja sem nome',
      score360,
    }
  }),
)

const enrichedDaily = computed(() =>
  dailyRows.value.map((row) => ({
    ...row,
    rowKey: buildRowKey(row),
    consultantName: normalizeText(row.consultantName) || 'Consultor sem nome',
    storeId: normalizeText(row.storeId),
    storeName: normalizeText(row.storeName) || 'Loja sem nome',
    score360: computeScore360(
      {
        conversionRate: Number(row.conversionRate || 0),
        soldValue: Number(row.soldValue || 0),
        qualityScore: Number(row.qualityScore || 0),
        paScore: Number(row.paScore || 0),
        queueJumpServices: Number(row.queueJumpServices || 0),
        attendances: Number(row.attendances || 0),
      },
      {
        maxSold: monthlyConsultantMaxSold.value,
        maxPa: monthlyConsultantMaxPa.value,
        weights: scoreWeights.value,
      },
    ),
  })),
)

const dailyScoreMap = computed(() => {
  const map = new Map()
  enrichedDaily.value.forEach((row) => {
    map.set(row.rowKey, row.score360)
  })
  return map
})

function variationFor(rowKey, monthlyScore) {
  const daily = dailyScoreMap.value.get(rowKey)
  if (typeof daily !== 'number' || daily === 0 || monthlyScore === 0) return null
  return ((daily - monthlyScore) / monthlyScore) * 100
}

function buildStoreAggregates(rows) {
  const grouped = new Map()

  rows.forEach((row) => {
    const storeId = normalizeText(row.storeId)
    if (!storeId) {
      return
    }

    const weight = Math.max(1, Number(row.attendances || 0))
    const current = grouped.get(storeId) || {
      rowKey: `store:${storeId}`,
      consultantId: storeId,
      consultantName: normalizeText(row.storeName) || 'Loja sem nome',
      storeId,
      storeName: normalizeText(row.storeName) || 'Loja sem nome',
      attendances: 0,
      conversions: 0,
      soldValue: 0,
      score360Weighted: 0,
      score360Weight: 0,
      ticketAverageWeighted: 0,
      ticketAverageWeight: 0,
      totalPieces: 0,
      qualityWeighted: 0,
      qualityWeight: 0,
      avgDurationTotal: 0,
      queueJumpServices: 0,
      consultantCount: 0,
    }

    current.attendances += Number(row.attendances || 0)
    current.conversions += Number(row.conversions || 0)
    current.soldValue += Number(row.soldValue || 0)
    current.score360Weighted += Number(row.score360 || 0) * weight
    current.score360Weight += weight
    current.ticketAverageWeighted += Number(row.ticketAverage || 0) * weight
    current.ticketAverageWeight += weight
    current.totalPieces += Math.max(
      Number(row.conversions || 0),
      Number(row.paScore || 0) * Number(row.conversions || 0),
    )
    current.qualityWeighted += Number(row.qualityScore || 0) * weight
    current.qualityWeight += weight
    current.avgDurationTotal += Number(row.avgDurationMs || 0) * Number(row.attendances || 0)
    current.queueJumpServices += Number(row.queueJumpServices || 0)
    current.consultantCount += 1

    grouped.set(storeId, current)
  })

  return [...grouped.values()].map((entry) => ({
    rowKey: entry.rowKey,
    consultantId: entry.consultantId,
    consultantName: entry.consultantName,
    storeId: entry.storeId,
    storeName: entry.storeName,
    attendances: entry.attendances,
    conversions: entry.conversions,
    soldValue: entry.soldValue,
    conversionRate: entry.attendances > 0 ? (entry.conversions / entry.attendances) * 100 : 0,
    ticketAverage:
      entry.ticketAverageWeight > 0
        ? entry.ticketAverageWeighted / entry.ticketAverageWeight
        : 0,
    paScore: entry.conversions > 0 ? Math.max(1, entry.totalPieces / entry.conversions) : 0,
    qualityScore: entry.qualityWeight > 0 ? entry.qualityWeighted / entry.qualityWeight : 0,
    avgDurationMs: entry.attendances > 0 ? entry.avgDurationTotal / entry.attendances : 0,
    queueJumpServices: entry.queueJumpServices,
    score360: entry.score360Weight > 0 ? entry.score360Weighted / entry.score360Weight : 0,
    consultantCount: entry.consultantCount,
  }))
}

const monthlyStoreRows = computed(() => buildStoreAggregates(enrichedMonthly.value))
const dailyStoreScoreMap = computed(() => {
  const map = new Map()
  buildStoreAggregates(enrichedDaily.value).forEach((row) => {
    map.set(row.rowKey, row.score360)
  })
  return map
})

function getMetricValue(row, key) {
  if (key === 'score360') return row.score360 || 0
  return Number(row[key] || 0)
}

const storeOptions = computed(() => {
  const stores = new Map()
  enrichedMonthly.value.forEach((row) => {
    const storeId = normalizeText(row.storeId)
    const storeName = normalizeText(row.storeName)
    if (!storeId || !storeName || stores.has(storeId)) return
    stores.set(storeId, { value: storeId, label: storeName })
  })
  return [
    { value: FILTER_ALL, label: 'Todas as lojas' },
    ...[...stores.values()].sort((a, b) => a.label.localeCompare(b.label)),
  ]
})

const singleStoreMode = computed(() => props.integratedScope && storeFilter.value !== FILTER_ALL)

const selectedStoreLabel = computed(() => {
  if (storeFilter.value === FILTER_ALL) {
    return 'Todas as lojas'
  }

  return (
    storeOptions.value.find((option) => option.value === storeFilter.value)?.label ||
    'Loja selecionada'
  )
})

const normalizedSearchTerm = computed(() => normalizeSearch(searchTerm.value))

const filteredConsultantRows = computed(() => {
  return enrichedMonthly.value.filter((row) => {
    if (
      props.integratedScope &&
      storeFilter.value !== FILTER_ALL &&
      row.storeId !== storeFilter.value
    ) {
      return false
    }

    return matchesSearch(normalizedSearchTerm.value, [row.consultantName, row.storeName])
  })
})

function sortRows(rows, key) {
  return [...rows].sort((left, right) => {
    const diff = getMetricValue(right, key) - getMetricValue(left, key)
    if (diff !== 0) {
      return diff
    }

    const leftName = String(left.consultantName || '')
    const rightName = String(right.consultantName || '')

    return leftName.localeCompare(rightName)
  })
}

const sortedSingleStoreRows = computed(() => sortRows(filteredConsultantRows.value, metric.value))
const sortedStoreRows = computed(() => {
  if (!props.integratedScope || singleStoreMode.value) {
    return []
  }

  const matchingStoreIds = new Set(
    filteredConsultantRows.value.map((row) => normalizeText(row.storeId)).filter(Boolean),
  )

  return sortRows(
    monthlyStoreRows.value.filter((row) => matchingStoreIds.has(normalizeText(row.storeId))),
    metric.value,
  )
})

function buildPodiumRows(rows, subtitleResolver) {
  return rows.slice(0, 3).map((row) => ({
    key: row.rowKey,
    name: row.consultantName,
    subtitle: typeof subtitleResolver === 'function' ? subtitleResolver(row) : subtitleResolver,
    metricValue: getMetricValue(row, metric.value),
    score360: row.score360,
    soldValue: row.soldValue,
  }))
}

function buildLeaderboardRows(rows, options = {}) {
  const { subtitleResolver = null, showVariation = true, storeVariation = false } = options

  return rows.slice(3).map((row, index) => {
    const position = index + 4
    const variation = showVariation
      ? storeVariation
        ? (() => {
            const daily = dailyStoreScoreMap.value.get(row.rowKey)
            if (typeof daily !== 'number' || daily === 0 || row.score360 === 0) return null
            return ((daily - row.score360) / row.score360) * 100
          })()
        : variationFor(row.rowKey, row.score360)
      : null

    return {
      ...row,
      position,
      variation,
      subtitle: typeof subtitleResolver === 'function' ? subtitleResolver(row) : subtitleResolver,
    }
  })
}

const singleStorePodiumRows = computed(() => buildPodiumRows(sortedSingleStoreRows.value, null))
const singleStoreLeaderboardRows = computed(() => buildLeaderboardRows(sortedSingleStoreRows.value))

const groupOrderMap = computed(
  () =>
    new Map(sortedStoreRows.value.map((row, index) => [normalizeText(row.storeId), index])),
)

const groupedConsultantRows = computed(() => {
  if (!props.integratedScope || singleStoreMode.value) {
    return []
  }

  const groups = new Map()

  filteredConsultantRows.value.forEach((row) => {
    const storeId = normalizeText(row.storeId)
    const current =
      groups.get(storeId) || {
        storeId,
        storeName: normalizeText(row.storeName) || 'Loja sem nome',
        rows: [],
      }

    current.rows.push(row)
    groups.set(storeId, current)
  })

  return [...groups.values()]
    .map((group) => {
      const rows = sortRows(group.rows, metric.value)

      return {
        ...group,
        rows,
        podiumRows: buildPodiumRows(rows, null),
        leaderboardRows: buildLeaderboardRows(rows),
      }
    })
    .sort(
      (left, right) =>
        (groupOrderMap.value.get(left.storeId) ?? Number.MAX_SAFE_INTEGER) -
        (groupOrderMap.value.get(right.storeId) ?? Number.MAX_SAFE_INTEGER),
    )
})

const storePodiumRows = computed(() =>
  buildPodiumRows(sortedStoreRows.value, (row) => `${row.consultantCount || 0} consultor(es)`),
)
const storeLeaderboardRows = computed(() =>
  buildLeaderboardRows(sortedStoreRows.value, {
    subtitleResolver: (row) => `${row.consultantCount || 0} consultor(es)`,
    storeVariation: true,
  }),
)

const selectedDrawerRow = computed(() => {
  const key = drawer.currentRowKey.value
  if (!key) return null
  return filteredConsultantRows.value.find((row) => row.rowKey === key) || null
})

const maxSold = computed(() =>
  Math.max(...filteredConsultantRows.value.map((row) => Number(row.soldValue || 0)), 1),
)
const maxPa = computed(() =>
  Math.max(...filteredConsultantRows.value.map((row) => Number(row.paScore || 0)), 0.01),
)

const selectedLegacyRows = computed(() => sortRows(filteredConsultantRows.value, metric.value))
const currentMetricLabel = computed(
  () => METRIC_OPTIONS.find((option) => option.value === metric.value)?.label || 'Score 360',
)

watch(
  () => props.integratedScope,
  (value) => {
    if (!value) {
      storeFilter.value = FILTER_ALL
    }
  },
  { immediate: true },
)

watch(storeOptions, (options) => {
  if (storeFilter.value === FILTER_ALL) {
    return
  }

  const exists = options.some((option) => option.value === storeFilter.value)
  if (!exists) {
    storeFilter.value = FILTER_ALL
  }
})

function openDrawer(rowKey) {
  drawer.open(rowKey)
}

function noop() {}

function updateMetric(next) {
  metric.value = String(next || 'score360')
}
</script>

<template>
  <section class="admin-panel" data-testid="ranking-panel">
    <header class="admin-panel__header">
      <h2 class="admin-panel__title">Ranking</h2>
      <p class="admin-panel__text">
        {{
          integratedScope
            ? 'Comparativo consolidado das lojas e consultores do tenant ativo.'
            : 'Comparativo de consultores da loja ativa por Score 360 e métricas core.'
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
      <article class="settings-card ranking-workspace__filters">
        <div class="ranking-workspace__filters-grid">
          <label class="settings-field ranking-workspace__search">
            <span>Buscar</span>
            <input v-model="searchTerm" type="text" placeholder="Consultor ou loja" />
          </label>

          <label v-if="integratedScope" class="settings-field">
            <span>Loja</span>
            <AppSelectField
              :model-value="storeFilter"
              :options="storeOptions"
              placeholder="Todas as lojas"
              @update:model-value="storeFilter = String($event || FILTER_ALL)"
            />
          </label>

          <label class="settings-field">
            <span>Ordenar por</span>
            <AppSelectField
              :model-value="metric"
              :options="METRIC_OPTIONS"
              placeholder="Score 360"
              @update:model-value="updateMetric"
            />
          </label>
        </div>
      </article>

      <template v-if="integratedScope && !singleStoreMode">
        <section class="ranking-workspace__section ranking-workspace__section--static">
          <header class="ranking-workspace__section-header">
            <div>
              <h3 class="ranking-workspace__section-title">Ranking de lojas</h3>
              <p class="ranking-workspace__section-text">
                Consolidado por loja no tenant ativo usando {{ currentMetricLabel.toLowerCase() }}.
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
                  :key="row.rowKey"
                  :row-key="row.rowKey"
                  :position="row.position"
                  :name="row.consultantName"
                  :subtitle="row.subtitle"
                  :metric="metric"
                  :metric-value="getMetricValue(row, metric)"
                  :variation="row.variation"
                  @select="noop"
                />
              </div>
            </div>
          </div>
          <div v-else class="player-grid__empty">Nenhuma loja encontrada para os filtros atuais.</div>
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
                :key="row.rowKey"
                :row-key="row.rowKey"
                :position="row.position"
                :name="row.consultantName"
                :metric="metric"
                :metric-value="getMetricValue(row, metric)"
                :variation="row.variation"
                @select="openDrawer"
              />
            </div>
          </section>
        </div>

        <div v-else class="player-grid__empty">Nenhum consultor encontrado para os filtros atuais.</div>
      </template>

      <template v-else>
        <section class="ranking-workspace__section">
          <header class="ranking-workspace__section-header">
            <div>
              <h3 class="ranking-workspace__section-title">
                {{ integratedScope ? `Ranking de ${selectedStoreLabel}` : 'Ranking de consultores' }}
              </h3>
              <p class="ranking-workspace__section-text">
                {{ currentMetricLabel }} no recorte mensal atual.
              </p>
            </div>
            <span class="insight-tag">{{ sortedSingleStoreRows.length }} consultores</span>
          </header>

          <div v-if="sortedSingleStoreRows.length">
            <RankingPodium :rows="singleStorePodiumRows" :metric="metric" @select="openDrawer" />

            <div v-if="singleStoreLeaderboardRows.length" class="ranking-workspace__leaderboard">
              <RankingLeaderboardCard
                v-for="row in singleStoreLeaderboardRows"
                :key="row.rowKey"
                :row-key="row.rowKey"
                :position="row.position"
                :name="row.consultantName"
                :subtitle="integratedScope ? undefined : row.storeName"
                :metric="metric"
                :metric-value="getMetricValue(row, metric)"
                :variation="row.variation"
                @select="openDrawer"
              />
            </div>
          </div>
          <div v-else class="player-grid__empty">Nenhum consultor encontrado para os filtros atuais.</div>
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
.ranking-workspace__filters-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.7fr) repeat(2, minmax(0, 1fr));
  gap: 0.85rem;
}

.ranking-workspace__search {
  min-width: 0;
}

.ranking-workspace__search input {
  width: 100%;
}

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
  color: rgba(248, 250, 252, 0.96);
}

.ranking-workspace__section-text {
  margin: 0.2rem 0 0;
  font-size: 0.78rem;
  color: rgba(148, 163, 184, 0.92);
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
  color: rgba(148, 163, 184, 0.92);
}

.ranking-workspace__alerts-hint {
  display: grid;
  gap: 0.25rem;
  padding: 0.7rem 0.9rem;
  border-radius: 0.7rem;
  border: 1px solid rgba(244, 114, 182, 0.32);
  background: rgba(244, 114, 182, 0.08);
  color: rgba(248, 250, 252, 0.92);
  font-size: 0.82rem;
}

.ranking-workspace__alerts-hint-text {
  color: rgba(148, 163, 184, 0.92);
  font-size: 0.74rem;
}

@media (max-width: 980px) {
  .ranking-workspace__filters-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 720px) {
  .ranking-workspace__filters-grid {
    grid-template-columns: minmax(0, 1fr);
  }

  .ranking-workspace__section-header {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
