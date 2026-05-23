<script setup>
import '~/components/users/users-access-manager.css'

import UsersAccessCreateModal from '~/components/users/UsersAccessCreateModal.vue'
import UsersAccessDetailDrawer from '~/components/users/UsersAccessDetailDrawer.vue'
import UsersAccessTable from '~/components/users/UsersAccessTable.vue'
import {
  provideUsersAccessContext,
  useUsersAccessManager,
} from '~/composables/useUsersAccessManager'

const ctx = await useUsersAccessManager()
provideUsersAccessContext(ctx)
</script>

<template>
  <section class="users-access-manager">
    <UsersAccessCreateModal />
    <UsersAccessTable />
    <UsersAccessDetailDrawer />
  </section>
</template>
