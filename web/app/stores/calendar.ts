import { computed, ref, watch } from 'vue'
import { defineStore } from 'pinia'

import { useTenantsStore } from '~/stores/tenants'
import { useAuthStore } from '~/stores/auth'
import { createApiRequest } from '~/utils/api-client'
import { useCoreAccountStore } from '../../layers/core/stores/account'
import { useCalendarViewport } from '~/composables/useCalendarViewport'
import { useCalendarDayMedia } from '~/composables/useCalendarDayMedia'
import * as calendarApi from '~/domain/calendar/calendar-api'
import {
  addDaysToKey,
  addMonthsToKey,
  clientColorFor,
  defaultCalendarConfig,
  monthKeyOf,
  resolveClientColor,
  startOfWeekKey,
  type CalendarClient,
  type CalendarConfig,
  type CalendarEvent,
  type CalendarEventInput,
  type CalendarHoliday,
  type CalendarMediaItem,
  type CalendarMember,
  type CalendarPerson,
} from '~/utils/calendar'

const NOTES_PANEL_KEY = 'omni-calendar:notes-panel-open'

function readLocal(key: string): string {
  if (typeof localStorage === 'undefined') return ''
  return localStorage.getItem(key) || ''
}

function writeLocal(key: string, value: string): void {
  if (typeof localStorage === 'undefined') return
  localStorage.setItem(key, value)
}

function lastDayOfMonth(monthKey: string): string {
  return addDaysToKey(`${addMonthsToKey(monthKey, 1)}-01`, -1)
}

