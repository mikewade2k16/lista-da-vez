<script setup lang="ts">
import type {
  DropdownMenuItem,
  EditorCustomHandlers,
  EditorEmojiMenuItem,
  EditorMentionMenuItem,
  EditorSuggestionMenuItem,
  EditorToolbarItem,
} from '@nuxt/ui'
import { mapEditorItems } from '@nuxt/ui/utils/editor'
import { Emoji, gitHubEmojis } from '@tiptap/extension-emoji'
import { TextAlign } from '@tiptap/extension-text-align'
import type { Editor, JSONContent } from '@tiptap/vue-3'

const props = withDefaults(
  defineProps<{
    modelValue: string
    contentType?: 'html' | 'markdown' | 'json'
    editable?: boolean
    people?: string[]
    clients?: string[]
    tasks?: string[]
    placeholder?: string
    minHeight?: string
    maxHeight?: string
    compact?: boolean
  }>(),
  {
    modelValue: '',
    contentType: 'html',
    editable: true,
    people: () => [],
    clients: () => [],
    tasks: () => [],
    placeholder:
      'Pressione / para comandos, @ para pessoas, # para clientes e tasks, : para emojis...',
    minHeight: '320px',
    maxHeight: '58vh',
    compact: false,
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: string]
  'ai-action': [payload: { action: string; text: string }]
}>()

const lastEmittedModelValue = ref('')

function isSemanticallyEmptyContent(value: string) {
  const normalized = String(value || '')
    .replace(/<p>\s*<\/p>/gi, '')
    .replace(/<p>\s*<br\s*\/?>\s*<\/p>/gi, '')
    .replace(/<br\s*\/?>/gi, '')
    .replace(/&nbsp;/gi, ' ')
    .replace(/<[^>]+>/g, '')
    .trim()

  return normalized.length === 0
}

// Emissao PASSIVA vazia = o editor virou "vazio" SEM o usuario estar editando (montagem, troca de
// modelValue, normalizacao do tiptap para <p></p>). Nunca propagar isso: senao, no reload a nota
// carrega async e o editor monta vazio -> emitiria <p></p> -> PUT vazio agendado dispara DEPOIS que
// o conteudo real chegou e o apaga (bug real: toda nota virava <p></p>). Limpar de verdade acontece
// com o editor FOCADO (isFocused), e ai a emissao passa normalmente.
function shouldIgnorePassiveEmptyEmission(value: string, editor?: Editor | null) {
  if (editor?.isFocused) return false
  return isSemanticallyEmptyContent(value)
}

function serializeEditorContent(editor: Editor) {
  try {
    if (props.contentType === 'json') {
      return JSON.stringify(editor.getJSON())
    }
    if (props.contentType === 'markdown') {
      return editor.getMarkdown()
    }
    return editor.getHTML()
  } catch {
    return editor.getText()
  }
}

function emitModelValueIfChanged(value: string, editor?: Editor | null) {
  const nextValue = value || ''
  if (shouldIgnorePassiveEmptyEmission(nextValue, editor)) return
  if (nextValue === lastEmittedModelValue.value) return
  lastEmittedModelValue.value = nextValue
  emit('update:modelValue', nextValue)
}

function handleModelValueUpdate(value: string) {
  emitModelValueIfChanged(value)
}

function handleEditorTransaction(payload: { editor: any }) {
  const editor = payload?.editor as Editor | null | undefined
  if (!editor) return
  emitModelValueIfChanged(serializeEditorContent(editor), editor)
}

watch(
  () => props.modelValue,
  (value) => {
    lastEmittedModelValue.value = String(value || '')
  },
  { immediate: true },
)

const linkDraft = ref('')
const imageDraft = ref('')
const imageInputRef = ref<HTMLInputElement | null>(null)
const activeImageEditor = shallowRef<Editor | null>(null)
const selectedNode = ref<{ node: JSONContent; pos: number } | null>(null)

const editorStyle = computed(() => ({
  '--omni-editor-min-height': props.minHeight,
  '--omni-editor-max-height': props.maxHeight,
}))

function appendEditorMenuTo(): HTMLElement {
  return document.body
}

const userMentionItems = computed<EditorMentionMenuItem[]>(() =>
  uniqueLabels(props.people).map((label) => ({
    label,
    icon: 'i-lucide-user-round',
    avatar: { text: initials(label) },
    description: 'Pessoa',
  })),
)

