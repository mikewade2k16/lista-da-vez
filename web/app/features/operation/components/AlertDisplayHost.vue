<script setup lang="ts">
import { computed } from "vue"
import OperationAlertBanner from "~/features/operation/components/OperationAlertBanner.vue"
import AlertDisplayCornerPopup from "~/features/operation/components/AlertDisplayCornerPopup.vue"
import AlertDisplayCenterModal from "~/features/operation/components/AlertDisplayCenterModal.vue"
import AlertDisplayFullscreen from "~/features/operation/components/AlertDisplayFullscreen.vue"
import { useAlertsStore } from "~/stores/alerts"
import { useOperationsStore } from "~/stores/operations"

const props = defineProps<{
  storeId: string
}>()

const alertsStore = useAlertsStore()
const operationsStore = useOperationsStore()

const activeServiceSnapshot = computed(() => {
  const normalizedStoreId = String(props.storeId || "").trim()
  const state = operationsStore.state || {}

  if (normalizedStoreId && normalizedStoreId === String(state.activeStoreId || "").trim()) {
    return {
      trusted: true,
      services: Array.isArray(state.activeServices) ? state.activeServices : []
    }
  }

  const scopedSnapshot = state.storeSnapshots?.[normalizedStoreId]
  if (scopedSnapshot && Number(scopedSnapshot?._operationSnapshotFetchedAt || 0) > 0) {
    return {
      trusted: true,
      services: Array.isArray(scopedSnapshot.activeServices) ? scopedSnapshot.activeServices : []
    }
  }

  const overview = operationsStore.overview || null
  const overviewActiveServices = Array.isArray(overview?.activeServices)
    ? overview.activeServices.filter((service: Record<string, any>) => String(service?.storeId || "").trim() === normalizedStoreId)
    : []
  if (overview) {
    return {
      trusted: true,
      services: overviewActiveServices
    }
  }

  return {
    trusted: false,
    services: []
  }
})

const activeServiceIds = computed(() => new Set(
  activeServiceSnapshot.value.services
    .map((service: Record<string, any>) => String(service?.serviceId || "").trim())
    .filter(Boolean)
))

const activeAlerts = computed(() => {
  const alerts = alertsStore.activeAlertsForStore(props.storeId)

  if (!activeServiceSnapshot.value.trusted) {
    return alerts
  }

  return alerts.filter((alert) => {
    const serviceId = String(alert?.serviceId || "").trim()
    return !serviceId || activeServiceIds.value.has(serviceId)
  })
})

const alertsByKind = computed(() => {
  const grouped: Record<string, Array<Record<string, any>>> = {}

  for (const alert of activeAlerts.value) {
    const kind = alert.displayKind || "banner"
    if (!grouped[kind]) {
      grouped[kind] = []
    }
    grouped[kind].push(alert)
  }

  return grouped
})

const hasBanners = computed(() => Boolean(alertsByKind.value.banner?.length))
const hasCornerPopups = computed(() => Boolean(alertsByKind.value.corner_popup?.length))
const hasCenterModals = computed(() => Boolean(alertsByKind.value.center_modal?.length))
const hasFullscreens = computed(() => Boolean(alertsByKind.value.fullscreen?.length))
</script>

<template>
  <div class="alert-display-host">
    <!-- Banner (persists at top) -->
    <OperationAlertBanner v-if="hasBanners" :alerts="alertsByKind.banner || []" />

    <!-- Corner Popups (stackable, non-blocking) -->
    <AlertDisplayCornerPopup v-if="hasCornerPopups" :alerts="alertsByKind.corner_popup || []" />

    <!-- Center Modal (blocking) -->
    <AlertDisplayCenterModal v-if="hasCenterModals" :alerts="alertsByKind.center_modal || []" />

    <!-- Fullscreen (most aggressive, fully blocking) -->
    <AlertDisplayFullscreen v-if="hasFullscreens" :alerts="alertsByKind.fullscreen || []" />

    <!-- Toast and card_badge are handled by toast system and card respectively -->
  </div>
</template>

<style scoped>
.alert-display-host {
  position: relative;
  width: 100%;
}
</style>
