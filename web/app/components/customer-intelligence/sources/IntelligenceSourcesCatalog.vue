<script setup lang="ts">
import { ref } from 'vue'
import CustomerIntelligenceStatus from '~/components/customer-intelligence/CustomerIntelligenceStatus.vue'
import AppToggleSwitch from '~/components/ui/AppToggleSwitch.vue'
import { useCustomerIntelligenceAccess } from '~/composables/customer-intelligence/useCustomerIntelligenceAccess'
import { useCustomerIntelligenceSources } from '~/composables/customer-intelligence/useCustomerIntelligenceSources'
import type {
  IntelligenceSourceConfig,
  IntelligenceSourceDescriptor,
  IntelligenceSourceDraft,
} from '~/domain/customer-intelligence/sources'
import IntelligenceSourceConfigDrawer from './IntelligenceSourceConfigDrawer.vue'
import IntelligenceSourceHealth from './IntelligenceSourceHealth.vue'

const access = useCustomerIntelligenceAccess()
const sourcesState = useCustomerIntelligenceSources()
const drawerOpen = ref(false)
const selectedDescriptor = ref<IntelligenceSourceDescriptor | null>(null)

function sourceFor(descriptor: IntelligenceSourceDescriptor): IntelligenceSourceConfig | null {
  return (
    sourcesState.sources.value.find((source) => source.sourceKey === descriptor.sourceKey) ?? null
  )
}

function configure(descriptor: IntelligenceSourceDescriptor): void {
  selectedDescriptor.value = descriptor
  drawerOpen.value = true
}

function toggleDescriptor(descriptor: IntelligenceSourceDescriptor, enabled: boolean): void {
  const source = sourceFor(descriptor)
  if (source) void sourcesState.toggle(source, enabled)
}

function testDescriptor(descriptor: IntelligenceSourceDescriptor): void {
  const source = sourceFor(descriptor)
  if (source) void sourcesState.test(source)
}

async function save(draft: IntelligenceSourceDraft): Promise<void> {
  if (!selectedDescriptor.value) return
  const saved = await sourcesState.save(
    selectedDescriptor.value,
    sourceFor(selectedDescriptor.value),
    draft,
  )
  if (saved) drawerOpen.value = false
}
</script>

<template>
  <div class="source-catalog">
    <CustomerIntelligenceStatus
      v-if="sourcesState.loading.value"
      title="Carregando catalogo de fontes"
      loading
    />
    <CustomerIntelligenceStatus
      v-else-if="sourcesState.error.value"
      title="Fontes indisponiveis"
      :error="sourcesState.error.value"
    />
    <CustomerIntelligenceStatus
      v-else-if="!sourcesState.catalog.value.length"
      title="Catalogo vazio"
      empty
      empty-text="Nenhum descriptor de fonte foi publicado pelo backend."
    />
    <article
      v-for="descriptor in sourcesState.catalog.value"
      v-else
      :key="descriptor.sourceKey"
      class="source-card"
    >
      <header>
        <div>
          <small>{{ descriptor.category }} · {{ descriptor.moduleId }}</small>
          <h3>{{ descriptor.name }}</h3>
          <p>{{ descriptor.description }}</p>
        </div>
        <AppToggleSwitch
          v-if="sourceFor(descriptor)"
          :model-value="sourceFor(descriptor)?.enabled === true"
          :disabled="!access.canManageSources.value"
          label="Habilitada"
          @update:model-value="toggleDescriptor(descriptor, $event)"
        />
      </header>
      <IntelligenceSourceHealth :source="sourceFor(descriptor)" />
      <div class="source-card__meta">
        <span v-for="purpose in descriptor.purposeKeys" :key="purpose">{{ purpose }}</span>
      </div>
      <footer>
        <button type="button" @click="configure(descriptor)">
          {{ sourceFor(descriptor) ? 'Configurar' : 'Criar fonte' }}
        </button>
        <button
          v-if="sourceFor(descriptor)"
          type="button"
          :disabled="!access.canManageSources.value"
          @click="testDescriptor(descriptor)"
        >
          Sincronizar
        </button>
      </footer>
    </article>

    <IntelligenceSourceConfigDrawer
      v-model:open="drawerOpen"
      :descriptor="selectedDescriptor"
      :source="selectedDescriptor ? sourceFor(selectedDescriptor) : null"
      :saving="sourcesState.saving.value"
      :can-manage="access.canManageSources.value"
      @save="save"
    />
  </div>
</template>

<style scoped>
.source-catalog {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 1rem;
}

.source-card {
  display: grid;
  gap: 0.8rem;
  padding: 1rem;
  border: 1px solid rgb(var(--border) / 0.8);
  border-radius: 1rem;
  background: rgb(var(--surface) / 0.9);
}

.source-card header,
.source-card footer {
  display: flex;
  justify-content: space-between;
  gap: 0.75rem;
}

.source-card h3,
.source-card p {
  margin: 0.2rem 0;
}

.source-card small,
.source-card p {
  color: rgb(var(--muted));
}

.source-card__meta {
  display: flex;
  gap: 0.35rem;
  flex-wrap: wrap;
}

.source-card__meta span {
  padding: 0.2rem 0.45rem;
  border-radius: 999px;
  background: rgb(var(--primary) / 0.1);
  font-size: 0.7rem;
}

.source-card button {
  min-height: 2.2rem;
  padding: 0 0.75rem;
  border: 1px solid rgb(var(--primary) / 0.25);
  border-radius: 999px;
  background: transparent;
  color: rgb(var(--primary));
  font-weight: 700;
}

@media (max-width: 850px) {
  .source-catalog {
    grid-template-columns: 1fr;
  }
}
</style>
