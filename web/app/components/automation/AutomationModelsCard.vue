<script setup lang="ts">
import {
  useAutomationModels,
  type AutomationModelView,
  type CatalogModelView,
} from '~/composables/useAutomationModels'

const emit = defineEmits<{ (e: 'change'): void }>()
const { catalog, selection, loading, savingRole, errorMessage, loadModels, saveModel } =
  useAutomationModels()

// Estado editavel local por funcao (role): modelo escolhido + temperatura.
interface RoleDraft {
  provider: string
  modelId: string
  temperature: number
  saved: boolean
}
const drafts = ref<Record<string, RoleDraft>>({})

const ROLE_LABELS: Record<string, string> = {
  chat: 'Cerebro / resposta (chat)',
  vision: 'Visao / imagem',
  audio: 'Transcricao de audio',
  classifier: 'Classificador / resumo',
}

const ROLE_HINTS: Record<string, string> = {
  chat: 'O modelo que conversa com o cliente.',
  vision: 'Interpreta as fotos recebidas. Modelos de raciocinio nao funcionam aqui.',
  audio: 'Transcreve os audios antes do cerebro. O Whisper e o padrao.',
  classifier: 'Classifica a etapa da conversa e gera resumos internos.',
}

const ROLE_ORDER = ['chat', 'vision', 'audio', 'classifier']

// Funcoes com draft ja carregado, na ordem canonica (evita v-if junto de v-for).
const roleRows = computed(() => ROLE_ORDER.filter((role) => drafts.value[role]))

function syncDrafts(list: AutomationModelView[]) {
  const next: Record<string, RoleDraft> = {}
  for (const m of list) {
    const temp = typeof m.params?.temperature === 'number' ? m.params.temperature : 0.3
    next[m.role] = { provider: m.provider, modelId: m.modelId, temperature: temp, saved: false }
  }
  drafts.value = next
}

watch(selection, syncDrafts, { immediate: true })

// Composto "provider::modelId" para o <select> (a PK do catalogo e provider+id+kind).
function optionKey(m: CatalogModelView): string {
  return `${m.provider}::${m.id}`
}

function optionsForRole(role: string): CatalogModelView[] {
  return catalog.value.filter((m) => m.kind === role)
}

function selectedCatalog(role: string): CatalogModelView | undefined {
  const d = drafts.value[role]
  if (!d) return undefined
  return catalog.value.find(
    (m) => m.kind === role && m.provider === d.provider && m.id === d.modelId,
  )
}

// Regra do MODELOS.md aplicada pela UI: modelo de raciocinio nao expoe temperatura.
function showsTemperature(role: string): boolean {
  const cat = selectedCatalog(role)
  return !!cat?.acceptsTemperature
}

function reasoningNote(role: string): string {
  const cat = selectedCatalog(role)
  if (!cat) return ''
  if (cat.requiresResponsesApi) {
    return 'Modelo de raciocinio: usa a Responses API e nao aceita temperatura.'
  }
  if (!cat.acceptsTemperature) {
    return 'Este modelo nao aceita ajuste de temperatura.'
  }
  return ''
}

function onSelect(role: string, event: Event) {
  const value = (event.target as HTMLSelectElement).value
  const [provider, modelId] = value.split('::')
  const d = drafts.value[role]
  if (d && provider && modelId) {
    d.provider = provider
    d.modelId = modelId
    d.saved = false
  }
}

async function save(role: string) {
  const d = drafts.value[role]
  if (!d) return
  const params: Record<string, unknown> = {}
  if (showsTemperature(role)) {
    params.temperature = d.temperature
  }
  const ok = await saveModel(role, d.provider, d.modelId, params)
  if (ok) {
    d.saved = true
    emit('change')
    setTimeout(() => {
      d.saved = false
    }, 2000)
  }
}

onMounted(() => void loadModels())
</script>

