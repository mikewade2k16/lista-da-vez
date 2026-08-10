import { existsSync, readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const planningPages = new URL('../../pages/planejamento/', import.meta.url)

describe('planning route contract', () => {
  it.each([
    ['metas.vue', 'goals'],
    ['funcionamento.vue', 'operation'],
    ['escalas.vue', 'schedule'],
  ])('keeps %s bound to the expected workspace section', (page, section) => {
    const source = readFileSync(new URL(page, planningPages), 'utf8')
    expect(source).toContain(`<PlanningWorkspace section="${section}" />`)
  })

  it('does not reuse a dynamic page between planning sections', () => {
    expect(existsSync(fileURLToPath(new URL('[section].vue', planningPages)))).toBe(false)
  })
})
