<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import CalendarMediaUploader from '~/components/calendar/CalendarMediaUploader.vue'
import { useCalendarStore } from '~/stores/calendar'
import {
  EVENT_TYPE_META,
  PRIORITY_META,
  STATUS_META,
  type CalendarClient,
  type CalendarEvent,
  type CalendarEventInput,
  type CalendarEventStatus,
  type CalendarEventType,
  type CalendarMediaItem,
  type CalendarPerson,
  type CalendarPriority,
} from '~/utils/calendar'

const props = withDefaults(
  defineProps<{
    open: boolean
    event: CalendarEvent | null
    defaultDate: string
    clients: CalendarClient[]
    people: CalendarPerson[]
    // Presenca (SPEC-F9): "Fulano editando" quando OUTRO usuario edita ESTE evento.
    editingLabel?: string
  }>(),
  { editingLabel: '' },
)

const emit = defineEmits<{
  submit: [input: CalendarEventInput]
  cancel: []
  remove: [id: string]
}>()

const title = ref('')
const clientId = ref('')
const date = ref('')
const time = ref('')
const type = ref('post')
const status = ref('planejado')
const priority = ref('media')
const responsibleId = ref('')
const description = ref('')
const media = ref<CalendarMediaItem[]>([])

const store = useCalendarStore()

// Integracao com Tasks (contrato C10 + WAVE 5): so vale na CRIACAO. O toggle vem LIGADO por
// padrao (decisao do dono 2026-07-05) — toda criacao de evento sugere criar a task; o usuario
// pode desligar. Se o board nao esta configurado (config tasks.boardId vazio), o toggle vira
// aviso acionavel com link para a aba Integracoes da config (drawer aberto por ?config=integracoes).
const createTask = ref(true)
const tasksBoardConfigured = computed(() => Boolean(store.config.tasks?.boardId))

function onToggleCreateTask(event: Event): void {
  createTask.value = (event.target as HTMLInputElement).checked
}

const isEdit = computed(() => Boolean(props.event))
const canSave = computed(() => title.value.trim() !== '' && date.value !== '')

const typeOptions = Object.entries(EVENT_TYPE_META).map(([value, meta]) => ({
  value,
  label: meta.label,
}))
const statusOptions = Object.entries(STATUS_META).map(([value, meta]) => ({
  value,
  label: meta.label,
}))
const priorityOptions = Object.entries(PRIORITY_META).map(([value, meta]) => ({
  value,
  label: meta.label,
}))

watch(
  () => [props.open, props.event, props.defaultDate] as const,
  () => {
    if (!props.open) return
    const e = props.event
    title.value = e?.title || ''
    clientId.value = e?.clientId || ''
    date.value = e?.date || props.defaultDate || ''
    time.value = e?.time || ''
    type.value = e?.type || 'post'
    status.value = e?.status || 'planejado'
    priority.value = e?.priority || 'media'
    responsibleId.value = e?.responsibleId || ''
    description.value = e?.description || ''
    media.value = e?.media ? [...e.media] : []
    // WAVE 5: o toggle de criar task volta LIGADO a cada abertura (default-on; so ao criar).
    createTask.value = true
  },
  { immediate: true },
)

function submit(): void {
  if (!canSave.value) return
  const payload: CalendarEventInput = {
    date: date.value,
    time: time.value.trim(),
    clientId: clientId.value,
    type: type.value as CalendarEventType,
    title: title.value.trim(),
    status: status.value as CalendarEventStatus,
    priority: priority.value as CalendarPriority,
    responsibleId: responsibleId.value,
    involvedIds: props.event?.involvedIds || [],
    media: media.value,
    description: description.value.trim(),
  }
  // createTask so vale ao CRIAR e com board configurado (C10); senao nem envia.
  if (!isEdit.value && tasksBoardConfigured.value && createTask.value) {
    payload.createTask = true
  }
  emit('submit', payload)
}

function remove(): void {
  if (props.event) emit('remove', props.event.id)
}

// Fecha no Esc (regra de modal/popover do design system).
function onKeydown(event: KeyboardEvent): void {
  if (event.key === 'Escape' && props.open) emit('cancel')
}

onMounted(() => document.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => document.removeEventListener('keydown', onKeydown))
</script>

