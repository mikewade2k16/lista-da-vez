<script setup lang="ts">
import type Quill from 'quill'
import { Eye, EyeOff, FileDown, GripVertical, Trash2, X } from 'lucide-vue-next'
import ScriptQuillCell from '~/components/tools/ScriptQuillCell.vue'
import ScriptFloatingToolbar from '~/components/tools/ScriptFloatingToolbar.vue'
import ScriptSortable from '~/components/tools/ScriptSortable.vue'
import ScriptSidebar from '~/components/tools/ScriptSidebar.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import { useScriptWorkspace } from '~/composables/useScriptWorkspace'
import { scriptImagesKey, useScriptImages } from '~/composables/useScriptImages'
import { exportScriptPdf } from '~/composables/useScriptPdf'
import type { ScriptRow } from '~/composables/useScriptsManager'
import 'quill/dist/quill.snow.css'
import '@fontsource/courier-prime/400.css'
import '@fontsource/courier-prime/700.css'
import '@fontsource/courier-prime/400-italic.css'

const {
  remoteNotice,
  reloadOpened,
  documents,
  search,
  clientFilter,
  clients,
  shared,
  canEdit,
  accountId,
  canView,
  canManage,
  loading,
  opening,
  saving,
  error,
  hasMore,
  load,
  selectedId,
  editing,
  draft,
  dirty,
  saveNow,
  maySwitch,
  select,
  create,
  removeDocument,
  moveRow,
  moveDocument,
  hasOtherDrafts,
} = useScriptWorkspace()
const ui = useUiStore()
const clientEditOptions = computed(() => [
  { value: '', label: 'Sem cliente' },
  ...clients.value.map((client) => ({ value: client.id, label: client.name })),
])
const images = useScriptImages(accountId)
provide(scriptImagesKey, images)
const paper = ref<HTMLElement>()
const activeEditor = shallowRef<Quill>()
const activeHost = shallowRef<HTMLElement>()
const uploading = ref(false)
const exporting = ref(false)
const toolbarVisible = ref(false)
const rowKeys = new WeakMap<ScriptRow, number>()
let rowSequence = 0
function rowKey(row: ScriptRow) {
  if (!rowKeys.has(row)) rowKeys.set(row, ++rowSequence)
  return rowKeys.get(row)
}
// The toolbar only shows for a text selection (or an empty line, to insert images) and hides
// itself after a few seconds unless the pointer is over it, so it never sits on top of text.
const TOOLBAR_IDLE_MS = 3000
let toolbarTimer: ReturnType<typeof setTimeout> | undefined
let toolbarHovered = false
function hideToolbarLater() {
  clearTimeout(toolbarTimer)
  if (!toolbarHovered)
    toolbarTimer = setTimeout(() => (toolbarVisible.value = false), TOOLBAR_IDLE_MS)
}
function activate(editor: Quill, host: HTMLElement) {
  activeEditor.value = editor
  activeHost.value = host
}
function selectionChanged(editor: Quill, wanted: boolean) {
  if (activeEditor.value !== editor) return
  if (!toolbarVisible.value) toolbarHovered = false
  toolbarVisible.value = wanted
  if (wanted) hideToolbarLater()
  else clearTimeout(toolbarTimer)
}
function toolbarHover(inside: boolean) {
  toolbarHovered = inside
  if (inside) clearTimeout(toolbarTimer)
  else hideToolbarLater()
}
watch(selectedId, () => {
  activeEditor.value = undefined
  activeHost.value = undefined
  toolbarVisible.value = false
})
function deactivate(editor: Quill) {
  if (activeEditor.value === editor) {
    activeEditor.value = undefined
    activeHost.value = undefined
  }
}
function dismissToolbar(event: PointerEvent) {
  if (!(event.target instanceof Node) || !activeHost.value?.contains(event.target))
    toolbarVisible.value = false
}
function keyboard(event: KeyboardEvent) {
  if (event.key === 'Escape') toolbarVisible.value = false
}
// The contenteditable title is seeded only when the document changes; binding draft.title
// reactively would rewrite the text node on every keystroke and move the caret to the start.
const titleSeed = ref(draft.value.title)
watch(selectedId, () => (titleSeed.value = draft.value.title), { immediate: true })
function titleInput(event: Event) {
  draft.value.title = (event.target as HTMLElement).textContent?.slice(0, 200) || ''
}
function titlePaste(event: ClipboardEvent) {
  event.preventDefault()
  const text = event.clipboardData?.getData('text/plain').replace(/\s+/g, ' ') || ''
  document.execCommand('insertText', false, text)
}
async function upload(editor: Quill) {
  const input = document.createElement('input')
  input.type = 'file'
  input.accept = 'image/png,image/jpeg,image/webp,image/gif'
  const range = editor.getSelection(true) || { index: editor.getLength(), length: 0 }
  input.onchange = async () => {
    const file = input.files?.[0]
    if (!file) return
    uploading.value = true
    try {
      const url = await images.upload(file)
      if (activeEditor.value !== editor) return
      editor.insertEmbed(range.index, 'image', url, 'user')
      editor.setSelection(range.index + 1, 0, 'user')
    } catch (err) {
      ui.error(err instanceof Error ? err.message : 'Não foi possível enviar a imagem.')
    } finally {
      uploading.value = false
    }
  }
  input.click()
}
async function downloadPdf() {
  if (!paper.value || exporting.value) return
  exporting.value = true
  toolbarVisible.value = false
  try {
    await exportScriptPdf(paper.value, draft.value.title)
  } catch {
    ui.error('Não foi possível gerar o PDF. Tente novamente.')
  } finally {
    exporting.value = false
  }
}
function beforeUnload(event: BeforeUnloadEvent) {
  if (dirty.value || saving.value || hasOtherDrafts()) {
    event.preventDefault()
    event.returnValue = ''
  }
}
onBeforeRouteLeave(async () => {
  if (saving.value) await saveNow()
  if (!(await maySwitch())) return false
  if (!hasOtherDrafts()) return true
  const result = await ui.confirm({
    title: 'Rascunhos de outras contas',
    message: 'Descartar os rascunhos não salvos das outras contas?',
    confirmLabel: 'Descartar',
  })
  return Boolean(result && typeof result === 'object' && 'confirmed' in result && result.confirmed)
})
watch(accountId, () => {
  images.dispose()
  activeEditor.value = undefined
  activeHost.value = undefined
  toolbarVisible.value = false
})
onMounted(() => {
  window.addEventListener('beforeunload', beforeUnload)
  document.addEventListener('pointerdown', dismissToolbar)
  document.addEventListener('keydown', keyboard)
})
onBeforeUnmount(() => {
  clearTimeout(toolbarTimer)
  images.dispose()
  window.removeEventListener('beforeunload', beforeUnload)
  document.removeEventListener('pointerdown', dismissToolbar)
  document.removeEventListener('keydown', keyboard)
})
</script>

