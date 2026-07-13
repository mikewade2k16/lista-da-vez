<script setup>
import { computed } from 'vue'
import OperationConsultantStrip from '~/components/operation/OperationConsultantStrip.vue'
import OperationQueueColumns from '~/components/operation/OperationQueueColumns.vue'
import { canMutateOperations } from '~/domain/utils/permissions'
import { useAuthStore } from '~/stores/auth'
import { defineLazyComponent } from '~/utils/lazy-component'

const OperationFinishModal = defineLazyComponent(
  () => import('~/components/operation/OperationFinishModal.vue'),
)

const props = defineProps({
  state: {
    type: Object,
    required: true,
  },
  overview: {
    type: Object,
    default: null,
  },
  scopeMode: {
    type: String,
    default: 'single',
  },
  canSeeIntegrated: {
    type: Boolean,
    default: false,
  },
  stores: {
    type: Array,
    default: () => [],
  },
  integratedStoreId: {
    type: String,
    default: '',
  },
})

const auth = useAuthStore()
const canOperate = computed(() =>
  canMutateOperations(auth.role, auth.permissionKeys, auth.permissionsResolved),
)
const showIntegratedView = computed(() => props.canSeeIntegrated && props.scopeMode === 'all')

// No modo "Todas as lojas", quando o usuario filtra UMA loja com snapshot real
// carregado, aquela loja vira contexto operavel (iniciar/encerrar/pausar) como
// um operador comum. Sem filtro (agregado), segue somente leitura.
const operableStoreId = computed(() => {
  if (!showIntegratedView.value) {
    return ''
  }

  const storeId = String(props.integratedStoreId || '').trim()
  if (!storeId) {
    return ''
  }

  return hasTrustedScopedSnapshot(storeId) ? storeId : ''
})
const isOperatingSingleStore = computed(() => Boolean(operableStoreId.value))
const childIntegratedMode = computed(
  () => showIntegratedView.value && !isOperatingSingleStore.value,
)

function shouldIncludeStore(storeId) {
  const filterStoreId = String(props.integratedStoreId || '').trim()
  return (
    !showIntegratedView.value || !filterStoreId || String(storeId || '').trim() === filterStoreId
  )
}

function mapIntegratedWaitingItem(person) {
  return {
    id: String(person?.personId || '').trim(),
    storeId: String(person?.storeId || '').trim(),
    storeName: String(person?.storeName || '').trim(),
    storeCode: String(person?.storeCode || '').trim(),
    name: String(person?.name || '').trim(),
    role: String(person?.role || '').trim(),
    initials: String(person?.initials || '').trim(),
    color: String(person?.color || '').trim(),
    monthlyGoal: Math.max(0, Number(person?.monthlyGoal || 0) || 0),
    commissionRate: Math.max(0, Number(person?.commissionRate || 0) || 0),
    goalStats: person?.goalStats ?? null,
    queueJoinedAt: Number(person?.queueJoinedAt || 0) || 0,
  }
}

function mapIntegratedActiveItem(person) {
  return {
    id: String(person?.personId || '').trim(),
    storeId: String(person?.storeId || '').trim(),
    storeName: String(person?.storeName || '').trim(),
    storeCode: String(person?.storeCode || '').trim(),
    name: String(person?.name || '').trim(),
    role: String(person?.role || '').trim(),
    initials: String(person?.initials || '').trim(),
    color: String(person?.color || '').trim(),
    monthlyGoal: Math.max(0, Number(person?.monthlyGoal || 0) || 0),
    commissionRate: Math.max(0, Number(person?.commissionRate || 0) || 0),
    goalStats: person?.goalStats ?? null,
    serviceId: String(person?.serviceId || '').trim(),
    serviceStartedAt: Number(person?.serviceStartedAt || 0) || 0,
    queueJoinedAt: Number(person?.queueJoinedAt || 0) || 0,
    queueWaitMs: Number(person?.queueWaitMs || 0) || 0,
    queuePositionAtStart: Math.max(
      1,
      Number(person?.queuePositionAtStart || person?.queuePosition || 1) || 1,
    ),
    startMode: String(person?.startMode || 'queue').trim() || 'queue',
    skippedPeople: Array.isArray(person?.skippedPeople) ? person.skippedPeople : [],
    parallelGroupId: String(person?.parallelGroupId || '').trim(),
    parallelStartIndex:
      typeof person?.parallelStartIndex === 'number' ? person.parallelStartIndex : null,
    siblingServiceIds: Array.isArray(person?.siblingServiceIds) ? person.siblingServiceIds : [],
    startOffsetMs: Number(person?.startOffsetMs || 0) || 0,
    stoppedAt: Math.max(0, Number(person?.stoppedAt || 0) || 0),
    effectiveFinishedAt: Math.max(0, Number(person?.effectiveFinishedAt || 0) || 0),
    stopReason: String(person?.stopReason || '').trim(),
  }
}

