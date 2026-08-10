import type { Ref } from 'vue'
import { storeToRefs } from 'pinia'
import { usePlanningPersistenceStore } from '~/stores/planning-persistence'
import { usePlanningStore } from '~/stores/planning'
import { useUiStore } from '~/stores/ui'
import { weekStartForGoalPeriod } from '~/domain/planning/scheduler'
import { goalWeekPeriods } from '~/utils/goal-periods'

function offsetISODate(value: string, days: number): string {
  const date = new Date(`${value}T00:00:00Z`)
  date.setUTCDate(date.getUTCDate() + days)
  return date.toISOString().slice(0, 10)
}

export function usePlanningWeekCopy(
  readonly: Readonly<Ref<boolean>>,
  pending: Ref<boolean>,
  persistSchedule: () => Promise<boolean>,
) {
  const planning = usePlanningStore()
  const persistence = usePlanningPersistenceStore()
  const ui = useUiStore()
  const { activeStoreId, activeStaff, activeShifts, selectedMonth, weekStart } =
    storeToRefs(planning)

  async function copyPreviousWeek() {
    const storeId = activeStoreId.value
    if (!storeId || readonly.value) return
    pending.value = true
    try {
      const source = (await persistence.load(storeId, offsetISODate(weekStart.value, -7))).schedule
      if (!source?.shifts.length) {
        ui.error('A semana anterior não possui uma escala salva.', 'Nada para copiar')
        return
      }
      const activeIDs = new Set(activeStaff.value.map((member) => member.id))
      planning.replaceActiveShifts(
        source.shifts
          .filter((shift) => activeIDs.has(shift.staffId))
          .map((shift) => ({ ...shift, isoDate: offsetISODate(shift.isoDate, 7) })),
      )
      if (await persistSchedule()) {
        ui.success('Semana anterior copiada e salva no banco.', 'Escala copiada')
      }
    } catch {
      ui.error('Não foi possível copiar a semana anterior.', 'Falha ao copiar')
    } finally {
      pending.value = false
    }
  }

  async function clearWeek() {
    if (readonly.value) return
    const { confirmed } = (await ui.confirm({
      title: 'Limpar esta semana?',
      message: 'Todos os turnos desta semana serão removidos e a alteração será salva no banco.',
      confirmLabel: 'Limpar semana',
    })) as { confirmed: boolean }
    if (!confirmed) return
    planning.replaceActiveShifts([])
    if (await persistSchedule()) ui.success('Semana limpa e salva no banco.', 'Escala limpa')
  }

  async function replicateMonthWeeks() {
    const storeId = activeStoreId.value
    if (!storeId || readonly.value || !activeShifts.value.length) return
    const { confirmed } = (await ui.confirm({
      title: 'Replicar para todas as semanas do mês?',
      message: 'O padrão atual substituirá as semanas ainda não publicadas deste mês.',
      confirmLabel: 'Replicar escala',
    })) as { confirmed: boolean }
    if (!confirmed) return
    pending.value = true
    try {
      const periods = goalWeekPeriods(selectedMonth.value).map((period, index) => ({
        period,
        goalWeek: index + 1,
        targetWeekStart: weekStartForGoalPeriod(selectedMonth.value, period),
      }))
      const snapshots = await Promise.all(
        periods.map((item) => persistence.load(storeId, item.targetWeekStart)),
      )
      if (snapshots.some((snapshot) => snapshot.schedule?.status === 'published')) {
        ui.error('Reabra as semanas publicadas antes de replicar o mês.', 'Replicação bloqueada')
        return
      }
      const sourceStart = new Date(`${weekStart.value}T00:00:00Z`).getTime()
      for (let index = 0; index < periods.length; index += 1) {
        const item = periods[index]!
        if (item.targetWeekStart === weekStart.value) continue
        const targetStart = new Date(`${item.targetWeekStart}T00:00:00Z`).getTime()
        const offsetDays = Math.round((targetStart - sourceStart) / 86400000)
        await persistence.saveSchedule(
          storeId,
          item.targetWeekStart,
          activeShifts.value.map((shift) => ({
            ...shift,
            isoDate: offsetISODate(shift.isoDate, offsetDays),
          })),
          selectedMonth.value,
          item.goalWeek,
          'saved',
          snapshots[index]?.schedule?.version,
        )
      }
      ui.success(`Escala replicada nas ${periods.length} semanas disponíveis.`, 'Mês atualizado')
    } catch {
      ui.error('Não foi possível replicar todas as semanas.', 'Falha ao replicar')
    } finally {
      pending.value = false
    }
  }

  return { copyPreviousWeek, clearWeek, replicateMonthWeeks }
}