export const useCalendarStore = defineStore('calendar', () => {
  const runtimeConfig = useRuntimeConfig()
  const tenantsStore = useTenantsStore()
  const auth = useAuthStore()
  const accountStore = useCoreAccountStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  // Viewport: modo, foco, janela renderizada (scroll infinito), week rail e navegacao.
  // Mora em composables/useCalendarViewport.ts (mantem este arquivo < 450 linhas).
  const viewport = useCalendarViewport()
  const {
    weekStartsOn,
    view,
    currentMonthKey,
    currentWeekKey,
    focusMonthKey,
    focusWeekKey,
    renderedMonthKeys,
    renderedWeekKeys,
    periodTitle,
    weeksOfFocusedMonth,
    railActiveIndex,
    currentRailIndex,
  } = viewport

  const selectedClientId = ref('')
  const selectedDate = ref('')
  const drawerOpen = ref(false)
  const notesPanelOpen = ref(true)
  const notesByMonth = ref<Record<string, string>>({})
  const notesLoaded = ref<Set<string>>(new Set())
  const initialized = ref(false)

  const events = ref<CalendarEvent[]>([])
  const holidays = ref<CalendarHoliday[]>([])
  const responsibles = ref<CalendarPerson[]>([]) // usuarios reais (GET /responsibles)
  const members = ref<CalendarMember[]>([]) // usuarios da conta (config)
  const config = ref<CalendarConfig>(defaultCalendarConfig())

  // --- Clientes (reais, do banco) -----------------------------------------------
  // Cor: override da config (`#rrggbb`/`none`) vence a paleta-semente por indice.
  const clients = computed<CalendarClient[]>(() =>
    (tenantsStore.tenants || [])
      .filter((tenant) => tenant.id && tenant.active)
      .map((tenant, index) => ({
        id: tenant.id,
        name: tenant.name || tenant.slug || 'Cliente',
        color: resolveClientColor(
          config.value.clientColors?.[tenant.id],
          clientColorFor(tenant.id, index),
        ),
      })),
  )

  const clientsById = computed<Map<string, CalendarClient>>(
    () => new Map(clients.value.map((client) => [client.id, client])),
  )

  const people = computed<CalendarPerson[]>(() => responsibles.value)
  const peopleById = computed<Map<string, CalendarPerson>>(
    () => new Map(people.value.map((person) => [person.id, person])),
  )

  // --- Janela buscada -----------------------------------------------------------
  const fetchRange = computed(() => {
    if (view.value === 'month') {
      const monthsList = renderedMonthKeys.value
      if (!monthsList.length) return { from: '', to: '' }
      return { from: `${monthsList[0]}-01`, to: lastDayOfMonth(monthsList[monthsList.length - 1]!) }
    }
    const weeksList = renderedWeekKeys.value
    if (!weeksList.length) return { from: '', to: '' }
    return { from: weeksList[0]!, to: addDaysToKey(weeksList[weeksList.length - 1]!, 6) }
  })

  const visibleEvents = computed(() =>
    selectedClientId.value
      ? events.value.filter((event) => event.clientId === selectedClientId.value)
      : events.value,
  )

  const eventsByDate = computed<Map<string, CalendarEvent[]>>(() => {
    const map = new Map<string, CalendarEvent[]>()
    for (const event of visibleEvents.value) {
      const bucket = map.get(event.date)
      if (bucket) bucket.push(event)
      else map.set(event.date, [event])
    }
    for (const bucket of map.values()) {
      bucket.sort((a, b) => a.time.localeCompare(b.time))
    }
    return map
  })

  const holidaysByDate = computed<Map<string, CalendarHoliday[]>>(() => {
    const map = new Map<string, CalendarHoliday[]>()
    for (const holiday of holidays.value) {
      const bucket = map.get(holiday.date)
      if (bucket) bucket.push(holiday)
      else map.set(holiday.date, [holiday])
    }
    return map
  })

  const selectedEvents = computed(() =>
    selectedDate.value ? eventsByDate.value.get(selectedDate.value) || [] : [],
  )

  const activeNotesMonthKey = computed(() => focusMonthKey.value)
  const activeNotes = computed(() => notesByMonth.value[activeNotesMonthKey.value] || '')

  // --- Fetch da janela (eventos + feriados) + notas -----------------------------
  let windowFetchTimer = 0
  let notesSaveTimer = 0

  // Garante sessao antes de bater na API; engole erro (mantem o estado atual).
  async function withSession(run: () => Promise<void>): Promise<void> {
    await auth.ensureSession()
    if (!auth.isAuthenticated) return
    try {
      await run()
    } catch {
      // silencioso: mantem o ultimo estado bom
    }
  }

  async function fetchEvents(): Promise<void> {
    const { from, to } = fetchRange.value
    if (!from || !to) return
    await withSession(async () => {
      events.value = await calendarApi.fetchEventsInRange(apiRequest, from, to)
    })
  }

  async function fetchHolidays(): Promise<void> {
    const { from, to } = fetchRange.value
    if (!from || !to) return
    await withSession(async () => {
      holidays.value = await calendarApi.fetchHolidaysInRange(apiRequest, from, to)
    })
  }

  // Anexos avulsos por dia (fundo do dia SPEC-F2 + drawer): estado + I/O em
  // composables/useCalendarDayMedia.ts (mantem este arquivo < 450 linhas). Busca
  // na mesma janela dos eventos e alimenta o DayDrawer sem refetch por dia.
  const dayMedia = useCalendarDayMedia(apiRequest, fetchRange, withSession)
  const { dayMediaByDate, fetchDayMedia, saveDayMedia } = dayMedia
  const selectedDayMedia = computed<CalendarMediaItem[]>(() => dayMedia.mediaForDate(selectedDate))

  function scheduleWindowFetch(): void {
    if (windowFetchTimer) window.clearTimeout(windowFetchTimer)
    windowFetchTimer = window.setTimeout(() => {
      void fetchEvents()
      void fetchHolidays()
      void fetchDayMedia()
    }, 250)
  }

  async function fetchNotes(month: string): Promise<void> {
    if (!month || notesLoaded.value.has(month)) return
    await withSession(async () => {
      const content = await calendarApi.fetchNotesForMonth(apiRequest, month)
      notesByMonth.value = { ...notesByMonth.value, [month]: content }
      notesLoaded.value = new Set(notesLoaded.value).add(month)
    })
  }

  function setNotesForActiveMonth(html: string): void {
    const month = activeNotesMonthKey.value
    notesByMonth.value = { ...notesByMonth.value, [month]: html }
    notesLoaded.value = new Set(notesLoaded.value).add(month)
    if (notesSaveTimer) window.clearTimeout(notesSaveTimer)
    notesSaveTimer = window.setTimeout(() => {
      void calendarApi.putNotesForMonth(apiRequest, month, html).catch(() => {})
    }, 800)
  }

  // --- Responsaveis / config / membros ------------------------------------------
  async function fetchResponsibles(): Promise<void> {
    await withSession(async () => {
      responsibles.value = await calendarApi.fetchResponsibles(apiRequest)
    })
  }

  async function fetchConfig(): Promise<void> {
    await withSession(async () => {
      config.value = await calendarApi.fetchConfig(apiRequest)
    })
  }

  async function fetchMembers(): Promise<void> {
    await withSession(async () => {
      members.value = await calendarApi.fetchMembers(apiRequest)
    })
  }

  async function saveConfig(next: CalendarConfig): Promise<boolean> {
    try {
      config.value = await calendarApi.putConfig(apiRequest, next)
      await fetchResponsibles()
      await fetchHolidays()
      return true
    } catch {
      return false
    }
  }

  // A config manda no inicio da semana: espelha `config.weekStartsOn` no viewport
  // (fonte unica = banco; o default sunday so vale ate a config chegar).
  watch(
    () => config.value.weekStartsOn,
    (v) => (weekStartsOn.value = v === 'monday' ? 'monday' : 'sunday'),
    { immediate: true },
  )

  watch(fetchRange, scheduleWindowFetch)
  watch(activeNotesMonthKey, (month) => void fetchNotes(month))
  watch(
    () => accountStore.activeAccountId,
    () => {
      events.value = []
      holidays.value = []
      dayMedia.reset()
      notesByMonth.value = {}
      notesLoaded.value = new Set()
      void fetchEvents()
      void fetchHolidays()
      void fetchDayMedia()
      void fetchNotes(activeNotesMonthKey.value)
      void fetchResponsibles()
      void fetchConfig()
    },
  )

  // --- CRUD de eventos ----------------------------------------------------------
  async function createEvent(input: CalendarEventInput): Promise<boolean> {
    try {
      await calendarApi.postEvent(apiRequest, input)
      await fetchEvents()
      return true
    } catch {
      return false
    }
  }

  async function updateEvent(id: string, input: CalendarEventInput): Promise<boolean> {
    try {
      await calendarApi.putEvent(apiRequest, id, input)
      await fetchEvents()
      return true
    } catch {
      return false
    }
  }

  async function deleteEvent(id: string): Promise<boolean> {
    try {
      await calendarApi.deleteEvent(apiRequest, id)
      await fetchEvents()
      return true
    } catch {
      return false
    }
  }

  // Scroll infinito + foco/navegacao: em composables/useCalendarViewport.ts.
  const {
    prependMonths,
    appendMonths,
    prependWeeks,
    appendWeeks,
    ensureMonthRendered,
    ensureWeekRendered,
    setFocusMonth,
    setFocusWeek,
    setView,
    goToday,
    goPrev,
    goNext,
    selectWeek,
  } = viewport

  function setClientFilter(clientId: string): void {
    selectedClientId.value = clientId || ''
  }

  // Seleciona o dia (abre drawer) e leva o foco/janela ate ele via viewport.
  function selectDay(dateKey: string): void {
    selectedDate.value = dateKey
    drawerOpen.value = true
    if (view.value === 'week') {
      setFocusWeek(startOfWeekKey(dateKey, weekStartsOn.value))
      ensureWeekRendered(focusWeekKey.value)
    } else {
      const monthKey = monthKeyOf(dateKey)
      setFocusMonth(monthKey)
      ensureMonthRendered(monthKey)
    }
  }

  function closeDrawer(): void {
    drawerOpen.value = false
    selectedDate.value = ''
  }

  function toggleNotesPanel(): void {
    notesPanelOpen.value = !notesPanelOpen.value
    writeLocal(NOTES_PANEL_KEY, notesPanelOpen.value ? '1' : '0')
  }

  async function loadClients(): Promise<void> {
    try {
      await tenantsStore.ensureLoaded()
    } catch {
      // sem sessao/permissao: clientes vazios
    }
  }

  function init(): void {
    if (initialized.value) return
    initialized.value = true

    const storedPanel = readLocal(NOTES_PANEL_KEY)
    if (storedPanel) notesPanelOpen.value = storedPanel === '1'

    void loadClients()
    void fetchResponsibles()
    void fetchConfig()
    void fetchNotes(activeNotesMonthKey.value)
    void fetchEvents()
    void fetchHolidays()
    void fetchDayMedia()
  }

  return {
    // estado
    view,
    weekStartsOn,
    selectedClientId,
    selectedDate,
    drawerOpen,
    notesPanelOpen,
    config,
    members,
    // derivados
    clients,
    clientsById,
    people,
    peopleById,
    focusMonthKey,
    focusWeekKey,
    currentMonthKey,
    currentWeekKey,
    renderedMonthKeys,
    renderedWeekKeys,
    weeksOfFocusedMonth,
    railActiveIndex,
    currentRailIndex,
    eventsByDate,
    holidaysByDate,
    dayMediaByDate,
    selectedEvents,
    selectedDayMedia,
    periodTitle,
    activeNotesMonthKey,
    activeNotes,
    // acoes
    init,
    setView,
    setClientFilter,
    goToday,
    goPrev,
    goNext,
    selectWeek,
    selectDay,
    closeDrawer,
    toggleNotesPanel,
    setNotesForActiveMonth,
    setFocusMonth,
    setFocusWeek,
    prependMonths,
    appendMonths,
    prependWeeks,
    appendWeeks,
    createEvent,
    updateEvent,
    deleteEvent,
    fetchConfig,
    fetchMembers,
    saveConfig,
    saveDayMedia,
  }
})
