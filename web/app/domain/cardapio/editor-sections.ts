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
  gate: 'all' | 'platform'
}

export const CARDAPIO_SECTIONS: SectionTab[] = [
  { id: 'dados', label: 'Dados', gate: 'all' },
  { id: 'categorias', label: 'Categorias', gate: 'all' },
  { id: 'produtos', label: 'Produtos', gate: 'all' },
  { id: 'avaliacoes', label: 'Avaliacoes', gate: 'all' },
  { id: 'pedidos', label: 'Pedidos', gate: 'all' },
  { id: 'relatorios', label: 'Relatorios', gate: 'all' },
  { id: 'entrega', label: 'Entrega', gate: 'all' },
  { id: 'dominios', label: 'Dominios', gate: 'platform' },
  { id: 'aparencia', label: 'Aparencia', gate: 'all' },
  { id: 'site', label: 'Site', gate: 'platform' },
]

export function resolveCardapioSection(value: unknown, allowed: readonly SectionId[]): SectionId {
  return allowed.find((id) => id === value) ?? allowed[0] ?? 'dados'
}

export function cardapioSectionLocation(
  route: { path: string; query: LocationQuery; hash: string },
  section: SectionId,
) {
  return { path: route.path, query: { ...route.query, aba: section }, hash: route.hash }
}