<template>
  <div
    v-if="open"
    class="calendar-form-overlay"
    role="dialog"
    aria-modal="true"
    :aria-label="isEdit ? 'Editar item' : 'Novo item'"
    @click.self="emit('cancel')"
  >
    <div class="calendar-form">
      <header class="calendar-form__header">
        <strong class="calendar-form__title">{{ isEdit ? 'Editar item' : 'Novo item' }}</strong>
        <span v-if="isEdit && editingLabel" class="calendar-form__presence" :title="editingLabel">
          <UIcon name="i-lucide-pencil-line" aria-hidden="true" />
          {{ editingLabel }}
        </span>
        <button
          type="button"
          class="calendar-form__close"
          aria-label="Fechar"
          @click="emit('cancel')"
        >
          <UIcon name="i-lucide-x" aria-hidden="true" />
        </button>
      </header>

      <div class="calendar-form__body">
        <label class="calendar-form__field calendar-form__field--full">
          <span class="calendar-form__label">Título</span>
          <input
            v-model="title"
            class="calendar-form__input"
            placeholder="Ex.: Reels institucional"
            @keydown.enter.prevent="submit"
          />
        </label>

        <label class="calendar-form__field">
          <span class="calendar-form__label">Cliente</span>
          <select v-model="clientId" class="calendar-form__input">
            <option value="">Sem cliente</option>
            <option v-for="client in clients" :key="client.id" :value="client.id">
              {{ client.name }}
            </option>
          </select>
        </label>

        <label class="calendar-form__field">
          <span class="calendar-form__label">Responsável</span>
          <select v-model="responsibleId" class="calendar-form__input">
            <option value="">—</option>
            <option v-for="person in people" :key="person.id" :value="person.id">
              {{ person.name }}
            </option>
          </select>
        </label>

        <label class="calendar-form__field">
          <span class="calendar-form__label">Data</span>
          <input v-model="date" type="date" class="calendar-form__input" />
        </label>

        <label class="calendar-form__field">
          <span class="calendar-form__label">Horário</span>
          <input v-model="time" type="time" class="calendar-form__input" />
        </label>

        <label class="calendar-form__field">
          <span class="calendar-form__label">Tipo</span>
          <select v-model="type" class="calendar-form__input">
            <option v-for="opt in typeOptions" :key="opt.value" :value="opt.value">
              {{ opt.label }}
            </option>
          </select>
        </label>

        <label class="calendar-form__field">
          <span class="calendar-form__label">Status</span>
          <select v-model="status" class="calendar-form__input">
            <option v-for="opt in statusOptions" :key="opt.value" :value="opt.value">
              {{ opt.label }}
            </option>
          </select>
        </label>

        <label class="calendar-form__field">
          <span class="calendar-form__label">Prioridade</span>
          <select v-model="priority" class="calendar-form__input">
            <option v-for="opt in priorityOptions" :key="opt.value" :value="opt.value">
              {{ opt.label }}
            </option>
          </select>
        </label>

        <label class="calendar-form__field calendar-form__field--full">
          <span class="calendar-form__label">Descrição</span>
          <textarea
            v-model="description"
            class="calendar-form__input calendar-form__textarea"
            rows="3"
          ></textarea>
        </label>

        <div class="calendar-form__field calendar-form__field--full">
          <CalendarMediaUploader v-model="media" label="Anexos (imagem / vídeo)" />
        </div>

        <!-- Integracao com Tasks (C10): so ao criar. Board configurado -> toggle;
             sem board -> aviso acionavel que abre a config na aba Integracoes. -->
        <div v-if="!isEdit" class="calendar-form__field calendar-form__field--full">
          <label v-if="tasksBoardConfigured" class="calendar-form__toggle">
            <input
              type="checkbox"
              class="calendar-form__toggle-input"
              :checked="createTask"
              @change="onToggleCreateTask"
            />
            <span class="calendar-form__toggle-text">
              <span class="calendar-form__toggle-title">Criar task no board</span>
              <span class="calendar-form__toggle-hint">
                Gera uma task vinculada no board configurado em Integrações.
              </span>
            </span>
          </label>
          <p v-else class="calendar-form__task-warn">
            <UIcon name="i-lucide-alert-triangle" aria-hidden="true" />
            <span>
              Nenhum board configurado para criar tasks.
              <NuxtLink
                to="/calendario?config=integracoes"
                class="calendar-form__task-link"
                @click="emit('cancel')"
              >
                Configurar
              </NuxtLink>
            </span>
          </p>
        </div>
      </div>

      <footer class="calendar-form__footer">
        <AppPanelButton v-if="isEdit" variant="danger" @click="remove">Excluir</AppPanelButton>
        <span class="calendar-form__footer-spacer"></span>
        <AppPanelButton variant="ghost" @click="emit('cancel')">Cancelar</AppPanelButton>
        <AppPanelButton variant="primary" :disabled="!canSave" @click="submit">
          Salvar
        </AppPanelButton>
      </footer>
    </div>
  </div>
</template>
