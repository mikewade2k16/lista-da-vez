<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue"

import AppDetailDialog from "~/components/ui/AppDetailDialog.vue"
import AppEntityGrid from "~/components/ui/AppEntityGrid.vue"
import AppPanelButton from "~/components/ui/AppPanelButton.vue"
import AppSelectField from "~/components/ui/AppSelectField.vue"
import AppToggleSwitch from "~/components/ui/AppToggleSwitch.vue"
import AlertRuleEditor from "~/components/alerts/AlertRuleEditor.vue"
import AlertRuleList from "~/components/alerts/AlertRuleList.vue"
import { hasPermission } from "~/domain/utils/permissions"
import { useAuthStore } from "~/stores/auth"
import { useAlertsStore } from "~/stores/alerts"
import { useUiStore } from "~/stores/ui"

const alertsStore = useAlertsStore()
const auth = useAuthStore()
const ui = useUiStore()

const activeTab = ref<"rules" | "history">("rules")
const selectedAlert = ref<Record<string, unknown> | null>(null)
const detailOpen = ref(false)
const searchValue = ref("")
const actionNote = ref("")
const actionPending = ref(false)
const savingRules = ref(false)
const showRuleEditor = ref(false)
const editingRule = ref<Record<string, any> | null>(null)

const rulesDraft = reactive({
  notifyDashboard: true,
  notifyOperationContext: true,
  notifyExternal: false
})

const typeOptions = [
  { value: "", label: "Todos" },
  { value: "long_open_service", label: "Atendimento longo" }
]

const statusOptions = [
  { value: "active", label: "Ativos" },
  { value: "acknowledged", label: "Reconhecidos" },
  { value: "resolved", label: "Resolvidos" },
  { value: "", label: "Todos" }
]

const columns = [
  { id: "status", label: "Status", width: "130px", align: "center" },
  { id: "severity", label: "Severidade", width: "130px", align: "center" },
  { id: "headline", label: "Resumo", width: "360px", align: "left" },
  { id: "storeId", label: "Loja", width: "180px", align: "left" },
  { id: "lastTriggeredAt", label: "Ultimo trigger", width: "180px", align: "left" },
  { id: "actions", label: "Acoes", width: "160px", align: "center" }
]

const canManageRules = computed(() => {
  if (auth.permissionsResolved) {
    return hasPermission(auth.permissionKeys, "alerts.rules.manage") || hasPermission(auth.permissionKeys, "workspace.alertas.edit")
  }

  return auth.role === "owner" || auth.role === "platform_admin"
})

const canManageActions = computed(() => {
  if (auth.permissionsResolved) {
    return hasPermission(auth.permissionKeys, "alerts.actions.manage") || hasPermission(auth.permissionKeys, "workspace.alertas.edit")
  }

  return ["store_terminal", "manager", "owner", "platform_admin"].includes(String(auth.role || ""))
})

const storeNameById = computed(() => new Map((auth.storeContext || []).map((store) => [String(store?.id || "").trim(), String(store?.name || "").trim()])))

const overviewCards = computed(() => {
  const current = alertsStore.overview || {
    totalActive: 0,
    criticalActive: 0,
    acknowledged: 0,
    resolvedToday: 0
  }

  return [
    { id: "total", label: "Alertas ativos", value: current.totalActive, tone: "default" },
    { id: "critical", label: "Criticos", value: current.criticalActive, tone: "critical" },
    { id: "ack", label: "Reconhecidos", value: current.acknowledged, tone: "warning" },
    { id: "resolved", label: "Resolvidos hoje", value: current.resolvedToday, tone: "success" }
  ]
})

const filteredAlerts = computed(() => {
  const search = searchValue.value.trim().toLowerCase()
  if (!search) {
    return alertsStore.items
  }

  return alertsStore.items.filter((alert) => {
    const storeLabel = resolveStoreLabel(alert.storeId).toLowerCase()
    return [alert.headline, alert.body, alert.serviceId, storeLabel]
      .map((value) => String(value || "").toLowerCase())
      .some((value) => value.includes(search))
  })
})

