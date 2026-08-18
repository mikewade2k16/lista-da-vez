<script setup lang="ts">
import { AlertTriangle, CalendarCheck2, CircleCheck, RefreshCw } from 'lucide-vue-next'
import { computed, onMounted } from 'vue'
import { useContentOperations } from '~/composables/useContentOperations'
import type { ContentOperationsAlert } from '~/domain/content-operations/content-operations-api'

definePageMeta({ layout: 'dashboard', workspaceId: 'calendar', pageLabel: 'Operação de conteúdo' })
const { brief, loading, error, refresh } = useContentOperations()
const grouped = computed(() => {
  const groups = new Map<string, ContentOperationsAlert[]>()
  for (const alert of brief.value?.alerts ?? [])
    groups.set(alert.clientId, [...(groups.get(alert.clientId) ?? []), alert])
  return [...groups.entries()].map(([clientId, alerts]) => ({
    clientId,
    clientName: alerts[0]?.clientName ?? 'Cliente',
    alerts,
  }))
})
onMounted(() => void refresh())
</script>

<template>
  <div class="content-ops-page">
    <header class="content-ops-page__header">
      <div>
        <small>Organização automática</small>
        <h1>{{ brief?.headline || 'Operação de conteúdo' }}</h1>
        <p>{{ brief?.summary || 'Lembretes calculados pelas tasks e pelo calendário.' }}</p>
      </div>
      <button type="button" :disabled="loading" @click="refresh()">
        <RefreshCw :size="17" :class="{ spin: loading }" />
        Atualizar
      </button>
    </header>
    <p v-if="error" class="content-ops-page__error">{{ error }}</p>
    <section v-if="brief" class="content-ops-stats">
      <article>
        <AlertTriangle :size="20" />
        <strong>{{ brief.counts.critical }}</strong>
        <span>urgentes</span>
      </article>
      <article>
        <CalendarCheck2 :size="20" />
        <strong>{{ brief.counts.attention }}</strong>
        <span>para acompanhar</span>
      </article>
      <article>
        <CircleCheck :size="20" />
        <strong>{{ brief.clients.length }}</strong>
        <span>clientes analisados</span>
      </article>
    </section>
    <section v-if="grouped.length" class="content-ops-groups">
      <article v-for="group in grouped" :key="group.clientId" class="content-ops-client">
        <header>
          <div>
            <small>Cliente</small>
            <h2>{{ group.clientName }}</h2>
          </div>
          <span>{{ group.alerts.length }} lembrete{{ group.alerts.length === 1 ? '' : 's' }}</span>
        </header>
        <NuxtLink
          v-for="alert in group.alerts"
          :key="alert.id"
          :to="alert.linkPath"
          class="content-ops-alert"
          :class="`is-${alert.severity}`"
        >
          <span class="content-ops-alert__dot"></span>
          <span>
            <strong>{{ alert.title }}</strong>
            <p>{{ alert.body }}</p>
          </span>
          <span>Abrir</span>
        </NuxtLink>
      </article>
    </section>
    <section v-else-if="brief && !loading" class="content-ops-empty">
      <CircleCheck :size="34" />
      <h2>Tudo organizado</h2>
      <p>Não há lembretes ativos neste momento.</p>
    </section>
  </div>
</template>

<style scoped>
.content-ops-page {
  width: min(1180px, 100%);
  margin: 0 auto;
  padding: 1.25rem 1rem 3rem;
  color: rgb(var(--text));
  overflow: auto;
}
.content-ops-page__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  padding: 1.4rem;
  border: 1px solid rgb(var(--border) / 0.85);
  border-radius: 1.3rem;
  background: linear-gradient(135deg, rgb(var(--primary) / 0.12), rgb(var(--surface)));
}
h1,
h2,
p {
  margin: 0;
}
.content-ops-page__header small,
.content-ops-client small {
  color: rgb(var(--primary));
  font-size: 0.72rem;
  font-weight: 800;
  text-transform: uppercase;
  letter-spacing: 0.08em;
}
.content-ops-page__header h1 {
  margin: 0.15rem 0;
  font-size: clamp(1.5rem, 3vw, 2.2rem);
}
.content-ops-page__header p {
  color: rgb(var(--muted));
}
.content-ops-page__header button {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.65rem 0.8rem;
  border: 1px solid rgb(var(--border));
  border-radius: 0.75rem;
  background: rgb(var(--surface));
  color: inherit;
  cursor: pointer;
}
.content-ops-page__error {
  margin-top: 1rem;
  padding: 0.8rem;
  border-radius: 0.8rem;
  background: rgb(var(--danger) / 0.1);
  color: rgb(var(--danger));
}
.content-ops-stats {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 0.8rem;
  margin: 1rem 0;
}
.content-ops-stats article {
  display: grid;
  grid-template-columns: auto auto 1fr;
  align-items: center;
  gap: 0.55rem;
  padding: 1rem;
  border: 1px solid rgb(var(--border) / 0.8);
  border-radius: 1rem;
  background: rgb(var(--surface));
}
.content-ops-stats strong {
  font-size: 1.45rem;
}
.content-ops-stats span {
  color: rgb(var(--muted));
}
.content-ops-groups {
  display: grid;
  gap: 1rem;
}
.content-ops-client {
  padding: 1rem;
  border: 1px solid rgb(var(--border) / 0.85);
  border-radius: 1.1rem;
  background: rgb(var(--surface));
}
.content-ops-client > header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0.2rem 0.2rem 0.8rem;
}
.content-ops-client > header > span {
  color: rgb(var(--muted));
  font-size: 0.8rem;
}
.content-ops-alert {
  display: grid;
  grid-template-columns: auto 1fr auto;
  align-items: center;
  gap: 0.7rem;
  padding: 0.85rem 0.4rem;
  border-top: 1px solid rgb(var(--border) / 0.7);
  color: inherit;
  text-decoration: none;
}
.content-ops-alert p {
  margin-top: 0.15rem;
  color: rgb(var(--muted));
  font-size: 0.85rem;
}
.content-ops-alert > span:last-child {
  color: rgb(var(--primary));
  font-size: 0.78rem;
  font-weight: 700;
}
.content-ops-alert__dot {
  width: 0.6rem;
  height: 0.6rem;
  border-radius: 50%;
  background: rgb(var(--primary));
}
.content-ops-alert.is-critical .content-ops-alert__dot {
  background: rgb(var(--danger));
}
.content-ops-alert.is-attention .content-ops-alert__dot {
  background: #f59e0b;
}
.content-ops-empty {
  display: grid;
  place-items: center;
  gap: 0.35rem;
  padding: 4rem;
  color: rgb(var(--muted));
}
.spin {
  animation: spin 0.8s linear infinite;
}
@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}
@media (max-width: 700px) {
  .content-ops-page__header {
    display: grid;
  }
  .content-ops-stats {
    grid-template-columns: 1fr;
  }
  .content-ops-alert {
    grid-template-columns: auto 1fr;
  }
  .content-ops-alert > span:last-child {
    display: none;
  }
}
</style>
