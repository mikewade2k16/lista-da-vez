<script setup lang="ts">
import type { AvailablePermission } from '~/types/admin-users'
import AppSearchInput from '~/components/ui/AppSearchInput.vue'
import AppSegmentedFilter from '~/components/ui/AppSegmentedFilter.vue'
import AdminRolePermissionMatrix from '~/components/admin/users/AdminRolePermissionMatrix.vue'

// Matriz de permissoes binaria (on/off) de UM papel-padrao, com toolbar de busca e
// filtro por modulo. 100% APRESENTACAO + filtro client-side: recebe o catalogo
// `available` + o conjunto de keys marcadas e emite toggle(key, checked). A grade de
// checkboxes em si reusa o AdminRolePermissionMatrix (mesmo componente da matriz de
// papeis por-conta). Estado (selecao/persistencia) vive no host. Quando readonly, a
// grade some e mostra so um resumo das permissoes concedidas (read-only do sistema).

const props = defineProps<{
  available: AvailablePermission[]
  // Conjunto de keys marcadas (passado pelo host; predicate evita prop por-item).
  checkedKeys: Set<string>
  // Quando true (template de sistema): some a grade editavel, mostra so o resumo.
  readonly?: boolean
}>()

const emit = defineEmits<{ toggle: [key: string, checked: boolean] }>()

// Re-emite o toggle da grade interna (AdminRolePermissionMatrix) para o host, sem
// arrow inline no template (evita realocacao por render e fica explicito).
function onToggle(key: string, checked: boolean) {
  emit('toggle', key, checked)
}

// Filtros client-side (NAO mudam o contrato de salvar — fonte e o host).
const search = ref('')
// Sentinela 'all' (regra do projeto: nunca value=''); convertida na borda.
const moduleFilter = ref('all')

// Modulos distintos do catalogo, para o filtro segmentado.
const moduleOptions = computed(() => {
  const counts = new Map<string, number>()
  for (const perm of props.available || []) {
    const moduleId = String(perm.moduleId || 'outros')
    counts.set(moduleId, (counts.get(moduleId) || 0) + 1)
  }
  const modules = [...counts.entries()]
    .sort((a, b) => a[0].localeCompare(b[0], 'pt-BR'))
    .map(([value, count]) => ({ value, label: value, count }))
  return [{ value: 'all', label: 'Todos', count: props.available?.length || 0 }, ...modules]
})

// Permissoes filtradas por busca (label OU key) + modulo, depois agrupadas por
// moduleId para a grade. So o que passa no filtro entra na grade visivel.
const filteredGroups = computed(() => {
  const term = search.value.trim().toLowerCase()
  const onlyModule = moduleFilter.value === 'all' ? '' : moduleFilter.value
  const groups = new Map<string, AvailablePermission[]>()
  for (const perm of props.available || []) {
    const moduleId = String(perm.moduleId || 'outros')
    if (onlyModule && moduleId !== onlyModule) continue
    if (
      term &&
      !perm.label.toLowerCase().includes(term) &&
      !perm.key.toLowerCase().includes(term)
    ) {
      continue
    }
    if (!groups.has(moduleId)) groups.set(moduleId, [])
    groups.get(moduleId)!.push(perm)
  }
  return [...groups.entries()]
    .map(([moduleId, items]) => ({
      moduleId,
      items: [...items].sort((a, b) => a.label.localeCompare(b.label, 'pt-BR')),
    }))
    .sort((a, b) => a.moduleId.localeCompare(b.moduleId, 'pt-BR'))
})

// Resumo read-only (template de sistema): so as permissoes concedidas, agrupadas.
const grantedGroups = computed(() => {
  const groups = new Map<string, AvailablePermission[]>()
  for (const perm of props.available || []) {
    if (!props.checkedKeys.has(perm.key)) continue
    const moduleId = String(perm.moduleId || 'outros')
    if (!groups.has(moduleId)) groups.set(moduleId, [])
    groups.get(moduleId)!.push(perm)
  }
  return [...groups.entries()]
    .map(([moduleId, items]) => ({
      moduleId,
      items: [...items].sort((a, b) => a.label.localeCompare(b.label, 'pt-BR')),
    }))
    .sort((a, b) => a.moduleId.localeCompare(b.moduleId, 'pt-BR'))
})

const checkedCount = computed(() => props.checkedKeys.size)
</script>

<template>
  <div class="role-template-matrix">
    <template v-if="readonly">
      <p class="role-template-matrix__readonly-note">
        Permissoes concedidas ({{ checkedCount }}). Template de sistema: somente leitura.
      </p>
      <div v-if="!grantedGroups.length" class="role-template-matrix__empty">
        Este template nao concede nenhuma permissao.
      </div>
      <div
        v-for="group in grantedGroups"
        :key="group.moduleId"
        class="role-template-matrix__readonly-group"
      >
        <h5 class="role-template-matrix__readonly-title">{{ group.moduleId }}</h5>
        <ul class="role-template-matrix__readonly-list">
          <li
            v-for="perm in group.items"
            :key="perm.key"
            class="role-template-matrix__readonly-item"
          >
            <strong>{{ perm.label }}</strong>
            <span class="role-template-matrix__readonly-key">{{ perm.key }}</span>
          </li>
        </ul>
      </div>
    </template>

    <template v-else>
      <div class="role-template-matrix__toolbar">
        <AppSearchInput
          v-model="search"
          class="role-template-matrix__search"
          placeholder="Buscar permissao (nome ou chave)..."
          aria-label="Buscar permissao"
        />
        <AppSegmentedFilter
          v-model="moduleFilter"
          :options="moduleOptions"
          aria-label="Filtrar por modulo"
        />
      </div>
      <p class="role-template-matrix__count">{{ checkedCount }} permissao(oes) marcada(s).</p>
      <AdminRolePermissionMatrix
        :groups="filteredGroups"
        :checked-keys="checkedKeys"
        @toggle="onToggle"
      />
    </template>
  </div>
</template>

<style scoped>
.role-template-matrix {
  display: grid;
  gap: 0.6rem;
}

.role-template-matrix__toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.6rem;
}

.role-template-matrix__search {
  flex: 1;
  min-width: 14rem;
}

.role-template-matrix__count,
.role-template-matrix__readonly-note {
  margin: 0;
  font-size: 0.74rem;
  color: rgb(var(--muted));
}

.role-template-matrix__empty {
  margin: 0;
  font-size: 0.8rem;
  color: rgb(var(--muted));
}

.role-template-matrix__readonly-group {
  display: grid;
  gap: 0.35rem;
}

.role-template-matrix__readonly-title {
  margin: 0;
  font-size: 0.74rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: rgb(var(--muted));
}

.role-template-matrix__readonly-list {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(15rem, 1fr));
  gap: 0.35rem;
  margin: 0;
  padding: 0;
  list-style: none;
}

.role-template-matrix__readonly-item {
  display: grid;
  gap: 0.1rem;
  padding: 0.4rem 0.55rem;
  border-radius: var(--radius-xs);
  border: 1px solid rgb(var(--border) / 0.7);
  background: rgb(var(--surface-2) / 0.6);
  min-width: 0;
}

.role-template-matrix__readonly-item strong {
  font-size: 0.78rem;
  font-weight: 600;
  color: rgb(var(--text));
}

.role-template-matrix__readonly-key {
  font-size: 0.68rem;
  color: rgb(var(--muted));
  word-break: break-word;
}
</style>
