<script setup>
import { computed } from 'vue'

import SettingsOperationTemplateManager from '~/components/settings/SettingsOperationTemplateManager.vue'

const props = defineProps({
  ctx: {
    type: Object,
    required: true,
  },
})

const scoreWeightTotal = computed(() => {
  const settings = props.ctx?.state?.settings || {}
  return (
    Number(settings.scoreWeightConversion ?? 35) +
    Number(settings.scoreWeightSoldValue ?? 25) +
    Number(settings.scoreWeightQuality ?? 20) +
    Number(settings.scoreWeightPa ?? 15) +
    Number(settings.scoreWeightQueueDiscipline ?? 5)
  )
})
</script>

<template>
  <div>
    <SettingsOperationTemplateManager
      :templates="ctx.state.operationTemplates || []"
      :selected-operation-template-id="ctx.state.selectedOperationTemplateId"
      :disabled="!ctx.canEditSettings"
      @apply="ctx.applyTemplate"
    />

    <div class="settings-grid" style="margin-top: 16px">
      <article class="settings-card">
        <header class="settings-card__header">
          <h3 class="settings-card__title">Limites e timings</h3>
        </header>
        <label class="settings-field">
          <span>Atendimentos simultaneos</span>
          <input
            :value="Number(ctx.state.settings.maxConcurrentServices || 10)"
            type="number"
            min="1"
            max="100"
            :disabled="!ctx.canEditSettings"
            @change="ctx.updateNumericSetting('maxConcurrentServices', $event.target.value)"
          />
        </label>
        <label class="settings-field">
          <span>Atendimentos em aberto por consultor</span>
          <input
            :value="Number(ctx.state.settings.maxConcurrentServicesPerConsultant || 1)"
            type="number"
            min="1"
            :max="ctx.maxParallelPerConsultantLimit"
            :disabled="!ctx.canEditSettings"
            @change="
              ctx.updateNumericSetting('maxConcurrentServicesPerConsultant', $event.target.value)
            "
          />
        </label>
        <p class="settings-card__text">
          Quantos atendimentos cada consultor pode manter em aberto antes de encerrar os anteriores.
          Limite atual: 1 a {{ ctx.maxParallelPerConsultantLimit }}.
        </p>
        <label class="settings-field">
          <span>Fechamento rapido (min)</span>
          <input
            :value="Number(ctx.state.settings.timingFastCloseMinutes || 5)"
            type="number"
            min="1"
            max="120"
            :disabled="!ctx.canEditSettings"
            @change="ctx.updateNumericSetting('timingFastCloseMinutes', $event.target.value)"
          />
        </label>
        <label class="settings-field">
          <span>Atendimento demorado (min)</span>
          <input
            :value="Number(ctx.state.settings.timingLongServiceMinutes || 25)"
            type="number"
            min="1"
            max="240"
            :disabled="!ctx.canEditSettings"
            @change="ctx.updateNumericSetting('timingLongServiceMinutes', $event.target.value)"
          />
        </label>
        <label class="settings-field">
          <span>Venda baixa (R$)</span>
          <input
            :value="Number(ctx.state.settings.timingLowSaleAmount || 1200)"
            type="number"
            min="1"
            step="1"
            :disabled="!ctx.canEditSettings"
            @change="ctx.updateNumericSetting('timingLowSaleAmount', $event.target.value)"
          />
        </label>
        <label class="settings-field">
          <span>Janela de cancelamento (seg)</span>
          <input
            :value="Number(ctx.state.settings.serviceCancelWindowSeconds || 30)"
            type="number"
            min="0"
            max="300"
            :disabled="!ctx.canEditSettings"
            @change="ctx.updateNumericSetting('serviceCancelWindowSeconds', $event.target.value)"
          />
        </label>
        <p class="settings-card__text">
          Dentro dessa janela, o botao principal troca para cancelar atendimento e desfaz o inicio
          sem encerrar o fluxo completo.
        </p>
        <label class="settings-toggle">
          <input
            :checked="Boolean(ctx.state.settings.testModeEnabled)"
            type="checkbox"
            :disabled="!ctx.canEditSettings"
            @change="ctx.updateBooleanSetting('testModeEnabled', $event.target.checked)"
          />
          <span>Modo teste</span>
        </label>
        <label class="settings-toggle">
          <input
            :checked="Boolean(ctx.state.settings.autoFillFinishModal)"
            type="checkbox"
            :disabled="!ctx.canEditSettings"
            @change="ctx.updateBooleanSetting('autoFillFinishModal', $event.target.checked)"
          />
          <span>Preencher modal automaticamente</span>
        </label>
      </article>

      <article class="settings-card">
        <header class="settings-card__header">
          <h3 class="settings-card__title">Score 360</h3>
          <p class="settings-card__text">
            Define o peso de cada componente do Score 360 usado no ranking e nos cards de
            consultor.
          </p>
        </header>
        <label class="settings-field">
          <span>Conversao</span>
          <input
            :value="Number(ctx.state.settings.scoreWeightConversion ?? 35)"
            type="number"
            min="0"
            step="1"
            :disabled="!ctx.canEditSettings"
            @change="ctx.updateNumericSetting('scoreWeightConversion', $event.target.value)"
          />
        </label>
        <label class="settings-field">
          <span>Valor vendido</span>
          <input
            :value="Number(ctx.state.settings.scoreWeightSoldValue ?? 25)"
            type="number"
            min="0"
            step="1"
            :disabled="!ctx.canEditSettings"
            @change="ctx.updateNumericSetting('scoreWeightSoldValue', $event.target.value)"
          />
        </label>
        <label class="settings-field">
          <span>Qualidade</span>
          <input
            :value="Number(ctx.state.settings.scoreWeightQuality ?? 20)"
            type="number"
            min="0"
            step="1"
            :disabled="!ctx.canEditSettings"
            @change="ctx.updateNumericSetting('scoreWeightQuality', $event.target.value)"
          />
        </label>
        <label class="settings-field">
          <span>P.A.</span>
          <input
            :value="Number(ctx.state.settings.scoreWeightPa ?? 15)"
            type="number"
            min="0"
            step="1"
            :disabled="!ctx.canEditSettings"
            @change="ctx.updateNumericSetting('scoreWeightPa', $event.target.value)"
          />
        </label>
        <label class="settings-field">
          <span>Disciplina de fila</span>
          <input
            :value="Number(ctx.state.settings.scoreWeightQueueDiscipline ?? 5)"
            type="number"
            min="0"
            step="1"
            :disabled="!ctx.canEditSettings"
            @change="ctx.updateNumericSetting('scoreWeightQueueDiscipline', $event.target.value)"
          />
        </label>
        <p class="settings-card__text">
          Total atual: {{ scoreWeightTotal }}. Use 100 para manter a escala padrao; 0 desliga um
          componente.
        </p>
      </article>
    </div>
  </div>
</template>
