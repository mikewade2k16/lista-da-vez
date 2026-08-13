<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import {
  useCalendarMedia,
  type CalendarMediaUploadError,
  type CalendarMediaUploadPhase,
} from '~/composables/useCalendarMedia'
import OmniMediaGrid, { type OmniMediaGridItem } from '~/components/ui/OmniMediaGrid.vue'
import type { OmniMediaViewerItem } from '~/components/ui/OmniMediaViewer.vue'
import { reorderMediaItems } from '~/components/ui/media-grid/utils'
import { getApiBase } from '~/utils/api-client'
import { resolveMediaUrl } from '~/utils/media'
import { formatBytes, type CalendarMediaItem } from '~/utils/calendar'
import OmniSelectMenuInput from '../../../layers/tasks/components/inputs/OmniSelectMenuInput.vue'

const props = withDefaults(
  defineProps<{
    modelValue: CalendarMediaItem[]
    readonly?: boolean
    label?: string
    viewerItems?: CalendarMediaItem[]
    dayLayout?: boolean
    items?: { value: string; label: string; clientId?: string }[]
  }>(),
  { readonly: false, label: 'Anexos', viewerItems: () => [], dayLayout: false, items: () => [] },
)

const emit = defineEmits<{ 'update:modelValue': [value: CalendarMediaItem[]] }>()
const { mediaLimits, fetchMediaLimits, uploadMedia, uploadVideoWithPoster } = useCalendarMedia()
const pending = ref<Pending[]>([])
const error = ref('')
const apiBase = getApiBase(useRuntimeConfig())
let pendingSeq = 0
const slowTimers = new Map<number, ReturnType<typeof setTimeout>>()

interface Pending {
  key: number
  name: string
  kind: 'image' | 'video'
  pct: number
  phase: CalendarMediaUploadPhase | 'slow'
  previewUrl: string
}

const showItemPicker = computed(() => !props.readonly && props.items.length > 0)
const videoLimitLabel = computed(() => formatBytes(mediaLimits.value.videoMaxBytes))
const imageLimitLabel = computed(() => formatBytes(mediaLimits.value.imageMaxBytes))
const hint = computed(() =>
  props.readonly
    ? ''
    : props.dayLayout
      ? `Imagem até ${imageLimitLabel.value} · vídeo até ${videoLimitLabel.value}`
      : `Imagem até ${imageLimitLabel.value} (jpg/png/webp/gif/avif) · vídeo até ${videoLimitLabel.value} (mp4/webm/mov).`,
)
const gridItems = computed<OmniMediaGridItem[]>(() => [
  ...props.modelValue.map((item) => ({
    id: item.id,
    name: item.name,
    url: srcOf(item.url),
    type: item.type,
    posterUrl: item.posterUrl ? srcOf(item.posterUrl) : undefined,
    sizeLabel: formatBytes(item.sizeBytes),
    removable: !props.readonly,
  })),
  ...pending.value.map((item) => ({
    id: `pending:${item.key}`,
    name: item.name,
    url: item.previewUrl,
    type: item.kind,
    pending: true,
    progress: item.pct,
    status: pendingLabel(item),
    removable: false,
  })),
])
const gridViewerItems = computed<OmniMediaViewerItem[]>(() =>
  (props.viewerItems.length ? props.viewerItems : props.modelValue).map((item) => ({
    id: item.id,
    name: item.name,
    url: srcOf(item.url),
    type: item.type,
    posterUrl: item.posterUrl ? srcOf(item.posterUrl) : undefined,
    sizeLabel: formatBytes(item.sizeBytes),
  })),
)

function srcOf(url: string): string {
  return resolveMediaUrl(url, apiBase)
}

function calendarItemById(id: string): CalendarMediaItem | undefined {
  return props.modelValue.find((item) => item.id === id)
}

function itemLabel(eventId: string | undefined): string {
  if (!eventId) return ''
  return props.items.find((item) => item.value === eventId)?.label || ''
}

