import { describe, expect, it } from 'vitest'

import {
  canEditPerformanceFeedback,
  canViewPerformanceFeedback,
  getAllowedWorkspaces,
} from './permissions'

// Matriz papel x workspace de getAllowedWorkspaces. Separado de permissions.test.ts
// para nao estourar o limite de linhas por arquivo. Cobre: modo legado (defaults do
// papel), bypass de platform_admin (contrato isPlatformAdmin || has(...)), fail-closed
// com permissoes resolvidas, workspace de modulo por prefixo e aliases por papel.
describe('getAllowedWorkspaces — legacy mode (permissions unresolved)', () => {
  it('falls back to the role defaults', () => {
    expect(getAllowedWorkspaces('consultant')).toEqual(['operacao', 'comunicados', 'consultor'])
    expect(getAllowedWorkspaces('store_terminal')).toEqual(
      expect.arrayContaining(['operacao', 'consultor', 'relatorios', 'alertas']),
    )
  })

  it('fails closed to the consultant defaults for an unknown role', () => {
    expect(getAllowedWorkspaces('papel_custom_desconhecido')).toEqual([
      'operacao',
      'comunicados',
      'consultor',
    ])
  })

  it('resolves the admin alias to the platform_admin defaults', () => {
    expect(getAllowedWorkspaces('admin')).toContain('manage')
  })
})

describe('getAllowedWorkspaces — platform_admin bypass', () => {
  it('returns the full role list even with zero resolved permissions', () => {
    expect(getAllowedWorkspaces('platform_admin', [], true)).toEqual(
      getAllowedWorkspaces('platform_admin'),
    )
  })
})

describe('getAllowedWorkspaces — fail-closed with resolved permissions', () => {
  it('hides every workspace without the matching key', () => {
    expect(getAllowedWorkspaces('consultant', [], true)).toEqual([])
  })

  it('shows operacao once the view key is present', () => {
    expect(getAllowedWorkspaces('consultant', ['workspace.operacao.view'], true)).toEqual([
      'operacao',
      'comunicados',
    ])
  })

  it('keeps feedback inside the consultant workspace', () => {
    expect(getAllowedWorkspaces('consultant', ['workspace.consultor.view'], true)).toEqual([
      'consultor',
    ])
    expect(
      getAllowedWorkspaces('consultant', ['workspace.performance_feedback.view'], true),
    ).toEqual([])
  })
})

describe('performance feedback action', () => {
  it('uses its dedicated permission without creating a separate workspace', () => {
    expect(canViewPerformanceFeedback('manager')).toBe(true)
    expect(canViewPerformanceFeedback('store_terminal')).toBe(false)
    expect(canViewPerformanceFeedback('consultant', [], true)).toBe(false)
    expect(
      canViewPerformanceFeedback('consultant', ['workspace.performance_feedback.view'], true),
    ).toBe(true)
  })

  it('restricts configuration to roles allowed to edit feedback', () => {
    expect(canEditPerformanceFeedback('manager')).toBe(true)
    expect(canEditPerformanceFeedback('consultant')).toBe(false)
    expect(
      canEditPerformanceFeedback('consultant', ['workspace.performance_feedback.view'], true),
    ).toBe(false)
    expect(
      canEditPerformanceFeedback('manager', ['workspace.performance_feedback.edit'], true),
    ).toBe(true)
  })
})

describe('global storage workspace', () => {
  it('is exclusive to the platform administrator defaults', () => {
    expect(getAllowedWorkspaces('platform_admin')).toContain('storage_admin')
    expect(getAllowedWorkspaces('owner')).not.toContain('storage_admin')
  })
})

describe('sensitive queue workspaces', () => {
  it('does not expose transcriptions through operation access', () => {
    expect(getAllowedWorkspaces('consultant', ['workspace.operacao.view'], true)).not.toContain(
      'transcricoes',
    )
    expect(getAllowedWorkspaces('owner', ['workspace.transcricoes.view'], true)).toContain(
      'transcricoes',
    )
  })

  it('keeps campaigns out of the marketing defaults', () => {
    expect(getAllowedWorkspaces('marketing')).not.toContain('campanhas')
    expect(getAllowedWorkspaces('owner')).toContain('campanhas')
  })
})

describe('getAllowedWorkspaces — module workspace by permission prefix', () => {
  it('exposes tasks for a custom role holding any tasks.* permission', () => {
    expect(getAllowedWorkspaces('consultant', ['tasks.boards.manage'], true)).toContain('tasks')
  })
})

describe('getAllowedWorkspaces — role aliases and defaults', () => {
  it('keeps owner defaults without leaking fine-grained workspaces', () => {
    const owner = getAllowedWorkspaces('owner', [], true)
    expect(owner).toContain('tasks')
    expect(owner).not.toContain('campanhas')
    expect(owner).not.toContain('usuarios')
    expect(owner).not.toContain('clientes')
    expect(owner).not.toContain('relatorios')
  })

  it('gates cardapio_web by the cardapio.view permission for non-owner roles', () => {
    expect(getAllowedWorkspaces('manager', ['cardapio.view'], true)).toContain('cardapio_web')
    expect(getAllowedWorkspaces('manager', [], true)).not.toContain('cardapio_web')
  })
})

describe('getAllowedWorkspaces - social publishing', () => {
  it('includes the workspace in the platform_admin, owner and marketing role defaults', () => {
    expect(getAllowedWorkspaces('platform_admin')).toContain('social_publishing')
    expect(getAllowedWorkspaces('owner')).toContain('social_publishing')
    expect(getAllowedWorkspaces('marketing')).toContain('social_publishing')
  })

  it('fails closed until the resolved view permission is present', () => {
    expect(getAllowedWorkspaces('marketing', [], true)).not.toContain('social_publishing')
    expect(getAllowedWorkspaces('marketing', ['social_publishing.view'], true)).toContain(
      'social_publishing',
    )
  })
})

describe('planning workspace', () => {
  it('uses its own resolved permission without exposing it to consultants by default', () => {
    expect(getAllowedWorkspaces('owner')).toContain('planejamento')
    expect(getAllowedWorkspaces('director', [], true)).not.toContain('planejamento')
    expect(getAllowedWorkspaces('manager', ['workspace.planejamento.view'], true)).toContain(
      'planejamento',
    )
    expect(getAllowedWorkspaces('manager', ['workspace.multiloja.edit'], true)).not.toContain(
      'planejamento',
    )
    expect(getAllowedWorkspaces('consultant')).not.toContain('planejamento')
  })
})
