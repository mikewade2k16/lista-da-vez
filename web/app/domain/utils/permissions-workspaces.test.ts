import { describe, expect, it } from 'vitest'

import { getAllowedWorkspaces } from './permissions'

// Matriz papel x workspace de getAllowedWorkspaces. Separado de permissions.test.ts
// para nao estourar o limite de linhas por arquivo. Cobre: modo legado (defaults do
// papel), bypass de platform_admin (contrato isPlatformAdmin || has(...)), fail-closed
// com permissoes resolvidas, workspace de modulo por prefixo e aliases por papel.
describe('getAllowedWorkspaces — legacy mode (permissions unresolved)', () => {
  it('falls back to the role defaults', () => {
    expect(getAllowedWorkspaces('consultant')).toEqual(['operacao'])
    expect(getAllowedWorkspaces('store_terminal')).toEqual(
      expect.arrayContaining(['operacao', 'consultor', 'relatorios', 'alertas']),
    )
  })

  it('fails closed to the consultant defaults for an unknown role', () => {
    expect(getAllowedWorkspaces('papel_custom_desconhecido')).toEqual(['operacao'])
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
    ])
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
