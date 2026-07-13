<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import CalendarMediaUploader from '~/components/calendar/CalendarMediaUploader.vue'
import { useCalendarStore } from '~/stores/calendar'
import {
  EVENT_TYPE_META,
  PRIORITY_META,
  STATUS_META,
  eventTypeMeta,
  statusMeta,
  formatDayTitle,
  rgba,
  type CalendarClient,
  type CalendarEvent,
  type CalendarEventInput,
  type CalendarEventStatus,
  type CalendarEventType,
  type CalendarMediaItem,
  type CalendarPerson,
  type CalendarPriority,
  type RgbTriplet,
  type StatusTone,
} from '~/utils/calendar'

const FALLBACK_COLOR: RgbTriplet = [148, 163, 184]

const props = defineProps<{
  dateKey: string
  events: CalendarEvent[]
  clientsById: Map<string, CalendarClient>
  peopleById: Map<string, CalendarPerson>
}>()

const emit = defineEmits<{
  close: []
  'new-item': []
  edit: [event: CalendarEvent]
  remove: [id: string]
}>()

const activeEventId = ref('')

// Anexos avulsos do dia: fonte unica no store (buscados na janela, sem refetch por
// dia). O uploader edita e o store persiste + atualiza o Map local.
const store = useCalendarStore()
const dayMedia = computed<CalendarMediaItem[]>(() => store.selectedDayMedia)

// Collapses SEMPRE fechados por padrao (so abrem no clique). Reseta ao TROCAR DE DIA apenas — NAO a
// cada mudanca de props.events: senao o refetch do realtime re-abriria/fecharia sozinho ("abrindo e
// abrindo"). Assim um item que o usuario abriu continua aberto quando os eventos atualizam no mesmo dia.
watch(
  () => props.dateKey,
  () => {
    activeEventId.value = ''
  },
  { immediate: true },
)

// WAVE 11 ("midias sao tarefas especiais"): anexo NOVO subido SEM item vira um ITEM ESPECIAL
// do calendario — um evento source='media' com titulo = nome do arquivo (a task vinculada nasce
// com esse titulo; o calendario mostra so a midia, sem titulo). Anexos vinculados a um evento
// existente e remocoes/edicoes seguem o fluxo normal do day_media.
async function onDayMedia(next: CalendarMediaItem[]): Promise<void> {
  if (!props.dateKey) return
  const known = new Set(dayMedia.value.map((m) => m.id))
  const fresh = next.filter((m) => !known.has(m.id) && !m.eventId)
  const rest = next.filter((m) => known.has(m.id) || m.eventId)
  for (const item of fresh) {
    const ok = await store.createEvent({
      date: props.dateKey,
      time: '',
      clientId: item.clientId || '',
      type: 'post',
      title: item.name || 'Mídia',
      status: 'planejado',
      priority: 'media',
      responsibleId: '',
      involvedIds: [],
      media: [item],
      description: '',
      mediaItem: true,
      createTask: Boolean(store.config.tasks?.boardId),
    } as CalendarEventInput)
    // Falha ao criar o item especial: preserva o anexo no day_media (nada se perde).
    if (!ok) rest.push(item)
  }
  await store.saveDayMedia(props.dateKey, rest)
}

const activeEvent = computed(
  () => props.events.find((event) => event.id === activeEventId.value) || props.events[0] || null,
)
const activeClient = computed(() =>
  activeEvent.value ? props.clientsById.get(activeEvent.value.clientId) : undefined,
)
const dayTitle = computed(() => (props.dateKey ? formatDayTitle(props.dateKey) : ''))

