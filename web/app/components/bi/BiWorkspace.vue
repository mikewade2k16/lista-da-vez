<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import {
  Activity,
  BrainCircuit,
  KeyRound,
  LogIn,
  RefreshCw,
  ShieldCheck,
  ShieldOff,
  Trash2,
} from 'lucide-vue-next'

import SettingsTabs from '~/components/settings/SettingsTabs.vue'
import BiDatasetTable from '~/components/bi/BiDatasetTable.vue'
import BiIntelligencePanel from '~/components/bi/BiIntelligencePanel.vue'
import { useBiStore } from '~/stores/bi'
import { useUiStore } from '~/stores/ui'

const biStore = useBiStore()
const ui = useUiStore()
const {
  overview,
  loading,
  inventoryLoading,
  loggingIn,
  error,
  loginError,
  metrics,
  sources,
  insights,
  sections,
  tables,
  manualConfig,
  manualToken,
  hasManualToken,
} = storeToRefs(biStore)

const activeTab = ref('visao')
const manualOpen = ref(false)
const showManualSecrets = ref(false)

const tabs = computed(() => [
  { id: 'visao', label: 'Visao', icon: 'dashboard' },
  ...tables.value.map((table) => ({
    id: table.key,
    label: table.label,
    icon:
      table.key === 'items'
        ? 'inventory_2'
        : table.key === 'inventario'
          ? 'inventory'
          : 'table_chart',
  })),
  { id: 'inteligencia', label: 'Inteligencia', icon: 'psychology' },
])

const activeTable = computed(
  () => tables.value.find((table) => table.key === activeTab.value) || null,
)
const activeSource = computed(
  () => sources.value.find((source) => source.key === activeTab.value) || null,
)
const activeTableLoading = computed(
  () => loading.value || (activeTable.value?.key === 'inventario' && inventoryLoading.value),
)
const itemsTable = computed(() => tables.value.find((table) => table.key === 'items') || null)

const generatedAtLabel = computed(() => {
  const raw = overview.value?.generatedAt
  if (!raw) return 'Ainda nao atualizado'

  return new Intl.DateTimeFormat('pt-BR', {
    dateStyle: 'short',
    timeStyle: 'short',
  }).format(new Date(raw))
})

const manualTokenPreview = computed(() => {
  const value = manualToken.value
  if (!value) return ''
  if (value.length <= 14) return value
  return `${value.slice(0, 8)}...${value.slice(-4)}`
})

async function refresh() {
  const response = await biStore.refreshOverview()
  if (!response.ok) {
    ui.error(response.message || 'Nao foi possivel carregar o BI.')
    return
  }

  const inventoryPending = Boolean(
    response.data?.sources?.find((source) => source.key === 'inventario')?.pending,
  )
  ui.success(
    inventoryPending
      ? 'Visao rapida atualizada. O inventario segue carregando em segundo plano.'
      : 'BI Perola atualizado.',
  )
}

async function generateManualToken() {
  const response = await biStore.loginPerola()
  if (!response.ok) {
    ui.error(response.message || 'Falha ao gerar token Perola.')
    return
  }

  ui.success(
    'Token manual da Perola gerado. Clique em "Atualizar BI" quando quiser carregar os dados.',
  )
}

function inputValue(event: Event) {
  return String((event.target as HTMLInputElement | null)?.value || '')
}

function clearManualSession() {
  biStore.clearManualSession()
  ui.success('Token manual limpo. O BI volta a usar o backend automaticamente.')
}

watch(
  tables,
  () => {
    if (activeTab.value === 'visao' || activeTab.value === 'inteligencia') {
      return
    }
    if (!activeTable.value) {
      activeTab.value = 'visao'
    }
  },
  { deep: true },
)
</script>

