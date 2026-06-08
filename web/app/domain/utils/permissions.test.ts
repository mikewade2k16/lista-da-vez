import { describe, expect, it } from 'vitest'

import {
  canAccessReports,
  canMutateOperations,
  canViewAlerts,
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
        'site_tracking_web',
        'clientes',
        'clientes_web',
      ]),
    )
  })

  it('keeps site admin surfaces for owner without exposing deprecated reference workspaces', () => {
    const allowedWorkspaces = getAllowedWorkspaces('owner')

    expect(allowedWorkspaces).not.toContain('reference_preview')
    expect(allowedWorkspaces).not.toContain('reference_site_products')
    expect(allowedWorkspaces).not.toContain('reference_site_leads')
    expect(allowedWorkspaces).not.toContain('reference_clients')
    expect(allowedWorkspaces).not.toContain('clientes_web')
    expect(allowedWorkspaces).toEqual(
      expect.arrayContaining(['site', 'site_produtos_web', 'site_leads_web', 'site_tracking_web']),
    )
  })

  it('treats alert action permission as enough to view alert notifications', () => {
    const permissions = ['alerts.actions.manage']

    expect(canViewAlerts('store_terminal', permissions, true)).toBe(true)
    expect(getAllowedWorkspaces('store_terminal', permissions, true)).toContain('alertas')
  })

  it('maps legacy queue permissions to current queue workspaces', () => {
    const permissions = [
      'queue.dashboard.read',
      'queue.operations.manage',
      'queue.alerts.manage',
      'queue.reports.read',
    ]
    const allowedWorkspaces = getAllowedWorkspaces('store_terminal', permissions, true)

    expect(canViewAlerts('store_terminal', permissions, true)).toBe(true)
    expect(canAccessReports('store_terminal', permissions, true)).toBe(true)
    expect(allowedWorkspaces).toEqual(expect.arrayContaining(['operacao', 'alertas', 'relatorios']))
    expect(allowedWorkspaces).not.toContain('multiloja')
  })

  it('requires resolved operation edit to mutate operations; view-only stays read-only', () => {
    expect(canMutateOperations('consultant', ['workspace.operacao.view'], true)).toBe(false)
    expect(canMutateOperations('consultant', ['workspace.operacao.edit'], true)).toBe(true)
    expect(canMutateOperations('director', ['workspace.operacao.view'], true)).toBe(false)
  })

  it('keeps tenant operator roles able to mutate operations in legacy role mode', () => {
    expect(canMutateOperations('consultant', [], false)).toBe(true)
    expect(canMutateOperations('store_terminal', [], false)).toBe(true)
    expect(canMutateOperations('manager', [], false)).toBe(true)
    expect(canMutateOperations('director', [], false)).toBe(false)
    expect(canMutateOperations('marketing', [], false)).toBe(false)
  })
})
