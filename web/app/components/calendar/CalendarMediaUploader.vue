<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useCalendarMedia } from '~/composables/useCalendarMedia'
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
    // WAVE 6 (W6-4): itens (eventos) do dia para vincular cada anexo. value=eventId,
    // label=titulo, clientId=cliente do evento (HERDADO pelo anexo). Vazio = sem seletor
    // (ex.: midia do evento, que ja e' do proprio evento).
    items?: { value: string; label: string; clientId?: string }[]
  }>(),
  { readonly: false, label: 'Anexos', items: () => [] },
)

// Mostra o seletor de item (evento) por anexo so quando ha itens do dia.
const showItemPicker = computed(() => !props.readonly && props.items.length > 0)

// Largura MINIMA da coluna do grid (%). auto-fit + minmax(--min, 1fr) faz os itens ESTICAREM para
// preencher a linha inteira (imagens+selects maiores). Anexos do dia (com seletor): 45% => 2 por
// linha (o select cabe o nome do evento). Midia do post (read-only, compacto): 30% => ~3 por linha.
const gridMin = computed(() => (showItemPicker.value ? '45%' : '30%'))

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
}
const pending = ref<Pending[]>([])
const error = ref('')
const input = ref<HTMLInputElement | null>(null)
let pendingSeq = 0

const videoLimitLabel = computed(() => formatBytes(mediaLimits.value.videoMaxBytes))
const imageLimitLabel = computed(() => formatBytes(mediaLimits.value.imageMaxBytes))

// Absolutiza /uploads/* para a apiBase (dev: web :3003 e api :9091 separados;
// url relativa cairia no host do front e a thumb quebra).
const apiBase = getApiBase(useRuntimeConfig())
function srcOf(url: string): string {
  return resolveMediaUrl(url, apiBase)
}

// Mensagem acionavel por code do back (principio: nunca um "falhou" seco).
function uploadFailMessage(name: string, code: string, status: number): string {
  switch (code) {
    case 'invalid_media':
      return `${name}: formato não aceito. Imagens: jpg, png, webp, gif, avif · vídeos: mp4, webm, mov.`
    case 'media_too_large':
      return `${name}: acima do limite do servidor (imagem ${imageLimitLabel.value} · vídeo ${videoLimitLabel.value}).`
    case 'network':
      return `${name}: falha de rede ao enviar — a api não respondeu.`
    case 'timeout':
      return `${name}: o envio travou e estourou o tempo limite. Tente de novo; se repetir, recrie o container da api.`
    default:
      return `Falha ao enviar ${name}${status ? ` (HTTP ${status})` : ''}.`
  }
}

onMounted(() => void fetchMediaLimits())

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

async function handleFile(file: File): Promise<void> {
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
  pending.value = [...pending.value, { key, name: file.name, kind, pct: 0 }]

  const onPct = (pct: number): void => {
    pending.value = pending.value.map((p) => (p.key === key ? { ...p, pct } : p))
  }
  let failCode = ''
  let failStatus = 0
  const onErr = (code: string, status: number): void => {
    failCode = code
    failStatus = status
  }
  // Video sobe pelo fluxo que tambem gera+sobe o poster (falha do poster nao
  // falha o upload); imagem segue no upload simples.
  const item =
    kind === 'video'
      ? await uploadVideoWithPoster(file, onPct, onErr)
      : await uploadMedia(file, onPct, onErr)
  pending.value = pending.value.filter((p) => p.key !== key)

  if (!item) {
    error.value = uploadFailMessage(file.name, failCode, failStatus)
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
</script>

<template>
  <div class="calendar-media" :class="{ 'calendar-media--withpicker': showItemPicker }">
    <span v-if="label" class="calendar-media__label">{{ label }}</span>

    <div class="calendar-media__grid" :style="{ '--min': gridMin }">
      <div v-for="(item, idx) in modelValue" :key="item.id" class="calendar-media__cell">
        <div
          class="calendar-media__item calendar-media__item--clickable"
          role="button"
          tabindex="0"
          :title="`${item.name} · ${formatBytes(item.sizeBytes)}`"
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
      >
        <UIcon
          :name="p.kind === 'video' ? 'i-lucide-film' : 'i-lucide-image'"
          class="calendar-media__loading-icon"
          aria-hidden="true"
        />
        <span class="calendar-media__pct">{{ p.pct }}%</span>
        <span class="calendar-media__bar">
          <span class="calendar-media__bar-fill" :style="{ width: `${p.pct}%` }"></span>
        </span>
      </div>

      <button
        v-if="!readonly"
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

    <p v-if="error" class="calendar-media__error">{{ error }}</p>
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
