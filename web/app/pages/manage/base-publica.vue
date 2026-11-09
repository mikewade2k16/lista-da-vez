<script setup lang="ts">
import AdminPublicKnowledgeWorkspace from '~/components/admin/knowledge/AdminPublicKnowledgeWorkspace.vue'

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'knowledge_admin',
  pageLabel: 'Base pública',
})
</script>

<template>
  <AdminPublicKnowledgeWorkspace />
</template>
