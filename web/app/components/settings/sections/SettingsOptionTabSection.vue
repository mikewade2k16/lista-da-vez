<script setup>
import AppSelectField from '~/components/ui/AppSelectField.vue'
import SettingsOptionManager from '~/components/settings/SettingsOptionManager.vue'

defineProps({
  config: {
    type: Object,
    required: true,
  },
  ctx: {
    type: Object,
    required: true,
  },
})
</script>

<template>
  <div :class="{ 'settings-grid': config.selectionKey }">
    <article v-if="config.selectionKey" class="settings-card">
      <header class="settings-card__header">
        <h3 class="settings-card__title">Comportamento do campo</h3>
        <p class="settings-card__text">
          Defina aqui como o campo aparece no modal antes de cadastrar as opcoes.
        </p>
      </header>
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
