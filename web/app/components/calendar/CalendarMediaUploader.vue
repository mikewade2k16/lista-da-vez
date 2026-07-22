<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import {
  useCalendarMedia,
  type CalendarMediaUploadError,
  type CalendarMediaUploadPhase,
} from '~/composables/useCalendarMedia'
import CalendarMediaViewer from '~/components/calendar/CalendarMediaViewer.vue'
import { getApiBase } from '~/utils/api-client'
import { resolveMediaUrl } from '~/utils/media'
import { formatBytes, type CalendarMediaItem } from '~/utils/calendar'
// Select customizado do Tasks (mesmo componente do board) — dropdown com busca + largura de conteúdo.
import OmniSelectMenuInput from '../../../layers/tasks/components/inputs/OmniSelectMenuInput.vue'

const props = withDefaults(
  defineProps<{
    modelValue: CalendarMediaItem[]
    readonly?: boolean
    label?: string
    // Layout compacto e limitado usado exclusivamente na secao "Anexos do dia" do drawer.
    dayLayout?: boolean
    // WAVE 6 (W6-4): itens (eventos) do dia para vincular cada anexo. value=eventId,
    // label=titulo, clientId=cliente do evento (HERDADO pelo anexo). Vazio = sem seletor
    // (ex.: midia do evento, que ja e' do proprio evento).
    items?: { value: string; label: string; clientId?: string }[]
  }>(),
  { readonly: false, label: 'Anexos', dayLayout: false, items: () => [] },
)

// Mostra o seletor de item (evento) por anexo so quando ha itens do dia.
const showItemPicker = computed(() => !props.readonly && props.items.length > 0)

function itemLabel(eventId: string | undefined): string {
  if (!eventId) return ''
  return props.items.find((i) => i.value === eventId)?.label || ''
}

// Vincula (ou desvincula) o anexo a um evento/item do dia; o anexo HERDA o cliente do evento.
function setItemEvent(id: string, eventId: string): void {
  const chosen = props.items.find((i) => i.value === eventId)
  emit(
    'update:modelValue',
    props.modelValue.map((m) =>
      m.id === id ? { ...m, eventId, clientId: eventId ? chosen?.clientId || '' : m.clientId } : m,
    ),
  )
}

const emit = defineEmits<{ 'update:modelValue': [value: CalendarMediaItem[]] }>()

const { mediaLimits, fetchMediaLimits, uploadMedia, uploadVideoWithPoster } = useCalendarMedia()

// Indice do item aberto no viewer (null = fechado).
const viewerIndex = ref<number | null>(null)

function openViewer(idx: number): void {
  viewerIndex.value = idx
}

// Uploads em voo (ainda sem MediaItem): id local + nome + tipo + progresso.
interface Pending {
  key: number
  name: string
  kind: 'image' | 'video'
  pct: number
  phase: CalendarMediaUploadPhase | 'slow'
}
const pending = ref<Pending[]>([])
const error = ref('')
const input = ref<HTMLInputElement | null>(null)
let pendingSeq = 0
const slowTimers = new Map<number, ReturnType<typeof setTimeout>>()

const videoLimitLabel = computed(() => formatBytes(mediaLimits.value.videoMaxBytes))
const imageLimitLabel = computed(() => formatBytes(mediaLimits.value.imageMaxBytes))

// Absolutiza /uploads/* para a apiBase (dev: web :3003 e api :9091 separados;
// url relativa cairia no host do front e a thumb quebra).
const apiBase = getApiBase(useRuntimeConfig())
function srcOf(url: string): string {
  return resolveMediaUrl(url, apiBase)
}

// Mensagem acionavel por code do back (principio: nunca um "falhou" seco).
function uploadFailMessage(name: string, failure: CalendarMediaUploadError): string {
  switch (failure.code) {
    case 'invalid_media':
      return `${name}: ${failure.message || 'arquivo incompleto ou formato não aceito.'}`
    case 'media_too_large':
      return `${name}: acima do limite do servidor (imagem ${imageLimitLabel.value} · vídeo ${videoLimitLabel.value}).`
    case 'upload_timeout':
      return `${name}: a conexão parou de enviar dados antes de concluir o arquivo.`
    case 'upload_unavailable':
      return `${name}: o servidor não conseguiu iniciar o upload. Tente novamente em instantes.`
    case 'network':
      return `${name}: a conexão com a API foi interrompida durante o envio.`
    case 'timeout':
      return `${name}: a API não concluiu o upload dentro de 15 minutos.`
    case 'aborted':
      return `${name}: o upload foi cancelado.`
    case 'invalid_response':
      return `${name}: o servidor recebeu o arquivo, mas devolveu uma resposta inválida.`
    default:
      if (failure.status === 401)
        return `${name}: sua sessão expirou. Entre novamente e repita o envio.`
      if (failure.status === 403) return `${name}: sua conta não tem permissão para este upload.`
      if (failure.message) return `${name}: ${failure.message}`
      if (failure.status >= 500)
        return `${name}: a API encontrou um erro interno (HTTP ${failure.status}).`
      return `Falha ao enviar ${name}${failure.status ? ` (HTTP ${failure.status})` : ''}.`
  }
}