const rulesDirty = computed(() => {
  const rules = alertsStore.rules
  if (!rules) {
    return false
  }

  return (
    Boolean(rulesDraft.notifyDashboard) !== Boolean(rules.notifyDashboard) ||
    Boolean(rulesDraft.notifyOperationContext) !== Boolean(rules.notifyOperationContext) ||
    Boolean(rulesDraft.notifyExternal) !== Boolean(rules.notifyExternal)
  )
})

const scopeDescription = computed(() => {
  if (alertsStore.integratedScope) {
    return "Escopo: tenant inteiro com invalidação por contexto operacional em tempo quase imediato."
  }

  return `Loja ativa: ${resolveStoreLabel(alertsStore.activeStoreId)}.`
})

function syncRulesDraft() {
  const rules = alertsStore.rules
  if (!rules) {
    return
  }

  rulesDraft.notifyDashboard = Boolean(rules.notifyDashboard)
  rulesDraft.notifyOperationContext = Boolean(rules.notifyOperationContext)
  rulesDraft.notifyExternal = Boolean(rules.notifyExternal)
}

function resolveStoreLabel(storeId: string) {
  return storeNameById.value.get(String(storeId || "").trim()) || String(storeId || "").trim() || "Escopo atual"
}

function formatDate(value: string) {
  const normalized = String(value || "").trim()
  if (!normalized) {
    return "-"
  }

  try {
    return new Date(normalized).toLocaleString("pt-BR", {
      day: "2-digit",
      month: "2-digit",
      year: "numeric",
      hour: "2-digit",
      minute: "2-digit"
    })
  } catch {
    return normalized
  }
}

function statusLabel(status: string) {
  return {
    active: "Ativo",
    acknowledged: "Reconhecido",
    resolved: "Resolvido",
    closed_by_admin: "Fechado"
  }[String(status || "").trim()] || String(status || "").trim()
}

function severityLabel(severity: string) {
  return {
    critical: "Critica",
    attention: "Atencao",
    info: "Informativa"
  }[String(severity || "").trim()] || String(severity || "").trim()
}

function openDetail(alert: Record<string, unknown>) {
  selectedAlert.value = alert
  actionNote.value = ""
  detailOpen.value = true
}

function openNewRuleEditor() {
  editingRule.value = null
  showRuleEditor.value = true
}

function openEditRuleEditor(rule: Record<string, any>) {
  editingRule.value = rule
  showRuleEditor.value = true
}

async function handleSaveRule(rule: Record<string, unknown>) {
  try {
    if (editingRule.value?.id) {
      await alertsStore.updateRule(editingRule.value.id, rule)
      ui.success("Regra atualizada com sucesso.")
    } else {
      await alertsStore.createRule(rule)
      ui.success("Regra criada com sucesso.")
    }
    showRuleEditor.value = false
    editingRule.value = null
  } catch (error) {
    ui.error(alertsStore.errorMessage || "Erro ao salvar regra.")
  }
}

async function handleDeleteRule(ruleId: string) {
  if (!confirm("Tem certeza que deseja excluir esta regra?")) {
    return
  }

  try {
    await alertsStore.deleteRule(ruleId)
    ui.success("Regra excluída com sucesso.")
  } catch (error) {
    ui.error(alertsStore.errorMessage || "Erro ao excluir regra.")
  }
}

async function handleToggleRule(ruleId: string, isActive: boolean) {
  try {
    await alertsStore.updateRule(ruleId, { isActive })
    ui.success(isActive ? "Regra ativada." : "Regra desativada.")
  } catch (error) {
    ui.error(alertsStore.errorMessage || "Erro ao atualizar regra.")
  }
}

async function handleApplyRuleNow(ruleId: string) {
  try {
    const result = await alertsStore.applyRuleNow(ruleId)
    ui.success(`Regra aplicada. ${result.appliedCount} alerta(s) gerado(s).`)
  } catch (error) {
    ui.error(alertsStore.errorMessage || "Erro ao aplicar regra.")
  }
}

