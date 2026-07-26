<script setup lang="ts">
import { computed } from 'vue'

import type { SocialPublishingPortfolio } from '~/domain/social-publishing/model'

const props = defineProps<{
  portfolio: SocialPublishingPortfolio | null
  loading: boolean
  error: string
}>()

const emit = defineEmits<{
  retry: []
  select: [accountId: string]
}>()

const numberFormatter = new Intl.NumberFormat('pt-BR')
const dateFormatter = new Intl.DateTimeFormat('pt-BR', {
  day: '2-digit',
  month: 'short',
  hour: '2-digit',
  minute: '2-digit',
})

const summaryCards = computed(() => {
  const portfolio = props.portfolio
  if (!portfolio) return []
  return [
    {
      label: 'Instagrams conectados',
      value: `${portfolio.connectedClients}/${portfolio.clientCount}`,
      icon: 'i-lucide-instagram',
    },
    {
      label: 'Na fila',
      value: formatNumber(portfolio.scheduled + portfolio.publishing),
      icon: 'i-lucide-calendar-clock',
    },
    {
      label: 'Publicadas',
      value: formatNumber(portfolio.published),
      icon: 'i-lucide-circle-check-big',
    },
    {
      label: 'Visualizações',
      value: formatNumber(portfolio.views),
      icon: 'i-lucide-eye',
    },
    {
      label: 'Alcance',
      value: formatNumber(portfolio.reach),
      icon: 'i-lucide-radio-tower',
    },
    {
      label: 'Interações',
      value: formatNumber(portfolio.totalInteractions),
      icon: 'i-lucide-messages-square',
    },
  ]
})

function formatNumber(value: number): string {
  return numberFormatter.format(value)
}

function formatDate(value: string | null): string {
  if (!value) return 'Nenhuma postagem agendada'
  const parsed = new Date(value)
  return Number.isNaN(parsed.getTime())
    ? 'Nenhuma postagem agendada'
    : `Próxima em ${dateFormatter.format(parsed)}`
}
</script>

