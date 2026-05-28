<script setup>
import { computed } from 'vue'
import { storeToRefs } from 'pinia'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import { useAuthStore } from '~/stores/auth'
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
const { isAuthenticated } = storeToRefs(auth)

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

.workspace-nav-context__profile-select :deep(.app-select-field__trigger) {
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