async function applyFilters() {
  try {
    await alertsStore.applyFilters({
      status: alertsStore.filters.status,
      type: alertsStore.filters.type
    })
  } catch {
    ui.error(alertsStore.errorMessage || "Erro ao atualizar a lista de alertas")
  }
}

async function handleAcknowledge() {
  if (!selectedAlert.value?.id) {
    return
  }

  actionPending.value = true
  try {
    const updated = await alertsStore.acknowledgeAlert(String(selectedAlert.value.id), actionNote.value)
    selectedAlert.value = updated
    ui.success("Alerta reconhecido.")
  } catch (error) {
    ui.error(alertsStore.errorMessage || String(error || "Erro ao reconhecer alerta."))
  } finally {
    actionPending.value = false
  }
}

async function handleResolve() {
  if (!selectedAlert.value?.id) {
    return
  }

  actionPending.value = true
  try {
    const updated = await alertsStore.resolveAlert(String(selectedAlert.value.id), actionNote.value)
    selectedAlert.value = updated
    ui.success("Alerta resolvido.")
  } catch (error) {
    ui.error(alertsStore.errorMessage || String(error || "Erro ao resolver alerta."))
  } finally {
    actionPending.value = false
  }
}

async function handleSaveGlobalRules() {
  savingRules.value = true
  try {
    await alertsStore.updateRules({ ...rulesDraft })
    syncRulesDraft()
    ui.success("Configurações globais atualizadas.")
  } catch (error) {
    ui.error(alertsStore.errorMessage || String(error || "Erro ao atualizar configurações."))
  } finally {
    savingRules.value = false
  }
}

watch(
  () => alertsStore.rules,
  () => {
    syncRulesDraft()
  },
  { immediate: true }
)

onMounted(async () => {
  const loaded = await alertsStore.ensureLoaded()
  if (!loaded && alertsStore.errorMessage) {
    ui.error(alertsStore.errorMessage)
  }

  try {
    await alertsStore.fetchRuleDefinitions()
  } catch {
    // Erro ao carregar regras dinâmicas, mas continua funcionando
  }
})
</script>

