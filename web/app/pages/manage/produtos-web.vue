<script setup lang="ts">
import SiteProductsAdminWorkspace from '~/components/site/SiteProductsAdminWorkspace.vue'

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'site_produtos_web',
  pageLabel: 'Produtos Web',
})
</script>

<template>
  <div class="page-workspace">
    <SiteProductsAdminWorkspace />
  </div>
</template>