<template>
  <section class="admin-panel bi-panel" data-testid="bi-panel">
    <header class="bi-panel__header">
      <div>
        <h2 class="bi-panel__title">BI Perola</h2>
        <p class="bi-panel__text">
          Leitura tratada da API Perola BI com credenciais e token protegidos no backend.
        </p>
      </div>

      <button class="bi-panel__refresh" type="button" :disabled="loading" @click="refresh">
        <RefreshCw :size="16" aria-hidden="true" />
        {{ loading ? 'Atualizando...' : 'Atualizar BI' }}
      </button>
    </header>

    <section class="bi-manual" :data-open="manualOpen">
      <header class="bi-manual__head">
        <div class="bi-manual__title">
          <KeyRound :size="16" aria-hidden="true" />
          <h3>Conexao manual</h3>
          <span
            class="bi-manual__badge"
            :data-state="hasManualToken ? 'ok' : 'off'"
            :title="
              hasManualToken ? `Token: ${manualTokenPreview}` : 'Usando credenciais do backend'
            "
          >
            <component
              :is="hasManualToken ? ShieldCheck : ShieldOff"
              :size="13"
              aria-hidden="true"
            />
            {{ hasManualToken ? 'Token manual' : 'Automatico' }}
          </span>
        </div>

        <button class="bi-manual__toggle" type="button" @click="manualOpen = !manualOpen">
          {{ manualOpen ? 'Recolher' : 'Configurar manual' }}
        </button>
      </header>

      <div v-if="manualOpen" class="bi-manual__body">
        <p class="bi-manual__hint">
          Se colar um Bearer Token, nao precisa preencher Login e Pass. Se gerar token, preencha
          CompanyKey, Login e Pass.
        </p>

        <div class="bi-manual__grid">
          <label class="bi-manual__field">
            <span>dsCompanyKey</span>
            <input
              :type="showManualSecrets ? 'text' : 'password'"
              :value="manualConfig.companyKey"
              autocomplete="off"
              spellcheck="false"
              placeholder="Opcional: usa o backend se vazio"
              @input="biStore.updateManualConfig({ companyKey: inputValue($event) })"
            />
          </label>

          <label class="bi-manual__field">
            <span>dsCnpjEmpresa</span>
            <input
              :value="manualConfig.cnpjEmpresa"
              inputmode="numeric"
              autocomplete="off"
              spellcheck="false"
              placeholder="Opcional: usa o backend se vazio"
              @input="biStore.updateManualConfig({ cnpjEmpresa: inputValue($event) })"
            />
          </label>

          <label class="bi-manual__field">
            <span>Login</span>
            <input
              :value="manualConfig.login"
              autocomplete="username"
              spellcheck="false"
              placeholder="Opcional se houver token"
              @input="biStore.updateManualConfig({ login: inputValue($event) })"
            />
          </label>

          <label class="bi-manual__field">
            <span>Pass</span>
            <input
              :type="showManualSecrets ? 'text' : 'password'"
              :value="manualConfig.pass"
              autocomplete="current-password"
              placeholder="Opcional se houver token"
              @input="biStore.updateManualConfig({ pass: inputValue($event) })"
            />
          </label>

          <label class="bi-manual__field bi-manual__field--wide">
            <span>Bearer Token</span>
            <input
              :type="showManualSecrets ? 'text' : 'password'"
              :value="manualToken"
              autocomplete="off"
              spellcheck="false"
              placeholder="Cole um JWT ou gere pelo botao"
              @input="biStore.setManualToken(inputValue($event))"
            />
          </label>
        </div>

        <p v-if="loginError" class="bi-manual__error">{{ loginError }}</p>

        <div class="bi-manual__actions">
          <button
            class="bi-manual__button bi-manual__button--primary"
            type="button"
            :disabled="loggingIn"
            @click="generateManualToken"
          >
            <LogIn :size="14" aria-hidden="true" />
            {{ loggingIn ? 'Gerando...' : 'Gerar token + carregar' }}
          </button>

          <button
            class="bi-manual__button"
            type="button"
            :disabled="!hasManualToken || loading"
            @click="refresh"
          >
            <RefreshCw :size="14" aria-hidden="true" />
            Carregar com token
          </button>

          <button
            class="bi-manual__button bi-manual__button--ghost"
            type="button"
            :disabled="!hasManualToken"
            @click="clearManualSession"
          >
            <Trash2 :size="14" aria-hidden="true" />
            Limpar manual
          </button>

          <label class="bi-manual__secret">
            <input v-model="showManualSecrets" type="checkbox" />
            Mostrar segredos
          </label>
        </div>
      </div>
    </section>

    <p v-if="error" class="bi-panel__error">{{ error }}</p>

    <section class="bi-panel__metrics" aria-label="Resumo Perola BI">
      <article
        v-for="metric in metrics"
        :key="metric.key"
        class="bi-panel__metric"
        :data-tone="metric.tone"
      >
        <span>{{ metric.label }}</span>
        <strong>{{ metric.value }}</strong>
        <small>{{ metric.detail }}</small>
      </article>
    </section>

    <div class="bi-panel__meta">
      <span>CNPJ {{ overview?.cnpjEmpresa || '-' }}</span>
      <span>Atualizado em {{ generatedAtLabel }}</span>
    </div>

    <SettingsTabs :tabs="tabs" :active-tab="activeTab" @update:active-tab="activeTab = $event" />

    <section v-if="activeTab === 'visao'" class="bi-panel__overview">
      <div class="bi-panel__insights">
        <article
          v-for="insight in insights"
          :key="insight.title"
          class="bi-panel__insight"
          :data-tone="insight.tone"
        >
          <BrainCircuit :size="18" aria-hidden="true" />
          <div>
            <h3>{{ insight.title }}</h3>
            <p>{{ insight.body }}</p>
          </div>
        </article>
      </div>

      <aside class="bi-panel__sources">
        <h3>Fontes consultadas</h3>
        <article v-for="source in sources" :key="source.key" class="bi-panel__source">
          <Activity :size="16" aria-hidden="true" />
          <div>
            <strong>{{ source.label }}</strong>
            <span>
              {{ source.pending ? 'Em carga' : source.ok ? 'OK' : 'Falha' }} -
              {{ source.fetched.toLocaleString('pt-BR') }}
              <template v-if="source.total > source.fetched">
                de {{ source.total.toLocaleString('pt-BR') }}
              </template>
              registros -
              {{ source.durationMs }} ms
            </span>
            <small v-if="source.truncated && !source.error">
              Amostra rapida carregada para manter a tela agil.
            </small>
            <small v-if="source.error">{{ source.error }}</small>
          </div>
        </article>
      </aside>
    </section>

    <section v-else-if="activeTab === 'inteligencia'" class="bi-panel__intelligence">
      <BiIntelligencePanel
        :sections="sections"
        :items-table="itemsTable"
        :inventory-loading="inventoryLoading"
      />
    </section>

    <BiDatasetTable
      v-else-if="activeTable"
      :table="activeTable"
      :source="activeSource"
      :loading="activeTableLoading"
      @refresh="refresh"
    />
  </section>
