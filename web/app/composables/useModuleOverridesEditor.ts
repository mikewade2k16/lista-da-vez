import { computed, reactive, ref } from 'vue'
import type {
  AvailablePermission,
  PermissionEffect,
  UserPermissionOverride,
} from '~/types/admin-users'

// Editor client-side de overrides de modulo (tri-estado por permissao) do painel
// "Modulos". Concentra o ESTADO de edicao (rascunho + snapshot salvo) e o estado de
// VIEW (busca/filtros/lote), deixando o componente fino: ele so carrega/salva via
// API e renderiza. NADA aqui muda o contrato de salvar — busca/filtro/lote sao puro
// estado local sobre o catalogo ja carregado; a persistencia (replace tri-estado)
// e do componente, que chama buildPayload() e re-hidrata com hydrate().

export type TriState = 'inherit' | PermissionEffect
type EffectFilter = 'all' | TriState

export const MODULE_FILTER_ALL = 'all'

export function useModuleOverridesEditor() {
  const available = ref<AvailablePermission[]>([])
  // Rascunho por permissionKey. 'inherit' = sem override explicito.
  const states = reactive<Record<string, TriState>>({})
  // Snapshot do ultimo estado SALVO (re-hidratado do backend). Base do dirty/restore.
  const savedStates = reactive<Record<string, TriState>>({})

  // --- Estado de VIEW (nao afeta o payload) ---
  const searchTerm = ref('')
  const effectFilter = ref<EffectFilter>('all')
  const moduleFilter = ref<string>(MODULE_FILTER_ALL)

  // Reaplica rascunho + snapshot a partir da resposta autoritativa do backend.
  function hydrate(availablePerms: AvailablePermission[], overrides: UserPermissionOverride[]) {
    available.value = availablePerms
    for (const key of Object.keys(states)) delete states[key]
    for (const key of Object.keys(savedStates)) delete savedStates[key]
    for (const perm of availablePerms) {
      states[perm.key] = 'inherit'
      savedStates[perm.key] = 'inherit'
    }
    for (const ov of overrides) {
      states[ov.permissionKey] = ov.effect
      savedStates[ov.permissionKey] = ov.effect
    }
  }

  function reset() {
    available.value = []
    for (const key of Object.keys(states)) delete states[key]
    for (const key of Object.keys(savedStates)) delete savedStates[key]
  }

  // Agrupa as permissoes por moduleId, preservando a ordem de chegada.
  const groups = computed(() => {
    const map = new Map<string, AvailablePermission[]>()
    for (const perm of available.value) {
      const list = map.get(perm.moduleId) ?? []
      list.push(perm)
      map.set(perm.moduleId, list)
    }
    return [...map.entries()].map(([moduleId, permissions]) => ({ moduleId, permissions }))
  })

  const moduleFilterOptions = computed(() => [
    { value: MODULE_FILTER_ALL, label: 'Todos os modulos' },
    ...groups.value.map((g) => ({ value: g.moduleId, label: g.moduleId })),
  ])

  const effectCounts = computed(() => {
    const counts = { all: available.value.length, inherit: 0, allow: 0, deny: 0 }
    for (const perm of available.value) counts[states[perm.key] ?? 'inherit'] += 1
    return counts
  })

  const effectFilterOptions = computed(() => [
    { value: 'all', label: 'Todos', count: effectCounts.value.all },
    { value: 'inherit', label: 'Herdar', count: effectCounts.value.inherit },
    { value: 'allow', label: 'Permitir', count: effectCounts.value.allow },
    { value: 'deny', label: 'Negar', count: effectCounts.value.deny },
  ])

  function groupSummary(perms: AvailablePermission[]): string {
    const overrides = perms.filter((p) => (states[p.key] ?? 'inherit') !== 'inherit').length
    return overrides > 0 ? `${overrides}/${perms.length} override` : `${perms.length} permissoes`
  }

  function matchesSearch(perm: AvailablePermission): boolean {
    const term = searchTerm.value.trim().toLowerCase()
    if (!term) return true
    return perm.label.toLowerCase().includes(term) || perm.key.toLowerCase().includes(term)
  }

  function matchesEffect(perm: AvailablePermission): boolean {
    if (effectFilter.value === 'all') return true
    return (states[perm.key] ?? 'inherit') === effectFilter.value
  }

  // Grupos apos busca + filtro de efeito + filtro de modulo (grupo vazio some).
  const visibleGroups = computed(() => {
    const moduleSel = moduleFilter.value
    return groups.value
      .filter((g) => moduleSel === MODULE_FILTER_ALL || g.moduleId === moduleSel)
      .map((g) => ({
        moduleId: g.moduleId,
        permissions: g.permissions.filter((p) => matchesSearch(p) && matchesEffect(p)),
      }))
      .filter((g) => g.permissions.length > 0)
  })

  const visiblePermKeys = computed(() =>
    visibleGroups.value.flatMap((g) => g.permissions.map((p) => p.key)),
  )

  const hasActiveView = computed(
    () =>
      searchTerm.value.trim() !== '' ||
      effectFilter.value !== 'all' ||
      moduleFilter.value !== MODULE_FILTER_ALL,
  )

  function clearView() {
    searchTerm.value = ''
    effectFilter.value = 'all'
    moduleFilter.value = MODULE_FILTER_ALL
  }

  function setEffectFilter(value: string) {
    effectFilter.value = value as EffectFilter
  }

  function setState(key: string, value: TriState) {
    states[key] = value
  }

  // Chaves visiveis de UM modulo (apos busca/filtro). Vazio se o modulo sumiu da view.
  function visibleKeysOf(moduleId: string): string[] {
    const group = visibleGroups.value.find((g) => g.moduleId === moduleId)
    return group ? group.permissions.map((p) => p.key) : []
  }

  // Lote POR MODULO: aplica o efeito so as permissoes VISIVEIS daquele modulo
  // (respeita busca/filtro). So mexe no rascunho — nao auto-salva.
  function applyBulkToModule(moduleId: string, effect: TriState) {
    for (const key of visibleKeysOf(moduleId)) states[key] = effect
  }

  // Restaura POR MODULO: reverte as permissoes daquele modulo (todas, nao so as
  // visiveis) para o ultimo estado salvo. Restaurar ignora o filtro de propósito,
  // para nao deixar edicoes pendentes escondidas por um filtro ativo.
  function restoreModule(moduleId: string) {
    const group = groups.value.find((g) => g.moduleId === moduleId)
    if (!group) return
    for (const perm of group.permissions) states[perm.key] = savedStates[perm.key] ?? 'inherit'
  }

  // Reverte o rascunho inteiro para o ultimo estado salvo ("Restaurar tudo" global).
  function restoreSaved() {
    for (const key of Object.keys(states)) states[key] = savedStates[key] ?? 'inherit'
  }

  const isDirty = computed(() =>
    Object.keys(states).some((key) => states[key] !== (savedStates[key] ?? 'inherit')),
  )

  function isRowDirty(key: string): boolean {
    return states[key] !== savedStates[key]
  }

  // Um modulo tem edicao pendente se qualquer permissao sua difere do snapshot.
  function isModuleDirty(moduleId: string): boolean {
    const group = groups.value.find((g) => g.moduleId === moduleId)
    if (!group) return false
    return group.permissions.some((p) => states[p.key] !== (savedStates[p.key] ?? 'inherit'))
  }

  const overrideCount = computed(() => Object.values(states).filter((s) => s !== 'inherit').length)

  // Monta o payload de replace (so allow/deny; inherit = ausencia de entrada).
  function buildPayload(): UserPermissionOverride[] {
    const payload: UserPermissionOverride[] = []
    for (const [permissionKey, state] of Object.entries(states)) {
      if (state === 'inherit') continue
      payload.push({ permissionKey, effect: state })
    }
    return payload
  }

  return {
    available,
    states,
    savedStates,
    searchTerm,
    effectFilter,
    moduleFilter,
    groups,
    moduleFilterOptions,
    effectFilterOptions,
    visibleGroups,
    visiblePermKeys,
    hasActiveView,
    isDirty,
    overrideCount,
    hydrate,
    reset,
    clearView,
    setEffectFilter,
    setState,
    applyBulkToModule,
    restoreModule,
    restoreSaved,
    isRowDirty,
    isModuleDirty,
    groupSummary,
    buildPayload,
  }
}
