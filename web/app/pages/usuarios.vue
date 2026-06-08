<script setup>
import { onMounted } from 'vue'
import UsersWorkspace from '~/components/users/UsersWorkspace.vue'
import LegacyMarker from '~/components/admin/LegacyMarker.vue'
import { useUsersStore } from '~/stores/users'

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'usuarios',
  alias: ['/operacao/usuarios'],
  // Path canonico passou a ser /operacao/usuarios (decisao 2026-05-29:
  // essa tela e do modulo Fila e fica dentro de /operacao). O alias mantem
  // /usuarios funcionando para nao quebrar URLs antigas.
})

const usersStore = useUsersStore()

onMounted(() => {
  void usersStore.ensureLoaded()
})
</script>

<template>
  <div class="page-workspace">
    <LegacyMarker
      label="Papeis 100% em core.* (user_*_roles dropadas no U4c). Identidade ainda lida via view public.users."
      kind="legacy"
      detail="Resta o item 2 do docs/LEGADO.md: public.users e VIEW de compat sobre core.users."
    />
    <UsersWorkspace mode="queue" />
  </div>
</template>