<template>
  <section class="am-section">
    <header class="am-section__head">
      <div>
        <h2 class="am-section__title">Modelos</h2>
        <p class="am-section__subtitle">
          Escolha o modelo de IA por funcao. As regras de cada modelo sao aplicadas automaticamente.
        </p>
      </div>
    </header>

    <p v-if="errorMessage" class="am-error">{{ errorMessage }}</p>
    <p v-if="loading" class="am-muted">Carregando...</p>

    <article v-for="role in roleRows" :key="role" class="am-row">
      <div class="am-row__head">
        <h3 class="am-row__title">{{ ROLE_LABELS[role] }}</h3>
        <span v-if="drafts[role]?.saved" class="am-saved">Salvo</span>
      </div>
      <p class="am-row__hint">{{ ROLE_HINTS[role] }}</p>

      <div class="am-row__controls">
        <label class="am-field">
          <span class="am-field__label">Modelo</span>
          <select
            class="am-field__select"
            :value="`${drafts[role]?.provider}::${drafts[role]?.modelId}`"
            @change="onSelect(role, $event)"
          >
            <option
              v-for="opt in optionsForRole(role)"
              :key="optionKey(opt)"
              :value="optionKey(opt)"
            >
              {{ opt.label }}
            </option>
          </select>
        </label>

        <label v-if="showsTemperature(role)" class="am-field am-field--temp">
          <span class="am-field__label">
            Temperatura ({{ drafts[role]?.temperature.toFixed(1) }})
          </span>
          <input
            v-model.number="drafts[role].temperature"
            type="range"
            min="0"
            max="1"
            step="0.1"
            class="am-field__range"
          />
        </label>

        <p v-else-if="reasoningNote(role)" class="am-row__note">{{ reasoningNote(role) }}</p>
      </div>

      <footer class="am-row__foot">
        <button
          type="button"
          class="am-btn am-btn--primary"
          :disabled="savingRole === role"
          @click="save(role)"
        >
          {{ savingRole === role ? 'Salvando...' : 'Salvar' }}
        </button>
      </footer>
    </article>
  </section>
</template>

<style scoped>
.am-section {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  padding: 1.25rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.9);
  box-shadow: var(--shadow-card);
}

.am-section__title {
  font-size: 1.05rem;
  font-weight: 600;
}

.am-section__subtitle {
  font-size: 0.8rem;
  color: var(--text-muted);
  margin-top: 0.15rem;
}

.am-error {
  color: rgb(var(--danger));
  background: rgb(var(--danger) / 0.12);
  padding: 0.5rem 0.75rem;
  border-radius: 0.4rem;
  font-size: 0.85rem;
}

.am-muted {
  font-size: 0.85rem;
  color: var(--text-muted);
}

.am-row {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  padding: 1rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-soft);
  background: rgb(var(--surface-2) / 0.5);
}

.am-row__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.am-row__title {
  font-size: 0.95rem;
  font-weight: 600;
}

.am-saved {
  font-size: 0.8rem;
  font-weight: 600;
  color: rgb(var(--success));
}

.am-row__hint {
  font-size: 0.8rem;
  color: var(--text-muted);
}

.am-row__controls {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
  align-items: flex-end;
}

.am-row__note {
  font-size: 0.8rem;
  color: var(--accent-warning);
  align-self: center;
}

.am-field {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  min-width: 220px;
}

.am-field--temp {
  min-width: 180px;
}

.am-field__label {
  font-size: 0.78rem;
  font-weight: 600;
  color: var(--text-muted);
}

.am-field__select {
  padding: 0.5rem 0.7rem;
  border: 1px solid var(--line-soft);
  border-radius: 0.5rem;
  background: rgb(var(--surface-2) / 0.76);
  color: var(--text-main);
  font: inherit;
}

.am-field__select:focus {
  outline: none;
  border-color: rgb(var(--ring) / 0.5);
  box-shadow: 0 0 0 3px rgb(var(--ring) / 0.16);
}

.am-field__range {
  width: 100%;
  accent-color: rgb(var(--primary));
}

.am-row__foot {
  display: flex;
}

.am-btn {
  font-size: 0.88rem;
  font-weight: 600;
  padding: 0.45rem 1rem;
  border-radius: 0.45rem;
  cursor: pointer;
  border: 1px solid transparent;
}

.am-btn:disabled {
  opacity: 0.6;
  cursor: progress;
}

.am-btn--primary {
  background: linear-gradient(135deg, rgb(var(--primary)), rgb(var(--primary-600)));
  color: rgb(255 255 255);
}
</style>
