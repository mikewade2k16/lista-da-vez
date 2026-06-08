<script setup lang="ts">
import SiteTrackingAdminWorkspace from '~/components/site/SiteTrackingAdminWorkspace.vue'

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'site_tracking_web',
  pageLabel: 'Tracking do Site',
})
</script>

<template>
  <div class="page-workspace">
    <SiteTrackingAdminWorkspace />
  </div>
</template>
