<script setup>
import { reactive, watch } from 'vue'

import SettingsCrmConsultantRules from '~/components/settings/sections/SettingsCrmConsultantRules.vue'

const props = defineProps({
  ctx: {
    type: Object,
    required: true,
  },
})

const payoutGroups = [
  { id: 'consultant', label: 'Consultor' },
  { id: 'managerShopping', label: 'Gerente Shopping' },
  { id: 'managerBairro', label: 'Gerente Lojas Bairro' },
  { id: 'support', label: 'Caixa e auxiliar' },
]

let localRuleSeq = 0

function nextLocalId() {
  localRuleSeq += 1
  return `payout-rule-${Date.now()}-${localRuleSeq}`
}

function toDraftRule(rule) {
  return {
    _id: nextLocalId(),
    threshold: rule?.threshold ?? '',
    value: rule?.value ?? '',
    mode: rule?.mode === 'amount' ? 'amount' : 'percent',
  }
}

// Rascunho local editavel por grupo. O usuario edita livremente (valor vazio/0
// transitorio nao derruba a linha) e so persistimos no blur / botao "Salvar faixas".
const draft = reactive(Object.fromEntries(payoutGroups.map((group) => [group.id, []])))

// Marca quais grupos tem edicao pendente para nao sobrescrever o rascunho com a
// fonte enquanto o usuario digita.
const dirty = reactive(Object.fromEntries(payoutGroups.map((group) => [group.id, false])))

function syncGroupFromSource(groupId) {
  const source = props.ctx.crmGoalPayoutPolicy?.[groupId] || []
  draft[groupId] = source.map((rule) => toDraftRule(rule))
  dirty[groupId] = false
}

function syncAllFromSource() {
  for (const group of payoutGroups) {
    if (!dirty[group.id]) syncGroupFromSource(group.id)
  }
}

watch(() => props.ctx.crmGoalPayoutPolicy, syncAllFromSource, { immediate: true, deep: true })

function groupSummary(groupId) {
  const count = draft[groupId]?.length || 0
  const suffix = count === 1 ? 'faixa' : 'faixas'
  const pending = dirty[groupId] ? ' - alteracoes nao salvas' : ''
  return `${count} ${suffix}${pending}`
}

function markDirty(groupId) {
  dirty[groupId] = true
}

function payoutValueStep(rule) {
  return rule?.mode === 'amount' ? '1' : '0.1'
}

function addRule(groupId) {
  draft[groupId] = [
    ...(draft[groupId] || []),
    toDraftRule({ threshold: '', value: '', mode: 'percent' }),
  ]
  markDirty(groupId)
}

async function removeRule(groupId, localId) {
  draft[groupId] = (draft[groupId] || []).filter((rule) => rule._id !== localId)
  markDirty(groupId)
  await saveGroup(groupId)
}

async function changeMode(groupId) {
  markDirty(groupId)
  await saveGroup(groupId)
}

function groupLabel(groupId) {
  return payoutGroups.find((group) => group.id === groupId)?.label || groupId
}

async function saveGroup(groupId) {
  if (!props.ctx.canEditCrmCommercialPolicy || !dirty[groupId]) return
  const rules = (draft[groupId] || []).map((rule) => ({
    threshold: rule.threshold,
    value: rule.value,
    mode: rule.mode,
  }))
  const result = await props.ctx.saveCrmGoalPayoutGroup(groupId, rules, groupLabel(groupId))
  if (result?.ok !== false) {
    dirty[groupId] = false
    syncGroupFromSource(groupId)
  }
}
</script>

