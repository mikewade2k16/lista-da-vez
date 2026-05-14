<script setup lang="ts">
import type {
  DropdownMenuItem,
  EditorCustomHandlers,
  EditorEmojiMenuItem,
  EditorMentionMenuItem,
  EditorSuggestionMenuItem,
  EditorToolbarItem
} from "@nuxt/ui";
import { mapEditorItems } from "@nuxt/ui/utils/editor";
import { Emoji, gitHubEmojis } from "@tiptap/extension-emoji";
import { TextAlign } from "@tiptap/extension-text-align";
import type { Editor, JSONContent } from "@tiptap/vue-3";

const props = withDefaults(defineProps<{
  modelValue: string
  contentType?: "html" | "markdown" | "json"
  people?: string[]
  clients?: string[]
  tasks?: string[]
  placeholder?: string
  minHeight?: string
  maxHeight?: string
}>(), {
  modelValue: "",
  contentType: "html",
  people: () => [],
  clients: () => [],
  tasks: () => [],
  placeholder: "Pressione / para comandos, @ para pessoas, # para clientes e tasks, : para emojis...",
  minHeight: "320px",
  maxHeight: "58vh"
});

const emit = defineEmits<{
  "update:modelValue": [value: string]
  "ai-action": [payload: { action: string, text: string }]
}>();

const editorValue = computed({
  get: () => props.modelValue || "",
  set: (value: string) => emit("update:modelValue", value)
});

const linkDraft = ref("");
const imageDraft = ref("");
const imageInputRef = ref<HTMLInputElement | null>(null);
const activeImageEditor = shallowRef<Editor | null>(null);
const selectedNode = ref<{ node: JSONContent, pos: number } | null>(null);

const editorStyle = computed(() => ({
  "--omni-editor-min-height": props.minHeight,
  "--omni-editor-max-height": props.maxHeight
}));

const userMentionItems = computed<EditorMentionMenuItem[]>(() =>
  uniqueLabels(props.people).map((label) => ({
    label,
    icon: "i-lucide-user-round",
    avatar: { text: initials(label) },
    description: "Pessoa"
  }))
);

const entityMentionItems = computed<EditorMentionMenuItem[][]>(() => [
  uniqueLabels(props.clients).map((label) => ({
    label,
    icon: "i-lucide-building-2",
    description: "Cliente"
  })),
  uniqueLabels(props.tasks).map((label) => ({
    label,
    icon: "i-lucide-circle-check",
    description: "Task"
  }))
].filter(group => group.length > 0));

const emojiItems = computed<EditorEmojiMenuItem[]>(() =>
  gitHubEmojis.filter(emoji => !emoji.name.startsWith("regional_indicator_"))
);

function uniqueLabels(labels: string[]) {
  return Array.from(new Set(labels.map(label => String(label || "").trim()).filter(Boolean)));
}

function initials(label: string) {
  return label
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map(part => part.slice(0, 1).toUpperCase())
    .join("") || "?";
}

function nodeLabel(type?: string) {
  const labels: Record<string, string> = {
    paragraph: "Paragraph",
    heading: "Heading",
    bulletList: "Bullet list",
    orderedList: "Numbered list",
    blockquote: "Blockquote",
    codeBlock: "Code block",
    horizontalRule: "Divider",
    image: "Image"
  };
  return labels[String(type || "")] || "Block";
}

function selectionText(editor: Editor) {
  const { from, to } = editor.state.selection;
  return editor.state.doc.textBetween(from, to, "\n").trim();
}

const customHandlers = {
  aiContinue: {
    canExecute: (editor: Editor) => editor.isEditable,
    execute: (editor: Editor) => {
      emit("ai-action", { action: "continue", text: selectionText(editor) });
      return editor.chain().focus();
    },
    isActive: () => false,
    isDisabled: (editor: Editor) => !editor.isEditable
  },
  htmlBlock: {
    canExecute: (editor: Editor) => editor.isEditable,
    execute: (editor: Editor) => {
      const html = import.meta.client ? window.prompt("Cole o HTML que deseja inserir")?.trim() : "";
      if (!html) return editor.chain().focus();
      return editor.chain().focus().insertContent(html);
    },
    isActive: () => false,
    isDisabled: (editor: Editor) => !editor.isEditable
  }
} satisfies EditorCustomHandlers;

