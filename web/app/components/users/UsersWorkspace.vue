<script setup>
import { computed, ref, watch } from 'vue'

import SettingsTabs from '~/components/settings/SettingsTabs.vue'
import UsersAccessManager from '~/components/users/UsersAccessManager.vue'
import UsersRoleMatrixManager from '~/components/users/UsersRoleMatrixManager.vue'

const props = defineProps({
  mode: {
    type: String,
    default: 'admin',
  },
})

const activeTab = ref('usuarios')
const isQueueMode = computed(() => props.mode === 'queue')
const tabs = computed(() => {
  if (isQueueMode.value) {
    return [{ id: 'usuarios', label: 'Usuarios da fila', icon: 'group' }]
  }

  return [
    { id: 'usuarios', label: 'Usuarios', icon: 'group' },
    { id: 'perfis', label: 'Perfis e padroes', icon: 'fact_check' },
  ]
})
const title = computed(() => (isQueueMode.value ? 'Usuarios da fila' : 'Usuarios e acessos'))
const description = computed(() =>
  isQueueMode.value
    ? 'Gerencie somente os usuarios com acesso a operacao e ajuste os modulos no detalhe quando necessario.'
    : 'Gerencie contas, overrides individuais e o padrao de visibilidade do painel por tipo de usuario.',
)

watch(
  () => props.mode,
  (mode) => {
    if (mode === 'queue') {
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
      v-if="!isQueueMode"
      :tabs="tabs"
      :active-tab="activeTab"
      @update:active-tab="activeTab = $event"
    />

    <UsersAccessManager v-if="activeTab === 'usuarios'" :mode="props.mode" />
    <UsersRoleMatrixManager v-else />
  </section>
</template>
