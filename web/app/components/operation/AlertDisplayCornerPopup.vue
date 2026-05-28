<script setup lang="ts">
import { ref, computed } from 'vue'
import { useAlertsStore } from '~/stores/alerts'
import { normalizeAlertHexColor } from '~/utils/alert-colors'

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
    consultant: alert.consultantName || 'Consultor',
    elapsed: formatElapsed(alert.lastTriggeredAt),
    threshold: String(alert.thresholdMinutes || 0),
  }

  for (const [key, value] of Object.entries(vars)) {
    result = result.replace(`{${key}}`, value)
  }

  return result
}

function formatElapsed(dateString: string) {
  if (!dateString) return '0m'
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
    '--alert-color': normalizeAlertHexColor(colorTheme),
  }
}
</script>

<template>
  <div class="corner-popups">
    <div
      v-for="(alert, idx) in visibleAlerts"
      :key="alert.id"
      class="corner-popup"
      :style="getColorStyle(alert.colorTheme || 'amber')"
    >
      <div class="corner-popup__header">
        <h4 class="corner-popup__title">
          {{ renderTemplate(alert.headline || alert.titleTemplate || 'Alerta operacional', alert) }}
        </h4>
        <button
          v-if="alert.interactionKind === 'dismiss'"
          class="corner-popup__close"
          @click="dismiss(alert.id)"
        >
          ✕
        </button>
      </div>

      <p class="corner-popup__body">
        {{ renderTemplate(alert.body || alert.bodyTemplate || '', alert) }}
      </p>

      <div v-if="alert.responseOptions?.length" class="corner-popup__actions">
        <button
          v-for="opt in alert.responseOptions"
          :key="opt.value"
          class="corner-popup__action-btn"
          @click="respond(alert.id, opt.value)"
        >
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
  background: rgb(var(--surface));
  color: rgb(var(--text));
  border-radius: 8px;
  padding: 1rem;
  box-shadow: var(--shadow-sm);
  border-left: 4px solid var(--alert-color, rgb(var(--primary)));
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
  color: rgb(var(--text));
}

.corner-popup__close {
  background: none;
  border: none;
  color: rgb(var(--muted));
  cursor: pointer;
  font-size: 1rem;
  padding: 0;
  line-height: 1;
}

.corner-popup__close:hover {
  color: rgb(var(--text));
}

.corner-popup__body {
  margin: 0 0 0.75rem 0;
  font-size: 0.875rem;
  color: rgb(var(--muted));
}

.corner-popup__actions {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.corner-popup__action-btn {
  padding: 0.4rem 0.8rem;
  border-radius: 4px;
  border: 1px solid rgb(var(--border));
  background: rgb(var(--surface));
  color: rgb(var(--text));
  font-size: 0.8rem;
  cursor: pointer;
  transition: all 0.2s;
}

.corner-popup__action-btn:hover {
  background: rgb(var(--primary) / 0.12);
  border-color: rgb(var(--ring) / 0.36);
}

/* Color theme variants */
.corner-popup--amber {
  border-left-color: rgb(var(--primary));
}

.corner-popup--red {
  border-left-color: rgb(var(--danger));
}

.corner-popup--blue {
  border-left-color: rgb(var(--primary));
}

.corner-popup--green {
  border-left-color: rgb(var(--success));
}

.corner-popup--purple {
  border-left-color: rgb(var(--primary-600));
}

.corner-popup--slate {
  border-left-color: rgb(var(--muted));
}
</style>
