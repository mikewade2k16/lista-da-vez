import type { TasksRealtimeEvent } from '../composables/useTasksRealtime'

export function isTaskUpdatedEventAlreadyApplied(event: TasksRealtimeEvent, localVersion: unknown) {
  if (event.type !== 'task.updated') return false
  const incomingVersion = Number(event.version || 0)
  return incomingVersion > 0 && Number(localVersion || 0) >= incomingVersion
}