onMounted(() => void fetchMediaLimits())
onUnmounted(() => {
  for (const timer of slowTimers.values()) clearTimeout(timer)
  slowTimers.clear()
})

function clearSlowTimer(key: number): void {
  const timer = slowTimers.get(key)
  if (timer) clearTimeout(timer)
  slowTimers.delete(key)
}

function armSlowTimer(key: number): void {
  clearSlowTimer(key)
  slowTimers.set(
    key,
    setTimeout(() => {
      pending.value = pending.value.map((item) =>
        item.key === key && item.phase === 'uploading' ? { ...item, phase: 'slow' } : item,
      )
      slowTimers.delete(key)
    }, 10_000),
  )
}

function pendingLabel(item: Pending): string {
  switch (item.phase) {
    case 'slow':
      return 'Conexão lenta · ainda tentando'
    case 'processing':
      return 'Upload concluído · processando'
    case 'poster':
      return 'Gerando miniatura do vídeo'
    default:
      return 'Enviando arquivo'
  }
}

function pick(): void {
  error.value = ''
  input.value?.click()
}

function kindOf(file: File): 'image' | 'video' | '' {
  if (file.type.startsWith('image/')) return 'image'
  if (file.type.startsWith('video/')) return 'video'
  return ''
}

async function onFiles(event: Event): Promise<void> {
  const target = event.target as HTMLInputElement
  const files = Array.from(target.files || [])
  target.value = '' // permite re-selecionar o mesmo arquivo
  for (const file of files) {
    await handleFile(file)
  }
}

// Drop de ARQUIVOS externos (arrastar do PC para o uploader). Distinto do drag de REORDENACAO
// interno (abaixo): o drop de arquivo carrega dataTransfer com kind 'Files'; o reorder carrega
// 'text/plain'. `fileDragActive` acende a drop-zone. So habilitado quando editavel (!readonly).
const fileDragActive = ref(false)

function isExternalFileDrag(event: DragEvent): boolean {
  const types = event.dataTransfer?.types
  if (!types) return false
  return Array.from(types).includes('Files')
}

function onFileDragOver(event: DragEvent): void {
  if (props.readonly || !isExternalFileDrag(event)) return
  event.preventDefault()
  if (event.dataTransfer) event.dataTransfer.dropEffect = 'copy'
  fileDragActive.value = true
}

function onFileDragLeave(event: DragEvent): void {
  // So apaga quando sai DE VERDADE do contorno (evita piscar ao passar sobre filhos).
  if (event.currentTarget instanceof HTMLElement && event.relatedTarget instanceof Node) {
    if (event.currentTarget.contains(event.relatedTarget)) return
  }
  fileDragActive.value = false
}

async function onFileDrop(event: DragEvent): Promise<void> {
  if (props.readonly || !isExternalFileDrag(event)) return
  event.preventDefault()
  fileDragActive.value = false
  error.value = ''
  const files = Array.from(event.dataTransfer?.files || [])
  for (const file of files) {
    await handleFile(file)
  }
}

