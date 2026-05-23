import { describe, expect, it } from 'vitest'

import { getWorkspaceAccessDefinition, writeWorkspaceAccessState } from './permissions'

describe('permissions utils', () => {
  it('adds view and edit permissions without dropping unrelated permission keys', () => {
    const workspace = getWorkspaceAccessDefinition('campanhas')

    expect(
      writeWorkspaceAccessState(workspace, ['workspace.operacao.view'], 'edit'),
    ).toEqual([
      'workspace.operacao.view',
      'workspace.campanhas.view',
      'workspace.campanhas.edit',
    ])
  })
})