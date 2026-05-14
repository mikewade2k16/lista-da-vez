<script setup lang="ts">
import Image from '@tiptap/extension-image'
import Placeholder from '@tiptap/extension-placeholder'
import StarterKit from '@tiptap/starter-kit'
import { EditorContent, useEditor } from '@tiptap/vue-3'

const props = withDefaults(defineProps<{
  modelValue: string
  people?: string[]
  clients?: string[]
  tasks?: string[]
  placeholder?: string
}>(), {
  modelValue: '',
  people: () => [],
  clients: () => [],
  tasks: () => [],
  placeholder: 'Pressione espaco para IA ou / para comandos'
})

const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

const htmlDraft = ref('')
const linkDraft = ref('')
const imageDraft = ref('')
const fileInputRef = ref<HTMLInputElement | null>(null)
const isSyncingFromParent = ref(false)
const emojiOptions = ['\u{1F600}', '\u{1F525}', '\u{2705}', '\u{26A0}\u{FE0F}', '\u{1F4A1}', '\u{1F3AF}', '\u{1F680}', '\u{2764}\u{FE0F}']

const mentionOptions = computed(() => [
  ...props.people.map(label => ({ label, icon: 'i-lucide-user-round', prefix: '@' })),
  ...props.clients.map(label => ({ label, icon: 'i-lucide-building-2', prefix: '#' })),
  ...props.tasks.map(label => ({ label, icon: 'i-lucide-circle-check', prefix: 'task:' }))
])

const editor = useEditor({
  content: props.modelValue || '<p></p>',
  extensions: [
    StarterKit.configure({
      link: {
        autolink: true,
        linkOnPaste: true,
        openOnClick: false,
        HTMLAttributes: { rel: 'noopener noreferrer', target: '_blank' }
      }
    }),
    Image.configure({ allowBase64: true, HTMLAttributes: { class: 'tasks-rich-editor__image' } }),
    Placeholder.configure({ placeholder: props.placeholder })
  ],
  editorProps: { attributes: { class: 'tasks-rich-editor__content' } },
  onUpdate: ({ editor }) => {
    if (isSyncingFromParent.value) return
    emit('update:modelValue', editor.getHTML())
  }
})

watch(() => props.modelValue, (value) => {
  if (!editor.value) return
  if (value === editor.value.getHTML()) return
  isSyncingFromParent.value = true
  editor.value.commands.setContent(value || '<p></p>', { emitUpdate: false })
  nextTick(() => { isSyncingFromParent.value = false })
})

onBeforeUnmount(() => {
  editor.value?.destroy()
})

function run(command: () => void) {
  if (!editor.value) return
  command()
}

function insertHtml() {
  const html = htmlDraft.value.trim()
  if (!html || !editor.value) return
  editor.value.chain().focus().insertContent(html).run()
  htmlDraft.value = ''
}

function applyLink() {
  const href = linkDraft.value.trim()
  if (!href || !editor.value) return
  editor.value.chain().focus().extendMarkRange('link').setLink({ href }).run()
  linkDraft.value = ''
}

function insertImageUrl() {
  const src = imageDraft.value.trim()
  if (!src || !editor.value) return
  editor.value.chain().focus().setImage({ src }).run()
  imageDraft.value = ''
}

function onImageFileChange(event: Event) {
  const input = event.target as HTMLInputElement | null
  const file = input?.files?.[0]
  if (!file || !editor.value) return

  const reader = new FileReader()
  reader.onload = () => {
    const src = String(reader.result || '')
    if (src) editor.value?.chain().focus().setImage({ src }).run()
  }
  reader.readAsDataURL(file)

  if (input) input.value = ''
}

function insertTextToken(value: string) {
  if (!editor.value) return
  editor.value.chain().focus().insertContent(value).run()
}
</script>

