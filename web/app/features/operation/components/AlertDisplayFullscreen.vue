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
</script>

<template>
  <div v-if="primaryAlert" class="fullscreen-alert" :style="{ background: getColorGradient(primaryAlert.colorTheme || 'amber') }">
    <div class="fullscreen-alert__container">
      <div class="fullscreen-alert__header">
        <h1 class="fullscreen-alert__title">⚠️ {{ renderTemplate(primaryAlert.titleTemplate || "", primaryAlert) }}</h1>
      </div>

      <div class="fullscreen-alert__content">
        <p class="fullscreen-alert__body">{{ renderTemplate(primaryAlert.body || primaryAlert.bodyTemplate || "", primaryAlert) }}</p>
      </div>

      <div class="fullscreen-alert__footer">
        <button
          v-if="primaryAlert.responseOptions?.length"
          v-for="opt in primaryAlert.responseOptions"
          :key="opt.value"
          class="fullscreen-alert__action-btn"
          :disabled="respondingToId === primaryAlert.id"
          @click="handleResponse(opt.value)"
        >
          <span>{{ respondingToId === primaryAlert.id ? "Processando..." : opt.label }}</span>
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.fullscreen-alert {
  position: fixed;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 700;
  padding: 2rem;
}

.fullscreen-alert__container {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 3rem;
  max-width: 600px;
  text-align: center;
}

.fullscreen-alert__header {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.fullscreen-alert__title {
  margin: 0;
  font-size: 3rem;
  font-weight: 900;
  color: white;
  text-shadow: 0 2px 8px rgba(0, 0, 0, 0.3);
  line-height: 1.2;
}

.fullscreen-alert__content {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.fullscreen-alert__body {
  margin: 0;
  font-size: 1.5rem;
  color: rgba(255, 255, 255, 0.95);
  line-height: 1.6;
  text-shadow: 0 1px 4px rgba(0, 0, 0, 0.2);
}

.fullscreen-alert__footer {
  display: flex;
  gap: 1rem;
  flex-wrap: wrap;
  justify-content: center;
  margin-top: 2rem;
}

.fullscreen-alert__action-btn {
  padding: 1rem 2.5rem;
  border-radius: 8px;
  border: 3px solid white;
  background: rgba(255, 255, 255, 0.15);
  color: white;
  font-size: 1.1rem;
  font-weight: 700;
  cursor: pointer;
  transition: all 0.3s;
  backdrop-filter: blur(10px);
  min-width: 200px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.fullscreen-alert__action-btn:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.25);
  transform: scale(1.05);
}

.fullscreen-alert__action-btn:disabled {
  opacity: 0.7;
  cursor: not-allowed;
}

@media (max-width: 768px) {
  .fullscreen-alert {
    padding: 1rem;
  }

  .fullscreen-alert__container {
    gap: 2rem;
  }

  .fullscreen-alert__title {
    font-size: 2rem;
  }

  .fullscreen-alert__body {
    font-size: 1.1rem;
  }

  .fullscreen-alert__action-btn {
    padding: 0.75rem 1.5rem;
    font-size: 0.95rem;
    min-width: 150px;
  }
}
</style>
