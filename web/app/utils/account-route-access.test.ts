import { describe, expect, it } from 'vitest'

import queueNav from '../../layers/queue/nav.config'
import type { NavItem } from '~/stores/nav'
import {
  AGENCY_ONLY_PATHS,
  EDITOR_MODULE_PATH_GUARD,
  findEditorModulePathGuard,
  isAgencyOnlyPath,
} from './account-route-access'

function findNavItemByPath(path: string): NavItem | undefined {
  const visit = (items: NavItem[]): NavItem | undefined => {
    for (const item of items) {
      if (item.path === path) return item
      const child = visit(item.children || [])
      if (child) return child
    }
    return undefined
  }

  return visit(queueNav.sections.flatMap((section) => section.items) as NavItem[])
}

describe('account route access contract', () => {
  it('gates Editor by the tools module in navigation and direct routes', () => {
    expect(findNavItemByPath('/editor')?.moduleId).toBe('tools')
    expect(EDITOR_MODULE_PATH_GUARD).toEqual({ prefix: '/editor', moduleId: 'tools' })
    expect(findEditorModulePathGuard('/editor/documento')?.moduleId).toBe('tools')
    expect(findEditorModulePathGuard('/editorial')).toBeUndefined()
  })

  it('keeps performance feedback inside the queue-gated consultant route', () => {
    const item = findNavItemByPath('/consultor')
    expect(item?.workspaceId).toBe('consultor')
    expect(item?.moduleId).toBe('queue')
    expect(findNavItemByPath('/feedback-desempenho')).toBeUndefined()
  })

  it.each(AGENCY_ONLY_PATHS)(
    'keeps the internal path %s agency-only in navigation and direct routes',
    (path) => {
      expect(isAgencyOnlyPath(path)).toBe(true)
      if (['/themes', '/manage/auditoria', '/manage/integracoes', '/roadmap'].includes(path)) {
        expect(findNavItemByPath(path)?.agencyOnly).toBe(true)
      }
    },
  )

  it('does not classify the contracted calendar route as an internal agency path', () => {
    expect(isAgencyOnlyPath('/calendario')).toBe(false)
  })
})
