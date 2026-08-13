<script setup lang="ts">
import { computed, ref } from 'vue'
import OmniMediaViewer, { type OmniMediaViewerItem } from './OmniMediaViewer.vue'
import { reorderMediaItems } from './media-grid/utils'

export interface OmniMediaGridItem extends OmniMediaViewerItem {
  pending?: boolean
  progress?: number
  status?: string
  removable?: boolean
  reorderable?: boolean
  badgeLabel?: string
  badgeTone?: 'neutral' | 'primary'
}

export interface OmniMediaGridReorderPayload {
  from: number
  to: number
  orderedIds: string[]
}

const props = withDefaults(
  defineProps<{
    items: OmniMediaGridItem[]
    viewerItems?: OmniMediaViewerItem[]
    label?: string
    hint?: string
    error?: string
    accept?: string
    readonly?: boolean
    reorderable?: boolean
    showCover?: boolean
    uploadLabel?: string
    variant?: 'compact' | 'day'
  }>(),
  {
    viewerItems: () => [],
    label: '',
    hint: '',
    error: '',
    accept: 'image/*,video/*',
    readonly: false,
    reorderable: true,
    showCover: true,
    uploadLabel: 'Adicionar mídia',
    variant: 'compact',
  },
)

const emit = defineEmits<{
  files: [files: File[]]
  reorder: [payload: OmniMediaGridReorderPayload]
  remove: [id: string]
}>()

const input = ref<HTMLInputElement | null>(null)
const viewerIndex = ref<number | null>(null)
const dragIndex = ref<number | null>(null)
const dragOverIndex = ref<number | null>(null)
const fileDragActive = ref(false)
const effectiveViewerItems = computed(() =>
  props.viewerItems.length ? props.viewerItems : props.items.filter((item) => item.url),
)
const reorderableItemCount = computed(
  () => props.items.filter((item) => item.reorderable !== false && !item.pending).length,
)
const canReorder = computed(
  () =>
    !props.readonly &&
    props.reorderable &&
    reorderableItemCount.value > 1 &&
    !props.items.some((item) => item.pending),
)

function canReorderItem(item: OmniMediaGridItem): boolean {
  return canReorder.value && item.reorderable !== false
}

function mediaOrder(index: number): number {
  return props.items.slice(0, index).filter((item) => item.reorderable !== false && !item.pending)
    .length
}

function pick(): void {
  const fileInput = input.value
  if (!fileInput) return
  try {
    fileInput.showPicker()
  } catch {
    fileInput.click()
  }
}

function onFiles(event: Event): void {
  const target = event.target as HTMLInputElement
  const files = Array.from(target.files || [])
  target.value = ''
  if (files.length) emit('files', files)
}

function isExternalFileDrag(event: DragEvent): boolean {
  return Array.from(event.dataTransfer?.types || []).includes('Files')
}

function onFileDragOver(event: DragEvent): void {
  if (props.readonly || !isExternalFileDrag(event)) return
  event.preventDefault()
  if (event.dataTransfer) event.dataTransfer.dropEffect = 'copy'
  fileDragActive.value = true
}

function onFileDragLeave(event: DragEvent): void {
  if (event.currentTarget instanceof HTMLElement && event.relatedTarget instanceof Node) {
    if (event.currentTarget.contains(event.relatedTarget)) return
  }
  fileDragActive.value = false
}

function onFileDrop(event: DragEvent): void {
  if (props.readonly || !isExternalFileDrag(event)) return
  event.preventDefault()
  fileDragActive.value = false
  const files = Array.from(event.dataTransfer?.files || [])
  if (files.length) emit('files', files)
}

function openViewer(item: OmniMediaGridItem, fallbackIndex: number): void {
  if (!item.url) return
  const itemIndex = effectiveViewerItems.value.findIndex(
    (candidate) =>
      candidate.id === item.id && candidate.type === item.type && candidate.url === item.url,
  )
  viewerIndex.value = itemIndex >= 0 ? itemIndex : fallbackIndex
}

function onDragStart(index: number, event: DragEvent): void {
  if (!canReorderItem(props.items[index]!)) {
    event.preventDefault()
    return
  }
  dragIndex.value = index
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move'
    event.dataTransfer.setData('text/plain', String(index))
  }
}

function onDragOver(index: number, event: DragEvent): void {
  if (dragIndex.value === null || !canReorderItem(props.items[index]!)) return
  event.preventDefault()
  dragOverIndex.value = index
}