<template>
  <div class="settings-grid">
    <article class="settings-card">
      <header class="settings-card__header">
        <h3 class="settings-card__title">Uso da lista</h3>
      </header>

      <label class="settings-field">
        <span>Pedidos minimos para destaque</span>
        <input
          :value="ctx.crmListUsageMinOrdersForHighlight"
          type="number"
          min="1"
          step="1"
          :disabled="!ctx.canEditCrmCommercialPolicy"
          @change="ctx.updateCrmListUsageMinOrders($event.target.value)"
        />
      </label>

      <div class="crm-policy-list">
        <div
          v-for="(tier, index) in ctx.crmListUsageTiers"
          :key="tier.id || index"
          class="crm-policy-row"
        >
          <label class="settings-field">
            <span>Nome</span>
            <input
              :value="tier.label"
              type="text"
              :disabled="!ctx.canEditCrmCommercialPolicy"
              @change="ctx.updateCrmListUsageTier(index, 'label', $event.target.value)"
            />
          </label>
          <label class="settings-field">
            <span>Minimo %</span>
            <input
              :value="tier.minRate"
              type="number"
              min="0"
              max="100"
              step="0.1"
              :disabled="!ctx.canEditCrmCommercialPolicy"
              @change="ctx.updateCrmListUsageTier(index, 'minRate', $event.target.value)"
            />
          </label>
          <button
            type="button"
            class="crm-payout__icon-btn"
            title="Remover faixa"
            aria-label="Remover faixa"
            :disabled="!ctx.canEditCrmCommercialPolicy"
            @click="ctx.removeCrmListUsageTier(index)"
          >
            <span class="material-icons-round" aria-hidden="true">delete_outline</span>
          </button>
        </div>
      </div>

      <button
        type="button"
        class="settings-action-btn"
        :disabled="!ctx.canEditCrmCommercialPolicy"
        @click="ctx.addCrmListUsageTier"
      >
        Adicionar faixa
      </button>
    </article>

    <article class="settings-card settings-card--wide">
      <header class="settings-card__header">
        <h3 class="settings-card__title">Recebimento por atingimento de meta</h3>
      </header>

      <div class="crm-payout__groups">
        <details
          v-for="group in payoutGroups"
          :key="group.id"
          class="settings-collapse"
          :open="group.id === 'consultant'"
        >
          <summary class="settings-collapse__summary">
            <div class="settings-collapse__title-wrap">
              <strong class="settings-collapse__title">{{ group.label }}</strong>
              <span class="settings-collapse__text">Faixas por atingimento de meta</span>
            </div>
            <span class="settings-collapse__meta">{{ groupSummary(group.id) }}</span>
            <span class="material-icons-round settings-collapse__icon" aria-hidden="true">
              expand_more
            </span>
          </summary>

          <div class="settings-collapse__body">
            <p v-if="!draft[group.id]?.length" class="crm-payout__empty">
              Nenhuma faixa configurada. Adicione uma faixa para definir o recebimento.
            </p>

            <div
              v-for="rule in draft[group.id]"
              :key="rule._id"
              class="crm-policy-row crm-policy-row--payout"
            >
              <label class="settings-field">
                <span>Meta %</span>
                <input
                  v-model="rule.threshold"
                  type="number"
                  min="0"
                  step="0.1"
                  :disabled="!ctx.canEditCrmCommercialPolicy"
                  @input="markDirty(group.id)"
                  @blur="saveGroup(group.id)"
                />
              </label>
              <label class="settings-field">
                <span>Tipo</span>
                <select
                  v-model="rule.mode"
                  :disabled="!ctx.canEditCrmCommercialPolicy"
                  @change="changeMode(group.id)"
                >
                  <option value="percent">%</option>
                  <option value="amount">R$</option>
                </select>
              </label>
              <label class="settings-field">
                <span>Valor</span>
                <input
                  v-model="rule.value"
                  type="number"
                  min="0"
                  :step="payoutValueStep(rule)"
                  :disabled="!ctx.canEditCrmCommercialPolicy"
                  @input="markDirty(group.id)"
                  @blur="saveGroup(group.id)"
                />
              </label>
              <button
                type="button"
                class="crm-payout__icon-btn"
                title="Remover faixa"
                aria-label="Remover faixa"
                :disabled="!ctx.canEditCrmCommercialPolicy"
                @click="removeRule(group.id, rule._id)"
              >
                <span class="material-icons-round" aria-hidden="true">delete_outline</span>
              </button>
            </div>

            <div class="crm-payout__actions">
              <button
                type="button"
                class="settings-mini-btn"
                :disabled="!ctx.canEditCrmCommercialPolicy"
                @click="addRule(group.id)"
              >
                Adicionar
              </button>
              <button
                type="button"
                class="settings-action-btn"
                :disabled="!ctx.canEditCrmCommercialPolicy || !dirty[group.id]"
                @click="saveGroup(group.id)"
              >
                Salvar faixas
              </button>
            </div>
          </div>
        </details>
      </div>
    </article>

    <SettingsCrmConsultantRules :ctx="ctx" />
  </div>
</template>

<style scoped>
.crm-policy-list {
  display: grid;
  gap: 0.65rem;
}

.crm-policy-row {
  display: grid;
  grid-template-columns: minmax(140px, 1fr) 120px auto;
  gap: 0.65rem;
  align-items: end;
}

.crm-policy-row--payout {
  grid-template-columns: 110px 90px 110px auto;
}

.crm-payout__groups {
  display: grid;
  gap: 0.65rem;
}

.crm-payout__empty {
  margin: 0;
  font-size: 0.78rem;
  color: var(--text-muted);
}

.crm-payout__actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 0.5rem;
  margin-top: 0.25rem;
}

.settings-action-btn,
.settings-mini-btn {
  border: none;
  border-radius: 10px;
  background: rgb(var(--primary) / 0.12);
  color: rgb(var(--primary));
  font-weight: 800;
  cursor: pointer;
}

.settings-action-btn {
  min-height: 40px;
  padding: 0.65rem 0.9rem;
}

.settings-mini-btn {
  min-height: 34px;
  padding: 0.45rem 0.7rem;
  font-size: 0.76rem;
}

.crm-payout__icon-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 34px;
  min-width: 34px;
  border: 1px solid var(--line-soft);
  border-radius: 10px;
  background: rgb(var(--surface));
  color: var(--text-muted);
  cursor: pointer;
  transition:
    color 0.18s ease,
    border-color 0.18s ease,
    background 0.18s ease;
}

.crm-payout__icon-btn:hover:not(:disabled) {
  color: rgb(var(--danger));
  border-color: rgb(var(--danger) / 0.4);
  background: rgb(var(--danger) / 0.08);
}

.crm-payout__icon-btn .material-icons-round {
  font-size: 1.1rem;
}

.settings-action-btn:disabled,
.settings-mini-btn:disabled,
.crm-payout__icon-btn:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

@media (max-width: 760px) {
  .crm-policy-row,
  .crm-policy-row--payout {
    grid-template-columns: 1fr;
  }
}
</style>
