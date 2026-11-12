import type { LocationQuery } from 'vue-router'

export type SectionId =
  | 'dados'
  | 'categorias'
  | 'produtos'
  | 'avaliacoes'
  | 'pedidos'
  | 'relatorios'
  | 'entrega'
  | 'dominios'
  | 'aparencia'
  | 'site'

export interface SectionTab {
  id: SectionId
  label: string
  // Icone usado no mobile, onde as abas viram so icone.
  icon: string
  gate: 'all' | 'platform'
}

export const CARDAPIO_SECTIONS: SectionTab[] = [
  { id: 'dados', label: 'Dados', icon: 'i-lucide-store', gate: 'all' },
  { id: 'categorias', label: 'Categorias', icon: 'i-lucide-folder', gate: 'all' },
  { id: 'produtos', label: 'Produtos', icon: 'i-lucide-utensils', gate: 'all' },
  { id: 'avaliacoes', label: 'Avaliacoes', icon: 'i-lucide-star', gate: 'all' },
  { id: 'pedidos', label: 'Pedidos', icon: 'i-lucide-receipt', gate: 'all' },
  { id: 'relatorios', label: 'Relatorios', icon: 'i-lucide-chart-column', gate: 'all' },
  { id: 'entrega', label: 'Entrega', icon: 'i-lucide-truck', gate: 'all' },
  { id: 'dominios', label: 'Dominios', icon: 'i-lucide-globe', gate: 'platform' },
  { id: 'aparencia', label: 'Aparencia', icon: 'i-lucide-palette', gate: 'all' },
  { id: 'site', label: 'Site', icon: 'i-lucide-layout-template', gate: 'platform' },
]

// Aba aberta quando a URL nao informa ?aba=: Produtos (fluxo principal do
// cliente, que cadastra as fotos), se estiver liberada.
export const DEFAULT_CARDAPIO_SECTION: SectionId = 'produtos'

export function resolveCardapioSection(value: unknown, allowed: readonly SectionId[]): SectionId {
  const requested = allowed.find((id) => id === value)
  if (requested) return requested
  return allowed.includes(DEFAULT_CARDAPIO_SECTION)
    ? DEFAULT_CARDAPIO_SECTION
    : (allowed[0] ?? 'dados')
}

export function cardapioSectionLocation(
  route: { path: string; query: LocationQuery; hash: string },
  section: SectionId,
) {
  return { path: route.path, query: { ...route.query, aba: section }, hash: route.hash }
}
