<script setup lang="ts">
import { computed, ref } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import { simulateAgent } from '~/domain/omnichannel/config-api'
import type {
  OmniAgentVersion,
  OmniSimMessage,
  OmniSimulateResult,
} from '~/domain/omnichannel/config-types'

// Simulador mínimo (F9 C9.7). Manda messages[] (histórico) — é o que exercita a triagem.
// versionId ausente = versão publicada; para testar ANTES de publicar, escolha o rascunho.
// Mostra o traço (valid/validationErrors/extractedFields/matchedRule/wouldRoute/usage):
// prova "IA sugere, motor decide". A simulação chama o modelo, grava ai_runs e consome o
// limite mensal — mas NÃO envia mensagem ao cliente.
const props = defineProps<{
  agentId: string
  versions: OmniAgentVersion[]
  disabled?: boolean
}>()

const auth = useAuthStore()
const ui = useUiStore()
const runtimeConfig = useRuntimeConfig()
const api = createApiRequest(runtimeConfig, () => auth.accessToken)

type Row = { role: 'contact' | 'agent'; text: string }
const messages = ref<Row[]>([{ role: 'contact', text: '' }])
const contactName = ref('')
const versionId = ref('')
const running = ref(false)
const result = ref<OmniSimulateResult | null>(null)

const ROLE_OPTIONS = [
  { value: 'contact', label: 'Cliente' },
  { value: 'agent', label: 'Atendente' },
]

const versionOptions = computed(() => [
  { value: '', label: 'Versão publicada (ativa)' },
  ...props.versions.map((v) => ({
    value: v.id,
    label: `v${v.version} — ${v.status}`,
    meta: `${v.provider}/${v.model}`,
  })),
])

const canRun = computed(
  () => !props.disabled && !running.value && messages.value.some((m) => m.text.trim()),
)

function addRow(): void {
  const lastRole = messages.value[messages.value.length - 1]?.role
  messages.value.push({ role: lastRole === 'contact' ? 'agent' : 'contact', text: '' })
}
function removeRow(index: number): void {
  messages.value.splice(index, 1)
  if (messages.value.length === 0) messages.value.push({ role: 'contact', text: '' })
}
function setRole(row: Row, value: string): void {
  row.role = value === 'agent' ? 'agent' : 'contact'
}

const prettyOutput = computed(() => {
  if (!result.value) return ''
  try {
    return JSON.stringify(result.value.output, null, 2)
  } catch {
    return String(result.value.output)
  }
})

const extractedEntries = computed(() =>
  Object.entries(result.value?.extractedFields || {}).map(([k, v]) => ({
    key: k,
    value: typeof v === 'object' ? JSON.stringify(v) : String(v),
  })),
)

async function run(): Promise<void> {
  if (!canRun.value) return
  running.value = true
  try {
    const payload: OmniSimMessage[] = messages.value
      .filter((m) => m.text.trim())
      .map((m) => ({ role: m.role, text: m.text.trim() }))
    result.value = await simulateAgent(api, props.agentId, {
      versionId: versionId.value || undefined,
      messages: payload,
      contact: contactName.value.trim() ? { name: contactName.value.trim() } : undefined,
    })
  } catch (error) {
    // 409 acionável (ai_limit_exceeded / ai_provider_not_configured) chega com mensagem própria.
    ui.error(getApiErrorMessage(error, 'Não foi possível simular.'))
  } finally {
    running.value = false
  }
}
</script>

