<script setup lang="ts">
import { computed } from 'vue'
import type { AutomationAttendance, AutomationProfile } from '~/domain/omnichannel/automation-api'

const props = defineProps<{
  profiles: AutomationProfile[]
  interventions: AutomationAttendance[]
}>()

const configured = computed(() => props.profiles.filter((item) => item.configured).length)
const enabled = computed(() => props.profiles.filter((item) => item.enabled).length)
const ready = computed(() => props.profiles.filter((item) => item.ready).length)
const stopped = computed(
  () => props.interventions.filter((item) => item.mode === 'ai_stopped').length,
)
</script>

<template>
  <section class="automation-overview" aria-label="Resumo da automação">
    <div class="automation-overview__metrics">
      <article class="automation-metric">
        <span>Clientes configurados</span>
        <strong>{{ configured }}</strong>
        <small>de {{ profiles.length }} visíveis</small>
      </article>
      <article class="automation-metric">
        <span>Automações ligadas</span>
        <strong>{{ enabled }}</strong>
        <small>primeiro atendimento por IA</small>
      </article>
      <article class="automation-metric">
        <span>Prontas para atender</span>
        <strong>{{ ready }}</strong>
        <small>número e agente publicados</small>
      </article>
      <article class="automation-metric automation-metric--attention">
        <span>Precisam de humano</span>
        <strong>{{ stopped }}</strong>
        <small>handoffs aguardando entrada</small>
      </article>
    </div>

    <div class="automation-overview__flow">
      <div>
        <UIcon name="i-lucide-message-circle" />
        <span>Cliente chama no WhatsApp</span>
      </div>
      <UIcon name="i-lucide-arrow-right" class="automation-overview__arrow" />
      <div>
        <UIcon name="i-lucide-bot" />
        <span>IA faz o primeiro atendimento</span>
      </div>
      <UIcon name="i-lucide-arrow-right" class="automation-overview__arrow" />
      <div>
        <UIcon name="i-lucide-route" />
        <span>Go encerra ou transfere com segurança</span>
      </div>
    </div>
  </section>
</template>

<style scoped>
.automation-overview {
  display: grid;
  gap: 1rem;
}

.automation-overview__metrics {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 0.75rem;
}

.automation-metric {
  display: grid;
  gap: 0.25rem;
  padding: 1rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.72);
}

.automation-metric span,
.automation-metric small {
  color: var(--text-muted);
  font-size: 0.76rem;
}

.automation-metric strong {
  color: var(--text-main);
  font-size: 1.75rem;
  line-height: 1;
}

.automation-metric--attention {
  border-color: color-mix(in srgb, var(--accent-warning) 45%, var(--line-soft));
}

.automation-overview__flow {
  display: grid;
  grid-template-columns: 1fr auto 1fr auto 1fr;
  align-items: center;
  gap: 0.75rem;
  padding: 1rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface-2) / 0.45);
}

.automation-overview__flow > div {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  color: var(--text-main);
  text-align: center;
  font-size: 0.84rem;
}

.automation-overview__arrow {
  color: rgb(var(--primary));
}

@media (max-width: 900px) {
  .automation-overview__metrics {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .automation-overview__flow {
    grid-template-columns: 1fr;
  }

  .automation-overview__arrow {
    transform: rotate(90deg);
    justify-self: center;
  }
}
</style>
