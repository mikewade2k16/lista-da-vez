<script setup lang="ts">
import { useRoute } from 'vue-router'

import BioEditorWorkspace from '~/components/bio/BioEditorWorkspace.vue'

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'site_bio_web',
  pageLabel: 'Editar Bio',
})

const route = useRoute()
const bioId = computed(() => String(route.params.id || '').trim())
</script>

<template>
  <div class="page-workspace">
    <BioEditorWorkspace :bio-id="bioId" />
  </div>
</template>
