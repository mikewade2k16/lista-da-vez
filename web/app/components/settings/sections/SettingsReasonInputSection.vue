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

const currentModeLabel = computed(() => {
  const value = props.ctx.state.modalConfig[props.config.modeKey] || 'text'
  const option = props.ctx.reasonInputModeOptions?.find((item) => item.value === value)
  return option?.label || value
})
</script>

<template>
  <div class="settings-grid">
    <article class="settings-card">
      <details class="settings-collapse">
        <summary class="settings-collapse__summary">
          <div class="settings-collapse__title-wrap">
            <strong class="settings-collapse__title">{{ config.title }}</strong>
            <span class="settings-collapse__text">{{ config.text }}</span>
          </div>
          <span class="settings-collapse__meta">{{ currentModeLabel }}</span>
          <span class="material-icons-round settings-collapse__icon" aria-hidden="true">
            expand_more
          </span>
        </summary>

        <div class="settings-collapse__body">
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
      </details>
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
