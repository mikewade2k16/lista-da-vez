<script setup>
import { computed, ref } from 'vue'
import { storeToRefs } from 'pinia'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import DashboardThemeSwitcher from '~/components/dashboard/DashboardThemeSwitcher.vue'
import DashboardHeaderNav from '~/components/dashboard/DashboardHeaderNav.vue'
import DashboardHeaderProfileMenu from '~/components/dashboard/DashboardHeaderProfileMenu.vue'
import DashboardHeaderDrawer from '~/components/dashboard/DashboardHeaderDrawer.vue'
import SystemNotificationsDropdown from '~/components/notifications/SystemNotificationsDropdown.vue'
import CoreAccountSwitcher from '../../../layers/core/components/CoreAccountSwitcher.vue'
import { useCoreAccountStore } from '../../../layers/core/stores/account'
import { useAuthStore } from '~/stores/auth'

const props = defineProps({
  state: {
    type: Object,
    required: true,
  },
  showOperationsContext: {
    type: Boolean,
    default: true,
  },
  activeWorkspace: {
    type: String,
    default: '',
  },
  allowedWorkspaces: {
    type: Array,
    default: () => [],
  },
})

const emit = defineEmits(['store-change', 'profile-change'])
const auth = useAuthStore()
const accountStore = useCoreAccountStore()
const { isAuthenticated, role } = storeToRefs(auth)

// Seletor de conta (switcher v2) so aparece quando ha mais de uma account
// acessivel — agencia/platform_admin. Cliente de conta unica nao ve o botao.
const hasMultipleAccounts = computed(() => accountStore.accounts.length > 1)
const sidebarOpen = ref(false)

const activeServicesCount = computed(() => props.state.activeServices?.length || 0)
const profileSelectOptions = computed(() =>
  (props.state.profiles || []).map((profile) => ({
    value: String(profile.id || '').trim(),
    label: String(profile.name || '').trim(),
  })),
)

function handleProfileChange(value) {
  emit('profile-change', String(value || '').trim())
}
</script>

<template>
  <header class="app-header dashboard-header">
    <div class="brand-bar dashboard-header__bar">
      <div class="brand dashboard-header__brand">
        <UButton
          icon="i-lucide-menu"
          color="neutral"
          variant="ghost"
          size="sm"
          aria-label="Abrir sidebar"
          @click="sidebarOpen = true"
        />
        <picture class="dashboard-header__logo" aria-label="Logo da plataforma">
          <source srcset="/logo.avif" type="image/avif" />
          <source srcset="/logo.webp" type="image/webp" />
          <img src="/logo.png" alt="Logo da plataforma" />
        </picture>
      </div>

      <DashboardHeaderNav
        :active-workspace="activeWorkspace"
        :allowed-workspaces="allowedWorkspaces"
      />

      <div class="brand__meta dashboard-header__meta">
        <span v-if="showOperationsContext" class="summary-pill">
          {{ state.waitingList.length }} na fila
        </span>
        <span
          v-if="showOperationsContext"
          class="summary-pill"
          :class="{ 'summary-pill--active': activeServicesCount > 0 }"
        >
          {{ activeServicesCount }}/{{ state.settings.maxConcurrentServices }} em atendimento
        </span>
        <span v-if="showOperationsContext" class="summary-pill">
          {{ state.serviceHistory.length }} finalizados
        </span>
        <ClientOnly>
          <div class="dashboard-header__client-actions">
            <AppSelectField
              v-if="showOperationsContext && !isAuthenticated"
              class="summary-select"
              :model-value="state.activeProfileId"
              :options="profileSelectOptions"
              placeholder="Selecionar perfil"
              :show-leading-icon="false"
              compact
              @update:model-value="handleProfileChange"
            />
            <DashboardThemeSwitcher />
            <SystemNotificationsDropdown v-if="isAuthenticated" />
            <CoreAccountSwitcher
              v-if="isAuthenticated && hasMultipleAccounts"
              :can-enter-platform-view="role === 'platform_admin'"
            />
            <DashboardHeaderProfileMenu v-if="isAuthenticated" />
          </div>
        </ClientOnly>
      </div>
    </div>

    <DashboardHeaderDrawer
      v-model:open="sidebarOpen"
      :active-workspace="activeWorkspace"
      :allowed-workspaces="allowedWorkspaces"
    />
  </header>
</template>