<template>
  <section class="admin-panel alerts-panel" data-testid="alerts-panel">
    <header class="admin-panel__header">
      <h2 class="admin-panel__title">Alertas operacionais</h2>
      <p class="admin-panel__text">Monitoramento autoritativo dos incidentes materializados pela Operacao e atualizados por realtime.</p>
      <p class="admin-panel__text">{{ scopeDescription }}</p>
    </header>

    <article v-if="alertsStore.errorMessage && !alertsStore.pending && !alertsStore.ready" class="settings-card">
      <p class="settings-card__text">{{ alertsStore.errorMessage }}</p>
    </article>

    <!-- Tab Navigation -->
    <div class="alerts-panel__tabs">
      <button
        class="alerts-panel__tab"
        :class="{ 'alerts-panel__tab--active': activeTab === 'rules' }"
        @click="activeTab = 'rules'"
      >
        Regras
      </button>
      <button
        class="alerts-panel__tab"
        :class="{ 'alerts-panel__tab--active': activeTab === 'history' }"
        @click="activeTab = 'history'"
      >
        Histórico
      </button>
    </div>

    <!-- Rules Tab -->
    <template v-if="activeTab === 'rules'">
      <section class="alerts-panel__summary-grid" data-testid="alerts-summary">
        <article v-for="card in overviewCards" :key="card.id" class="metric-card alerts-panel__metric-card" :class="`alerts-panel__metric-card--${card.tone}`">
          <span class="metric-card__label">{{ card.label }}</span>
          <strong class="metric-card__value">{{ card.value }}</strong>
        </article>
      </section>

      <!-- Rules List -->
      <article class="settings-card alerts-panel__rules-section">
        <header class="settings-card__header alerts-panel__section-header">
          <div>
            <h3 class="settings-card__title">Regras dinâmicas</h3>
            <p class="settings-card__text">Crie e configure regras de alertas personalizadas com diferentes tipos de gatilho, display e interações.</p>
          </div>
          <AppPanelButton v-if="canManageRules" variant="primary" @click="openNewRuleEditor">
            + Nova regra
          </AppPanelButton>
        </header>

        <AlertRuleList
          :rules="alertsStore.ruleDefinitions || []"
          :pending="alertsStore.rulesPending"
          @edit="openEditRuleEditor"
          @delete="handleDeleteRule"
          @toggle="handleToggleRule"
          @apply-now="handleApplyRuleNow"
        />
      </article>

      <!-- Global Settings -->
      <article v-if="alertsStore.rules" class="settings-card alerts-panel__settings-section">
        <header class="settings-card__header">
          <div>
            <h3 class="settings-card__title">Configurações globais</h3>
            <p class="settings-card__text">Canais de notificação padrão para todos os alertas.</p>
          </div>
        </header>

        <div class="alerts-panel__toggle-grid">
          <AppToggleSwitch v-model="rulesDraft.notifyDashboard" :disabled="!canManageRules || savingRules" label="Publicar na workspace de alertas" />
          <AppToggleSwitch v-model="rulesDraft.notifyOperationContext" :disabled="!canManageRules || savingRules" label="Invalidar contexto operacional" />
          <AppToggleSwitch v-model="rulesDraft.notifyExternal" :disabled="!canManageRules || savingRules" label="Preparar entrega externa" />
        </div>

        <div v-if="canManageRules" class="alerts-panel__actions">
          <AppPanelButton class="alerts-panel__secondary-btn" variant="secondary" :disabled="savingRules || !rulesDirty" @click="syncRulesDraft">
            Descartar
          </AppPanelButton>
          <AppPanelButton class="alerts-panel__primary-btn" :disabled="savingRules || !rulesDirty" @click="handleSaveGlobalRules">
            {{ savingRules ? "Salvando..." : "Salvar configurações" }}
          </AppPanelButton>
        </div>
      </article>
    </template>

    <!-- History Tab -->
    <template v-if="activeTab === 'history'">
      <AppEntityGrid
        :columns="columns"
        :rows="filteredAlerts"
        :row-key="(alert) => alert.id"
        :search-value="searchValue"
        :loading="alertsStore.pending"
        empty-title="Nenhum alerta materializado"
        empty-text="Quando a Operacao detectar uma condicao e o modulo materializar o incidente, ele aparece aqui."
        search-placeholder="Buscar por resumo, atendimento ou loja..."
        storage-key="alerts-grid-columns"
        @update:search-value="searchValue = $event"
      >
        <template #toolbar-filters>
          <div class="alerts-panel__toolbar">
            <label class="settings-field alerts-panel__filter-field">
              <span>Status</span>
              <AppSelectField v-model="alertsStore.filters.status" :options="statusOptions" compact @change="applyFilters" />
            </label>
            <label class="settings-field alerts-panel__filter-field">
              <span>Tipo</span>
              <AppSelectField v-model="alertsStore.filters.type" :options="typeOptions" compact @change="applyFilters" />
            </label>
          </div>
        </template>

        <template #cell-status="{ row }">
          <span class="alerts-panel__status-badge" :class="`alerts-panel__status-badge--${row.status}`">{{ statusLabel(row.status) }}</span>
        </template>

        <template #cell-severity="{ row }">
          <span class="alerts-panel__severity-badge" :class="`alerts-panel__severity-badge--${row.severity}`">{{ severityLabel(row.severity) }}</span>
        </template>

        <template #cell-storeId="{ row }">
          <span>{{ resolveStoreLabel(row.storeId) }}</span>
        </template>

        <template #cell-lastTriggeredAt="{ row }">
          <span class="alerts-panel__muted">{{ formatDate(row.lastTriggeredAt) }}</span>
        </template>

        <template #cell-actions="{ row }">
          <AppPanelButton class="alerts-panel__table-btn" variant="secondary" @click="openDetail(row)">Detalhes</AppPanelButton>
        </template>
      </AppEntityGrid>
    </template>

    <!-- Alert Detail Dialog -->
    <AppDetailDialog
      v-model="detailOpen"
      title="Detalhes do alerta"
      :sections="[
        {
          id: 'summary',
          title: 'Contexto',
          description: 'Dados principais do incidente materializado',
          fields: [
            { label: 'Resumo', value: selectedAlert?.headline },
            { label: 'Status', value: statusLabel(String(selectedAlert?.status || '')) },
            { label: 'Severidade', value: severityLabel(String(selectedAlert?.severity || '')) },
            { label: 'Loja', value: resolveStoreLabel(String(selectedAlert?.storeId || '')) },
            { label: 'Atendimento', value: selectedAlert?.serviceId || '-' },
            { label: 'Ultimo trigger', value: formatDate(String(selectedAlert?.lastTriggeredAt || '')) }
          ]
        }
      ]"
    >
      <div class="alerts-panel__detail-body">
        <article class="settings-card">
          <header class="settings-card__header">
            <h3 class="settings-card__title">Descricao</h3>
          </header>
          <p class="settings-card__text">{{ selectedAlert?.body || 'Sem descricao adicional.' }}</p>
        </article>

        <article class="settings-card">
          <header class="settings-card__header">
            <h3 class="settings-card__title">Payload tecnico</h3>
          </header>
          <pre class="alerts-panel__payload">{{ JSON.stringify(selectedAlert?.metadata || {}, null, 2) }}</pre>
        </article>

        <article v-if="canManageActions" class="settings-card">
          <header class="settings-card__header">
            <h3 class="settings-card__title">Acoes do alerta</h3>
            <p class="settings-card__text">Registre uma observacao opcional antes de reconhecer ou resolver manualmente.</p>
          </header>
          <label class="settings-field">
            <span>Observacao</span>
            <textarea id="alerts-action-note" v-model="actionNote" class="alerts-panel__note" rows="4" placeholder="Justificativa opcional para acknowledge ou resolucao"></textarea>
          </label>
          <div class="alerts-panel__actions">
            <AppPanelButton class="alerts-panel__secondary-btn" variant="secondary" :disabled="actionPending" @click="handleAcknowledge">
              {{ actionPending ? 'Processando...' : 'Reconhecer' }}
            </AppPanelButton>
            <AppPanelButton class="alerts-panel__primary-btn" :disabled="actionPending" @click="handleResolve">
              {{ actionPending ? 'Processando...' : 'Resolver' }}
            </AppPanelButton>
          </div>
        </article>
      </div>
    </AppDetailDialog>

    <!-- Rule Editor Modal -->
    <AlertRuleEditor
      v-model="showRuleEditor"
      :rule="editingRule"
      :is-editing="Boolean(editingRule?.id)"
      @save="handleSaveRule"
    />
  </section>
