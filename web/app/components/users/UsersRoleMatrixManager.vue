<script setup>
import { computed, ref, watch } from "vue";

import AppSelectField from "~/components/ui/AppSelectField.vue";
import AppToggleSwitch from "~/components/ui/AppToggleSwitch.vue";
import {
  ADVANCED_ACCESS_DEFINITIONS,
  WORKSPACE_ACCESS_DEFINITIONS,
  canManageRoleDefaults,
  getRoleLabel,
  getWorkspaceAccessOptions,
  hasPermission,
  normalizePermissionKeys,
  readWorkspaceAccessState,
  writeWorkspaceAccessState
} from "~/domain/utils/permissions";
import { useAccessControlStore } from "~/stores/access-control";
import { useAuthStore } from "~/stores/auth";
import { useUiStore } from "~/stores/ui";

const auth = useAuthStore();
const ui = useUiStore();
const accessStore = useAccessControlStore();

const roleDrafts = ref({});
const expandedRoleIds = ref([]);
const savingRoleId = ref("");
const loadError = ref("");

const canEditRoleMatrix = computed(() =>
  canManageRoleDefaults(auth.role, auth.permissionKeys, auth.permissionsResolved)
);

const sortedRoles = computed(() =>
  [...accessStore.roleMatrix].sort((left, right) => getRoleLabel(left.role).localeCompare(getRoleLabel(right.role), "pt-BR"))
);

function createRoleDraft(entry) {
  return {
    permissionKeys: normalizePermissionKeys(entry?.permissionKeys || [])
  };
}

function getRoleDraft(roleId) {
  const normalizedRoleId = String(roleId || "").trim();
  if (!roleDrafts.value[normalizedRoleId]) {
    const entry = accessStore.roleLookup.get(normalizedRoleId);
    roleDrafts.value[normalizedRoleId] = createRoleDraft(entry);
  }

  return roleDrafts.value[normalizedRoleId];
}

function syncRoleDrafts() {
  const nextDrafts = {};

  for (const entry of accessStore.roleMatrix) {
    nextDrafts[entry.role] = createRoleDraft(entry);
  }

  roleDrafts.value = nextDrafts;

  if (!expandedRoleIds.value.length && accessStore.roleMatrix.length) {
    expandedRoleIds.value = [accessStore.roleMatrix[0].role];
  }
}

function isExpanded(roleId) {
  return expandedRoleIds.value.includes(String(roleId || "").trim());
}

function toggleRoleCard(roleId) {
  const normalizedRoleId = String(roleId || "").trim();
  if (!normalizedRoleId) {
    return;
  }

  expandedRoleIds.value = isExpanded(normalizedRoleId)
    ? expandedRoleIds.value.filter((entryRoleId) => entryRoleId !== normalizedRoleId)
    : [...expandedRoleIds.value, normalizedRoleId];
}

function getRoleSummary(entry) {
  const permissionKeys = getRoleDraft(entry.role).permissionKeys;
  const visibleWorkspaces = WORKSPACE_ACCESS_DEFINITIONS.filter((workspaceDefinition) =>
    readWorkspaceAccessState(workspaceDefinition, permissionKeys, "none") !== "none"
  ).length;
  const editableWorkspaces = WORKSPACE_ACCESS_DEFINITIONS.filter((workspaceDefinition) =>
    readWorkspaceAccessState(workspaceDefinition, permissionKeys, "none") === "edit"
  ).length;
  const advancedPermissions = ADVANCED_ACCESS_DEFINITIONS.filter((permissionDefinition) =>
    hasPermission(permissionKeys, permissionDefinition.key)
  ).length;

  return {
    visibleWorkspaces,
    editableWorkspaces,
    advancedPermissions
  };
}

function getWorkspaceState(roleId, workspaceDefinition) {
  return readWorkspaceAccessState(workspaceDefinition, getRoleDraft(roleId).permissionKeys, "none");
}

function updateWorkspaceState(roleId, workspaceDefinition, nextState) {
  const draft = getRoleDraft(roleId);
  draft.permissionKeys = writeWorkspaceAccessState(workspaceDefinition, draft.permissionKeys, nextState);
}