// WAVE 6 (W6-4, refino): "Mídia do post" do evento = midia do proprio evento (activeEvent.media)
// UNIDA aos anexos do dia que o usuario vinculou a este evento (dayMedia com eventId == id).
// Assim o anexo subido no "Anexos do dia" e apontado pro post aparece dentro do post tambem.
const activeEventMedia = computed<CalendarMediaItem[]>(() => {
  const ev = activeEvent.value
  if (!ev) return []
  const tagged = dayMedia.value.filter((m) => m.eventId === ev.id)
  // WAVE 6 cruzamento B: linkedMedia = videos da task vinculada, espelhados read-only.
  return [...ev.media, ...tagged, ...(ev.linkedMedia || [])]
})

// Qtd de anexos ligados a um evento (midia propria + anexos do dia apontados + midia da task) — indicador.
function eventAnexoCount(event: CalendarEvent): number {
  return (
    event.media.length +
    dayMedia.value.filter((m) => m.eventId === event.id).length +
    (event.linkedMedia?.length || 0)
  )
}

function toneClass(tone: StatusTone): string {
  return `calendar-pill--${tone}`
}

function clientOf(event: CalendarEvent): CalendarClient | undefined {
  return props.clientsById.get(event.clientId)
}

function markColor(event: CalendarEvent): RgbTriplet {
  return clientOf(event)?.color ?? FALLBACK_COLOR
}

function onEdit(): void {
  if (activeEvent.value) emit('edit', activeEvent.value)
}

function onRemove(): void {
  if (activeEvent.value) emit('remove', activeEvent.value.id)
}

// Accordion dos itens do dia (WAVE 6): clicar no header abre/fecha; um aberto por vez.
function toggleItem(id: string): void {
  activeEventId.value = activeEventId.value === id ? '' : id
}

// WAVE 6: criar a task para um evento SEM task (badge "sem task"). O store cria+vincula e refetcha.
const creatingTask = ref(false)
async function onCreateTask(): Promise<void> {
  const ev = activeEvent.value
  if (!ev || creatingTask.value) return
  creatingTask.value = true
  try {
    await store.createTaskForEvent(ev.id)
  } finally {
    creatingTask.value = false
  }
}

// ----- Edição INLINE (WAVE 6, W6-2) --------------------------------------------------
// Selects/inputs direto no painel do dia salvam o campo via store.updateEvent (full-replace
// + optimistic locking), sem abrir o modal. Options: status/prioridade/tipo dos META;
// responsável/cliente das Maps recebidas por prop (mesma fonte permission-scoped da página).
const statusOptions = Object.entries(STATUS_META).map(([value, meta]) => ({
  value,
  label: meta.label,
}))
const priorityOptions = Object.entries(PRIORITY_META).map(([value, meta]) => ({
  value,
  label: meta.label,
}))
const typeOptions = Object.entries(EVENT_TYPE_META).map(([value, meta]) => ({
  value,
  label: meta.label,
}))
const responsibleOptions = computed(() =>
  Array.from(props.peopleById.values()).map((person) => ({ value: person.id, label: person.name })),
)
const clientOptions = computed(() =>
  Array.from(props.clientsById.values()).map((client) => ({
    value: client.id,
    label: client.name,
  })),
)

// WAVE 6 (W6-4): itens (eventos/posts) do dia para vincular cada anexo. Escolher um evento faz
// o anexo herdar o cliente dele e ficar ligado ao item (e, por tabela, a task vinculada).
const dayItemOptions = computed(() =>
  props.events.map((event) => ({
    value: event.id,
    label: event.title || 'Item',
    clientId: event.clientId,
  })),
)

const savingInline = ref(false)

// updateField grava UM campo do evento ativo (merge sobre os campos atuais). Full-replace do
// contrato; version = optimistic locking (C12). Conflito/erro: o store ja refetcha (o painel
// re-renderiza com o estado fresco).
async function updateField(patch: Partial<CalendarEventInput>): Promise<void> {
  const ev = activeEvent.value
  if (!ev || savingInline.value) return
  savingInline.value = true
  try {
    const input: CalendarEventInput = {
      date: ev.date,
      time: ev.time,
      clientId: ev.clientId,
      type: ev.type,
      title: ev.title,
      status: ev.status,
      priority: ev.priority,
      responsibleId: ev.responsibleId,
      involvedIds: ev.involvedIds,
      media: ev.media,
      description: ev.description,
      ...patch,
    }
    await store.updateEvent(ev.id, input, ev.version)
  } finally {
    savingInline.value = false
  }
}