const fixedToolbarItems = [[{
  kind: "undo",
  icon: "i-lucide-undo",
  tooltip: { text: "Desfazer" }
}, {
  kind: "redo",
  icon: "i-lucide-redo",
  tooltip: { text: "Refazer" }
}], [{
  icon: "i-lucide-heading",
  tooltip: { text: "Titulos" },
  content: { align: "start" },
  items: [
    { kind: "heading", level: 1, icon: "i-lucide-heading-1", label: "Heading 1" },
    { kind: "heading", level: 2, icon: "i-lucide-heading-2", label: "Heading 2" },
    { kind: "heading", level: 3, icon: "i-lucide-heading-3", label: "Heading 3" },
    { kind: "paragraph", icon: "i-lucide-type", label: "Paragraph" }
  ]
}, {
  icon: "i-lucide-list",
  tooltip: { text: "Listas" },
  content: { align: "start" },
  items: [
    { kind: "bulletList", icon: "i-lucide-list", label: "Bullet list" },
    { kind: "orderedList", icon: "i-lucide-list-ordered", label: "Numbered list" }
  ]
}, {
  kind: "blockquote",
  icon: "i-lucide-text-quote",
  tooltip: { text: "Citacao" }
}, {
  kind: "codeBlock",
  icon: "i-lucide-square-code",
  tooltip: { text: "Codigo" }
}], [{
  kind: "mark",
  mark: "bold",
  icon: "i-lucide-bold",
  tooltip: { text: "Negrito" }
}, {
  kind: "mark",
  mark: "italic",
  icon: "i-lucide-italic",
  tooltip: { text: "Italico" }
}, {
  kind: "mark",
  mark: "underline",
  icon: "i-lucide-underline",
  tooltip: { text: "Sublinhado" }
}, {
  kind: "mark",
  mark: "strike",
  icon: "i-lucide-strikethrough",
  tooltip: { text: "Riscado" }
}, {
  kind: "mark",
  mark: "code",
  icon: "i-lucide-code",
  tooltip: { text: "Codigo inline" }
}], [{
  slot: "link",
  icon: "i-lucide-link",
  tooltip: { text: "Link" }
}, {
  slot: "image",
  icon: "i-lucide-image",
  tooltip: { text: "Imagem" }
}, {
  kind: "htmlBlock",
  icon: "i-lucide-code-xml",
  tooltip: { text: "Inserir HTML" }
}], [{
  icon: "i-lucide-align-justify",
  tooltip: { text: "Alinhamento" },
  content: { align: "end" },
  items: [
    { kind: "textAlign", align: "left", icon: "i-lucide-align-left", label: "Esquerda" },
    { kind: "textAlign", align: "center", icon: "i-lucide-align-center", label: "Centro" },
    { kind: "textAlign", align: "right", icon: "i-lucide-align-right", label: "Direita" },
    { kind: "textAlign", align: "justify", icon: "i-lucide-align-justify", label: "Justificado" }
  ]
}]] satisfies EditorToolbarItem<typeof customHandlers>[][];

