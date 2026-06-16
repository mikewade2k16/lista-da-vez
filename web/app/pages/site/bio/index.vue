<script setup lang="ts">
import BioListWorkspace from '~/components/bio/BioListWorkspace.vue'

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'site_bio_web',
  pageLabel: 'Bio',
})
</script>

<template>
  <div class="page-workspace">
    <BioListWorkspace />
  </div>
</template>
