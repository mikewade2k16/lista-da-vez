<script setup lang="ts">
import AdminStorageWorkspace from '~/components/admin/storage/AdminStorageWorkspace.vue'

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'storage_admin',
  pageLabel: 'Storage R2',
})
</script>

<template>
  <AdminStorageWorkspace />
</template>
