<script setup lang="ts">
import AdminUsersWorkspace from '~/components/admin/AdminUsersWorkspace.vue'

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'usuarios_admin',
  pageLabel: 'Usuarios Admin',
})
</script>

<template>
  <div class="page-workspace">
    <AdminUsersWorkspace />
  </div>
</template>