<template>
  <div class="tasks-rich-editor">
    <div class="tasks-rich-editor__toolbar">
      <UButton icon="i-lucide-heading-1" color="neutral" variant="ghost" size="xs" title="Titulo" @click="run(() => editor?.chain().focus().toggleHeading({ level: 1 }).run())" />
      <UButton icon="i-lucide-heading-2" color="neutral" variant="ghost" size="xs" title="Subtitulo" @click="run(() => editor?.chain().focus().toggleHeading({ level: 2 }).run())" />
      <UButton icon="i-lucide-bold" color="neutral" variant="ghost" size="xs" title="Negrito" @click="run(() => editor?.chain().focus().toggleBold().run())" />
      <UButton icon="i-lucide-italic" color="neutral" variant="ghost" size="xs" title="Italico" @click="run(() => editor?.chain().focus().toggleItalic().run())" />
      <UButton icon="i-lucide-list" color="neutral" variant="ghost" size="xs" title="Lista" @click="run(() => editor?.chain().focus().toggleBulletList().run())" />
      <UButton icon="i-lucide-list-ordered" color="neutral" variant="ghost" size="xs" title="Lista numerada" @click="run(() => editor?.chain().focus().toggleOrderedList().run())" />
      <UButton icon="i-lucide-quote" color="neutral" variant="ghost" size="xs" title="Citacao" @click="run(() => editor?.chain().focus().toggleBlockquote().run())" />

      <UPopover :content="{ side: 'bottom', align: 'start' }">
        <UButton icon="i-lucide-link" color="neutral" variant="ghost" size="xs" title="Link" />
        <template #content>
          <div class="tasks-rich-editor__popover">
            <UInput v-model="linkDraft" placeholder="https://..." size="sm" />
            <UButton label="Aplicar link" icon="i-lucide-check" color="primary" size="sm" @click="applyLink" />
          </div>
        </template>
      </UPopover>

      <UPopover :content="{ side: 'bottom', align: 'start' }">
        <UButton icon="i-lucide-image-plus" color="neutral" variant="ghost" size="xs" title="Imagem" />
        <template #content>
          <div class="tasks-rich-editor__popover">
            <UInput v-model="imageDraft" placeholder="URL da imagem" size="sm" />
            <div class="flex items-center gap-2">
              <UButton label="Inserir URL" icon="i-lucide-check" color="primary" size="sm" @click="insertImageUrl" />
              <UButton label="Upload" icon="i-lucide-upload" color="neutral" variant="soft" size="sm" @click="fileInputRef?.click()" />
            </div>
            <input ref="fileInputRef" type="file" accept="image/*" class="hidden" @change="onImageFileChange">
          </div>
        </template>
      </UPopover>

      <UPopover :content="{ side: 'bottom', align: 'start' }">
        <UButton icon="i-lucide-code-xml" color="neutral" variant="ghost" size="xs" title="HTML" />
        <template #content>
          <div class="tasks-rich-editor__popover tasks-rich-editor__popover--wide">
            <UTextarea v-model="htmlDraft" :rows="5" placeholder="<div>HTML</div>" />
            <UButton label="Inserir HTML" icon="i-lucide-check" color="primary" size="sm" @click="insertHtml" />
          </div>
        </template>
      </UPopover>

      <UPopover :content="{ side: 'bottom', align: 'start' }">
        <UButton icon="i-lucide-smile-plus" color="neutral" variant="ghost" size="xs" title="Emoji" />
        <template #content>
          <div class="tasks-rich-editor__emoji-grid">
            <button v-for="emoji in emojiOptions" :key="emoji" type="button" @click="insertTextToken(emoji)">
              {{ emoji }}
            </button>
          </div>
        </template>
      </UPopover>

      <UPopover :content="{ side: 'bottom', align: 'start' }">
        <UButton icon="i-lucide-at-sign" color="neutral" variant="ghost" size="xs" title="Mencionar" />
        <template #content>
          <div class="tasks-rich-editor__mention-menu">
            <button v-for="item in mentionOptions" :key="`${item.prefix}-${item.label}`" type="button" @click="insertTextToken(`${item.prefix}${item.label} `)">
              <UIcon :name="item.icon" class="h-4 w-4" />
              <span>{{ item.label }}</span>
            </button>
          </div>
        </template>
      </UPopover>
    </div>

    <EditorContent v-if="editor" :editor="editor" />
  </div>
</template>

<style scoped>
.tasks-rich-editor {
  border-top: 1px solid rgb(var(--border));
}

.tasks-rich-editor__toolbar {
  position: sticky;
  top: 0;
  z-index: 2;
  display: flex;
  flex-wrap: wrap;
  gap: 0.15rem;
  padding: 0.35rem 0;
  background: rgb(var(--surface));
}

.tasks-rich-editor :deep(.tasks-rich-editor__content) {
  min-height: 260px;
  padding: 1.25rem 0 4rem;
  color: rgb(var(--text));
  outline: none;
  line-height: 1.75;
}

.tasks-rich-editor :deep(.tasks-rich-editor__content p.is-editor-empty:first-child::before) {
  content: attr(data-placeholder);
  float: left;
  height: 0;
  color: rgb(var(--muted));
  pointer-events: none;
}

.tasks-rich-editor :deep(.tasks-rich-editor__content h1) {
  margin: 1rem 0 0.5rem;
  font-size: 1.875rem;
  font-weight: 800;
}

.tasks-rich-editor :deep(.tasks-rich-editor__content h2) {
  margin: 1rem 0 0.5rem;
  font-size: 1.35rem;
  font-weight: 750;
}

.tasks-rich-editor :deep(.tasks-rich-editor__content ul),
.tasks-rich-editor :deep(.tasks-rich-editor__content ol) {
  margin: 0.5rem 0;
  padding-left: 1.25rem;
}

.tasks-rich-editor :deep(.tasks-rich-editor__content blockquote) {
  margin: 0.75rem 0;
  border-left: 3px solid rgb(var(--primary));
  padding-left: 0.75rem;
  color: rgb(var(--muted));
}

.tasks-rich-editor :deep(.tasks-rich-editor__content a) {
  color: rgb(var(--primary));
  text-decoration: underline;
}

.tasks-rich-editor :deep(.tasks-rich-editor__image) {
  max-width: 100%;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
}

.tasks-rich-editor__popover {
  width: 18rem;
  display: grid;
  gap: 0.5rem;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  background: rgb(var(--surface));
  padding: 0.5rem;
  box-shadow: var(--shadow-md);
}

.tasks-rich-editor__popover--wide {
  width: min(28rem, calc(100vw - 2rem));
}

.tasks-rich-editor__emoji-grid {
  display: grid;
  grid-template-columns: repeat(4, 2rem);
  gap: 0.25rem;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  background: rgb(var(--surface));
  padding: 0.4rem;
}

.tasks-rich-editor__emoji-grid button,
.tasks-rich-editor__mention-menu button {
  border-radius: var(--radius-sm);
  padding: 0.35rem;
}

.tasks-rich-editor__emoji-grid button:hover,
.tasks-rich-editor__mention-menu button:hover {
  background: rgb(var(--surface-2));
}

.tasks-rich-editor__mention-menu {
  width: 16rem;
  max-height: 18rem;
  overflow: auto;
  display: grid;
  gap: 0.15rem;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  background: rgb(var(--surface));
  padding: 0.35rem;
  box-shadow: var(--shadow-md);
}

.tasks-rich-editor__mention-menu button {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  text-align: left;
}
</style>