function onSelect(event: Event): string {
  return (event.target as HTMLSelectElement).value
}

// ----- Campos opcionais escondidos até serem adicionados (WAVE 6, W6-3) --------------
// Cliente/responsável/horário só aparecem se TÊM valor OU foram adicionados pelo botão
// "Adicionar campo". Status/prioridade/tipo têm default, então mostram sempre. A lista de
// "adicionados" zera ao trocar de evento (cada evento decide o que mostrar).
type DrawerField = 'client' | 'responsible' | 'time' | 'description'
const OPTIONAL_FIELDS: { key: DrawerField; label: string; icon: string }[] = [
  { key: 'client', label: 'Cliente', icon: 'i-lucide-circle-dot' },
  { key: 'responsible', label: 'Responsável', icon: 'i-lucide-user' },
  { key: 'time', label: 'Horário', icon: 'i-lucide-clock' },
  { key: 'description', label: 'Descrição', icon: 'i-lucide-align-left' },
]
const addedFields = ref<Set<DrawerField>>(new Set())

watch(
  () => activeEventId.value,
  () => {
    addedFields.value = new Set()
  },
)

function fieldHasValue(key: DrawerField): boolean {
  const ev = activeEvent.value
  if (!ev) return false
  if (key === 'client') return Boolean(ev.clientId)
  if (key === 'responsible') return Boolean(ev.responsibleId)
  if (key === 'description') return Boolean(ev.description)
  return Boolean(ev.time)
}

function isFieldVisible(key: DrawerField): boolean {
  return fieldHasValue(key) || addedFields.value.has(key)
}

const hiddenFields = computed(() => OPTIONAL_FIELDS.filter((field) => !isFieldVisible(field.key)))

function addField(key: DrawerField): void {
  addedFields.value = new Set(addedFields.value).add(key)
}
</script>

