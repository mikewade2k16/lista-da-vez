<script setup lang="ts">
import type { SiteTrackingAnalytics, SiteTrackingCountItem } from '~/types/tracking'

const props = defineProps<{
  analytics: SiteTrackingAnalytics
  loading?: boolean
  days: number
}>()

const emit = defineEmits<{
  (event: 'update:days', value: number): void
  (event: 'refresh'): void
}>()

const dayOptions = [
  { label: '7 dias', value: 7 },
  { label: '14 dias', value: 14 },
  { label: '30 dias', value: 30 },
  { label: '90 dias', value: 90 },
]

const totals = computed(() => props.analytics.totals)

const kpis = computed(() => [
  {
    key: 'total',
    label: 'Total de eventos',
    value: totals.value.totalEvents,
    hint: `nos últimos ${props.analytics.rangeDays} dias`,
  },
  { key: 'today', label: 'Hoje', value: totals.value.today, hint: 'eventos de hoje' },
  { key: 'last7', label: 'Últimos 7 dias', value: totals.value.last7Days, hint: 'janela curta' },
  {
    key: 'sessions',
    label: 'Sessões',
    value: totals.value.totalSessions,
    hint: 'sessões distintas',
  },
  {
    key: 'visitors',
    label: 'Visitantes',
    value: totals.value.totalVisitors,
    hint: 'visitantes únicos',
  },
  {
    key: 'pageviews',
    label: 'Page views',
    value: totals.value.pageViews,
    hint: 'visualizações de página',
  },
])

function formatNumber(value: number) {
  return Number(value || 0).toLocaleString('pt-BR')
}

function barPct(count: number, items: SiteTrackingCountItem[]) {
  const max = items.reduce((acc, item) => Math.max(acc, item.count), 0)
  if (max <= 0) return '0%'
  return `${Math.max(4, Math.round((count / max) * 100))}%`
}

const accessMax = computed(() =>
  props.analytics.accessByDay.reduce((acc, item) => Math.max(acc, item.count), 0),
)

function accessPct(count: number) {
  if (accessMax.value <= 0) return '0%'
  return `${Math.max(2, Math.round((count / accessMax.value) * 100))}%`
}

function formatDay(value: string) {
  const parsed = new Date(`${value}T00:00:00`)
  if (Number.isNaN(parsed.getTime())) return value
  return parsed.toLocaleDateString('pt-BR', { day: '2-digit', month: '2-digit' })
}

function formatDateTime(value: string) {
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return value || '-'
  return parsed.toLocaleString('pt-BR')
}

function onDaysChange(value: string | number) {
  const parsed = Number(value)
  if (Number.isFinite(parsed)) emit('update:days', parsed)
}

const hasData = computed(() => totals.value.totalEvents > 0)
</script>

