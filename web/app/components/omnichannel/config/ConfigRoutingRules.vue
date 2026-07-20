<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import AppToggleSwitch from '~/components/ui/AppToggleSwitch.vue'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import {
  createRoutingRule,
  deleteRoutingRule,
  fetchQueues,
  fetchRoutingRules,
  reorderRoutingRules,
  updateRoutingRule,
} from '~/domain/omnichannel/config-api'
import type {
  OmniCondition,
  OmniConditionOp,
  OmniQueue,
  OmniRoutingRule,
} from '~/domain/omnichannel/config-types'

// Aba de regras de roteamento (perm omnichannel.settings.manage). Ordenadas por prioridade;
// reordenar usa PUT /routing-rules/order (uma transação, tudo ou nada) — nunca N PATCH de
// priority. IA sugere, motor decide: a regra que casa manda para a fila destino.
defineProps<{ canManage: boolean }>()

const auth = useAuthStore()
const ui = useUiStore()
const runtimeConfig = useRuntimeConfig()
const api = createApiRequest(runtimeConfig, () => auth.accessToken)

type DraftCondition = { field: string; op: OmniConditionOp; value: string }
type RuleDraft = {
  name: string
  targetQueueId: string
  isActive: boolean
  conditions: DraftCondition[]
}

const rules = ref<OmniRoutingRule[]>([])
const queues = ref<OmniQueue[]>([])
const loading = ref(true)
const busy = ref(false)
const drafts = reactive<Record<string, RuleDraft>>({})
const newRule = reactive<RuleDraft>({ name: '', targetQueueId: '', isActive: true, conditions: [] })

const OP_OPTIONS: Array<{ value: OmniConditionOp; label: string }> = [
  { value: 'eq', label: 'igual a' },
  { value: 'neq', label: 'diferente de' },
  { value: 'contains', label: 'contém' },
  { value: 'in', label: 'está em (lista, vírgula)' },
  { value: 'exists', label: 'existe' },
]

const queueOptions = computed(() =>
  queues.value.map((q) => ({ value: q.id, label: q.name, meta: q.isActive ? '' : 'desativada' })),
)

function queueName(id: string): string {
  return queues.value.find((q) => q.id === id)?.name || '—'
}

function toDraftConditions(conds: OmniCondition[]): DraftCondition[] {
  return (conds || []).map((c) => ({
    field: c.field,
    op: c.op,
    value: Array.isArray(c.value) ? c.value.join(', ') : c.value == null ? '' : String(c.value),
  }))
}

function toApiConditions(conds: DraftCondition[]): OmniCondition[] {
  return conds
    .filter((c) => c.field.trim())
    .map((c) => {
      if (c.op === 'exists') return { field: c.field.trim(), op: c.op, value: true }
      if (c.op === 'in')
        return {
          field: c.field.trim(),
          op: c.op,
          value: c.value
            .split(',')
            .map((v) => v.trim())
            .filter(Boolean),
        }
      return { field: c.field.trim(), op: c.op, value: c.value.trim() }
    })
}

async function load(): Promise<void> {
  loading.value = true
  try {
    const [rs, qs] = await Promise.all([fetchRoutingRules(api), fetchQueues(api)])
    rules.value = rs
    queues.value = qs
    for (const r of rs) {
      drafts[r.id] = {
        name: r.name,
        targetQueueId: r.targetQueueId,
        isActive: r.isActive,
        conditions: toDraftConditions(r.conditions),
      }
    }
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível carregar as regras.'))
  } finally {
    loading.value = false
  }
}

const createBlockedReason = computed(() => {
  if (!newRule.name.trim()) return 'Informe o nome da regra.'
  if (!newRule.targetQueueId) return 'Escolha a fila destino.'
  return ''
})

async function create(): Promise<void> {
  if (createBlockedReason.value || busy.value) return
  busy.value = true
  try {
    await createRoutingRule(api, {
      name: newRule.name.trim(),
      priority: rules.value.length + 1,
      isActive: newRule.isActive,
      conditions: toApiConditions(newRule.conditions),
      targetQueueId: newRule.targetQueueId,
    })
    newRule.name = ''
    newRule.targetQueueId = ''
    newRule.conditions = []
    ui.success('Regra criada.')
    await load()
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível criar a regra.'))
  } finally {
    busy.value = false
  }
}

