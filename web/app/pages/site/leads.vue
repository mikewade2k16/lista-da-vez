<script setup lang="ts">
import SiteLeadsAdminWorkspace from '~/components/site/SiteLeadsAdminWorkspace.vue'

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'site_leads_web',
  pageLabel: 'Leads do Site',
})
</script>

<template>
  <div class="page-workspace">
    <SiteLeadsAdminWorkspace />
  </div>
</template>
