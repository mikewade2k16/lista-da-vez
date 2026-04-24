<script setup>
defineProps({
  templates: {
    type: Array,
    default: () => []
  },
  selectedOperationTemplateId: {
    type: String,
    default: ""
  },
  disabled: {
    type: Boolean,
    default: false
  }
});

defineEmits(["apply"]);
</script>

<template>
  <section class="settings-grid">
    <article v-for="template in templates" :key="template.id" class="settings-card">
      <header class="settings-card__header">
        <h3 class="settings-card__title">{{ template.label }}</h3>
        <p class="settings-card__text">{{ template.description }}</p>
      </header>
      <div class="option-list">
        <span class="insight-tag">Max simultaneo <strong>{{ template.settings.maxConcurrentServices }}</strong></span>
        <span class="insight-tag">Fechamento rapido <strong>{{ template.settings.timingFastCloseMinutes }} min</strong></span>
        <span class="insight-tag">Atendimento demorado <strong>{{ template.settings.timingLongServiceMinutes }} min</strong></span>
      </div>
      <button class="option-add__button" type="button" :disabled="disabled" @click="$emit('apply', template.id)">
        {{ selectedOperationTemplateId === template.id ? "Template ativo" : "Aplicar template" }}
      </button>
    </article>
  </section>
</template>
