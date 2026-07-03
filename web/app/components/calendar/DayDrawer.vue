<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import CalendarMediaUploader from '~/components/calendar/CalendarMediaUploader.vue'
import { useCalendarStore } from '~/stores/calendar'
import {
  EVENT_TYPE_META,
  PRIORITY_META,
  STATUS_META,
  formatDayTitle,
  rgba,
  type CalendarClient,
  type CalendarEvent,
  type CalendarMediaItem,
  type CalendarPerson,
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

watch(
  () => [props.dateKey, props.events] as const,
  () => {
    activeEventId.value = props.events[0]?.id || ''
  },
  { immediate: true, deep: true },
)

async function onDayMedia(next: CalendarMediaItem[]): Promise<void> {
  if (props.dateKey) await store.saveDayMedia(props.dateKey, next)
}

const activeEvent = computed(
  () => props.events.find((event) => event.id === activeEventId.value) || props.events[0] || null,
)
const activeClient = computed(() =>
  activeEvent.value ? props.clientsById.get(activeEvent.value.clientId) : undefined,
)
const dayTitle = computed(() => (props.dateKey ? formatDayTitle(props.dateKey) : ''))

function toneClass(tone: StatusTone): string {
  return `calendar-pill--${tone}`
}

function clientOf(event: CalendarEvent): CalendarClient | undefined {
  return props.clientsById.get(event.clientId)
}

function markColor(event: CalendarEvent): RgbTriplet {
  return clientOf(event)?.color ?? FALLBACK_COLOR
}

function responsibleName(event: CalendarEvent): string {
  return props.peopleById.get(event.responsibleId)?.name || '—'
}

function initials(name: string): string {
  return (
    name
      .split(/\s+/)
      .filter(Boolean)
      .slice(0, 2)
      .map((part) => part.charAt(0).toUpperCase())
      .join('') || '?'
  )
}

function onEdit(): void {
  if (activeEvent.value) emit('edit', activeEvent.value)
}

function onRemove(): void {
  if (activeEvent.value) emit('remove', activeEvent.value.id)
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

    <div v-if="activeEvent" class="calendar-drawer__body">
      <!-- Detalhe do item -->
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
          <UIcon :name="EVENT_TYPE_META[activeEvent.type].icon" aria-hidden="true" />
          {{ EVENT_TYPE_META[activeEvent.type].label }}
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

      <dl class="calendar-drawer__fields">
        <div class="calendar-drawer__field">
          <dt class="calendar-drawer__field-label">
            <UIcon name="i-lucide-flag" aria-hidden="true" />
            Status
          </dt>
          <dd class="calendar-drawer__field-value">
            <span class="calendar-pill" :class="toneClass(STATUS_META[activeEvent.status].tone)">
              {{ STATUS_META[activeEvent.status].label }}
            </span>
          </dd>
        </div>

        <div class="calendar-drawer__field">
          <dt class="calendar-drawer__field-label">
            <UIcon name="i-lucide-signal-high" aria-hidden="true" />
            Prioridade
          </dt>
          <dd class="calendar-drawer__field-value">
            <span
              class="calendar-pill"
              :class="toneClass(PRIORITY_META[activeEvent.priority].tone)"
            >
              {{ PRIORITY_META[activeEvent.priority].label }}
            </span>
          </dd>
        </div>

        <div class="calendar-drawer__field">
          <dt class="calendar-drawer__field-label">
            <UIcon name="i-lucide-user" aria-hidden="true" />
            Responsável
          </dt>
          <dd class="calendar-drawer__field-value calendar-drawer__field-value--person">
            <span class="calendar-drawer__avatar">
              {{ initials(responsibleName(activeEvent)) }}
            </span>
            {{ responsibleName(activeEvent) }}
          </dd>
        </div>

        <div class="calendar-drawer__field">
          <dt class="calendar-drawer__field-label">
            <UIcon name="i-lucide-clock" aria-hidden="true" />
            Horário
          </dt>
          <dd class="calendar-drawer__field-value">{{ activeEvent.time || 'Dia inteiro' }}</dd>
        </div>
      </dl>

      <div class="calendar-drawer__media">
        <CalendarMediaUploader :model-value="activeEvent.media" readonly label="Mídia do post" />
      </div>

      <!-- Lista de todos os itens do dia -->
      <div class="calendar-drawer__list">
        <span class="calendar-drawer__list-label">Todos os itens do dia</span>
        <button
          v-for="event in events"
          :key="event.id"
          type="button"
          class="calendar-drawer__list-item"
          :class="{ 'is-active': event.id === activeEventId }"
          @click="activeEventId = event.id"
        >
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
            class="calendar-pill calendar-pill--sm"
            :class="toneClass(STATUS_META[event.status].tone)"
          >
            {{ STATUS_META[event.status].label }}
          </span>
        </button>
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
