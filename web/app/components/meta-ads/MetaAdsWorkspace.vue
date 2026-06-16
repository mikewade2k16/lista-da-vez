<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useMetaAdsStore } from '~/stores/meta-ads'

const store = useMetaAdsStore()

interface RangeOption {
  value: string
  label: string
}

// Espelha os ranges aceitos pelo backend de insights (level=account).
const RANGE_OPTIONS: RangeOption[] = [
  { value: 'last_7d', label: '7 dias' },
  { value: 'last_14d', label: '14 dias' },
  { value: 'last_30d', label: '30 dias' },
  { value: 'last_90d', label: '90 dias' },
]

type TabId = 'overview' | 'assistant' | 'connections'

interface TabOption {
  id: TabId
  label: string
}

const TABS: TabOption[] = [
  { id: 'overview', label: 'Visao geral' },
  { id: 'assistant', label: 'Assistente' },
  { id: 'connections', label: 'Conexoes' },
]

const activeTab = ref<TabId>('overview')

function onRangeChange(event: Event) {
  const value = (event.target as HTMLSelectElement).value
  void store.setRange(value)
}

function onSync() {
  if (store.syncing) return
  void store.sync()
}

onMounted(() => {
  void store.init()
})

// `connected` so fica true depois do init() async — carrega historico + health
// do assistente assim que a conexao confirmar (e em reconexoes futuras).
watch(
  () => store.connected,
  (connected) => {
    if (connected) {
      void store.loadAssistant()
    }
  },
  { immediate: true },
)
</script>

<template>
  <section class="meta-ads">
    <header class="meta-ads__header">
      <div class="meta-ads__header-text">
        <h1 class="meta-ads__title">Meta Ads</h1>
        <p class="meta-ads__subtitle">
          Conecte o Business Manager, comande por texto e acompanhe o desempenho das campanhas.
        </p>
      </div>

      <div v-if="store.connected" class="meta-ads__actions">
        <label class="meta-ads__range">
          <span class="meta-ads__range-label">Periodo</span>
          <select
            class="meta-ads__range-select"
            :value="store.range"
            :disabled="store.pending || !store.selectedAdAccountId"
            @change="onRangeChange"
          >
            <option v-for="option in RANGE_OPTIONS" :key="option.value" :value="option.value">
              {{ option.label }}
            </option>
          </select>
        </label>

        <button
          type="button"
          class="meta-ads__sync"
          :disabled="store.syncing || !store.selectedAdAccountId"
          @click="onSync"
        >
          <span v-if="store.syncing" class="meta-ads__spinner" aria-hidden="true"></span>
          {{ store.syncing ? 'Sincronizando...' : 'Sincronizar' }}
        </button>
      </div>
    </header>

    <p v-if="store.error" class="meta-ads__error">{{ store.error }}</p>

    <MetaAdsConnectionCard v-if="!store.connected" />

    <template v-else>
      <MetaAdsAccountPicker />

      <nav class="meta-ads__tabs" role="tablist">
        <button
          v-for="tab in TABS"
          :key="tab.id"
          type="button"
          role="tab"
          class="meta-ads__tab"
          :class="{ 'meta-ads__tab--active': activeTab === tab.id }"
          :aria-selected="activeTab === tab.id"
          @click="activeTab = tab.id"
        >
          {{ tab.label }}
        </button>
      </nav>

      <div v-show="activeTab === 'overview'" class="meta-ads__panel">
        <MetaAdsOverviewCard />
        <MetaAdsReportChart />
        <MetaAdsCampaignTable />
      </div>

      <div v-show="activeTab === 'assistant'" class="meta-ads__panel">
        <MetaAdsAssistantAuth />
        <MetaAdsAssistantCard />
        <MetaAdsAssistantSettings />
      </div>

      <div v-show="activeTab === 'connections'" class="meta-ads__panel">
        <MetaAdsConnectionCard />
      </div>
    </template>
  </section>
</template>

<style scoped>
.meta-ads {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
  padding: 1.5rem;
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}

.meta-ads__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  flex-wrap: wrap;
}

.meta-ads__title {
  font-size: 1.6rem;
  font-weight: 700;
  letter-spacing: -0.01em;
}

.meta-ads__subtitle {
  font-size: 0.9rem;
  color: var(--text-muted);
  margin-top: 0.25rem;
  max-width: 56ch;
}

.meta-ads__actions {
  display: flex;
  align-items: flex-end;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.meta-ads__range {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
}

.meta-ads__range-label {
  font-size: 0.68rem;
  font-weight: 700;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.meta-ads__range-select {
  padding: 0.55rem 0.85rem;
  border-radius: 0.55rem;
  border: 1px solid var(--line-soft);
  background: rgb(var(--surface-2) / 0.5);
  color: var(--text-main);
  font: inherit;
  font-size: 0.88rem;
  cursor: pointer;
  appearance: none;
  background-image:
    linear-gradient(45deg, transparent 50%, var(--text-muted) 50%),
    linear-gradient(135deg, var(--text-muted) 50%, transparent 50%);
  background-position:
    calc(100% - 18px) 50%,
    calc(100% - 13px) 50%;
  background-size:
    5px 5px,
    5px 5px;
  background-repeat: no-repeat;
}

.meta-ads__range-select:focus {
  outline: none;
  border-color: rgb(var(--ring) / 0.5);
  box-shadow: 0 0 0 3px rgb(var(--ring) / 0.16);
}

.meta-ads__sync {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.9rem;
  font-weight: 600;
  padding: 0.6rem 1.3rem;
  border-radius: 0.55rem;
  cursor: pointer;
  border: 1px solid transparent;
  background: linear-gradient(135deg, rgb(var(--primary)), rgb(var(--primary-600)));
  color: rgb(255 255 255);
}

.meta-ads__sync:disabled {
  opacity: 0.6;
  cursor: progress;
}

.meta-ads__spinner {
  width: 14px;
  height: 14px;
  border-radius: 50%;
  border: 2px solid rgb(255 255 255 / 0.4);
  border-top-color: rgb(255 255 255);
  animation: meta-ads-spin 0.7s linear infinite;
}

.meta-ads__error {
  color: rgb(var(--danger));
  background: rgb(var(--danger) / 0.16);
  padding: 0.6rem 0.85rem;
  border-radius: var(--radius-soft);
  font-size: 0.9rem;
}

.meta-ads__tabs {
  display: flex;
  gap: 0.25rem;
  border-bottom: 1px solid var(--line-soft);
  flex-wrap: wrap;
}

.meta-ads__tab {
  padding: 0.6rem 1.15rem;
  font: inherit;
  font-size: 0.9rem;
  font-weight: 600;
  color: var(--text-muted);
  background: transparent;
  border: none;
  border-bottom: 2px solid transparent;
  margin-bottom: -1px;
  cursor: pointer;
}

.meta-ads__tab:hover {
  color: var(--text-main);
}

.meta-ads__tab--active {
  color: var(--text-main);
  border-bottom-color: rgb(var(--primary));
}

.meta-ads__panel {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}

@keyframes meta-ads-spin {
  to {
    transform: rotate(360deg);
  }
}

@media (max-width: 720px) {
  .meta-ads__header {
    flex-direction: column;
  }
}
</style>
