import { describe, expect, it } from 'vitest'
import type { ObsSecurityFinding } from '~/types/observability'
import {
  countFindings,
  filterFindings,
  formatDueDate,
  isFindingOverdue,
  todayISO,
} from './findings'

const base: ObsSecurityFinding = {
  id: 'C-1',
  severity: 'high',
  title: 't',
  area: 'a',
  location: 'l',
  status: 'open',
  owner: '',
  dueDate: null,
  note: '',
  updatedBy: '',
  updatedAt: '',
  overdue: false,
}
const mk = (over: Partial<ObsSecurityFinding>) => ({ ...base, ...over })
const TODAY = '2026-11-24'

describe('findings', () => {
  it('todayISO usa a data local', () => {
    expect(todayISO(new Date(2026, 10, 4, 23, 59))).toBe('2026-11-04')
  })

  it('vencido so quando aberto e prazo anterior a hoje', () => {
    expect(isFindingOverdue(mk({ dueDate: '2026-11-23' }), TODAY)).toBe(true)
    expect(isFindingOverdue(mk({ dueDate: TODAY }), TODAY)).toBe(false)
    expect(isFindingOverdue(mk({ dueDate: '2026-11-23', status: 'fixed' }), TODAY)).toBe(false)
    expect(isFindingOverdue(mk({ dueDate: '2026-11-23', status: 'accepted' }), TODAY)).toBe(false)
    expect(isFindingOverdue(mk({ dueDate: null }), TODAY)).toBe(false)
  })

  it('filtra por abertos, vencidos e meus', () => {
    const list = [
      mk({ id: 'A-1', owner: 'Ana Souza', dueDate: '2026-01-01' }),
      mk({ id: 'A-2', owner: 'Bia', status: 'fixed' }),
      mk({ id: 'A-3', dueDate: '2027-01-01' }),
    ]
    expect(filterFindings(list, 'all', 'x', TODAY)).toHaveLength(3)
    expect(filterFindings(list, 'open', 'x', TODAY).map((f) => f.id)).toEqual(['A-1', 'A-3'])
    expect(filterFindings(list, 'overdue', 'x', TODAY).map((f) => f.id)).toEqual(['A-1'])
    expect(filterFindings(list, 'mine', ' ana souza ', TODAY).map((f) => f.id)).toEqual(['A-1'])
    expect(filterFindings(list, 'mine', '', TODAY)).toEqual([])
  })

  it('conta abertos por severidade e vencidos', () => {
    const list = [
      mk({ severity: 'critical', dueDate: '2026-01-01' }),
      mk({ severity: 'high' }),
      mk({ severity: 'high', status: 'fixed', dueDate: '2026-01-01' }),
      mk({ severity: 'low', status: 'accepted' }),
    ]
    const counters = countFindings(list, TODAY)
    expect(counters.open).toEqual({ critical: 1, high: 1, medium: 0, low: 0 })
    expect(counters.overdue).toBe(1)
  })

  it('formata o prazo', () => {
    expect(formatDueDate('2026-11-24')).toBe('24/11/2026')
    expect(formatDueDate(null)).toBe('-')
  })
})
