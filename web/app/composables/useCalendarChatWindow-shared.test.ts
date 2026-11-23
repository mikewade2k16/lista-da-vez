import { effectScope, nextTick, reactive, ref } from 'vue'
import { afterEach, expect, it, vi } from 'vitest'
import { useCalendarChatWindow } from './useCalendarChatWindow'
const mocks = vi.hoisted(() => ({
  save: vi.fn(),
  calendar: {} as Record<string, unknown>,
  account: {} as { activeAccountId: string },
}))
vi.mock('~/stores/calendar', () => ({ useCalendarStore: () => mocks.calendar }))
vi.mock('../../layers/core/stores/account', () => ({ useCoreAccountStore: () => mocks.account }))
vi.mock('vue', async (original) => ({
  ...(await original<typeof import('vue')>()),
  onMounted: () => {},
  onBeforeUnmount: () => {},
}))
afterEach(() => vi.unstubAllGlobals())
it('keeps identical geometry across Calendar and Finance without saving Finance edits to Calendar', async () => {
  mocks.calendar = reactive({
    isConfigLoaded: true,
    config: { chat: { position: 'center', width: 760, height: 600 } },
    saveConfig: mocks.save,
  })
  mocks.account = reactive({ activeAccountId: 'a' })
  mocks.save.mockReset()
  vi.stubGlobal('useState', (_key: string, init: () => unknown) => ref(init()))
  vi.stubGlobal('document', {
    querySelector: () => ({
      getBoundingClientRect: () => ({ left: 80, top: 90, width: 1600, height: 900 }),
    }),
  })
  vi.stubGlobal('window', { clearTimeout: vi.fn(), setTimeout: vi.fn() })
  const surface = ref<'calendar' | 'finance'>('calendar')
  const scope = effectScope()
  const layout = scope.run(() =>
    useCalendarChatWindow(
      () => '.workspace',
      () => surface.value,
    ),
  )!
  const original = { ...layout.panelStyle.value }
  surface.value = 'finance'
  await nextTick()
  expect(layout.panelStyle.value).toEqual(original)
  layout.setPosition('right')
  const edited = { ...layout.panelStyle.value }
  surface.value = 'calendar'
  await nextTick()
  expect(layout.panelStyle.value).toEqual(edited)
  expect(mocks.save).not.toHaveBeenCalled()
  mocks.account.activeAccountId = 'b'
  await nextTick()
  expect(layout.panelStyle.value).toEqual(original)
  scope.stop()
})

it('uses the whole viewport below the menu on phones, ignoring the saved side position', () => {
  mocks.calendar = reactive({
    isConfigLoaded: true,
    config: { chat: { position: 'left', width: 360, height: 500 } },
    saveConfig: mocks.save,
  })
  mocks.account = reactive({ activeAccountId: 'a' })
  mocks.save.mockReset()
  vi.stubGlobal('useState', (_key: string, init: () => unknown) => ref(init()))
  vi.stubGlobal('document', {
    querySelector: () => ({
      getBoundingClientRect: () => ({ left: 0, top: 300, width: 390, height: 200 }),
    }),
  })
  vi.stubGlobal('window', {
    clearTimeout: vi.fn(),
    setTimeout: vi.fn(),
    innerWidth: 390,
    innerHeight: 844,
  })
  const scope = effectScope()
  const layout = scope.run(() => useCalendarChatWindow(() => '.workspace'))!
  expect(layout.compact.value).toBe(true)
  expect(layout.panelStyle.value).toEqual({
    left: '8px',
    top: '72px',
    width: '374px',
    height: '764px',
  })
  expect(mocks.save).not.toHaveBeenCalled()
  scope.stop()
})