function resolveStoreMeta(storeId) {
  const normalizedStoreId = String(storeId || '').trim()
  const overviewStore = (Array.isArray(props.overview?.stores) ? props.overview.stores : []).find(
    (store) => String(store?.storeId || '').trim() === normalizedStoreId,
  )
  const scopedStore = (Array.isArray(props.stores) ? props.stores : []).find(
    (store) => String(store?.id || '').trim() === normalizedStoreId,
  )

  return {
    storeId: normalizedStoreId,
    storeName: String(overviewStore?.storeName || scopedStore?.name || '').trim(),
    storeCode: String(overviewStore?.storeCode || scopedStore?.code || '').trim(),
  }
}

function getScopedSnapshot(storeId) {
  const normalizedStoreId = String(storeId || '').trim()
  if (!normalizedStoreId) {
    return null
  }

  if (normalizedStoreId === String(props.state?.activeStoreId || '').trim()) {
    return props.state
  }

  return props.state?.storeSnapshots?.[normalizedStoreId] || null
}

function hasTrustedScopedSnapshot(storeId) {
  const normalizedStoreId = String(storeId || '').trim()
  const snapshot = getScopedSnapshot(normalizedStoreId)

  if (!snapshot) {
    return false
  }

  // Sinal PRIMARIO: o snapshot foi buscado do servidor.
  if (Number(snapshot?._operationSnapshotFetchedAt || 0) > 0) {
    return true
  }

  // Sinal SECUNDARIO (resiliencia): o snapshot ja carrega dados reais da loja
  // (fila/atendimentos/historico). Sob rajada de refreshes concorrentes o flag
  // _operationSnapshotFetchedAt pode oscilar por um tick e derrubava o board para a
  // visao agregada (sem historico => "0 finalizados", parecia bug de tela vazia). A
  // presenca de dados ja hidratados e um sinal ESTAVEL de que a loja e operavel,
  // mantendo o board sob a rajada.
  return (
    (Array.isArray(snapshot?.activeServices) && snapshot.activeServices.length > 0) ||
    (Array.isArray(snapshot?.waitingList) && snapshot.waitingList.length > 0) ||
    (Array.isArray(snapshot?.serviceHistory) && snapshot.serviceHistory.length > 0)
  )
}

function mapScopedActiveItem(service, storeMeta) {
  return {
    id: String(service?.id || service?.personId || '').trim(),
    storeId: storeMeta.storeId,
    storeName: storeMeta.storeName,
    storeCode: storeMeta.storeCode,
    name: String(service?.name || '').trim(),
    role: String(service?.role || '').trim(),
    initials: String(service?.initials || '').trim(),
    color: String(service?.color || '').trim(),
    monthlyGoal: Math.max(0, Number(service?.monthlyGoal || 0) || 0),
    commissionRate: Math.max(0, Number(service?.commissionRate || 0) || 0),
    goalStats: service?.goalStats ?? null,
    serviceId: String(service?.serviceId || '').trim(),
    serviceStartedAt: Number(service?.serviceStartedAt || 0) || 0,
    queueJoinedAt: Number(service?.queueJoinedAt || 0) || 0,
    queueWaitMs: Number(service?.queueWaitMs || 0) || 0,
    queuePositionAtStart: Math.max(1, Number(service?.queuePositionAtStart || 1) || 1),
    startMode: String(service?.startMode || 'queue').trim() || 'queue',
    skippedPeople: Array.isArray(service?.skippedPeople) ? service.skippedPeople : [],
    parallelGroupId: String(service?.parallelGroupId || '').trim(),
    parallelStartIndex:
      typeof service?.parallelStartIndex === 'number' ? service.parallelStartIndex : null,
    siblingServiceIds: Array.isArray(service?.siblingServiceIds) ? service.siblingServiceIds : [],
    startOffsetMs: Number(service?.startOffsetMs || 0) || 0,
    stoppedAt: Math.max(0, Number(service?.stoppedAt || 0) || 0),
    effectiveFinishedAt: Math.max(0, Number(service?.effectiveFinishedAt || 0) || 0),
    stopReason: String(service?.stopReason || '').trim(),
  }
}

