import { ref, type ComputedRef } from 'vue'

import type { PlanningShift, ShiftTemplateId } from '~/domain/planning/types'

type DragPayload =
  | { kind: 'staff'; staffId: string }
  | { kind: 'shift'; staffId: string; isoDate: string }

interface PlanningScheduleDragOptions {
  readonly: () => boolean
  loadedDates: ComputedRef<Set<string>>
  assign: (staffId: string, isoDate: string) => void
  move: (staffId: string, fromDate: string, toDate: string) => void
  place: (staffId: string, fromDate: string, toDate: string, templateId: ShiftTemplateId) => void
  remove: (staffId: string, isoDate: string) => void
}

export function usePlanningScheduleDrag(options: PlanningScheduleDragOptions) {
  const draggedStaffId = ref('')
  const draggedShift = ref<PlanningShift | null>(null)
  const dragTarget = ref('')

  function writeDragPayload(payload: DragPayload, event: DragEvent): void {
    if (!event.dataTransfer) return
    event.dataTransfer.effectAllowed = payload.kind === 'staff' ? 'copy' : 'move'
    event.dataTransfer.setData('application/x-omni-planning', JSON.stringify(payload))
    event.dataTransfer.setData('text/plain', payload.staffId)
  }

  function readDragPayload(event: DragEvent): DragPayload | null {
    if (draggedShift.value) {
      return {
        kind: 'shift',
        staffId: draggedShift.value.staffId,
        isoDate: draggedShift.value.isoDate,
      }
    }
    if (draggedStaffId.value) return { kind: 'staff', staffId: draggedStaffId.value }
    const raw = event.dataTransfer?.getData('application/x-omni-planning')
    if (!raw) return null
    try {
      const parsed = JSON.parse(raw) as Partial<DragPayload>
      if (
        parsed.kind === 'shift' &&
        typeof parsed.staffId === 'string' &&
        typeof parsed.isoDate === 'string'
      ) {
        return { kind: 'shift', staffId: parsed.staffId, isoDate: parsed.isoDate }
      }
      if (parsed.kind === 'staff' && typeof parsed.staffId === 'string') {
        return { kind: 'staff', staffId: parsed.staffId }
      }
    } catch {
      return null
    }
    return null
  }

  function startStaffDrag(staffId: string, event: DragEvent): void {
    if (options.readonly()) return
    draggedStaffId.value = staffId
    writeDragPayload({ kind: 'staff', staffId }, event)
  }

  function startShiftDrag(shift: PlanningShift, event: DragEvent): void {
    if (options.readonly()) return
    draggedShift.value = shift
    writeDragPayload({ kind: 'shift', staffId: shift.staffId, isoDate: shift.isoDate }, event)
  }

  function stopDrag(): void {
    draggedStaffId.value = ''
    draggedShift.value = null
    dragTarget.value = ''
  }

  function dropOnDay(isoDate: string, event: DragEvent): void {
    if (options.readonly() || !options.loadedDates.value.has(isoDate)) return
    const payload = readDragPayload(event)
    if (payload?.kind === 'shift') options.move(payload.staffId, payload.isoDate, isoDate)
    else if (payload?.kind === 'staff') options.assign(payload.staffId, isoDate)
    stopDrag()
  }

  function dropOnLane(isoDate: string, templateId: ShiftTemplateId, event: DragEvent): void {
    event.preventDefault()
    event.stopPropagation()
    if (options.readonly() || !options.loadedDates.value.has(isoDate)) {
      stopDrag()
      return
    }
    const payload = readDragPayload(event)
    if (payload?.kind === 'shift') {
      options.place(payload.staffId, payload.isoDate, isoDate, templateId)
    } else if (payload?.kind === 'staff') {
      options.place(payload.staffId, '', isoDate, templateId)
    }
    stopDrag()
  }

  function dragOverLane(isoDate: string, templateId: ShiftTemplateId, event: DragEvent): void {
    event.preventDefault()
    event.stopPropagation()
    if (options.readonly() || !options.loadedDates.value.has(isoDate)) return
    dragTarget.value = `${isoDate}:${templateId}`
    if (event.dataTransfer) event.dataTransfer.dropEffect = draggedShift.value ? 'move' : 'copy'
  }

  function dropToRemove(event: DragEvent): void {
    if (options.readonly()) return
    const payload = readDragPayload(event)
    if (payload?.kind === 'shift') options.remove(payload.staffId, payload.isoDate)
    stopDrag()
  }

  return {
    draggedShift,
    dragTarget,
    startStaffDrag,
    startShiftDrag,
    stopDrag,
    dropOnDay,
    dropOnLane,
    dragOverLane,
    dropToRemove,
  }
}
