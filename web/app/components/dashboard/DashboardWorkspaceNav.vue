<script setup>
import { computed } from 'vue'
import { storeToRefs } from 'pinia'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import { useAuthStore } from '~/stores/auth'
import { useOperationsStore } from '~/stores/operations'
import { QUEUE_WORKSPACES } from '~/utils/workspaces'

const props = defineProps({
  activeWorkspace: {
    type: String,
    required: true,
  },
  allowedWorkspaces: {
    type: Array,
    required: true,
  },
  state: {
    type: Object,
    default: null,
  },
  showOperationsContext: {
    type: Boolean,
    default: false,
  },
})

const emit = defineEmits(['store-change', 'profile-change'])
const auth = useAuthStore()
const operationsStore = useOperationsStore()
const { isAuthenticated, canUseAllStores, storeContext } = storeToRefs(auth)
const { integratedStoreId } = storeToRefs(operationsStore)

// O filtro de loja so existe na PROPRIA pagina de operacao. O nav shell tambem
// renderiza nas rotas filhas `/operacao/clientes` e `/operacao/usuarios` (mesmo
// prefixo), entao gateamos pelo workspace ativo exato, nao pelo path.
const isOperationWorkspace = computed(() => props.activeWorkspace === 'operacao')

// Filtro de loja do modo "Todas as lojas": so faz sentido para quem enxerga
// mais de uma loja. Escreve no operations store, lido pela pagina de operacao.
const storeFilterOptions = computed(() => [
  { value: '', label: 'Todas as lojas', meta: 'Sem filtro aplicado' },
  ...(storeContext.value || []).map((store) => ({
    value: String(store?.id || '').trim(),
    label: String(store?.name || '').trim(),
    meta: [String(store?.code || '').trim(), String(store?.city || '').trim()]
      .filter(Boolean)
      .join(' · '),
  })),
])

function handleStoreFilterChange(value) {
  operationsStore.setIntegratedStoreId(value)
}

const visibleWorkspaces = computed(() =>
  QUEUE_WORKSPACES.filter((workspace) => props.allowedWorkspaces.includes(workspace.id)).filter(
    (workspace) => workspace.id !== 'themes',
  ),
)
const activeServicesCount = computed(() => props.state?.activeServices?.length || 0)
const profileSelectOptions = computed(() =>
  (props.state?.profiles || []).map((profile) => ({
    value: String(profile.id || '').trim(),
    label: String(profile.name || '').trim(),
  })),
)

function handleProfileChange(value) {
  emit('profile-change', String(value || '').trim())
}
</script>

<template>
  <div class="workspace-nav-shell">
    <nav class="workspace-nav" aria-label="Areas do sistema">
      <NuxtLink
        v-for="workspace in visibleWorkspaces"
        :key="workspace.id"
        :to="workspace.path"
        class="workspace-nav__button"
        :class="{ 'workspace-nav__button--active': workspace.id === activeWorkspace }"
        :title="workspace.label"
      >
        <span class="material-icons-round workspace-nav__icon">{{ workspace.icon }}</span>
        <span class="workspace-nav__label">{{ workspace.label }}</span>
      </NuxtLink>
    </nav>

    <div v-if="showOperationsContext && state" class="workspace-nav-context">
      <AppSelectField
        v-if="isOperationWorkspace && canUseAllStores"
        class="summary-select workspace-nav-context__store-select"
        :model-value="integratedStoreId"
        :options="storeFilterOptions"
        placeholder="Todas as lojas"
        :show-leading-icon="false"
        compact
        testid="operation-filter-integrated-store"
        @update:model-value="handleStoreFilterChange"
      />
      <span class="summary-pill">{{ state.waitingList.length }} na fila</span>
      <span class="summary-pill" :class="{ 'summary-pill--active': activeServicesCount > 0 }">
        {{ activeServicesCount }}/{{ state.settings.maxConcurrentServices }} em atendimento
      </span>
      <span class="summary-pill">{{ state.serviceHistory.length }} finalizados</span>
      <AppSelectField
        v-if="!isAuthenticated"
        class="summary-select workspace-nav-context__profile-select"
        :model-value="state.activeProfileId"
        :options="profileSelectOptions"
        placeholder="Selecionar perfil"
        :show-leading-icon="false"
        compact
        @update:model-value="handleProfileChange"
      />
    </div>
  </div>
</template>

<style scoped>
.workspace-nav-shell {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  min-width: 0;
  width: 100%;

  padding-bottom: 0.75rem;
}

.workspace-nav-shell .workspace-nav {
  min-width: 0;
  flex: 1 1 auto;
  margin-bottom: 0;
}

.workspace-nav-context {
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.55rem;
  min-width: 0;
}

.workspace-nav-context__profile-select {
  width: 12rem;
}

.workspace-nav-context__store-select {
  width: 14rem;
}

.workspace-nav-context__profile-select :deep(.app-select-field__trigger),
.workspace-nav-context__store-select :deep(.app-select-field__trigger) {
  min-height: 2.45rem;
  padding: 0 0.82rem;
  border-radius: 999px;
  border-color: var(--admin-header-border);
  background: var(--admin-header-hover-bg);
  color: var(--admin-header-text);
}

@media (max-width: 1180px) {
  .workspace-nav-shell {
    align-items: stretch;
    flex-direction: column;
  }

  .workspace-nav-context {
    justify-content: flex-start;
    overflow-x: auto;
    padding-bottom: 0.25rem;
  }
}

@media (max-width: 720px) {
  .workspace-nav-context {
    flex-wrap: wrap;
  }

  .workspace-nav-context__store-select,
  .workspace-nav-context__profile-select {
    width: min(100%, 16rem);
  }
}
</style>
