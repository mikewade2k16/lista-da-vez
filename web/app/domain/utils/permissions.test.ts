import { describe, expect, it } from 'vitest'

import {
  canAccessReports,
  canManageConsultants,
  canManageCrmCommercialPolicy,
  canManageSettings,
  canMutateOperations,
  canViewAlerts,
  canViewConsultants,
  getAllowedWorkspaces,
  getRoleLabel,
  getWorkspaceAccessDefinition,
  getWorkspaceAccessOptions,
  hasPermission,
  normalizeAppRole,
  normalizePermissionKeys,
  readWorkspaceAccessState,
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

describe('role normalization and labels', () => {
  it('normalizes aliases, defaults and whitespace', () => {
    expect(normalizeAppRole('admin')).toBe('platform_admin')
    expect(normalizeAppRole('')).toBe('consultant')
    expect(normalizeAppRole(undefined)).toBe('consultant')
    expect(normalizeAppRole(' owner ')).toBe('owner')
  })

  it('resolves human-friendly labels through the normalized role', () => {
    expect(getRoleLabel('manager')).toBe('Gerente')
    expect(getRoleLabel('admin')).toBe('Admin da plataforma')
    expect(getRoleLabel('store_terminal')).toBe('Acesso da loja')
    expect(getRoleLabel('papel_custom')).toBe('papel_custom')
    // caracterizacao: normalizeAppRole('') vira 'consultant', entao o branch
    // 'Sem papel' fica inalcancavel para string vazia.
    expect(getRoleLabel('')).toBe('Consultor')
  })
})

describe('permission key helpers', () => {
  it('normalizes permission key lists defensively', () => {
    expect(normalizePermissionKeys('not-an-array' as never)).toEqual([])
    expect(normalizePermissionKeys([' a ', '', null as never])).toEqual(['a'])
  })

  it('checks a single permission key against a list', () => {
    expect(hasPermission(['a'], '')).toBe(false)
    expect(hasPermission(null as never, 'a')).toBe(false)
    expect(hasPermission([' a '], 'a')).toBe(true)
  })
})

describe('workspace access state read/write', () => {
  it('lists access options driven by the workspace definition', () => {
    const campanhas = getWorkspaceAccessDefinition('campanhas')
    expect(getWorkspaceAccessOptions(campanhas).map((option) => option.value)).toEqual([
      'none',
      'view',
      'edit',
    ])
    expect(
      getWorkspaceAccessOptions(campanhas, { includeInherit: true }).map((option) => option.value),
    ).toEqual(['inherit', 'none', 'view', 'edit'])

    const relatorios = getWorkspaceAccessDefinition('relatorios')
    expect(getWorkspaceAccessOptions(relatorios).map((option) => option.value)).not.toContain(
      'edit',
    )
  })

  it('reads the current access state from the permission keys', () => {
    const campanhas = getWorkspaceAccessDefinition('campanhas')
    expect(
      readWorkspaceAccessState(campanhas, ['workspace.campanhas.view', 'workspace.campanhas.edit']),
    ).toBe('edit')
    expect(readWorkspaceAccessState(campanhas, ['workspace.campanhas.view'])).toBe('view')
    expect(readWorkspaceAccessState(campanhas, [])).toBe('none')

    const tasks = getWorkspaceAccessDefinition('tasks')
    expect(readWorkspaceAccessState(tasks, [], 'inherit')).toBe('inherit')
  })

  it('writes idempotently and clears the workspace keys on none', () => {
    const campanhas = getWorkspaceAccessDefinition('campanhas')
    const once = writeWorkspaceAccessState(campanhas, ['workspace.operacao.view'], 'edit')
    const twice = writeWorkspaceAccessState(campanhas, once, 'edit')
    expect(twice).toEqual(once)

    const cleared = writeWorkspaceAccessState(campanhas, once, 'none')
    expect(cleared).toEqual(['workspace.operacao.view'])
  })
})

describe('capability functions', () => {
  it('gates settings management by superuser, resolved and legacy paths', () => {
    expect(canManageSettings('platform_admin', [], true)).toBe(true)
    expect(canManageSettings('owner', ['workspace.configuracoes.edit'], true)).toBe(true)
    expect(canManageSettings('owner', ['queue.settings.manage'], true)).toBe(true)
    expect(canManageSettings('owner', [], true)).toBe(false)
    expect(canManageSettings('owner', [], false)).toBe(true)
    expect(canManageSettings('manager', [], false)).toBe(false)
  })

  it('gates consultant management the same way as settings', () => {
    expect(canManageConsultants('platform_admin', [], true)).toBe(true)
    expect(canManageConsultants('owner', ['workspace.configuracoes.edit'], true)).toBe(true)
    expect(canManageConsultants('owner', ['queue.consultants.manage'], true)).toBe(true)
    expect(canManageConsultants('owner', [], true)).toBe(false)
    expect(canManageConsultants('owner', [], false)).toBe(true)
    expect(canManageConsultants('manager', [], false)).toBe(false)
  })

  it('lets resolved consultant read permission view consultants; legacy terminal too', () => {
    expect(canViewConsultants('manager', ['queue.consultants.manage'], true)).toBe(true)
    expect(canViewConsultants('manager', [], true)).toBe(false)
    expect(canViewConsultants('store_terminal', [], false)).toBe(true)
    expect(canViewConsultants('manager', [], false)).toBe(false)
  })

  it('gates CRM commercial policy strictly by role', () => {
    expect(canManageCrmCommercialPolicy('director')).toBe(true)
    expect(canManageCrmCommercialPolicy('platform_admin')).toBe(true)
    expect(canManageCrmCommercialPolicy('owner', ['workspace.configuracoes.edit'], true)).toBe(
      false,
    )
  })
})