const entityMentionItems = computed<EditorMentionMenuItem[][]>(() =>
  [
    uniqueLabels(props.clients).map((label) => ({
      label,
      icon: 'i-lucide-building-2',
      description: 'Cliente',
    })),
    uniqueLabels(props.tasks).map((label) => ({
      label,
      icon: 'i-lucide-circle-check',
      description: 'Task',
    })),
  ].filter((group) => group.length > 0),
)

const emojiItems = computed<EditorEmojiMenuItem[]>(() =>
  gitHubEmojis.filter((emoji) => !emoji.name.startsWith('regional_indicator_')),
)

function uniqueLabels(labels: string[]) {
  return Array.from(new Set(labels.map((label) => String(label || '').trim()).filter(Boolean)))
}

function initials(label: string) {
  return (
    label
      .split(/\s+/)
      .filter(Boolean)
      .slice(0, 2)
      .map((part) => part.slice(0, 1).toUpperCase())
      .join('') || '?'
  )
}

function nodeLabel(type?: string) {
  const labels: Record<string, string> = {
    paragraph: 'Paragraph',
    heading: 'Heading',
    bulletList: 'Bullet list',
    orderedList: 'Numbered list',
    blockquote: 'Blockquote',
    codeBlock: 'Code block',
    horizontalRule: 'Divider',
    image: 'Image',
  }
  return labels[String(type || '')] || 'Block'
}

function selectionText(editor: Editor) {
  const { from, to } = editor.state.selection
  return editor.state.doc.textBetween(from, to, '\n').trim()
}

const customHandlers = {
  aiContinue: {
    canExecute: (editor: Editor) => editor.isEditable,
    execute: (editor: Editor) => {
      emit('ai-action', { action: 'continue', text: selectionText(editor) })
      return editor.chain().focus()
    },
    isActive: () => false,
    isDisabled: (editor: Editor) => !editor.isEditable,
  },
  htmlBlock: {
    canExecute: (editor: Editor) => editor.isEditable,
    execute: (editor: Editor) => {
      const html = import.meta.client ? window.prompt('Cole o HTML que deseja inserir')?.trim() : ''
      if (!html) return editor.chain().focus()
      return editor.chain().focus().insertContent(html)
    },
    isActive: () => false,
    isDisabled: (editor: Editor) => !editor.isEditable,
  },
} satisfies EditorCustomHandlers

const fixedToolbarItems = [
  [
    {
      kind: 'undo',
      icon: 'i-lucide-undo',
      tooltip: { text: 'Desfazer' },
    },
    {
      kind: 'redo',
      icon: 'i-lucide-redo',
      tooltip: { text: 'Refazer' },
    },
  ],
  [
    {
      icon: 'i-lucide-heading',
      tooltip: { text: 'Titulos' },
      content: { align: 'start' },
      items: [
        { kind: 'heading', level: 1, icon: 'i-lucide-heading-1', label: 'Heading 1' },
        { kind: 'heading', level: 2, icon: 'i-lucide-heading-2', label: 'Heading 2' },
        { kind: 'heading', level: 3, icon: 'i-lucide-heading-3', label: 'Heading 3' },
        { kind: 'paragraph', icon: 'i-lucide-type', label: 'Paragraph' },
      ],
    },
    {
      icon: 'i-lucide-list',
      tooltip: { text: 'Listas' },
      content: { align: 'start' },
      items: [
        { kind: 'bulletList', icon: 'i-lucide-list', label: 'Bullet list' },
        { kind: 'orderedList', icon: 'i-lucide-list-ordered', label: 'Numbered list' },
      ],
    },
    {
      kind: 'blockquote',
      icon: 'i-lucide-text-quote',
      tooltip: { text: 'Citacao' },
    },
    {
      kind: 'codeBlock',
      icon: 'i-lucide-square-code',
      tooltip: { text: 'Codigo' },
    },
  ],
  [
    {
      kind: 'mark',
      mark: 'bold',
      icon: 'i-lucide-bold',
      tooltip: { text: 'Negrito' },
    },
    {
      kind: 'mark',
      mark: 'italic',
      icon: 'i-lucide-italic',
      tooltip: { text: 'Italico' },
    },
    {
      kind: 'mark',
      mark: 'underline',
      icon: 'i-lucide-underline',
      tooltip: { text: 'Sublinhado' },
    },
    {
      kind: 'mark',
      mark: 'strike',
      icon: 'i-lucide-strikethrough',
      tooltip: { text: 'Riscado' },
    },
    {
      kind: 'mark',
      mark: 'code',
      icon: 'i-lucide-code',
      tooltip: { text: 'Codigo inline' },
    },
  ],
  [
    {
      slot: 'link',
      icon: 'i-lucide-link',
      tooltip: { text: 'Link' },
    },
    {
      slot: 'image',
      icon: 'i-lucide-image',
      tooltip: { text: 'Imagem' },
    },
    {
      kind: 'htmlBlock',
      icon: 'i-lucide-code-xml',
      tooltip: { text: 'Inserir HTML' },
    },
  ],
  [
    {
      icon: 'i-lucide-align-justify',
      tooltip: { text: 'Alinhamento' },
      content: { align: 'end' },
      items: [
        { kind: 'textAlign', align: 'left', icon: 'i-lucide-align-left', label: 'Esquerda' },
        { kind: 'textAlign', align: 'center', icon: 'i-lucide-align-center', label: 'Centro' },
        { kind: 'textAlign', align: 'right', icon: 'i-lucide-align-right', label: 'Direita' },
        {
          kind: 'textAlign',
          align: 'justify',
          icon: 'i-lucide-align-justify',
          label: 'Justificado',
        },
      ],
    },
  ],
] satisfies EditorToolbarItem<typeof customHandlers>[][]