<style scoped>
.dashboard-header {
  position: relative;
  z-index: 9500;
  flex: 0 0 auto;
  overflow: visible;
  background: var(--admin-header-panel-bg);
  border-bottom: 1px solid var(--admin-header-border);
  box-shadow: var(--admin-header-shell-shadow);
  color: var(--admin-header-text);
  backdrop-filter: blur(var(--admin-header-panel-blur));
}

.dashboard-header__bar {
  width: min(100%, 1400px);
  width: min(100%, 95%);
  gap: 1rem;
  padding: 0.3rem 1rem;
  overflow: visible;
}

.dashboard-header__brand {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  min-width: 0;
  flex: 0 0 auto;
}

.dashboard-header__logo {
  display: inline-flex;
  width: clamp(4.5rem, 8vw, 5.6rem);
  max-width: 100%;
}

.dashboard-header__logo img {
  display: block;
  width: 100%;
  height: auto;
}

.dashboard-header__meta {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.7rem;
  flex-wrap: wrap;
  flex: 0 0 auto;
}

.dashboard-header__client-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.7rem;
  flex-wrap: wrap;
}

@media (max-width: 900px) {
  .dashboard-header__bar {
    align-items: center;
    flex-direction: row;
    flex-wrap: nowrap;
    gap: 0.45rem;
    padding-block: 0.45rem;
  }

  .dashboard-header__brand {
    justify-content: flex-start;
    gap: 0.4rem;
  }

  /* No mobile toda a navegacao vive no drawer. O header fica em uma unica
     linha: menu, logo, acoes, seletor de conta e perfil. */
  .dashboard-header__bar :deep(.dashboard-header__nav) {
    display: none;
  }

  .dashboard-header__meta {
    min-width: 0;
    margin-left: auto;
    flex: 1 1 auto;
    flex-wrap: nowrap;
    justify-content: flex-end;
    gap: 0.4rem;
  }

  .dashboard-header__client-actions {
    width: 100%;
    min-width: 0;
    flex-wrap: nowrap;
    gap: 0.38rem;
  }

  .dashboard-header__client-actions :deep(.core-account-switcher) {
    min-width: 5.5rem;
    max-width: 9.5rem;
    flex: 1 1 8rem;
  }

  .dashboard-header__client-actions :deep(.core-account-switcher__trigger) {
    width: 100%;
    min-height: 2.35rem;
    padding: 0.35rem 0.6rem;
    border-radius: 999px;
  }

  .dashboard-header__client-actions :deep(.core-account-switcher__name) {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .dashboard-header__client-actions :deep(.core-account-switcher__menu) {
    right: 0;
    left: auto;
    width: min(17rem, calc(100vw - 1.25rem));
  }

  .dashboard-header__client-actions :deep(.system-notifications__trigger),
  .dashboard-header__client-actions :deep(.dashboard-header__profile-trigger) {
    width: 2.35rem;
    height: 2.35rem;
  }

  .dashboard-header__client-actions :deep(.dashboard-header-avatar) {
    width: 1.9rem;
    height: 1.9rem;
  }
}

@media (max-width: 640px) {
  .dashboard-header__bar {
    width: 100%;
    padding-inline: 0.55rem;
    gap: 0.32rem;
  }

  .dashboard-header__brand {
    gap: 0.25rem;
  }

  .dashboard-header__brand > :deep(button) {
    width: 2.2rem;
    height: 2.2rem;
    padding: 0;
  }

  .dashboard-header__logo {
    width: clamp(3.65rem, 16vw, 4.4rem);
  }

  .dashboard-header__client-actions {
    gap: 0.25rem;
  }

  .dashboard-header__client-actions :deep(.core-account-switcher) {
    min-width: 4.75rem;
    max-width: 8.25rem;
  }

  .dashboard-header__client-actions :deep(.core-account-switcher__trigger) {
    min-height: 2.2rem;
    gap: 0.25rem;
    padding-inline: 0.48rem;
    font-size: 0.74rem;
  }

  .dashboard-header__client-actions :deep(.system-notifications__trigger),
  .dashboard-header__client-actions :deep(.dashboard-header__profile-trigger) {
    width: 2.2rem;
    height: 2.2rem;
  }

  .dashboard-header__client-actions :deep(.dashboard-header-avatar) {
    width: 1.78rem;
    height: 1.78rem;
  }

  .dashboard-header__client-actions :deep(.dashboard-header__profile-dropdown) {
    right: 0;
    left: auto;
    width: min(18.5rem, calc(100vw - 1rem));
  }
}
</style>
