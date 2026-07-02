<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useCalendarMedia } from '~/composables/useCalendarMedia'
import { formatBytes, type CalendarMediaItem } from '~/utils/calendar'

const props = withDefaults(
  defineProps<{
    modelValue: CalendarMediaItem[]
    readonly?: boolean
    label?: string
  }>(),
  { readonly: false, label: 'Anexos' },
)

const emit = defineEmits<{ 'update:modelValue': [value: CalendarMediaItem[]] }>()

const { mediaLimits, fetchMediaLimits, uploadMedia } = useCalendarMedia()

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

  const item = await uploadMedia(file, (pct) => {
    pending.value = pending.value.map((p) => (p.key === key ? { ...p, pct } : p))
  })
  pending.value = pending.value.filter((p) => p.key !== key)

  if (!item) {
    error.value = `Falha ao enviar ${file.name}.`
    return
  }
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
  <div class="calendar-media">
    <span v-if="label" class="calendar-media__label">{{ label }}</span>

    <div class="calendar-media__grid">
      <div
        v-for="item in modelValue"
        :key="item.id"
        class="calendar-media__item"
        :title="`${item.name} · ${formatBytes(item.sizeBytes)}`"
      >
        <img
          v-if="item.type === 'image'"
          :src="item.url"
          :alt="item.name"
          class="calendar-media__thumb"
        />
        <video
          v-else
          :src="item.url"
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
          @click="remove(item.id)"
        >
          <UIcon name="i-lucide-x" aria-hidden="true" />
        </button>
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
      Imagem até {{ imageLimitLabel }} · vídeo até {{ videoLimitLabel }} (mp4/webm/mov).
    </p>

    <input
      ref="input"
      type="file"
      accept="image/*,video/*"
      multiple
      class="calendar-media__input"
      @change="onFiles"
    />
  </div>
</template>
