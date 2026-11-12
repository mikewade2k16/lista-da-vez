<script setup lang="ts">
import type Quill from 'quill'
const props = defineProps<{ editor: Quill; host: HTMLElement; uploading: boolean }>()
const emit = defineEmits<{ image: [editor: Quill]; hover: [inside: boolean] }>()
const formats = ref<Record<string, unknown>>({})
// Never call getFormat() without a range here: Quill resolves it via getSelection(true), which
// focuses the editor. On blur that pulls the caret back into this cell and blocks other cells.
let savedRange: { index: number; length: number } | null = null
function refresh() {
  const range = props.editor.getSelection()
  if (range) {
    savedRange = range
    formats.value = props.editor.getFormat(range)
  }
}
function keepSelection(event: PointerEvent) {
  if (!(event.target instanceof HTMLSelectElement)) event.preventDefault()
}
function format(name: string) {
  props.editor.format(name, !props.editor.getFormat()[name], 'user')
  refresh()
  props.editor.focus()
}
function list(type: string) {
  props.editor.format('list', props.editor.getFormat().list === type ? false : type, 'user')
  refresh()
  props.editor.focus()
}
// Inline size (Quill "size" class format): applies only to the selected text, unlike the
// block "header" format the old "Title" picker used, which restyled the whole line.
function sizeChanged(event: Event) {
  const range = props.editor.getSelection() || savedRange
  if (!range) return
  props.editor.formatText(
    range.index,
    range.length,
    'size',
    (event.target as HTMLSelectElement).value || false,
    'user',
  )
  props.editor.setSelection(range.index, range.length, 'user')
  refresh()
}
function upload() {
  emit('image', props.editor)
}
watch(
  () => props.editor,
  (editor, old) => {
    old?.off('selection-change', refresh)
    editor.on('selection-change', refresh)
    refresh()
  },
  { immediate: true },
)
onBeforeUnmount(() => props.editor.off('selection-change', refresh))
</script>
<template>
  <Teleport :to="host">
    <div
      class="script-floating-toolbar ql-toolbar ql-snow toolbar-floating"
      @pointerdown="keepSelection"
      @pointerenter="emit('hover', true)"
      @pointerleave="emit('hover', false)"
    >
      <span class="ql-formats">
        <select
          class="script-size-select"
          title="Tamanho da fonte"
          aria-label="Tamanho da fonte"
          :value="formats.size || ''"
          @focus="emit('hover', true)"
          @blur="emit('hover', false)"
          @change="sizeChanged"
        >
          <option value="small">Pequeno</option>
          <option value="">Normal</option>
          <option value="large">Grande</option>
          <option value="huge">Enorme</option>
        </select>
      </span>
      <span class="ql-formats">
        <button
          type="button"
          class="ql-bold"
          :class="{ 'ql-active': formats['bold'] }"
          title="Negrito"
          @click="format('bold')"
        >
          <!-- format_bold -->
          <svg
            xmlns="http://www.w3.org/2000/svg"
            height="16"
            width="16"
            fill="currentColor"
            viewBox="0 -960 960 960"
          >
            <path
              d="M280-160v-640h240q83 0 141.5 58.5T720-600q0 51-25 92.5T640-440q42 20 71 61t29 99q0 83-58.5 141.5T540-80H280Zm120-520v160h120q33 0 56.5-23.5T600-560q0-33-23.5-56.5T520-640H400Zm0 280v160h160q33 0 56.5-23.5T640-280q0-33-23.5-56.5T560-360H400Z"
            />
          </svg>
        </button>
        <button
          type="button"
          class="ql-italic"
          :class="{ 'ql-active': formats['italic'] }"
          title="Itálico"
          @click="format('italic')"
        >
          <!-- format_italic -->
          <svg
            xmlns="http://www.w3.org/2000/svg"
            height="16"
            width="16"
            fill="currentColor"
            viewBox="0 -960 960 960"
          >
            <path d="M200-160v-80h160l120-480H320v-80h440v80H600L480-240h160v80H200Z" />
          </svg>
        </button>
        <button
          type="button"
          class="ql-underline"
          :class="{ 'ql-active': formats['underline'] }"
          title="Sublinhado"
          @click="format('underline')"
        >
          <!-- format_underlined -->
          <svg
            xmlns="http://www.w3.org/2000/svg"
            height="16"
            width="16"
            fill="currentColor"
            viewBox="0 -960 960 960"
          >
            <path
              d="M240-120v-80h480v80H240Zm240-200q-100 0-170-70t-70-170v-240h80v240q0 67 46.5 113.5T480-400q67 0 113.5-46.5T640-560v-240h80v240q0 100-70 170t-170 70Z"
            />
          </svg>
        </button>
      </span>

      <span class="ql-formats">
        <button
          type="button"
          class="ql-blockquote"
          :class="{ 'ql-active': formats['blockquote'] }"
          title="Citação"
          @click="format('blockquote')"
        >
          <!-- format_quote -->
          <svg
            xmlns="http://www.w3.org/2000/svg"
            height="16"
            width="16"
            fill="currentColor"
            viewBox="0 -960 960 960"
          >
            <path
              d="M280-360h120v-240H240v160h40v80Zm320 0h120v-240H560v160h40v80ZM160-120v-80h640v80H160Z"
            />
          </svg>
        </button>
        <button
          type="button"
          class="ql-code-block"
          :class="{ 'ql-active': formats['code-block'] }"
          title="Código"
          @click="format('code-block')"
        >
          <!-- code -->
          <svg
            xmlns="http://www.w3.org/2000/svg"
            height="16"
            width="16"
            fill="currentColor"
            viewBox="0 -960 960 960"
          >
            <path d="m400-200-56-56 200-200-200-200 56-56 256 256-256 256Z" />
          </svg>
        </button>
      </span>

      <span class="ql-formats">
        <button
          type="button"
          class="ql-list"
          value="ordered"
          :class="{ 'ql-active': formats.list === 'ordered' }"
          title="Lista ordenada"
          @click="list('ordered')"
        >
          <!-- format_list_numbered -->
          <svg
            xmlns="http://www.w3.org/2000/svg"
            height="16"
            width="16"
            fill="currentColor"
            viewBox="0 -960 960 960"
          >
            <path
              d="M160-120v-60h60v60h-60Zm0-200v-60h60v60h-60Zm0-200v-60h60v60h-60Zm160 400v-60h480v60H320Zm0-200v-60h480v60H320Zm0-200v-60h480v60H320Z"
            />
          </svg>
        </button>
        <button
          type="button"
          class="ql-list"
          value="bullet"
          :class="{ 'ql-active': formats.list === 'bullet' }"
          title="Lista com marcadores"
          @click="list('bullet')"
        >
          <!-- format_list_bulleted -->
          <svg
            xmlns="http://www.w3.org/2000/svg"
            height="16"
            width="16"
            fill="currentColor"
            viewBox="0 -960 960 960"
          >
            <path
              d="M160-120q-17 0-28.5-11.5T120-160q0-17 11.5-28.5T160-200q17 0 28.5 11.5T200-160q0 17-11.5 28.5T160-120Zm0-200q-17 0-28.5-11.5T120-360q0-17 11.5-28.5T160-400q17 0 28.5 11.5T200-360q0 17-11.5 28.5T160-320Zm0-200q-17 0-28.5-11.5T120-560q0-17 11.5-28.5T160-600q17 0 28.5 11.5T200-560q0 17-11.5 28.5T160-520ZM320-120v-60h480v60H320Zm0-200v-60h480v60H320Zm0-200v-60h480v60H320Z"
            />
          </svg>
        </button>
      </span>

      <span class="ql-formats">
        <button type="button" class="ql-image" :disabled="uploading" title="Imagem" @click="upload">
          <!-- image -->
          <svg
            xmlns="http://www.w3.org/2000/svg"
            height="16"
            width="16"
            fill="currentColor"
            viewBox="0 -960 960 960"
          >
            <path
              d="M160-120q-33 0-56.5-23.5T80-200v-560q0-33 23.5-56.5T160-840h640q33 0 56.5 23.5T880-760v560q0 33-23.5 56.5T800-120H160Zm0-80h640v-560H160v560Zm60-100h520L560-480 440-320l-80-120-140 200Z"
            />
          </svg>
        </button>
      </span>
    </div>
  </Teleport>
</template>
