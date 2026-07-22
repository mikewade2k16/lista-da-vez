<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import AppToggleSwitch from '~/components/ui/AppToggleSwitch.vue'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import {
  createHandoffPolicy,
  deleteHandoffPolicy,
  fetchHandoffPolicies,
  fetchQueues,
  updateHandoffPolicy,
} from '~/domain/omnichannel/config-api'
import type { OmniHandoffPolicy, OmniQueue } from '~/domain/omnichannel/config-types'

// Policies são avaliadas no Go sob lock. Esta tela apenas edita o contrato
// fechado; não executa regra localmente nem envia diretamente para Evolution/n8n.
const props = defineProps<{ canManage: boolean }>()
const auth = useAuthStore()
const ui = useUiStore()
const runtimeConfig = useRuntimeConfig()
const api = createApiRequest(runtimeConfig, () => auth.accessToken)

type ConditionDraft = { key: string; value: string }
type PolicyDraft = {
  name: string
  priority: number
  isActive: boolean
  targetQueueId: string
  fallbackQueueId: string
  customerNoticeTemplate: string
  conditions: ConditionDraft[]
}

const CONDITION_OPTIONS = [
  { value: 'reasonCode', label: 'motivo do handoff' },
  { value: 'sourceState', label: 'estado de origem' },
  { value: 'departmentId', label: 'setor (ID)' },
  { value: 'channel', label: 'canal' },
  { value: 'intent', label: 'intenção' },
  { value: 'relationshipStatus', label: 'ciclo do contato' },
  { value: 'lifecycle', label: 'lifecycle' },
  { value: 'tag', label: 'tag do contato' },
  { value: 'slaRisk', label: 'risco de SLA' },
  { value: 'confidenceMax', label: 'confiança máxima (0–1)' },
  { value: 'confidenceMin', label: 'confiança mínima (0–1)' },
  { value: 'hourUtc', label: 'janela UTC (ex.: 8-18)' },
]

const policies = ref<OmniHandoffPolicy[]>([])
const queues = ref<OmniQueue[]>([])
const loading = ref(true)
const busy = ref(false)
const drafts = reactive<Record<string, PolicyDraft>>({})
const newPolicy = reactive<PolicyDraft>(emptyDraft())

function emptyDraft(): PolicyDraft {
  return {
    name: '',
    priority: 100,
    isActive: true,
    targetQueueId: '',
    fallbackQueueId: '',
    customerNoticeTemplate: '',
    conditions: [],
  }
}

const queueOptions = computed(() =>
  queues.value.filter((q) => q.isActive).map((q) => ({ value: q.id, label: q.name })),
)

function queueName(id: string): string {
  return queues.value.find((q) => q.id === id)?.name || 'fila não encontrada'
}

function conditionValue(value: unknown): string {
  if (value && typeof value === 'object' && 'from' in value && 'to' in value) {
    return `${String((value as { from: unknown }).from)}-${String((value as { to: unknown }).to)}`
  }
  return Array.isArray(value) ? value.join(', ') : value == null ? '' : String(value)
}

function toDraft(policy: OmniHandoffPolicy): PolicyDraft {
  return {
    name: policy.name,
    priority: policy.priority,
    isActive: policy.isActive,
    targetQueueId: policy.targetQueueId || '',
    fallbackQueueId: policy.fallbackQueueId || '',
    customerNoticeTemplate: policy.customerNoticeTemplate || '',
    conditions: Object.entries(policy.conditions || {}).map(([key, value]) => ({
      key,
      value: conditionValue(value),
    })),
  }
}

function parseCondition(condition: ConditionDraft): [string, unknown] | null {
  const key = condition.key.trim()
  const value = condition.value.trim()
  if (!key || !value) return null
  if (key === 'confidenceMax' || key === 'confidenceMin') {
    const number = Number(value)
    return Number.isFinite(number) ? [key, number] : null
  }
  if (key === 'hourUtc') {
    const match = value.match(/^(\d{1,2})\s*-\s*(\d{1,2})$/)
    return match ? [key, { from: Number(match[1]), to: Number(match[2]) }] : null
  }
  const values = value
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
  return [key, values.length > 1 ? values : values[0]]
}

function toApi(draft: PolicyDraft) {
  const conditions: Record<string, unknown> = {}
  for (const condition of draft.conditions) {
    const parsed = parseCondition(condition)
    if (parsed) conditions[parsed[0]] = parsed[1]
  }
  return {
    name: draft.name.trim(),
    priority: Number(draft.priority),
    isActive: draft.isActive,
    conditions,
    targetQueueId: draft.targetQueueId || null,
    fallbackQueueId: draft.fallbackQueueId || null,
    customerNoticeTemplate: draft.customerNoticeTemplate.trim(),
  }
}

