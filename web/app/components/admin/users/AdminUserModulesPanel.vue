<script setup lang="ts">
import type { AccountMembershipItem, AdminUserItem } from '~/types/admin-users'
import OmniCollapse from '~/components/omni/OmniCollapse.vue'
import AppSearchInput from '~/components/ui/AppSearchInput.vue'
import AppSegmentedFilter from '~/components/ui/AppSegmentedFilter.vue'
import AdminModuleGroupActions from '~/components/admin/users/AdminModuleGroupActions.vue'
import AdminTriStateControl from '~/components/admin/users/AdminTriStateControl.vue'
import { useModuleOverridesEditor } from '~/composables/useModuleOverridesEditor'

// Painel "Modulos". Espelha a UX de overrides do UsersAccessPermissionPanel legado,
// mas batendo no core (useAdminUsersManager.getOverrides/setOverrides). Cada
// permissao tem tri-estado: Herdar (sem override) / Permitir (allow) / Negar (deny).
// O estado inicial vem dos overrides ativos do usuario; Herdar = nao mandar entrada.
// Escopo = cliente OU conta-agencia (organizacao): assim platform_admin/agency_owner
// ajustam modulos tambem de usuarios "sem cliente". A conta-agencia tem todos os
// modulos habilitados (migration 0158), entao o catalogo `available` vem completo.
//
// O ESTADO de edicao (rascunho/snapshot) e o estado de VIEW (busca/filtros/lote)
// ficam no composable useModuleOverridesEditor; este componente so carrega/salva
// pela API e renderiza. Busca/filtro/lote sao client-side e NAO mudam o contrato de
// salvar (PUT .../overrides faz replace do tri-estado).
const props = defineProps<{ user: AdminUserItem }>()
const emit = defineEmits<{ updated: [] }>()

const m = useAdminUsersManager()
const editor = useModuleOverridesEditor()

const memberships = ref<AccountMembershipItem[]>([])
const loadingMemberships = ref(false)
const loadingOverrides = ref(false)
const selectedAccountId = ref('')

// Escopos editaveis: clientes E conta-agencia (organizacao). Overrides de modulo
// sao por account_id; a conta-agencia (is_agency) tambem recebe overrides, o que
// permite ajustar modulos de usuarios "sem cliente". Clientes primeiro, agencia
// depois.
const scopeMemberships = computed(() =>
  [...memberships.value].sort((a, b) => Number(a.isAgency) - Number(b.isAgency)),
)
const hasScopes = computed(() => scopeMemberships.value.length > 0)

function scopeLabel(mb: AccountMembershipItem): string {
  return mb.isAgency ? `${mb.accountName} (Organizacao)` : mb.accountName
}

const savingOverrides = computed(() =>
  Boolean(m.savingMap.value[`${props.user.id}:overrides:${selectedAccountId.value}`]),
)

async function loadMemberships() {
  loadingMemberships.value = true
  memberships.value = await m.fetchMemberships(props.user.id)
  loadingMemberships.value = false
  // Default: primeiro escopo (cliente ou organizacao). Sem escopos, nada a ajustar.
  selectedAccountId.value = scopeMemberships.value[0]?.accountId ?? ''
}

async function loadOverrides() {
  if (!selectedAccountId.value) {
    editor.reset()
    return
  }
  loadingOverrides.value = true
  const data = await m.getOverrides(props.user.id, selectedAccountId.value)
  loadingOverrides.value = false
  if (!data) {
    editor.reset()
    return
  }
  // Re-hidrata rascunho + snapshot a partir da resposta autoritativa do backend.
  editor.hydrate(data.available, data.overrides)
}

watch(() => props.user.id, loadMemberships, { immediate: true })
// Trocar de escopo zera a view (busca/filtros) para nao confundir entre clientes.
watch(selectedAccountId, () => {
  editor.clearView()
  loadOverrides()
})

