// Paleta de cores das opcoes dos selects do board/modal de Tasks.
// Fonte unica compartilhada entre `OmniSelectMenuInput` (editor real) e
// `OmniLazySelectMenuInput` (placeholder leve montado antes da interacao),
// para que o badge exibido pelo placeholder tenha exatamente a mesma cor/estilo
// do badge renderizado pelo editor. Sem isso o card "piscaria" ao trocar o
// placeholder pelo input real.

export type OptionColorKey =
  | 'default'
  | 'gray'
  | 'brown'
  | 'orange'
  | 'yellow'
  | 'green'
  | 'blue'
  | 'purple'
  | 'pink'
  | 'red'

export type BadgeStyle = 'filled' | 'entity'

export interface OptionColorConfig {
  key: OptionColorKey
  label: string
  swatchClass: string
  badgeClass: string
  entityClass: string
}

export const OPTION_COLOR_PALETTE: OptionColorConfig[] = [
  {
    key: 'default',
    label: 'Padrao',
    swatchClass: 'bg-zinc-500',
    badgeClass: 'bg-zinc-700/45 text-zinc-100 ring-zinc-500/50',
    entityClass: 'bg-transparent text-[rgb(var(--text))] ring-transparent',
  },
  {
    key: 'gray',
    label: 'Cinza',
    swatchClass: 'bg-slate-500',
    badgeClass: 'bg-slate-600/50 text-slate-100 ring-slate-400/50',
    entityClass: 'bg-transparent text-[rgb(var(--text))] ring-slate-400/70',
  },
  {
    key: 'brown',
    label: 'Marrom',
    swatchClass: 'bg-amber-700',
    badgeClass: 'bg-amber-700/50 text-amber-50 ring-amber-500/50',
    entityClass: 'bg-transparent text-[rgb(var(--text))] ring-amber-600/70',
  },
  {
    key: 'orange',
    label: 'Laranja',
    swatchClass: 'bg-orange-500',
    badgeClass: 'bg-orange-600/55 text-orange-50 ring-orange-400/50',
    entityClass: 'bg-transparent text-[rgb(var(--text))] ring-orange-400/80',
  },
  {
    key: 'yellow',
    label: 'Amarelo',
    swatchClass: 'bg-yellow-500',
    badgeClass: 'bg-yellow-500/55 text-zinc-950 ring-yellow-400/50',
    entityClass: 'bg-transparent text-[rgb(var(--text))] ring-yellow-400/80',
  },
  {
    key: 'green',
    label: 'Verde',
    swatchClass: 'bg-emerald-500',
    badgeClass: 'bg-emerald-700/55 text-emerald-50 ring-emerald-400/50',
    entityClass: 'bg-transparent text-[rgb(var(--text))] ring-emerald-400/80',
  },
  {
    key: 'blue',
    label: 'Azul',
    swatchClass: 'bg-blue-500',
    badgeClass: 'bg-blue-700/55 text-blue-50 ring-blue-400/50',
    entityClass: 'bg-transparent text-[rgb(var(--text))] ring-blue-400/80',
  },
  {
    key: 'purple',
    label: 'Roxo',
    swatchClass: 'bg-violet-500',
    badgeClass: 'bg-violet-700/55 text-violet-50 ring-violet-400/50',
    entityClass: 'bg-transparent text-[rgb(var(--text))] ring-violet-400/80',
  },
  {
    key: 'pink',
    label: 'Rosa',
    swatchClass: 'bg-fuchsia-500',
    badgeClass: 'bg-fuchsia-700/55 text-fuchsia-50 ring-fuchsia-400/50',
    entityClass: 'bg-transparent text-[rgb(var(--text))] ring-fuchsia-400/80',
  },
  {
    key: 'red',
    label: 'Vermelho',
    swatchClass: 'bg-rose-500',
    badgeClass: 'bg-rose-700/55 text-rose-50 ring-rose-400/50',
    entityClass: 'bg-transparent text-[rgb(var(--text))] ring-rose-400/80',
  },
]

export function normalizeOptionText(value: unknown, max = 180) {
  return String(value ?? '')
    .replace(/\s+/g, ' ')
    .trim()
    .slice(0, max)
}

export function optionColorKey(value: unknown): OptionColorKey {
  const key = normalizeOptionText(value, 30).toLowerCase() as OptionColorKey
  const known = OPTION_COLOR_PALETTE.find((item) => item.key === key)
  return known?.key || 'default'
}

export function optionColorConfig(color: unknown): OptionColorConfig {
  const key = optionColorKey(color)
  return OPTION_COLOR_PALETTE.find((item) => item.key === key) || OPTION_COLOR_PALETTE[0]!
}
