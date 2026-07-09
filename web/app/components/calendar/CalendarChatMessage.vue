<script setup lang="ts">
import { computed, ref } from 'vue'
import CalendarMediaViewer from '~/components/calendar/CalendarMediaViewer.vue'
import type { CalendarChatMessage } from '~/composables/useCalendarChat'
import type {
  CalendarChatStoredProposal,
  CalendarChatScopeClient,
} from '~/domain/calendar/calendar-chat-api'
import { getApiBase } from '~/utils/api-client'
import { resolveMediaUrl } from '~/utils/media'
import type { CalendarMediaItem } from '~/utils/calendar'
import { useCalendarStore } from '~/stores/calendar'

// Multi-tarefa (WAVE 5.1): a mensagem pode trazer VARIAS propostas de criacao (colapsaveis).
// Cliente (WAVE 5.2): escopo cliente => tudo criado ja vai para ELE (rotulo fixo); escopo
// "todos" => seletor de cliente por item + popup [Continuar sem cliente]/[Escolher cliente]
// (aplica um para todas) quando algum selecionado fica sem cliente. O clientId resolvido sobe
// no accept-selected e o composable cria com ele.
const props = defineProps<{
  message: CalendarChatMessage
  busy: boolean
  clients: CalendarChatScopeClient[]
  scopeMode: 'client' | 'all'
  scopeClientId: string
}>()
const emit = defineEmits<{
  'accept-selected': [messageId: string, items: { id: string; clientId: string }[]]
  'reject-selected': [messageId: string, proposalIds: string[]]
}>()

const apiBase = getApiBase(useRuntimeConfig())
const viewerItems = ref<CalendarMediaItem[]>([])
const viewerIndex = ref(0)

// Colapso do bloco (pode minimizar para nao ocupar a tela). Selecao: guardamos os ids
// DESMARCADOS (excluded) — toda pendente nasce marcada. clientOverride = cliente escolhido
// por item no escopo "todos". askClient/picking/pickId dirigem o popup de cliente faltante.
const collapsed = ref(false)
const excluded = ref<Set<string>>(new Set())
const clientOverride = ref<Map<string, string>>(new Map())
const askClient = ref(false)
const picking = ref(false)
const pickId = ref('')
const calendarStore = useCalendarStore()

const isAll = computed(() => props.scopeMode === 'all')
const pending = computed(() => props.message.proposals.filter((p) => p.status === 'pending'))
const created = computed(
  () => props.message.proposals.filter((p) => p.status === 'accepted').length,
)
const selectedIds = computed(() =>
  pending.value.filter((p) => !excluded.value.has(p.id)).map((p) => p.id),
)

function clientName(id: string): string {
  if (!id) return 'Sem cliente'
  return props.clients.find((c) => c.id === id)?.name || 'Cliente'
}
function targetClientId(p: CalendarChatStoredProposal): string {
  const targetId = String(p.fields.targetId || '')
  if (!targetId) return ''
  const existing = calendarStore.getEventById(targetId)
  if (existing?.clientId) return String(existing.clientId)
  return props.message.calendarItems.find((item) => item.id === targetId)?.clientId || ''
}
function targetTitle(p: CalendarChatStoredProposal): string {
  const targetId = String(p.fields.targetId || '')
  if (!targetId) return ''
  const existing = calendarStore.getEventById(targetId)
  if (existing?.title) return String(existing.title)
  return props.message.calendarItems.find((item) => item.id === targetId)?.title || ''
}
function proposalTitle(p: CalendarChatStoredProposal): string {
  return String(p.fields.title || '').trim() || targetTitle(p) || '(sem titulo)'
}
// Cliente resolvido de uma proposta: edicoes herdam o cliente atual do alvo quando a IA nao pediu
// troca de cliente. Assim o modo "todos" nao zera o cliente de um item ja vinculado.
function resolvedClientId(p: CalendarChatStoredProposal): string {
  if (!isAll.value) return props.scopeClientId
  if (clientOverride.value.has(p.id)) return clientOverride.value.get(p.id) || ''
  const proposedClientId = String(p.fields.clientId || '')
  if (proposedClientId) return proposedClientId
  if (p.action === 'update') return targetClientId(p)
  return ''
}
const selectedItems = computed(() =>
  selectedIds.value.map((id) => {
    const p = pending.value.find((x) => x.id === id)!
    return { id, clientId: resolvedClientId(p) }
  }),
)
// So conta "sem cliente" nas propostas que USAM cliente (create/update). Delete NAO precisa de
// cliente, entao nao dispara o popup de cliente faltante.
const missingCount = computed(
  () =>
    selectedIds.value.filter((id) => {
      const p = pending.value.find((x) => x.id === id)
      return !!p && p.action === 'create' && !resolvedClientId(p)
    }).length,
)

