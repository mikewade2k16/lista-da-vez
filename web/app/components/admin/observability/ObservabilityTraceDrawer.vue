<script setup lang="ts">
import OmniEntityDrawer from '~/components/ui/OmniEntityDrawer.vue'
import ObservabilityState from '~/components/admin/observability/ObservabilityState.vue'
import { useObservabilityApi } from '~/composables/useObservability'
import { getApiErrorMessage } from '~/utils/api-client'
import type { ObsTrace } from '~/types/observability'
import { formatMs } from '~/utils/observability/format'

const props = defineProps<{ correlationId: string }>()
const emit = defineEmits<{ close: [] }>()

const api = useObservabilityApi()
const open = ref(true)
const trace = ref<ObsTrace | null>(null)
const loading = ref(false)
const error = ref('')

async function load() {
  loading.value = true
  error.value = ''
  trace.value = null
  try {
    trace.value = await api.get<ObsTrace>(`/trace/${encodeURIComponent(props.correlationId)}`)
  } catch (cause) {
    error.value = getApiErrorMessage(cause, 'Nao foi possivel carregar o trace.')
  } finally {
    loading.value = false
  }
}

watch(() => props.correlationId, load, { immediate: true })
watch(open, (value) => {
  if (!value) emit('close')
})

const request = computed(() => trace.value?.request || null)
const isEmpty = computed(
  () =>
    !trace.value ||
    (!request.value &&
      !trace.value.slowQueries?.length &&
      !trace.value.nPlusOne?.length &&
      !trace.value.externalCalls?.length &&
      !trace.value.frontendEvents?.length),
)

function field(row: Record<string, unknown>, key: string): string {
  const value = row?.[key]
  return value === null || value === undefined || value === '' ? '-' : String(value)
}
</script>

<template>
  <OmniEntityDrawer v-model="open" title="Trace" :subtitle="correlationId">
    <div class="obs-trace">
      <ObservabilityState
        :loading="loading"
        :error="error"
        :empty="isEmpty"
        empty-title="Nenhum dado para este correlation ID."
        empty-hint="O dado bruto expira conforme a retencao configurada (OBS_RETENTION_DAYS)."
        @retry="load"
      >
        <section v-if="request" class="obs-section">
          <h3 class="obs-section__title">1. Requisicao</h3>
          <dl class="obs-kv">
            <template
              v-for="key in [
                'method',
                'route',
                'status',
                'durationMs',
                'queryCount',
                'queryMs',
                'externalCount',
                'errorCode',
                'errorMessage',
              ]"
              :key="key"
            >
              <dt>{{ key }}</dt>
              <dd class="obs-mono">{{ field(request, key) }}</dd>
            </template>
          </dl>
        </section>

        <section v-if="trace?.slowQueries?.length || trace?.nPlusOne?.length" class="obs-section">
          <h3 class="obs-section__title">2. Consultas lentas e N+1</h3>
          <ul class="obs-list">
            <li v-for="(q, i) in trace?.slowQueries || []" :key="`s${i}`">
              <span class="obs-band obs-band--bad">{{ formatMs(q.durationMs) }}</span>
              <code class="obs-mono">{{ q.statement }}</code>
            </li>
            <li v-for="(q, i) in trace?.nPlusOne || []" :key="`n${i}`">
              <span class="obs-band obs-band--warn">{{ q.maxRepetitions }}x</span>
              <code class="obs-mono">{{ q.statement }}</code>
            </li>
          </ul>
        </section>

        <section v-if="trace?.externalCalls?.length" class="obs-section">
          <h3 class="obs-section__title">3. Chamadas externas</h3>
          <ul class="obs-list">
            <li v-for="(c, i) in trace.externalCalls" :key="i">
              <span class="obs-chip">{{ field(c, 'integration') }}</span>
              <code class="obs-mono">{{ field(c, 'method') }} {{ field(c, 'path') }}</code>
              <span class="obs-muted">
                status {{ field(c, 'status') }} · {{ formatMs(Number(c.durationMs)) }}
                <template v-if="c.timeout">· timeout</template>
              </span>
            </li>
          </ul>
        </section>

        <section v-if="trace?.frontendEvents?.length" class="obs-section">
          <h3 class="obs-section__title">4. Eventos do front</h3>
          <ul class="obs-list">
            <li v-for="(e, i) in trace.frontendEvents" :key="i">
              <span class="obs-chip">{{ field(e, 'kind') }}</span>
              <code class="obs-mono">{{ field(e, 'name') }}</code>
              <span class="obs-muted">{{ field(e, 'page') }} · valor {{ field(e, 'value') }}</span>
            </li>
          </ul>
        </section>
      </ObservabilityState>
    </div>
  </OmniEntityDrawer>
</template>
