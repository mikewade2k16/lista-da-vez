<script setup lang="ts">
// Modal "IA do mes" (SPEC-F5, contrato C4/C5). Seleciona 1+ clientes + mes, dispara
// POST /ai/plan e faz polling do resultado; aplica o plano nas notas (append) OU
// cria eventos (loop store.createEvent) e marca applied. Lista os planos anteriores
// do mes (index lean) com abrir/excluir. I/O em useCalendarAiPlans; render do
// resultado em CalendarAiPlanResult (mantem cada arquivo < 450 linhas).
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import CalendarAiPlanResult from '~/components/calendar/CalendarAiPlanResult.vue'
import { useCalendarAiPlans } from '~/composables/useCalendarAiPlans'
import { useCalendarStore } from '~/stores/calendar'
import { useUiStore } from '~/stores/ui'
import {
  AI_PROVIDER_LABEL,
  planContentToNotesHtml,
  planTypeToEventType,
  type CalendarAiPlanClient,
  type CalendarAiPlanDay,
  type CalendarAiPlanIndexItem,
  type CalendarAiPlanStatus,
  type CalendarEventInput,
} from '~/utils/calendar'

const props = defineProps<{ open: boolean; month: string }>()
const emit = defineEmits<{ close: [] }>()

const store = useCalendarStore()
const ui = useUiStore()
const { clients, config, activeNotesMonthKey } = storeToRefs(store)
const ai = useCalendarAiPlans()
const {
  plans,
  activePlan,
  starting,
  polling,
  fetchPlans,
  openPlan,
  startPlan,
  markApplied,
  removePlan,
  appendToMonthNotes,
  stopPolling,
} = ai

const monthKey = ref('')
const selectedIds = ref<string[]>([])
const applying = ref(false)
// Aviso acionavel quando o back devolve 503 ai_not_configured (envs/n8n faltando).
const notConfigured = ref(false)

const providerLabel = computed(() => AI_PROVIDER_LABEL[config.value.ai.provider] || 'IA')
const canGenerate = computed(
  () => selectedIds.value.length > 0 && !starting.value && !polling.value,
)
const statusLabels: Record<CalendarAiPlanStatus, string> = {
  pending: 'Gerando…',
  done: 'Pronto',
  error: 'Erro',
  applied: 'Aplicado',
}

// Reancora mes/selecao ao abrir e carrega os planos do mes; para o polling ao fechar.
watch(
  () => props.open,
  (open) => {
    if (!open) {
      stopPolling()
      return
    }
    monthKey.value = props.month || activeNotesMonthKey.value
    selectedIds.value = []
    notConfigured.value = false
    ai.setActivePlan(null)
    void fetchPlans(monthKey.value)
  },
  { immediate: true },
)

// Trocar o mes recarrega a lista de planos daquele mes.
watch(monthKey, (next) => {
  if (props.open) void fetchPlans(next)
})

function toggleClient(id: string): void {
  selectedIds.value = selectedIds.value.includes(id)
    ? selectedIds.value.filter((cid) => cid !== id)
    : [...selectedIds.value, id]
}

function selectAll(): void {
  selectedIds.value = clients.value.map((client) => client.id)
}

async function generate(): Promise<void> {
  notConfigured.value = false
  const res = await startPlan(monthKey.value, selectedIds.value)
  if (res.errorCode === 'ai_not_configured') {
    notConfigured.value = true
    return
  }
  if (res.errorCode) ui.error(res.errorMessage)
}

function goConfig(): void {
  emit('close')
  void navigateTo('/calendario/config')
}

// Anexa o plano na nota do mes-alvo. Mes ativo => store.setNotesForActiveMonth
// (fonte unica, sem refetch); outro mes => GET + append + PUT via composable.
async function applyNotes(): Promise<void> {
  const plan = activePlan.value
  if (!plan || applying.value) return
  applying.value = true
  const html = planContentToNotesHtml(plan.content)
  let ok = true
  if (plan.month === activeNotesMonthKey.value) {
    store.setNotesForActiveMonth(`${store.activeNotes || ''}${html}`)
  } else {
    ok = await appendToMonthNotes(plan.month, html)
  }
  applying.value = false
  if (ok) ui.success('Plano anexado nas notas do mês.')
  else ui.error('Não foi possível anexar nas notas.')
}

// Cria um evento por dia de cada cliente (title=idea, description=copy, type
// mapeado, planejado/media). Confirma antes de reaplicar um plano ja applied (evita
// duplicar em silencio). Ao final, marca o plano como applied.
async function createEvents(): Promise<void> {
  const plan = activePlan.value
  if (!plan || applying.value) return
  if (plan.status === 'applied') {
    const answer = await ui.confirm(
      'Este plano já foi aplicado. Criar os eventos de novo pode duplicá-los. Continuar?',
    )
    if (!answer?.confirmed) return
  }
  applying.value = true
  let created = 0
  let failed = 0
  for (const client of plan.content.clients) {
    for (const day of client.days) {
      if (!day.date || !day.idea.trim()) continue
      const ok = await store.createEvent(buildEventInput(client, day))
      if (ok) created += 1
      else failed += 1
    }
  }
  await markApplied(plan.id)
  applying.value = false
  if (created > 0) ui.success(`${created} evento(s) criado(s).`)
  if (failed > 0) ui.error(`${failed} evento(s) não puderam ser criados.`)
  if (created === 0 && failed === 0) ui.info('Nenhuma ideia com data para criar eventos.')
}

function buildEventInput(client: CalendarAiPlanClient, day: CalendarAiPlanDay): CalendarEventInput {
  return {
    date: day.date,
    time: '',
    clientId: client.clientId,
    type: planTypeToEventType(day.type),
    title: day.idea.trim(),
    status: 'planejado',
    priority: 'media',
    responsibleId: '',
    involvedIds: [],
    media: [],
    description: day.copy.trim(),
  }
}