<template>
  <section class="page-script">
    <div class="scripts-layout">
      <ScriptSidebar
        v-model:search="search"
        v-model:client-filter="clientFilter"
        :documents="documents"
        :clients="clients"
        :selected-id="selectedId"
        :account-id="accountId"
        :can-view="canView"
        :can-manage="canManage"
        :loading="loading"
        :saving="saving"
        :opening="opening"
        :has-more="hasMore"
        @create="create"
        @select="select"
        @remove="removeDocument"
        @move="moveDocument"
        @load-more="load(true)"
      />
      <main class="scripts-content">
        <div v-if="error" class="scripts-error" role="alert">
          {{ error }}
          <button type="button" @click="dirty ? saveNow() : load()">Tentar novamente</button>
        </div>
        <div v-if="remoteNotice === 'changed'" class="scripts-error" role="status">
          Este roteiro foi alterado em outra sessão.
          <button type="button" @click="reloadOpened">Recarregar (descarta suas alterações)</button>
        </div>
        <p v-if="remoteNotice === 'deleted'" class="scripts-notice" role="status">
          O roteiro aberto foi excluído em outra sessão.
        </p>
        <p v-if="opening" class="scripts-notice">Abrindo documento…</p>
        <p v-if="!editing" class="scripts-notice">
          Selecione um documento ou clique em + para criar um roteiro.
        </p>
        <div v-else class="content">
          <p v-if="shared" class="scripts-notice script-shared-notice">
            Roteiro compartilhado com você pela agência. Somente leitura.
          </p>
          <article ref="paper" class="document-script">
            <!-- Canto da folha, fora do PDF (data-html2canvas-ignore + lista do useScriptPdf). -->
            <div
              v-if="canEdit && clients.length"
              class="script-client-corner"
              data-html2canvas-ignore
            >
              <AppSelectField
                v-model="draft.clientAccountId"
                class="script-client-select"
                :options="clientEditOptions"
                :show-leading-icon="false"
                compact
                searchable
                placeholder="Sem cliente"
                search-placeholder="Buscar cliente"
              />
              <button
                v-if="draft.clientAccountId"
                type="button"
                class="script-icon-button script-client-visibility"
                :class="{ 'is-hidden': !draft.clientVisible }"
                :title="draft.clientVisible ? 'Visível para o cliente' : 'Oculto do cliente'"
                :aria-label="draft.clientVisible ? 'Visível para o cliente' : 'Oculto do cliente'"
                :aria-pressed="draft.clientVisible"
                @click="draft.clientVisible = !draft.clientVisible"
              >
                <Eye v-if="draft.clientVisible" :size="15" />
                <EyeOff v-else :size="15" />
              </button>
            </div>
            <div class="document-heading">
              <h1
                :key="selectedId"
                class="script-title"
                :contenteditable="canEdit ? 'true' : 'false'"
                spellcheck="false"
                role="textbox"
                aria-label="Título do roteiro"
                @input="titleInput"
                @paste="titlePaste"
                @keydown.enter.prevent
                v-text="titleSeed"
              ></h1>
              <div class="document-actions">
                <button
                  type="button"
                  class="script-icon-button"
                  title="Baixar PDF"
                  :disabled="exporting"
                  @click="downloadPdf"
                >
                  <FileDown :size="16" />
                </button>
                <button
                  type="button"
                  class="script-icon-button"
                  title="Excluir este documento"
                  :disabled="!canEdit || saving"
                  @click="removeDocument(selectedId)"
                >
                  <Trash2 :size="16" />
                </button>
              </div>
            </div>
            <table class="script-table">
              <thead>
                <tr>
                  <td><strong>Áudio</strong></td>
                  <td><strong>Vídeo</strong></td>
                </tr>
              </thead>
              <ScriptSortable
                tag="tbody"
                handle=".row-drag"
                :disabled="!canEdit || exporting"
                @move="moveRow"
              >
                <tr v-for="(row, index) in draft.rows" :key="rowKey(row)" class="script-row">
                  <td class="cell cell-audio">
                    <button
                      v-if="canEdit"
                      type="button"
                      class="script-icon-button row-drag"
                      title="Reordenar linha"
                      aria-label="Reordenar linha"
                    >
                      <GripVertical :size="18" />
                    </button>
                    <button
                      v-if="canEdit"
                      type="button"
                      class="script-icon-button row-remove"
                      title="Excluir linha"
                      aria-label="Excluir linha"
                      @click="draft.rows.splice(index, 1)"
                    >
                      <X :size="18" />
                    </button>
                    <ScriptQuillCell
                      v-model="row.audio"
                      :editable="canEdit && !exporting"
                      @activate="activate"
                      @selection="selectionChanged"
                      @deactivate="deactivate"
                    />
                  </td>
                  <td class="cell cell-video">
                    <ScriptQuillCell
                      v-model="row.video"
                      :editable="canEdit && !exporting"
                      @activate="activate"
                      @selection="selectionChanged"
                      @deactivate="deactivate"
                    />
                  </td>
                </tr>
              </ScriptSortable>
            </table>
            <button
              v-if="canEdit"
              type="button"
              class="add-row"
              :disabled="draft.rows.length >= 200 || exporting"
              @click="draft.rows.push({ audio: '', video: '' })"
            >
              + Linha
            </button>
            <div class="doc-editor">
              <ScriptQuillCell
                v-model="draft.description"
                :editable="canEdit && !exporting"
                @activate="activate"
                @selection="selectionChanged"
                @deactivate="deactivate"
              />
            </div>
          </article>
          <p class="scripts-save-status" role="status">
            {{ saving ? 'Salvando…' : dirty ? 'Alterações não salvas' : 'Salvo' }}
          </p>
        </div>
      </main>
    </div>
    <ScriptFloatingToolbar
      v-if="canEdit && toolbarVisible && activeEditor && activeHost"
      :editor="activeEditor"
      :host="activeHost"
      :uploading="uploading"
      @image="upload"
      @hover="toolbarHover"
    />
  </section>
</template>
<style src="./scripts-workspace.css" />
