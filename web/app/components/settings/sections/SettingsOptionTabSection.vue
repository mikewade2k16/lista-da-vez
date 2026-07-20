<script setup>
import { computed } from 'vue'

import AppSelectField from '~/components/ui/AppSelectField.vue'
import SettingsOptionManager from '~/components/settings/SettingsOptionManager.vue'

const props = defineProps({
  config: {
    type: Object,
    required: true,
  },
  ctx: {
    type: Object,
    required: true,
  },
})

const selectionModeLabel = computed(() => {
  if (!props.config.selectionKey) return ''
  const value =
    props.ctx.state.modalConfig[props.config.selectionKey] || props.config.selectionDefault
  const option = props.ctx.fieldSelectionOptions?.find((item) => item.value === value)
  return option?.label || value
})
</script>

<template>
  <div :class="{ 'settings-grid': config.selectionKey }">
    <article v-if="config.selectionKey" class="settings-card">
      <details class="settings-collapse">
        <summary class="settings-collapse__summary">
          <div class="settings-collapse__title-wrap">
            <strong class="settings-collapse__title">Comportamento do campo</strong>
            <span class="settings-collapse__text">
              Defina aqui como o campo aparece no modal antes de cadastrar as opcoes.
            </span>
          </div>
          <span class="settings-collapse__meta">{{ selectionModeLabel }}</span>
          <span class="material-icons-round settings-collapse__icon" aria-hidden="true">
            expand_more
          </span>
        </summary>

        <div class="settings-collapse__body">
          <AppSelectField
            class="settings-field"
            label="Selecao"
            :model-value="ctx.state.modalConfig[config.selectionKey] || config.selectionDefault"
            :options="ctx.fieldSelectionOptions"
            :disabled="!ctx.canEditSettings"
            @update:model-value="ctx.updateModalConfigValue(config.selectionKey, $event)"
          />
          <AppSelectField
            class="settings-field"
            label="Descricao"
            :model-value="ctx.state.modalConfig[config.detailKey] || config.detailDefault"
            :options="ctx.fieldDetailModeOptions"
            :disabled="!ctx.canEditSettings"
            @update:model-value="ctx.updateModalConfigValue(config.detailKey, $event)"
          />
        </div>
      </details>
    </article>

    <SettingsOptionManager
      :title="config.title"
      :description="config.description"
      :items="ctx.state[config.itemsKey] || []"
      :disabled="!ctx.canEditSettings"
      :add-placeholder="config.addPlaceholder"
      :testid="config.testid"
      @add="ctx.addOption(config.group, $event)"
      @update="(optionId, label) => ctx.updateOption(config.group, optionId, label)"
      @remove="ctx.removeOption(config.group, $event)"
      @reorder="ctx.reorderOption(config.group, $event)"
    />
  </div>
</template>
