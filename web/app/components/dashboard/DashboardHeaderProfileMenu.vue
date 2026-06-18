<script setup lang="ts">
import { ref } from 'vue'
import { LogOut, User } from 'lucide-vue-next'
import DashboardHeaderAvatar from '~/components/dashboard/DashboardHeaderAvatar.vue'
import { useDropdownDismiss } from '~/composables/useDropdownDismiss'
import { useHeaderProfile } from '~/composables/useHeaderProfile'
import { useAuthStore } from '~/stores/auth'

const auth = useAuthStore()
const { displayName, profileEmail, profileRoleLabel, avatarUrl, profileInitial } =
  useHeaderProfile()

const menuRef = ref<HTMLElement | null>(null)
const open = ref(false)

function toggle() {
  open.value = !open.value
}

function close() {
  open.value = false
}

useDropdownDismiss(() => open.value, close, { rootRef: menuRef })

async function handleLogout() {
  close()
  await auth.logout()
  await navigateTo('/auth/login', { replace: true })
}
</script>

<template>
  <div ref="menuRef" class="dashboard-header__profile-menu">
    <button
      class="dashboard-header__profile-trigger"
      type="button"
      aria-haspopup="menu"
      :aria-expanded="open ? 'true' : 'false'"
      aria-label="Abrir menu do perfil"
      @click="toggle"
    >
      <DashboardHeaderAvatar :avatar-url="avatarUrl" :initial="profileInitial" />
    </button>

    <Transition name="dashboard-header-menu">
      <div v-if="open" class="dashboard-header__profile-dropdown" role="menu">
        <div class="dashboard-header__profile-card">
          <span class="dashboard-header__profile-role">{{ profileRoleLabel }}</span>
          <strong class="dashboard-header__profile-fullname">
            {{ displayName || 'Conta autenticada' }}
          </strong>
          <span class="dashboard-header__profile-email">
            {{ profileEmail || 'Sessao ativa' }}
          </span>
        </div>

        <NuxtLink class="dashboard-header__menu-action" to="/perfil" role="menuitem" @click="close">
          <User :size="16" :stroke-width="2.15" />
          <span>Pagina de perfil</span>
        </NuxtLink>

        <button
          class="dashboard-header__menu-action dashboard-header__menu-action--danger"
          type="button"
          role="menuitem"
          @click="handleLogout"
        >
          <LogOut :size="16" :stroke-width="2.15" />
          <span>Sair da plataforma</span>
        </button>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.dashboard-header__profile-menu {
  position: relative;
}

.dashboard-header__profile-trigger {
  display: grid;
  place-items: center;
  width: 3rem;
  height: 3rem;
  padding: 0;
  border: 1px solid var(--admin-header-border);
  border-radius: 999px;
  background: var(--admin-header-hover-bg);
  color: var(--admin-header-text);
  cursor: pointer;
  transition:
    border-color 0.18s ease,
    background 0.18s ease,
    transform 0.18s ease;
}

.dashboard-header__profile-trigger:hover,
.dashboard-header__profile-trigger[aria-expanded='true'] {
  border-color: rgb(var(--ring) / 0.42);
  background: var(--admin-header-active-bg);
}

.dashboard-header__profile-dropdown {
  position: absolute;
  top: calc(100% + 0.55rem);
  right: 0;
  z-index: 30;
  display: grid;
  gap: 0.55rem;
  width: min(18.5rem, calc(100vw - 2rem));
  padding: 0.8rem;
  border: 1px solid var(--admin-header-border);
  border-radius: 1rem;
  background: var(--admin-header-panel-bg);
  box-shadow: var(--shadow-md);
  backdrop-filter: blur(var(--admin-header-panel-blur));
}

.dashboard-header__profile-card {
  display: grid;
  gap: 0.24rem;
  padding: 0.8rem 0.85rem;
  border: 1px solid rgb(var(--primary) / 0.16);
  border-radius: 0.85rem;
  background: var(--admin-header-brand-bg);
}

.dashboard-header__profile-role {
  color: rgb(var(--primary) / 0.9);
  font-size: 0.64rem;
  font-weight: 800;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.dashboard-header__profile-fullname {
  color: var(--admin-header-text);
  font-size: 0.95rem;
  font-weight: 700;
}

.dashboard-header__profile-email {
  color: var(--admin-header-muted);
  font-size: 0.75rem;
  word-break: break-word;
}

.dashboard-header__menu-action {
  display: flex;
  align-items: center;
  gap: 0.7rem;
  width: 100%;
  padding: 0.82rem 0.88rem;
  border: 1px solid var(--admin-header-border);
  border-radius: 0.85rem;
  background: transparent;
  color: var(--admin-header-text);
  text-decoration: none;
  font-size: 0.82rem;
  font-weight: 700;
  cursor: pointer;
  transition:
    border-color 0.18s ease,
    background 0.18s ease,
    color 0.18s ease;
}

.dashboard-header__menu-action:hover {
  border-color: rgb(var(--ring) / 0.28);
  background: var(--admin-header-hover-bg);
}

.dashboard-header__menu-action--danger {
  color: rgb(var(--danger));
  background: rgb(var(--danger) / 0.12);
  border-color: rgb(var(--danger) / 0.22);
}

.dashboard-header__menu-action--danger:hover {
  border-color: rgb(var(--danger) / 0.36);
  background: rgb(var(--danger) / 0.18);
}

.dashboard-header-menu-enter-active,
.dashboard-header-menu-leave-active {
  transition:
    opacity 0.18s ease,
    transform 0.18s ease;
}

.dashboard-header-menu-enter-from,
.dashboard-header-menu-leave-to {
  opacity: 0;
  transform: translateY(-6px);
}

@media (max-width: 640px) {
  .dashboard-header__profile-menu {
    width: auto;
  }

  .dashboard-header__profile-trigger {
    width: 3rem;
  }

  .dashboard-header__profile-dropdown {
    left: 0;
    right: auto;
    width: min(100%, 18.5rem);
  }
}
</style>