function hasAdvancedAccess(roleId, permissionKey) {
  return hasPermission(getRoleDraft(roleId).permissionKeys, permissionKey);
}

function toggleAdvancedAccess(roleId, permissionKey, nextValue) {
  const draft = getRoleDraft(roleId);
  const nextPermissions = normalizePermissionKeys(draft.permissionKeys).filter((currentKey) => currentKey !== permissionKey);
  if (nextValue) {
    nextPermissions.push(permissionKey);
  }
  draft.permissionKeys = normalizePermissionKeys(nextPermissions);
}

function isDirty(roleId) {
  const currentEntry = accessStore.roleLookup.get(String(roleId || "").trim());
  if (!currentEntry) {
    return false;
  }

  return JSON.stringify(normalizePermissionKeys(currentEntry.permissionKeys)) !== JSON.stringify(getRoleDraft(roleId).permissionKeys);
}

async function saveRole(roleId) {
  if (!canEditRoleMatrix.value || savingRoleId.value) {
    return;
  }

  savingRoleId.value = String(roleId || "").trim();
  const result = await accessStore.saveRolePermissions(roleId, getRoleDraft(roleId).permissionKeys);
  savingRoleId.value = "";

  if (result?.ok === false) {
    ui.error(result.message || "Nao foi possivel salvar o perfil.");
    return;
  }

  ui.success("Padrao do perfil atualizado.");
}

async function loadRoleMatrix() {
  loadError.value = "";
  await accessStore.ensureRoleMatrix();

  if (!accessStore.roleMatrix.length && accessStore.errorMessage) {
    loadError.value = accessStore.errorMessage;
  }
}

watch(
  () => accessStore.roleMatrix,
  () => {
    syncRoleDrafts();
  },
  { immediate: true, deep: true }
);

await loadRoleMatrix();
</script>