</template>

<style scoped>
.bi-panel {
  display: grid;
  gap: 1rem;
  align-content: start;
}

.bi-panel__header,
.bi-panel__meta,
.bi-panel__overview {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
}

.bi-panel__header {
  align-items: flex-start;
}

.bi-panel__title {
  margin: 0;
  color: var(--text-main);
  font-size: 1.5rem;
  font-weight: 750;
}

.bi-panel__text {
  margin: 0.35rem 0 0;
  color: var(--text-muted);
  line-height: 1.5;
}

.bi-panel__refresh {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.45rem;
  min-height: 2.45rem;
  padding: 0 0.95rem;
  border: 1px solid rgba(83, 198, 160, 0.34);
  border-radius: 0.8rem;
  background: linear-gradient(135deg, rgba(13, 102, 87, 0.95), rgba(14, 73, 67, 0.94));
  color: var(--text-main);
  font-weight: 750;
  cursor: pointer;
}

.bi-panel__refresh:disabled {
  opacity: 0.65;
  cursor: wait;
}

.bi-panel__error {
  margin: 0;
  padding: 0.8rem;
  border: 1px solid rgba(255, 120, 100, 0.28);
  border-radius: 0.8rem;
  background: rgba(120, 28, 28, 0.18);
  color: #ffb4a8;
}

.bi-manual {
  display: grid;
  gap: 0.75rem;
  padding: 0.9rem 1rem;
  border: 1px solid var(--line-soft);
  border-radius: 0.95rem;
  background: rgba(9, 13, 21, 0.82);
}

.bi-manual[data-open='true'] {
  border-color: rgba(83, 198, 160, 0.32);
}

.bi-manual__head,
.bi-manual__title,
.bi-manual__actions {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  flex-wrap: wrap;
}

.bi-manual__head {
  justify-content: space-between;
}

.bi-manual__title h3 {
  margin: 0;
  color: var(--text-main);
  font-size: 0.95rem;
}

.bi-manual__badge {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  min-height: 1.8rem;
  padding: 0 0.6rem;
  border: 1px solid var(--line-soft);
  border-radius: 999px;
  color: var(--text-muted);
  font-size: 0.74rem;
  font-weight: 800;
}

.bi-manual__badge[data-state='ok'] {
  border-color: rgba(83, 198, 160, 0.45);
  color: #b9ffd2;
}

.bi-manual__toggle,
.bi-manual__button {
  min-height: 2.25rem;
  padding: 0 0.85rem;
  border: 1px solid var(--line-soft);
  border-radius: 0.7rem;
  background: rgba(17, 24, 39, 0.92);
  color: var(--text-main);
  font-weight: 750;
  cursor: pointer;
}

.bi-manual__body {
  display: grid;
  gap: 0.85rem;
}

.bi-manual__hint {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.82rem;
  line-height: 1.5;
}

