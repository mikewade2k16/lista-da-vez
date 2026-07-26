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
  <div class="settings-option-tab">
    <section v-if="config.selectionKey" class="catalog-behavior">
      <header class="catalog-behavior__header">
        <strong>Comportamento do campo</strong>
        <span>Defina como o campo aparece no modal.</span>
      </header>

      <div class="catalog-behavior__fields">
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
    </section>

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

<style scoped>
.settings-option-tab {
  display: grid;
  gap: 0.55rem;
  min-width: 0;
}

.catalog-behavior {
  display: grid;
  gap: 0.45rem;
  padding: 0.55rem 0.65rem;
  border: 1px solid var(--line-soft);
  border-radius: 11px;
  background: rgb(var(--surface-2) / 0.64);
}

.catalog-behavior__header {
  display: flex;
  align-items: baseline;
  gap: 0.45rem;
  min-width: 0;
}

.catalog-behavior__header strong {
  flex: 0 0 auto;
  color: var(--text-main);
  font-size: 0.77rem;
}

.catalog-behavior__header span {
  min-width: 0;
  color: var(--text-muted);
  font-size: 0.68rem;
  line-height: 1.2;
}

.catalog-behavior__fields {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.5rem;
}

.catalog-behavior__fields :deep(.settings-field) {
  gap: 0.25rem;
  font-size: 0.72rem;
}

.catalog-behavior__fields :deep(.app-select-field__trigger) {
  min-height: 34px;
}

@media (max-width: 600px) {
  .catalog-behavior__header {
    align-items: flex-start;
    flex-direction: column;
    gap: 0.15rem;
  }

  .catalog-behavior__fields {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