<template>
  <aside class="calendar-drawer" role="dialog" :aria-label="`Itens de ${dayTitle}`">
    <header class="calendar-drawer__header">
      <div class="calendar-drawer__heading">
        <strong class="calendar-drawer__title">{{ dayTitle }}</strong>
        <span class="calendar-drawer__count">{{ events.length }} itens agendados</span>
      </div>
      <button
        type="button"
        class="calendar-drawer__close"
        aria-label="Fechar"
        @click="emit('close')"
      >
        <UIcon name="i-lucide-x" aria-hidden="true" />
      </button>
    </header>

    <div v-if="events.length" class="calendar-drawer__body">
      <span class="calendar-drawer__list-label">Itens do dia</span>
      <!-- WAVE 6: cada item do dia e um COLLAPSE (accordion). Header clicavel; o corpo (detalhe +
           edicao inline) abre so no item ativo. Um aberto por vez (activeEventId). -->
      <div
        v-for="event in events"
        :key="event.id"
        class="calendar-drawer__acc"
        :class="{ 'is-open': event.id === activeEventId }"
      >
        <button
          type="button"
          class="calendar-drawer__acc-head"
          :aria-expanded="event.id === activeEventId"
          @click="toggleItem(event.id)"
        >
          <UIcon
            class="calendar-drawer__acc-chevron"
            :name="event.id === activeEventId ? 'i-lucide-chevron-down' : 'i-lucide-chevron-right'"
            aria-hidden="true"
          />
          <span
            class="calendar-drawer__list-mark"
            :style="{ background: rgba(markColor(event), 1) }"
            aria-hidden="true"
          ></span>
          <span class="calendar-drawer__list-text">
            <span class="calendar-drawer__list-title">{{ event.title }}</span>
            <span class="calendar-drawer__list-meta">
              {{ clientOf(event)?.name || 'Cliente' }} · {{ event.time || 'Dia inteiro' }}
            </span>
          </span>
          <span
            v-if="eventAnexoCount(event)"
            class="calendar-drawer__list-anexo"
            :title="`${eventAnexoCount(event)} anexo(s)`"
          >
            <UIcon name="i-lucide-paperclip" aria-hidden="true" />
            {{ eventAnexoCount(event) }}
          </span>
          <span
            class="calendar-pill calendar-pill--sm"
            :class="toneClass(statusMeta(event.status).tone)"
          >
            {{ statusMeta(event.status).label }}
          </span>
        </button>

        <div v-if="event.id === activeEventId && activeEvent" class="calendar-drawer__acc-body">
          <!-- Detalhe do item ativo (edicao inline) -->
          <div class="calendar-drawer__pills">
            <span
              v-if="activeClient"
              class="calendar-drawer__pill"
              :style="{
                background: rgba(activeClient.color, 0.16),
                color: rgba(activeClient.color, 1),
              }"
            >
              <span
                class="calendar-drawer__pill-dot"
                :style="{ background: rgba(activeClient.color, 1) }"
              ></span>
              {{ activeClient.name }}
            </span>
            <span class="calendar-drawer__pill calendar-drawer__pill--type">
              <UIcon :name="eventTypeMeta(activeEvent.type).icon" aria-hidden="true" />
              {{ eventTypeMeta(activeEvent.type).label }}
            </span>
          </div>

          <h3 class="calendar-drawer__event-title">{{ activeEvent.title }}</h3>

          <div class="calendar-drawer__actions">
            <button type="button" class="calendar-drawer__action" @click="onEdit">
              <UIcon name="i-lucide-pencil" aria-hidden="true" />
              Editar
            </button>
            <button
              type="button"
              class="calendar-drawer__action calendar-drawer__action--danger"
              @click="onRemove"
            >
              <UIcon name="i-lucide-trash-2" aria-hidden="true" />
              Excluir
            </button>
          </div>

          <!-- Vinculo com Tasks (contrato C10 + WAVE 5, E6): deep-link para o CARD especifico no
           board (o /tasks abre o editor da task pelo ?task=). board = board da config. -->
          <NuxtLink
            v-if="activeEvent.taskId"
            :to="{
              path: '/tasks',
              query: { board: store.config.tasks?.boardId || undefined, task: activeEvent.taskId },
            }"
            class="calendar-drawer__tasklink"
          >
            <UIcon name="i-lucide-link-2" aria-hidden="true" />
            Abrir task vinculada
            <UIcon name="i-lucide-external-link" aria-hidden="true" />
          </NuxtLink>

          <!-- WAVE 6: evento SEM task vinculada — avisa (badge) e deixa criar a task manualmente. -->
          <div v-else class="calendar-drawer__notask">
            <span class="calendar-drawer__notask-badge" title="Este item não tem task no board">
              <UIcon name="i-lucide-unlink" aria-hidden="true" />
              Sem task
            </span>
            <button
              type="button"
              class="calendar-drawer__notask-btn"
              :disabled="creatingTask"
              @click="onCreateTask"
            >
              <UIcon
                :name="creatingTask ? 'i-lucide-loader-circle' : 'i-lucide-plus'"
                :class="{ 'animate-spin': creatingTask }"
                aria-hidden="true"
              />
              Criar task
            </button>
          </div>

          <!-- Campos INLINE (W6-2): editam direto aqui, sem abrir o modal. -->
          <dl class="calendar-drawer__fields">
            <div class="calendar-drawer__field">
              <dt class="calendar-drawer__field-label">
                <UIcon name="i-lucide-flag" aria-hidden="true" />
                Status
              </dt>
              <dd class="calendar-drawer__field-value">
                <select
                  class="calendar-drawer__inline"
                  :value="activeEvent.status"
                  :disabled="savingInline"
                  aria-label="Status"
                  @change="updateField({ status: onSelect($event) as CalendarEventStatus })"
                >
                  <option v-for="o in statusOptions" :key="o.value" :value="o.value">
                    {{ o.label }}
                  </option>
                </select>
              </dd>
            </div>

            <div class="calendar-drawer__field">
              <dt class="calendar-drawer__field-label">
                <UIcon name="i-lucide-signal-high" aria-hidden="true" />
                Prioridade
              </dt>
              <dd class="calendar-drawer__field-value">
                <select
                  class="calendar-drawer__inline"
                  :value="activeEvent.priority"
                  :disabled="savingInline"
                  aria-label="Prioridade"
                  @change="updateField({ priority: onSelect($event) as CalendarPriority })"
                >
                  <option v-for="o in priorityOptions" :key="o.value" :value="o.value">
                    {{ o.label }}
                  </option>
                </select>
              </dd>
            </div>

            <div class="calendar-drawer__field">
              <dt class="calendar-drawer__field-label">
                <UIcon name="i-lucide-clapperboard" aria-hidden="true" />
                Tipo
              </dt>
              <dd class="calendar-drawer__field-value">
                <select
                  class="calendar-drawer__inline"
                  :value="activeEvent.type"
                  :disabled="savingInline"
                  aria-label="Tipo"
                  @change="updateField({ type: onSelect($event) as CalendarEventType })"
                >
                  <option v-for="o in typeOptions" :key="o.value" :value="o.value">
                    {{ o.label }}
                  </option>
                </select>
              </dd>
            </div>

            <div v-if="isFieldVisible('client')" class="calendar-drawer__field">
              <dt class="calendar-drawer__field-label">
                <UIcon name="i-lucide-circle-dot" aria-hidden="true" />
                Cliente
              </dt>
              <dd class="calendar-drawer__field-value">
                <select
                  class="calendar-drawer__inline"
                  :value="activeEvent.clientId"
                  :disabled="savingInline"
                  aria-label="Cliente"
                  @change="updateField({ clientId: onSelect($event) })"
                >
                  <option value="">Sem cliente</option>
                  <option v-for="o in clientOptions" :key="o.value" :value="o.value">
                    {{ o.label }}
                  </option>
                </select>
              </dd>
            </div>

            <div v-if="isFieldVisible('responsible')" class="calendar-drawer__field">
              <dt class="calendar-drawer__field-label">
                <UIcon name="i-lucide-user" aria-hidden="true" />
                Responsável
              </dt>
              <dd class="calendar-drawer__field-value">
                <select
                  class="calendar-drawer__inline"
                  :value="activeEvent.responsibleId"
                  :disabled="savingInline"
                  aria-label="Responsável"
                  @change="updateField({ responsibleId: onSelect($event) })"
                >
                  <option value="">—</option>
                  <option v-for="o in responsibleOptions" :key="o.value" :value="o.value">
                    {{ o.label }}
                  </option>
                </select>
              </dd>
            </div>

            <div v-if="isFieldVisible('time')" class="calendar-drawer__field">
              <dt class="calendar-drawer__field-label">
                <UIcon name="i-lucide-clock" aria-hidden="true" />
                Horário
              </dt>
              <dd class="calendar-drawer__field-value">
                <input
                  type="time"
                  class="calendar-drawer__inline"
                  :value="activeEvent.time"
                  :disabled="savingInline"
                  aria-label="Horário"
                  @change="updateField({ time: onSelect($event) })"
                />
              </dd>
            </div>

            <!-- Descrição (textarea simples; salva no blur via @change, como os demais inline). -->
            <div
              v-if="isFieldVisible('description')"
              class="calendar-drawer__field calendar-drawer__field--full"
            >
              <dt class="calendar-drawer__field-label">
                <UIcon name="i-lucide-align-left" aria-hidden="true" />
                Descrição
              </dt>
              <dd class="calendar-drawer__field-value">
                <textarea
                  class="calendar-drawer__inline calendar-drawer__inline--textarea"
                  :value="activeEvent.description"
                  :disabled="savingInline"
                  rows="3"
                  aria-label="Descrição"
                  placeholder="Anotações, roteiro, referências…"
                  @change="
                    updateField({ description: ($event.target as HTMLTextAreaElement).value })
                  "
                ></textarea>
              </dd>
            </div>
          </dl>

          <!-- Adicionar campo (W6-3): revela os campos opcionais que estao vazios. -->
          <div v-if="hiddenFields.length" class="calendar-drawer__addfields">
            <button
              v-for="field in hiddenFields"
              :key="field.key"
              type="button"
              class="calendar-drawer__addfield"
              @click="addField(field.key)"
            >
              <UIcon name="i-lucide-plus" aria-hidden="true" />
              {{ field.label }}
            </button>
          </div>

          <div class="calendar-drawer__media">
            <CalendarMediaUploader :model-value="activeEventMedia" readonly label="Mídia do post" />
          </div>
        </div>
      </div>
    </div>

    <div v-else class="calendar-drawer__empty">
      <UIcon name="i-lucide-calendar-x" class="calendar-drawer__empty-icon" aria-hidden="true" />
      <p>Nenhum item agendado neste dia.</p>
    </div>

    <div class="calendar-drawer__daymedia">
      <CalendarMediaUploader
        :model-value="dayMedia"
        label="Anexos do dia"
        day-layout
        :items="dayItemOptions"
        @update:model-value="onDayMedia"
      />
    </div>

    <footer class="calendar-drawer__footer">
      <AppPanelButton variant="primary" block @click="emit('new-item')">
        <UIcon name="i-lucide-plus" aria-hidden="true" />
        Novo item neste dia
      </AppPanelButton>
    </footer>
  </aside>
