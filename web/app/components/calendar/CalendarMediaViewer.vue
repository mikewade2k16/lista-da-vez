<script setup lang="ts">
// Viewer em tela cheia dos anexos do calendario (imagem/video). Recebe a lista
// de itens e o indice inicial; navega com < >; fecha por X, Esc e clique no
// backdrop (as tres formas coexistem). Renderizado via Teleport no body para
// escapar de overflow/stacking dos cards de vidro.
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { getApiBase } from '~/utils/api-client'
import { resolveMediaUrl } from '~/utils/media'
import { formatBytes, type CalendarMediaItem } from '~/utils/calendar'

const props = defineProps<{
  items: CalendarMediaItem[]
  startIndex?: number
}>()

const emit = defineEmits<{ close: [] }>()

const index = ref(props.startIndex ?? 0)

// Reancora o indice se a lista ou o ponto de partida mudarem enquanto aberto.
watch(
  () => [props.items, props.startIndex] as const,
  () => {
    const start = props.startIndex ?? 0
    const max = Math.max(0, props.items.length - 1)
    index.value = Math.min(Math.max(start, 0), max)
  },
)

const current = computed<CalendarMediaItem | null>(() => props.items[index.value] ?? null)
const hasMany = computed(() => props.items.length > 1)

// Absolutiza /uploads/* para a apiBase (dev: web e api em portas diferentes).
const apiBase = getApiBase(useRuntimeConfig())
const currentSrc = computed(() =>
  current.value ? resolveMediaUrl(current.value.url, apiBase) : '',
)
const currentPoster = computed(() => {
  const poster = resolveMediaUrl(current.value?.posterUrl || '', apiBase)
  return poster || undefined
})

function prev(): void {
  if (!props.items.length) return
  index.value = (index.value - 1 + props.items.length) % props.items.length
}

function next(): void {
  if (!props.items.length) return
  index.value = (index.value + 1) % props.items.length
}

function onKeydown(event: KeyboardEvent): void {
  switch (event.key) {
    case 'Escape':
      emit('close')
      break
    case 'ArrowLeft':
      prev()
      break
    case 'ArrowRight':
      next()
      break
    default:
      break
  }
}

onMounted(() => document.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => document.removeEventListener('keydown', onKeydown))
</script>

<template>
  <Teleport to="body">
    <div
      class="calendar-viewer"
      role="dialog"
      aria-modal="true"
      aria-label="Visualizacao de anexo"
      @click.self="emit('close')"
    >
      <button
        type="button"
        class="calendar-viewer__close"
        aria-label="Fechar"
        @click="emit('close')"
      >
        <UIcon name="i-lucide-x" aria-hidden="true" />
      </button>

      <button
        v-if="hasMany"
        type="button"
        class="calendar-viewer__nav calendar-viewer__nav--prev"
        aria-label="Anterior"
        @click="prev"
      >
        <UIcon name="i-lucide-chevron-left" aria-hidden="true" />
      </button>

      <div class="calendar-viewer__stage" @click.self="emit('close')">
        <img
          v-if="current && current.type === 'image'"
          :src="currentSrc"
          :alt="current.name"
          class="calendar-viewer__media"
        />
        <video
          v-else-if="current"
          :key="current.id"
          :src="currentSrc"
          :poster="currentPoster"
          class="calendar-viewer__media"
          controls
          autoplay
          playsinline
        ></video>
      </div>

      <button
        v-if="hasMany"
        type="button"
        class="calendar-viewer__nav calendar-viewer__nav--next"
        aria-label="Proximo"
        @click="next"
      >
        <UIcon name="i-lucide-chevron-right" aria-hidden="true" />
      </button>

      <footer v-if="current" class="calendar-viewer__footer">
        <span class="calendar-viewer__name">{{ current.name }}</span>
        <span class="calendar-viewer__meta">
          {{ formatBytes(current.sizeBytes) }}
          <template v-if="hasMany">· {{ index + 1 }}/{{ items.length }}</template>
        </span>
      </footer>
    </div>
  </Teleport>
</template>
