<script setup lang="ts">
import AppSelectField from '~/components/ui/AppSelectField.vue'
import { useObservabilityApi, useObsQuery } from '~/composables/useObservability'
import { getApiErrorMessage } from '~/utils/api-client'
import type { ObsOverview, ObsRunRequests } from '~/types/observability'
import { formatDate } from '~/utils/observability/format'

const emit = defineEmits<{ finished: [] }>()

const api = useObservabilityApi()
const requests = useObsQuery<ObsRunRequests>('/run-requests')
const overview = useObsQuery<ObsOverview>('/overview', () => ({ days: 7 }))

const moduleChoice = ref('all')
const only = ref('')
const submitting = ref(false)
const message = ref('')
const submitError = ref('')

const canRun = computed(() => requests.data.value?.canRun === true)
const workerOnline = computed(() => requests.data.value?.workerOnline === true)
const items = computed(() => (requests.data.value?.items || []).slice(0, 8))
const hasActive = computed(() =>
  items.value.some((i) => i.status === 'queued' || i.status === 'running'),
)
const moduleOptions = computed(() => [
  { value: 'all', label: 'Todos os modulos' },
  ...(overview.data.value?.modules || []).map((m) => ({ value: m.module, label: m.module })),
])

const disabledReason = computed(() => {
  if (!requests.data.value) return ''
  if (!canRun.value)
    return 'Desabilitado em producao: o runner so executa em ambiente local ou de teste.'
  if (!workerOnline.value) return 'Worker offline. No terminal do projeto rode: npm run obs:worker'
  return ''
})

async function submit() {
  submitting.value = true
  message.value = ''
  submitError.value = ''
  try {
    await api.post('/run-requests', {
      module: moduleChoice.value === 'all' ? undefined : moduleChoice.value,
      only: only.value.trim() || undefined,
    })
    message.value = 'Execucao enfileirada. O worker inicia em instantes.'
    await requests.reload()
  } catch (cause) {
    submitError.value = getApiErrorMessage(cause, 'Nao foi possivel enfileirar a execucao.')
  } finally {
    submitting.value = false
  }
}

// Enquanto ha execucao na fila ou rodando, atualiza o status a cada 5 s.
let timer: ReturnType<typeof setInterval> | null = null
let wasActive = false
watch(hasActive, (active) => {
  if (wasActive && !active) emit('finished')
  wasActive = active
})
onMounted(() => {
  timer = setInterval(() => {
    if (hasActive.value || !workerOnline.value) void requests.reload()
  }, 5000)
})
onBeforeUnmount(() => timer && clearInterval(timer))

const STATUS_BAND = { queued: 'warn', running: 'warn', done: 'good', failed: 'bad' } as const
const STATUS_LABEL = {
  queued: 'na fila',
  running: 'rodando',
  done: 'concluida',
  failed: 'falhou',
} as const
</script>

<template>
  <section class="obs-section">
    <h3 class="obs-section__title">Rodar runner</h3>
    <div class="obs-runner">
      <span class="obs-workerstate">
        <span :class="['obs-dot', workerOnline ? 'obs-dot--on' : '']"></span>
        Worker {{ workerOnline ? 'online' : 'offline' }}
      </span>
      <AppSelectField
        v-model="moduleChoice"
        :options="moduleOptions"
        compact
        :show-leading-icon="false"
      />
      <UInput v-model="only" placeholder="only (opcional), ex.: api,pages" size="sm" />
      <UButton
        label="Rodar runner"
        icon="i-lucide-play"
        color="primary"
        :loading="submitting"
        :disabled="!canRun || !workerOnline || submitting"
        @click="submit"
      />
    </div>
    <p v-if="disabledReason" class="obs-hint obs-text--warn">{{ disabledReason }}</p>
    <p v-if="message" class="obs-hint obs-text--good">{{ message }}</p>
    <p v-if="submitError" class="obs-hint obs-text--bad">{{ submitError }}</p>
    <p v-if="requests.error.value" class="obs-hint obs-text--bad">{{ requests.error.value }}</p>

    <div v-if="items.length" class="obs-tablewrap">
      <table class="obs-table">
        <thead>
          <tr>
            <th>Pedido</th>
            <th>Modulo</th>
            <th>Only</th>
            <th>Status</th>
            <th>Execucao</th>
            <th>Quando</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in items" :key="item.id">
            <td class="obs-mono">{{ item.id }}</td>
            <td>{{ item.module || 'todos' }}</td>
            <td class="obs-mono">{{ item.only || '-' }}</td>
            <td>
              <span :class="['obs-band', `obs-band--${STATUS_BAND[item.status] || 'none'}`]">
                {{ STATUS_LABEL[item.status] || item.status }}
              </span>
            </td>
            <td class="obs-mono">{{ item.runId || '-' }}</td>
            <td>{{ formatDate(item.requestedAt || item.createdAt) }}</td>
          </tr>
        </tbody>
      </table>
    </div>
    <p v-else-if="!requests.loading.value" class="obs-hint">Nenhum pedido de execucao ainda.</p>
  </section>
</template>
