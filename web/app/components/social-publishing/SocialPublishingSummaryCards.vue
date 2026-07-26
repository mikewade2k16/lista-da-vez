<script setup lang="ts">
import type {
  SocialPublishingConnection,
  SocialPublishingOverview,
} from '~/domain/social-publishing/model'

const props = defineProps<{
  overview: SocialPublishingOverview | null
  connection: SocialPublishingConnection | null
}>()

const numberFormatter = new Intl.NumberFormat('pt-BR')
const dateFormatter = new Intl.DateTimeFormat('pt-BR', {
  day: '2-digit',
  month: 'short',
  hour: '2-digit',
  minute: '2-digit',
})

const cards = computed(() => [
  {
    label: 'Na fila',
    value: props.overview?.scheduled ?? null,
    detail: props.overview?.upcoming[0]?.scheduledFor
      ? `Próxima: ${dateFormatter.format(new Date(props.overview.upcoming[0].scheduledFor))}`
      : props.overview
        ? 'Nenhuma postagem agendada'
        : 'Total disponível com analytics',
    icon: 'i-lucide-calendar-clock',
    tone: 'primary',
  },
  {
    label: 'Rascunhos',
    value: props.overview?.draft ?? null,
    detail: props.overview ? 'Conteúdos em preparação' : 'Total disponível com analytics',
    icon: 'i-lucide-file-pen-line',
    tone: 'neutral',
  },
  {
    label: 'Publicadas',
    value: props.overview?.published ?? null,
    detail: props.overview ? 'Histórico deste cliente' : 'Total disponível com analytics',
    icon: 'i-lucide-circle-check',
    tone: 'success',
  },
  {
    label: 'Atenção',
    value: props.overview?.failed ?? null,
    detail: !props.connection?.connected
      ? 'Instagram desconectado'
      : props.overview
        ? 'Falhas que precisam de revisão'
        : 'Total disponível com analytics',
    icon: 'i-lucide-triangle-alert',
    tone: 'danger',
  },
])

function formatValue(value: number | null): string {
  return value === null ? '—' : numberFormatter.format(value)
}
</script>

<template>
  <section class="sp-summary" aria-label="Resumo das postagens">
    <article
      v-for="card in cards"
      :key="card.label"
      class="sp-summary__card omni-glass"
      :class="`sp-summary__card--${card.tone}`"
    >
      <div class="sp-summary__icon" aria-hidden="true">
        <UIcon :name="card.icon" />
      </div>
      <div>
        <p class="sp-summary__label">{{ card.label }}</p>
        <p class="sp-summary__value">{{ formatValue(card.value) }}</p>
        <p class="sp-summary__detail">{{ card.detail }}</p>
      </div>
    </article>
  </section>
</template>

<style scoped>
.sp-summary {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 0.85rem;
}

.sp-summary__card {
  display: flex;
  gap: 0.8rem;
  min-width: 0;
  padding: 1rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface));
  box-shadow: var(--shadow-xs);
}

.sp-summary__icon {
  display: grid;
  width: 2.4rem;
  height: 2.4rem;
  flex: 0 0 auto;
  place-items: center;
  border-radius: var(--radius-soft);
  color: rgb(var(--muted));
  background: rgb(var(--surface-2));
}

.sp-summary__card--primary .sp-summary__icon {
  color: rgb(var(--primary));
  background: rgb(var(--primary) / 0.12);
}

.sp-summary__card--success .sp-summary__icon {
  color: rgb(var(--success));
  background: rgb(var(--success) / 0.12);
}

.sp-summary__card--danger .sp-summary__icon {
  color: rgb(var(--danger));
  background: rgb(var(--danger) / 0.12);
}

.sp-summary__label,
.sp-summary__detail {
  margin: 0;
  color: rgb(var(--muted));
}

.sp-summary__label {
  font-size: 0.75rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.sp-summary__value {
  margin: 0.15rem 0 0;
  color: rgb(var(--text));
  font-size: 1.65rem;
  font-weight: 750;
  line-height: 1.1;
}

.sp-summary__detail {
  margin-top: 0.3rem;
  font-size: 0.75rem;
  line-height: 1.35;
}

@media (max-width: 1000px) {
  .sp-summary {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 560px) {
  .sp-summary {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
