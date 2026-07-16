<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useAuthStore } from '~/stores/auth'
import { useCalendarStore } from '~/stores/calendar'
import { createApiRequest } from '~/utils/api-client'
import * as calendarApi from '~/domain/calendar/calendar-api'
import {
  addMonthsToKey,
  eventTypeMeta,
  monthKeyOf,
  todayKey,
  type CalendarEvent,
} from '~/utils/calendar'

// SEARCH DO CALENDARIO (WAVE 14, pedido do dono): uma lupa que abre um campo de busca; digitar
// filtra os itens do CALENDARIO por titulo/cliente; clicar num resultado navega ao dia (foca o
// mes) e ABRE o modal do evento. Busca numa janela ampla ao redor do mes em foco (nao so a janela
// renderizada) para achar itens de outros meses. Fecha no clique-fora/Esc (regra do design system).
const emit = defineEmits<{ open: [event: CalendarEvent] }>()

const store = useCalendarStore()
const auth = useAuthStore()
const runtimeConfig = useRuntimeConfig()
const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

const open = ref(false)
const query = ref('')
const loading = ref(false)
const rootRef = ref<HTMLElement | null>(null)
const btnRef = ref<HTMLElement | null>(null)
const inputRef = ref<HTMLInputElement | null>(null)
// O painel usa position:fixed (escapa do overflow da coluna estreita dos controles). A posicao
// e calculada a partir do retangulo da lupa: abaixo dela, alinhado a esquerda mas sem sair da tela.
const panelStyle = ref<Record<string, string>>({})

function positionPanel(): void {
  const el = btnRef.value
  if (!el) return
  const r = el.getBoundingClientRect()
  const width = Math.min(352, window.innerWidth - 24)
  let left = r.left
  if (left + width > window.innerWidth - 12) left = window.innerWidth - 12 - width
  if (left < 12) left = 12
  panelStyle.value = {
    top: `${Math.round(r.bottom + 6)}px`,
    left: `${Math.round(left)}px`,
    width: `${width}px`,
  }
}
// Cache dos eventos da janela ampla (buscados 1x por abertura; a janela cobre -6/+12 meses).
const pool = ref<CalendarEvent[]>([])
let loadedForMonth = ''

const clientsById = computed(() => store.clientsById)

async function loadPool(): Promise<void> {
  const focus = store.focusMonthKey || monthKeyOf(todayKey())
  if (loadedForMonth === focus && pool.value.length) return
  loading.value = true
  try {
    const from = `${addMonthsToKey(focus, -6)}-01`
    // fim = ultimo dia de +12 meses (usa dia 28 como piso seguro + o mes seguinte -1 seria ideal,
    // mas a query aceita qualquer 'to'; usamos o 1o dia de +13 como limite exclusivo-1).
    const to = `${addMonthsToKey(focus, 12)}-28`
    pool.value = await calendarApi.fetchEventsInRange(apiRequest, from, to)
    loadedForMonth = focus
  } catch {
    // silencioso: mantem o pool anterior
  } finally {
    loading.value = false
  }
}

function normalize(value: string): string {
  return String(value || '')
    .trim()
    .toLowerCase()
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
}

const results = computed<CalendarEvent[]>(() => {
  const needle = normalize(query.value)
  if (needle.length < 2) return []
  const scored = pool.value.filter((ev) => {
    const title = normalize(ev.title)
    const client = normalize(clientsById.value.get(ev.clientId)?.name || '')
    return title.includes(needle) || client.includes(needle)
  })
  // Mais recentes primeiro; teto de 20 para nao virar uma lista gigante.
  return scored.sort((a, b) => (a.date < b.date ? 1 : a.date > b.date ? -1 : 0)).slice(0, 20)
})

function toggle(): void {
  open.value = !open.value
  if (open.value) {
    positionPanel()
    void loadPool()
    void nextTick(() => {
      positionPanel()
      inputRef.value?.focus()
    })
  }
}

function close(): void {
  open.value = false
  query.value = ''
}

function clientName(id: string): string {
  return clientsById.value.get(id)?.name || ''
}

function dateLabel(value: string): string {
  const [y, m, d] = value.split('-').map(Number)
  if (!y || !m || !d) return value
  return new Intl.DateTimeFormat('pt-BR', { day: '2-digit', month: 'short' }).format(
    new Date(y, m - 1, d),
  )
}

function pick(ev: CalendarEvent): void {
  emit('open', ev)
  close()
}

// Fecha no clique-fora e no Esc (regra de popover do design system).
function onDocClick(event: MouseEvent): void {
  if (!open.value) return
  if (rootRef.value && !rootRef.value.contains(event.target as Node)) close()
}
function onKeydown(event: KeyboardEvent): void {
  if (event.key === 'Escape' && open.value) close()
}
function onReflow(): void {
  if (open.value) positionPanel()
}
watch(open, (isOpen) => {
  if (isOpen) {
    document.addEventListener('mousedown', onDocClick)
    document.addEventListener('keydown', onKeydown)
    window.addEventListener('resize', onReflow)
    window.addEventListener('scroll', onReflow, true)
  } else {
    document.removeEventListener('mousedown', onDocClick)
    document.removeEventListener('keydown', onKeydown)
    window.removeEventListener('resize', onReflow)
    window.removeEventListener('scroll', onReflow, true)
  }
})
onBeforeUnmount(() => {
  document.removeEventListener('mousedown', onDocClick)
  document.removeEventListener('keydown', onKeydown)
  window.removeEventListener('resize', onReflow)
  window.removeEventListener('scroll', onReflow, true)
})
</script>