async function save() {
  if (!selectedAccountId.value) return
  const result = await m.setOverrides(props.user.id, selectedAccountId.value, editor.buildPayload())
  if (result) {
    // Fonte de verdade = retorno do backend; re-hidrata rascunho + snapshot.
    editor.hydrate(result.available, result.overrides)
    emit('updated')
  }
}
</script>

<template>
  <section class="admin-user-modules">
    <UAlert
      v-if="m.errorMessage.value"
      class="admin-user-modules__error"
      color="error"
      variant="soft"
      icon="i-lucide-alert-triangle"
      :description="m.errorMessage.value"
    />

    <p v-if="loadingMemberships" class="admin-user-modules__muted">Carregando clientes...</p>

    <UAlert
      v-else-if="!hasScopes"
      color="neutral"
      variant="soft"
      icon="i-lucide-info"
      description="Vincule um cliente ou uma organizacao ao usuario (aba Vinculos) antes de ajustar modulos."
    />

    <template v-else>
      <label class="admin-user-modules__account">
        <span class="admin-user-modules__label">Cliente / Organizacao</span>
        <select v-model="selectedAccountId" class="admin-user-modules__select">
          <option v-for="mb in scopeMemberships" :key="mb.accountId" :value="mb.accountId">
            {{ scopeLabel(mb) }}
          </option>
        </select>
      </label>

      <p class="admin-user-modules__legend">
        <strong>Herdar</strong>
        usa o que os papeis do usuario ja concedem.
        <strong>Permitir</strong>
        /
        <strong>Negar</strong>
        sobrescrevem so este usuario neste cliente.
      </p>

      <p v-if="loadingOverrides" class="admin-user-modules__muted">Carregando permissoes...</p>

      <p v-else-if="editor.groups.value.length === 0" class="admin-user-modules__muted">
        Nenhum modulo habilitado neste cliente para ajustar.
      </p>

      <template v-else>
        <div class="admin-user-modules__toolbar">
          <AppSearchInput
            :model-value="editor.searchTerm.value"
            placeholder="Buscar por nome ou chave (ex.: tasks.boards.manage)"
            aria-label="Buscar permissoes"
            @update:model-value="editor.searchTerm.value = $event"
          />
          <div class="admin-user-modules__filters">
            <AppSegmentedFilter
              :model-value="editor.effectFilter.value"
              :options="editor.effectFilterOptions.value"
              aria-label="Filtrar por efeito"
              @update:model-value="editor.setEffectFilter($event)"
            />
            <label class="admin-user-modules__module-filter">
              <span class="admin-user-modules__label">Modulo</span>
              <select v-model="editor.moduleFilter.value" class="admin-user-modules__select">
                <option
                  v-for="opt in editor.moduleFilterOptions.value"
                  :key="opt.value"
                  :value="opt.value"
                >
                  {{ opt.label }}
                </option>
              </select>
            </label>
          </div>
        </div>

        <p v-if="editor.visibleGroups.value.length === 0" class="admin-user-modules__muted">
          Nenhuma permissao corresponde a busca/filtro.
          <button type="button" class="admin-user-modules__link" @click="editor.clearView">
            Limpar
          </button>
        </p>

        <div v-else class="admin-user-modules__groups">
          <OmniCollapse
            v-for="group in editor.visibleGroups.value"
            :key="group.moduleId"
            :title="group.moduleId"
            :summary="editor.groupSummary(group.permissions)"
            :default-open="editor.hasActiveView.value"
          >
            <AdminModuleGroupActions
              :visible-count="group.permissions.length"
              :dirty="editor.isModuleDirty(group.moduleId)"
              :disabled="loadingOverrides || savingOverrides"
              @apply="editor.applyBulkToModule(group.moduleId, $event)"
              @restore="editor.restoreModule(group.moduleId)"
            />

            <ul class="admin-user-modules__perms">
              <li
                v-for="perm in group.permissions"
                :key="perm.key"
                class="admin-user-modules__perm"
                :class="{ 'admin-user-modules__perm--dirty': editor.isRowDirty(perm.key) }"
              >
                <div class="admin-user-modules__perm-copy">
                  <span class="admin-user-modules__perm-label">{{ perm.label }}</span>
                  <span class="admin-user-modules__perm-key">{{ perm.key }}</span>
                </div>
                <AdminTriStateControl
                  :model-value="editor.states[perm.key]"
                  :aria-label="perm.label"
                  @update="editor.setState(perm.key, $event)"
                />
              </li>
            </ul>
          </OmniCollapse>
        </div>

        <div class="admin-user-modules__foot">
          <span class="admin-user-modules__count">
            {{ editor.overrideCount.value }} override(s) explicito(s); o restante herda dos papeis.
            <span v-if="editor.isDirty.value" class="admin-user-modules__pending">pendente</span>
          </span>
          <div class="admin-user-modules__foot-actions">
            <button
              type="button"
              class="admin-user-modules__link"
              :disabled="!editor.isDirty.value || savingOverrides"
              @click="editor.restoreSaved"
            >
              Restaurar tudo
            </button>
            <UButton
              label="Salvar modulos"
              color="primary"
              :loading="savingOverrides"
              :disabled="loadingOverrides || !editor.isDirty.value"
              @click="save"
            />
          </div>
        </div>
      </template>
    </template>
  </section>
