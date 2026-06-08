<script setup lang="ts">
import AdminOrganizationsWorkspace from '~/components/admin/AdminOrganizationsWorkspace.vue'

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'organizations_admin',
  pageLabel: 'Organizations',
})
</script>

<template>
  <div class="page-workspace">
    <AdminOrganizationsWorkspace />
  </div>
</template>
