import { afterEach, describe, expect, it, vi } from 'vitest'
import { createFinanceSaveQueue } from './finance-save-queue'
import { financeRecurringRowId, financeRecurringStoreRowId } from './finance-ids'

afterEach(() => vi.useRealTimers())
describe('Finance captured save queue', () => {
  it('preserves each sheet/account and coalesces newer drafts', async () => {
    vi.useFakeTimers()
    const writes: string[] = []
    const queue = createFinanceSaveQueue(1200, () => {})
    queue.schedule('accountA:sheet1', async () => {
      writes.push('A1 old')
    })
    queue.schedule('accountA:sheet1', async () => {
      writes.push('A1 latest')
    })
    queue.schedule('accountB:sheet2', async () => {
      writes.push('B2')
    })
    expect(writes).toEqual([])
    expect(await queue.flush()).toBe(true)
    expect(writes).toEqual(['A1 latest', 'B2'])
  })
  it('keeps edits made during a slow request and serializes them', async () => {
    vi.useFakeTimers()
    let finish!: () => void
    const wait = new Promise<void>((resolve) => {
      finish = resolve
    })
    const writes: number[] = []
    const queue = createFinanceSaveQueue(1200, () => {})
    queue.schedule('A', async () => {
      await wait
      writes.push(1)
    })
    const saving = queue.flush()
    queue.schedule('A', async () => {
      writes.push(2)
    })
    finish()
    expect(await saving).toBe(true)
    expect(writes).toEqual([1, 2])
  })
  it('retains failed drafts for retry and reports pending/saving/error honestly', async () => {
    vi.useFakeTimers()
    const states: Array<{ pending: boolean; saving: boolean; failed: boolean }> = []
    const queue = createFinanceSaveQueue(1200, (state) => states.push(state))
    let failing = true
    queue.schedule('A', async () => {
      if (failing) throw new Error('offline')
    })
    expect(states.at(-1)?.pending).toBe(true)
    expect(await queue.flush()).toBe(false)
    expect(queue.has('A')).toBe(true)
    expect(states.at(-1)).toEqual({ pending: true, saving: false, failed: true })
    failing = false
    expect(await queue.flush()).toBe(true)
    expect(states.at(-1)).toEqual({ pending: false, saving: false, failed: false })
  })
})
it('uses distinct recurring row IDs per sheet and keeps them stable within a sheet', () => {
  expect(financeRecurringRowId('client', 'november')).not.toBe(
    financeRecurringRowId('client', 'december'),
  )
  expect(financeRecurringRowId('client', 'november')).toBe(
    financeRecurringRowId('client', 'november'),
  )
  expect(financeRecurringStoreRowId('client', 'store', 'november')).not.toBe(
    financeRecurringStoreRowId('client', 'store', 'december'),
  )
})
