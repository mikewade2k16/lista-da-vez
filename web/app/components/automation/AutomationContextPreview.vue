<script setup lang="ts">
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

interface KnowledgeDocView {
  id: string
  title: string
  enabled: boolean
}

interface ContextPreview {
  personaName: string
  instructions: string
  knowledgeDocs: KnowledgeDocView[]
  guardrails: string
  systemMessage: string
}

const runtimeConfig = useRuntimeConfig()
const auth = useAuthStore()
const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

const preview = ref<ContextPreview | null>(null)
const loading = ref(false)
const error = ref('')

const openSection = ref<'instructions' | 'knowledge' | 'guardrails' | 'assembled' | null>(null)

function toggle(section: 'instructions' | 'knowledge' | 'guardrails' | 'assembled') {
  openSection.value = openSection.value === section ? null : section
}

const activeDocs = computed(() => preview.value?.knowledgeDocs.filter((d) => d.enabled) ?? [])
const inactiveDocs = computed(() => preview.value?.knowledgeDocs.filter((d) => !d.enabled) ?? [])

async function load() {
  loading.value = true
  error.value = ''
  try {
    preview.value = (await apiRequest('/v1/automation/context-preview', {
      method: 'GET',
    })) as ContextPreview
  } catch (e) {
    error.value = getApiErrorMessage(e, 'Falha ao carregar o contexto.')
  } finally {
    loading.value = false
  }
}

onMounted(() => void load())

defineExpose({ refresh: load })
</script>

