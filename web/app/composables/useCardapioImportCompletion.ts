import { onScopeDispose, watch } from 'vue'
import type { ProductImportResult } from '~/domain/cardapio/product-transfer'

export function useCardapioImportCompletion(
  result: () => ProductImportResult | null,
  close: () => void,
) {
  let timer: ReturnType<typeof setTimeout> | undefined
  watch(result, (value) => {
    clearTimeout(timer)
    if (value && !value.failed) timer = setTimeout(close, 1500)
  })
  onScopeDispose(() => clearTimeout(timer))
}
