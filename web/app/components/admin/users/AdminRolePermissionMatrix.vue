<script setup lang="ts">
import type { AvailablePermission } from '~/types/admin-users'

// Grade de checkboxes da matriz de permissoes de um papel, agrupada por modulo.
// 100% APRESENTACAO: recebe os grupos ja montados + o conjunto de keys marcadas e
// emite toggle(key, checked). O estado (selecao, persistencia) vive no editor pai.
// Fatiado do AdminRoleMatrixEditor para o host nao estourar o limite de linhas;
// comportamento e marcacao IDENTICOS ao bloco inline anterior.

defineProps<{
  groups: { moduleId: string; items: AvailablePermission[] }[]
  // Conjunto de keys marcadas (passado pelo pai; o predicate evita prop por-item).
  checkedKeys: Set<string>
}>()
const emit = defineEmits<{ toggle: [key: string, checked: boolean] }>()
</script>

<template>
  <div v-if="!groups.length" class="role-matrix-perms__empty">
    Nenhuma permissao disponivel no catalogo deste cliente.
  </div>

  <div v-for="group in groups" :key="group.moduleId" class="role-matrix-perms__group">
    <h5 class="role-matrix-perms__group-title">{{ group.moduleId }}</h5>
    <div class="role-matrix-perms__perms">
      <label v-for="perm in group.items" :key="perm.key" class="role-matrix-perms__perm">
        <input
          type="checkbox"
          :checked="checkedKeys.has(perm.key)"
          @change="emit('toggle', perm.key, ($event.target as HTMLInputElement).checked)"
        />
        <span class="role-matrix-perms__perm-copy">
          <strong>{{ perm.label }}</strong>
          <span class="role-matrix-perms__perm-key">{{ perm.key }}</span>
        </span>
      </label>
    </div>
  </div>
</template>

<style scoped>
.role-matrix-perms__empty {
  margin: 0;
  font-size: 0.8rem;
  color: rgb(var(--muted));
}

.role-matrix-perms__group {
  display: grid;
  gap: 0.45rem;
}

.role-matrix-perms__group-title {
  margin: 0;
  font-size: 0.76rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: rgb(var(--muted));
}

.role-matrix-perms__perms {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(15rem, 1fr));
  gap: 0.45rem;
}

.role-matrix-perms__perm {
  display: flex;
  align-items: flex-start;
  gap: 0.5rem;
  padding: 0.45rem 0.55rem;
  border-radius: var(--radius-xs);
  border: 1px solid rgb(var(--border) / 0.7);
  background: rgb(var(--surface-2) / 0.6);
  cursor: pointer;
}

.role-matrix-perms__perm input {
  margin-top: 0.15rem;
}

.role-matrix-perms__perm-copy {
  display: grid;
  gap: 0.1rem;
  min-width: 0;
}

.role-matrix-perms__perm-copy strong {
  font-size: 0.78rem;
  font-weight: 600;
  color: rgb(var(--text));
}

.role-matrix-perms__perm-key {
  font-size: 0.68rem;
  color: rgb(var(--muted));
  word-break: break-word;
}
</style>
