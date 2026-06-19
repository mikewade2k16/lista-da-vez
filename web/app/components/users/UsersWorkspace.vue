<script setup>
import { computed, ref, watch } from 'vue'

import SettingsTabs from '~/components/settings/SettingsTabs.vue'
import UsersAccessManager from '~/components/users/UsersAccessManager.vue'
import UsersRoleMatrixManager from '~/components/users/UsersRoleMatrixManager.vue'
import { canManageRoleDefaults } from '~/domain/utils/permissions'
import { useAuthStore } from '~/stores/auth'

const props = defineProps({
  mode: {
    type: String,
    default: 'admin',
  },
})

const auth = useAuthStore()
// A aba "Perfis e padroes" (matriz padrao por papel) volta a aparecer na operacao
// (modo queue), mas SO para quem pode editar o padrao por perfil (platform_admin /
// access.role_defaults.manage). Usuario comum da fila nao ve.
const canManageProfiles = computed(() =>
  canManageRoleDefaults(auth.role, auth.permissionKeys, auth.permissionsResolved),
)

const activeTab = ref('usuarios')
const isQueueMode = computed(() => props.mode === 'queue')
const tabs = computed(() => {
  const usuariosTab = {
    id: 'usuarios',
    label: isQueueMode.value ? 'Usuarios da fila' : 'Usuarios',
    icon: 'group',
  }
  const perfisTab = { id: 'perfis', label: 'Perfis e padroes', icon: 'fact_check' }

  if (isQueueMode.value) {
    return canManageProfiles.value ? [usuariosTab, perfisTab] : [usuariosTab]
  }

  return [usuariosTab, perfisTab]
})
const title = computed(() => (isQueueMode.value ? 'Usuarios da fila' : 'Usuarios e acessos'))
const description = computed(() =>
  isQueueMode.value
    ? 'Gerencie somente os usuarios com acesso a operacao e ajuste os modulos no detalhe quando necessario.'
    : 'Gerencie contas, overrides individuais e o padrao de visibilidade do painel por tipo de usuario.',
)

// Se a aba ativa virar indisponivel (ex.: perdeu permissao de perfis, ou trocou
// de modo), volta para "usuarios" para nao ficar numa aba vazia.
watch(
  [tabs, canManageProfiles],
  () => {
    if (!tabs.value.some((tab) => tab.id === activeTab.value)) {
      activeTab.value = 'usuarios'
    }
  },
  { immediate: true },
)
</script>

<template>
  <section class="admin-panel" data-testid="users-panel">
    <header class="admin-panel__header">
      <h2 class="admin-panel__title">{{ title }}</h2>
      <p class="admin-panel__text">{{ description }}</p>
    </header>

    <SettingsTabs
      v-if="tabs.length > 1"
      :tabs="tabs"
      :active-tab="activeTab"
      @update:active-tab="activeTab = $event"
    />

    <UsersAccessManager v-if="activeTab === 'usuarios'" :mode="props.mode" />
    <UsersRoleMatrixManager v-else-if="canManageProfiles" />
  </section>
</template>