function onDrop(index: number, event: DragEvent): void {
  if (isExternalFileDrag(event)) {
    event.stopPropagation()
    onFileDrop(event)
    return
  }
  event.preventDefault()
  event.stopPropagation()
  if (!canReorderItem(props.items[index]!)) return
  const from = dragIndex.value
  dragIndex.value = null
  dragOverIndex.value = null
  if (from === null || from === index) return
  emit('reorder', {
    from,
    to: index,
    orderedIds: reorderMediaItems(
      props.items.map((item) => item.id),
      from,
      index,
    ),
  })
}

function onDragEnd(): void {
  dragIndex.value = null
  dragOverIndex.value = null
}
</script>

<template>
  <section
    class="omni-media-grid"
    :class="[`is-${variant}`, { 'is-file-dragging': fileDragActive }]"
    @dragover="onFileDragOver"
    @dragleave="onFileDragLeave"
    @drop="onFileDrop"
  >
    <header v-if="label || (variant === 'day' && !readonly)" class="omni-media-grid__header">
      <strong v-if="label">{{ label }}</strong>
      <button v-if="variant === 'day' && !readonly" type="button" @click="pick">
        <UIcon name="i-lucide-plus" aria-hidden="true" />
        Adicionar
      </button>
    </header>

    <div v-if="fileDragActive" class="omni-media-grid__drop-overlay" aria-hidden="true">
      <UIcon name="i-lucide-upload-cloud" />
      <span>Solte para enviar</span>
    </div>

    <div class="omni-media-grid__items">
      <article
        v-for="(item, index) in items"
        :key="item.id"
        class="omni-media-grid__cell"
        :class="{
          'is-dragging': dragIndex === index,
          'is-drag-over': dragOverIndex === index && dragIndex !== index,
        }"
        :draggable="canReorderItem(item)"
        @dragstart.capture="onDragStart(index, $event)"
        @dragover="onDragOver(index, $event)"
        @drop="onDrop(index, $event)"
        @dragend.capture="onDragEnd"
      >
        <div
          class="omni-media-grid__preview"
          role="button"
          tabindex="0"
          :aria-label="`Abrir ${item.name}`"
          @click="openViewer(item, index)"
          @keydown.enter.prevent="openViewer(item, index)"
          @keydown.space.prevent="openViewer(item, index)"
        >
          <img
            v-if="item.type === 'image' || item.posterUrl"
            :src="item.type === 'image' ? item.url : item.posterUrl"
            :alt="item.name"
            draggable="false"
          />
          <video
            v-else
            :src="item.url"
            preload="metadata"
            muted
            playsinline
            draggable="false"
          ></video>

          <span
            v-if="item.badgeLabel"
            class="omni-media-grid__source"
            :class="{ 'is-primary': item.badgeTone === 'primary' }"
          >
            {{ item.badgeLabel }}
          </span>
          <span
            v-if="showCover && !readonly && item.reorderable !== false"
            class="omni-media-grid__order"
            :class="{ 'has-source': item.badgeLabel }"
          >
            {{ mediaOrder(index) === 0 ? 'Capa' : mediaOrder(index) + 1 }}
          </span>
          <span v-if="canReorderItem(item)" class="omni-media-grid__grip" aria-hidden="true">
            <UIcon name="i-lucide-grip" />
          </span>
          <span v-if="item.type === 'video'" class="omni-media-grid__play" aria-hidden="true">
            <UIcon name="i-lucide-play" />
          </span>
          <button
            v-if="!readonly && item.removable !== false && !item.pending"
            type="button"
            class="omni-media-grid__remove"
            :aria-label="`Remover ${item.name}`"
            @click.stop="emit('remove', item.id)"
          >
            <UIcon name="i-lucide-x" />
          </button>
          <div v-if="item.pending" class="omni-media-grid__progress" aria-live="polite">
            <strong>{{ item.progress || 0 }}%</strong>
            <span>{{ item.status }}</span>
            <progress :value="item.progress || 0" max="100"></progress>
          </div>
        </div>
        <slot name="item-footer" :item="item" :index="index"></slot>
      </article>

      <button
        v-if="!readonly && variant !== 'day'"
        type="button"
        class="omni-media-grid__add"
        :aria-label="uploadLabel"
        @click.stop="pick"
      >
        <UIcon name="i-lucide-plus" aria-hidden="true" />
      </button>
    </div>

    <p v-if="error" class="omni-media-grid__error" role="alert">{{ error }}</p>
    <p v-else-if="hint" class="omni-media-grid__hint">{{ hint }}</p>

    <input ref="input" type="file" :accept="accept" multiple tabindex="-1" @change="onFiles" />

    <OmniMediaViewer
      v-if="viewerIndex !== null && effectiveViewerItems.length"
      :items="effectiveViewerItems"
      :start-index="viewerIndex"
      @close="viewerIndex = null"
    />
  </section>