async function save(id: string): Promise<void> {
  const d = drafts[id]
  if (!d || busy.value) return
  if (!d.name.trim() || !d.targetQueueId) {
    ui.error('Nome e fila destino são obrigatórios.')
    return
  }
  busy.value = true
  try {
    await updateRoutingRule(api, id, {
      name: d.name.trim(),
      isActive: d.isActive,
      targetQueueId: d.targetQueueId,
      conditions: toApiConditions(d.conditions),
    })
    ui.success('Regra atualizada.')
    await load()
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível salvar a regra.'))
  } finally {
    busy.value = false
  }
}

async function remove(id: string): Promise<void> {
  const { confirmed } = await ui.confirm({
    title: 'Excluir regra?',
    message: 'A regra deixa de ser avaliada no roteamento. As conversas já roteadas não mudam.',
    confirmLabel: 'Excluir',
    cancelLabel: 'Cancelar',
  })
  if (!confirmed) return
  busy.value = true
  try {
    await deleteRoutingRule(api, id)
    ui.success('Regra excluída.')
    await load()
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível excluir a regra.'))
  } finally {
    busy.value = false
  }
}

async function move(index: number, delta: number): Promise<void> {
  const target = index + delta
  if (busy.value || target < 0 || target >= rules.value.length) return
  const order = rules.value.map((r) => r.id)
  const [moved] = order.splice(index, 1)
  order.splice(target, 0, moved)
  busy.value = true
  try {
    rules.value = await reorderRoutingRules(api, order)
    ui.success('Ordem das regras atualizada.')
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível reordenar as regras.'))
    await load()
  } finally {
    busy.value = false
  }
}

function addCondition(target: RuleDraft): void {
  target.conditions.push({ field: '', op: 'eq', value: '' })
}
function removeCondition(target: RuleDraft, index: number): void {
  target.conditions.splice(index, 1)
}
function setCondOp(cond: DraftCondition, value: string): void {
  cond.op = value as OmniConditionOp
}

onMounted(() => void load())
</script>

