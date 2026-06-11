<script setup lang="ts">
import { useKnowledgeDocs, type KnowledgeDocView } from '~/composables/useKnowledgeDocs'

const emit = defineEmits<{ (e: 'change'): void }>()
const { docs, loading, globalError, loadDocs, createDoc, patchDoc, toggleEnabled, deleteDoc } =
  useKnowledgeDocs()

// Estado local por doc: { title, body, saving, saved, error }
interface LocalState {
  title: string
  body: string
  saving: boolean
  saved: boolean
  error: string
}
const local = ref<Record<string, LocalState>>({})

function initLocal(doc: KnowledgeDocView) {
  if (!local.value[doc.id]) {
    local.value[doc.id] = {
      title: doc.title,
      body: doc.body,
      saving: false,
      saved: false,
      error: '',
    }
  }
}

watch(docs, (list) => list.forEach(initLocal), { immediate: true })

async function save(doc: KnowledgeDocView) {
  const s = local.value[doc.id]
  if (!s || !s.body.trim()) {
    s.error = 'O conteudo nao pode ficar vazio.'
    return
  }
  s.saving = true
  s.error = ''
  const updated = await patchDoc(doc.id, s.title, s.body, doc.sortOrder, doc.enabled)
  s.saving = false
  if (updated) {
    s.saved = true
    emit('change')
    setTimeout(() => {
      s.saved = false
    }, 2000)
  } else {
    s.error = 'Erro ao salvar.'
  }
}

async function remove(id: string) {
  if (!confirm('Apagar este documento?')) return
  delete local.value[id]
  await deleteDoc(id)
  emit('change')
}

// Novo documento
const showNew = ref(false)
const newTitle = ref('')
const newBody = ref('')
const newSaving = ref(false)
const newError = ref('')

async function saveNew() {
  if (!newBody.value.trim()) {
    newError.value = 'O conteudo nao pode ficar vazio.'
    return
  }
  newSaving.value = true
  newError.value = ''
  const doc = await createDoc(newTitle.value, newBody.value)
  newSaving.value = false
  if (doc) {
    showNew.value = false
    newTitle.value = ''
    newBody.value = ''
    emit('change')
  } else {
    newError.value = 'Erro ao criar o documento.'
  }
}

onMounted(() => void loadDocs())
</script>

<template>
  <section class="kd-section">
    <header class="kd-section__head">
      <div>
        <h2 class="kd-section__title">Conhecimento</h2>
        <p class="kd-section__subtitle">
          Cada documento e injetado separadamente no contexto do bot.
        </p>
      </div>
      <button v-if="!showNew" type="button" class="kd-add-btn" @click="showNew = true">
        + Novo documento
      </button>
    </header>

    <p v-if="globalError" class="kd-global-error">{{ globalError }}</p>
    <p v-if="loading" class="kd-muted">Carregando...</p>

    <!-- Card de novo documento -->
    <article v-if="showNew" class="kd-card kd-card--new">
      <header class="kd-card__head">
        <input
          v-model="newTitle"
          type="text"
          class="kd-card__title-input"
          placeholder="Nome do documento (ex.: Tabela de Precos)"
          autofocus
        />
        <button
          type="button"
          class="kd-icon-btn"
          title="Cancelar"
          @click="
            showNew = false
            newTitle = ''
            newBody = ''
          "
        >
          ✕
        </button>
      </header>
      <textarea
        v-model="newBody"
        class="kd-card__textarea"
        rows="12"
        spellcheck="false"
        placeholder="Conteudo do documento..."
      ></textarea>
      <p v-if="newError" class="kd-card__error">{{ newError }}</p>
      <footer class="kd-card__foot">
        <button type="button" class="kd-btn kd-btn--primary" :disabled="newSaving" @click="saveNew">
          {{ newSaving ? 'Criando...' : 'Criar documento' }}
        </button>
        <button
          type="button"
          class="kd-btn kd-btn--ghost"
          @click="
            showNew = false
            newTitle = ''
            newBody = ''
          "
        >
          Cancelar
        </button>
      </footer>
    </article>

    <!-- Um card por documento -->
    <article
      v-for="doc in docs"
      :key="doc.id"
      class="kd-card"
      :class="{ 'kd-card--off': !doc.enabled }"
    >
      <header class="kd-card__head">
        <input
          v-if="local[doc.id]"
          v-model="local[doc.id].title"
          type="text"
          class="kd-card__title-input"
          placeholder="Nome do documento"
        />
        <div class="kd-card__head-actions">
          <!-- Toggle habilitado -->
          <button
            type="button"
            class="kd-switch"
            :class="{ 'kd-switch--on': doc.enabled }"
            :title="doc.enabled ? 'Desabilitar' : 'Habilitar'"
            @click="toggleEnabled(doc).then(() => emit('change'))"
          >
            <span class="kd-switch__track"><span class="kd-switch__thumb"></span></span>
            <span class="kd-switch__label">{{ doc.enabled ? 'Ativo' : 'Inativo' }}</span>
          </button>
          <button
            type="button"
            class="kd-icon-btn kd-icon-btn--danger"
            title="Excluir"
            @click="remove(doc.id)"
          >
            ✕
          </button>
        </div>
      </header>

      <textarea
        v-if="local[doc.id]"
        v-model="local[doc.id].body"
        class="kd-card__textarea"
        rows="14"
        spellcheck="false"
      ></textarea>

      <p v-if="local[doc.id]?.error" class="kd-card__error">{{ local[doc.id].error }}</p>

      <footer class="kd-card__foot">
        <button
          type="button"
          class="kd-btn kd-btn--primary"
          :disabled="local[doc.id]?.saving"
          @click="save(doc)"
        >
          {{ local[doc.id]?.saving ? 'Salvando...' : 'Salvar' }}
        </button>
        <span v-if="local[doc.id]?.saved" class="kd-saved-badge">Salvo</span>
      </footer>
    </article>

    <p v-if="!loading && docs.length === 0 && !showNew" class="kd-muted">
      Nenhum documento ainda. Clique em "+ Novo documento" para adicionar.
    </p>
  </section>
