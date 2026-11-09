import { useUiStore } from '~/stores/ui'

export async function confirmFiscalAction(
  message: string,
  scopeKey: () => string,
): Promise<boolean> {
  const scope = scopeKey()
  const ui = useUiStore()
  const result = (await ui.confirm({ title: 'Cadastro fiscal', message })) as { confirmed: boolean }
  return result.confirmed && scope === scopeKey()
}
