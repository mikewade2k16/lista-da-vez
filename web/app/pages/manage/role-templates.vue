<script setup lang="ts">
import AdminRoleTemplatesWorkspace from '~/components/admin/role-templates/AdminRoleTemplatesWorkspace.vue'

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'role_templates_admin',
  pageLabel: 'Role Templates',
})
</script>

<template>
  <div class="page-workspace">
    <AdminRoleTemplatesWorkspace />
  </div>
</template>