</template>

<style scoped>
/* Controle de edição inline no painel do dia (W6-2): parece texto, vira campo no hover/foco. */
.calendar-drawer__inline {
  width: 100%;
  min-height: 1.9rem;
  max-width: 100%;
  padding: 0.2rem 0.4rem;
  border: 1px solid transparent;
  border-radius: var(--radius-sm, 8px);
  background: transparent;
  color: rgb(var(--text));
  font: inherit;
  font-size: 0.84rem;
  font-weight: 600;
  cursor: pointer;
  transition:
    background 0.15s ease,
    border-color 0.15s ease;
}

.calendar-drawer__inline:hover:not(:disabled),
.calendar-drawer__inline:focus-visible {
  background: rgb(var(--surface-2));
  border-color: rgb(var(--border));
  outline: none;
}

.calendar-drawer__inline:disabled {
  opacity: 0.6;
  cursor: progress;
}

.calendar-drawer__inline option {
  color: rgb(var(--text));
  background: rgb(var(--surface));
}

/* Descrição (D): textarea multilinha ocupando a linha inteira (label em cima, campo largo). */
.calendar-drawer__inline--textarea {
  min-height: 4.5rem;
  resize: vertical;
  line-height: 1.4;
}

.calendar-drawer__field--full {
  grid-template-columns: 1fr;
  align-items: stretch;
}

/* Botões "Adicionar campo" (W6-3): revelam os campos opcionais vazios. */
.calendar-drawer__addfields {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
  margin-top: 0.5rem;
}

.calendar-drawer__addfield {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  padding: 0.2rem 0.55rem;
  border: 1px dashed rgb(var(--border));
  border-radius: 999px;
  background: transparent;
  color: rgb(var(--muted));
  font-size: 0.76rem;
  font-weight: 600;
  cursor: pointer;
  transition:
    color 0.15s ease,
    border-color 0.15s ease,
    background 0.15s ease;
}

.calendar-drawer__addfield:hover {
  color: rgb(var(--primary));
  border-color: rgb(var(--primary) / 0.5);
  background: rgb(var(--primary) / 0.08);
}
</style>
