import { computed, ref, type Ref } from 'vue'

// Estado de VIEW (busca + filtro + agrupamento) do painel "Paginas". Puramente
// client-side sobre as linhas ja carregadas: NAO altera o contrato de salvar. O
// componente passa as linhas controlaveis e o mapa de estados tri (rascunho) e
// recebe de volta as secoes colapsaveis filtradas. Separar isto enxuga o painel.

export type PageTriState = 'inherit' | 'allow' | 'deny'
type EffectFilter = 'all' | PageTriState

export interface PageRow {
  id: string
  label: string
  description: string
  viewPermission: string
}

export function usePageOverridesView(rows: Ref<PageRow[]>, states: Record<string, PageTriState>) {
  const searchTerm = ref('')
  const effectFilter = ref<EffectFilter>('all')

  function matchesSearch(row: PageRow): boolean {
    const term = searchTerm.value.trim().toLowerCase()
    if (!term) return true
    return row.label.toLowerCase().includes(term) || row.description.toLowerCase().includes(term)
  }

  function matchesEffect(row: PageRow): boolean {
    if (effectFilter.value === 'all') return true
    return (states[row.id] ?? 'inherit') === effectFilter.value
  }

  const effectCounts = computed(() => {
    const counts = { all: rows.value.length, inherit: 0, allow: 0, deny: 0 }
    for (const row of rows.value) counts[states[row.id] ?? 'inherit'] += 1
    return counts
  })

  const effectFilterOptions = computed(() => [
    { value: 'all', label: 'Todos', count: effectCounts.value.all },
    { value: 'inherit', label: 'Herdar', count: effectCounts.value.inherit },
    { value: 'allow', label: 'Mostrar', count: effectCounts.value.allow },
    { value: 'deny', label: 'Ocultar', count: effectCounts.value.deny },
  ])

  const filteredRows = computed(() =>
    rows.value.filter((row) => matchesSearch(row) && matchesEffect(row)),
  )

  // Secoes colapsaveis: com override primeiro, herdando depois (secao vazia some).
  const groupedRows = computed(() => {
    const overridden: PageRow[] = []
    const inherited: PageRow[] = []
    for (const row of filteredRows.value) {
      if ((states[row.id] ?? 'inherit') === 'inherit') inherited.push(row)
      else overridden.push(row)
    }
    const sections: { key: string; title: string; rows: PageRow[] }[] = []
    if (overridden.length) {
      sections.push({ key: 'overridden', title: 'Com override por usuario', rows: overridden })
    }
    if (inherited.length) {
      sections.push({ key: 'inherited', title: 'Herdando o papel', rows: inherited })
    }
    return sections
  })

  const hasActiveView = computed(
    () => searchTerm.value.trim() !== '' || effectFilter.value !== 'all',
  )

  function clearView() {
    searchTerm.value = ''
    effectFilter.value = 'all'
  }

  function setEffectFilter(value: string) {
    effectFilter.value = value as EffectFilter
  }

  return {
    searchTerm,
    effectFilter,
    effectFilterOptions,
    groupedRows,
    hasActiveView,
    clearView,
    setEffectFilter,
  }
}