async function load(): Promise<void> {
  loading.value = true
  try {
    const [loadedPolicies, loadedQueues] = await Promise.all([
      fetchHandoffPolicies(api),
      fetchQueues(api),
    ])
    policies.value = loadedPolicies
    queues.value = loadedQueues
    for (const policy of loadedPolicies) drafts[policy.id] = toDraft(policy)
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível carregar as políticas de handoff.'))
  } finally {
    loading.value = false
  }
}

function addCondition(draft: PolicyDraft): void {
  draft.conditions.push({ key: 'reasonCode', value: '' })
}

function removeCondition(draft: PolicyDraft, index: number): void {
  draft.conditions.splice(index, 1)
}

async function create(): Promise<void> {
  if (!props.canManage || busy.value || !newPolicy.name.trim()) return
  busy.value = true
  try {
    await createHandoffPolicy(api, toApi(newPolicy))
    Object.assign(newPolicy, emptyDraft())
    ui.success('Política de handoff criada.')
    await load()
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível criar a política.'))
  } finally {
    busy.value = false
  }
}

async function save(id: string): Promise<void> {
  const draft = drafts[id]
  if (!props.canManage || busy.value || !draft?.name.trim()) return
  busy.value = true
  try {
    await updateHandoffPolicy(api, id, toApi(draft))
    ui.success('Política de handoff atualizada.')
    await load()
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível salvar a política.'))
  } finally {
    busy.value = false
  }
}

async function remove(id: string): Promise<void> {
  const { confirmed } = (await ui.confirm({
    title: 'Desativar política?',
    message: 'A política deixa de ser avaliada para novos handoffs; o histórico permanece.',
    confirmLabel: 'Desativar',
    cancelLabel: 'Cancelar',
  })) as { confirmed?: boolean }
  if (!confirmed) return
  busy.value = true
  try {
    await deleteHandoffPolicy(api, id)
    ui.success('Política desativada.')
    await load()
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível desativar a política.'))
  } finally {
    busy.value = false
  }
}

onMounted(() => void load())
</script>

