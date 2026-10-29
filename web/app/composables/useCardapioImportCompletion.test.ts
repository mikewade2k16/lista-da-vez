import { effectScope, nextTick, ref } from 'vue'
import { afterEach, expect, it, vi } from 'vitest'
import { useCardapioImportCompletion } from './useCardapioImportCompletion'
import type { ProductImportResult } from '~/domain/cardapio/product-transfer'

afterEach(() => vi.useRealTimers())
const success: ProductImportResult = { created: 220, updated: 0, skipped: 0, failed: 0, errors: [] }
it('mostra o resultado antes de fechar e cancela ao desmontar ou haver erros', async () => {
  vi.useFakeTimers()
  const result = ref<ProductImportResult | null>(null)
  const close = vi.fn()
  const scope = effectScope()
  scope.run(() => useCardapioImportCompletion(() => result.value, close))
  result.value = success
  await nextTick()
  vi.advanceTimersByTime(1499)
  expect(close).not.toHaveBeenCalled()
  vi.advanceTimersByTime(1)
  expect(close).toHaveBeenCalledTimes(1)
  result.value = { ...success, failed: 1 }
  await nextTick()
  vi.advanceTimersByTime(2000)
  expect(close).toHaveBeenCalledTimes(1)
  result.value = success
  await nextTick()
  scope.stop()
  vi.advanceTimersByTime(2000)
  expect(close).toHaveBeenCalledTimes(1)
})