<template>
  <article class="ctx-card">
    <header class="ctx-card__head">
      <div>
        <h2 class="ctx-card__title">Contexto do bot</h2>
        <p class="ctx-card__subtitle">
          Tudo que vai para o nó
          <strong>AI Agent</strong>
          (campo
          <code>systemMessage</code>
          ) no n8n, em ordem.
        </p>
      </div>
      <button type="button" class="ctx-card__refresh" :disabled="loading" @click="load">
        {{ loading ? '...' : 'Atualizar' }}
      </button>
    </header>

    <p v-if="error" class="ctx-card__error">{{ error }}</p>

    <template v-if="preview">
      <!-- Diagrama de fluxo -->
      <div class="ctx-flow">
        <div class="ctx-flow__step">
          <span class="ctx-flow__num">1</span>
          <div class="ctx-flow__info">
            <span class="ctx-flow__label">Instrucoes</span>
            <span class="ctx-flow__meta">Persona: {{ preview.personaName }}</span>
          </div>
        </div>
        <span class="ctx-flow__arrow">+</span>
        <div class="ctx-flow__step">
          <span class="ctx-flow__num">2</span>
          <div class="ctx-flow__info">
            <span class="ctx-flow__label">Conhecimento</span>
            <span class="ctx-flow__meta">
              {{ activeDocs.length }} doc{{ activeDocs.length !== 1 ? 's' : '' }} ativo{{
                activeDocs.length !== 1 ? 's' : ''
              }}
              <template v-if="inactiveDocs.length">
                &nbsp;({{ inactiveDocs.length }} desabilitado{{
                  inactiveDocs.length !== 1 ? 's' : ''
                }})
              </template>
            </span>
          </div>
        </div>
        <span class="ctx-flow__arrow">+</span>
        <div class="ctx-flow__step">
          <span class="ctx-flow__num">3</span>
          <div class="ctx-flow__info">
            <span class="ctx-flow__label">Guardrails</span>
            <span class="ctx-flow__meta">automatico (nao editavel)</span>
          </div>
        </div>
        <span class="ctx-flow__arrow">→</span>
        <div class="ctx-flow__step ctx-flow__step--target">
          <div class="ctx-flow__info">
            <span class="ctx-flow__label">AI Agent</span>
            <span class="ctx-flow__meta">systemMessage</span>
          </div>
        </div>
      </div>

      <!-- Secao 1: Instrucoes -->
      <div class="ctx-section">
        <button type="button" class="ctx-section__toggle" @click="toggle('instructions')">
          <span class="ctx-section__badge ctx-section__badge--1">1</span>
          <span class="ctx-section__name">Instrucoes — persona "{{ preview.personaName }}"</span>
          <span class="ctx-section__chars">{{ preview.instructions.length }} caracteres</span>
          <span
            class="ctx-section__chevron"
            :class="{ 'ctx-section__chevron--open': openSection === 'instructions' }"
          >
            ▾
          </span>
        </button>
        <div v-if="openSection === 'instructions'" class="ctx-section__body">
          <p class="ctx-section__hint">Card "Comportamento" acima → campo "Instrucoes".</p>
          <textarea class="ctx-pre" readonly :value="preview.instructions" rows="12"></textarea>
        </div>
      </div>

      <!-- Secao 2: Conhecimento -->
      <div class="ctx-section">
        <button type="button" class="ctx-section__toggle" @click="toggle('knowledge')">
          <span class="ctx-section__badge ctx-section__badge--2">2</span>
          <span class="ctx-section__name">Conhecimento</span>
          <span class="ctx-section__chars">
            {{
              activeDocs.length > 0 ? `${activeDocs.length} doc(s) incluido(s)` : 'nenhum doc ativo'
            }}
          </span>
          <span
            class="ctx-section__chevron"
            :class="{ 'ctx-section__chevron--open': openSection === 'knowledge' }"
          >
            ▾
          </span>
        </button>
        <div v-if="openSection === 'knowledge'" class="ctx-section__body">
          <p class="ctx-section__hint">
            Card "Conhecimento" abaixo → documentos habilitados, em ordem.
          </p>
          <p v-if="activeDocs.length === 0" class="ctx-section__empty">
            Nenhum documento ativo. O bot responde so com as instrucoes da persona.
          </p>
          <ul v-else class="ctx-doc-list">
            <li
              v-for="doc in activeDocs"
              :key="doc.id"
              class="ctx-doc-list__item ctx-doc-list__item--active"
            >
              {{ doc.title || '(sem titulo)' }}
            </li>
          </ul>
          <ul v-if="inactiveDocs.length" class="ctx-doc-list">
            <li
              v-for="doc in inactiveDocs"
              :key="doc.id"
              class="ctx-doc-list__item ctx-doc-list__item--off"
            >
              {{ doc.title || '(sem titulo)' }} — desabilitado
            </li>
          </ul>
        </div>
      </div>

      <!-- Secao 3: Guardrails -->
      <div class="ctx-section">
        <button type="button" class="ctx-section__toggle" @click="toggle('guardrails')">
          <span class="ctx-section__badge ctx-section__badge--3">3</span>
          <span class="ctx-section__name">
            Guardrails
            <span class="ctx-section__auto">(automatico)</span>
          </span>
          <span class="ctx-section__chars">{{ preview.guardrails.length }} caracteres</span>
          <span
            class="ctx-section__chevron"
            :class="{ 'ctx-section__chevron--open': openSection === 'guardrails' }"
          >
            ▾
          </span>
        </button>
        <div v-if="openSection === 'guardrails'" class="ctx-section__body">
          <p class="ctx-section__hint">
            Regras de formato e postura (PT-BR, texto puro, baloes). Sempre anexadas ao final — nao
            precisa colocar nas instrucoes.
          </p>
          <textarea class="ctx-pre" readonly :value="preview.guardrails" rows="10"></textarea>
        </div>
      </div>

      <!-- systemMessage montado -->
      <div class="ctx-section ctx-section--assembled">
        <button type="button" class="ctx-section__toggle" @click="toggle('assembled')">
          <span class="ctx-section__badge ctx-section__badge--ai">AI</span>
          <span class="ctx-section__name">systemMessage completo (o que o bot recebe)</span>
          <span class="ctx-section__chars">{{ preview.systemMessage.length }} caracteres</span>
          <span
            class="ctx-section__chevron"
            :class="{ 'ctx-section__chevron--open': openSection === 'assembled' }"
          >
            ▾
          </span>
        </button>
        <div v-if="openSection === 'assembled'" class="ctx-section__body">
          <p class="ctx-section__hint">
            Exatamente o que chega no campo systemMessage do no AI Agent — igual ao retorno de
            <code>/v1/runtime/automation/config?session=default</code>
            .
          </p>
          <textarea class="ctx-pre" readonly :value="preview.systemMessage" rows="18"></textarea>
        </div>
      </div>
    </template>

    <p v-else-if="!loading && !error" class="ctx-card__empty">Sem dados.</p>
  </article>
</template>

<style scoped>
.ctx-card {
  display: flex;
  flex-direction: column;
  gap: 0.85rem;
  padding: 1.25rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.9);
  box-shadow: var(--shadow-card);
}

.ctx-card__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.ctx-card__title {
  font-size: 1.05rem;
  font-weight: 600;
}

.ctx-card__subtitle {
  font-size: 0.82rem;
  color: var(--text-muted);
  margin-top: 0.2rem;
}

.ctx-card__subtitle code {
  font-size: 0.8rem;
  background: rgb(var(--border) / 0.5);
  padding: 0.05rem 0.3rem;
  border-radius: 0.25rem;
}

.ctx-card__refresh {
  font-size: 0.8rem;
  padding: 0.3rem 0.7rem;
  border: 1px solid var(--line-soft);
  border-radius: 0.4rem;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  flex-shrink: 0;
}