<template>
  <section class="sp-portfolio" aria-labelledby="sp-portfolio-title">
    <div v-if="loading && !portfolio" class="sp-portfolio__state" aria-live="polite">
      <span class="sp-portfolio__spinner" aria-hidden="true"></span>
      <p>Consolidando as postagens dos clientes…</p>
    </div>

    <div v-else-if="error" class="sp-portfolio__state sp-portfolio__state--error" role="alert">
      <UIcon name="i-lucide-circle-alert" aria-hidden="true" />
      <h2 id="sp-portfolio-title">Não foi possível carregar o consolidado</h2>
      <p>{{ error }}</p>
      <UButton
        type="button"
        color="error"
        variant="soft"
        size="sm"
        label="Tentar novamente"
        @click="emit('retry')"
      />
    </div>

    <div v-else-if="portfolio" class="sp-portfolio__content">
      <header class="sp-portfolio__heading">
        <div>
          <p class="sp-portfolio__eyebrow">Visão geral</p>
          <h2 id="sp-portfolio-title">Todos os clientes</h2>
          <p>Dados consolidados das contas que você pode acompanhar.</p>
        </div>
        <span v-if="portfolio.capturedAt" class="sp-portfolio__captured">
          Atualizado em {{ formatDate(portfolio.capturedAt).replace('Próxima em ', '') }}
        </span>
      </header>

      <div class="sp-portfolio__summary">
        <article v-for="card in summaryCards" :key="card.label" class="sp-portfolio__metric">
          <UIcon :name="card.icon" aria-hidden="true" />
          <span>{{ card.label }}</span>
          <strong>{{ card.value }}</strong>
        </article>
      </div>

      <div class="sp-portfolio__section-heading">
        <div>
          <h3>Clientes</h3>
          <p>Selecione um cliente para abrir fila, conteúdo, conexão e analytics detalhados.</p>
        </div>
        <span>{{ portfolio.clientCount }} cliente(s)</span>
      </div>

      <div v-if="portfolio.clients.length" class="sp-portfolio__clients">
        <button
          v-for="client in portfolio.clients"
          :key="client.accountId"
          type="button"
          class="sp-portfolio__client"
          :aria-label="`Abrir postagens de ${client.accountName}`"
          @click="emit('select', client.accountId)"
        >
          <span class="sp-portfolio__client-head">
            <span class="sp-portfolio__identity">
              <span class="sp-portfolio__avatar" aria-hidden="true">
                {{ client.accountName.slice(0, 1).toUpperCase() }}
              </span>
              <span>
                <strong>{{ client.accountName }}</strong>
                <small>
                  {{ client.username ? `@${client.username}` : 'Instagram não identificado' }}
                </small>
              </span>
            </span>
            <span
              class="sp-portfolio__status"
              :class="{ 'sp-portfolio__status--connected': client.connected }"
            >
              {{ client.connected ? 'Conectado' : 'Desconectado' }}
            </span>
          </span>

          <span class="sp-portfolio__client-metrics">
            <span>
              <strong>{{ formatNumber(client.scheduled) }}</strong>
              agendadas
            </span>
            <span>
              <strong>{{ formatNumber(client.published) }}</strong>
              publicadas
            </span>
            <span>
              <strong>{{ formatNumber(client.reach) }}</strong>
              alcance
            </span>
            <span>
              <strong>{{ formatNumber(client.totalInteractions) }}</strong>
              interações
            </span>
          </span>

          <span class="sp-portfolio__client-foot">
            <span>{{ formatDate(client.nextScheduledFor) }}</span>
            <span class="sp-portfolio__open">
              Abrir cliente
              <UIcon name="i-lucide-arrow-right" aria-hidden="true" />
            </span>
          </span>
        </button>
      </div>

      <div v-else class="sp-portfolio__empty">
        <UIcon name="i-lucide-users" aria-hidden="true" />
        <p>
          Nenhum cliente deste escopo está com o módulo Agendamento de Postagens habilitado.
          Habilite-o no cliente pela gestão padrão de módulos.
        </p>
      </div>
    </div>

    <div v-else class="sp-portfolio__state">
      <UIcon name="i-lucide-chart-no-axes-column" aria-hidden="true" />
      <p>O consolidado ainda não está disponível.</p>
      <UButton type="button" variant="soft" size="sm" label="Carregar" @click="emit('retry')" />
    </div>
  </section>
</template>

<style scoped>
.sp-portfolio,
.sp-portfolio__content {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 1rem;
}
.sp-portfolio__state {
  display: grid;
  min-height: 18rem;
  place-content: center;
  justify-items: center;
  gap: 0.7rem;
  padding: 2rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  color: rgb(var(--muted));
  background: rgb(var(--surface));
  text-align: center;
}
.sp-portfolio__state :deep(svg) {
  width: 2rem;
  height: 2rem;
}
.sp-portfolio__state h2,
.sp-portfolio__state p {
  margin: 0;
}
.sp-portfolio__state h2 {
  color: rgb(var(--text));
  font-size: 1rem;
}
.sp-portfolio__state--error :deep(svg) {
  color: rgb(var(--danger));
}
.sp-portfolio__spinner {
  width: 1.8rem;
  height: 1.8rem;
  border: 2px solid rgb(var(--border));
  border-top-color: rgb(var(--primary));
  border-radius: 999px;
  animation: sp-portfolio-spin 0.8s linear infinite;
}
.sp-portfolio__heading,
.sp-portfolio__section-heading,
.sp-portfolio__client-head,
.sp-portfolio__client-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
}
.sp-portfolio__heading h2,
.sp-portfolio__section-heading h3 {
  margin: 0;
  color: rgb(var(--text));
}
.sp-portfolio__heading h2 {
  font-size: 1.15rem;
}
.sp-portfolio__heading p,
.sp-portfolio__section-heading p {
  margin: 0.2rem 0 0;
  color: rgb(var(--muted));
  font-size: 0.78rem;
}
.sp-portfolio__eyebrow {
  color: rgb(var(--primary)) !important;
  font-weight: 750;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}