const bubbleToolbarItems = computed(
  () =>
    [
      [
        {
          icon: 'i-lucide-sparkles',
          label: 'AI',
          activeColor: 'neutral',
          activeVariant: 'ghost',
          content: { align: 'start' },
          items: [{ kind: 'aiContinue', icon: 'i-lucide-sparkles', label: 'Continue writing' }],
        },
      ],
      [
        {
          label: 'Turn into',
          trailingIcon: 'i-lucide-chevron-down',
          activeColor: 'neutral',
          activeVariant: 'ghost',
          content: { align: 'start' },
          items: [
            { type: 'label', label: 'Turn into' },
            { kind: 'paragraph', label: 'Paragraph', icon: 'i-lucide-type' },
            { kind: 'heading', level: 1, label: 'Heading 1', icon: 'i-lucide-heading-1' },
            { kind: 'heading', level: 2, label: 'Heading 2', icon: 'i-lucide-heading-2' },
            { kind: 'heading', level: 3, label: 'Heading 3', icon: 'i-lucide-heading-3' },
            { kind: 'bulletList', label: 'Bullet list', icon: 'i-lucide-list' },
            { kind: 'orderedList', label: 'Numbered list', icon: 'i-lucide-list-ordered' },
            { kind: 'blockquote', label: 'Blockquote', icon: 'i-lucide-text-quote' },
            { kind: 'codeBlock', label: 'Code block', icon: 'i-lucide-square-code' },
          ],
        },
      ],
      [
        {
          kind: 'mark',
          mark: 'bold',
          icon: 'i-lucide-bold',
          tooltip: { text: 'Negrito' },
        },
        {
          kind: 'mark',
          mark: 'italic',
          icon: 'i-lucide-italic',
          tooltip: { text: 'Italico' },
        },
        {
          kind: 'mark',
          mark: 'underline',
          icon: 'i-lucide-underline',
          tooltip: { text: 'Sublinhado' },
        },
        {
          kind: 'mark',
          mark: 'strike',
          icon: 'i-lucide-strikethrough',
          tooltip: { text: 'Riscado' },
        },
        {
          kind: 'mark',
          mark: 'code',
          icon: 'i-lucide-code',
          tooltip: { text: 'Codigo inline' },
        },
      ],
      [
        {
          slot: 'link',
          icon: 'i-lucide-link',
          tooltip: { text: 'Link' },
        },
        {
          slot: 'image',
          icon: 'i-lucide-image',
          tooltip: { text: 'Imagem' },
        },
      ],
    ] satisfies EditorToolbarItem<typeof customHandlers>[][],
)

