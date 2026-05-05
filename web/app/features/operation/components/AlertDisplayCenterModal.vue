<script setup lang="ts">
import { ref, computed } from "vue"
import { useAlertsStore } from "~/stores/alerts"
import { alertGradient } from "~/utils/alert-colors"

const props = defineProps<{
  alerts: Array<Record<string, any>>
}>()

const alertsStore = useAlertsStore()
const primaryAlert = computed(() => props.alerts?.[0] || null)
const respondingToId = ref<string | null>(null)

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

function getColorGradient(colorTheme: string) {
  return alertGradient(colorTheme)
}

async function handleResponse(optionValue: string) {
  if (!primaryAlert.value?.id) return

  respondingToId.value = primaryAlert.value.id
  try {
    await alertsStore.respondToAlert(primaryAlert.value.id, optionValue as any)
  } finally {
    respondingToId.value = null
  }
}

async function handleDismiss() {
  if (!primaryAlert.value?.id) return

  respondingToId.value = primaryAlert.value.id
  try {
    // For dismiss, we just acknowledge the alert
    await alertsStore.acknowledgeAlert(primaryAlert.value.id, "Dismissed via modal")
  } finally {
    respondingToId.value = null
  }
}
</script>

<template>
  <div v-if="primaryAlert" class="center-modal-overlay">
    <div class="center-modal" :style="{ '--gradient': getColorGradient(primaryAlert.colorTheme || 'amber') }">
      <div class="center-modal__header">
        <h2 class="center-modal__title">{{ renderTemplate(primaryAlert.headline || primaryAlert.titleTemplate || "Alerta operacional", primaryAlert) }}</h2>
        <button v-if="!primaryAlert.isMandatory && primaryAlert.interactionKind === 'dismiss'" class="center-modal__close" @click="handleDismiss">✕</button>
      </div>

      <p class="center-modal__body">{{ renderTemplate(primaryAlert.body || primaryAlert.bodyTemplate || "", primaryAlert) }}</p>

      <div class="center-modal__footer">
        <template v-if="primaryAlert.responseOptions?.length">
          <button
            v-for="opt in primaryAlert.responseOptions"
            :key="opt.value"
            class="center-modal__action-btn"
            :disabled="respondingToId === primaryAlert.id"
            @click="handleResponse(opt.value)"
          >
            {{ respondingToId === primaryAlert.id ? "Processando..." : opt.label }}
          </button>
        </template>
        <template v-else-if="primaryAlert.interactionKind === 'dismiss'">
          <button class="center-modal__action-btn center-modal__action-btn--primary" :disabled="respondingToId === primaryAlert.id" @click="handleDismiss">
            {{ respondingToId === primaryAlert.id ? "Processando..." : "Entendi" }}
          </button>
        </template>
      </div>
    </div>
  </div>
</template>

<style scoped>
.center-modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.6);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 600;
  padding: 1rem;
}

.center-modal {
  background: white;
  border-radius: 12px;
  width: 100%;
  max-width: 480px;
  padding: 2rem;
  box-shadow: 0 20px 25px rgba(0, 0, 0, 0.25);
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
  position: relative;
  overflow: hidden;
}

.center-modal::before {
  content: "";
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 4px;
  background: var(--gradient);
}

.center-modal__header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 1rem;
}

.center-modal__title {
  margin: 0;
  font-size: 1.5rem;
  font-weight: 700;
  color: #1f2937;
}

.center-modal__close {
  background: none;
  border: none;
  color: #9ca3af;
  cursor: pointer;
  font-size: 1.5rem;
  padding: 0;
  line-height: 1;
}

.center-modal__close:hover {
  color: #6b7280;
}

.center-modal__body {
  margin: 0;
  font-size: 1rem;
  color: #6b7280;
  line-height: 1.6;
}

.center-modal__footer {
  display: flex;
  gap: 0.75rem;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.center-modal__action-btn {
  padding: 0.75rem 1.5rem;
  border-radius: 6px;
  border: none;
  background: #e5e7eb;
  color: #374151;
  font-size: 0.95rem;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
}

.center-modal__action-btn:hover:not(:disabled) {
  background: #d1d5db;
}

.center-modal__action-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.center-modal__action-btn--primary {
  background: var(--gradient);
  color: white;
}

.center-modal__action-btn--primary:hover:not(:disabled) {
  opacity: 0.9;
}

@media (max-width: 480px) {
  .center-modal {
    max-width: 100%;
    padding: 1.5rem;
  }

  .center-modal__title {
    font-size: 1.25rem;
  }

  .center-modal__footer {
    flex-direction: column-reverse;
  }

  .center-modal__action-btn {
    flex: 1;
  }
}
</style>
