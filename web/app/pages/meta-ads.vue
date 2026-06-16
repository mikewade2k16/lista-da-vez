<script setup lang="ts">
import MetaAdsWorkspace from '~/components/meta-ads/MetaAdsWorkspace.vue'

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'meta_ads',
  pageLabel: 'Meta Ads',
})
</script>

<template>
  <div class="page-workspace">
    <MetaAdsWorkspace />
  </div>
</template>
