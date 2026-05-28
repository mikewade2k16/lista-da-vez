import { describe, expect, it } from 'vitest'

import {
  getAllowedWorkspaces,
  getWorkspaceAccessDefinition,
  writeWorkspaceAccessState,
} from './permissions'

describe('permissions utils', () => {
  it('adds view and edit permissions without dropping unrelated permission keys', () => {
    const workspace = getWorkspaceAccessDefinition('campanhas')

    expect(writeWorkspaceAccessState(workspace, ['workspace.operacao.view'], 'edit')).toEqual([
      'workspace.operacao.view',
      'workspace.campanhas.view',
      'workspace.campanhas.edit',
    ])
  })

  it('keeps platform admin pages visible even when resolved permissions are stale', () => {
    expect(getAllowedWorkspaces('platform_admin', ['workspace.operacao.view'], true)).toEqual(
      expect.arrayContaining([
        'manage',
        'themes',
        'tools',
        'usuarios',
        'site',
        'site_produtos_web',
        'site_leads_web',
        'clientes',
        'clientes_web',
      ]),
    )
  })

  it('does not expose deprecated reference workspaces to owner', () => {
    const allowedWorkspaces = getAllowedWorkspaces('owner')

    expect(allowedWorkspaces).not.toContain('reference_preview')
    expect(allowedWorkspaces).not.toContain('reference_site_products')
    expect(allowedWorkspaces).not.toContain('reference_site_leads')
    expect(allowedWorkspaces).not.toContain('reference_clients')
    expect(allowedWorkspaces).not.toContain('site_produtos_web')
    expect(allowedWorkspaces).not.toContain('site_leads_web')
    expect(allowedWorkspaces).not.toContain('clientes_web')
  })
})
