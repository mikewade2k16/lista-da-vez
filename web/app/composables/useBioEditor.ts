import { computed, onBeforeUnmount, ref, watch } from 'vue'

import { useBioStore } from '~/stores/bio'
import type { BioData, BioDetail, BioMediaKind } from '~/domain/bio/types'

// Editor de uma bio: carrega o detalhe, mantem um draft local (clone profundo
// do dataDraft), faz dirty-check contra o snapshot salvo e persiste via PATCH
// enviando o dataDraft INTEIRO (semantica do contrato: dataDraft substitui o
// draft completo). Upload de midia grava a URL retornada direto no draft.
//
// As secoes editam o draft por referencia (o objeto retornado em `draft` e
// reativo); o dirty-check compara JSON do draft com o snapshot salvo.
//
// AUTO-SAVE: um watch profundo no draft+name+slug salva sozinho (debounce
// ~800ms) quando ha alteracoes — nao ha botao "Salvar" manual. UNDO: cada
// mudanca estavel empilha um snapshot (limite ~50); undo() restaura o anterior.

const AUTOSAVE_DEBOUNCE_MS = 800
const SNAPSHOT_DEBOUNCE_MS = 500
const MAX_SNAPSHOTS = 50

type SaveState = 'idle' | 'saving' | 'saved' | 'error'

interface DraftSnapshot {
  data: string
  name: string
  slug: string
}

function deepClone<T>(value: T): T {
  return JSON.parse(JSON.stringify(value ?? null)) as T
}

function stableStringify(value: BioData): string {
  // JSON.stringify ja e estavel o suficiente para dirty-check porque o draft e
  // sempre derivado da mesma fonte (clone do dataDraft + edicoes nas mesmas
  // chaves). Nao precisa de ordenacao de chaves aqui.
  return JSON.stringify(value ?? {})
}