<template>
  <div class="cfg-tab">
    <p class="cfg-tab__lead">
      Regras avaliadas de cima para baixo; a primeira que casa manda a conversa para a fila destino.
      A IA sugere; a regra decide.
    </p>
    <p v-if="loading" class="cfg-tab__loading">Carregando regras…</p>

    <template v-else>
      <details class="settings-collapse">
        <summary class="settings-collapse__summary">
          <div class="settings-collapse__title-wrap">
            <strong class="settings-collapse__title">Adicionar regra</strong>
            <span class="settings-collapse__text">Nova regra no fim da ordem de prioridade.</span>
          </div>
          <span class="settings-collapse__meta">novo</span>
          <span class="material-icons-round settings-collapse__icon" aria-hidden="true">
            expand_more
          </span>
        </summary>
        <div class="settings-collapse__body">
          <div class="cfg-grid">
            <label class="cfg-field">
              <span class="cfg-field__label">Nome da regra *</span>
              <input v-model="newRule.name" class="cfg-input" type="text" :disabled="!canManage" />
            </label>
            <AppSelectField
              class="cfg-field"
              label="Fila destino *"
              :model-value="newRule.targetQueueId"
              :options="queueOptions"
              :disabled="!canManage"
              @update:model-value="newRule.targetQueueId = $event"
            />
          </div>
          <div class="cfg-cond">
            <div class="cfg-cond__head">
              <span class="cfg-field__label">Condições (todas precisam casar)</span>
              <AppPanelButton variant="ghost" :disabled="!canManage" @click="addCondition(newRule)">
                + condição
              </AppPanelButton>
            </div>
            <div v-for="(c, i) in newRule.conditions" :key="i" class="cfg-cond__row">
              <input
                v-model="c.field"
                class="cfg-input"
                type="text"
                placeholder="campo"
                :disabled="!canManage"
              />
              <AppSelectField
                :model-value="c.op"
                :options="OP_OPTIONS"
                :disabled="!canManage"
                @update:model-value="setCondOp(c, $event)"
              />
              <input
                v-model="c.value"
                class="cfg-input"
                type="text"
                placeholder="valor"
                :disabled="!canManage || c.op === 'exists'"
              />
              <button
                class="cfg-cond__del"
                type="button"
                :disabled="!canManage"
                @click="removeCondition(newRule, i)"
              >
                remover
              </button>
            </div>
          </div>
          <div class="cfg-tab__form-foot">
            <span v-if="createBlockedReason" class="cfg-tab__hint">{{ createBlockedReason }}</span>
            <AppPanelButton
              variant="primary"
              :disabled="!canManage || busy || !!createBlockedReason"
              @click="create"
            >
              Criar regra
            </AppPanelButton>
          </div>
        </div>
      </details>

      <p v-if="rules.length === 0" class="cfg-empty">Nenhuma regra cadastrada ainda.</p>

      <details v-for="(r, index) in rules" :key="r.id" class="settings-collapse">
        <summary class="settings-collapse__summary">
          <div class="settings-collapse__title-wrap">
            <strong class="settings-collapse__title">{{ index + 1 }}. {{ r.name }}</strong>
            <span class="settings-collapse__text">→ {{ queueName(r.targetQueueId) }}</span>
          </div>
          <span class="settings-collapse__meta">{{ r.isActive ? 'ativa' : 'inativa' }}</span>
          <span class="material-icons-round settings-collapse__icon" aria-hidden="true">
            expand_more
          </span>
        </summary>
        <div v-if="drafts[r.id]" class="settings-collapse__body">
          <div class="cfg-reorder">
            <AppPanelButton
              variant="ghost"
              :disabled="!canManage || busy || index === 0"
              @click="move(index, -1)"
            >
              ↑ subir
            </AppPanelButton>
            <AppPanelButton
              variant="ghost"
              :disabled="!canManage || busy || index === rules.length - 1"
              @click="move(index, 1)"
            >
              ↓ descer
            </AppPanelButton>
          </div>
          <div class="cfg-grid">
            <label class="cfg-field">
              <span class="cfg-field__label">Nome *</span>
              <input
                v-model="drafts[r.id].name"
                class="cfg-input"
                type="text"
                :disabled="!canManage"
              />
            </label>
            <AppSelectField
              class="cfg-field"
              label="Fila destino *"
              :model-value="drafts[r.id].targetQueueId"
              :options="queueOptions"
              :disabled="!canManage"
              @update:model-value="drafts[r.id].targetQueueId = $event"
            />
          </div>
          <AppToggleSwitch v-model="drafts[r.id].isActive" :disabled="!canManage" label="Ativa" />
          <div class="cfg-cond">
            <div class="cfg-cond__head">
              <span class="cfg-field__label">Condições (todas precisam casar)</span>
              <AppPanelButton
                variant="ghost"
                :disabled="!canManage"
                @click="addCondition(drafts[r.id])"
              >
                + condição
              </AppPanelButton>
            </div>
            <p v-if="drafts[r.id].conditions.length === 0" class="cfg-tab__hint">
              Sem condições = a regra casa sempre (fallback).
            </p>
            <div v-for="(c, i) in drafts[r.id].conditions" :key="i" class="cfg-cond__row">
              <input
                v-model="c.field"
                class="cfg-input"
                type="text"
                placeholder="campo"
                :disabled="!canManage"
              />
              <AppSelectField
                :model-value="c.op"
                :options="OP_OPTIONS"
                :disabled="!canManage"
                @update:model-value="setCondOp(c, $event)"
              />
              <input
                v-model="c.value"
                class="cfg-input"
                type="text"
                placeholder="valor"
                :disabled="!canManage || c.op === 'exists'"
              />
              <button
                class="cfg-cond__del"
                type="button"
                :disabled="!canManage"
                @click="removeCondition(drafts[r.id], i)"
              >
                remover
              </button>
            </div>
          </div>
          <div class="cfg-tab__form-foot">
            <AppPanelButton variant="ghost" :disabled="!canManage || busy" @click="remove(r.id)">
              Excluir
            </AppPanelButton>
            <AppPanelButton variant="primary" :disabled="!canManage || busy" @click="save(r.id)">
              Salvar regra
            </AppPanelButton>
          </div>
        </div>
      </details>
    </template>
  </div>
</template>

<style scoped>
.cfg-tab {
  display: grid;
  gap: 0.75rem;
}

.cfg-tab__lead,
.cfg-tab__loading,
.cfg-empty {
  margin: 0;
  color: rgb(var(--muted));
  font-size: 0.82rem;
  line-height: 1.4;
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

.cfg-cond {
  display: grid;
  gap: 0.5rem;
  margin-top: 0.5rem;
}

.cfg-cond__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}

.cfg-cond__row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) minmax(0, 1fr) auto;
  gap: 0.5rem;
  align-items: center;
}

.cfg-cond__del {
  background: transparent;
  border: 0;
  color: rgb(var(--danger));
  font-size: 0.76rem;
  cursor: pointer;
}

.cfg-cond__del:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.cfg-reorder {
  display: flex;
  gap: 0.5rem;
  margin-bottom: 0.5rem;
}

.cfg-tab__form-foot {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.75rem;
  margin-top: 0.5rem;
}

.cfg-tab__hint {
  margin: 0;
  color: rgb(var(--muted));
  font-size: 0.76rem;
}
</style>
