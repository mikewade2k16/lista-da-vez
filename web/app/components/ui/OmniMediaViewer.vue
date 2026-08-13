<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import OmniVideoPlayer from './OmniVideoPlayer.vue'
import type { OmniVideoPlayerControl } from './video-player/types'

export interface OmniMediaViewerItem {
  id: string
  name: string
  url: string
  type: 'image' | 'video'
  sizeLabel?: string
  posterUrl?: string
}

const props = defineProps<{ items: OmniMediaViewerItem[]; startIndex?: number }>()
const emit = defineEmits<{ close: [] }>()
const index = ref(props.startIndex ?? 0)
const videoFailed = ref(false)
const current = computed(() => props.items[index.value] ?? null)
const hasMany = computed(() => props.items.length > 1)
const viewerControls: readonly OmniVideoPlayerControl[] = [
  'play',
  'seek-back',
  'seek-forward',
  'progress',
  'time',
  'volume',
  'speed',
  'pip',
  'fullscreen',
]

watch(
  () => [props.items, props.startIndex] as const,
  () => {
    index.value = Math.min(Math.max(props.startIndex ?? 0, 0), Math.max(props.items.length - 1, 0))
  },
)
watch(
  () => current.value?.url,
  () => {
    videoFailed.value = false
  },
)

function previous(): void {
  if (props.items.length) index.value = (index.value - 1 + props.items.length) % props.items.length
}
function next(): void {
  if (props.items.length) index.value = (index.value + 1) % props.items.length
}
function onKeydown(event: KeyboardEvent): void {
  if (event.defaultPrevented) return
  if (event.key === 'Escape') emit('close')
  else if (event.key === 'ArrowLeft') previous()
  else if (event.key === 'ArrowRight') next()
}
onMounted(() => document.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => document.removeEventListener('keydown', onKeydown))
</script>

<template>
  <Teleport to="body">
    <div
      class="omni-media-viewer"
      role="dialog"
      aria-modal="true"
      aria-label="Visualização de mídia"
      @click.self="emit('close')"
    >
      <section class="omni-media-viewer__modal">
        <header class="omni-media-viewer__header">
          <div class="min-w-0">
            <strong class="omni-media-viewer__name">{{ current?.name }}</strong>
            <small v-if="current">
              {{ current.sizeLabel }}
              <template v-if="hasMany">· {{ index + 1 }}/{{ items.length }}</template>
            </small>
          </div>
          <button
            type="button"
            class="omni-media-viewer__icon"
            aria-label="Fechar"
            @click="emit('close')"
          >
            <UIcon name="i-lucide-x" />
          </button>
        </header>
        <div class="omni-media-viewer__stage">
          <img v-if="current?.type === 'image'" :src="current.url" :alt="current.name" />
          <OmniVideoPlayer
            v-else-if="current"
            :key="`${current.id}:${current.url}`"
            class="omni-media-viewer__player"
            :src="current.url"
            :poster="current.posterUrl"
            :title="current.name"
            theme="cinema"
            fit="contain"
            autofocus
            :controls="viewerControls"
            @error="videoFailed = true"
          />
          <div v-if="videoFailed && current?.type === 'video'" class="omni-media-viewer__fallback">
            <strong>Este navegador não consegue reproduzir o codec do arquivo original.</strong>
            <span>O original permanece preservado sem alterações.</span>
            <a :href="current.url" target="_blank" rel="noopener noreferrer">
              Abrir arquivo original
            </a>
          </div>
          <button
            v-if="hasMany"
            type="button"
            class="omni-media-viewer__nav is-previous"
            aria-label="Arquivo anterior"
            @click="previous"
          >
            <UIcon name="i-lucide-chevron-left" />
          </button>
          <button
            v-if="hasMany"
            type="button"
            class="omni-media-viewer__nav is-next"
            aria-label="Próximo arquivo"
            @click="next"
          >
            <UIcon name="i-lucide-chevron-right" />
          </button>
        </div>
      </section>
    </div>
  </Teleport>
</template>

<style scoped>
.omni-media-viewer {
  position: fixed;
  inset: 0;
  z-index: 10000;
  display: grid;
  place-items: center;
  padding: clamp(0.75rem, 3vw, 2rem);
  background: rgb(var(--modal-scrim) / 0.82);
  backdrop-filter: blur(4px);
  -webkit-backdrop-filter: blur(4px);
}
.omni-media-viewer__modal {
  display: flex;
  flex-direction: column;
  width: min(92vw, 76rem);
  height: min(90vh, 52rem);
  height: min(90dvh, 52rem);
  min-height: 0;
  overflow: hidden;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-lg);
  background: rgb(var(--surface));
  box-shadow: var(--shadow-sm);
}
.omni-media-viewer__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.75rem 1rem;
  border-bottom: 1px solid rgb(var(--border));
}
.omni-media-viewer__header > div {
  display: flex;
  flex-direction: column;
}
.omni-media-viewer__header small {
  color: rgb(var(--muted));
}
.omni-media-viewer__name {
  overflow: hidden;
  color: rgb(var(--text));
  text-overflow: ellipsis;
  white-space: nowrap;
}
.omni-media-viewer__icon,
.omni-media-viewer__nav {
  display: inline-grid;
  place-items: center;
  border: 0;
  border-radius: 999px;
  background: rgb(var(--surface-2));
  color: rgb(var(--text));
  cursor: pointer;
}
.omni-media-viewer__fallback {
  position: absolute;
  inset: auto 1rem 1rem;
  display: grid;
  gap: 0.4rem;
  justify-items: center;
  padding: 0.85rem;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-md);
  background: rgb(var(--surface));
  color: rgb(var(--text));
  text-align: center;
}
.omni-media-viewer__fallback span {
  color: rgb(var(--muted));
}
.omni-media-viewer__fallback a {
  border: 0;
  border-radius: var(--radius-md);
  padding: 0.5rem 0.8rem;
  background: rgb(var(--primary));
  color: rgb(var(--primary-foreground));
  cursor: pointer;
  text-decoration: none;
}
.omni-media-viewer__icon {
  width: 2.25rem;
  height: 2.25rem;
}
.omni-media-viewer__stage {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  flex: 1;
  min-height: 0;
  overflow: hidden;
  padding: 1rem;
  background: rgb(var(--surface-2));
}
.omni-media-viewer__stage img,
.omni-media-viewer__player {
  display: block;
  width: 100%;
  height: 100%;
  min-width: 0;
  min-height: 0;
}
.omni-media-viewer__stage img {
  background: rgb(var(--surface-2));
  object-fit: contain;
  object-position: center;
}
.omni-media-viewer__nav {
  position: absolute;
  top: 50%;
  width: 2.75rem;
  height: 2.75rem;
  transform: translateY(-50%);
  background: color-mix(in srgb, rgb(var(--text)) 58%, transparent);
  color: rgb(var(--surface));
}
.omni-media-viewer__nav.is-previous {
  left: 0.75rem;
}
.omni-media-viewer__nav.is-next {
  right: 0.75rem;
}
@media (max-width: 640px) {
  .omni-media-viewer {
    padding: 0.5rem;
  }
  .omni-media-viewer__modal {
    width: 100%;
    height: min(92vh, 52rem);
    height: min(92dvh, 52rem);
  }
}
</style>