</template>

<style scoped>
.admin-user-modules {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.admin-user-modules__error {
  margin: 0;
}

.admin-user-modules__muted {
  margin: 0;
  font-size: 0.8rem;
  color: rgb(var(--muted));
}

.admin-user-modules__account {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  max-width: 22rem;
}

.admin-user-modules__label {
  font-size: 0.78rem;
  color: rgb(var(--muted));
}

.admin-user-modules__select {
  width: 100%;
  padding: 0.45rem 0.55rem;
  font-size: 0.85rem;
  border-radius: var(--radius-md);
  border: 1px solid rgb(var(--border));
  background: rgb(var(--surface));
  color: rgb(var(--text));
}

.admin-user-modules__legend {
  margin: 0;
  font-size: 0.76rem;
  color: rgb(var(--muted));
}

.admin-user-modules__legend strong {
  color: rgb(var(--text));
}

.admin-user-modules__toolbar {
  display: flex;
  flex-direction: column;
  gap: 0.55rem;
}

.admin-user-modules__filters {
  display: flex;
  align-items: flex-end;
  gap: 0.6rem;
  flex-wrap: wrap;
}

.admin-user-modules__module-filter {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  min-width: 12rem;
}

.admin-user-modules__link {
  border: 0;
  background: none;
  padding: 0;
  color: rgb(var(--primary-600));
  font-size: inherit;
  font-weight: 600;
  cursor: pointer;
  text-decoration: underline;
}

.admin-user-modules__link:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.admin-user-modules__foot-actions {
  display: inline-flex;
  align-items: center;
  gap: 0.85rem;
}

.admin-user-modules__pending {
  margin-left: 0.4rem;
  font-size: 0.68rem;
  font-weight: 700;
  padding: 0.05rem 0.45rem;
  border-radius: 999px;
  background: rgb(var(--primary) / 0.16);
  color: rgb(var(--primary-600));
}

.admin-user-modules__groups {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.admin-user-modules__perms {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
}

.admin-user-modules__perm {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.5rem 0.6rem;
  border-radius: var(--radius-md);
  border-left: 2px solid transparent;
  background: rgb(var(--surface-2));
}

/* Realca a permissao com edicao pendente (difere do ultimo estado salvo). */
.admin-user-modules__perm--dirty {
  border-left-color: rgb(var(--primary));
}

.admin-user-modules__perm-copy {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 0.1rem;
}

.admin-user-modules__perm-label {
  font-size: 0.83rem;
  font-weight: 500;
  color: rgb(var(--text));
}

.admin-user-modules__perm-key {
  font-size: 0.7rem;
  color: rgb(var(--muted));
}

.admin-user-modules__foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.admin-user-modules__count {
  font-size: 0.76rem;
  color: rgb(var(--muted));
}
</style>
