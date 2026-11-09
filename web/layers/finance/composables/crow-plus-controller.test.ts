import { expect, it, vi } from 'vitest'
import {
  createCrowPlusController,
  type CrowPlusTransport,
  type CrowPlusContext,
} from './crow-plus-controller'
function setup() {
  const context: CrowPlusContext = {
    accountId: 'a',
    actorId: 'u',
    modules: ['finance'],
    permissions: ['finance.sheets.view'],
    sheetId: 's',
    month: '2026-11',
  }
  const api = vi
    .fn()
    .mockImplementation(async (_account, path) =>
      path.includes('/status') ? { data: { enabled: true } } : { conversations: [] },
    )
  const controller = createCrowPlusController(
    () => context,
    api as CrowPlusTransport,
    () => 'stable-request-key',
  )
  return { context, api, controller }
}
it('does not fetch without effective permission', async () => {
  const { context, api, controller } = setup()
  context.permissions = []
  expect(await controller.load()).toBe(false)
  expect(api).not.toHaveBeenCalled()
})
it('uses authoritative flag and skips history when disabled', async () => {
  const { api, controller } = setup()
  api.mockResolvedValue({ data: { enabled: false } })
  await controller.load()
  expect(controller.state.statusKnown).toBe(true)
  expect(controller.state.enabled).toBe(false)
  expect(api).toHaveBeenCalledTimes(1)
})
it('discards old status and does not fetch history after account reset', async () => {
  const { context, api, controller } = setup()
  let resolve!: (value: unknown) => void
  api.mockImplementationOnce(
    () =>
      new Promise((r) => {
        resolve = r
      }),
  )
  const pending = controller.load()
  context.accountId = 'b'
  controller.reset()
  resolve({ data: { enabled: true } })
  expect(await pending).toBe(false)
  expect(api).toHaveBeenCalledTimes(1)
  expect(controller.state.enabled).toBe(false)
})
it('keeps message and idempotency key on transport retry without legacy scope', async () => {
  const { api, controller } = setup()
  await controller.load()
  controller.state.draft = 'resumo'
  api.mockRejectedValue(new Error('offline'))
  await controller.send()
  await controller.send()
  expect(controller.state.draft).toBe('resumo')
  const attempts = api.mock.calls.slice(-2)
  expect(attempts[0]?.[2].key).toBe(attempts[1]?.[2].key)
  expect(attempts[0]?.[2].body).toMatchObject({
    mode: 'plus',
    surface: 'finance',
    financeSheetId: 's',
  })
  expect(attempts[0]?.[2].body).not.toHaveProperty('clientId')
})
it('quarantines uncertain preparation and refuses automatic repeat', async () => {
  const { api, controller } = setup()
  await controller.load()
  controller.state.draft = 'preparar nota'
  api.mockRejectedValue({ data: { error: { code: 'request_in_progress' } } })
  await controller.send({ draftId: 'draft' })
  expect(controller.state.blocked).toBe(true)
  api.mockClear()
  await controller.send({ draftId: 'draft' })
  expect(api).not.toHaveBeenCalled()
})
it('rejects secrets before transport and clears context on issuer change', async () => {
  const { context, api, controller } = setup()
  await controller.load()
  api.mockClear()
  controller.state.draft = 'senha teste'
  await controller.send()
  expect(api).not.toHaveBeenCalled()
  context.modules = ['finance', 'accounting']
  context.permissions = ['finance.sheets.view', 'accounting.issuers.view']
  controller.state.messages = [{ id: 'old', role: 'user', content: 'old' }]
  await controller.chooseIssuer('issuer-b')
  expect(controller.state.messages).toEqual([])
  expect(controller.state.draft).toBe('')
})
it('does not incorporate a response from another issuer', async () => {
  const { api, controller } = setup()
  await controller.load()
  controller.state.draft = 'resumo'
  api.mockResolvedValue({ context: { mode: 'plus', surface: 'finance', issuerId: 'other' } })
  await controller.send()
  expect(controller.state.messages).toEqual([])
  expect(controller.state.draft).toBe('resumo')
  expect(controller.state.error).toContain('incompatível')
})