<template>
  <section class="site-tracking-dashboard grid gap-4">
    <header class="flex flex-wrap items-center justify-between gap-3">
      <p class="text-sm text-[rgb(var(--muted))]">
        Resumo agregado dos eventos recebidos no período selecionado.
      </p>
      <div class="flex items-center gap-2">
        <USelect
          :model-value="props.days"
          :items="dayOptions"
          @update:model-value="onDaysChange($event as string | number)"
        />
        <UButton
          icon="i-lucide-refresh-cw"
          color="neutral"
          variant="soft"
          :loading="props.loading"
          @click="emit('refresh')"
        />
      </div>
    </header>

    <UAlert
      v-if="!hasData && !props.loading"
      color="neutral"
      variant="soft"
      icon="i-lucide-inbox"
      title="Sem eventos no período"
      description="Nenhum evento de tracking foi recebido na janela selecionada. Aumente o período ou aguarde novos eventos do site."
    />

    <!-- KPIs -->
    <div class="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
      <article
        v-for="kpi in kpis"
        :key="kpi.key"
        class="grid gap-1 rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] p-4"
      >
        <span class="text-xs uppercase tracking-wide text-[rgb(var(--muted))]">
          {{ kpi.label }}
        </span>
        <strong class="text-2xl font-semibold text-[rgb(var(--text))]">
          {{ formatNumber(kpi.value) }}
        </strong>
        <span class="text-xs text-[rgb(var(--muted))]">{{ kpi.hint }}</span>
      </article>
    </div>

    <!-- Conversões dinâmicas -->
    <section
      class="grid gap-2 rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] p-4"
    >
      <h3 class="text-sm font-semibold uppercase tracking-wide text-[rgb(var(--muted))]">
        Conversões
      </h3>
      <p v-if="!props.analytics.conversions.length" class="text-sm text-[rgb(var(--muted))]">
        Nenhum evento de interação no período (ex.: WhatsApp, mapa, cookie). Quando o site enviar
        esses eventos, eles aparecem aqui automaticamente.
      </p>
      <div v-else class="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
        <article
          v-for="conversion in props.analytics.conversions"
          :key="conversion.key"
          class="grid gap-1 rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface))] p-3"
        >
          <span class="text-xs uppercase tracking-wide text-[rgb(var(--muted))]">
            {{ conversion.label }}
          </span>
          <strong class="text-xl font-semibold text-[rgb(var(--text))]">
            {{ formatNumber(conversion.count) }}
          </strong>
          <span class="text-xs text-[rgb(var(--muted))]">
            {{ conversion.percentOfVisitors }}% dos visitantes
          </span>
        </article>
      </div>
    </section>

    <!-- Dispositivos + Eventos -->
    <div class="grid gap-3 lg:grid-cols-2">
      <section
        class="grid gap-3 rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] p-4"
      >
        <h3 class="text-sm font-semibold uppercase tracking-wide text-[rgb(var(--muted))]">
          Dispositivos
        </h3>
        <p v-if="!props.analytics.devices.length" class="text-sm text-[rgb(var(--muted))]">
          Sem dados.
        </p>
        <div v-for="item in props.analytics.devices" :key="item.label" class="grid gap-1">
          <div class="flex items-center justify-between text-sm">
            <span class="text-[rgb(var(--text))]">{{ item.label }}</span>
            <span class="text-[rgb(var(--muted))]">{{ formatNumber(item.count) }}</span>
          </div>
          <div class="h-2 overflow-hidden rounded-full bg-[rgb(var(--surface))]">
            <div
              class="h-full rounded-full bg-[rgb(var(--primary))]"
              :style="{ width: barPct(item.count, props.analytics.devices) }"
            ></div>
          </div>
        </div>
      </section>

      <section
        class="grid gap-3 rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] p-4"
      >
        <h3 class="text-sm font-semibold uppercase tracking-wide text-[rgb(var(--muted))]">
          Eventos por tipo
        </h3>
        <p v-if="!props.analytics.eventsByType.length" class="text-sm text-[rgb(var(--muted))]">
          Sem dados.
        </p>
        <div v-for="item in props.analytics.eventsByType" :key="item.label" class="grid gap-1">
          <div class="flex items-center justify-between text-sm">
            <span class="truncate text-[rgb(var(--text))]">{{ item.label }}</span>
            <span class="text-[rgb(var(--muted))]">{{ formatNumber(item.count) }}</span>
          </div>
          <div class="h-2 overflow-hidden rounded-full bg-[rgb(var(--surface))]">
            <div
              class="h-full rounded-full bg-[rgb(var(--primary))]"
              :style="{ width: barPct(item.count, props.analytics.eventsByType) }"
            ></div>
          </div>
        </div>
      </section>
    </div>

    <!-- Acessos por dia -->
    <section
      class="grid gap-2 rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] p-4"
    >
      <h3 class="text-sm font-semibold uppercase tracking-wide text-[rgb(var(--muted))]">
        Acessos por dia ({{ props.analytics.rangeDays }} dias)
      </h3>
      <div
        v-for="item in props.analytics.accessByDay"
        :key="item.date"
        class="flex items-center gap-3 text-sm"
      >
        <span class="w-14 shrink-0 text-[rgb(var(--muted))]">{{ formatDay(item.date) }}</span>
        <div class="h-2 flex-1 overflow-hidden rounded-full bg-[rgb(var(--surface))]">
          <div
            class="h-full rounded-full bg-[rgb(var(--primary))]"
            :style="{ width: accessPct(item.count) }"
          ></div>
        </div>
        <span class="w-10 shrink-0 text-right text-[rgb(var(--muted))]">
          {{ formatNumber(item.count) }}
        </span>
      </div>
    </section>

    <!-- Origem do tráfego -->
    <section
      class="grid gap-2 rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] p-4"
    >
      <h3 class="text-sm font-semibold uppercase tracking-wide text-[rgb(var(--muted))]">
        Origem do tráfego (top 10)
      </h3>
      <p v-if="!props.analytics.topReferrers.length" class="text-sm text-[rgb(var(--muted))]">
        Sem dados.
      </p>
      <div
        v-for="item in props.analytics.topReferrers"
        :key="item.label"
        class="flex items-center gap-3 text-sm"
      >
        <span class="w-48 shrink-0 truncate text-[rgb(var(--text))]" :title="item.label">
          {{ item.label }}
        </span>
        <div class="h-2 flex-1 overflow-hidden rounded-full bg-[rgb(var(--surface))]">
          <div
            class="h-full rounded-full bg-[rgb(var(--primary))]"
            :style="{ width: barPct(item.count, props.analytics.topReferrers) }"
          ></div>
        </div>
        <span class="w-10 shrink-0 text-right text-[rgb(var(--muted))]">
          {{ formatNumber(item.count) }}
        </span>
      </div>
    </section>

    <!-- Últimas visitas -->
    <section
      class="grid gap-2 rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] p-4"
    >
      <h3 class="text-sm font-semibold uppercase tracking-wide text-[rgb(var(--muted))]">
        Últimas visitas
      </h3>
      <p v-if="!props.analytics.recentVisits.length" class="text-sm text-[rgb(var(--muted))]">
        Sem dados.
      </p>
      <div v-else class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="text-left text-xs uppercase tracking-wide text-[rgb(var(--muted))]">
              <th class="py-2 pr-3 font-medium">Data / hora</th>
              <th class="py-2 pr-3 font-medium">Dispositivo</th>
              <th class="py-2 pr-3 font-medium">Página</th>
              <th class="py-2 pr-3 font-medium">IP</th>
              <th class="py-2 font-medium">Origem</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="(visit, index) in props.analytics.recentVisits"
              :key="`${visit.receivedAt}-${index}`"
              class="border-t border-[rgb(var(--border))]"
            >
              <td class="py-2 pr-3 text-[rgb(var(--text))]">
                {{ formatDateTime(visit.receivedAt) }}
              </td>
              <td class="py-2 pr-3">
                <UBadge color="neutral" variant="soft" size="sm">{{ visit.deviceType }}</UBadge>
              </td>
              <td class="py-2 pr-3 text-[rgb(var(--muted))]">{{ visit.pagePath || '-' }}</td>
              <td class="py-2 pr-3 text-[rgb(var(--muted))]">{{ visit.ip }}</td>
              <td class="py-2 text-[rgb(var(--muted))]">{{ visit.referrer || '(direto)' }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </section>
</template>
