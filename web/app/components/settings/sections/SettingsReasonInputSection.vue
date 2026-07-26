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
  <div class="settings-reason-section">
    <section class="catalog-behavior">
      <header class="catalog-behavior__header">
        <strong>{{ config.title }}</strong>
        <span>{{ config.text }}</span>
      </header>

      <div class="catalog-behavior__fields catalog-behavior__fields--reason">
        <AppSelectField
          class="settings-field"
          label="Modo do campo"
          :model-value="ctx.state.modalConfig[config.modeKey] || 'text'"
          :options="ctx.reasonInputModeOptions"
          :disabled="!ctx.canEditSettings"
          @update:model-value="ctx.updateModalConfigValue(config.modeKey, $event)"
        />
        <label class="settings-field">
          <span>Label</span>
          <input
            :value="ctx.state.modalConfig[config.labelKey] || config.labelDefault"
            type="text"
            :disabled="!ctx.canEditSettings"
            @change="ctx.updateModalConfigValue(config.labelKey, $event.target.value)"
          />
        </label>
        <label class="settings-field">
          <span>Placeholder</span>
          <input
            :value="ctx.state.modalConfig[config.placeholderKey] || config.placeholderDefault"
            type="text"
            :disabled="!ctx.canEditSettings"
            @change="ctx.updateModalConfigValue(config.placeholderKey, $event.target.value)"
          />
        </label>
        <label class="settings-field">
          <span>Label do outro</span>
          <input
            :value="ctx.state.modalConfig[config.otherLabelKey] || config.otherLabelDefault"
            type="text"
            :disabled="!ctx.canEditSettings"
            @change="ctx.updateModalConfigValue(config.otherLabelKey, $event.target.value)"
          />
        </label>
        <label class="settings-field">
          <span>Placeholder do outro</span>
          <input
            :value="
              ctx.state.modalConfig[config.otherPlaceholderKey] || config.otherPlaceholderDefault
            "
            type="text"
            :disabled="!ctx.canEditSettings"
            @change="ctx.updateModalConfigValue(config.otherPlaceholderKey, $event.target.value)"
          />
        </label>
      </div>
    </section>

    <SettingsOptionManager
      :title="config.optionTitle"
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
.settings-reason-section {
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
  min-width: 0;
  gap: 0.45rem;
}

.catalog-behavior__fields--reason {
  grid-template-columns: repeat(5, minmax(0, 1fr));
}

.catalog-behavior__fields :deep(.settings-field) {
  gap: 0.25rem;
  font-size: 0.72rem;
}

.catalog-behavior__fields :deep(.settings-field input) {
  padding: 0.45rem 0.55rem;
}

.catalog-behavior__fields :deep(.app-select-field__trigger) {
  min-height: 34px;
}

@media (max-width: 760px) {
  .catalog-behavior__header {
    align-items: flex-start;
    flex-direction: column;
    gap: 0.15rem;
  }

  .catalog-behavior__fields--reason {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 520px) {
  .catalog-behavior__fields--reason {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