const suggestionItems = [
  [
    {
      type: 'label',
      label: 'AI',
    },
    {
      kind: 'aiContinue',
      label: 'Continue writing',
      icon: 'i-lucide-sparkles',
    },
  ],
  [
    {
      type: 'label',
      label: 'Style',
    },
    {
      kind: 'paragraph',
      label: 'Paragraph',
      icon: 'i-lucide-type',
    },
    {
      kind: 'heading',
      level: 1,
      label: 'Heading 1',
      icon: 'i-lucide-heading-1',
    },
    {
      kind: 'heading',
      level: 2,
      label: 'Heading 2',
      icon: 'i-lucide-heading-2',
    },
    {
      kind: 'heading',
      level: 3,
      label: 'Heading 3',
      icon: 'i-lucide-heading-3',
    },
    {
      kind: 'bulletList',
      label: 'Bullet list',
      icon: 'i-lucide-list',
    },
    {
      kind: 'orderedList',
      label: 'Numbered list',
      icon: 'i-lucide-list-ordered',
    },
    {
      kind: 'blockquote',
      label: 'Blockquote',
      icon: 'i-lucide-text-quote',
    },
    {
      kind: 'codeBlock',
      label: 'Code block',
      icon: 'i-lucide-square-code',
    },
  ],
  [
    {
      type: 'label',
      label: 'Insert',
    },
    {
      kind: 'mention',
      label: 'Mention person',
      icon: 'i-lucide-at-sign',
    },
    {
      kind: 'emoji',
      label: 'Emoji',
      icon: 'i-lucide-smile-plus',
    },
    {
      kind: 'image',
      label: 'Image URL',
      icon: 'i-lucide-image',
    },
    {
      kind: 'htmlBlock',
      label: 'HTML',
      icon: 'i-lucide-code-xml',
    },
    {
      kind: 'horizontalRule',
      label: 'Divider',
      icon: 'i-lucide-separator-horizontal',
    },
  ],
] as unknown as EditorSuggestionMenuItem<typeof customHandlers>[][]

function handleItems(editor: Editor): DropdownMenuItem[][] {
  if (!selectedNode.value?.node?.type) return []

  return mapEditorItems(
    editor,
    [
      [
        { type: 'label', label: nodeLabel(selectedNode.value.node.type) },
        {
          label: 'Turn into',
          icon: 'i-lucide-repeat-2',
          children: [
            { kind: 'paragraph', label: 'Paragraph', icon: 'i-lucide-type' },
            { kind: 'heading', level: 1, label: 'Heading 1', icon: 'i-lucide-heading-1' },
            { kind: 'heading', level: 2, label: 'Heading 2', icon: 'i-lucide-heading-2' },
            { kind: 'heading', level: 3, label: 'Heading 3', icon: 'i-lucide-heading-3' },
            { kind: 'bulletList', label: 'Bullet list', icon: 'i-lucide-list' },
            { kind: 'orderedList', label: 'Numbered list', icon: 'i-lucide-list-ordered' },
            { kind: 'blockquote', label: 'Blockquote', icon: 'i-lucide-text-quote' },
            { kind: 'codeBlock', label: 'Code block', icon: 'i-lucide-square-code' },
          ],
        },
        {
          kind: 'clearFormatting',
          pos: selectedNode.value.pos,
          label: 'Reset formatting',
          icon: 'i-lucide-rotate-ccw',
        },
      ],
      [
        {
          kind: 'duplicate',
          pos: selectedNode.value.pos,
          label: 'Duplicate',
          icon: 'i-lucide-copy',
        },
        {
          label: 'Copy text',
          icon: 'i-lucide-clipboard',
          onSelect: async () => {
            if (!selectedNode.value || !import.meta.client) return
            const node = editor.state.doc.nodeAt(selectedNode.value.pos)
            if (node) await navigator.clipboard.writeText(node.textContent)
          },
        },
      ],
      [
        {
          kind: 'moveUp',
          pos: selectedNode.value.pos,
          label: 'Move up',
          icon: 'i-lucide-arrow-up',
        },
        {
          kind: 'moveDown',
          pos: selectedNode.value.pos,
          label: 'Move down',
          icon: 'i-lucide-arrow-down',
        },
      ],
      [
        {
          kind: 'delete',
          pos: selectedNode.value.pos,
          label: 'Delete',
          icon: 'i-lucide-trash',
        },
      ],
    ],
    customHandlers,
  ) as DropdownMenuItem[][]
}

