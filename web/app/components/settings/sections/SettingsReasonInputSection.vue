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
  <div class="settings-grid">
    <article class="settings-card">
      <header class="settings-card__header">
        <h3 class="settings-card__title">{{ config.title }}</h3>
        <p class="settings-card__text">{{ config.text }}</p>
      </header>
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
    </article>

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
