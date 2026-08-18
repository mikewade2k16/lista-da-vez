<script setup lang="ts">
import { computed, ref } from 'vue'
import CalendarMediaViewer from '~/components/calendar/CalendarMediaViewer.vue'
import type { CalendarChatMessage } from '~/composables/useCalendarChat'
import type {
  CalendarChatProposalFields,
  CalendarChatStoredProposal,
  CalendarChatScopeClient,
} from '~/domain/calendar/calendar-chat-api'
import { getApiBase } from '~/utils/api-client'
import { resolveMediaUrl } from '~/utils/media'
import type { CalendarMediaItem } from '~/utils/calendar'
import {
  calendarProposalChanges,
  calendarProposalTargetClientId,
  calendarProposalTargetTitle,
} from '~/utils/calendar-chat-proposal-preview'
import { editableFields, getFieldByPath, setFieldByPath } from '~/utils/calendar-chat-proposal-edit'
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
  'accept-selected': [
    messageId: string,
    items: { id: string; clientId: string; fields?: CalendarChatProposalFields }[],
  ]
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
// Edit inline (WAVE 9): ids em edicao + rascunho editavel de cada proposta (clone dos fields; ao
// Aplicar usamos o rascunho). Alcance = ajustar os campos QUE a IA propos (nao adiciona campos novos).
const editingIds = ref<Set<string>>(new Set())
const edits = ref<Map<string, CalendarChatProposalFields>>(new Map())
const calendarStore = useCalendarStore()
const taskItemStatusOptions = [
  { value: 'captured', label: 'Gravado' },
  { value: 'editing', label: 'Em edição' },
  { value: 'approval', label: 'Em aprovação' },
  { value: 'approved', label: 'Aprovado' },
  { value: 'scheduled', label: 'Agendado' },
  { value: 'posted', label: 'Postado' },
]

const isAll = computed(() => props.scopeMode === 'all')
const pending = computed(() => props.message.proposals.filter((p) => p.status === 'pending'))
const accepted = computed(
  () => props.message.proposals.filter((p) => p.status === 'accepted').length,
)
// Guarda anti-mentira (rede de seguranca): se a IA falar como se JA tivesse feito (1a pessoa no
// passado: "adicionei", "criei"...) mas NAO houver nenhum cartao/proposta nem resultado, avisa que
// NADA foi aplicado. Nada e salvo sem o usuario aprovar um cartao — entao "fiz" sem cartao e mentira.
const FALSE_CLAIM_RE =
  /\b(adicionei|criei|editei|alterei|atualizei|salvei|apliquei|preenchi|removi|apaguei|exclu[ií]|cadastrei|inclu[ií]|registrei|deletei)\b/i
const falseClaim = computed(
  () =>
    props.message.role === 'assistant' &&
    !props.message.proposals.length &&
    !props.message.calendarItems.length &&
    FALSE_CLAIM_RE.test(props.message.text || ''),
)
const selectedPending = computed(() => pending.value.filter((p) => !excluded.value.has(p.id)))
const selectedIds = computed(() => selectedPending.value.map((p) => p.id))
const proposalTargetIds = computed(
  () =>
    new Set(
      props.message.proposals.map((p) => String(p.fields?.targetId || '').trim()).filter(Boolean),
    ),
)
const visibleCalendarItems = computed(() =>
  props.message.calendarItems.filter(
    (item) =>
      !proposalTargetIds.value.has(item.id) && !proposalTargetIds.value.has(item.taskId || ''),
  ),
)