function applyLink(editor: Editor) {
  const href = linkDraft.value.trim()
  if (!href) {
    editor.chain().focus().extendMarkRange('link').unsetLink().run()
    return
  }

  editor.chain().focus().extendMarkRange('link').setLink({ href }).run()
  linkDraft.value = ''
}

function prepareLink(editor: Editor, open: boolean) {
  if (!open) return
  linkDraft.value = String(editor.getAttributes('link')?.href || '')
}

function insertImageUrl(editor: Editor) {
  const src = imageDraft.value.trim()
  if (!src) return
  editor.chain().focus().setImage({ src }).run()
  imageDraft.value = ''
}

function openImageFilePicker(editor: Editor) {
  activeImageEditor.value = editor
  imageInputRef.value?.click()
}

function onImageFileChange(event: Event) {
  const input = event.target as HTMLInputElement | null
  const file = input?.files?.[0]
  const editor = activeImageEditor.value
  if (!file || !editor) return

  const reader = new FileReader()
  reader.onload = () => {
    const src = String(reader.result || '')
    if (src) editor.chain().focus().setImage({ src }).run()
  }
  reader.readAsDataURL(file)

  if (input) input.value = ''
  activeImageEditor.value = null
}
</script>

<template>
  <div class="omni-editor" :class="{ 'omni-editor--compact': props.compact }" :style="editorStyle">
    <UEditor
      v-slot="{ editor, handlers }"
      :key="props.editable ? 'editable' : 'readonly'"
      :model-value="props.modelValue || ''"
      :editable="props.editable"
      :content-type="contentType"
      :extensions="[Emoji, TextAlign.configure({ types: ['heading', 'paragraph'] })]"
      :handlers="customHandlers"
      :placeholder="{ placeholder, mode: 'everyLine', includeChildren: true }"
      :image="{ allowBase64: true, HTMLAttributes: { class: 'omni-editor__image' } }"
      :ui="{ base: 'omni-editor__content' }"
      class="omni-editor__instance"
      :on-transaction="handleEditorTransaction"
      @update:model-value="handleModelValueUpdate"
    >
      <UEditorToolbar :editor="editor" :items="fixedToolbarItems" class="omni-editor__toolbar">
        <template #link>
          <UPopover
            :content="{ side: 'bottom', align: 'start' }"
            @update:open="prepareLink(editor, $event)"
          >
            <UButton
              icon="i-lucide-link"
              color="neutral"
              variant="ghost"
              size="sm"
              :active="editor.isActive('link')"
            />
            <template #content>
              <div class="omni-editor__popover">
                <UInput
                  v-model="linkDraft"
                  placeholder="https://..."
                  size="sm"
                  @keydown.enter.prevent="applyLink(editor)"
                />
                <div class="omni-editor__popover-actions">
                  <UButton
                    label="Aplicar"
                    icon="i-lucide-check"
                    color="primary"
                    size="sm"
                    @click="applyLink(editor)"
                  />
                  <UButton
                    label="Remover"
                    icon="i-lucide-unlink"
                    color="neutral"
                    variant="soft"
                    size="sm"
                    @click="
                      () => {
                        linkDraft = ''
                        applyLink(editor)
                      }
                    "
                  />
                </div>
              </div>
            </template>
          </UPopover>
        </template>

        <template #image>
          <UPopover :content="{ side: 'bottom', align: 'start' }">
            <UButton
              icon="i-lucide-image"
              color="neutral"
              variant="ghost"
              size="sm"
              :active="editor.isActive('image')"
            />
            <template #content>
              <div class="omni-editor__popover">
                <UInput
                  v-model="imageDraft"
                  placeholder="URL da imagem"
                  size="sm"
                  @keydown.enter.prevent="insertImageUrl(editor)"
                />
                <div class="omni-editor__popover-actions">
                  <UButton
                    label="Inserir URL"
                    icon="i-lucide-check"
                    color="primary"
                    size="sm"
                    @click="insertImageUrl(editor)"
                  />
                  <UButton
                    label="Upload"
                    icon="i-lucide-upload"
                    color="neutral"
                    variant="soft"
                    size="sm"
                    @click="openImageFilePicker(editor)"
                  />
                </div>
              </div>
            </template>
          </UPopover>
        </template>
      </UEditorToolbar>

      <UEditorToolbar
        :editor="editor"
        :items="bubbleToolbarItems"
        layout="bubble"
        :options="{ placement: 'top', offset: 8 }"
        :should-show="
          ({ view, state }) =>
            view.hasFocus() && !state.selection.empty && !editor.isActive('image')
        "
      >
        <template #link>
          <UPopover
            :content="{ side: 'bottom', align: 'start' }"
            @update:open="prepareLink(editor, $event)"
          >
            <UButton
              icon="i-lucide-link"
              color="neutral"
              variant="ghost"
              size="sm"
              :active="editor.isActive('link')"
            />
            <template #content>
              <div class="omni-editor__popover">
                <UInput
                  v-model="linkDraft"
                  placeholder="https://..."
                  size="sm"
                  @keydown.enter.prevent="applyLink(editor)"
                />
                <div class="omni-editor__popover-actions">
                  <UButton
                    label="Aplicar"
                    icon="i-lucide-check"
                    color="primary"
                    size="sm"
                    @click="applyLink(editor)"
                  />
                  <UButton
                    label="Remover"
                    icon="i-lucide-unlink"
                    color="neutral"
                    variant="soft"
                    size="sm"
                    @click="
                      () => {
                        linkDraft = ''
                        applyLink(editor)
                      }
                    "
                  />
                </div>
              </div>
            </template>
          </UPopover>
        </template>

        <template #image>
          <UButton
            icon="i-lucide-image"
            color="neutral"
            variant="ghost"
            size="sm"
            @click="openImageFilePicker(editor)"
          />
        </template>
      </UEditorToolbar>

      <UEditorDragHandle
        v-slot="{ ui, onClick }"
        :editor="editor"
        @node-change="selectedNode = $event"
      >
        <UButton
          icon="i-lucide-plus"
          color="neutral"
          variant="ghost"
          size="sm"
          :class="ui.handle()"
          @click="
            (event: MouseEvent) => {
              event.stopPropagation()
              const selected = onClick()
              handlers.suggestion?.execute(editor, { pos: selected?.pos }).run()
            }
          "
        />

        <UDropdownMenu
          v-slot="{ open }"
          :modal="false"
          :items="handleItems(editor)"
          :content="{ side: 'left', align: 'start' }"
          :ui="{ content: 'z-[10020] w-56', label: 'text-xs' }"
          @update:open="editor.chain().setMeta('lockDragHandle', $event).run()"
        >
          <UButton
            color="neutral"
            variant="ghost"
            active-variant="soft"
            size="sm"
            icon="i-lucide-grip-vertical"
            :active="open"
            :class="ui.handle()"
          />
        </UDropdownMenu>
      </UEditorDragHandle>

      <UEditorSuggestionMenu
        :editor="editor"
        :items="suggestionItems"
        :append-to="appendEditorMenuTo"
        :options="{ placement: 'bottom-start', offset: 6, strategy: 'fixed' }"
        :ui="{ content: 'z-[10020]' }"
      />
      <UEditorMentionMenu
        :editor="editor"
        :items="userMentionItems"
        :append-to="appendEditorMenuTo"
        :options="{ placement: 'bottom-start', offset: 6, strategy: 'fixed' }"
        :ui="{ content: 'z-[10020]' }"
      />
      <UEditorMentionMenu
        v-if="entityMentionItems.length"
        :editor="editor"
        :items="entityMentionItems"
        char="#"
        plugin-key="entityMentionMenu"
        :append-to="appendEditorMenuTo"
        :options="{ placement: 'bottom-start', offset: 6, strategy: 'fixed' }"
        :ui="{ content: 'z-[10020]' }"
      />
      <UEditorEmojiMenu
        :editor="editor"
        :items="emojiItems"
        :append-to="appendEditorMenuTo"
        :options="{ placement: 'bottom-start', offset: 6, strategy: 'fixed' }"
        :ui="{ content: 'z-[10020]' }"
      />
    </UEditor>

    <input
      ref="imageInputRef"
      type="file"
      accept="image/*"
      class="omni-editor__file-input"
      @change="onImageFileChange"
    />
  </div>