function isSelected(id: string): boolean {
  return !excluded.value.has(id)
}
function toggle(id: string): void {
  const next = new Set(excluded.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  excluded.value = next
}
function setItemClient(id: string, clientId: string): void {
  clientOverride.value = new Map(clientOverride.value).set(id, clientId)
}

function emitCreate(items: { id: string; clientId: string }[]): void {
  askClient.value = false
  picking.value = false
  emit('accept-selected', props.message.id, items)
}
// Escopo "todos" com cliente faltando em algum selecionado => abre o popup; senao cria direto.
function acceptSelected(): void {
  if (!selectedItems.value.length) return
  if (isAll.value && props.clients.length && missingCount.value > 0) {
    askClient.value = true
    picking.value = false
    pickId.value = ''
    return
  }
  emitCreate(selectedItems.value)
}
function continueWithout(): void {
  emitCreate(selectedItems.value)
}
// "Um para todas": aplica o cliente escolhido a todos os selecionados SEM cliente e cria.
function applyClientToAll(): void {
  if (!pickId.value) return
  const next = new Map(clientOverride.value)
  for (const item of selectedItems.value) {
    if (!item.clientId) next.set(item.id, pickId.value)
  }
  clientOverride.value = next
  emitCreate(
    selectedIds.value.map((id) => ({
      id,
      clientId: resolvedClientId(pending.value.find((x) => x.id === id)!),
    })),
  )
}

function rejectAll(): void {
  emit(
    'reject-selected',
    props.message.id,
    pending.value.map((p) => p.id),
  )
}
function rejectOne(id: string): void {
  emit('reject-selected', props.message.id, [id])
}

function kindLabel(p: CalendarChatStoredProposal): string {
  return p.kind === 'task' ? 'Tarefa' : 'Evento'
}
// Rotulo da acao (CRUD): create=Criar, update=Editar, delete=Excluir.
function actionLabel(p: CalendarChatStoredProposal): string {
  if (p.action === 'update') return 'Editar'
  if (p.action === 'delete') return 'Excluir'
  return 'Criar'
}
// Botao de lote: "Criar" quando tudo e create; "Aplicar" quando ha edicao/exclusao no meio.
const anyNonCreate = computed(() => pending.value.some((p) => p.action !== 'create'))
const confirmLabel = computed(() => (anyNonCreate.value ? 'Aplicar' : 'Criar'))
function proposalDate(p: CalendarChatStoredProposal): string {
  const f = p.fields || {}
  const date = f.date || f.dueDate || ''
  if (!date) return ''
  const label = dateLabel(date)
  return f.time ? `${label} · ${f.time}` : label
}
function dateLabel(value: string): string {
  const [year, month, day] = value.split('-').map(Number)
  if (!year || !month || !day) return value
  return new Intl.DateTimeFormat('pt-BR', { day: '2-digit', month: 'short' }).format(
    new Date(year, month - 1, day),
  )
}
function mediaUrl(url?: string): string {
  return resolveMediaUrl(url || '', apiBase)
}
function openMedia(items: CalendarMediaItem[], index: number): void {
  viewerItems.value = items
  viewerIndex.value = index
}
</script>

<template>
  <article class="calendar-chat__message" :class="`calendar-chat__message--${message.role}`">
    <div class="calendar-chat__msg" :class="`calendar-chat__msg--${message.role}`">
      {{ message.text }}
    </div>

    <section v-if="message.calendarItems.length" class="calendar-chat__results">
      <header class="calendar-chat__results-head">
        <span>
          <UIcon name="i-lucide-calendar-days" aria-hidden="true" />
          Calendário
        </span>
        <strong>
          {{ message.calendarItems.length }}
          {{ message.calendarItems.length === 1 ? 'item' : 'itens' }}
        </strong>
      </header>
      <div class="calendar-chat__results-list">
        <article v-for="item in message.calendarItems" :key="item.id" class="calendar-chat__result">
          <div v-if="item.media.length" class="calendar-chat__result-media">
            <button
              v-for="(media, mediaIndex) in item.media.slice(0, 3)"
              :key="media.id || media.url"
              type="button"
              class="calendar-chat__result-media-btn"
              :aria-label="`Abrir ${media.name || 'mídia'}`"
              @click="openMedia(item.media, mediaIndex)"
            >
              <img
                v-if="media.type === 'image' || media.posterUrl"
                :src="mediaUrl(media.type === 'video' ? media.posterUrl : media.url)"
                :alt="media.name || item.title"
                loading="lazy"
              />
              <span v-else class="calendar-chat__result-video" title="Vídeo">
                <UIcon name="i-lucide-play" aria-hidden="true" />
              </span>
            </button>
            <span v-if="item.media.length > 3" class="calendar-chat__result-more">
              +{{ item.media.length - 3 }}
            </span>
          </div>
          <div class="calendar-chat__result-body">
            <div class="calendar-chat__result-meta">
              <span>
                {{ dateLabel(item.date) }}
                <template v-if="item.time">· {{ item.time }}</template>
              </span>
              <span class="calendar-chat__result-status">{{ item.status }}</span>
            </div>
            <strong>{{ item.title }}</strong>
            <span v-if="item.clientName" class="calendar-chat__result-client">
              {{ item.clientName }}
            </span>
          </div>
        </article>
      </div>
    </section>

    <section v-if="message.proposals.length" class="calendar-chat__proposal">
      <button
        type="button"
        class="calendar-chat__proposal-head calendar-chat__proposal-toggle"
        :aria-expanded="!collapsed"
        @click="collapsed = !collapsed"
      >
        <UIcon
          :name="collapsed ? 'i-lucide-chevron-right' : 'i-lucide-chevron-down'"
          aria-hidden="true"
        />
        <UIcon name="i-lucide-list-checks" aria-hidden="true" />
        <strong>
          {{ message.proposals.length }}
          {{ message.proposals.length === 1 ? 'proposta' : 'propostas' }}
        </strong>
        <span class="calendar-chat__proposal-summary">
          <template v-if="pending.length">{{ pending.length }} pendente(s)</template>
          <template v-else-if="created">{{ created }} criada(s)</template>
        </span>
      </button>

      <template v-if="!collapsed">
        <p v-if="!isAll && scopeClientId" class="calendar-chat__proposal-scope">
          <UIcon name="i-lucide-user-round" aria-hidden="true" />
          Tudo será criado para
          <strong>{{ clientName(scopeClientId) }}</strong>
        </p>

        <ul class="calendar-chat__proposal-list">
          <li
            v-for="p in message.proposals"
            :key="p.id"
            class="calendar-chat__proposal-item"
            :class="{ 'is-resolved': p.status !== 'pending', 'is-delete': p.action === 'delete' }"
          >
            <label
              v-if="p.status === 'pending'"
              class="calendar-chat__proposal-check"
              :title="isSelected(p.id) ? 'Não criar este' : 'Criar este'"
            >
              <input
                type="checkbox"
                :checked="isSelected(p.id)"
                :disabled="busy"
                @change="toggle(p.id)"
              />
            </label>
            <span v-else class="calendar-chat__proposal-state" :class="`is-${p.status}`">
              {{ p.status === 'accepted' ? 'Criado' : 'Recusado' }}
            </span>

            <div class="calendar-chat__proposal-item-body">
              <strong>{{ proposalTitle(p) }}</strong>
              <span class="calendar-chat__proposal-item-meta">
                <span
                  v-if="p.action !== 'create'"
                  class="calendar-chat__proposal-action"
                  :class="`is-${p.action}`"
                >
                  {{ actionLabel(p) }}
                </span>
                <UIcon
                  :name="p.kind === 'task' ? 'i-lucide-square-check-big' : 'i-lucide-calendar-plus'"
                  aria-hidden="true"
                />
                {{ kindLabel(p) }}
                <template v-if="proposalDate(p)">· {{ proposalDate(p) }}</template>
              </span>
              <!-- Escopo "todos": seletor de cliente por item (editar uma a uma). -->
              <select
                v-if="isAll && p.status === 'pending' && clients.length && p.action !== 'delete'"
                class="calendar-chat__proposal-client"
                :value="resolvedClientId(p)"
                :disabled="busy"
                @change="setItemClient(p.id, ($event.target as HTMLSelectElement).value)"
              >
                <option value="">Sem cliente</option>
                <option v-for="c in clients" :key="c.id" :value="c.id">{{ c.name }}</option>
              </select>
            </div>

            <button
              v-if="p.status === 'pending'"
              type="button"
              class="calendar-chat__proposal-remove"
              :disabled="busy"
              :aria-label="`Recusar ${proposalTitle(p) || 'item'}`"
              title="Recusar"
              @click="rejectOne(p.id)"
            >
              <UIcon name="i-lucide-x" aria-hidden="true" />
            </button>
          </li>
        </ul>

        <!-- Popup de cliente faltante (escopo "todos"). -->
        <div v-if="askClient" class="calendar-chat__proposal-ask">
          <span>
            {{ missingCount }} {{ missingCount === 1 ? 'item sem cliente' : 'itens sem cliente' }}.
          </span>
          <template v-if="!picking">
            <button
              type="button"
              class="calendar-chat__proposal-dismiss"
              :disabled="busy"
              @click="continueWithout"
            >
              Continuar sem cliente
            </button>
            <button
              type="button"
              class="calendar-chat__proposal-confirm"
              :disabled="busy"
              @click="picking = true"
            >
              Escolher cliente
            </button>
          </template>
          <template v-else>
            <select v-model="pickId" class="calendar-chat__proposal-client" :disabled="busy">
              <option value="">Selecione…</option>
              <option v-for="c in clients" :key="c.id" :value="c.id">{{ c.name }}</option>
            </select>
            <button
              type="button"
              class="calendar-chat__proposal-confirm"
              :disabled="busy || !pickId"
              @click="applyClientToAll"
            >
              Aplicar a todas e criar
            </button>
          </template>
        </div>

        <div v-else-if="pending.length" class="calendar-chat__proposal-actions">
          <button
            type="button"
            class="calendar-chat__proposal-dismiss"
            :disabled="busy"
            @click="rejectAll"
          >
            Recusar todas
          </button>
          <button
            type="button"
            class="calendar-chat__proposal-confirm"
            :disabled="busy || !selectedIds.length"
            @click="acceptSelected"
          >
            <UIcon
              v-if="busy"
              name="i-lucide-loader-circle"
              class="calendar-chat__spin"
              aria-hidden="true"
            />
            <UIcon v-else name="i-lucide-check" aria-hidden="true" />
            {{ confirmLabel }} {{ selectedIds.length }} selecionada{{
              selectedIds.length === 1 ? '' : 's'
            }}
          </button>
        </div>
      </template>
    </section>

    <CalendarMediaViewer
      v-if="viewerItems.length"
      :items="viewerItems"
      :start-index="viewerIndex"
      @close="viewerItems = []"
    />
  </article>
</template>
