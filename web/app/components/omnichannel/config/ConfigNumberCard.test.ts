import { describe, expect, it, vi } from 'vitest'
import { useOmnichannelInstanceAccessEditor } from '~/composables/omnichannel/useOmnichannelInstanceAccessEditor'
import type { ApiRequest } from '~/domain/omnichannel/config-api'
import type { OmniInstanceAccessAdmin } from '~/domain/omnichannel/config-types'

const managerId = '00000000-0000-0000-0000-000000000001'
const agentId = '00000000-0000-0000-0000-000000000002'

function accessView(overrides: Partial<OmniInstanceAccessAdmin> = {}): OmniInstanceAccessAdmin {
  return {
    accessRevision: 3,
    accessPolicy: 'RESTRICTED',
    responsibleUserId: managerId,
    grants: [
      { userId: managerId, accessLevel: 'manage', isActive: true, revision: 1 },
      { userId: agentId, accessLevel: 'reply', isActive: false, revision: 2 },
    ],
    myCapabilities: { view: true, reply: true, manage: true, resetHistory: false },
    ...overrides,
  }
}

function editorWith(api: ReturnType<typeof vi.fn>) {
  return useOmnichannelInstanceAccessEditor({
    api: api as unknown as ApiRequest,
    instanceId: () => 'instance-a',
  })
}

describe('ConfigNumberCard access editor', () => {
  it('loads only authoritative active grants and never revives revoked rows', async () => {
    const api = vi.fn().mockResolvedValue(accessView())
    const editor = editorWith(api)

    expect(editor.status.value).toBe('idle')
    expect(await editor.load()).toBe(true)

    expect(editor.status.value).toBe('ready')
    expect(editor.grantLevels.value).toEqual({ [managerId]: 'manage' })
    expect(editor.authoritative.value?.grants).toHaveLength(2)
    expect(api).toHaveBeenCalledWith('/v1/omnichannel/tenant/whatsapp/instances/instance-a/users', {
      dedupe: false,
    })
  })

  it('saves revision, policy, responsible and levels while blocking a second click', async () => {
    let resolveSave: (view: OmniInstanceAccessAdmin) => void = () => undefined
    const pendingSave = new Promise<OmniInstanceAccessAdmin>((resolve) => {
      resolveSave = resolve
    })
    const api = vi.fn().mockResolvedValueOnce(accessView()).mockReturnValueOnce(pendingSave)
    const editor = editorWith(api)
    await editor.load()
    editor.setResponsible(managerId)
    editor.setGrant(agentId, 'reply')

    const first = editor.save()
    expect(await editor.save()).toBe('busy')
    resolveSave(
      accessView({
        accessRevision: 4,
        grants: [
          { userId: managerId, accessLevel: 'manage', isActive: true, revision: 1 },
          { userId: agentId, accessLevel: 'reply', isActive: true, revision: 3 },
        ],
      }),
    )

    expect(await first).toBe('saved')
    expect(api).toHaveBeenNthCalledWith(
      2,
      '/v1/omnichannel/tenant/whatsapp/instances/instance-a/users',
      {
        method: 'PUT',
        body: {
          accessRevision: 3,
          accessPolicy: 'RESTRICTED',
          responsibleUserId: managerId,
          grants: [
            { userId: managerId, accessLevel: 'manage' },
            { userId: agentId, accessLevel: 'reply' },
          ],
        },
      },
    )
  })

  it('refetches on 409 without replaying the mutation', async () => {
    const conflict = Object.assign(new Error('conflict'), { statusCode: 409 })
    const api = vi
      .fn()
      .mockResolvedValueOnce(accessView())
      .mockRejectedValueOnce(conflict)
      .mockResolvedValueOnce(accessView({ accessRevision: 7, accessPolicy: 'ACCOUNT_SHARED' }))
    const editor = editorWith(api)
    await editor.load()

    expect(await editor.save()).toBe('conflict')
    expect(api).toHaveBeenCalledTimes(3)
    expect(editor.status.value).toBe('ready')
    expect(editor.authoritative.value?.accessRevision).toBe(7)
    expect(editor.accessPolicy.value).toBe('ACCOUNT_SHARED')
    expect(editor.errorMessage.value).toContain('recarregado')
  })

  it('fails closed when the access GET fails', async () => {
    const api = vi.fn().mockRejectedValue(new Error('offline'))
    const editor = editorWith(api)

    expect(await editor.load()).toBe(false)
    expect(editor.status.value).toBe('error')
    expect(editor.authoritative.value).toBeNull()
    expect(editor.grantLevels.value).toEqual({})
  })
})