</template>

<style scoped>
.kd-section {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.kd-section__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.kd-section__title {
  font-size: 1.05rem;
  font-weight: 600;
}

.kd-section__subtitle {
  font-size: 0.8rem;
  color: var(--text-muted);
  margin-top: 0.15rem;
}

.kd-add-btn {
  font-size: 0.85rem;
  font-weight: 600;
  padding: 0.35rem 0.85rem;
  border-radius: 0.5rem;
  border: 1px solid var(--line-soft);
  background: transparent;
  color: var(--text-main);
  cursor: pointer;
  flex-shrink: 0;
}

.kd-add-btn:hover {
  background: rgb(var(--border) / 0.3);
}

.kd-global-error {
  color: rgb(var(--danger));
  font-size: 0.85rem;
  background: rgb(var(--danger) / 0.1);
  padding: 0.5rem 0.75rem;
  border-radius: 0.4rem;
}

.kd-muted {
  font-size: 0.85rem;
  color: var(--text-muted);
}

/* Card individual */
.kd-card {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  padding: 1.25rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.9);
  box-shadow: var(--shadow-card);
}

.kd-card--new {
  border-style: dashed;
  border-color: rgb(var(--primary) / 0.4);
}

.kd-card--off {
  opacity: 0.65;
}

.kd-card__head {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.kd-card__title-input {
  flex: 1;
  font-size: 1rem;
  font-weight: 600;
  padding: 0.3rem 0.5rem;
  border: 1px solid transparent;
  border-radius: 0.35rem;
  background: transparent;
  color: var(--text-main);
  font-family: inherit;
}

.kd-card__title-input:focus {
  outline: none;
  border-color: rgb(var(--ring) / 0.4);
  background: rgb(var(--surface-2) / 0.6);
}

.kd-card__head-actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-shrink: 0;
}

.kd-card__textarea {
  width: 100%;
  padding: 0.6rem 0.75rem;
  border: 1px solid var(--line-soft);
  border-radius: 0.5rem;
  background: rgb(var(--surface-2) / 0.76);
  color: var(--text-main);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.84rem;
  line-height: 1.55;
  resize: vertical;
  min-height: 180px;
}

.kd-card__textarea:focus {
  outline: none;
  border-color: rgb(var(--ring) / 0.5);
  box-shadow: 0 0 0 3px rgb(var(--ring) / 0.14);
}

.kd-card__error {
  font-size: 0.82rem;
  color: rgb(var(--danger));
}

.kd-card__foot {
  display: flex;
  align-items: center;
  gap: 0.6rem;
}

/* Botoes */
.kd-btn {
  font-size: 0.88rem;
  font-weight: 600;
  padding: 0.45rem 1rem;
  border-radius: 0.45rem;
  cursor: pointer;
  border: 1px solid transparent;
}

.kd-btn:disabled {
  opacity: 0.6;
  cursor: progress;
}

.kd-btn--primary {
  background: linear-gradient(135deg, rgb(var(--primary)), rgb(var(--primary-600)));
  color: #fff;
}

.kd-btn--ghost {
  background: transparent;
  border-color: var(--line-soft);
  color: var(--text-main);
}

.kd-icon-btn {
  width: 28px;
  height: 28px;
  border-radius: 0.35rem;
  border: 1px solid var(--line-soft);
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 0.75rem;
  display: flex;
  align-items: center;
  justify-content: center;
}

.kd-icon-btn--danger {
  border-color: rgb(var(--danger) / 0.35);
  color: rgb(var(--danger));
}
.kd-icon-btn--danger:hover {
  background: rgb(var(--danger) / 0.1);
}

/* Toggle switch */
.kd-switch {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  background: transparent;
  border: none;
  cursor: pointer;
  padding: 0;
}

.kd-switch__track {
  width: 34px;
  height: 20px;
  border-radius: 999px;
  background: rgb(var(--border));
  position: relative;
  transition: background 0.15s ease;
  flex-shrink: 0;
}

.kd-switch--on .kd-switch__track {
  background: rgb(var(--primary));
}

.kd-switch__thumb {
  position: absolute;
  top: 2px;
  left: 2px;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: #fff;
  transition: transform 0.15s ease;
}

.kd-switch--on .kd-switch__thumb {
  transform: translateX(14px);
}

.kd-switch__label {
  font-size: 0.8rem;
  color: var(--text-muted);
}

.kd-saved-badge {
  font-size: 0.8rem;
  color: rgb(var(--success));
  font-weight: 600;
}
</style>
