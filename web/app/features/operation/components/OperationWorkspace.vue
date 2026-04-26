<script setup>
import { computed } from "vue";
import OperationFinishModal from "~/features/operation/components/OperationFinishModal.vue";
import OperationConsultantStrip from "~/features/operation/components/OperationConsultantStrip.vue";
import OperationQueueColumns from "~/features/operation/components/OperationQueueColumns.vue";
import OperationScopeBar from "~/features/operation/components/OperationScopeBar.vue";
import { canMutateOperations } from "~/domain/utils/permissions";
import { useAuthStore } from "~/stores/auth";

const props = defineProps({
  state: {
    type: Object,
    required: true
  },
  overview: {
    type: Object,
    default: null
  },
  scopeMode: {
    type: String,
    default: "single"
  },
  canSeeIntegrated: {
    type: Boolean,
    default: false
  },
  stores: {
    type: Array,
    default: () => []
  },
  integratedStoreId: {
    type: String,
    default: ""
  }
});

const emit = defineEmits(["integrated-store-change"]);
const auth = useAuthStore();
const canOperate = computed(() => canMutateOperations(auth.role, auth.permissionKeys, auth.permissionsResolved));
const showIntegratedView = computed(() => props.canSeeIntegrated && props.scopeMode === "all");

function shouldIncludeStore(storeId) {
  const filterStoreId = String(props.integratedStoreId || "").trim();
  return !showIntegratedView.value || !filterStoreId || String(storeId || "").trim() === filterStoreId;
}

function mapIntegratedWaitingItem(person) {
  return {
    id: String(person?.personId || "").trim(),
    storeId: String(person?.storeId || "").trim(),
    storeName: String(person?.storeName || "").trim(),
    storeCode: String(person?.storeCode || "").trim(),
    name: String(person?.name || "").trim(),
    role: String(person?.role || "").trim(),
    initials: String(person?.initials || "").trim(),
    color: String(person?.color || "").trim(),
    monthlyGoal: Math.max(0, Number(person?.monthlyGoal || 0) || 0),
    commissionRate: Math.max(0, Number(person?.commissionRate || 0) || 0),
    queueJoinedAt: Number(person?.queueJoinedAt || 0) || 0
  };
}

function mapIntegratedActiveItem(person) {
  return {
    id: String(person?.personId || "").trim(),
    storeId: String(person?.storeId || "").trim(),
    storeName: String(person?.storeName || "").trim(),
    storeCode: String(person?.storeCode || "").trim(),
    name: String(person?.name || "").trim(),
    role: String(person?.role || "").trim(),
    initials: String(person?.initials || "").trim(),
    color: String(person?.color || "").trim(),
    monthlyGoal: Math.max(0, Number(person?.monthlyGoal || 0) || 0),
    commissionRate: Math.max(0, Number(person?.commissionRate || 0) || 0),
    serviceId: String(person?.serviceId || "").trim(),
    serviceStartedAt: Number(person?.serviceStartedAt || 0) || 0,
    queueJoinedAt: Number(person?.queueJoinedAt || 0) || 0,
    queueWaitMs: Number(person?.queueWaitMs || 0) || 0,
    queuePositionAtStart: Math.max(1, Number(person?.queuePosition || 1) || 1),
    startMode: String(person?.startMode || "queue").trim() || "queue",
    skippedPeople: Array.isArray(person?.skippedPeople) ? person.skippedPeople : []
  };
}

function mapIntegratedPausedItem(person) {
  return {
    personId: String(person?.personId || "").trim(),
    storeId: String(person?.storeId || "").trim(),
    storeName: String(person?.storeName || "").trim(),
    storeCode: String(person?.storeCode || "").trim(),
    reason: String(person?.pauseReason || "").trim(),
    kind: String(person?.pauseKind || "pause").trim() || "pause",
    startedAt: Number(person?.statusStartedAt || 0) || 0
  };
}

function upsertRosterPerson(rosterMap, person) {
  const id = String(person?.personId || person?.id || "").trim();
  if (!id) {
    return;
  }

  rosterMap.set(id, {
    id,
    storeId: String(person?.storeId || "").trim(),
    storeName: String(person?.storeName || "").trim(),
    storeCode: String(person?.storeCode || "").trim(),
    name: String(person?.name || "").trim(),
    role: String(person?.role || "").trim(),
    initials: String(person?.initials || "").trim(),
    color: String(person?.color || "").trim(),
    monthlyGoal: Math.max(0, Number(person?.monthlyGoal || 0) || 0),
    commissionRate: Math.max(0, Number(person?.commissionRate || 0) || 0)
  });
}

const displayState = computed(() => {
  if (!showIntegratedView.value || !props.overview) {
    return props.state;
  }

  const waitingSource = (Array.isArray(props.overview.waitingList) ? props.overview.waitingList : []).filter((person) =>
    shouldIncludeStore(person?.storeId)
  );
  const activeSource = (Array.isArray(props.overview.activeServices) ? props.overview.activeServices : []).filter((person) =>
    shouldIncludeStore(person?.storeId)
  );
  const pausedSource = (Array.isArray(props.overview.pausedEmployees) ? props.overview.pausedEmployees : []).filter((person) =>
    shouldIncludeStore(person?.storeId)
  );
  const availableSource = (Array.isArray(props.overview.availableConsultants) ? props.overview.availableConsultants : []).filter((person) =>
    shouldIncludeStore(person?.storeId)
  );

  const rosterMap = new Map();
  waitingSource.forEach((person) => upsertRosterPerson(rosterMap, person));
  activeSource.forEach((person) => upsertRosterPerson(rosterMap, person));
  pausedSource.forEach((person) => upsertRosterPerson(rosterMap, person));
  availableSource.forEach((person) => upsertRosterPerson(rosterMap, person));

  const roster = Array.from(rosterMap.values()).sort((left, right) => {
    const leftStore = `${left.storeName}-${left.name}`.toLowerCase();
    const rightStore = `${right.storeName}-${right.name}`.toLowerCase();
    return leftStore.localeCompare(rightStore, "pt-BR");
  });

  return {
    ...props.state,
    waitingList: waitingSource.map(mapIntegratedWaitingItem),
    activeServices: activeSource.map(mapIntegratedActiveItem),
    pausedEmployees: pausedSource.map(mapIntegratedPausedItem),
    roster
  };
});
</script>

<template>
  <OperationScopeBar
    :state="props.state"
    :scope-mode="scopeMode"
    :stores="stores"
    :integrated-store-id="integratedStoreId"
    @integrated-store-change="emit('integrated-store-change', $event)"
  />
  <div v-if="!canOperate" class="insight-card">
    <p class="settings-card__text">Este perfil acompanha a operacao em tempo real, mas nao executa fila, pausas nem encerramentos.</p>
  </div>
  <OperationQueueColumns :state="displayState" :read-only="!canOperate" :integrated-mode="showIntegratedView" />
  <OperationConsultantStrip v-if="canOperate" :state="displayState" :integrated-mode="showIntegratedView" />
  <OperationFinishModal :state="props.state" />
</template>