function setItemEvent(id: string, eventId: string): void {
  const chosen = props.items.find((item) => item.value === eventId)
  emit(
    'update:modelValue',
    props.modelValue.map((item) =>
      item.id === id
        ? { ...item, eventId, clientId: eventId ? chosen?.clientId || '' : item.clientId }
        : item,
    ),
  )
}

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
      return 'Arquivo recebido · preparando entrega segura'
    case 'poster':
      return 'Gerando miniatura do vídeo'
    default:
      return 'Preview disponível · enviando arquivo'
  }
}

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

function kindOf(file: File): 'image' | 'video' | '' {
  if (file.type.startsWith('image/')) return 'image'
  if (file.type.startsWith('video/')) return 'video'
  return ''
}

async function onGridFiles(files: File[]): Promise<void> {
  for (const file of files) await handleFile(file)
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
  const previewUrl = URL.createObjectURL(file)
  pending.value = [
    ...pending.value,
    { key, name: file.name, kind, pct: 0, phase: 'uploading', previewUrl },
  ]
  armSlowTimer(key)
  const onPct = (pct: number): void => {
    pending.value = pending.value.map((entry) =>
      entry.key === key ? { ...entry, pct, phase: 'uploading' } : entry,
    )
    armSlowTimer(key)
  }
  const onPhase = (phase: CalendarMediaUploadPhase): void => {
    if (phase === 'uploading') armSlowTimer(key)
    else clearSlowTimer(key)
    pending.value = pending.value.map((entry) => (entry.key === key ? { ...entry, phase } : entry))
  }
  let failure: CalendarMediaUploadError = { code: '', status: 0, message: '' }
  const onError = (nextFailure: CalendarMediaUploadError): void => {
    failure = nextFailure
  }
  const item =
    kind === 'video'
      ? await uploadVideoWithPoster(file, onPct, onError, onPhase)
      : await uploadMedia(file, onPct, onError, onPhase)

  clearSlowTimer(key)
  pending.value = pending.value.filter((entry) => entry.key !== key)
  URL.revokeObjectURL(previewUrl)
  if (!item) {
    error.value = uploadFailMessage(file.name, failure)
    return
  }
  emit('update:modelValue', [...props.modelValue, item])
}

function remove(id: string): void {
  emit(
    'update:modelValue',
    props.modelValue.filter((item) => item.id !== id),
  )
}

function reorder(payload: { from: number; to: number }): void {
  emit('update:modelValue', reorderMediaItems(props.modelValue, payload.from, payload.to))
}

onMounted(() => void fetchMediaLimits())
onUnmounted(() => {
  for (const timer of slowTimers.values()) clearTimeout(timer)
  slowTimers.clear()
  for (const item of pending.value) URL.revokeObjectURL(item.previewUrl)
})
</script>

<template>
  <OmniMediaGrid
    :items="gridItems"
    :viewer-items="gridViewerItems"
    :label="label"
    :hint="hint"
    :error="error"
    :readonly="readonly"
    :reorderable="!readonly && pending.length === 0"
    :variant="dayLayout ? 'day' : 'compact'"
    upload-label="Adicionar anexo"
    @files="onGridFiles"
    @remove="remove"
    @reorder="reorder"
  >
    <template #item-footer="{ item }">
      <template v-if="calendarItemById(item.id)">
        <OmniSelectMenuInput
          v-if="showItemPicker"
          :model-value="calendarItemById(item.id)?.eventId || null"
          :items="items"
          placeholder="Sem item"
          :searchable="items.length > 6"
          :full-content-width="true"
          item-display-mode="text"
          size="xs"
          color="neutral"
          variant="soft"
          :clear="true"
          @update:model-value="
            (value: unknown) => setItemEvent(item.id, value ? String(value) : '')
          "
        />
        <span
          v-else-if="
            calendarItemById(item.id)?.eventId && itemLabel(calendarItemById(item.id)?.eventId)
          "
          class="calendar-media__pick-tag"
        >
          {{ itemLabel(calendarItemById(item.id)?.eventId) }}
        </span>
      </template>
    </template>
  </OmniMediaGrid>
</template>

<style scoped>
.calendar-media__pick-tag {
  overflow: hidden;
  border-radius: var(--radius-sm);
  padding: 0.15rem 0.35rem;
  background: rgb(var(--primary) / 0.08);
  color: rgb(var(--primary));
  font-size: 0.66rem;
  font-weight: 600;
  text-align: center;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