<template>
  <div ref="rootRef" class="calendar-search">
    <button
      ref="btnRef"
      type="button"
      class="calendar-controls__gear"
      :class="{ 'is-active': open }"
      aria-label="Buscar no calendário"
      title="Buscar item no calendário"
      @click="toggle"
    >
      <UIcon name="i-lucide-search" aria-hidden="true" />
    </button>

    <div
      v-if="open"
      class="calendar-search__panel"
      role="dialog"
      aria-label="Buscar no calendário"
      :style="panelStyle"
    >
      <div class="calendar-search__field">
        <UIcon name="i-lucide-search" class="calendar-search__field-icon" aria-hidden="true" />
        <input
          ref="inputRef"
          v-model="query"
          type="text"
          class="calendar-search__input"
          placeholder="Buscar item do calendário…"
          aria-label="Termo de busca"
        />
        <button
          v-if="query"
          type="button"
          class="calendar-search__clear"
          aria-label="Limpar"
          @click="query = ''"
        >
          <UIcon name="i-lucide-x" aria-hidden="true" />
        </button>
      </div>

      <div class="calendar-search__results">
        <p v-if="loading" class="calendar-search__hint">Carregando itens…</p>
        <p v-else-if="query.length < 2" class="calendar-search__hint">
          Digite ao menos 2 letras para buscar.
        </p>
        <p v-else-if="!results.length" class="calendar-search__hint">
          Nenhum item do calendário encontrado.
        </p>
        <ul v-else class="calendar-search__list">
          <li v-for="ev in results" :key="ev.id">
            <button type="button" class="calendar-search__item" @click="pick(ev)">
              <UIcon
                :name="eventTypeMeta(ev.type).icon"
                class="calendar-search__item-icon"
                aria-hidden="true"
              />
              <span class="calendar-search__item-body">
                <span class="calendar-search__item-title">{{ ev.title }}</span>
                <span class="calendar-search__item-meta">
                  {{ dateLabel(ev.date) }}
                  <template v-if="clientName(ev.clientId)">
                    · {{ clientName(ev.clientId) }}
                  </template>
                </span>
              </span>
              <UIcon
                name="i-lucide-arrow-right"
                class="calendar-search__item-go"
                aria-hidden="true"
              />
            </button>
          </li>
        </ul>
      </div>
    </div>
  </div>
</template>

<style scoped>
.calendar-search {
  position: relative;
  display: inline-flex;
}

/* position:fixed + coordenadas calculadas: o painel escapa do overflow da coluna estreita dos
   controles (senao ficaria cortado). top/left/width vem do panelStyle (rect da lupa). */
.calendar-search__panel {
  position: fixed;
  z-index: 80;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-md);
  background: rgb(var(--surface));
  box-shadow: var(--shadow-lg);
  overflow: hidden;
}

.calendar-search__field {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.5rem 0.6rem;
  border-bottom: 1px solid rgb(var(--border));
}

.calendar-search__field-icon {
  color: rgb(var(--muted));
  flex: 0 0 auto;
}

.calendar-search__input {
  flex: 1;
  min-width: 0;
  border: none;
  background: transparent;
  color: rgb(var(--text));
  font-size: 0.88rem;
  outline: none;
}

.calendar-search__clear {
  flex: 0 0 auto;
  color: rgb(var(--muted));
  padding: 0.1rem;
  border-radius: 999px;
}

.calendar-search__clear:hover {
  color: rgb(var(--text));
  background: rgb(var(--surface-2));
}

.calendar-search__results {
  max-height: 60vh;
  overflow-y: auto;
}

.calendar-search__hint {
  padding: 0.75rem 0.7rem;
  color: rgb(var(--muted));
  font-size: 0.82rem;
}

.calendar-search__list {
  list-style: none;
  margin: 0;
  padding: 0.25rem;
}

.calendar-search__item {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 0.55rem;
  padding: 0.5rem 0.55rem;
  border-radius: var(--radius-sm);
  background: transparent;
  color: rgb(var(--text));
  text-align: left;
}

.calendar-search__item:hover {
  background: rgb(var(--surface-2));
}

.calendar-search__item-icon {
  flex: 0 0 auto;
  color: rgb(var(--muted));
}

.calendar-search__item-body {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}

.calendar-search__item-title {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.86rem;
  font-weight: 600;
}

.calendar-search__item-meta {
  color: rgb(var(--muted));
  font-size: 0.74rem;
}

.calendar-search__item-go {
  flex: 0 0 auto;
  color: rgb(var(--muted));
  opacity: 0;
  transition: opacity 0.12s ease;
}

.calendar-search__item:hover .calendar-search__item-go {
  opacity: 1;
}
</style>
