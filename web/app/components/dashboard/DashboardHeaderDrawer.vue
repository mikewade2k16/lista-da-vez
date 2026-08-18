<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { X } from 'lucide-vue-next'
import DashboardSidebarNav from '~/components/dashboard/DashboardSidebarNav.vue'
import DashboardThemeSwitcher from '~/components/dashboard/DashboardThemeSwitcher.vue'
import DashboardHeaderAvatar from '~/components/dashboard/DashboardHeaderAvatar.vue'
import SystemNotificationsDropdown from '~/components/notifications/SystemNotificationsDropdown.vue'
import { useDropdownDismiss } from '~/composables/useDropdownDismiss'
import { useHeaderProfile } from '~/composables/useHeaderProfile'
import { useAuthStore } from '~/stores/auth'

const props = defineProps<{
  open: boolean
  activeWorkspace?: string
  allowedWorkspaces?: unknown[]
}>()

const emit = defineEmits<{ 'update:open': [boolean] }>()

const auth = useAuthStore()
const { isAuthenticated } = storeToRefs(auth)
const { displayName, profileRoleLabel, avatarUrl, profileInitial } = useHeaderProfile()

function close() {
  emit('update:open', false)
}

// Fecha no Esc e ao navegar; o clique-fora e tratado pelo backdrop proprio.
useDropdownDismiss(() => props.open, close)
</script>

<template>
  <Teleport to="body">
    <Transition name="dashboard-sidebar-drawer">
      <div
        v-if="open"
        class="dashboard-header__sidebar-drawer"
        role="dialog"
        aria-modal="true"
        aria-label="Menu do sistema"
      >
        <button
          class="dashboard-header__sidebar-backdrop"
          type="button"
          aria-label="Fechar sidebar"
          @click="close"
        ></button>
        <aside class="dashboard-header__sidebar-panel">
          <button
            class="dashboard-header__sidebar-close"
            type="button"
            aria-label="Fechar sidebar"
            @click="close"
          >
            <X :size="18" :stroke-width="2.2" aria-hidden="true" />
          </button>
          <div class="dashboard-header__sidebar-body">
            <DashboardSidebarNav
              class="dashboard-header__drawer-nav"
              :active-workspace="activeWorkspace"
              :allowed-workspaces="allowedWorkspaces"
              always-expanded
            />
          </div>
          <div class="dashboard-header__sidebar-footer">
            <ClientOnly>
              <div class="dashboard-header__sidebar-actions">
                <DashboardThemeSwitcher />
                <SystemNotificationsDropdown v-if="isAuthenticated" />
                <NuxtLink
                  v-if="isAuthenticated"
                  class="dashboard-header__sidebar-profile"
                  aria-label="Abrir perfil"
                  to="/perfil"
                  @click="close"
                >
                  <DashboardHeaderAvatar :avatar-url="avatarUrl" :initial="profileInitial" />
                  <span class="dashboard-header__sidebar-profile-copy">
                    <strong>{{ displayName || 'Conta autenticada' }}</strong>
                    <small>{{ profileRoleLabel }}</small>
                  </span>
                </NuxtLink>
              </div>
            </ClientOnly>
          </div>
        </aside>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.dashboard-header__drawer-nav {
  width: 100%;
  height: 100%;
  max-height: none;
  border-radius: 0;
  border: 0;
  box-shadow: none;
}

/* DashboardSidebarNav usa grades e alturas menores quando aparece solto no
   conteudo mobile. Dentro do drawer ele deve ocupar toda a altura e rolar como
   uma lista unica, senao sobra um bloco vazio enorme antes do rodape. */
.dashboard-header__drawer-nav :deep(.dashboard-sidebar__scroll) {
  display: block;
  overflow-x: hidden;
  overflow-y: auto;
  scrollbar-gutter: stable;
}

.dashboard-header__drawer-nav :deep(.dashboard-sidebar__section + .dashboard-sidebar__section) {
  margin-top: 0.9rem;
}

.dashboard-header__sidebar-drawer {
  position: fixed;
  inset: 0;
  z-index: 10000;
  pointer-events: none;
}

