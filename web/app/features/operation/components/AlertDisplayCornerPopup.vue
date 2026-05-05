<script setup lang="ts">
import { ref, computed } from "vue"
import { useAlertsStore } from "~/stores/alerts"
import { normalizeAlertHexColor } from "~/utils/alert-colors"

const props = defineProps<{
  alerts: Array<Record<string, any>>
}>()

const dismissedIds = ref<Set<string>>(new Set())
const alertsStore = useAlertsStore()

const visibleAlerts = computed(() => {
  return props.alerts.filter((alert) => !dismissedIds.value.has(alert.id))
})

function dismiss(alertId: string) {
  dismissedIds.value.add(alertId)
}

async function respond(alertId: string, optionValue: string) {
  await alertsStore.respondToAlert(alertId, optionValue as any)
}

function renderTemplate(template: string, alert: Record<string, any>) {
  let result = template
  const vars: Record<string, string> = {
    consultant: alert.consultantName || "Consultor",
    elapsed: formatElapsed(alert.lastTriggeredAt),
    threshold: String(alert.thresholdMinutes || 0)
  }

  for (const [key, value] of Object.entries(vars)) {
    result = result.replace(`{${key}}`, value)
  }

  return result
}

function formatElapsed(dateString: string) {
  if (!dateString) return "0m"
  const start = new Date(dateString)
  const now = new Date()
  const minutes = Math.floor((now.getTime() - start.getTime()) / 60000)

  if (minutes < 60) return `${minutes}m`
  const hours = Math.floor(minutes / 60)
  const rem = minutes % 60
  return rem === 0 ? `${hours}h` : `${hours}h${rem}m`
}

function getColorStyle(colorTheme: string) {
  return {
    "--alert-color": normalizeAlertHexColor(colorTheme)
  }
}
</script>

<template>
  <div class="corner-popups">
    <div v-for="(alert, idx) in visibleAlerts" :key="alert.id" class="corner-popup" :style="getColorStyle(alert.colorTheme || 'amber')">
      <div class="corner-popup__header">
        <h4 class="corner-popup__title">{{ renderTemplate(alert.headline || alert.titleTemplate || "Alerta operacional", alert) }}</h4>
        <button v-if="alert.interactionKind === 'dismiss'" class="corner-popup__close" @click="dismiss(alert.id)">✕</button>
      </div>

      <p class="corner-popup__body">{{ renderTemplate(alert.body || alert.bodyTemplate || "", alert) }}</p>

      <div v-if="alert.responseOptions?.length" class="corner-popup__actions">
        <button v-for="opt in alert.responseOptions" :key="opt.value" class="corner-popup__action-btn" @click="respond(alert.id, opt.value)">
          {{ opt.label }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.corner-popups {
  position: fixed;
  bottom: 1rem;
  right: 1rem;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  z-index: 500;
  max-width: 360px;
  pointer-events: none;
}

.corner-popup {
  background: white;
  border-radius: 8px;
  padding: 1rem;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  border-left: 4px solid var(--alert-color, #f59e0b);
  pointer-events: auto;
  animation: slideIn 0.3s ease-out;
}

@keyframes slideIn {
  from {
    transform: translateX(400px);
    opacity: 0;
  }
  to {
    transform: translateX(0);
    opacity: 1;
  }
}

.corner-popup__header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 0.5rem;
  margin-bottom: 0.5rem;
}

.corner-popup__title {
  margin: 0;
  font-size: 0.95rem;
  font-weight: 600;
  color: #1f2937;
}

.corner-popup__close {
  background: none;
  border: none;
  color: #9ca3af;
  cursor: pointer;
  font-size: 1rem;
  padding: 0;
  line-height: 1;
}

.corner-popup__close:hover {
  color: #6b7280;
}

.corner-popup__body {
  margin: 0 0 0.75rem 0;
  font-size: 0.875rem;
  color: #6b7280;
}

.corner-popup__actions {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.corner-popup__action-btn {
  padding: 0.4rem 0.8rem;
  border-radius: 4px;
  border: 1px solid #d1d5db;
  background: white;
  color: #374151;
  font-size: 0.8rem;
  cursor: pointer;
  transition: all 0.2s;
}

.corner-popup__action-btn:hover {
  background: #f3f4f6;
  border-color: #9ca3af;
}

/* Color theme variants */
.corner-popup--amber {
  border-left-color: #f59e0b;
}

.corner-popup--red {
  border-left-color: #ef4444;
}

.corner-popup--blue {
  border-left-color: #3b82f6;
}

.corner-popup--green {
  border-left-color: #10b981;
}

.corner-popup--purple {
  border-left-color: #a855f7;
}

.corner-popup--slate {
  border-left-color: #64748b;
}
</style>