</template>

<style scoped>
.omni-media-grid {
  position: relative;
  display: grid;
  gap: 0.4rem;
  min-width: 0;
}
.omni-media-grid__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  color: rgb(var(--muted));
  font-size: 0.74rem;
}
.omni-media-grid__header button {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  border: 1px dashed rgb(var(--border));
  border-radius: var(--radius-sm);
  padding: 0.18rem 0.45rem;
  background: transparent;
  color: rgb(var(--muted));
  cursor: pointer;
}
.omni-media-grid__items {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(5.25rem, 1fr));
  gap: 0.5rem;
}
.omni-media-grid__cell {
  display: grid;
  gap: 0.3rem;
  min-width: 0;
}
.omni-media-grid__cell[draggable='true'] {
  cursor: grab;
}
.omni-media-grid__cell.is-dragging {
  opacity: 0.45;
}
.omni-media-grid__cell.is-drag-over .omni-media-grid__preview {
  outline: 2px dashed rgb(var(--primary) / 0.75);
  outline-offset: 2px;
}
.omni-media-grid__preview,
.omni-media-grid__add {
  position: relative;
  width: 100%;
  aspect-ratio: 1;
  overflow: hidden;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2));
}
.omni-media-grid__preview {
  cursor: pointer;
}
.omni-media-grid__preview img,
.omni-media-grid__preview video {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: contain;
}
.omni-media-grid__order,
.omni-media-grid__source,
.omni-media-grid__grip,
.omni-media-grid__remove {
  position: absolute;
  z-index: 3;
  display: inline-grid;
  place-items: center;
  border: 0;
  background: rgb(var(--surface) / 0.86);
  color: rgb(var(--text));
}
.omni-media-grid__order,
.omni-media-grid__source {
  top: 0.2rem;
  left: 0.2rem;
  min-height: 1.2rem;
  padding: 0.1rem 0.35rem;
  border-radius: 999px;
  font-size: 0.58rem;
  font-weight: 700;
}
.omni-media-grid__order.has-source {
  top: auto;
  bottom: 0.2rem;
}
.omni-media-grid__source.is-primary {
  background: rgb(var(--primary) / 0.9);
  color: rgb(var(--primary-foreground));
}
.omni-media-grid__grip {
  right: 0.2rem;
  bottom: 0.2rem;
  width: 1.25rem;
  height: 1.25rem;
  border-radius: var(--radius-sm);
  pointer-events: none;
}
.omni-media-grid__remove {
  top: 0.15rem;
  right: 0.15rem;
  width: 1.3rem;
  height: 1.3rem;
  border-radius: 999px;
  cursor: pointer;
}
.omni-media-grid__play {
  position: absolute;
  inset: 0;
  display: grid;
  place-items: center;
  background: rgb(var(--modal-scrim) / 0.28);
  color: rgb(var(--primary-foreground));
  pointer-events: none;
}
.omni-media-grid__progress {
  position: absolute;
  inset: 0;
  z-index: 2;
  display: grid;
  align-content: end;
  gap: 0.2rem;
  padding: 0.45rem;
  background: linear-gradient(transparent, rgb(var(--modal-scrim) / 0.88));
  color: rgb(var(--primary-foreground));
  font-size: 0.62rem;
}
.omni-media-grid__progress progress {
  width: 100%;
  height: 0.25rem;
  accent-color: rgb(var(--primary));
}
.omni-media-grid__add {
  display: grid;
  place-items: center;
  border-style: dashed;
  background: transparent;
  color: rgb(var(--primary));
  cursor: pointer;
}
.omni-media-grid__hint,
.omni-media-grid__error {
  margin: 0;
  font-size: 0.7rem;
}
.omni-media-grid__hint {
  color: rgb(var(--muted));
}
.omni-media-grid__error {
  color: rgb(var(--error));
}
.omni-media-grid > input[type='file'] {
  position: fixed;
  width: 1px;
  height: 1px;
  opacity: 0;
  pointer-events: none;
}
.omni-media-grid__drop-overlay {
  position: absolute;
  inset: 0;
  z-index: 5;
  display: grid;
  place-content: center;
  justify-items: center;
  gap: 0.3rem;
  border: 2px dashed rgb(var(--primary));
  border-radius: var(--radius-sm);
  background: rgb(var(--surface) / 0.92);
  color: rgb(var(--primary));
  pointer-events: none;
}
.omni-media-grid.is-day .omni-media-grid__items {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}
</style>
