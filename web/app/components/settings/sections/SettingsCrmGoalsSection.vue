<script setup>
const props = defineProps({
  ctx: {
    type: Object,
    required: true,
  },
})

const payoutGroups = [
  { id: 'consultant', label: 'Consultor' },
  { id: 'manager', label: 'Gerente' },
  { id: 'support', label: 'Caixa e auxiliar' },
]

function payoutRules(group) {
  return props.ctx.crmGoalPayoutPolicy?.[group] || []
}

function payoutValueStep(rule) {
  return rule?.mode === 'amount' ? '1' : '0.1'
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
            class="settings-mini-btn"
            :disabled="!ctx.canEditCrmCommercialPolicy"
            @click="ctx.removeCrmListUsageTier(index)"
          >
            Remover
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

      <section v-for="group in payoutGroups" :key="group.id" class="crm-payout-group">
        <header class="crm-payout-group__header">
          <strong>{{ group.label }}</strong>
          <button
            type="button"
            class="settings-mini-btn"
            :disabled="!ctx.canEditCrmCommercialPolicy"
            @click="ctx.addCrmGoalPayoutRule(group.id)"
          >
            Adicionar
          </button>
        </header>

        <div
          v-for="(rule, index) in payoutRules(group.id)"
          :key="`${group.id}-${index}-${rule.threshold}`"
          class="crm-policy-row crm-policy-row--payout"
        >
          <label class="settings-field">
            <span>Meta %</span>
            <input
              :value="rule.threshold"
              type="number"
              min="0"
              step="0.1"
              :disabled="!ctx.canEditCrmCommercialPolicy"
              @change="
                ctx.updateCrmGoalPayoutRule(group.id, index, 'threshold', $event.target.value)
              "
            />
          </label>
          <label class="settings-field">
            <span>Tipo</span>
            <select
              :value="rule.mode"
              :disabled="!ctx.canEditCrmCommercialPolicy"
              @change="ctx.updateCrmGoalPayoutRule(group.id, index, 'mode', $event.target.value)"
            >
              <option value="percent">%</option>
              <option value="amount">R$</option>
            </select>
          </label>
          <label class="settings-field">
            <span>Valor</span>
            <input
              :value="rule.value"
              type="number"
              min="0"
              :step="payoutValueStep(rule)"
              :disabled="!ctx.canEditCrmCommercialPolicy"
              @change="ctx.updateCrmGoalPayoutRule(group.id, index, 'value', $event.target.value)"
            />
          </label>
          <button
            type="button"
            class="settings-mini-btn"
            :disabled="!ctx.canEditCrmCommercialPolicy"
            @click="ctx.removeCrmGoalPayoutRule(group.id, index)"
          >
            Remover
          </button>
        </div>
      </section>
    </article>
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

.crm-payout-group {
  display: grid;
  gap: 0.65rem;
  padding-block: 0.85rem;
  border-top: 1px solid rgb(var(--border));
}

.crm-payout-group:first-of-type {
  border-top: none;
}

.crm-payout-group__header {
  display: flex;
  justify-content: space-between;
  gap: 0.75rem;
  align-items: center;
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

.settings-action-btn:disabled,
.settings-mini-btn:disabled {
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
