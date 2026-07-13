<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import OperationWorkspace from '~/components/operation/OperationWorkspace.vue'
import OperationScopeBar from '~/components/operation/OperationScopeBar.vue'
import OperationSkeleton from '~/components/operation/OperationSkeleton.vue'
import AlertDisplayHost from '~/components/operation/AlertDisplayHost.vue'
import ArchivedStoreBanner from '~/components/operation/ArchivedStoreBanner.vue'
import { storeToRefs } from 'pinia'
import { useAuthStore } from '~/stores/auth'
import { useOperationsStore } from '~/stores/operations'
import { useAlertsStore } from '~/stores/alerts'
import { useOperationsRealtime } from '~/composables/useOperationsRealtime'
import { canUseAllStoresScope, canViewAlerts } from '~/domain/utils/permissions'
import { getApiErrorMessage } from '~/utils/api-client'

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'operacao',
  supportsAllStoresScope: true,
})

const auth = useAuthStore()
const operationsStore = useOperationsStore()
const alertsStore = useAlertsStore()
const loadError = ref('')
// Filtro de loja compartilhado: o seletor agora vive no nav (DashboardWorkspaceNav),
// que escreve neste mesmo ref do store. A pagina so reage a ele.
const { integratedStoreId } = storeToRefs(operationsStore)
const storeOptions = computed(() => auth.storeContext || [])

const canSeeIntegrated = computed(() => canUseAllStoresScope(auth.accessibleStoreIds))
const scopeMode = computed(() => {
  if (!canSeeIntegrated.value) {
    return 'single'
  }

  return 'all'
})
const useTenantAlertScope = computed(() => scopeMode.value === 'all' && canSeeIntegrated.value)
const canLoadAlerts = computed(() =>
  canViewAlerts(auth.role, auth.permissionKeys, auth.permissionsResolved),
)

useOperationsRealtime({ scopeMode })

async function refreshOperationAlerts() {
  if (!auth.isAuthenticated || !canLoadAlerts.value) {
    return
  }

  try {
    await alertsStore.refreshAlerts({ tenantScope: useTenantAlertScope.value })
  } catch {
    // Alertas nao devem impedir a operacao de carregar.
  }
}

async function loadOperationView() {
  if (!auth.isAuthenticated) {
    return
  }

  try {
    loadError.value = ''

    if (scopeMode.value === 'all' && canSeeIntegrated.value) {
      await operationsStore.refreshOverview()
      void refreshOperationAlerts()
      return
    }

    operationsStore.clearOverview()
    await operationsStore.refreshActiveStore()
    void refreshOperationAlerts()
  } catch (error) {
    loadError.value = getApiErrorMessage(error, 'Nao foi possivel carregar a operacao.')
  }
}

onMounted(async () => {
  await auth.ensureSession()
  await loadOperationView()
  // Carrega a loja OPERAVEL (filtro de "Todas as lojas") ja no primeiro mount. O
  // watch(integratedStoreId) so dispara em MUDANCA, entao numa navegacao SPA com o
  // filtro ja setado (persistido no Pinia) o board ficava vazio ate trocar de loja
  // ou recarregar. loadOperableStoreSnapshot faz no-op quando nao ha filtro/escopo.
  await loadOperableStoreSnapshot()
})

const { state, overview, overviewPending, overviewError } = storeToRefs(operationsStore)

const isRemoteRosterReady = computed(() => {
  if (scopeMode.value === 'all' && canSeeIntegrated.value) {
    return !overviewPending.value || Boolean(overview.value)
  }

  if (!auth.isAuthenticated || loadError.value) {
    return false
  }

  const activeStoreId = String(auth.activeStoreId || state.value?.activeStoreId || '').trim()
  const roster = Array.isArray(state.value?.roster) ? state.value.roster : []

  if (!activeStoreId) {
    return false
  }

  if (roster.length === 0) {
    return true
  }

  return roster.every((consultant) => String(consultant?.storeId || '').trim() === activeStoreId)
})

const pageErrorMessage = computed(() => {
  if (loadError.value) {
    return loadError.value
  }

  if (scopeMode.value === 'all' && overviewError.value) {
    return overviewError.value
  }

  return ''
})

watch([scopeMode, () => auth.activeStoreId, () => auth.isAuthenticated], () => {
  void loadOperationView()
})

watch(
  storeOptions,
  (stores) => {
    const normalizedFilter = String(integratedStoreId.value || '').trim()
    if (!normalizedFilter) {
      return
    }

    const exists = (stores || []).some(
      (store) => String(store?.id || '').trim() === normalizedFilter,
    )
    if (!exists) {
      integratedStoreId.value = ''
    }
  },
  { immediate: true },
)

watch(scopeMode, (nextMode) => {
  if (nextMode !== 'all') {
    integratedStoreId.value = ''
  }
})

// Ao filtrar UMA loja no modo "Todas as lojas", carrega o snapshot REAL dela
// para transformar aquela loja em contexto operavel (iniciar/encerrar/pausar).
// Sem filtro, segue a visao agregada (somente leitura).
async function loadOperableStoreSnapshot() {
  const storeId = String(integratedStoreId.value || '').trim()

  if (scopeMode.value !== 'all' || !canSeeIntegrated.value || !storeId) {
    return
  }

  try {
    await operationsStore.refreshOperationSnapshot(storeId)
  } catch {
    // Falha ao carregar o snapshot escopado mantem a visao agregada de leitura.
  }
}

watch(integratedStoreId, () => {
  void loadOperableStoreSnapshot()
})

const bannerStoreId = computed(() => {
  if (scopeMode.value === 'single') {
    return String(auth.activeStoreId || '').trim()
  }

  return String(integratedStoreId.value || '').trim()
})

watch(
  () => alertsStore.pendingFinishForServiceId,
  (serviceId) => {
    if (serviceId) {
      operationsStore.openFinishModal(serviceId)
      alertsStore.pendingFinishForServiceId = null
    }
  },
)
</script>

<template>
  <div class="workspace-host">
    <div v-if="pageErrorMessage" class="loading-state">
      <strong class="loading-state__title">Nao foi possivel carregar a operacao</strong>
      <p class="workspace__text">{{ pageErrorMessage }}</p>
    </div>
    <OperationSkeleton v-else-if="!isRemoteRosterReady" :scope-mode="scopeMode" />
    <template v-else>
      <ArchivedStoreBanner :store-id="bannerStoreId || ''" />
      <AlertDisplayHost
        v-if="bannerStoreId && canLoadAlerts"
        :store-id="bannerStoreId"
        class="operation-page-alerts"
      />
      <OperationScopeBar :state="state" class="operation-page-campaign" />
      <OperationWorkspace
        :state="state"
        :overview="overview"
        :scope-mode="scopeMode"
        :can-see-integrated="canSeeIntegrated"
        :stores="storeOptions"
        :integrated-store-id="integratedStoreId"
      />
    </template>
  </div>
</template>

<style scoped>
/* O filtro de loja foi para o nav (DashboardWorkspaceNav); aqui restam o banner
   de alerta e o destaque de campanha. A margem so existe quando o elemento
   realmente renderiza (banner = stack com v-if; campanha = section com v-if),
   entao nao sobra espaco vertical morto quando nao ha nada a mostrar. */
.operation-page-alerts :deep(.operation-alert-banner-stack) {
  margin-bottom: 0.5rem;
}

.operation-page-campaign {
  margin-bottom: 0.5rem;
}
</style>
