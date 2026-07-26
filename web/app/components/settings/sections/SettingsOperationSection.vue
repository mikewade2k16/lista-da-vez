<script setup>
import SettingsOperationTemplateManager from '~/components/settings/SettingsOperationTemplateManager.vue'

defineProps({
  ctx: {
    type: Object,
    required: true,
  },
})
</script>

<template>
  <div class="settings-operation">
    <SettingsOperationTemplateManager
      :templates="ctx.state.operationTemplates || []"
      :selected-operation-template-id="ctx.state.selectedOperationTemplateId"
      :disabled="!ctx.canEditSettings"
      @apply="ctx.applyTemplate"
    />

    <div class="settings-operation__sections">
      <article class="settings-card settings-operation__card">
        <header class="settings-card__header">
          <h3 class="settings-card__title">Capacidade e fila</h3>
          <p class="settings-card__text">Atendimentos simultaneos e por consultor</p>
        </header>

        <div class="settings-operation__fields settings-operation__fields--capacity">
          <div class="settings-operation__field-wrap">
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
          </div>
          <div class="settings-operation__field-wrap">
            <label class="settings-field">
              <span>Atendimentos em aberto por consultor</span>
              <input
                :value="Number(ctx.state.settings.maxConcurrentServicesPerConsultant || 1)"
                type="number"
                min="1"
                :max="ctx.maxParallelPerConsultantLimit"
                :disabled="!ctx.canEditSettings"
                @change="
                  ctx.updateNumericSetting(
                    'maxConcurrentServicesPerConsultant',
                    $event.target.value,
                  )
                "
              />
            </label>
            <p class="settings-card__text">
              Quantos atendimentos cada consultor pode manter em aberto antes de encerrar os
              anteriores. Limite atual: 1 a {{ ctx.maxParallelPerConsultantLimit }}.
            </p>
          </div>
        </div>
      </article>

      <article class="settings-card settings-operation__card">
        <header class="settings-card__header">
          <h3 class="settings-card__title">Tempos e alertas</h3>
          <p class="settings-card__text">Limiares de fechamento, demora e venda baixa</p>
        </header>

        <div class="settings-operation__fields settings-operation__fields--timings">
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
        </div>
      </article>

      <article class="settings-card settings-operation__card">
        <header class="settings-card__header">
          <h3 class="settings-card__title">Comportamento do atendimento</h3>
          <p class="settings-card__text">Janela de cancelamento e automacoes</p>
        </header>

        <div class="settings-operation__fields settings-operation__fields--behavior">
          <div class="settings-operation__field-wrap">
            <label class="settings-field">
              <span>Janela de cancelamento (seg)</span>
              <input
                :value="Number(ctx.state.settings.serviceCancelWindowSeconds || 30)"
                type="number"
                min="0"
                max="300"
                :disabled="!ctx.canEditSettings"
                @change="
                  ctx.updateNumericSetting('serviceCancelWindowSeconds', $event.target.value)
                "
              />
            </label>
            <p class="settings-card__text">
              Dentro dessa janela, o botao principal troca para cancelar atendimento e desfaz o
              inicio sem encerrar o fluxo completo.
            </p>
          </div>
          <label class="settings-toggle settings-operation__toggle">
            <input
              :checked="Boolean(ctx.state.settings.testModeEnabled)"
              type="checkbox"
              :disabled="!ctx.canEditSettings"
              @change="ctx.updateBooleanSetting('testModeEnabled', $event.target.checked)"
            />
            <span>Modo teste</span>
          </label>
          <label class="settings-toggle settings-operation__toggle">
            <input
              :checked="Boolean(ctx.state.settings.autoFillFinishModal)"
              type="checkbox"
              :disabled="!ctx.canEditSettings"
              @change="ctx.updateBooleanSetting('autoFillFinishModal', $event.target.checked)"
            />
            <span>Preencher modal automaticamente</span>
          </label>
        </div>
      </article>
    </div>
  </div>
</template>

<style scoped>
.settings-operation,
.settings-operation__sections {
  display: grid;
  gap: 8px;
}

.settings-operation__card {
  grid-template-columns: minmax(190px, 0.32fr) minmax(0, 1fr);
  align-items: center;
  gap: 12px;
  padding: 9px 11px;
  border-radius: 12px;
}

.settings-operation__card .settings-card__header {
  gap: 2px;
}

.settings-operation__card .settings-card__title {
  font-size: 0.82rem;
  line-height: 1.2;
}

.settings-operation__card .settings-card__text {
  font-size: 0.68rem;
  line-height: 1.25;
}

.settings-operation__fields {
  display: grid;
  align-items: start;
  gap: 8px;
}

.settings-operation__fields--capacity {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.settings-operation__fields--timings,
.settings-operation__fields--behavior {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.settings-operation__field-wrap {
  display: grid;
  gap: 3px;
  min-width: 0;
}

.settings-operation__card .settings-field {
  gap: 4px;
  min-width: 0;
  font-size: 0.72rem;
}

.settings-operation__card .settings-field input {
  min-height: 34px;
  padding: 6px 9px;
  border-radius: 8px;
  font-size: 0.75rem;
}

.settings-operation__toggle {
  min-height: 34px;
  align-self: end;
  padding: 6px 8px;
  border-radius: 9px;
  background: rgb(var(--surface-2) / 0.72);
  font-size: 0.72rem;
}

@media (max-width: 1100px) {
  .settings-operation__card {
    grid-template-columns: 1fr;
    gap: 7px;
  }

  .settings-operation__card .settings-card__header {
    display: flex;
    align-items: baseline;
    gap: 8px;
  }
}

@media (max-width: 760px) {
  .settings-operation__fields--capacity,
  .settings-operation__fields--timings,
  .settings-operation__fields--behavior {
    grid-template-columns: 1fr;
  }

  .settings-operation__toggle {
    align-self: stretch;
  }

  .settings-operation__card .settings-card__header {
    display: grid;
    gap: 2px;
  }
}
</style>