function clientName(id: string): string {
  if (!id) return 'Sem cliente'
  return props.clients.find((c) => c.id === id)?.name || 'Cliente'
}
const previewContext = computed(() => ({
  clients: props.clients,
  calendarItems: props.message.calendarItems,
  people: calendarStore.people || [],
  getEventById: calendarStore.getEventById,
}))
function targetClientId(p: CalendarChatStoredProposal): string {
  return calendarProposalTargetClientId(p, previewContext.value)
}
function targetTitle(p: CalendarChatStoredProposal): string {
  return calendarProposalTargetTitle(p, previewContext.value)
}
function proposalTitle(p: CalendarChatStoredProposal): string {
  if (p.kind === 'clientProfile') {
    const cid = resolvedClientId(p)
    return cid ? `Perfil · ${clientName(cid)}` : 'Perfil do cliente'
  }
  if (p.kind === 'note') {
    const month = String(p.fields.note?.month || '').trim()
    return month ? `Anotações · ${month}` : 'Anotações do mês'
  }
  if (p.kind === 'taskItem') {
    const item = p.fields.taskItem || {}
    return String(item.itemTitle || item.title || '').trim() || '(item sem título)'
  }
  // Update/delete: o cabecalho mostra o item ALVO (titulo atual, resolvido do snapshot
  // server-side nos calendarItems) — e o que sera alterado; o titulo novo aparece no diff.
  if (p.action !== 'create') {
    return targetTitle(p) || String(p.fields.title || '').trim() || '(sem titulo)'
  }
  return String(p.fields.title || '').trim() || targetTitle(p) || '(sem titulo)'
}
function proposalTaskTitle(p: CalendarChatStoredProposal): string {
  return p.kind === 'taskItem' ? String(p.fields.taskItem?.taskTitle || '').trim() : ''
}
function proposalChanges(p: CalendarChatStoredProposal) {
  return calendarProposalChanges(p, previewContext.value)
}
// normName normaliza nome (minusculo, sem acento) para casar cliente por nome.
function normName(v: string): string {
  return String(v || '')
    .trim()
    .toLowerCase()
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
}
// resolveClientIdByName resolve o id do cliente a partir do NOME citado (ex.: "Pérola") contra os
// clientes visiveis. Match unico (exato ou prefixo) => id; ambiguo/nenhum => ''.
function resolveClientIdByName(name: string): string {
  const n = normName(name)
  if (!n) return ''
  const exact = props.clients.filter((c) => normName(c.name) === n)
  if (exact.length === 1) return exact[0]!.id
  const partial = props.clients.filter(
    (c) => normName(c.name).startsWith(n) || n.startsWith(normName(c.name)),
  )
  return partial.length === 1 ? partial[0]!.id : ''
}
// Cliente resolvido de uma proposta: usa o clientId da IA; senao resolve pelo NOME citado
// (clientName) contra os clientes visiveis; edicoes de evento/task herdam o cliente do alvo. No
// perfil do cliente (clientProfile) o cliente e a IDENTIDADE do perfil — resolve por nome/escopo.
function resolvedClientId(p: CalendarChatStoredProposal): string {
  if (!isAll.value) return props.scopeClientId
  if (clientOverride.value.has(p.id)) return clientOverride.value.get(p.id) || ''
  const proposedClientId = String(p.fields.clientId || '')
  if (proposedClientId) return proposedClientId
  const byName = resolveClientIdByName(String(p.fields.clientName || ''))
  if (byName) return byName
  if (p.action === 'update') return targetClientId(p)
  return ''
}
const selectedItems = computed(() =>
  selectedIds.value.map((id) => {
    const p = pending.value.find((x) => x.id === id)!
    return { id, clientId: resolvedClientId(p), fields: edits.value.get(id) }
  }),
)
// needsClient: propostas que EXIGEM um cliente resolvido. Evento/task so no create; perfil do
// cliente em qualquer acao (WAVE 7); anotacao nunca (e por mes, nao por cliente).
function needsClient(p: CalendarChatStoredProposal): boolean {
  if (p.kind === 'note' || p.kind === 'taskItem') return false
  if (p.kind === 'clientProfile') return true
  return p.action === 'create'
}
// showClientPicker: quando exibir o seletor de cliente no cartao (escopo "todos"). Perfil do cliente
// NAO deixa trocar o cliente (ele e a identidade do perfil): so aparece como RESGATE quando a IA nao
// resolveu o cliente; resolvido => sem seletor (o titulo ja mostra "Perfil de X"). Evento/task no
// create: cliente ja resolvido => rotulo fixo + botao "Trocar" (o select so abre se PEDIDO);
// sem cliente => select direto. Update/delete herdam o cliente do alvo (sem select).
const pickerRequested = ref<Set<string>>(new Set())
function requestPicker(id: string): void {
  pickerRequested.value = new Set(pickerRequested.value).add(id)
}
function showClientPicker(p: CalendarChatStoredProposal): boolean {
  if (!isAll.value || p.status !== 'pending' || !props.clients.length) return false
  if (p.kind === 'clientProfile') return !resolvedClientId(p)
  if (p.action !== 'create') return false
  return !resolvedClientId(p) || pickerRequested.value.has(p.id)
}
// showClientLabel: create com cliente ja resolvido (e select fechado) => mostra o cliente
// como rotulo com a opcao de trocar, em vez de abrir o select sem necessidade.
function showClientLabel(p: CalendarChatStoredProposal): boolean {
  if (!isAll.value || p.status !== 'pending' || !props.clients.length) return false
  if (p.kind !== 'event' && p.kind !== 'task') return false
  if (p.action !== 'create') return false
  return Boolean(resolvedClientId(p)) && !pickerRequested.value.has(p.id)
}
// --- Edit inline (WAVE 9): estado aqui, helpers puros no util calendar-chat-proposal-edit ---
function isEditing(id: string): boolean {
  return editingIds.value.has(id)
}
function startEdit(p: CalendarChatStoredProposal): void {
  const clone = JSON.parse(JSON.stringify(p.fields || {})) as CalendarChatProposalFields
  edits.value = new Map(edits.value).set(p.id, clone)
  editingIds.value = new Set(editingIds.value).add(p.id)
}
function cancelEdit(id: string): void {
  const nextEdits = new Map(edits.value)
  nextEdits.delete(id)
  edits.value = nextEdits
  editingIds.value = new Set([...editingIds.value].filter((x) => x !== id))
}
function getEdit(id: string, path: string): string {
  return getFieldByPath(edits.value.get(id), path)
}
function setEdit(id: string, path: string, value: unknown): void {
  setFieldByPath(edits.value.get(id), path, value)
  edits.value = new Map(edits.value)
}
function proposalEditFields(p: CalendarChatStoredProposal) {
  return editableFields(p)
}
// Anotacao: alterna Acrescentar <-> Reescrever no editor inline.
function toggleNoteMode(id: string): void {
  setEdit(id, 'note.mode', getEdit(id, 'note.mode') === 'replace' ? 'append' : 'replace')
}
function noteModeLabel(id: string): string {
  return getEdit(id, 'note.mode') === 'replace' ? 'Reescrever' : 'Acrescentar'
}
// So conta "sem cliente" nas propostas que USAM cliente. Delete de evento/task e anotacao nao
// disparam o popup de cliente faltante.
const missingCount = computed(
  () => selectedPending.value.filter((p) => needsClient(p) && !resolvedClientId(p)).length,
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
      fields: edits.value.get(id),
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
  if (p.kind === 'task') return 'Tarefa'
  if (p.kind === 'taskItem') return 'Item da tarefa'
  if (p.kind === 'note') return 'Anotação'
  if (p.kind === 'clientProfile') return 'Perfil'
  return 'Evento'
}
// Icone por kind da proposta.
function kindIcon(p: CalendarChatStoredProposal): string {
  if (p.kind === 'task') return 'i-lucide-square-check-big'
  if (p.kind === 'taskItem') return 'i-lucide-list-checks'
  if (p.kind === 'note') return 'i-lucide-notebook-pen'
  if (p.kind === 'clientProfile') return 'i-lucide-user-round-cog'
  return 'i-lucide-calendar-plus'
}
// showBefore: so evento/task em update mostram "antes -> depois" (temos o snapshot do alvo);
// perfil/anotacao mostram so o valor proposto (nao ha snapshot do estado atual na mensagem).
function showBefore(p: CalendarChatStoredProposal): boolean {
  return p.action === 'update' && (p.kind === 'event' || p.kind === 'task')
}
// Rotulo da acao (CRUD): create=Criar, update=Editar, delete=Excluir.
function actionLabel(p: CalendarChatStoredProposal): string {
  if (p.action === 'update') return 'Editar'
  if (p.action === 'delete') return 'Excluir'
  return 'Criar'
}
// Botao de lote: "Criar" quando tudo e create; "Aplicar" quando ha edicao/exclusao OU proposta
// de anotacao/perfil no lote (WAVE 7).
const anyNonCreate = computed(() =>
  pending.value.some(
    (p) => p.action !== 'create' || p.kind === 'note' || p.kind === 'clientProfile',
  ),
)
const confirmLabel = computed(() => (anyNonCreate.value ? 'Aplicar' : 'Criar'))
function resolvedStatusLabel(p: CalendarChatStoredProposal): string {
  if (p.status === 'rejected') return 'Recusado'
  if (p.action === 'delete') return 'Excluído'
  if (p.action === 'update') return 'Aplicado'
  return 'Criado'
}
function proposalDate(p: CalendarChatStoredProposal): string {
  const f = p.fields || {}
  if (p.kind === 'taskItem') {
    const item = f.taskItem || {}
    const value = item.statusDate || item.completedDate || ''
    return value ? dateLabel(value) : ''
  }
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

    <p v-if="falseClaim" class="calendar-chat__false-claim" role="note">
      <UIcon name="i-lucide-shield-alert" aria-hidden="true" />
      <span>
        A IA respondeu como se tivesse feito, mas
        <strong>nada foi aplicado</strong>
        — nenhuma alteração é salva sem você aprovar num cartão. Peça de novo para ela preparar a
        proposta.
      </span>
    </p>

    <section v-if="visibleCalendarItems.length" class="calendar-chat__results">
      <header class="calendar-chat__results-head">
        <span>
          <UIcon name="i-lucide-calendar-days" aria-hidden="true" />
          Calendário
        </span>
        <strong>
          {{ visibleCalendarItems.length }}
          {{ visibleCalendarItems.length === 1 ? 'item' : 'itens' }}
        </strong>
      </header>
      <div class="calendar-chat__results-list">
        <article v-for="item in visibleCalendarItems" :key="item.id" class="calendar-chat__result">
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
          <template v-else-if="accepted">{{ accepted }} aplicada(s)</template>
        </span>
      </button>

      <template v-if="!collapsed">
        <p
          v-if="!isAll && scopeClientId && pending.some((p) => needsClient(p))"
          class="calendar-chat__proposal-scope"
        >
          <UIcon name="i-lucide-user-round" aria-hidden="true" />
          Novos itens serão criados para
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
              :title="isSelected(p.id) ? 'Não aplicar esta' : 'Aplicar esta'"
            >
              <input
                type="checkbox"
                :checked="isSelected(p.id)"
                :disabled="busy"
                @change="toggle(p.id)"
              />
            </label>
            <span v-else class="calendar-chat__proposal-state" :class="`is-${p.status}`">
              {{ resolvedStatusLabel(p) }}
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
                <UIcon :name="kindIcon(p)" aria-hidden="true" />
                {{ kindLabel(p) }}
                <template v-if="proposalTaskTitle(p)">· {{ proposalTaskTitle(p) }}</template>
                <template v-if="proposalDate(p)">· {{ proposalDate(p) }}</template>
              </span>
              <dl
                v-if="!isEditing(p.id) && proposalChanges(p).length"
                class="calendar-chat__proposal-diff"
              >
                <div v-for="change in proposalChanges(p)" :key="change.key">
                  <dt>{{ change.label }}</dt>
                  <dd>
                    <template v-if="showBefore(p)">
                      <span class="calendar-chat__proposal-before">{{ change.before }}</span>
                      <UIcon name="i-lucide-arrow-right" aria-hidden="true" />
                    </template>
                    <span class="calendar-chat__proposal-after">{{ change.after }}</span>
                  </dd>
                </div>
              </dl>
              <p
                v-else-if="!isEditing(p.id) && showBefore(p)"
                class="calendar-chat__proposal-nochange"
              >
                Nenhuma mudança visível nessa proposta.
              </p>

              <!-- Edit inline (WAVE 9): ajusta os campos que a IA propôs antes de aprovar. -->
              <div v-if="isEditing(p.id)" class="calendar-chat__proposal-edit">
                <label
                  v-for="ef in proposalEditFields(p)"
                  :key="ef.key"
                  class="calendar-chat__proposal-edit-field"
                >
                  <span class="calendar-chat__proposal-edit-label">{{ ef.label }}</span>
                  <textarea
                    v-if="ef.kind === 'textarea'"
                    class="calendar-chat__proposal-edit-input"
                    :value="getEdit(p.id, ef.path)"
                    :disabled="busy"
                    rows="2"
                    @input="setEdit(p.id, ef.path, ($event.target as HTMLTextAreaElement).value)"
                  ></textarea>
                  <button
                    v-else-if="ef.kind === 'mode'"
                    type="button"
                    class="calendar-chat__proposal-edit-modebtn"
                    :disabled="busy"
                    @click="toggleNoteMode(p.id)"
                  >
                    {{ noteModeLabel(p.id) }}
                  </button>
                  <select
                    v-else-if="ef.kind === 'taskStatus'"
                    class="calendar-chat__proposal-edit-input"
                    :value="getEdit(p.id, ef.path)"
                    :disabled="busy"
                    @change="setEdit(p.id, ef.path, ($event.target as HTMLSelectElement).value)"
                  >
                    <option
                      v-for="option in taskItemStatusOptions"
                      :key="option.value"
                      :value="option.value"
                    >
                      {{ option.label }}
                    </option>
                  </select>
                  <input
                    v-else-if="ef.kind === 'boolean'"
                    type="checkbox"
                    :checked="getEdit(p.id, ef.path) === 'true'"
                    :disabled="busy"
                    @change="setEdit(p.id, ef.path, ($event.target as HTMLInputElement).checked)"
                  />
                  <input
                    v-else
                    class="calendar-chat__proposal-edit-input"
                    :type="ef.kind === 'date' ? 'date' : 'text'"
                    :value="getEdit(p.id, ef.path)"
                    :disabled="busy"
                    @input="setEdit(p.id, ef.path, ($event.target as HTMLInputElement).value)"
                  />
                </label>
              </div>

              <!-- Escopo "todos": cliente resolvido vira rotulo fixo + "Trocar" (select so se pedido). -->
              <p v-if="showClientLabel(p)" class="calendar-chat__proposal-client-label">
                <UIcon name="i-lucide-user-round" aria-hidden="true" />
                <span>{{ clientName(resolvedClientId(p)) }}</span>
                <button
                  type="button"
                  class="calendar-chat__proposal-client-swap"
                  :disabled="busy"
                  @click="requestPicker(p.id)"
                >
                  Trocar
                </button>
              </p>
              <select
                v-else-if="showClientPicker(p)"
                class="calendar-chat__proposal-client"
                :value="resolvedClientId(p)"
                :disabled="busy"
                @change="setItemClient(p.id, ($event.target as HTMLSelectElement).value)"
              >
                <option value="">Sem cliente</option>
                <option v-for="c in clients" :key="c.id" :value="c.id">{{ c.name }}</option>
              </select>

              <!-- Editar inline: abre/fecha os inputs dos campos propostos (WAVE 9). -->
              <button
                v-if="p.status === 'pending'"
                type="button"
                class="calendar-chat__proposal-edit-toggle"
                :disabled="busy"
                @click="isEditing(p.id) ? cancelEdit(p.id) : startEdit(p)"
              >
                <UIcon
                  :name="isEditing(p.id) ? 'i-lucide-x' : 'i-lucide-pencil'"
                  aria-hidden="true"
                />
                {{ isEditing(p.id) ? 'Cancelar edição' : 'Editar' }}
              </button>
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
