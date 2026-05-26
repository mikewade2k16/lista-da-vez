<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useAlertsStore } from '~/stores/alerts'
import { alertBannerStyle } from '~/utils/alert-colors'

const props = defineProps<{
  alerts: Array<Record<string, any>>
}>()

const alertsStore = useAlertsStore()
const respondingAlertId = ref('')
const nowMs = ref(Date.now())
let timerId: ReturnType<typeof window.setInterval> | null = null

function getAlertStyle(alert: Record<string, any>) {
  return alertBannerStyle(alert.colorTheme || 'amber')
}

function alertStartedAt(alert: Record<string, any>) {
  const metadataStartedAt = Number(alert?.metadata?.serviceStartedAt || 0) || 0
  if (metadataStartedAt > 0) {
    return metadataStartedAt
  }

  const openedAt = Date.parse(String(alert?.openedAt || alert?.lastTriggeredAt || ''))
  return Number.isFinite(openedAt) ? openedAt : 0
}

function formatElapsedMs(elapsedMs: number) {
  const minutes = Math.max(0, Math.floor(elapsedMs / 60000))

  if (minutes < 60) return `${minutes}m`
  const hours = Math.floor(minutes / 60)
  const rem = minutes % 60
  return rem === 0 ? `${hours}h` : `${hours}h${rem}m`
}

function getDynamicElapsed(alert: Record<string, any>) {
  const startedAt = alertStartedAt(alert)
  if (!startedAt) {
    return ''
  }

  return formatElapsedMs(nowMs.value - startedAt)
}

function getAlertTitle(alert: Record<string, any>) {
  if (alert.type === 'long_open_service') {
    const elapsed = getDynamicElapsed(alert)
    if (elapsed) {
      return `Atendimento em aberto ha ${elapsed}`
    }
  }

  if (alert.headline) return alert.headline
  if (alert.consultantName) return `${alert.consultantName} — atendimento longo`
  return 'Atendimento longo'
}

function getAlertBody(alert: Record<string, any>) {
  return alert.body || ''
}

async function respond(alertId: string, optionValue: string) {
  if (respondingAlertId.value) {
    return
  }

  respondingAlertId.value = alertId

  try {
    await alertsStore.respondToAlert(alertId, optionValue as any)
  } finally {
    respondingAlertId.value = ''
  }
}

onMounted(() => {
  timerId = window.setInterval(() => {
    nowMs.value = Date.now()
  }, 30000)
})

onBeforeUnmount(() => {
  if (timerId) {
    window.clearInterval(timerId)
  }
})
</script>

<template>
  <div v-if="alerts.length > 0" class="operation-alert-banner-stack">
    <div
      v-for="alert in alerts"
      :key="alert.id"
      class="operation-alert-banner"
      :style="getAlertStyle(alert)"
      role="alert"
      aria-live="assertive"
    >
      <span class="material-icons-round operation-alert-banner__icon" aria-hidden="true">
        timer
      </span>

      <div class="operation-alert-banner__body">
        <strong class="operation-alert-banner__title">{{ getAlertTitle(alert) }}</strong>
        <span v-if="getAlertBody(alert)" class="operation-alert-banner__elapsed">
          {{ getAlertBody(alert) }}
        </span>
      </div>

      <div v-if="alert.responseOptions?.length" class="operation-alert-banner__actions">
        <button
          v-for="opt in alert.responseOptions"
          :key="opt.value"
          class="operation-alert-banner__btn operation-alert-banner__btn--secondary"
          :disabled="respondingAlertId === alert.id"
          @click="respond(alert.id, opt.value)"
        >
          <span
            v-if="respondingAlertId === alert.id"
            class="material-icons-round operation-alert-banner__spinner"
            aria-hidden="true"
          >
            refresh
          </span>
          <span v-else>{{ opt.label }}</span>
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.operation-alert-banner-stack {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.operation-alert-banner {
  display: flex;
  align-items: center;
  gap: 0.85rem;
  padding: 0.7rem 1rem;
  background: linear-gradient(
    90deg,
    var(--alert-color-darker, rgb(var(--primary-600))) 0%,
    var(--alert-color-dark, rgb(var(--primary))) 100%
  );
  border-left: 3px solid var(--alert-color, rgb(var(--primary)));
  color: rgb(255 255 255);
  border-radius: 0 6px 6px 0;
  box-shadow: 0 2px 12px rgb(var(--primary) / 0.28);
}

.operation-alert-banner--red {
  background: linear-gradient(90deg, rgb(var(--danger) / 0.28) 0%, rgb(var(--danger) / 0.18) 100%);
  border-left-color: rgb(var(--danger));
}

.operation-alert-banner--blue {
  background: linear-gradient(90deg, rgb(var(--primary-600)) 0%, rgb(var(--primary)) 100%);
  border-left-color: rgb(var(--primary));
}

.operation-alert-banner--green {
  background: linear-gradient(
    90deg,
    rgb(var(--success) / 0.28) 0%,
    rgb(var(--success) / 0.18) 100%
  );
  border-left-color: rgb(var(--success));
}

.operation-alert-banner--purple {
  background: linear-gradient(
    90deg,
    rgb(var(--primary) / 0.28) 0%,
    rgb(var(--primary) / 0.18) 100%
  );
  border-left-color: rgb(var(--primary));
}

.operation-alert-banner--slate {
  background: linear-gradient(90deg, rgb(var(--surface-2)) 0%, rgb(var(--surface)) 100%);
  border-left-color: rgb(var(--border));
}

.operation-alert-banner__icon {
  font-size: 1.2rem;
  color: var(--alert-color-light, rgb(var(--primary)));
  flex-shrink: 0;
}

.operation-alert-banner--red .operation-alert-banner__icon {
  color: rgb(var(--danger));
}

.operation-alert-banner--blue .operation-alert-banner__icon {
  color: rgb(var(--primary));
}

.operation-alert-banner--green .operation-alert-banner__icon {
  color: rgb(var(--success));
}

.operation-alert-banner--purple .operation-alert-banner__icon {
  color: rgb(var(--primary));
}

.operation-alert-banner--slate .operation-alert-banner__icon {
  color: rgb(var(--muted));
}

.operation-alert-banner__body {
  display: flex;
  align-items: baseline;
  gap: 0.5rem;
  flex: 1;
  min-width: 0;
}

.operation-alert-banner__title {
  font-size: 0.88rem;
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.operation-alert-banner__elapsed {
  font-size: 0.8rem;
  color: rgb(255 255 255 / 0.75);
  white-space: nowrap;
}

.operation-alert-banner__actions {
  display: flex;
  gap: 0.5rem;
  flex-shrink: 0;
}

.operation-alert-banner__btn {
  display: flex;
  align-items: center;
  gap: 0.25rem;
  padding: 0.3rem 0.75rem;
  border-radius: 4px;
  font-size: 0.82rem;
  font-weight: 500;
  cursor: pointer;
  border: 1px solid transparent;
  transition: opacity 0.15s;
  background: rgb(255 255 255 / 0.12);
  border-color: rgb(255 255 255 / 0.25);
  color: rgb(255 255 255);
}

.operation-alert-banner__btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.operation-alert-banner__btn:not(:disabled):hover {
  background: rgb(255 255 255 / 0.2);
}

.operation-alert-banner__spinner {
  font-size: 0.95rem;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

@media (max-width: 640px) {
  .operation-alert-banner {
    flex-direction: column;
    align-items: flex-start;
  }

  .operation-alert-banner__actions {
    width: 100%;
  }

  .operation-alert-banner__btn {
    flex: 1;
    justify-content: center;
  }
}
</style>
