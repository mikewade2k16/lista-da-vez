const LEGACY_TASK_CLIENT_NAMES: Record<string, string> = {
  '1': 'Pérola',
  '2': 'Dr Antonio Tavares',
  '4': 'Crow Visuals',
  '5': 'Cléo Moraes',
  '6': 'AM Malls',
  '7': 'UNO',
  '8': 'Juliana Oliveira',
  '10': 'Mostarda',
  '11': 'Duby',
  '101': 'Pérola',
  '104': 'Dr Antonio Tavares',
  '105': 'UNO',
  '106': 'Crow Visuals',
}

function isNumericLegacyId(value: string) {
  return /^\d+$/.test(value)
}

export function legacyTaskClientLabel(clientId: unknown, persistedLabel: unknown): string {
  const id = String(clientId ?? '').trim()
  const canonical = LEGACY_TASK_CLIENT_NAMES[id]
  if (canonical) return canonical

  const label = String(persistedLabel ?? '').trim()
  if (!isNumericLegacyId(id)) return label

  // Um ID sem mapeamento nao e nome de cliente. Preserva apenas um nome legado humano e
  // rejeita placeholders que voltariam a expor "Cliente 123" na interface.
  if (!label || /^cliente\s*#?\s*\d+$/i.test(label)) return ''
  return label
}
