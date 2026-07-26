<script setup>
import { computed } from 'vue'

const props = defineProps({
  data: {
    type: Object,
    default: () => ({}),
  },
})

const summary = computed(
  () =>
    props.data?.summary || {
      total: 0,
      pending: 0,
      validated: 0,
      cancelled: 0,
      snoozeCount: 0,
    },
)

const summaryItems = computed(() => [
  { label: 'Nao encerrados pelo consultor', value: summary.value.total },
  { label: 'Pendentes da gestao', value: summary.value.pending },
  { label: 'Encerrados pela gestao', value: summary.value.validated },
  { label: 'Metricas canceladas', value: summary.value.cancelled },
  { label: 'Adiamentos antes do fechamento', value: summary.value.snoozeCount },
])

function formatTimestamp(value) {
  const timestamp = Number(value || 0)
  if (timestamp <= 0) return '-'

  return new Intl.DateTimeFormat('pt-BR', {
    dateStyle: 'short',
    timeStyle: 'short',
  }).format(new Date(timestamp))
}

function statusLabel(value) {
  switch (String(value || '').trim()) {
    case 'validated':
      return 'Encerrado pela gestao'
    case 'cancelled':
      return 'Metrica cancelada'
    default:
      return 'Pendente'
  }
}

function closedByLabel(item) {
  if (item?.status === 'pending') return 'Aguardando'
  return item?.closedByName || item?.closedByUserId || 'Nao identificado'
}

function reasonLabel(item) {
  if (item?.status === 'pending') return 'Aguardando justificativa da gestao'
  return item?.reason || 'Motivo nao informado'
}
</script>

<template>
  <article class="insight-card insight-card--wide" data-testid="data-auto-close">
    <div>
      <h3 class="insight-card__title">Atendimentos nao encerrados pelo consultor</h3>
      <p class="auto-close-data__description">
        Auditoria dos fechamentos automaticos, com responsabilidade do consultor e da gestao.
      </p>
    </div>

    <div class="insight-time-grid">
      <span v-for="item in summaryItems" :key="item.label" class="insight-tag">
        {{ item.label }}:
        <strong>{{ item.value }}</strong>
      </span>
    </div>

    <div class="auto-close-data__grids">
      <section>
        <h4 class="auto-close-data__subtitle">Numeros por consultor</h4>
        <div class="insight-table-wrap">
          <table class="insight-table">
            <thead>
              <tr>
                <th>Quem deixou de encerrar</th>
                <th>Loja</th>
                <th>Total</th>
                <th>Pendentes</th>
                <th>Encerrados</th>
                <th>Cancelados</th>
                <th>Adiamentos</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="!data?.byConsultant?.length">
                <td colspan="7">Nenhum fechamento automatico no periodo.</td>
              </tr>
              <tr
                v-for="item in data?.byConsultant || []"
                :key="`${item.storeId}-${item.consultantId}`"
              >
                <td>{{ item.consultantName || 'Consultor nao identificado' }}</td>
                <td>{{ item.storeName || '-' }}</td>
                <td>{{ item.total }}</td>
                <td>{{ item.pending }}</td>
                <td>{{ item.validated }}</td>
                <td>{{ item.cancelled }}</td>
                <td>{{ item.snoozeCount }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section>
        <h4 class="auto-close-data__subtitle">Numeros por loja</h4>
        <div class="insight-table-wrap">
          <table class="insight-table">
            <thead>
              <tr>
                <th>Loja</th>
                <th>Total</th>
                <th>Pendentes</th>
                <th>Encerrados</th>
                <th>Cancelados</th>
                <th>Adiamentos</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="!data?.byStore?.length">
                <td colspan="6">Nenhum fechamento automatico no periodo.</td>
              </tr>
              <tr v-for="item in data?.byStore || []" :key="item.storeId">
                <td>{{ item.storeName || '-' }}</td>
                <td>{{ item.total }}</td>
                <td>{{ item.pending }}</td>
                <td>{{ item.validated }}</td>
                <td>{{ item.cancelled }}</td>
                <td>{{ item.snoozeCount }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </div>

    <section>
      <h4 class="auto-close-data__subtitle">Auditoria recente</h4>
      <div class="insight-table-wrap">
        <table class="insight-table">
          <thead>
            <tr>
              <th>Quem deixou de encerrar</th>
              <th>Loja</th>
              <th>Fechamento automatico</th>
              <th>Status</th>
              <th>Quem encerrou</th>
              <th>Quando encerrou</th>
              <th>Motivo</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="!data?.recent?.length">
              <td colspan="7">Nenhum atendimento para auditar.</td>
            </tr>
            <tr v-for="item in data?.recent || []" :key="item.serviceId">
              <td>{{ item.consultantName || 'Consultor nao identificado' }}</td>
              <td>{{ item.storeName || '-' }}</td>
              <td>{{ formatTimestamp(item.autoClosedAt) }}</td>
              <td>{{ statusLabel(item.status) }}</td>
              <td>{{ closedByLabel(item) }}</td>
              <td>{{ formatTimestamp(item.validatedAt) }}</td>
              <td class="auto-close-data__reason">{{ reasonLabel(item) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </article>
</template>

<style scoped>
.auto-close-data__description {
  margin: 4px 0 0;
  color: var(--text-muted);
  font-size: 0.78rem;
}

.auto-close-data__grids {
  display: grid;
  grid-template-columns: minmax(0, 1.35fr) minmax(0, 0.85fr);
  gap: 12px;
}

.auto-close-data__subtitle {
  margin: 0 0 6px;
  color: var(--text-soft);
  font-size: 0.78rem;
}

.auto-close-data__reason {
  min-width: 240px;
  white-space: normal;
}

@media (max-width: 980px) {
  .auto-close-data__grids {
    grid-template-columns: 1fr;
  }
}
</style>
