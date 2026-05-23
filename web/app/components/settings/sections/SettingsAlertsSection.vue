<script setup>
defineProps({
  ctx: {
    type: Object,
    required: true,
  },
})
</script>

<template>
  <div class="settings-grid">
    <article class="settings-card">
      <header class="settings-card__header">
        <h3 class="settings-card__title">Limites de desempenho</h3>
        <p class="settings-card__text">
          Consultores que ficarem abaixo (ou acima) desses limites no mes atual aparecem como
          alertas em /ranking. Deixe em 0 para desativar.
        </p>
      </header>
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
    </article>
  </div>
</template>
