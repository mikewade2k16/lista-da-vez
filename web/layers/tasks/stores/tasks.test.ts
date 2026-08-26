import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import { useTasksStore } from './tasks'
import { useCoreAccountStore } from '../../core/stores/account'

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

  it('ignores a board archived between list and detail during refresh', async () => {
    useCoreAccountStore().activeAccountId = 'account-1'
    const fetchMock = globalThis.$fetch as ReturnType<typeof vi.fn>
    fetchMock.mockImplementation((path: string) => {
      if (path === '/v1/tasks/preferences') {
        return Promise.resolve({ preferences: { lastBoardId: 'board-archived' } })
      }
      if (path === '/v1/tasks/boards') {
        return Promise.resolve({
          boards: [
            { id: 'board-archived', name: 'Campanha' },
            { id: 'board-active', name: 'Tasks' },
          ],
        })
      }
      if (path === '/v1/task-boards/board-archived') {
        return Promise.reject(
          Object.assign(new Error('Recurso nao encontrado.'), { statusCode: 404 }),
        )
      }
      if (path === '/v1/task-boards/board-active') {
        return Promise.resolve({
          board: {
            id: 'board-active',
            name: 'Tasks',
            columns: [{ id: 'column-1', label: 'Raw', sortOrder: 100 }],
            fields: [{ id: 'field-1', key: 'status', label: 'Status', type: 'status' }],
            views: [{ id: 'view-1', name: 'Board', type: 'board', config: {} }],
          },
        })
      }
      if (path.startsWith('/v1/tasks/boards/board-active/tasks?')) {
        return Promise.resolve({ tasks: [], nextCursor: '' })
      }
      return Promise.reject(new Error(`Request inesperada: ${path}`))
    })

    const store = useTasksStore()
    await expect(store.refresh()).resolves.toBeUndefined()

    expect(store.projects.map((project) => project.id)).toEqual(['board-active'])
    expect(store.activeProjectId).toBe('board-active')
    expect(store.errorMessage).toBe('')
  })
})
