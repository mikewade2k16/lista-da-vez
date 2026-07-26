<script setup lang="ts">
import { useAuthStore } from '~/stores/auth'
import { createApiRequest } from '~/utils/api-client'

type SaleRow = Record<string, unknown>
type RecentSalesPayload = Record<string, unknown> | SaleRow[]

const auth = useAuthStore()
const apiRequest = createApiRequest(useRuntimeConfig(), () => auth.accessToken)

const rows = ref<SaleRow[]>([])
const loading = ref(false)
const error = ref('')
const durationMs = ref(0)
const loadedAt = ref<Date | null>(null)

const preferredColumns = [
  'data',
  'date',
  'dtVenda',
  'dataVenda',
  'vendaId',
  'idVenda',
  'codigo',
  'colaborador',
  'nomeColaborador',
  'quantidade',
  'valor',
  'valorVenda',
  'total',
]

const columns = computed(() => {
  const available = new Set(rows.value.flatMap((row) => Object.keys(row)))
  const ordered = preferredColumns.filter((key) => available.has(key))

  for (const row of rows.value) {
    for (const key of Object.keys(row)) {
      if (!ordered.includes(key)) ordered.push(key)
      if (ordered.length >= 8) return ordered
    }
  }

  return ordered
})

function extractRows(payload: RecentSalesPayload): SaleRow[] {
  if (Array.isArray(payload)) return payload.slice(0, 20)

  for (const key of ['records', 'items', 'sales', 'vendas', 'data', 'results']) {
    const candidate = payload[key]
    if (Array.isArray(candidate)) return candidate.slice(0, 20) as SaleRow[]
  }

  return []
}

function label(key: string): string {
  return key
    .replace(/([a-z0-9])([A-Z])/g, '$1 $2')
    .replaceAll('_', ' ')
    .replace(/\b\w/g, (letter) => letter.toUpperCase())
}

function display(value: unknown): string {
  if (value === null || value === undefined || value === '') return '-'
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}

async function loadSales(): Promise<void> {
  loading.value = true
  error.value = ''
  const startedAt = performance.now()

  try {
    await auth.ensureSession()
    const response = (await apiRequest('/v1/bi/perola/sales/recent')) as RecentSalesPayload
    rows.value = extractRows(response)
    loadedAt.value = new Date()
  } catch (cause) {
    rows.value = []
    error.value =
      cause instanceof Error
        ? cause.message
        : 'Não foi possível consultar GET vendas/colaboradores.'
  } finally {
    durationMs.value = Math.round(performance.now() - startedAt)
    loading.value = false
  }
}

onMounted(loadSales)
</script>

<template>
  <section class="sales">
    <header class="sales__hero">
      <div>
        <p class="sales__eyebrow">VENDAS DATAJOIAS</p>
        <h2>Últimas 20 vendas</h2>
        <p class="sales__description">
          Dados exclusivos do GET vendas/colaboradores, ordenados do mais recente para o mais
          antigo.
        </p>
      </div>

      <button class="sales__refresh" type="button" :disabled="loading" @click="loadSales">
        {{ loading ? 'Consultando API...' : 'Atualizar vendas' }}
      </button>
    </header>

    <div class="sales__panel">
      <div class="sales__summary">
        <div>
          <strong>
            {{ loading ? 'Consultando a Datajoias...' : `${rows.length} vendas exibidas` }}
          </strong>
          <span v-if="loadedAt">
            Atualizado às
            {{ loadedAt.toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' }) }}
          </span>
        </div>
        <span class="sales__source">GET vendas/colaboradores · {{ durationMs }} ms</span>
      </div>

      <div v-if="error" class="sales__state sales__state--error">
        <strong>Falha na API Datajoias.</strong>
        <span>{{ error }}</span>
      </div>

      <div v-else-if="loading && rows.length === 0" class="sales__state">
        Buscando as vendas na API...
      </div>

      <div v-else-if="rows.length === 0" class="sales__state">
        O GET vendas/colaboradores retornou zero registros.
      </div>

      <div v-else class="sales__table-wrap">
        <table class="sales__table">
          <thead>
            <tr>
              <th v-for="column in columns" :key="column">{{ label(column) }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(row, index) in rows" :key="index">
              <td v-for="column in columns" :key="column">{{ display(row[column]) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </section>
</template>

<style scoped>
.sales {
  display: grid;
  gap: 16px;
}

.sales__hero,
.sales__panel {
  border: 1px solid var(--border-subtle, #1e293b);
  border-radius: 18px;
  background:
    radial-gradient(circle at 96% 0%, rgb(34 197 94 / 7%), transparent 28%), var(--surface, #0c1320);
}

.sales__hero {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  padding: 22px 20px;
}

.sales__eyebrow {
  margin: 0 0 10px;
  color: #22c55e;
  font-size: 12px;
  font-weight: 800;
  letter-spacing: 0.06em;
}

.sales h2 {
  margin: 0;
  color: var(--text-primary, #f8fafc);
  font-size: clamp(22px, 3vw, 28px);
  font-weight: 650;
}

.sales__description {
  margin: 8px 0 0;
  color: var(--text-secondary, #93a8c7);
  font-size: 14px;
}

.sales__refresh {
  flex: 0 0 auto;
  min-height: 42px;
  padding: 0 18px;
  border: 1px solid #22c55e;
  border-radius: 14px;
  background: rgb(34 197 94 / 8%);
  color: #22c55e;
  cursor: pointer;
  font: inherit;
  font-weight: 750;
}

.sales__refresh:disabled {
  cursor: wait;
  opacity: 0.65;
}

.sales__panel {
  overflow: hidden;
}

.sales__summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 18px 20px;
  border-bottom: 1px solid var(--border-subtle, #1e293b);
}

.sales__summary > div {
  display: grid;
  gap: 5px;
}

.sales__summary strong {
  color: var(--text-primary, #f8fafc);
}

.sales__summary span {
  color: var(--text-secondary, #93a8c7);
  font-size: 13px;
}

.sales__source {
  color: #4ade80 !important;
  font-weight: 700;
}

.sales__state {
  display: grid;
  place-content: center;
  min-height: 180px;
  gap: 8px;
  padding: 28px;
  color: var(--text-secondary, #93a8c7);
  text-align: center;
}

.sales__state--error {
  color: #fda4af;
}

.sales__table-wrap {
  overflow-x: auto;
}

.sales__table {
  width: 100%;
  min-width: 880px;
  border-collapse: collapse;
  font-size: 13px;
}

.sales__table th,
.sales__table td {
  max-width: 280px;
  padding: 13px 16px;
  overflow: hidden;
  border-bottom: 1px solid var(--border-subtle, #1e293b);
  color: var(--text-secondary, #b3c2d8);
  text-align: left;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sales__table th {
  color: var(--text-primary, #f8fafc);
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.05em;
  text-transform: uppercase;
}

.sales__table tbody tr:hover {
  background: rgb(148 163 184 / 4%);
}

.sales__table tbody tr:last-child td {
  border-bottom: 0;
}

@media (max-width: 720px) {
  .sales__hero,
  .sales__summary {
    align-items: stretch;
    flex-direction: column;
  }

  .sales__refresh {
    width: 100%;
  }
}
</style>