.dashboard-header__sidebar-backdrop {
  position: absolute;
  inset: 0;
  border: 0;
  background: rgb(3 6 12 / 0.48);
  pointer-events: auto;
}

.dashboard-header__sidebar-panel {
  position: absolute;
  inset-block: 0;
  left: 0;
  width: min(18rem, calc(100vw - 2rem));
  padding: 0;
  display: grid;
  grid-template-rows: minmax(0, 1fr) auto;
  background: var(--admin-header-panel-bg);
  border-right: 1px solid var(--admin-header-border);
  box-shadow: var(--shadow-md);
  pointer-events: auto;
}

.dashboard-header__sidebar-body {
  min-height: 0;
  overflow: hidden;
}

.dashboard-header__sidebar-close {
  position: absolute;
  top: 0.95rem;
  right: 0.95rem;
  z-index: 2;
  display: grid;
  place-items: center;
  width: 2rem;
  height: 2rem;
  border: 1px solid var(--admin-header-border);
  border-radius: 10px;
  background: var(--admin-header-hover-bg);
  color: var(--admin-header-muted);
  cursor: pointer;
  transition:
    color 0.16s ease,
    background 0.16s ease,
    border-color 0.16s ease;
}

.dashboard-header__sidebar-close:hover {
  border-color: rgb(var(--ring) / 0.32);
  color: var(--admin-header-text);
}

.dashboard-header__sidebar-footer {
  min-height: 4.4rem;
  display: flex;
  align-items: center;
  gap: 0.75rem;
  border-top: 1px solid var(--admin-header-separator);
  padding: 0.8rem 1rem;
  background: var(--admin-header-panel-bg);
}

.dashboard-header__sidebar-actions {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: flex-start;
  gap: 0.7rem;
  flex-wrap: nowrap;
}

.dashboard-header__sidebar-profile {
  min-width: 0;
  margin-right: auto;
  order: -1;
  display: inline-flex;
  align-items: center;
  gap: 0.7rem;
  border: 0;
  background: transparent;
  color: var(--admin-header-text);
  cursor: pointer;
  text-decoration: none;
}

@media (max-width: 640px) {
  .dashboard-header__sidebar-panel {
    width: min(18rem, calc(100vw - 1.5rem));
  }

  .dashboard-header__sidebar-footer {
    padding: 0.68rem 0.8rem;
  }

  .dashboard-header__sidebar-actions {
    gap: 0.45rem;
  }

  .dashboard-header__sidebar-profile {
    gap: 0.55rem;
  }

  .dashboard-header__sidebar-profile-copy strong,
  .dashboard-header__sidebar-profile-copy small {
    max-width: 7.5rem;
  }
}

.dashboard-header__sidebar-profile-copy {
  min-width: 0;
  display: grid;
  gap: 0.15rem;
  text-align: left;
}

.dashboard-header__sidebar-profile-copy strong,
.dashboard-header__sidebar-profile-copy small {
  max-width: 10rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dashboard-header__sidebar-profile-copy strong {
  color: var(--admin-header-text);
  font-size: 0.82rem;
  font-weight: 800;
}

.dashboard-header__sidebar-profile-copy small {
  color: var(--admin-header-muted);
  font-size: 0.72rem;
  font-weight: 700;
}

.dashboard-sidebar-drawer-enter-active,
.dashboard-sidebar-drawer-leave-active {
  transition: opacity 0.18s ease;
}

.dashboard-sidebar-drawer-enter-active .dashboard-header__sidebar-panel,
.dashboard-sidebar-drawer-leave-active .dashboard-header__sidebar-panel {
  transition: transform 0.18s ease;
}

.dashboard-sidebar-drawer-enter-from,
.dashboard-sidebar-drawer-leave-to {
  opacity: 0;
}

.dashboard-sidebar-drawer-enter-from .dashboard-header__sidebar-panel,
.dashboard-sidebar-drawer-leave-to .dashboard-header__sidebar-panel {
  transform: translateX(-100%);
}
</style>
