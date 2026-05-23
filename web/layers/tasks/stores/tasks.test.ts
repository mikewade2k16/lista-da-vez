import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import { useTasksStore } from './tasks'

describe('useTasksStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('persists the selected active project in UI metadata', () => {
    const store = useTasksStore()

    store.projects = [{ id: 'project-1' }] as any
    store.setActiveProject('project-1')

    expect(store.activeProjectId).toBe('project-1')
    expect(JSON.parse(localStorage.getItem('omni.tasks.api.workspace.ui.v1') || '{}')).toEqual(
      expect.objectContaining({
        activeProjectId: 'project-1',
      }),
    )
  })
})