</template>

<style scoped>
/* LAYOUT (fix estrutural, WAVE 11): a TOOLBAR fica FORA da area de rolagem — so o CONTEUDO
   rola (overflow no .omni-editor__content). Antes a toolbar era sticky DENTRO do scroll e o
   texto rolado vazava por cima dela (bug visual das anotacoes); com a coluna flex + scroll
   apenas no conteudo, a sobreposicao e impossivel em qualquer tema/browser. */
.omni-editor {
  position: relative;
  max-height: var(--omni-editor-max-height);
  min-height: var(--omni-editor-min-height);
  overflow: hidden;
  display: flex;
  flex-direction: column;
  border-top: 1px solid rgb(var(--border));
  color: rgb(var(--text));
}

.omni-editor__instance {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.omni-editor__toolbar {
  flex: 0 0 auto;
  border-bottom: 1px solid rgb(var(--border));
  background: rgb(var(--surface));
  padding: 0.45rem 0.15rem;
  overflow: visible;
  scrollbar-width: none;
}

.omni-editor__toolbar::-webkit-scrollbar {
  display: none;
}

.omni-editor :deep(.omni-editor__content) {
  flex: 1 1 auto;
  min-height: 0;
  overflow-y: auto;
  overflow-x: hidden;
  scrollbar-gutter: stable;
  padding: 1.4rem 2.2rem 5rem 2.55rem;
  outline: none;
  line-height: 1.75;
}

.omni-editor--compact .omni-editor__toolbar {
  padding: 0.2rem 0.1rem;
}

.omni-editor--compact :deep(.omni-editor__content) {
  padding: 0.75rem 1rem 1.5rem;
  line-height: 1.55;
}

.omni-editor--compact :deep(.tiptap > * + *) {
  margin-top: 0.45rem;
}

.omni-editor :deep(.tiptap) {
  outline: none;
}

.omni-editor :deep(.tiptap > * + *) {
  margin-top: 0.7rem;
}

.omni-editor :deep(.tiptap h1) {
  margin: 1rem 0 0.65rem;
  font-size: 2rem;
  font-weight: 850;
  line-height: 1.15;
  letter-spacing: 0;
}

.omni-editor :deep(.tiptap h2) {
  margin: 0.9rem 0 0.5rem;
  font-size: 1.45rem;
  font-weight: 800;
  line-height: 1.2;
}

.omni-editor :deep(.tiptap h3) {
  margin: 0.8rem 0 0.45rem;
  font-size: 1.12rem;
  font-weight: 800;
}

.omni-editor :deep(.tiptap ul),
.omni-editor :deep(.tiptap ol) {
  padding-left: 1.35rem;
}

.omni-editor :deep(.tiptap blockquote) {
  border-left: 3px solid rgb(var(--primary));
  padding-left: 0.85rem;
  color: rgb(var(--muted));
}

.omni-editor :deep(.tiptap pre) {
  overflow: auto;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2));
  padding: 0.85rem 1rem;
}