async function handleFile(file: File): Promise<void> {
  error.value = ''
  const kind = kindOf(file)
  if (!kind) {
    error.value = 'Tipo não suportado. Use imagem ou vídeo.'
    return
  }
  const limit = kind === 'video' ? mediaLimits.value.videoMaxBytes : mediaLimits.value.imageMaxBytes
  if (file.size > limit) {
    error.value = `${file.name} tem ${formatBytes(file.size)} — acima do limite de ${formatBytes(limit)}.`
    return
  }

  const key = ++pendingSeq
  pending.value = [...pending.value, { key, name: file.name, kind, pct: 0, phase: 'uploading' }]
  armSlowTimer(key)

  const onPct = (pct: number): void => {
    pending.value = pending.value.map((p) =>
      p.key === key ? { ...p, pct, phase: 'uploading' } : p,
    )
    armSlowTimer(key)
  }
  const onPhase = (phase: CalendarMediaUploadPhase): void => {
    if (phase === 'uploading') armSlowTimer(key)
    else clearSlowTimer(key)
    pending.value = pending.value.map((p) => (p.key === key ? { ...p, phase } : p))
  }
  let failure: CalendarMediaUploadError = { code: '', status: 0, message: '' }
  const onErr = (nextFailure: CalendarMediaUploadError): void => {
    failure = nextFailure
  }
  // Video sobe pelo fluxo que tambem gera+sobe o poster (falha do poster nao
  // falha o upload); imagem segue no upload simples.
  const item =
    kind === 'video'
      ? await uploadVideoWithPoster(file, onPct, onErr, onPhase)
      : await uploadMedia(file, onPct, onErr, onPhase)
  clearSlowTimer(key)
  pending.value = pending.value.filter((p) => p.key !== key)

  if (!item) {
    error.value = uploadFailMessage(file.name, failure)
    return
  }
  // O anexo entra sem item; o usuario escolhe o evento na barrinha da miniatura (W6-4).
  emit('update:modelValue', [...props.modelValue, item])
}

function remove(id: string): void {
  emit(
    'update:modelValue',
    props.modelValue.filter((m) => m.id !== id),
  )
}

// Drag-and-drop (WAVE 11): arrastar as miniaturas reordena as midias — a ORDEM define qual
// e' a 1a (a que aparece no fundo do dia). HTML5 nativo, sem lib; a nova ordem sobe pelo
// mesmo update:modelValue (persiste no save do evento/dia).
const dragIndex = ref<number | null>(null)
const dragOverIndex = ref<number | null>(null)
const canReorder = computed(() => !props.readonly && props.modelValue.length > 1)

function onDragStart(idx: number, event: DragEvent): void {
  if (!canReorder.value) return
  dragIndex.value = idx
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move'
    event.dataTransfer.setData('text/plain', String(idx))
  }
}

function onDragOver(idx: number, event: DragEvent): void {
  if (dragIndex.value === null) return
  event.preventDefault()
  dragOverIndex.value = idx
}

function onDrop(idx: number): void {
  const from = dragIndex.value
  dragIndex.value = null
  dragOverIndex.value = null
  if (from === null || from === idx) return
  const next = [...props.modelValue]
  const [moved] = next.splice(from, 1)
  if (!moved) return
  next.splice(idx, 0, moved)
  emit('update:modelValue', next)
}

function onDragEnd(): void {
  dragIndex.value = null
  dragOverIndex.value = null
}
</script>