async function onOpenPlan(id: string): Promise<void> {
  await openPlan(id)
}

async function onDeletePlan(item: CalendarAiPlanIndexItem): Promise<void> {
  const answer = await ui.confirm('Excluir este plano? A ação não pode ser desfeita.')
  if (!answer?.confirmed) return
  const ok = await removePlan(item.id)
  if (ok) ui.success('Plano excluído.')
  else ui.error('Não foi possível excluir o plano.')
}

function onKeydown(event: KeyboardEvent): void {
  if (event.key === 'Escape' && props.open) emit('close')
}

if (import.meta.client) document.addEventListener('keydown', onKeydown)
onBeforeUnmount(() => {
  if (import.meta.client) document.removeEventListener('keydown', onKeydown)
  stopPolling()
})
</script>

<template>
  <Teleport to="body">
    <div
      v-if="open"
      class="calendar-ai-overlay"
      role="dialog"
      aria-modal="true"
      aria-label="Plano de IA do mês"
      @click.self="emit('close')"
    >
      <div class="calendar-ai">
        <header class="calendar-ai__header">
          <strong class="calendar-ai__title">
            <UIcon name="i-lucide-sparkles" aria-hidden="true" />
            IA do mês
          </strong>
          <button
            type="button"
            class="calendar-ai__close"
            aria-label="Fechar"
            @click="emit('close')"
          >
            <UIcon name="i-lucide-x" aria-hidden="true" />
          </button>
        </header>

        <div class="calendar-ai__body">
          <p class="calendar-ai__provider">
            Provedor:
            <strong>{{ providerLabel }}</strong>
            <span v-if="config.ai.model">· {{ config.ai.model }}</span>
            <button type="button" class="calendar-ai__config-link" @click="goConfig">
              configurar
            </button>
          </p>

          <div class="calendar-ai__row">
            <label class="calendar-ai__field">
              <span class="calendar-ai__label">Mês</span>
              <input v-model="monthKey" type="month" class="calendar-ai__input" />
            </label>
          </div>

          <div class="calendar-ai__clients-head">
            <span class="calendar-ai__label">Clientes ({{ selectedIds.length }})</span>
            <button
              v-if="clients.length"
              type="button"
              class="calendar-ai__link"
              @click="selectAll"
            >
              Selecionar todos
            </button>
          </div>
          <p v-if="!clients.length" class="calendar-ai__empty">
            Nenhum cliente disponível para gerar o plano.
          </p>
          <div v-else class="calendar-ai__clients">
            <label v-for="client in clients" :key="client.id" class="calendar-ai__client">
              <input
                type="checkbox"
                :checked="selectedIds.includes(client.id)"
                @change="toggleClient(client.id)"
              />
              <span class="calendar-ai__client-name">{{ client.name }}</span>
            </label>
          </div>

          <p v-if="notConfigured" class="calendar-ai__warn">
            <UIcon name="i-lucide-shield-alert" aria-hidden="true" />
            A IA do calendário ainda não está configurada. Defina os envs
            <code>CALENDAR_AI_WEBHOOK_URL</code>
            ,
            <code>CALENDAR_AI_SERVICE_TOKEN</code>
            e
            <code>CALENDAR_AI_CALLBACK_BASE</code>
            e importe o workflow no n8n.
            <button type="button" class="calendar-ai__link" @click="goConfig">
              Ver configuração
            </button>
          </p>

          <p v-if="activePlan && activePlan.status === 'pending'" class="calendar-ai__pending">
            <UIcon name="i-lucide-loader-circle" class="calendar-ai__spin" aria-hidden="true" />
            Gerando o plano… isso pode levar alguns minutos.
          </p>
          <p v-else-if="activePlan && activePlan.status === 'error'" class="calendar-ai__warn">
            <UIcon name="i-lucide-alert-triangle" aria-hidden="true" />
            {{ activePlan.error || 'A IA não conseguiu gerar o plano. Tente novamente.' }}
          </p>

          <CalendarAiPlanResult
            v-if="activePlan && (activePlan.status === 'done' || activePlan.status === 'applied')"
            :plan="activePlan"
            :applying="applying"
            @apply-notes="applyNotes"
            @create-events="createEvents"
          />

          <section v-if="plans.length" class="calendar-ai__history">
            <h4 class="calendar-ai__history-title">Planos anteriores deste mês</h4>
            <ul class="calendar-ai__history-list">
              <li v-for="item in plans" :key="item.id" class="calendar-ai__history-item">
                <span class="calendar-ai__history-status" :data-status="item.status">
                  {{ statusLabels[item.status] }}
                </span>
                <span class="calendar-ai__history-meta">
                  {{ item.provider }} · {{ item.clientIds.length }} cliente(s)
                </span>
                <span class="calendar-ai__spacer"></span>
                <button type="button" class="calendar-ai__link" @click="onOpenPlan(item.id)">
                  Abrir
                </button>
                <button
                  type="button"
                  class="calendar-ai__link calendar-ai__link--danger"
                  @click="onDeletePlan(item)"
                >
                  Excluir
                </button>
              </li>
            </ul>
          </section>
        </div>

        <footer class="calendar-ai__footer">
          <span class="calendar-ai__spacer"></span>
          <AppPanelButton variant="ghost" @click="emit('close')">Fechar</AppPanelButton>
          <AppPanelButton variant="primary" :disabled="!canGenerate" @click="generate">
            <UIcon name="i-lucide-sparkles" aria-hidden="true" />
            Gerar plano
          </AppPanelButton>
        </footer>
      </div>
    </div>
  </Teleport>
</template>
