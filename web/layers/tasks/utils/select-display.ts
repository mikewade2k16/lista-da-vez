// Helpers puros de exibicao de options de select — extraidos de `useTasksPageContext.ts`
// (F-17 split).
//
// `selectOptionColor` mapeia o nome de cor de uma option (status/tipo/prioridade) para a paleta
// canonica do `OmniSelectMenuInput`, com fallback ciclico por indice. `initialsFor` deriva as
// iniciais para o avatar de pessoa/cliente. Sao puras (dependem so do valor recebido), entao
// vivem aqui para serem testaveis e aliviar o agregador. O `normalizeKey` e' injetado pelo
// contexto (depende da forma local de normalizacao), mantendo o comportamento identico.

export function selectOptionColor(
  normalizeKey: (value: unknown) => string,
  value: unknown,
  index = 0,
): string {
  const key = normalizeKey(value)
  if (key === 'slate' || key === 'gray' || key === 'cinza') return 'gray'
  if (key === 'emerald' || key === 'green' || key === 'verde') return 'green'
  if (key === 'amber' || key === 'yellow' || key === 'amarelo') return 'yellow'
  if (key === 'rose' || key === 'red' || key === 'vermelho') return 'red'
  if (key === 'violet' || key === 'indigo' || key === 'purple' || key === 'roxo') return 'purple'
  if (key === 'blue' || key === 'azul') return 'blue'
  if (key === 'orange' || key === 'laranja') return 'orange'
  if (key === 'pink' || key === 'rosa') return 'pink'
  return ['blue', 'purple', 'green', 'orange', 'pink', 'yellow', 'red', 'gray'][index % 8]!
}

export function initialsFor(value: unknown): string {
  const s = String(value ?? '').trim()
  if (!s) return '?'
  const parts = s.split(/\s+/).filter(Boolean).slice(0, 2)
  const initials = parts.map((p) => p[0]?.toUpperCase() || '').join('')
  return initials || s[0]!.toUpperCase()
}