<template>
  <div class="cfg-sim">
    <p class="cfg-sim__note">
      A simulação chama o modelo de verdade, grava auditoria e consome o limite mensal de IA.
      <strong>Não envia mensagem ao cliente.</strong>
    </p>

    <div class="cfg-grid">
      <AppSelectField
        class="cfg-field"
        label="Versão a testar"
        :model-value="versionId"
        :options="versionOptions"
        :disabled="disabled"
        @update:model-value="versionId = $event"
      />
      <label class="cfg-field">
        <span class="cfg-field__label">Nome do contato (opcional)</span>
        <input v-model="contactName" class="cfg-input" type="text" :disabled="disabled" />
      </label>
    </div>

    <div class="cfg-sim__msgs">
      <span class="cfg-field__label">Histórico da conversa</span>
      <div v-for="(m, i) in messages" :key="i" class="cfg-sim__row">
        <AppSelectField
          :model-value="m.role"
          :options="ROLE_OPTIONS"
          :disabled="disabled"
          @update:model-value="setRole(m, $event)"
        />
        <input
          v-model="m.text"
          class="cfg-input"
          type="text"
          placeholder="mensagem"
          :disabled="disabled"
        />
        <button class="cfg-sim__del" type="button" :disabled="disabled" @click="removeRow(i)">
          remover
        </button>
      </div>
      <AppPanelButton variant="ghost" :disabled="disabled" @click="addRow">
        + mensagem
      </AppPanelButton>
    </div>

    <div class="cfg-sim__foot">
      <span v-if="!canRun && !running" class="cfg-sim__hint">
        Escreva ao menos uma mensagem para simular.
      </span>
      <AppPanelButton variant="primary" :disabled="!canRun" @click="run">
        {{ running ? 'Simulando…' : 'Simular' }}
      </AppPanelButton>
    </div>

    <section v-if="result" class="cfg-sim__result">
      <div class="cfg-sim__badges">
        <span class="cfg-sim__badge" :class="result.valid ? 'is-ok' : 'is-bad'">
          {{ result.valid ? 'Saída válida' : 'Saída inválida' }}
        </span>
        <span class="cfg-sim__badge is-muted">esquema {{ result.schemaVersion || '—' }}</span>
      </div>

      <p v-if="result.validationErrors.length" class="cfg-sim__errors">
        {{ result.validationErrors.join(' · ') }}
      </p>

      <div class="cfg-sim__trace">
        <div class="cfg-sim__trace-item">
          <span class="cfg-field__label">Regra que casou</span>
          <span>
            {{
              result.matchedRule
                ? `${result.matchedRule.name} (prioridade ${result.matchedRule.priority})`
                : 'nenhuma — cai no fallback/unrouted'
            }}
          </span>
        </div>
        <div class="cfg-sim__trace-item">
          <span class="cfg-field__label">Fila destino (motor decidiria)</span>
          <span>{{ result.wouldRoute?.queueId || 'não roteada (unrouted)' }}</span>
        </div>
        <div class="cfg-sim__trace-item">
          <span class="cfg-field__label">Custo</span>
          <span>
            {{ result.usage.totalTokens }} tokens · US$ {{ result.usage.costUsd.toFixed(4) }}
          </span>
        </div>
      </div>

      <div v-if="extractedEntries.length" class="cfg-sim__fields">
        <span class="cfg-field__label">Campos extraídos</span>
        <ul>
          <li v-for="f in extractedEntries" :key="f.key">
            <strong>{{ f.key }}:</strong>
            {{ f.value }}
          </li>
        </ul>
      </div>

      <details class="cfg-sim__raw">
        <summary>Saída bruta do modelo</summary>
        <pre>{{ prettyOutput }}</pre>
      </details>
    </section>
  </div>
</template>

<style scoped>
.cfg-sim {
  display: grid;
  gap: 0.75rem;
}

.cfg-sim__note {
  margin: 0;
  padding: 0.5rem 0.65rem;
  border: 1px solid rgb(var(--border) / 0.8);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2) / 0.6);
  color: rgb(var(--text));
  font-size: 0.78rem;
  line-height: 1.35;
}

.cfg-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 0.75rem;
}

.cfg-field {
  display: grid;
  gap: 0.3rem;
  min-width: 0;
}

.cfg-field__label {
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  color: rgb(var(--muted));
}

.cfg-input {
  min-height: 36px;
  padding: 0 0.75rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface));
  color: rgb(var(--text));
  font-size: 0.82rem;
}

.cfg-input:focus {
  outline: none;
  border-color: rgb(var(--primary) / 0.6);
}

.cfg-input:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.cfg-sim__msgs {
  display: grid;
  gap: 0.5rem;
}

.cfg-sim__row {
  display: grid;
  grid-template-columns: 160px minmax(0, 1fr) auto;
  gap: 0.5rem;
  align-items: center;
}

.cfg-sim__del {
  background: transparent;
  border: 0;
  color: rgb(var(--danger));
  font-size: 0.76rem;
  cursor: pointer;
}

.cfg-sim__del:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.cfg-sim__foot {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.75rem;
}

.cfg-sim__hint {
  color: rgb(var(--muted));
  font-size: 0.76rem;
}

.cfg-sim__result {
  display: grid;
  gap: 0.6rem;
  padding-top: 0.6rem;
  border-top: 1px solid rgb(var(--border) / 0.6);
}

.cfg-sim__badges {
  display: flex;
  gap: 0.5rem;
}

.cfg-sim__badge {
  padding: 0.15rem 0.5rem;
  border-radius: 999px;
  font-size: 0.74rem;
  font-weight: 700;
}

.cfg-sim__badge.is-ok {
  background: rgb(var(--success) / 0.16);
  color: rgb(var(--success));
}

.cfg-sim__badge.is-bad {
  background: rgb(var(--danger) / 0.16);
  color: rgb(var(--danger));
}

.cfg-sim__badge.is-muted {
  background: rgb(var(--surface-2));
  color: rgb(var(--muted));
}

.cfg-sim__errors {
  margin: 0;
  color: rgb(var(--danger));
  font-size: 0.8rem;
}

.cfg-sim__trace {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 0.6rem;
}

.cfg-sim__trace-item {
  display: grid;
  gap: 0.2rem;
  font-size: 0.82rem;
  color: rgb(var(--text));
}

.cfg-sim__fields ul {
  margin: 0.2rem 0 0;
  padding-left: 1.1rem;
  font-size: 0.82rem;
  color: rgb(var(--text));
}

.cfg-sim__raw pre {
  margin: 0.3rem 0 0;
  padding: 0.6rem;
  overflow-x: auto;
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2) / 0.7);
  color: rgb(var(--text));
  font-size: 0.76rem;
}
</style>