function buildActiveItems(activeSource) {
  const trustedStoreIds = new Set()

  activeSource.forEach((person) => {
    const storeId = String(person?.storeId || '').trim()
    if (shouldIncludeStore(storeId) && hasTrustedScopedSnapshot(storeId)) {
      trustedStoreIds.add(storeId)
    }
  })

  Object.entries(props.state?.storeSnapshots || {}).forEach(([storeId, snapshot]) => {
    if (
      shouldIncludeStore(storeId) &&
      Number(snapshot?._operationSnapshotFetchedAt || 0) > 0 &&
      Array.isArray(snapshot?.activeServices) &&
      snapshot.activeServices.length > 0
    ) {
      trustedStoreIds.add(storeId)
    }
  })

  const activeStoreId = String(props.state?.activeStoreId || '').trim()
  if (
    showIntegratedView.value &&
    shouldIncludeStore(activeStoreId) &&
    hasTrustedScopedSnapshot(activeStoreId) &&
    Array.isArray(props.state?.activeServices) &&
    props.state.activeServices.length > 0
  ) {
    trustedStoreIds.add(activeStoreId)
  }

  const overviewItems = activeSource
    .filter((person) => !trustedStoreIds.has(String(person?.storeId || '').trim()))
    .map(mapIntegratedActiveItem)

  const scopedItems = [...trustedStoreIds].flatMap((storeId) => {
    const snapshot = getScopedSnapshot(storeId)
    const storeMeta = resolveStoreMeta(storeId)

    return (Array.isArray(snapshot?.activeServices) ? snapshot.activeServices : [])
      .map((service) => mapScopedActiveItem(service, storeMeta))
      .filter((service) => service.id && service.serviceId)
  })

  return [...overviewItems, ...scopedItems]
}

function mapIntegratedPausedItem(person) {
  return {
    personId: String(person?.personId || '').trim(),
    storeId: String(person?.storeId || '').trim(),
    storeName: String(person?.storeName || '').trim(),
    storeCode: String(person?.storeCode || '').trim(),
    reason: String(person?.pauseReason || '').trim(),
    kind: String(person?.pauseKind || 'pause').trim() || 'pause',
    startedAt: Number(person?.statusStartedAt || 0) || 0,
  }
}

function upsertRosterPerson(rosterMap, person) {
  const id = String(person?.personId || person?.id || '').trim()
  if (!id) {
    return
  }

  rosterMap.set(id, {
    id,
    storeId: String(person?.storeId || '').trim(),
    storeName: String(person?.storeName || '').trim(),
    storeCode: String(person?.storeCode || '').trim(),
    name: String(person?.name || '').trim(),
    role: String(person?.role || '').trim(),
    initials: String(person?.initials || '').trim(),
    color: String(person?.color || '').trim(),
    monthlyGoal: Math.max(0, Number(person?.monthlyGoal || 0) || 0),
    commissionRate: Math.max(0, Number(person?.commissionRate || 0) || 0),
  })
}

