import { describe, expect, it } from 'vitest'

import { canManageAssistantConfiguration, canViewAssistantConfiguration } from './assistant-access'

describe('assistant config access', () => {
  it('allows a module manager to view but not change cross-domain configuration', () => {
    const context = {
      role: 'member',
      effectivePermissionKeys: ['calendar.manage'],
      effectivePermissionsResolved: true,
    }
    expect(canViewAssistantConfiguration(context)).toBe(true)
    expect(canManageAssistantConfiguration(context)).toBe(false)
  })

  it('allows account-wide managers to view and change configuration', () => {
    const context = {
      role: 'member',
      effectivePermissionKeys: ['core.account.manage'],
      effectivePermissionsResolved: true,
    }
    expect(canViewAssistantConfiguration(context)).toBe(true)
    expect(canManageAssistantConfiguration(context)).toBe(true)
  })

  it('keeps ordinary members fail-closed while preserving owner access', () => {
    expect(canViewAssistantConfiguration({ role: 'member' })).toBe(false)
    expect(canManageAssistantConfiguration({ role: 'member' })).toBe(false)
    expect(canViewAssistantConfiguration({ role: 'owner' })).toBe(true)
    expect(canManageAssistantConfiguration({ role: 'owner' })).toBe(true)
  })

  it('ignores stale permission keys until account RBAC is resolved', () => {
    const context = {
      role: 'member',
      effectivePermissionKeys: ['core.account.manage'],
      effectivePermissionsResolved: false,
    }
    expect(canViewAssistantConfiguration(context)).toBe(false)
    expect(canManageAssistantConfiguration(context)).toBe(false)
  })
})