<template>
  <div
    class="calendar-media"
    :class="{ 'calendar-media--day': dayLayout, 'calendar-media--dropping': fileDragActive }"
    @dragover="onFileDragOver"
    @dragleave="onFileDragLeave"
    @drop="onFileDrop"
  >
    <div v-if="fileDragActive" class="calendar-media__dropzone" aria-hidden="true">
      <UIcon name="i-lucide-upload-cloud" />
      <span>Solte para enviar</span>
    </div>
    <div v-if="dayLayout" class="calendar-media__head">
      <span v-if="label" class="calendar-media__label">{{ label }}</span>
      <button
        type="button"
        class="calendar-media__add-compact"
        aria-label="Adicionar anexo"
        @click="pick"
      >
        <UIcon name="i-lucide-plus" aria-hidden="true" />
        Adicionar
      </button>
    </div>
    <span v-else-if="label" class="calendar-media__label">{{ label }}</span>

    <div
      class="calendar-media__grid"
      :class="{ 'calendar-media__grid--scrollable': dayLayout && modelValue.length > 6 }"
    >
      <div
        v-for="(item, idx) in modelValue"
        :key="item.id"
        class="calendar-media__cell"
        :class="{
          'is-dragging': dragIndex === idx,
          'is-drag-over': dragOverIndex === idx && dragIndex !== idx,
        }"
        :draggable="canReorder"
        @dragstart="onDragStart(idx, $event)"
        @dragover="onDragOver(idx, $event)"
        @drop.prevent="onDrop(idx)"
        @dragend="onDragEnd"
      >
        <div
          class="calendar-media__item calendar-media__item--clickable"
          role="button"
          tabindex="0"
          :title="`${item.name} · ${formatBytes(item.sizeBytes)}${canReorder ? ' · arraste para reordenar (a 1ª aparece no dia)' : ''}`"
          :aria-label="`Abrir ${item.name}`"
          @click="openViewer(idx)"
          @keydown.enter.prevent="openViewer(idx)"
          @keydown.space.prevent="openViewer(idx)"
        >
          <img
            v-if="item.type === 'image'"
            :src="srcOf(item.url)"
            :alt="item.name"
            class="calendar-media__thumb"
          />
          <img
            v-else-if="item.posterUrl"
            :src="srcOf(item.posterUrl)"
            :alt="item.name"
            class="calendar-media__thumb"
          />
          <video
            v-else
            :src="srcOf(item.url)"
            class="calendar-media__thumb"
            preload="metadata"
            muted
          ></video>
          <span v-if="item.type === 'video'" class="calendar-media__badge">
            <UIcon name="i-lucide-play" aria-hidden="true" />
          </span>
          <button
            v-if="!readonly"
            type="button"
            class="calendar-media__remove"
            aria-label="Remover anexo"
            @click.stop="remove(item.id)"
          >
            <UIcon name="i-lucide-x" aria-hidden="true" />
          </button>
        </div>

        <!-- WAVE 6 (W6-4): a que ITEM/evento do dia o anexo pertence. Dropdown legivel ABAIXO da
             miniatura (antes era uma barrinha sobreposta e cortada). Escolher herda o cliente do evento. -->
        <OmniSelectMenuInput
          v-if="showItemPicker"
          class="calendar-media__pick"
          :model-value="item.eventId || null"
          :items="items"
          placeholder="Sem item"
          :searchable="items.length > 6"
          :full-content-width="true"
          item-display-mode="text"
          size="xs"
          color="neutral"
          variant="soft"
          :clear="true"
          @update:model-value="(v: unknown) => setItemEvent(item.id, v ? String(v) : '')"
        />
        <span
          v-else-if="item.eventId && itemLabel(item.eventId)"
          class="calendar-media__pick-tag"
          :title="itemLabel(item.eventId)"
        >
          {{ itemLabel(item.eventId) }}
        </span>
      </div>

      <div
        v-for="p in pending"
        :key="p.key"
        class="calendar-media__item calendar-media__item--loading"
        :class="{ 'is-slow': p.phase === 'slow' }"
        aria-live="polite"
      >
        <UIcon
          :name="p.kind === 'video' ? 'i-lucide-film' : 'i-lucide-image'"
          class="calendar-media__loading-icon"
          aria-hidden="true"
        />
        <span class="calendar-media__pct">{{ p.pct }}%</span>
        <span class="calendar-media__status">{{ pendingLabel(p) }}</span>
        <span class="calendar-media__bar">
          <span class="calendar-media__bar-fill" :style="{ width: `${p.pct}%` }"></span>
        </span>
      </div>

      <button
        v-if="!readonly && !dayLayout"
        type="button"
        class="calendar-media__add"
        aria-label="Adicionar anexo"
        @click="pick"
      >
        <UIcon name="i-lucide-plus" aria-hidden="true" />
      </button>

      <div
        v-if="readonly && !modelValue.length && !pending.length"
        class="calendar-media__item calendar-media__item--empty"
      ></div>
    </div>

    <p v-if="error" class="calendar-media__error" role="alert">{{ error }}</p>
    <p v-else-if="!readonly && dayLayout" class="calendar-media__hint">
      Imagem até {{ imageLimitLabel }} · vídeo até {{ videoLimitLabel }}
    </p>
    <p v-else-if="!readonly" class="calendar-media__hint">
      Imagem até {{ imageLimitLabel }} (jpg/png/webp/gif/avif) · vídeo até {{ videoLimitLabel }}
      (mp4/webm/mov).
    </p>

    <input
      ref="input"
      type="file"
      accept="image/*,video/*"
      multiple
      class="calendar-media__input"
      @change="onFiles"
    />

    <CalendarMediaViewer
      v-if="viewerIndex !== null && modelValue.length"
      :items="modelValue"
      :start-index="viewerIndex"
      @close="viewerIndex = null"
    />
  </div>
</template>

<!-- estilos do uploader (grid/cell/pick) ficam em ~/assets/styles/calendar/media.css (fonte unica) -->