.sp-portfolio__captured,
.sp-portfolio__section-heading > span {
  color: rgb(var(--muted));
  font-size: 0.72rem;
  white-space: nowrap;
}
.sp-portfolio__summary {
  display: grid;
  grid-template-columns: repeat(6, minmax(0, 1fr));
  gap: 0.7rem;
}
.sp-portfolio__metric {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 0.35rem 0.45rem;
  padding: 0.85rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-soft);
  background: rgb(var(--surface));
}
.sp-portfolio__metric :deep(svg) {
  width: 1rem;
  height: 1rem;
  color: rgb(var(--primary));
}
.sp-portfolio__metric span {
  color: rgb(var(--muted));
  font-size: 0.68rem;
}
.sp-portfolio__metric strong {
  grid-column: 1 / -1;
  color: rgb(var(--text));
  font-size: 1.2rem;
}
.sp-portfolio__section-heading {
  margin-top: 0.2rem;
}
.sp-portfolio__section-heading h3 {
  font-size: 0.95rem;
}
.sp-portfolio__clients {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem;
}
.sp-portfolio__client {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 0.9rem;
  padding: 1rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-soft);
  color: inherit;
  background: rgb(var(--surface));
  cursor: pointer;
  font: inherit;
  text-align: left;
  transition:
    border-color 0.18s ease,
    transform 0.18s ease;
}
.sp-portfolio__client:hover {
  border-color: rgb(var(--primary) / 0.38);
  transform: translateY(-1px);
}
.sp-portfolio__client:focus-visible {
  outline: 2px solid rgb(var(--ring));
  outline-offset: 2px;
}
.sp-portfolio__identity {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.65rem;
}
.sp-portfolio__identity > span:last-child {
  display: grid;
  min-width: 0;
  gap: 0.15rem;
}
.sp-portfolio__identity strong,
.sp-portfolio__identity small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.sp-portfolio__identity strong {
  color: rgb(var(--text));
  font-size: 0.84rem;
}
.sp-portfolio__identity small {
  color: rgb(var(--muted));
  font-size: 0.7rem;
}
.sp-portfolio__avatar {
  display: grid;
  width: 2rem;
  height: 2rem;
  flex: 0 0 auto;
  place-items: center;
  border-radius: 0.65rem;
  color: rgb(var(--primary));
  background: rgb(var(--primary) / 0.12);
  font-size: 0.78rem;
  font-weight: 800;
}
.sp-portfolio__status {
  flex: 0 0 auto;
  padding: 0.25rem 0.45rem;
  border-radius: 999px;
  color: rgb(var(--muted));
  background: rgb(var(--surface-2));
  font-size: 0.64rem;
  font-weight: 700;
}
.sp-portfolio__status--connected {
  color: rgb(var(--success));
  background: rgb(var(--success) / 0.12);
}
.sp-portfolio__client-metrics {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.5rem;
}
.sp-portfolio__client-metrics > span {
  padding: 0.5rem;
  border-radius: var(--radius-xs);
  color: rgb(var(--muted));
  background: rgb(var(--surface-2) / 0.7);
  font-size: 0.68rem;
}
.sp-portfolio__client-metrics strong {
  color: rgb(var(--text));
}
.sp-portfolio__client-foot {
  color: rgb(var(--muted));
  font-size: 0.68rem;
}
.sp-portfolio__open {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  color: rgb(var(--primary));
  font-weight: 700;
}
.sp-portfolio__open :deep(svg) {
  width: 0.8rem;
  height: 0.8rem;
}
.sp-portfolio__empty {
  display: grid;
  min-height: 10rem;
  place-content: center;
  justify-items: center;
  color: rgb(var(--muted));
  text-align: center;
}
@keyframes sp-portfolio-spin {
  to {
    transform: rotate(360deg);
  }
}
@media (prefers-reduced-motion: reduce) {
  .sp-portfolio__spinner,
  .sp-portfolio__client {
    animation: none;
    transition: none;
  }
}
@media (max-width: 980px) {
  .sp-portfolio__summary {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}
@media (max-width: 680px) {
  .sp-portfolio__heading,
  .sp-portfolio__section-heading {
    align-items: flex-start;
    flex-direction: column;
  }
  .sp-portfolio__clients {
    grid-template-columns: 1fr;
  }
}
@media (max-width: 460px) {
  .sp-portfolio__summary {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