<template>
  <div class="cfg-tab">
    <p class="cfg-tab__lead">
      Políticas determinísticas escolhem a fila do handoff por prioridade. O Go valida e registra a
      policy aplicada; n8n e a IA não podem criar condições nem enviar mensagens.
    </p>
    <p v-if="loading" class="cfg-tab__loading">Carregando políticas…</p>
    <template v-else>
      <section class="policy-card">
        <div class="policy-card__head">
          <strong>Nova política</strong>
          <span>menor prioridade numérica primeiro</span>
        </div>
        <div class="cfg-grid">
          <label class="cfg-field">
            <span class="cfg-field__label">Nome *</span>
            <input v-model="newPolicy.name" class="cfg-input" :disabled="!canManage" />
          </label>
          <label class="cfg-field">
            <span class="cfg-field__label">Prioridade</span>
            <input
              v-model.number="newPolicy.priority"
              class="cfg-input"
              type="number"
              min="0"
              max="100000"
              :disabled="!canManage"
            />
          </label>
          <label class="cfg-field">
            <span class="cfg-field__label">Fila destino</span>
            <select v-model="newPolicy.targetQueueId" class="cfg-input" :disabled="!canManage">
              <option value="">Usar roteamento padrão</option>
              <option v-for="q in queueOptions" :key="q.value" :value="q.value">
                {{ q.label }}
              </option>
            </select>
          </label>
          <label class="cfg-field">
            <span class="cfg-field__label">Fila fallback</span>
            <select v-model="newPolicy.fallbackQueueId" class="cfg-input" :disabled="!canManage">
              <option value="">Nenhuma</option>
              <option v-for="q in queueOptions" :key="q.value" :value="q.value">
                {{ q.label }}
              </option>
            </select>
          </label>
        </div>
        <div class="policy-conditions">
          <div class="policy-card__head">
            <span>Condições (todas)</span>
            <AppPanelButton variant="ghost" :disabled="!canManage" @click="addCondition(newPolicy)">
              + condição
            </AppPanelButton>
          </div>
          <div
            v-for="(condition, index) in newPolicy.conditions"
            :key="index"
            class="policy-condition"
          >
            <select v-model="condition.key" class="cfg-input" :disabled="!canManage">
              <option v-for="option in CONDITION_OPTIONS" :key="option.value" :value="option.value">
                {{ option.label }}
              </option>
            </select>
            <input
              v-model="condition.value"
              class="cfg-input"
              placeholder="valor"
              :disabled="!canManage"
            />
            <button
              type="button"
              class="policy-remove"
              :disabled="!canManage"
              @click="removeCondition(newPolicy, index)"
            >
              ×
            </button>
          </div>
        </div>
        <label class="cfg-field">
          <span class="cfg-field__label">Aviso ao cliente (opcional; envio passa pela outbox)</span>
          <textarea
            v-model="newPolicy.customerNoticeTemplate"
            class="cfg-input policy-textarea"
            maxlength="2000"
            :disabled="!canManage"
          ></textarea>
        </label>
        <div class="policy-actions">
          <AppToggleSwitch
            v-model="newPolicy.isActive"
            label="Ativa"
            :disabled="!canManage"
            compact
          />
          <AppPanelButton :disabled="!canManage || busy || !newPolicy.name.trim()" @click="create">
            Criar política
          </AppPanelButton>
        </div>
      </section>

      <section v-for="policy in policies" :key="policy.id" class="policy-card">
        <div class="policy-card__head">
          <strong>{{ policy.name }}</strong>
          <span>
            #{{ policy.priority }} ·
            {{ policy.targetQueueId ? queueName(policy.targetQueueId) : 'roteamento padrão' }}
          </span>
        </div>
        <div v-if="drafts[policy.id]" class="cfg-grid">
          <label class="cfg-field">
            <span class="cfg-field__label">Nome</span>
            <input v-model="drafts[policy.id].name" class="cfg-input" :disabled="!canManage" />
          </label>
          <label class="cfg-field">
            <span class="cfg-field__label">Prioridade</span>
            <input
              v-model.number="drafts[policy.id].priority"
              class="cfg-input"
              type="number"
              min="0"
              max="100000"
              :disabled="!canManage"
            />
          </label>
          <label class="cfg-field">
            <span class="cfg-field__label">Fila destino</span>
            <select
              v-model="drafts[policy.id].targetQueueId"
              class="cfg-input"
              :disabled="!canManage"
            >
              <option value="">Usar roteamento padrão</option>
              <option v-for="q in queueOptions" :key="q.value" :value="q.value">
                {{ q.label }}
              </option>
            </select>
          </label>
          <label class="cfg-field">
            <span class="cfg-field__label">Fila fallback</span>
            <select
              v-model="drafts[policy.id].fallbackQueueId"
              class="cfg-input"
              :disabled="!canManage"
            >
              <option value="">Nenhuma</option>
              <option v-for="q in queueOptions" :key="q.value" :value="q.value">
                {{ q.label }}
              </option>
            </select>
          </label>
        </div>
        <div v-if="drafts[policy.id]" class="policy-conditions">
          <div class="policy-card__head">
            <span>Condições</span>
            <AppPanelButton
              variant="ghost"
              :disabled="!canManage"
              @click="addCondition(drafts[policy.id])"
            >
              + condição
            </AppPanelButton>
          </div>
          <div
            v-for="(condition, index) in drafts[policy.id].conditions"
            :key="index"
            class="policy-condition"
          >
            <select v-model="condition.key" class="cfg-input" :disabled="!canManage">
              <option v-for="option in CONDITION_OPTIONS" :key="option.value" :value="option.value">
                {{ option.label }}
              </option>
            </select>
            <input
              v-model="condition.value"
              class="cfg-input"
              placeholder="valor"
              :disabled="!canManage"
            />
            <button
              type="button"
              class="policy-remove"
              :disabled="!canManage"
              @click="removeCondition(drafts[policy.id], index)"
            >
              ×
            </button>
          </div>
        </div>
        <div v-if="drafts[policy.id]" class="policy-actions">
          <AppToggleSwitch
            v-model="drafts[policy.id].isActive"
            label="Ativa"
            :disabled="!canManage"
            compact
          />
          <AppPanelButton
            variant="secondary"
            :disabled="!canManage || busy"
            @click="save(policy.id)"
          >
            Salvar
          </AppPanelButton>
          <AppPanelButton
            variant="danger"
            :disabled="!canManage || busy"
            @click="remove(policy.id)"
          >
            Desativar
          </AppPanelButton>
        </div>
      </section>
      <p v-if="!policies.length" class="cfg-tab__empty">
        Nenhuma política criada. O handoff seguirá o roteamento configurado.
      </p>
    </template>
  </div>
</template>

<style scoped>
.policy-card {
  display: flex;
  flex-direction: column;
  gap: 0.8rem;
  padding: 1rem;
  margin-bottom: 0.8rem;
  border: 1px solid var(--line-soft);
  border-radius: 16px;
  background: rgb(var(--surface-2) / 0.32);
}
.policy-card__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  color: var(--text-main);
  font-size: 0.82rem;
}
.policy-card__head span {
  color: var(--text-muted);
  font-size: 0.72rem;
}
.policy-conditions {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}
.policy-condition {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1.2fr) auto;
  gap: 0.45rem;
}
.policy-remove {
  border: 1px solid var(--line-soft);
  border-radius: 8px;
  background: transparent;
  color: var(--text-muted);
  padding: 0 0.65rem;
  cursor: pointer;
}
.policy-textarea {
  min-height: 4.5rem;
  resize: vertical;
}
.policy-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.5rem;
}
.cfg-tab__empty {
  color: var(--text-muted);
  font-size: 0.82rem;
}
@media (max-width: 700px) {
  .policy-condition {
    grid-template-columns: 1fr;
  }
  .policy-actions {
    flex-wrap: wrap;
    justify-content: flex-start;
  }
}
</style>