.bi-manual__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(15rem, 1fr));
  gap: 0.65rem 0.75rem;
}

.bi-manual__field {
  display: grid;
  gap: 0.3rem;
  min-width: 0;
}

.bi-manual__field--wide {
  grid-column: 1 / -1;
}

.bi-manual__field span {
  color: var(--text-muted);
  font-size: 0.72rem;
  font-weight: 800;
  letter-spacing: 0.05em;
  text-transform: uppercase;
}

.bi-manual__field input {
  width: 100%;
  min-height: 2.3rem;
  padding: 0 0.7rem;
  border: 1px solid var(--line-soft);
  border-radius: 0.7rem;
  background: rgb(var(--surface) / 0.95);
  color: var(--text-main);
  font: inherit;
}

.bi-manual__field input:focus {
  outline: none;
  border-color: rgba(83, 198, 160, 0.55);
  box-shadow: 0 0 0 3px rgba(83, 198, 160, 0.18);
}

.bi-manual__button {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
}

.bi-manual__button:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.bi-manual__button--primary {
  border-color: rgba(83, 198, 160, 0.34);
  background: linear-gradient(135deg, rgba(13, 102, 87, 0.95), rgba(14, 73, 67, 0.94));
}

.bi-manual__button--ghost {
  background: transparent;
}

.bi-manual__secret {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  margin-left: auto;
  color: var(--text-muted);
  font-size: 0.78rem;
}

.bi-manual__error {
  margin: 0;
  padding: 0.6rem 0.8rem;
  border: 1px solid rgba(255, 120, 100, 0.28);
  border-radius: 0.7rem;
  background: rgba(120, 28, 28, 0.18);
  color: #ffb4a8;
}

.bi-panel__metrics {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(10.5rem, 1fr));
  gap: 0.75rem;
}

.bi-panel__intelligence {
  display: block;
}

.bi-panel__metric,
.bi-panel__insight,
.bi-panel__source,
.bi-panel__opportunity {
  border: 1px solid var(--line-soft);
  border-radius: 0.9rem;
  background: rgb(var(--surface) / 0.9);
  box-shadow: var(--shadow-card);
}

.bi-panel__metric {
  display: grid;
  gap: 0.25rem;
  padding: 0.9rem;
}

.bi-panel__metric span,
.bi-panel__metric small,
.bi-panel__meta,
.bi-panel__source span,
.bi-panel__source small,
.bi-panel__opportunity p,
.bi-panel__insight p {
  color: var(--text-muted);
}

.bi-panel__metric span {
  font-size: 0.72rem;
  text-transform: uppercase;
  letter-spacing: 0.07em;
}

.bi-panel__metric strong {
  color: var(--text-main);
  font-size: 1.35rem;
  font-variant-numeric: tabular-nums;
}

.bi-panel__metric[data-tone='success'] strong {
  color: #b9ffd2;
}

.bi-panel__metric[data-tone='warning'] strong {
  color: #ffd38a;
}

.bi-panel__meta {
  flex-wrap: wrap;
  font-size: 0.8rem;
}

.bi-panel__overview {
  align-items: stretch;
}

.bi-panel__insights {
  flex: 1 1 0;
  display: grid;
  gap: 0.75rem;
}

.bi-panel__insight,
.bi-panel__source {
  display: flex;
  gap: 0.75rem;
  padding: 0.9rem;
}

.bi-panel__insight h3,
.bi-panel__sources h3,
.bi-panel__opportunity h3 {
  margin: 0;
  color: var(--text-main);
  font-size: 0.98rem;
}

.bi-panel__insight p,
.bi-panel__opportunity p {
  margin: 0.3rem 0 0;
  line-height: 1.55;
}

.bi-panel__sources {
  flex: 0 1 24rem;
  display: grid;
  gap: 0.65rem;
  align-content: start;
  padding: 0.95rem;
  border: 1px solid var(--line-soft);
  border-radius: 0.9rem;
  background: rgba(9, 13, 21, 0.72);
}

.bi-panel__source {
  box-shadow: none;
}

.bi-panel__source strong {
  display: block;
  color: var(--text-main);
}

.bi-panel__source small {
  display: block;
  margin-top: 0.15rem;
}

.bi-panel__opportunity {
  padding: 1rem;
}

@media (max-width: 980px) {
  .bi-panel__header,
  .bi-panel__overview {
    flex-direction: column;
  }

  .bi-panel__refresh {
    width: 100%;
  }

  .bi-manual__button,
  .bi-manual__toggle {
    flex: 1 1 12rem;
  }
}
</style>
