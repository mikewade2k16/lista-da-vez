<script setup>
import { computed } from 'vue'

const props = defineProps({
  ctx: {
    type: Object,
    required: true,
  },
})

function activeCount(ids) {
  const settings = props.ctx.state.settings || {}
  const active = ids.filter((id) => Number(settings[id] || 0) > 0).length
  return `${active}/${ids.length} ativos`
}

const conversionMeta = computed(() =>
  activeCount(['alertMinConversionRate', 'alertMaxQueueJumpRate']),
)
const valueMeta = computed(() => activeCount(['alertMinPaScore', 'alertMinTicketAverage']))
</script>

<template>
  <div class="settings-grid">
    <article class="settings-card">
      <details class="settings-collapse">
        <summary class="settings-collapse__summary">
          <div class="settings-collapse__title-wrap">
            <strong class="settings-collapse__title">Conversao e fila</strong>
            <span class="settings-collapse__text">Limites de conversao e fora da vez</span>
          </div>
          <span class="settings-collapse__meta">{{ conversionMeta }}</span>
          <span class="material-icons-round settings-collapse__icon" aria-hidden="true">
            expand_more
          </span>
        </summary>

        <div class="settings-collapse__body">
          <p class="settings-card__text settings-alerts__intro">
            Consultores abaixo (ou acima) desses limites no mes atual aparecem como alertas em
            /ranking. Deixe em 0 para desativar.
          </p>
          <label class="settings-field">
            <span>Conversao minima (%)</span>
            <input
              :value="Number(ctx.state.settings.alertMinConversionRate || 0)"
              type="number"
              min="0"
              max="100"
              step="1"
              :disabled="!ctx.canEditSettings"
              @change="ctx.updateNumericSetting('alertMinConversionRate', $event.target.value)"
            />
          </label>
          <label class="settings-field">
            <span>Fora da vez maximo (%)</span>
            <input
              :value="Number(ctx.state.settings.alertMaxQueueJumpRate || 0)"
              type="number"
              min="0"
              max="100"
              step="1"
              :disabled="!ctx.canEditSettings"
              @change="ctx.updateNumericSetting('alertMaxQueueJumpRate', $event.target.value)"
            />
          </label>
        </div>
      </details>
    </article>

    <article class="settings-card">
      <details class="settings-collapse">
        <summary class="settings-collapse__summary">
          <div class="settings-collapse__title-wrap">
            <strong class="settings-collapse__title">Valores por atendimento</strong>
            <span class="settings-collapse__text">Limites de P.A. e ticket medio</span>
          </div>
          <span class="settings-collapse__meta">{{ valueMeta }}</span>
          <span class="material-icons-round settings-collapse__icon" aria-hidden="true">
            expand_more
          </span>
        </summary>

        <div class="settings-collapse__body">
          <p class="settings-card__text settings-alerts__intro">
            Consultores abaixo desses limites no mes atual aparecem como alertas em /ranking. Deixe
            em 0 para desativar.
          </p>
          <label class="settings-field">
            <span>P.A. minimo</span>
            <input
              :value="Number(ctx.state.settings.alertMinPaScore || 0)"
              type="number"
              min="0"
              step="0.1"
              :disabled="!ctx.canEditSettings"
              @change="ctx.updateNumericSetting('alertMinPaScore', $event.target.value)"
            />
          </label>
          <label class="settings-field">
            <span>Ticket medio minimo (R$)</span>
            <input
              :value="Number(ctx.state.settings.alertMinTicketAverage || 0)"
              type="number"
              min="0"
              step="100"
              :disabled="!ctx.canEditSettings"
              @change="ctx.updateNumericSetting('alertMinTicketAverage', $event.target.value)"
            />
          </label>
        </div>
      </details>
    </article>
  </div>
</template>

<style scoped>
.settings-alerts__intro {
  margin: 0 0 0.75rem;
}
</style>
