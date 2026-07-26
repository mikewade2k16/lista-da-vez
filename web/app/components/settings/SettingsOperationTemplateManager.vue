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
  <section class="settings-grid operation-template-manager">
    <article class="settings-card operation-template-manager__shell">
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
.operation-template-manager {
  grid-template-columns: 1fr;
  gap: 0;
}

.operation-template-manager__shell {
  gap: 0;
  padding: 0;
  border: 0;
  border-radius: 0;
  background: transparent;
}

.operation-template-manager .settings-collapse {
  border-radius: 12px;
}

.operation-template-manager .settings-collapse__summary {
  gap: 8px;
  min-height: 44px;
  padding: 7px 10px;
}

.operation-template-manager .settings-collapse__title-wrap {
  gap: 2px;
}

.operation-template-manager .settings-collapse__title {
  font-size: 0.8rem;
}

.operation-template-manager .settings-collapse__text {
  font-size: 0.68rem;
  line-height: 1.25;
}

.operation-template-manager .settings-collapse__meta {
  min-height: 24px;
  padding-inline: 8px;
  font-size: 0.68rem;
}

.operation-template-manager .settings-collapse__icon {
  font-size: 18px;
}

.operation-template-manager .settings-collapse__body {
  gap: 8px;
  padding: 8px 10px 10px;
}

.operation-templates {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 8px;
}

.operation-templates__card {
  display: grid;
  align-content: start;
  gap: 7px;
  padding: 9px;
  border: 1px solid var(--line-soft);
  border-radius: 10px;
  background: rgb(var(--surface-2) / 0.5);
}

.operation-templates__card.is-active {
  border-color: rgb(var(--primary) / 0.6);
  background: rgb(var(--primary) / 0.08);
}

.operation-templates__card .settings-card__header {
  gap: 2px;
}

.operation-templates__card .settings-card__title {
  font-size: 0.8rem;
}

.operation-templates__card .settings-card__text {
  font-size: 0.68rem;
  line-height: 1.25;
}

.operation-templates__card .option-list {
  gap: 5px;
}

.operation-templates__card .insight-tag {
  gap: 4px;
  padding: 4px 7px;
  font-size: 0.68rem;
}

.operation-templates__card .option-add__button {
  min-height: 30px;
  padding-inline: 10px;
  border-radius: 8px;
  cursor: pointer;
}

.operation-templates__card .option-add__button:disabled {
  cursor: not-allowed;
}

@media (max-width: 760px) {
  .operation-template-manager .settings-collapse__summary {
    grid-template-columns: minmax(0, 1fr) auto;
  }

  .operation-template-manager .settings-collapse__meta {
    grid-column: 1 / -1;
  }
}
</style>