<template>
  <section class="users-role-matrix">
    <header class="settings-card users-role-matrix__intro">
      <div class="settings-card__header">
        <div>
          <h3 class="settings-card__title">Matriz padrao por perfil</h3>
          <p class="settings-card__text">
            Defina o que cada tipo de usuario pode ver no painel e o que pode alterar antes dos overrides individuais.
          </p>
        </div>

        <span class="users-role-matrix__summary-pill">{{ loadError ? 'Falha na carga' : `${sortedRoles.length} perfis` }}</span>
      </div>

      <p class="users-role-matrix__intro-note">
        {{ canEditRoleMatrix ? "As alteracoes abaixo viram o padrao para novos acessos e para usuarios sem override." : "Voce esta vendo a matriz em modo leitura. Para editar os padroes, e preciso a permissao de matriz por perfil." }}
      </p>
    </header>

    <div v-if="loadError" class="users-role-matrix__error-card">
      <div>
        <strong>Nao foi possivel carregar os perfis.</strong>
        <p>{{ loadError }}</p>
      </div>

      <button class="users-role-matrix__retry-btn" type="button" @click="loadRoleMatrix">Tentar novamente</button>
    </div>

    <div v-else-if="!sortedRoles.length" class="users-role-matrix__empty-card">
      <strong>Nenhum perfil retornado pela API.</strong>
      <p>Quando a rota de access estiver ativa, a matriz padrao por perfil aparece aqui.</p>
    </div>

    <div v-else class="users-role-matrix__grid">
      <article v-for="entry in sortedRoles" :key="entry.role" class="settings-card users-role-matrix__card">
        <button
          class="users-role-matrix__card-toggle"
          type="button"
          :aria-expanded="isExpanded(entry.role) ? 'true' : 'false'"
          @click="toggleRoleCard(entry.role)"
        >
          <div class="users-role-matrix__card-copy">
            <div>
              <h3 class="settings-card__title">{{ entry.label || getRoleLabel(entry.role) }}</h3>
              <p class="settings-card__text">Escopo {{ entry.scope || "tenant" }}.</p>
            </div>

            <div class="users-role-matrix__card-summary">
              <span class="users-role-matrix__summary-chip">{{ getRoleSummary(entry).visibleWorkspaces }} visoes</span>
              <span class="users-role-matrix__summary-chip">{{ getRoleSummary(entry).editableWorkspaces }} edicoes</span>
              <span class="users-role-matrix__summary-chip">{{ getRoleSummary(entry).advancedPermissions }} sensiveis</span>
            </div>
          </div>

          <div class="users-role-matrix__card-meta">
            <span class="users-role-matrix__role-pill">{{ entry.role }}</span>
            <span class="material-icons-round users-role-matrix__collapse-icon">expand_more</span>
          </div>
        </button>

        <div v-if="isExpanded(entry.role)" class="users-role-matrix__card-body">
          <div class="users-role-matrix__workspace-list">
            <div
              v-for="workspaceDefinition in WORKSPACE_ACCESS_DEFINITIONS"
              :key="`${entry.role}-${workspaceDefinition.id}`"
              class="users-role-matrix__workspace-row"
            >
              <div class="users-role-matrix__workspace-copy">
                <strong>{{ workspaceDefinition.label }}</strong>
                <p>{{ workspaceDefinition.description }}</p>
              </div>

              <AppSelectField
                class="users-role-matrix__select"
                label="Nivel"
                :model-value="getWorkspaceState(entry.role, workspaceDefinition)"
                :options="getWorkspaceAccessOptions(workspaceDefinition)"
                :disabled="!canEditRoleMatrix"
                @update:model-value="updateWorkspaceState(entry.role, workspaceDefinition, $event)"
              />
            </div>
          </div>

          <div class="users-role-matrix__advanced-list">
            <div
              v-for="permissionDefinition in ADVANCED_ACCESS_DEFINITIONS"
              :key="`${entry.role}-${permissionDefinition.key}`"
              class="users-role-matrix__advanced-row"
            >
              <div>
                <strong>{{ permissionDefinition.label }}</strong>
                <p>{{ permissionDefinition.description }}</p>
              </div>

              <AppToggleSwitch
                compact
                :model-value="hasAdvancedAccess(entry.role, permissionDefinition.key)"
                :disabled="!canEditRoleMatrix"
                :label="hasAdvancedAccess(entry.role, permissionDefinition.key) ? 'Ativo' : 'Inativo'"
                @update:model-value="toggleAdvancedAccess(entry.role, permissionDefinition.key, $event)"
              />
            </div>
          </div>

          <footer class="users-role-matrix__card-actions">
            <span class="users-role-matrix__draft-state" :class="{ 'is-dirty': isDirty(entry.role) }">
              {{ isDirty(entry.role) ? "Alteracoes pendentes" : "Sem alteracoes" }}
            </span>

            <button
              class="users-role-matrix__save-btn"
              type="button"
              :disabled="!canEditRoleMatrix || !isDirty(entry.role) || savingRoleId === entry.role"
              @click="saveRole(entry.role)"
            >
              {{ savingRoleId === entry.role ? "Salvando..." : "Salvar perfil" }}
            </button>
          </footer>
        </div>
      </article>
    </div>
  </section>
</template>

<style scoped>
.users-role-matrix {
  display: grid;
  gap: 1rem;
}

.users-role-matrix__intro {
  padding: 1rem;
}

.users-role-matrix__summary-pill,
.users-role-matrix__role-pill {
  display: inline-flex;
  align-items: center;
  min-height: 2rem;
  padding: 0 0.8rem;
  border-radius: 999px;
  background: rgba(129, 140, 248, 0.16);
  color: #dbe4ff;
  font-size: 0.74rem;
  font-weight: 700;
}

.users-role-matrix__intro-note {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.8rem;
}

.users-role-matrix__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(22rem, 1fr));
  gap: 1rem;
}

.users-role-matrix__error-card,
.users-role-matrix__empty-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  flex-wrap: wrap;
  padding: 1rem;
  border-radius: 1rem;
  border: 1px solid rgba(248, 113, 113, 0.2);
  background: rgba(69, 10, 10, 0.2);
}

.users-role-matrix__empty-card {
  border-color: rgba(255, 255, 255, 0.08);
  background: rgba(15, 23, 42, 0.44);
}

