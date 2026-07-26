<script setup>
import { computed, ref } from 'vue'

import SettingsOptionTabSection from '~/components/settings/sections/SettingsOptionTabSection.vue'
import SettingsReasonInputSection from '~/components/settings/sections/SettingsReasonInputSection.vue'

const props = defineProps({
  ctx: {
    type: Object,
    required: true,
  },
})

const catalogs = [
  {
    id: 'motivos',
    label: 'Motivos da visita',
    description: 'Intencoes informadas no encerramento do atendimento.',
    type: 'option',
    config: props.ctx.optionTabConfigs.motivos,
  },
  {
    id: 'cancelamento',
    label: 'Cancelamentos',
    description: 'Formato e motivos usados ao cancelar um atendimento.',
    type: 'reason',
    config: props.ctx.reasonInputSectionConfigs.cancelamento,
  },
  {
    id: 'pausas',
    label: 'Pausas',
    description: 'Motivos exibidos ao pausar um consultor.',
    type: 'option',
    config: props.ctx.optionTabConfigs.pausas,
  },
  {
    id: 'motivos-perda',
    label: 'Perdas',
    description: 'Motivos usados quando o atendimento termina sem venda.',
    type: 'option',
    config: props.ctx.optionTabConfigs['motivos-perda'],
  },
  {
    id: 'motivos-fora-da-vez',
    label: 'Fora da vez',
    description: 'Justificativas para atendimentos iniciados fora da fila.',
    type: 'option',
    config: props.ctx.optionTabConfigs['motivos-fora-da-vez'],
  },
  {
    id: 'origens',
    label: 'Origens',
    description: 'Canais de entrada e origem dos clientes.',
    type: 'option',
    config: props.ctx.optionTabConfigs.origens,
  },
  {
    id: 'profissoes',
    label: 'Profissoes',
    description: 'Profissoes disponiveis no cadastro do atendimento.',
    type: 'option',
    config: props.ctx.optionTabConfigs.profissoes,
  },
]

const activeCatalogId = ref(catalogs[0]?.id || '')
const activeCatalog = computed(
  () => catalogs.find((catalog) => catalog.id === activeCatalogId.value) || catalogs[0],
)

function itemCount(config) {
  const items = props.ctx.state?.[config?.itemsKey]
  return Array.isArray(items) ? items.length : 0
}
</script>

<template>
  <div class="settings-catalogs">
    <aside class="settings-catalogs__sidebar" aria-label="Tipos de motivos e cadastros">
      <span class="settings-catalogs__sidebar-title">Tipos de cadastro</span>

      <nav class="settings-catalogs__nav">
        <button
          v-for="catalog in catalogs"
          :key="catalog.id"
          class="settings-catalogs__nav-item"
          :class="{ 'is-active': activeCatalogId === catalog.id }"
          type="button"
          :aria-current="activeCatalogId === catalog.id ? 'page' : undefined"
          @click="activeCatalogId = catalog.id"
        >
          <span>{{ catalog.label }}</span>
          <span class="settings-catalogs__nav-count">{{ itemCount(catalog.config) }}</span>
        </button>
      </nav>
    </aside>

    <section v-if="activeCatalog" class="settings-catalogs__panel">
      <header class="settings-catalogs__header">
        <div class="settings-catalogs__heading">
          <h3>{{ activeCatalog.label }}</h3>
          <p>{{ activeCatalog.description }}</p>
        </div>
        <span class="settings-catalogs__count">
          {{ itemCount(activeCatalog.config) }} cadastrados
        </span>
      </header>

      <div class="settings-catalogs__content">
        <SettingsReasonInputSection
          v-if="activeCatalog.type === 'reason'"
          :ctx="ctx"
          :config="activeCatalog.config"
        />
        <SettingsOptionTabSection v-else :ctx="ctx" :config="activeCatalog.config" />
      </div>
    </section>
  </div>
</template>

