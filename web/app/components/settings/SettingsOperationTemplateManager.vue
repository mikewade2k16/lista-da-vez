<script setup>
import { computed } from 'vue'

const props = defineProps({
  templates: {
    type: Array,
    default: () => [],
  },
  selectedOperationTemplateId: {
    type: String,
    default: '',
  },
  disabled: {
    type: Boolean,
    default: false,
  },
})

defineEmits(['apply'])

const activeTemplateLabel = computed(() => {
  const active = props.templates.find(
    (template) => template.id === props.selectedOperationTemplateId,
  )
  return active ? active.label : 'Nenhum aplicado'
})
</script>

<template>
  <section class="settings-grid">
    <article class="settings-card">
      <details class="settings-collapse">
        <summary class="settings-collapse__summary">
          <div class="settings-collapse__title-wrap">
            <strong class="settings-collapse__title">Templates de operacao</strong>
            <span class="settings-collapse__text">Presets rapidos de configuracao da operacao</span>
          </div>
          <span class="settings-collapse__meta">{{ activeTemplateLabel }}</span>
          <span class="material-icons-round settings-collapse__icon" aria-hidden="true">
            expand_more
          </span>
        </summary>

        <div class="settings-collapse__body">
          <div class="operation-templates">
            <article
              v-for="template in templates"
              :key="template.id"
              class="operation-templates__card"
              :class="{ 'is-active': selectedOperationTemplateId === template.id }"
            >
              <header class="settings-card__header">
                <h3 class="settings-card__title">{{ template.label }}</h3>
                <p class="settings-card__text">{{ template.description }}</p>
              </header>
              <div class="option-list">
                <span class="insight-tag">
                  Max simultaneo
                  <strong>{{ template.settings.maxConcurrentServices }}</strong>
                </span>
                <span class="insight-tag">
                  Fechamento rapido
                  <strong>{{ template.settings.timingFastCloseMinutes }} min</strong>
                </span>
                <span class="insight-tag">
                  Atendimento demorado
                  <strong>{{ template.settings.timingLongServiceMinutes }} min</strong>
                </span>
              </div>
              <button
                class="option-add__button"
                type="button"
                :disabled="disabled"
                @click="$emit('apply', template.id)"
              >
                {{
                  selectedOperationTemplateId === template.id
                    ? 'Template ativo'
                    : 'Aplicar template'
                }}
              </button>
            </article>
          </div>
        </div>
      </details>
    </article>
  </section>
</template>

<style scoped>
.operation-templates {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 12px;
}

.operation-templates__card {
  display: grid;
  align-content: start;
  gap: 10px;
  padding: 12px;
  border: 1px solid var(--line-soft);
  border-radius: 14px;
  background: rgb(var(--surface-2) / 0.5);
}

.operation-templates__card.is-active {
  border-color: rgb(var(--primary) / 0.6);
  background: rgb(var(--primary) / 0.08);
}
</style>