function buildOperableStoreState(storeId) {
  const snapshot = getScopedSnapshot(storeId) || {}
  const storeMeta = resolveStoreMeta(storeId)
  const decorate = (items) =>
    (Array.isArray(items) ? items : []).map((item) => ({
      ...item,
      storeId,
      storeName: storeMeta.storeName,
      storeCode: storeMeta.storeCode,
    }))

  // Mantem a config tenant-wide (settings/modalConfig/opcoes/estado do modal) do
  // estado de topo e troca apenas as listas operacionais pela loja operada,
  // fixando activeStoreId nela para o modal/rascunho mirarem a loja certa.
  return {
    ...props.state,
    activeStoreId: storeId,
    roster: decorate(snapshot.roster),
    waitingList: decorate(snapshot.waitingList),
    activeServices: decorate(snapshot.activeServices),
    pausedEmployees: (Array.isArray(snapshot.pausedEmployees) ? snapshot.pausedEmployees : []).map(
      (item) => ({ ...item, storeId }),
    ),
    serviceHistory: Array.isArray(snapshot.serviceHistory) ? snapshot.serviceHistory : [],
    pendingValidations: Array.isArray(snapshot.pendingValidations)
      ? snapshot.pendingValidations.map((item) => ({ ...item, storeId }))
      : [],
    consultantActivitySessions: Array.isArray(snapshot.consultantActivitySessions)
      ? snapshot.consultantActivitySessions
      : [],
    consultantCurrentStatus:
      snapshot.consultantCurrentStatus && typeof snapshot.consultantCurrentStatus === 'object'
        ? snapshot.consultantCurrentStatus
        : {},
  }
}

const displayState = computed(() => {
  if (isOperatingSingleStore.value) {
    return buildOperableStoreState(operableStoreId.value)
  }

  if (!showIntegratedView.value || !props.overview) {
    return props.state
  }

  const waitingSource = (
    Array.isArray(props.overview.waitingList) ? props.overview.waitingList : []
  ).filter((person) => shouldIncludeStore(person?.storeId))
  const activeSource = (
    Array.isArray(props.overview.activeServices) ? props.overview.activeServices : []
  ).filter((person) => shouldIncludeStore(person?.storeId))
  const pausedSource = (
    Array.isArray(props.overview.pausedEmployees) ? props.overview.pausedEmployees : []
  ).filter((person) => shouldIncludeStore(person?.storeId))
  const availableSource = (
    Array.isArray(props.overview.availableConsultants) ? props.overview.availableConsultants : []
  ).filter((person) => shouldIncludeStore(person?.storeId))

  const rosterMap = new Map()
  waitingSource.forEach((person) => upsertRosterPerson(rosterMap, person))
  const activeItems = buildActiveItems(activeSource)
  activeItems.forEach((person) => upsertRosterPerson(rosterMap, person))
  pausedSource.forEach((person) => upsertRosterPerson(rosterMap, person))
  availableSource.forEach((person) => upsertRosterPerson(rosterMap, person))

  const roster = Array.from(rosterMap.values()).sort((left, right) => {
    const leftStore = `${left.storeName}-${left.name}`.toLowerCase()
    const rightStore = `${right.storeName}-${right.name}`.toLowerCase()
    return leftStore.localeCompare(rightStore, 'pt-BR')
  })

  return {
    ...props.state,
    waitingList: waitingSource.map(mapIntegratedWaitingItem),
    activeServices: activeItems,
    pausedEmployees: pausedSource.map(mapIntegratedPausedItem),
    roster,
    // Auto-encerramento (2h): pendencias AGREGADAS de todas as lojas acessiveis, para
    // a caixa de Pendencias funcionar na visao "Todas as lojas" (nao so por loja).
    pendingValidations: Array.isArray(props.overview?.pendingValidations)
      ? props.overview.pendingValidations
      : [],
  }
})

const isFinishModalOpen = computed(() =>
  Boolean(String(displayState.value?.finishModalServiceId || '').trim()),
)
</script>

<template>
  <section class="operation-workspace">
    <div v-if="!canOperate" class="insight-card">
      <p class="settings-card__text">
        Este perfil acompanha a operacao em tempo real, mas nao executa fila, pausas nem
        encerramentos.
      </p>
    </div>
    <div class="operation-workspace__main">
      <OperationQueueColumns
        :state="displayState"
        :read-only="!canOperate"
        :integrated-mode="childIntegratedMode"
        :operating-store-id="operableStoreId"
      />
      <OperationConsultantStrip
        v-if="canOperate"
        :state="displayState"
        :integrated-mode="childIntegratedMode"
        :operating-store-id="operableStoreId"
      />
    </div>
    <Suspense v-if="isFinishModalOpen">
      <OperationFinishModal :state="displayState" />
      <template #fallback>
        <CoreLoadingOverlay />
      </template>
    </Suspense>
  </section>
</template>
