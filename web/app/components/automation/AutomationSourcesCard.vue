<script setup lang="ts">
import { useAutomationSources } from '~/composables/useAutomationSources'

const {
  catalogEnabled,
  siteUrls,
  loading,
  saving,
  saved,
  errorMessage,
  loadSources,
  saveSources,
  addUrl,
  removeUrl,
} = useAutomationSources()

const newUrlInput = ref('')

function handleAddUrl() {
  const val = newUrlInput.value.trim()
  if (!val) return
  addUrl(val)
  newUrlInput.value = ''
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Enter') {
    event.preventDefault()
    handleAddUrl()
  }
}

onMounted(() => void loadSources())
</script>

<template>
  <article class="asc-card">
    <header class="asc-card__head">
      <div>
        <h2 class="asc-card__title">Fontes de conhecimento</h2>
        <p class="asc-card__subtitle">
          Define o que o bot pode consultar alem dos documentos manuais.
        </p>
      </div>
      <span v-if="saved" class="asc-card__saved">Salvo</span>
    </header>

    <p v-if="errorMessage" class="asc-card__error">{{ errorMessage }}</p>
    <p v-if="loading" class="asc-muted">Carregando...</p>

    <div v-if="!loading" class="asc-card__body">
      <!-- Toggle catalogo -->
      <div class="asc-row">
        <div class="asc-row__info">
          <span class="asc-row__label">Consultar catalogo de produtos do cliente</span>
          <span class="asc-row__desc">
            O bot usa o catalogo da conta para responder perguntas de produto e preco.
          </span>
        </div>
        <button
          type="button"
          class="asc-switch"
          :class="{ 'asc-switch--on': catalogEnabled }"
          role="switch"
          :aria-checked="catalogEnabled"
          :disabled="saving"
          @click="catalogEnabled = !catalogEnabled"
        >
          <span class="asc-switch__track">
            <span class="asc-switch__thumb"></span>
          </span>
        </button>
      </div>

      <!-- URLs do site -->
      <div class="asc-section">
        <label class="asc-field__label">URLs do site (o bot pode referenciar essas paginas)</label>

        <ul v-if="siteUrls.length" class="asc-url-list">
          <li v-for="(url, idx) in siteUrls" :key="idx" class="asc-url-list__item">
            <span class="asc-url-list__url">{{ url }}</span>
            <button
              type="button"
              class="asc-icon-btn asc-icon-btn--danger"
              :aria-label="`Remover ${url}`"
              :disabled="saving"
              @click="removeUrl(idx)"
            >
              &#x2715;
            </button>
          </li>
        </ul>

        <p v-else class="asc-muted">Nenhuma URL adicionada.</p>

        <div class="asc-url-add">
          <input
            v-model="newUrlInput"
            type="url"
            class="asc-url-add__input"
            placeholder="https://exemplo.com/pagina"
            :disabled="saving"
            @keydown="handleKeydown"
          />
          <button
            type="button"
            class="asc-btn asc-btn--ghost"
            :disabled="!newUrlInput.trim() || saving"
            @click="handleAddUrl"
          >
            Adicionar
          </button>
        </div>
      </div>
    </div>

    <footer class="asc-card__foot">
      <button
        type="button"
        class="asc-btn asc-btn--primary"
        :disabled="saving || loading"
        @click="saveSources"
      >
        {{ saving ? 'Salvando...' : 'Salvar fontes' }}
      </button>
    </footer>
  </article>
</template>

<style scoped>
.asc-card {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  padding: 1.25rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.9);
  box-shadow: var(--shadow-card);
}

.asc-card__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.asc-card__title {
  font-size: 1.05rem;
  font-weight: 600;
}

.asc-card__subtitle {
  font-size: 0.8rem;
  color: var(--text-muted);
  margin-top: 0.15rem;
}

.asc-card__saved {
  font-size: 0.8rem;
  font-weight: 600;
  color: rgb(var(--success));
  flex-shrink: 0;
}

.asc-card__error {
  color: rgb(var(--danger));
  background: rgb(var(--danger) / 0.1);
  padding: 0.5rem 0.75rem;
  border-radius: 0.4rem;
  font-size: 0.85rem;
}

.asc-card__body {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.asc-card__foot {
  display: flex;
}

.asc-muted {
  font-size: 0.85rem;
  color: var(--text-muted);
}

/* Linha toggle */
.asc-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.85rem 1rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-soft);
  background: rgb(var(--surface-2) / 0.5);
}

.asc-row__info {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
}

.asc-row__label {
  font-size: 0.9rem;
  font-weight: 600;
}

.asc-row__desc {
  font-size: 0.78rem;
  color: var(--text-muted);
}

/* Switch toggle */
.asc-switch {
  background: transparent;
  border: none;
  cursor: pointer;
  padding: 0;
  flex-shrink: 0;
}

.asc-switch:disabled {
  opacity: 0.6;
  cursor: progress;
}

.asc-switch__track {
  width: 42px;
  height: 24px;
  border-radius: 999px;
  background: rgb(var(--border));
  position: relative;
  transition: background 0.15s ease;
  display: block;
}

.asc-switch--on .asc-switch__track {
  background: rgb(var(--primary));
}

.asc-switch__thumb {
  position: absolute;
  top: 2px;
  left: 2px;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background: rgb(255 255 255);
  transition: transform 0.15s ease;
}

.asc-switch--on .asc-switch__thumb {
  transform: translateX(18px);
}

/* Secao de URLs */
.asc-section {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
}

.asc-field__label {
  font-size: 0.8rem;
  font-weight: 600;
  color: var(--text-muted);
}

.asc-url-list {
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.asc-url-list__item {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.4rem 0.6rem;
  border: 1px solid var(--line-soft);
  border-radius: 0.4rem;
  background: rgb(var(--surface-2) / 0.5);
}

.asc-url-list__url {
  flex: 1;
  font-size: 0.85rem;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  color: var(--text-main);
  word-break: break-all;
  min-width: 0;
}

.asc-url-add {
  display: flex;
  gap: 0.5rem;
}

.asc-url-add__input {
  flex: 1;
  padding: 0.5rem 0.7rem;
  border: 1px solid var(--line-soft);
  border-radius: 0.5rem;
  background: rgb(var(--surface-2) / 0.76);
  color: var(--text-main);
  font: inherit;
  font-size: 0.88rem;
  min-width: 0;
}

.asc-url-add__input:focus {
  outline: none;
  border-color: rgb(var(--ring) / 0.5);
  box-shadow: 0 0 0 3px rgb(var(--ring) / 0.16);
}

/* Botoes */
.asc-btn {
  font-size: 0.88rem;
  font-weight: 600;
  padding: 0.45rem 1rem;
  border-radius: 0.45rem;
  cursor: pointer;
  border: 1px solid transparent;
  white-space: nowrap;
}

.asc-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.asc-btn--primary {
  background: linear-gradient(135deg, rgb(var(--primary)), rgb(var(--primary-600)));
  color: rgb(255 255 255);
}

.asc-btn--ghost {
  background: transparent;
  border-color: var(--line-soft);
  color: var(--text-main);
}

.asc-icon-btn {
  width: 24px;
  height: 24px;
  border-radius: 0.3rem;
  border: 1px solid var(--line-soft);
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 0.7rem;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.asc-icon-btn--danger {
  border-color: rgb(var(--danger) / 0.35);
  color: rgb(var(--danger));
}

.asc-icon-btn--danger:hover {
  background: rgb(var(--danger) / 0.1);
}

.asc-icon-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