export function useBioEditor() {
  const store = useBioStore()

  const draft = ref<BioData>({})
  const savedSnapshot = ref<string>('{}')
  // Snapshot do que esta PUBLICADO (data_published). Comparado com o draft salvo
  // para sinalizar "ha alteracoes salvas mas ainda nao publicadas". Ambos vem do
  // back (mesma serializacao jsonb), entao a comparacao por string e estavel.
  const publishedSnapshot = ref<string>('{}')
  const name = ref('')
  const slug = ref('')
  const savedName = ref('')
  const savedSlug = ref('')

  const loading = ref(false)
  const saving = ref(false)
  const errorMessage = ref('')
  const savedAt = ref(0)
  const saveState = ref<SaveState>('idle')

  const uploading = ref<Record<string, boolean>>({})

  // Id da bio ativa (setado no load). O auto-save usa este id; assim as secoes
  // continuam editando o draft sem precisar passar o id a cada keystroke.
  const activeId = ref('')

  // Pilha de undo: snapshots do estado estavel do draft+name+slug. O topo e o
  // estado atual; undo() remove o topo e restaura o anterior. `suppress*` evita
  // que a propria restauracao gere novos snapshots/saves em cascata.
  const undoStack = ref<DraftSnapshot[]>([])
  const canUndo = computed(() => undoStack.value.length > 1)
  let suppressTracking = false

  let autosaveTimer: ReturnType<typeof setTimeout> | null = null
  let snapshotTimer: ReturnType<typeof setTimeout> | null = null

  function currentSnapshot(): DraftSnapshot {
    return {
      data: stableStringify(draft.value),
      name: name.value.trim(),
      slug: slug.value.trim(),
    }
  }

  function pushSnapshot() {
    const snap = currentSnapshot()
    const top = undoStack.value[undoStack.value.length - 1]
    if (top && top.data === snap.data && top.name === snap.name && top.slug === snap.slug) {
      return
    }
    undoStack.value.push(snap)
    if (undoStack.value.length > MAX_SNAPSHOTS) {
      undoStack.value.splice(0, undoStack.value.length - MAX_SNAPSHOTS)
    }
  }

  function resetUndo() {
    undoStack.value = [currentSnapshot()]
  }

  const dirty = computed(
    () =>
      stableStringify(draft.value) !== savedSnapshot.value ||
      name.value.trim() !== savedName.value ||
      slug.value.trim() !== savedSlug.value,
  )

  // True quando o rascunho salvo difere do publicado: o usuario precisa
  // (Re)publicar para o front publico refletir. So relevante quando publicada.
  const hasUnpublishedChanges = computed(() => savedSnapshot.value !== publishedSnapshot.value)

  function hydrateFromDetail(detail: BioDetail) {
    // A hidratacao (load/save/publish) substitui o estado todo; suprime o
    // tracking para nao empilhar snapshot nem disparar auto-save por isso.
    suppressTracking = true
    draft.value = deepClone(detail.dataDraft || {})
    savedSnapshot.value = stableStringify(draft.value)
    publishedSnapshot.value = stableStringify(deepClone(detail.dataPublished || {}))
    name.value = detail.name || ''
    slug.value = detail.slug || ''
    savedName.value = name.value.trim()
    savedSlug.value = slug.value.trim()
  }

  async function load(id: string) {
    loading.value = true
    errorMessage.value = ''
    activeId.value = String(id || '').trim()
    try {
      const detail = await store.loadBio(id)
      if (!detail) {
        errorMessage.value = store.detailError || 'Bio nao encontrada.'
        return null
      }
      hydrateFromDetail(detail)
      saveState.value = 'idle'
      resetUndo()
      return detail
    } finally {
      loading.value = false
    }
  }

  async function save(id?: string): Promise<{ ok: boolean; message: string }> {
    const bioId = String(id || activeId.value || '').trim()
    if (saving.value || !dirty.value || !bioId) {
      return { ok: true, message: '' }
    }

    saving.value = true
    saveState.value = 'saving'
    errorMessage.value = ''
    try {
      const result = await store.patchBio(bioId, {
        name: name.value.trim(),
        slug: slug.value.trim(),
        dataDraft: deepClone(draft.value),
      })
      if (!result.ok) {
        errorMessage.value = result.message
        saveState.value = 'error'
        return { ok: false, message: result.message }
      }
      hydrateFromDetail(result.bio)
      savedAt.value = Date.now()
      saveState.value = 'saved'
      return { ok: true, message: '' }
    } finally {
      saving.value = false
    }
  }

  // Salva agora se ja houver debounce pendente (usado antes de publicar). Garante
  // que o draft no servidor reflita o que esta na tela antes do publish.
  async function flushSave(id?: string): Promise<{ ok: boolean; message: string }> {
    if (autosaveTimer) {
      clearTimeout(autosaveTimer)
      autosaveTimer = null
    }
    return save(id)
  }

  async function uploadMedia(id: string, kind: BioMediaKind, file: File): Promise<string | null> {
    if (!file) {
      return null
    }

    uploading.value = { ...uploading.value, [kind]: true }
    errorMessage.value = ''
    try {
      const result = await store.uploadMedia(id, kind, file)
      return result?.url ?? null
    } catch (caught) {
      errorMessage.value =
        (caught as { data?: { error?: { message?: string } } })?.data?.error?.message ||
        'Nao foi possivel enviar o arquivo.'
      return null
    } finally {
      uploading.value = { ...uploading.value, [kind]: false }
    }
  }

  function isUploading(kind: BioMediaKind): boolean {
    return Boolean(uploading.value[kind])
  }

  // Restaura o estado anterior da pilha de undo. Marca dirty e deixa o auto-save
  // persistir o resultado (sem botao Salvar). Suprime o tracking durante a
  // restauracao para nao re-empilhar o estado que acabou de sair.
  function undo() {
    if (!canUndo.value) {
      return
    }
    undoStack.value.pop()
    const target = undoStack.value[undoStack.value.length - 1]
    if (!target) {
      return
    }
    suppressTracking = true
    draft.value = deepClone(JSON.parse(target.data) as BioData)
    name.value = target.name
    slug.value = target.slug
  }

  // Watcher unico: a cada mudanca estavel do draft+name+slug, agenda (1) um
  // snapshot de undo e (2) o auto-save debounced. `suppressTracking` cobre as
  // mutacoes do proprio editor (hydrate/undo) — consumido uma vez por flush do
  // microtask para nao "comer" edicoes legitimas que venham logo depois.
  watch(
    [draft, name, slug],
    () => {
      if (suppressTracking) {
        suppressTracking = false
        return
      }

      if (snapshotTimer) {
        clearTimeout(snapshotTimer)
      }
      snapshotTimer = setTimeout(pushSnapshot, SNAPSHOT_DEBOUNCE_MS)

      if (saveState.value === 'saved' || saveState.value === 'error') {
        saveState.value = 'idle'
      }
      if (autosaveTimer) {
        clearTimeout(autosaveTimer)
      }
      autosaveTimer = setTimeout(() => {
        autosaveTimer = null
        if (dirty.value) {
          void save()
        }
      }, AUTOSAVE_DEBOUNCE_MS)
    },
    { deep: true },
  )

  onBeforeUnmount(() => {
    if (autosaveTimer) {
      clearTimeout(autosaveTimer)
    }
    if (snapshotTimer) {
      clearTimeout(snapshotTimer)
    }
  })

  return {
    draft,
    name,
    slug,
    loading,
    saving,
    saveState,
    dirty,
    hasUnpublishedChanges,
    canUndo,
    errorMessage,
    savedAt,
    load,
    save,
    flushSave,
    undo,
    uploadMedia,
    isUploading,
  }
}
