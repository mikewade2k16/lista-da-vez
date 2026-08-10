<script setup lang="ts">
import { computed } from 'vue'
import {
  AlertCircle,
  CalendarRange,
  CheckCircle2,
  Copy,
  Eraser,
  History,
  RefreshCw,
  RotateCcw,
  Sparkles,
  UploadCloud,
} from 'lucide-vue-next'

import type { PlanningScheduleRevision, ScheduleStatus } from '~/domain/planning/types'

const props = defineProps<{
  status: ScheduleStatus
  canEdit: boolean
  history: PlanningScheduleRevision[]
  saveError: string
  lastSavedAt: string
}>()

const emit = defineEmits<{
  publish: []
  reopen: []
  copyPrevious: []
  replicateMonth: []
  clearWeek: []
  applyDefault: []
  retrySave: []
}>()

const statusLabels: Record<ScheduleStatus, string> = {
  loading: 'Carregando',
  unsaved: 'Alterações não salvas',
  saving: 'Salvando',
  saved: 'Salva no banco',
  published: 'Publicada',
}
const savedTime = computed(() => {
  const date = new Date(props.lastSavedAt)
  return props.lastSavedAt && !Number.isNaN(date.getTime())
    ? new Intl.DateTimeFormat('pt-BR', { hour: '2-digit', minute: '2-digit' }).format(date)
    : ''
})
const statusLabel = computed(() => {
  if (props.saveError) return 'Erro ao salvar'
  if (props.status === 'saved' && savedTime.value) return `Salva às ${savedTime.value}`
  return statusLabels[props.status]
})
const disabled = computed(
  () =>
    !props.canEdit ||
    props.status === 'published' ||
    props.status === 'loading' ||
    props.status === 'saving',
)
const dateTime = new Intl.DateTimeFormat('pt-BR', { dateStyle: 'short', timeStyle: 'short' })
</script>

<template>
  <div class="planning-schedule-actions" aria-label="Ações da escala">
    <button
      v-if="status === 'published'"
      type="button"
      class="planning-schedule-actions__icon"
      title="Reabrir escala publicada"
      aria-label="Reabrir escala publicada"
      :disabled="!canEdit"
      @click="emit('reopen')"
    >
      <RotateCcw :size="14" />
    </button>
    <button
      v-else
      type="button"
      class="planning-schedule-actions__icon is-success"
      title="Publicar escala"
      aria-label="Publicar escala"
      :disabled="disabled"
      @click="emit('publish')"
    >
      <UploadCloud :size="14" />
    </button>
    <span
      class="planning-schedule-actions__icon is-status"
      :class="`is-${saveError ? 'error' : status}`"
      :title="`Status: ${statusLabel}`"
      :aria-label="`Status: ${statusLabel}`"
    >
      <AlertCircle v-if="saveError" :size="14" />
      <CheckCircle2 v-else :size="14" />
    </span>
    <button
      v-if="saveError"
      type="button"
      class="planning-schedule-actions__icon"
      title="Tentar salvar novamente"
      aria-label="Tentar salvar novamente"
      @click="emit('retrySave')"
    >
      <RefreshCw :size="14" />
    </button>
    <button
      type="button"
      class="planning-schedule-actions__icon"
      title="Copiar semana anterior"
      aria-label="Copiar semana anterior"
      :disabled="disabled"
      @click="emit('copyPrevious')"
    >
      <Copy :size="14" />
    </button>
    <button
      type="button"
      class="planning-schedule-actions__icon"
      title="Replicar para todas as semanas do mês"
      aria-label="Replicar para todas as semanas do mês"
      :disabled="disabled"
      @click="emit('replicateMonth')"
    >
      <CalendarRange :size="14" />
    </button>
    <button
      type="button"
      class="planning-schedule-actions__icon"
      title="Aplicar modelo padrão"
      aria-label="Aplicar modelo padrão"
      :disabled="disabled"
      @click="emit('applyDefault')"
    >
      <Sparkles :size="14" />
    </button>
    <button
      type="button"
      class="planning-schedule-actions__icon is-danger"
      title="Limpar semana"
      aria-label="Limpar semana"
      :disabled="disabled"
      @click="emit('clearWeek')"
    >
      <Eraser :size="14" />
    </button>
    <details class="planning-schedule-actions__history">
      <summary title="Ver histórico" aria-label="Ver histórico"><History :size="14" /></summary>
      <ul v-if="history.length">
        <li v-for="revision in history" :key="revision.version">
          <strong>
            v{{ revision.version }} ·
            {{ revision.status === 'published' ? 'Publicada' : 'Alterada' }}
          </strong>
          <small>
            {{ revision.changedByName }} · {{ dateTime.format(new Date(revision.createdAt)) }}
          </small>
        </li>
      </ul>
      <small v-else>Nenhuma alteração registrada.</small>
    </details>
  </div>
</template>

<style scoped>
.planning-schedule-actions {
  display: flex;
  align-items: center;
  gap: 0.25rem;
  flex: 0 0 auto;
}
.planning-schedule-actions__icon,
.planning-schedule-actions__history summary {
  display: grid;
  place-items: center;
  width: 2rem;
  height: 2rem;
  border: 1px solid rgb(var(--border) / 0.65);
  border-radius: 0.5rem;
  background: rgb(var(--surface-2) / 0.68);
  color: var(--text-muted);
  cursor: pointer;
}
.planning-schedule-actions__icon:hover,
.planning-schedule-actions__history summary:hover {
  border-color: rgb(var(--primary) / 0.5);
  color: rgb(var(--primary));
}
.planning-schedule-actions__icon:disabled {
  opacity: 0.38;
  cursor: not-allowed;
}
.planning-schedule-actions__icon.is-success,
.planning-schedule-actions__icon.is-saved,
.planning-schedule-actions__icon.is-published {
  color: rgb(var(--success));
}
.planning-schedule-actions__icon.is-danger,
.planning-schedule-actions__icon.is-error {
  color: rgb(var(--danger));
}
.planning-schedule-actions__icon.is-status {
  cursor: help;
}
.planning-schedule-actions__history {
  position: relative;
}
.planning-schedule-actions__history summary {
  list-style: none;
}
.planning-schedule-actions__history summary::-webkit-details-marker {
  display: none;
}
.planning-schedule-actions__history > ul,
.planning-schedule-actions__history > small {
  position: absolute;
  z-index: 30;
  top: calc(100% + 0.35rem);
  right: 0;
  width: 18rem;
  margin: 0;
  border: 1px solid rgb(var(--border) / 0.72);
  border-radius: 0.65rem;
  padding: 0.55rem;
  background: rgb(var(--surface));
  box-shadow: var(--shadow-card);
  list-style: none;
}
.planning-schedule-actions__history li {
  display: grid;
  gap: 0.1rem;
  padding: 0.35rem;
}
.planning-schedule-actions__history small {
  color: var(--text-muted);
  font-size: 0.64rem;
}
</style>