<style scoped>
.settings-catalogs {
  display: grid;
  grid-template-columns: minmax(190px, 230px) minmax(0, 1fr);
  align-items: start;
  gap: 0.65rem;
  min-width: 0;
}

.settings-catalogs__sidebar,
.settings-catalogs__panel {
  border: 1px solid var(--line-soft);
  border-radius: 12px;
  background: rgb(var(--surface));
}

.settings-catalogs__sidebar {
  display: grid;
  gap: 0.35rem;
  padding: 0.5rem;
}

.settings-catalogs__sidebar-title {
  padding: 0 0.3rem;
  color: var(--text-muted);
  font-size: 0.65rem;
  font-weight: 800;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.settings-catalogs__nav {
  display: grid;
  gap: 0.15rem;
}

.settings-catalogs__nav-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  width: 100%;
  min-height: 34px;
  padding: 0.35rem 0.5rem;
  border: 1px solid transparent;
  border-radius: 9px;
  background: transparent;
  color: var(--text-muted);
  font: inherit;
  font-size: 0.74rem;
  font-weight: 700;
  text-align: left;
  cursor: pointer;
  transition:
    background 0.18s ease,
    border-color 0.18s ease,
    color 0.18s ease,
    transform 0.18s ease;
}

.settings-catalogs__nav-item:hover {
  border-color: var(--line-soft);
  background: rgb(var(--surface-2));
  color: var(--text-main);
  transform: translateX(2px);
}

.settings-catalogs__nav-item:focus-visible {
  outline: 2px solid rgb(var(--primary) / 0.48);
  outline-offset: 2px;
}

.settings-catalogs__nav-item.is-active {
  border-color: rgb(var(--primary) / 0.38);
  background: rgb(var(--primary) / 0.12);
  color: rgb(var(--primary));
}

.settings-catalogs__nav-count,
.settings-catalogs__count {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 24px;
  min-height: 21px;
  padding: 0 0.4rem;
  border-radius: 999px;
  background: rgb(var(--surface-2));
  color: var(--text-muted);
  font-size: 0.66rem;
  font-weight: 800;
  white-space: nowrap;
}

.settings-catalogs__nav-item.is-active .settings-catalogs__nav-count {
  background: rgb(var(--primary) / 0.16);
  color: rgb(var(--primary));
}

.settings-catalogs__panel {
  min-width: 0;
  overflow: visible;
}

.settings-catalogs__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.65rem;
  padding: 0.6rem 0.75rem;
  border-bottom: 1px solid var(--line-soft);
}

.settings-catalogs__heading {
  display: grid;
  gap: 0.1rem;
  min-width: 0;
}

.settings-catalogs__heading h3,
.settings-catalogs__heading p {
  margin: 0;
}

.settings-catalogs__heading h3 {
  color: var(--text-main);
  font-size: 0.86rem;
}

.settings-catalogs__heading p {
  color: var(--text-muted);
  font-size: 0.7rem;
  line-height: 1.25;
}

.settings-catalogs__count {
  background: rgb(var(--primary) / 0.12);
  color: rgb(var(--primary));
}

.settings-catalogs__content {
  min-width: 0;
  padding: 0.6rem;
}

@media (max-width: 900px) {
  .settings-catalogs {
    grid-template-columns: minmax(0, 1fr);
  }

  .settings-catalogs__sidebar {
    overflow-x: auto;
  }

  .settings-catalogs__sidebar-title {
    display: none;
  }

  .settings-catalogs__nav {
    display: flex;
    min-width: max-content;
  }

  .settings-catalogs__nav-item {
    width: auto;
    min-width: 138px;
  }

  .settings-catalogs__nav-item:hover {
    transform: translateY(-1px);
  }
}

@media (max-width: 600px) {
  .settings-catalogs__header {
    align-items: flex-start;
  }

  .settings-catalogs__count {
    display: none;
  }

  .settings-catalogs__content {
    padding: 0.5rem;
  }
}
</style>
