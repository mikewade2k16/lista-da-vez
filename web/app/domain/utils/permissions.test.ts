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
      expect.arrayContaining(['manage', 'themes', 'tools', 'usuarios']),
    )
  })
})