.users-role-matrix__error-card strong,
.users-role-matrix__empty-card strong {
  color: #ffffff;
  font-size: 0.85rem;
}

.users-role-matrix__error-card p,
.users-role-matrix__empty-card p {
  margin: 0.24rem 0 0;
  color: var(--text-muted);
  font-size: 0.78rem;
  line-height: 1.45;
}

.users-role-matrix__retry-btn {
  min-height: 2.35rem;
  padding: 0 0.95rem;
  border-radius: 999px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  background: rgba(18, 25, 38, 0.92);
  color: var(--text-main);
  font-weight: 700;
  font-size: 0.78rem;
  cursor: pointer;
}

.users-role-matrix__card {
  display: grid;
  gap: 0;
  overflow: hidden;
}

.users-role-matrix__card-toggle {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.9rem;
  width: 100%;
  padding: 1rem;
  border: 0;
  background: transparent;
  color: inherit;
  text-align: left;
  cursor: pointer;
}

.users-role-matrix__card-copy {
  min-width: 0;
  display: grid;
  gap: 0.75rem;
}

.users-role-matrix__card-summary {
  display: flex;
  gap: 0.45rem;
  flex-wrap: wrap;
}

.users-role-matrix__summary-chip {
  display: inline-flex;
  align-items: center;
  min-height: 1.75rem;
  padding: 0 0.68rem;
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.14);
  color: var(--text-muted);
  font-size: 0.72rem;
  font-weight: 700;
}

.users-role-matrix__card-meta {
  display: inline-flex;
  align-items: center;
  gap: 0.6rem;
  flex-shrink: 0;
}

.users-role-matrix__collapse-icon {
  font-size: 1.35rem;
  color: var(--text-muted);
  transition: transform 0.2s ease;
}

.users-role-matrix__card-toggle[aria-expanded="true"] .users-role-matrix__collapse-icon {
  transform: rotate(180deg);
}

.users-role-matrix__card-body {
  display: grid;
  gap: 1rem;
  padding: 0 1rem 1rem;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
}

.users-role-matrix__workspace-list,
.users-role-matrix__advanced-list {
  display: grid;
  gap: 0.75rem;
}

.users-role-matrix__workspace-row,
.users-role-matrix__advanced-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(10.5rem, 12rem);
  gap: 0.9rem;
  align-items: start;
  padding: 0.9rem;
  border-radius: 1rem;
  border: 1px solid rgba(255, 255, 255, 0.06);
  background: rgba(15, 23, 42, 0.44);
}

.users-role-matrix__workspace-copy,
.users-role-matrix__advanced-row div {
  display: grid;
  gap: 0.22rem;
}

.users-role-matrix__workspace-copy strong,
.users-role-matrix__advanced-row strong {
  color: #ffffff;
  font-size: 0.84rem;
}

.users-role-matrix__workspace-copy p,
.users-role-matrix__advanced-row p {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.76rem;
  line-height: 1.45;
}

.users-role-matrix__select {
  min-width: 0;
}

.users-role-matrix__card-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.users-role-matrix__draft-state {
  color: var(--text-muted);
  font-size: 0.76rem;
}

.users-role-matrix__draft-state.is-dirty {
  color: #fde68a;
}

.users-role-matrix__save-btn {
  min-height: 2.35rem;
  padding: 0 0.95rem;
  border-radius: 999px;
  border: 1px solid rgba(34, 197, 94, 0.28);
  background: rgba(34, 197, 94, 0.16);
  color: #dcfce7;
  font-weight: 700;
  font-size: 0.78rem;
  cursor: pointer;
}

.users-role-matrix__save-btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

@media (max-width: 760px) {
  .users-role-matrix__card-toggle,
  .users-role-matrix__workspace-row,
  .users-role-matrix__advanced-row {
    grid-template-columns: minmax(0, 1fr);
  }

  .users-role-matrix__card-toggle {
    align-items: start;
  }

  .users-role-matrix__card-meta {
    width: 100%;
    justify-content: space-between;
  }

  .users-role-matrix__card-actions {
    align-items: stretch;
  }

  .users-role-matrix__save-btn {
    width: 100%;
  }
}
</style>