.omni-editor :deep(.tiptap code) {
  border-radius: 0.35rem;
  background: rgb(var(--surface-2));
  padding: 0.12rem 0.3rem;
  font-size: 0.9em;
}

.omni-editor :deep(.tiptap a) {
  color: rgb(var(--primary));
  text-decoration: underline;
  text-underline-offset: 0.18em;
}

.omni-editor :deep(.omni-editor__image),
.omni-editor :deep(.tiptap img) {
  max-width: 100%;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
}

.omni-editor :deep(.tiptap [data-type='mention']) {
  border-radius: 0.35rem;
  background: rgb(var(--primary) / 0.12);
  color: rgb(var(--primary));
  padding: 0.05rem 0.28rem;
  font-weight: 700;
}

.omni-editor :deep(.is-editor-empty:first-child::before),
.omni-editor :deep(.is-empty::before) {
  content: attr(data-placeholder);
  float: left;
  height: 0;
  color: rgb(var(--muted));
  pointer-events: none;
}

.omni-editor__popover {
  width: min(22rem, calc(100vw - 2rem));
  display: grid;
  gap: 0.55rem;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  background: rgb(var(--surface));
  padding: 0.65rem;
  box-shadow: var(--shadow-md);
}

.omni-editor__popover-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.45rem;
}

.omni-editor__file-input {
  display: none;
}

@media (max-width: 720px) {
  .omni-editor {
    max-height: 62vh;
  }

  .omni-editor :deep(.omni-editor__content) {
    padding-inline: 1rem;
  }
}
</style>
