import { ref, type ComputedRef, type Ref } from 'vue'

import * as calendarApi from '~/domain/calendar/calendar-api'
import type { ApiRequest } from '~/domain/calendar/calendar-api'
import type { CalendarMediaItem } from '~/utils/calendar'

// useCalendarDayMedia concentra os ANEXOS AVULSOS por dia (fundo do dia da SPEC-F2
// + drawer): estado `dayMediaByDate` (Map por data), busca na janela e o PUT por
// dia. Fica fora do store (stores/calendar.ts) para manter cada arquivo < 450
// linhas; o store injeta o apiRequest, a janela (`fetchRange`) e o wrapper de
// sessao, e re-exporta o que os componentes consomem. Sem refetch por dia: o Map
// alimenta tanto o fundo das celulas quanto o DayDrawer (fonte unica).
export function useCalendarDayMedia(
  apiRequest: ApiRequest,
  fetchRange: ComputedRef<{ from: string; to: string }>,
  withSession: (run: () => Promise<void>) => Promise<void>,
) {
  const dayMediaByDate = ref<Map<string, CalendarMediaItem[]>>(new Map())

  async function fetchDayMedia(): Promise<void> {
    const { from, to } = fetchRange.value
    if (!from || !to) return
    await withSession(async () => {
      const days = await calendarApi.fetchDayMediaInRange(apiRequest, from, to)
      const map = new Map<string, CalendarMediaItem[]>()
      for (const entry of days) map.set(entry.date, entry.media)
      dayMediaByDate.value = map
    })
  }

  // Salva os anexos avulsos de um dia (PUT) e atualiza o Map local (fonte unica).
  async function saveDayMedia(date: string, media: CalendarMediaItem[]): Promise<boolean> {
    if (!date) return false
    try {
      await calendarApi.putDayMedia(apiRequest, date, media)
      const next = new Map(dayMediaByDate.value)
      if (media.length) next.set(date, media)
      else next.delete(date)
      dayMediaByDate.value = next
      return true
    } catch {
      return false
    }
  }

  function mediaForDate(date: Ref<string>): CalendarMediaItem[] {
    return date.value ? dayMediaByDate.value.get(date.value) || [] : []
  }

  function reset(): void {
    dayMediaByDate.value = new Map()
  }

  return { dayMediaByDate, fetchDayMedia, saveDayMedia, mediaForDate, reset }
}
