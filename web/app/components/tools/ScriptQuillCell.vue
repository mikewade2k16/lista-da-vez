<script setup lang="ts">
import type Quill from 'quill'
import { scriptImagesKey } from '~/composables/useScriptImages'

const images = inject(scriptImagesKey)
const props = withDefaults(defineProps<{ modelValue: string; editable?: boolean }>(), {
  editable: true,
})
const emit = defineEmits<{
  'update:modelValue': [value: string]
  activate: [editor: Quill, host: HTMLElement]
  deactivate: [editor: Quill]
  /** Toolbar wanted: text selected, or caret on an empty line (where images are inserted). */
  selection: [editor: Quill, wanted: boolean]
}>()
const host = ref<HTMLElement>()
const mount = ref<HTMLElement>()
const loadError = ref(false)
let editor: Quill | undefined
let disposed = false
let hydration = 0
let ready = false
function activate() {
  if (editor && host.value && ready) emit('activate', editor, host.value)
}
async function hydrate() {
  if (!editor) return
  const revision = ++hydration
  ready = false
  loadError.value = false
  editor.enable(false)
  try {
    const html = images ? await images.display(props.modelValue || '') : props.modelValue || ''
    if (disposed || revision !== hydration) return
    editor.setContents(editor.clipboard.convert({ html }), 'silent')
    ready = true
    editor.enable(props.editable)
  } catch {
    if (!disposed && revision === hydration) loadError.value = true
  }
}
onMounted(async () => {
  try {
    const [{ default: QuillEditor }, { default: ImageBlot }] = await Promise.all([
      import('quill'),
      import('quill/formats/image'),
    ])
    if (disposed || !mount.value) return
    class PrivateScriptImage extends ImageBlot {
      static override sanitize(url: string) {
        return url.startsWith('blob:') ? url : ImageBlot.sanitize(url)
      }
    }
    // Register once per Quill registry; private image URLs are resolved through authenticated fetch.
    QuillEditor.register('formats/image', PrivateScriptImage, true)
    editor = new QuillEditor(mount.value, {
      theme: 'snow',
      readOnly: true,
      modules: { toolbar: false },
    })
    editor.on('text-change', (_delta, _old, source) => {
      if (source === 'user' && ready && editor)
        emit(
          'update:modelValue',
          images ? images.persist(editor.root.innerHTML) : editor.root.innerHTML,
        )
    })
    editor.on('selection-change', (range) => {
      if (!range || !editor) return
      activate()
      const [line] = editor.getLine(range.index)
      emit('selection', editor, range.length > 0 || !line || line.length() <= 1)
    })
    editor.root.addEventListener('focus', activate)
    await hydrate()
  } catch {
    loadError.value = true
  }
})
watch(
  () => props.modelValue,
  (value) => {
    if (
      !editor ||
      value === (images ? images.persist(editor.root.innerHTML) : editor.root.innerHTML)
    )
      return
    void hydrate()
  },
)
watch(
  () => props.editable,
  (editable) => {
    if (ready) editor?.enable(editable)
  },
)
onBeforeUnmount(() => {
  disposed = true
  if (editor) {
    emit('deactivate', editor)
    editor.off('text-change')
    editor.off('selection-change')
    editor.root.removeEventListener('focus', activate)
  }
})
</script>
<template>
  <div ref="host" class="quill-cell toolbar-host">
    <div v-if="loadError" class="scripts-error" role="alert">
      Não foi possível carregar o conteúdo.
      <button type="button" @click="hydrate">Tentar novamente</button>
    </div>
    <div ref="mount"></div>
  </div>
</template>