const bubbleToolbarItems = computed(() => [[{
  icon: "i-lucide-sparkles",
  label: "AI",
  activeColor: "neutral",
  activeVariant: "ghost",
  content: { align: "start" },
  items: [
    { kind: "aiContinue", icon: "i-lucide-sparkles", label: "Continue writing" }
  ]
}], [{
  label: "Turn into",
  trailingIcon: "i-lucide-chevron-down",
  activeColor: "neutral",
  activeVariant: "ghost",
  content: { align: "start" },
  items: [
    { type: "label", label: "Turn into" },
    { kind: "paragraph", label: "Paragraph", icon: "i-lucide-type" },
    { kind: "heading", level: 1, label: "Heading 1", icon: "i-lucide-heading-1" },
    { kind: "heading", level: 2, label: "Heading 2", icon: "i-lucide-heading-2" },
    { kind: "heading", level: 3, label: "Heading 3", icon: "i-lucide-heading-3" },
    { kind: "bulletList", label: "Bullet list", icon: "i-lucide-list" },
    { kind: "orderedList", label: "Numbered list", icon: "i-lucide-list-ordered" },
    { kind: "blockquote", label: "Blockquote", icon: "i-lucide-text-quote" },
    { kind: "codeBlock", label: "Code block", icon: "i-lucide-square-code" }
  ]
}], [{
  kind: "mark",
  mark: "bold",
  icon: "i-lucide-bold",
  tooltip: { text: "Negrito" }
}, {
  kind: "mark",
  mark: "italic",
  icon: "i-lucide-italic",
  tooltip: { text: "Italico" }
}, {
  kind: "mark",
  mark: "underline",
  icon: "i-lucide-underline",
  tooltip: { text: "Sublinhado" }
}, {
  kind: "mark",
  mark: "strike",
  icon: "i-lucide-strikethrough",
  tooltip: { text: "Riscado" }
}, {
  kind: "mark",
  mark: "code",
  icon: "i-lucide-code",
  tooltip: { text: "Codigo inline" }
}], [{
  slot: "link",
  icon: "i-lucide-link",
  tooltip: { text: "Link" }
}, {
  slot: "image",
  icon: "i-lucide-image",
  tooltip: { text: "Imagem" }
}]] satisfies EditorToolbarItem<typeof customHandlers>[][]);

const suggestionItems = [[{
  type: "label",
  label: "AI"
}, {
  kind: "aiContinue",
  label: "Continue writing",
  icon: "i-lucide-sparkles"
}], [{
  type: "label",
  label: "Style"
}, {
  kind: "paragraph",
  label: "Paragraph",
  icon: "i-lucide-type"
}, {
  kind: "heading",
  level: 1,
  label: "Heading 1",
  icon: "i-lucide-heading-1"
}, {
  kind: "heading",
  level: 2,
  label: "Heading 2",
  icon: "i-lucide-heading-2"
}, {
  kind: "heading",
  level: 3,
  label: "Heading 3",
  icon: "i-lucide-heading-3"
}, {
  kind: "bulletList",
  label: "Bullet list",
  icon: "i-lucide-list"
}, {
  kind: "orderedList",
  label: "Numbered list",
  icon: "i-lucide-list-ordered"
}, {
  kind: "blockquote",
  label: "Blockquote",
  icon: "i-lucide-text-quote"
}, {
  kind: "codeBlock",
  label: "Code block",
  icon: "i-lucide-square-code"
}], [{
  type: "label",
  label: "Insert"
}, {
  kind: "mention",
  label: "Mention person",
  icon: "i-lucide-at-sign"
}, {
  kind: "emoji",
  label: "Emoji",
  icon: "i-lucide-smile-plus"
}, {
  kind: "image",
  label: "Image URL",
  icon: "i-lucide-image"
}, {
  kind: "htmlBlock",
  label: "HTML",
  icon: "i-lucide-code-xml"
}, {
  kind: "horizontalRule",
  label: "Divider",
  icon: "i-lucide-separator-horizontal"
}]] satisfies EditorSuggestionMenuItem<typeof customHandlers>[][];

function handleItems(editor: Editor): DropdownMenuItem[][] {
  if (!selectedNode.value?.node?.type) return [];

  return mapEditorItems(editor, [[
    { type: "label", label: nodeLabel(selectedNode.value.node.type) },
    {
      label: "Turn into",
      icon: "i-lucide-repeat-2",
      children: [
        { kind: "paragraph", label: "Paragraph", icon: "i-lucide-type" },
        { kind: "heading", level: 1, label: "Heading 1", icon: "i-lucide-heading-1" },
        { kind: "heading", level: 2, label: "Heading 2", icon: "i-lucide-heading-2" },
        { kind: "heading", level: 3, label: "Heading 3", icon: "i-lucide-heading-3" },
        { kind: "bulletList", label: "Bullet list", icon: "i-lucide-list" },
        { kind: "orderedList", label: "Numbered list", icon: "i-lucide-list-ordered" },
        { kind: "blockquote", label: "Blockquote", icon: "i-lucide-text-quote" },
        { kind: "codeBlock", label: "Code block", icon: "i-lucide-square-code" }
      ]
    },
    {
      kind: "clearFormatting",
      pos: selectedNode.value.pos,
      label: "Reset formatting",
      icon: "i-lucide-rotate-ccw"
    }
  ], [
    {
      kind: "duplicate",
      pos: selectedNode.value.pos,
      label: "Duplicate",
      icon: "i-lucide-copy"
    },
    {
      label: "Copy text",
      icon: "i-lucide-clipboard",
      onSelect: async () => {
        if (!selectedNode.value || !import.meta.client) return;
        const node = editor.state.doc.nodeAt(selectedNode.value.pos);
        if (node) await navigator.clipboard.writeText(node.textContent);
      }
    }
  ], [
    {
      kind: "moveUp",
      pos: selectedNode.value.pos,
      label: "Move up",
      icon: "i-lucide-arrow-up"
    },
    {
      kind: "moveDown",
      pos: selectedNode.value.pos,
      label: "Move down",
      icon: "i-lucide-arrow-down"
    }
  ], [
    {
      kind: "delete",
      pos: selectedNode.value.pos,
      label: "Delete",
      icon: "i-lucide-trash"
    }
  ]], customHandlers) as DropdownMenuItem[][];
}