</template>

<style scoped>
.alerts-panel {
  min-height: 0;
  overflow-y: auto;
}

.alerts-panel__tabs {
  display: flex;
  gap: 0.5rem;
  margin: 1.5rem 0 1rem 0;
  border-bottom: 1px solid var(--line-soft);
}

.alerts-panel__tab {
  padding: 0.75rem 1.2rem;
  background: none;
  border: none;
  border-bottom: 2px solid transparent;
  color: var(--text-muted);
  font-size: 0.95rem;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
  position: relative;
  bottom: -1px;
}

.alerts-panel__tab:hover {
  color: var(--text-main);
}

.alerts-panel__tab--active {
  color: var(--text-main);
  border-bottom-color: var(--accent);
}

.alerts-panel__summary-grid {
  display: grid;
  gap: 0.85rem;
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

.alerts-panel__rules-section,
.alerts-panel__settings-section {
  gap: 0.85rem;
}

.alerts-panel__section-header,
.alerts-panel__actions,
.alerts-panel__toolbar {
  display: flex;
  gap: 0.9rem;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
}

.alerts-panel__source {
  display: inline-flex;
  align-items: center;
  min-height: 32px;
  padding: 0 0.8rem;
  border-radius: 999px;
  border: 1px solid rgba(148, 163, 184, 0.18);
  background: rgba(15, 23, 42, 0.24);
  color: var(--text-muted);
  font-size: 0.76rem;
  text-transform: uppercase;
  letter-spacing: 0.08em;
}

.alerts-panel__toggle-grid {
  display: grid;
  gap: 0.8rem;
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.alerts-panel__number-input,
.alerts-panel__note {
  width: 100%;
  border: 1px solid var(--line-soft);
  border-radius: 14px;
  padding: 0.75rem 0.9rem;
  background: rgba(15, 23, 42, 0.54);
  color: var(--text-main);
}

.alerts-panel__number-input:disabled,
.alerts-panel__note:disabled {
  opacity: 0.65;
  cursor: not-allowed;
}

.alerts-panel__metric-card {
  gap: 0.35rem;
  min-height: 88px;
  border: 1px solid var(--line-soft);
}

.alerts-panel__metric-card .metric-card__label {
  letter-spacing: 0.05em;
}

.alerts-panel__metric-card .metric-card__value {
  font-size: 1.55rem;
  line-height: 1.05;
}

.alerts-panel__metric-card--critical {
  border-color: rgba(248, 113, 113, 0.38);
}

.alerts-panel__metric-card--warning {
  border-color: rgba(251, 191, 36, 0.34);
}

.alerts-panel__metric-card--success {
  border-color: rgba(74, 222, 128, 0.34);
}

.alerts-panel__filter-field {
  min-width: 180px;
}

.alerts-panel__actions :deep(.app-panel-button) {
  min-width: 120px;
}

.alerts-panel__table-btn {
  min-width: 0;
}

.alerts-panel__table-btn :deep(.app-panel-button),
.alerts-panel__table-btn {
  font-size: 0.76rem;
}

.alerts-panel__status-badge,
.alerts-panel__severity-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 96px;
  padding: 0.4rem 0.7rem;
  border-radius: 999px;
  font-size: 0.76rem;
  font-weight: 700;
  text-transform: uppercase;
}

.alerts-panel__status-badge--active {
  background: #fee2e2;
  color: #b91c1c;
}

.alerts-panel__status-badge--acknowledged {
  background: #fef3c7;
  color: #92400e;
}

.alerts-panel__status-badge--resolved {
  background: #dcfce7;
  color: #166534;
}

.alerts-panel__severity-badge--critical {
  background: #fecaca;
  color: #991b1b;
}

.alerts-panel__severity-badge--attention {
  background: #fde68a;
  color: #92400e;
}

.alerts-panel__severity-badge--info {
  background: #dbeafe;
  color: #1d4ed8;
}

.alerts-panel__muted {
  color: var(--text-muted);
}

.alerts-panel__detail-body {
  display: grid;
  gap: 1rem;
}

.alerts-panel__payload {
  margin: 0;
  overflow: auto;
  border-radius: 14px;
  padding: 0.9rem 1rem;
  background: #0f172a;
  color: #e2e8f0;
  white-space: pre-wrap;
  word-break: break-word;
}

@media (max-width: 1100px) {
  .alerts-panel__summary-grid,
  .alerts-panel__toggle-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 780px) {
  .alerts-panel__summary-grid,
  .alerts-panel__toggle-grid {
    grid-template-columns: 1fr;
  }

  .alerts-panel__filter-field {
    min-width: 100%;
  }

  .alerts-panel__actions {
    width: 100%;
  }

  .alerts-panel__actions > button {
    flex: 1 1 0;
  }

  .alerts-panel__tabs {
    gap: 0;
  }

  .alerts-panel__tab {
    flex: 1 1 0;
    text-align: center;
  }
}
</style>