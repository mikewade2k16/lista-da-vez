<script setup>
import { computed } from 'vue'
import { useReportsStore } from '~/stores/reports'

const reportsStore = useReportsStore()

const pauses = computed(() => reportsStore.pauses || null)
const summary = computed(
  () =>
    pauses.value?.summary || {
      totalPauses: 0,
      totalDurationMs: 0,
      averageDurationMs: 0,
      distinctConsultants: 0,
    },
)
const byConsultant = computed(() =>
  Array.isArray(pauses.value?.byConsultant) ? pauses.value.byConsultant : [],
)
const byReason = computed(() =>
  Array.isArray(pauses.value?.byReason) ? pauses.value.byReason : [],
)
const byHour = computed(() => (Array.isArray(pauses.value?.byHour) ? pauses.value.byHour : []))
const recentRows = computed(() =>
  (Array.isArray(pauses.value?.rows) ? pauses.value.rows : []).slice(0, 20),
)

const hasPauses = computed(() => summary.value.totalPauses > 0)
const maxReasonCount = computed(() =>
  Math.max(1, ...byReason.value.map((item) => Number(item.count || 0))),
)
const maxHourCount = computed(() =>
  Math.max(1, ...byHour.value.map((item) => Number(item.count || 0))),
)

function formatDuration(ms) {
  const totalSeconds = Math.max(0, Math.round(Number(ms || 0) / 1000))
  const hours = Math.floor(totalSeconds / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  const seconds = totalSeconds % 60

  if (hours > 0) {
    return `${hours}h${String(minutes).padStart(2, '0')}m`
  }

  if (minutes > 0) {
    return `${minutes}m${String(seconds).padStart(2, '0')}s`
  }

  return `${seconds}s`
}

function formatClock(epochMillis) {
  const value = Number(epochMillis || 0)
  if (!value) {
    return '--:--'
  }

  return new Date(value).toLocaleTimeString('pt-BR', {
    hour: '2-digit',
    minute: '2-digit',
  })
}

function formatDate(epochMillis) {
  const value = Number(epochMillis || 0)
  if (!value) {
    return '-'
  }

  return new Date(value).toLocaleDateString('pt-BR', {
    day: '2-digit',
    month: '2-digit',
  })
}

function reasonWidth(count) {
  return `${((Number(count || 0) / maxReasonCount.value) * 100).toFixed(1)}%`
}

function hourWidth(count) {
  return `${((Number(count || 0) / maxHourCount.value) * 100).toFixed(1)}%`
}
</script>

<template>
  <section class="reports-pauses" data-testid="reports-pauses">
    <header class="intel-card__header">
      <h3 class="insight-card__title">Pausas dos consultores</h3>
      <span class="insight-tag">{{ summary.totalPauses }} pausas</span>
    </header>

    <p v-if="!pauses" class="insight-empty">
      Metrica de pausas indisponivel no momento. Recarregue apos a atualizacao do servidor.
    </p>

    <template v-else>
      <section class="metric-grid" data-testid="reports-pauses-summary">
        <article class="metric-card">
          <span class="metric-card__label">Total de pausas</span>
          <strong class="metric-card__value">{{ summary.totalPauses }}</strong>
        </article>
        <article class="metric-card">
          <span class="metric-card__label">Tempo total pausado</span>
          <strong class="metric-card__value">{{ formatDuration(summary.totalDurationMs) }}</strong>
        </article>
        <article class="metric-card">
          <span class="metric-card__label">Duracao media</span>
          <strong class="metric-card__value">
            {{ formatDuration(summary.averageDurationMs) }}
          </strong>
        </article>
        <article class="metric-card">
          <span class="metric-card__label">Consultores com pausa</span>
          <strong class="metric-card__value">{{ summary.distinctConsultants }}</strong>
        </article>
      </section>

      <p v-if="!hasPauses" class="insight-empty">
        Nenhuma pausa registrada no periodo e filtros selecionados.
      </p>

      <template v-else>
        <div class="report-dist-grid">
          <article class="insight-card">
            <header class="intel-card__header">
              <h3 class="insight-card__title">Motivos de pausa</h3>
            </header>
            <div v-for="item in byReason" :key="item.reason" class="dist-bar-row">
              <span class="dist-bar-row__label">{{ item.reason }}</span>
              <div class="dist-bar-row__track">
                <div class="dist-bar-row__fill" :style="{ width: reasonWidth(item.count) }"></div>
              </div>
              <span class="dist-bar-row__count">
                {{ item.count }} · {{ formatDuration(item.totalDurationMs) }}
              </span>
            </div>
          </article>

          <article class="insight-card">
            <header class="intel-card__header">
              <h3 class="insight-card__title">Pausas por hora (UTC)</h3>
            </header>
            <span v-if="!byHour.length" class="insight-empty">Sem dados para o periodo.</span>
            <div v-for="item in byHour" v-else :key="item.hour" class="dist-bar-row">
              <span class="dist-bar-row__label">{{ item.hour }}h</span>
              <div class="dist-bar-row__track">
                <div class="dist-bar-row__fill" :style="{ width: hourWidth(item.count) }"></div>
              </div>
              <span class="dist-bar-row__count">{{ item.count }}</span>
            </div>
          </article>
        </div>

        <article class="insight-card insight-card--wide">
          <header class="intel-card__header">
            <h3 class="insight-card__title">Pausas por consultor</h3>
            <span class="insight-tag">{{ byConsultant.length }} consultores</span>
          </header>
          <div class="insight-table-wrap">
            <table class="insight-table">
              <thead>
                <tr>
                  <th>Consultor</th>
                  <th>Pausas</th>
                  <th>Tempo total</th>
                  <th>Duracao media</th>
                  <th>Principais motivos</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="row in byConsultant" :key="row.consultantId">
                  <td>{{ row.consultantName || '-' }}</td>
                  <td>{{ row.pauseCount }}</td>
                  <td>{{ formatDuration(row.totalDurationMs) }}</td>
                  <td>{{ formatDuration(row.averageDurationMs) }}</td>
                  <td>
                    {{
                      (row.byReason || [])
                        .slice(0, 3)
                        .map((reason) => `${reason.reason} (${reason.count})`)
                        .join(', ') || '-'
                    }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </article>

        <article class="insight-card insight-card--wide">
          <header class="intel-card__header">
            <h3 class="insight-card__title">Ultimas pausas</h3>
            <span class="insight-tag">{{ recentRows.length }} registros</span>
          </header>
          <div class="insight-table-wrap">
            <table class="insight-table">
              <thead>
                <tr>
                  <th>Consultor</th>
                  <th>Motivo</th>
                  <th>Data</th>
                  <th>Inicio</th>
                  <th>Fim</th>
                  <th>Duracao</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="(row, index) in recentRows"
                  :key="`${row.consultantId}-${row.startedAt}-${index}`"
                >
                  <td>{{ row.consultantName || '-' }}</td>
                  <td>{{ row.reason }}</td>
                  <td>{{ formatDate(row.startedAt) }}</td>
                  <td>{{ formatClock(row.startedAt) }}</td>
                  <td>{{ formatClock(row.endedAt) }}</td>
                  <td>{{ formatDuration(row.durationMs) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </article>
      </template>
    </template>
  </section>
</template>

<style scoped>
.reports-pauses {
  display: grid;
  gap: 1rem;
}
</style>