.ctx-card__error {
  color: rgb(var(--danger));
  font-size: 0.85rem;
}

.ctx-card__empty {
  color: var(--text-muted);
  font-size: 0.85rem;
}

/* Diagrama de fluxo */
.ctx-flow {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-wrap: wrap;
  padding: 0.75rem 1rem;
  background: rgb(var(--surface-2) / 0.5);
  border-radius: 0.5rem;
  border: 1px solid var(--line-soft);
}

.ctx-flow__step {
  display: flex;
  align-items: center;
  gap: 0.4rem;
}

.ctx-flow__num {
  width: 22px;
  height: 22px;
  border-radius: 50%;
  background: rgb(var(--primary) / 0.15);
  color: rgb(var(--primary));
  font-size: 0.75rem;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.ctx-flow__step--target .ctx-flow__info {
  background: rgb(var(--primary) / 0.1);
  border: 1px solid rgb(var(--primary) / 0.3);
  border-radius: 0.35rem;
  padding: 0.15rem 0.5rem;
}

.ctx-flow__info {
  display: flex;
  flex-direction: column;
}

.ctx-flow__label {
  font-size: 0.8rem;
  font-weight: 600;
}

.ctx-flow__meta {
  font-size: 0.7rem;
  color: var(--text-muted);
}

.ctx-flow__arrow {
  font-size: 0.9rem;
  color: var(--text-muted);
  font-weight: 600;
}

/* Secoes colapsaveis */
.ctx-section {
  border: 1px solid var(--line-soft);
  border-radius: 0.5rem;
  overflow: hidden;
}

.ctx-section--assembled {
  border-color: rgb(var(--primary) / 0.3);
}

.ctx-section__toggle {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 0.6rem;
  padding: 0.65rem 0.85rem;
  background: rgb(var(--surface-2) / 0.5);
  border: none;
  cursor: pointer;
  text-align: left;
}

.ctx-section__toggle:hover {
  background: rgb(var(--surface-2) / 0.8);
}

.ctx-section__badge {
  width: 22px;
  height: 22px;
  border-radius: 50%;
  font-size: 0.72rem;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.ctx-section__badge--1 {
  background: rgb(var(--primary) / 0.15);
  color: rgb(var(--primary));
}
.ctx-section__badge--2 {
  background: rgb(var(--success) / 0.15);
  color: rgb(var(--success));
}
.ctx-section__badge--3 {
  background: color-mix(in srgb, var(--accent-warning) 18%, transparent);
  color: var(--accent-warning);
}
.ctx-section__badge--ai {
  background: rgb(var(--primary) / 0.25);
  color: rgb(var(--primary));
  font-size: 0.65rem;
}

.ctx-section__name {
  font-size: 0.88rem;
  font-weight: 600;
  flex: 1;
}

.ctx-section__auto {
  font-weight: 400;
  color: var(--text-muted);
  font-size: 0.8rem;
}

.ctx-section__chars {
  font-size: 0.75rem;
  color: var(--text-muted);
  flex-shrink: 0;
}

.ctx-section__chevron {
  font-size: 0.9rem;
  color: var(--text-muted);
  transition: transform 0.15s ease;
  flex-shrink: 0;
}

.ctx-section__chevron--open {
  transform: rotate(180deg);
}

.ctx-section__body {
  padding: 0.75rem 0.85rem;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  border-top: 1px solid var(--line-soft);
}

.ctx-section__hint {
  font-size: 0.8rem;
  color: var(--text-muted);
}

.ctx-section__hint code {
  font-size: 0.78rem;
  background: rgb(var(--border) / 0.5);
  padding: 0.05rem 0.3rem;
  border-radius: 0.25rem;
}

.ctx-section__empty {
  font-size: 0.85rem;
  color: var(--text-muted);
}

.ctx-doc-list {
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.ctx-doc-list__item {
  font-size: 0.85rem;
  padding: 0.25rem 0.5rem;
  border-radius: 0.35rem;
}

.ctx-doc-list__item--active {
  background: rgb(var(--success) / 0.1);
  color: rgb(var(--success));
}

.ctx-doc-list__item--off {
  color: var(--text-muted);
  font-style: italic;
}

.ctx-pre {
  width: 100%;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.8rem;
  line-height: 1.5;
  padding: 0.6rem 0.75rem;
  border: 1px solid var(--line-soft);
  border-radius: 0.4rem;
  background: rgb(var(--surface-2) / 0.76);
  color: var(--text-main);
  resize: vertical;
  white-space: pre;
  overflow: auto;
}
</style>