function applyLink(editor: Editor) {
  const href = linkDraft.value.trim();
  if (!href) {
    editor.chain().focus().extendMarkRange("link").unsetLink().run();
    return;
  }

  editor.chain().focus().extendMarkRange("link").setLink({ href }).run();
  linkDraft.value = "";
}

function prepareLink(editor: Editor, open: boolean) {
  if (!open) return;
  linkDraft.value = String(editor.getAttributes("link")?.href || "");
}

function insertImageUrl(editor: Editor) {
  const src = imageDraft.value.trim();
  if (!src) return;
  editor.chain().focus().setImage({ src }).run();
  imageDraft.value = "";
}

function openImageFilePicker(editor: Editor) {
  activeImageEditor.value = editor;
  imageInputRef.value?.click();
}

function onImageFileChange(event: Event) {
  const input = event.target as HTMLInputElement | null;
  const file = input?.files?.[0];
  const editor = activeImageEditor.value;
  if (!file || !editor) return;

  const reader = new FileReader();
  reader.onload = () => {
    const src = String(reader.result || "");
    if (src) editor.chain().focus().setImage({ src }).run();
  };
  reader.readAsDataURL(file);

  if (input) input.value = "";
  activeImageEditor.value = null;
}
</script>

<template>
  <div class="omni-editor" :style="editorStyle">
    <UEditor
      v-model="editorValue"
      v-slot="{ editor, handlers }"
      :content-type="contentType"
      :extensions="[Emoji, TextAlign.configure({ types: ['heading', 'paragraph'] })]"
      :handlers="customHandlers"
      :placeholder="{ placeholder, mode: 'everyLine', includeChildren: true }"
      :image="{ allowBase64: true, HTMLAttributes: { class: 'omni-editor__image' } }"
      :ui="{ base: 'omni-editor__content' }"
      class="omni-editor__instance"
    >
      <UEditorToolbar
        :editor="editor"
        :items="fixedToolbarItems"
        class="omni-editor__toolbar"
      >
        <template #link>
          <UPopover :content="{ side: 'bottom', align: 'start' }" @update:open="prepareLink(editor, $event)">
            <UButton icon="i-lucide-link" color="neutral" variant="ghost" size="sm" :active="editor.isActive('link')" />
            <template #content>
              <div class="omni-editor__popover">
                <UInput v-model="linkDraft" placeholder="https://..." size="sm" @keydown.enter.prevent="applyLink(editor)" />
                <div class="omni-editor__popover-actions">
                  <UButton label="Aplicar" icon="i-lucide-check" color="primary" size="sm" @click="applyLink(editor)" />
                  <UButton label="Remover" icon="i-lucide-unlink" color="neutral" variant="soft" size="sm" @click="() => { linkDraft = ''; applyLink(editor) }" />
                </div>
              </div>
            </template>
          </UPopover>
        </template>

        <template #image>
          <UPopover :content="{ side: 'bottom', align: 'start' }">
            <UButton icon="i-lucide-image" color="neutral" variant="ghost" size="sm" :active="editor.isActive('image')" />
            <template #content>
              <div class="omni-editor__popover">
                <UInput v-model="imageDraft" placeholder="URL da imagem" size="sm" @keydown.enter.prevent="insertImageUrl(editor)" />
                <div class="omni-editor__popover-actions">
                  <UButton label="Inserir URL" icon="i-lucide-check" color="primary" size="sm" @click="insertImageUrl(editor)" />
                  <UButton label="Upload" icon="i-lucide-upload" color="neutral" variant="soft" size="sm" @click="openImageFilePicker(editor)" />
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
        :should-show="({ view, state }) => view.hasFocus() && !state.selection.empty && !editor.isActive('image')"
      >
        <template #link>
          <UPopover :content="{ side: 'bottom', align: 'start' }" @update:open="prepareLink(editor, $event)">
            <UButton icon="i-lucide-link" color="neutral" variant="ghost" size="sm" :active="editor.isActive('link')" />
            <template #content>
              <div class="omni-editor__popover">
                <UInput v-model="linkDraft" placeholder="https://..." size="sm" @keydown.enter.prevent="applyLink(editor)" />
                <div class="omni-editor__popover-actions">
                  <UButton label="Aplicar" icon="i-lucide-check" color="primary" size="sm" @click="applyLink(editor)" />
                  <UButton label="Remover" icon="i-lucide-unlink" color="neutral" variant="soft" size="sm" @click="() => { linkDraft = ''; applyLink(editor) }" />
                </div>
              </div>
            </template>
          </UPopover>
        </template>

        <template #image>
          <UButton icon="i-lucide-image" color="neutral" variant="ghost" size="sm" @click="openImageFilePicker(editor)" />
        </template>
      </UEditorToolbar>

      <UEditorDragHandle v-slot="{ ui, onClick }" :editor="editor" @node-change="selectedNode = $event">
        <UButton
          icon="i-lucide-plus"
          color="neutral"
          variant="ghost"
          size="sm"
          :class="ui.handle()"
          @click="(event: MouseEvent) => {
            event.stopPropagation();
            const selected = onClick();
            handlers.suggestion?.execute(editor, { pos: selected?.pos }).run();
          }"
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
        :options="{ placement: 'bottom-start', offset: 6 }"
        :ui="{ content: 'z-[10020]' }"
      />
      <UEditorMentionMenu
        :editor="editor"
        :items="userMentionItems"
        :options="{ placement: 'bottom-start', offset: 6 }"
        :ui="{ content: 'z-[10020]' }"
      />
      <UEditorMentionMenu
        v-if="entityMentionItems.length"
        :editor="editor"
        :items="entityMentionItems"
        char="#"
        plugin-key="entityMentionMenu"
        :options="{ placement: 'bottom-start', offset: 6 }"
        :ui="{ content: 'z-[10020]' }"
      />
      <UEditorEmojiMenu
        :editor="editor"
        :items="emojiItems"
        :options="{ placement: 'bottom-start', offset: 6 }"
        :ui="{ content: 'z-[10020]' }"
      />
    </UEditor>

    <input ref="imageInputRef" type="file" accept="image/*" class="omni-editor__file-input" @change="onImageFileChange">
  </div>
</template>

<style scoped>
.omni-editor {
  position: relative;
  max-height: var(--omni-editor-max-height);
  min-height: var(--omni-editor-min-height);
  overflow-y: auto;
  overflow-x: hidden;
  border-top: 1px solid rgb(var(--border));
  color: rgb(var(--text));
  scrollbar-gutter: stable;
}

.omni-editor__instance {
  min-height: var(--omni-editor-min-height);
}

.omni-editor__toolbar {
  position: sticky;
  top: 0;
  z-index: 25;
  border-bottom: 1px solid rgb(var(--border));
  background: rgb(var(--surface));
  padding: 0.45rem 0.15rem;
  overflow-x: auto;
  scrollbar-width: none;
}

.omni-editor__toolbar::-webkit-scrollbar {
  display: none;
}

.omni-editor :deep(.omni-editor__content) {
  min-height: var(--omni-editor-min-height);
  padding: 1.4rem 2.2rem 5rem 2.55rem;
  outline: none;
  line-height: 1.75;
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

.omni-editor :deep(.tiptap [data-type="mention"]) {
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
