import { expect, it, vi } from 'vitest'
import {
  createObligationsController,
  type ObligationContext,
  type ObligationTransport,
} from './obligations-controller'
const context = (): ObligationContext => ({
  accountId: 'a',
  actorId: 'u',
  issuerId: 'i',
  modules: ['accounting'],
  permissions: ['accounting.obligations.view', 'accounting.obligations.manage'],
})
it('does not fetch without a module or read permission', async () => {
  const api = vi.fn(),
    c = createObligationsController(() => ({ ...context(), permissions: [] }), api)
  await c.load()
  expect(api).not.toHaveBeenCalled()
})
it('keeps unknown amounts and dates; creates only an explicit pending record', async () => {
  const api = vi.fn().mockResolvedValue({
    items: [{ id: 'o', amount: null, dueDate: null, pending: ['rule_unavailable'] }],
  })
  const c = createObligationsController(context, api, () => 'stable-key')
  c.state.code = 'company.tax_review'
  c.state.period = '2026-11'
  await c.create()
  expect(api.mock.calls[0]?.[2]).toMatchObject({
    method: 'POST',
    key: 'stable-key',
    body: { base: null, expectedVersion: 0, code: 'company.tax_review' },
  })
  expect(c.state.items[0]?.amount).toBeNull()
  expect(c.state.items[0]?.dueDate).toBeNull()
})
it('discards old account responses and stops follow-up after context change', async () => {
  let ctx = context(),
    finish!: (v: unknown) => void
  const api = vi.fn(
    () =>
      new Promise((resolve) => {
        finish = resolve
      }),
  )
  const c = createObligationsController(() => ctx, api as ObligationTransport)
  const pending = c.create()
  ctx = { ...ctx, accountId: 'b' }
  c.reset()
  finish({ items: [{ id: 'old' }] })
  await pending
  expect(api).toHaveBeenCalledTimes(1)
  expect(c.state.items).toEqual([])
  expect(c.state.busy).toBe(false)
})
it('preserves a pending error and never retries writes automatically', async () => {
  const api = vi.fn().mockRejectedValue(new Error('409'))
  const c = createObligationsController(context, api)
  c.state.period = '2026-11'
  await c.create()
  expect(api).toHaveBeenCalledTimes(1)
  expect(c.state.error).not.toBe('')
  expect(c.state.period).toBe('2026-11')
})
