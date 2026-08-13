<script setup lang="ts">
import { computed } from 'vue'
import OmniMediaViewer, { type OmniMediaViewerItem } from '~/components/ui/OmniMediaViewer.vue'
import { getApiBase } from '~/utils/api-client'
import { resolveMediaUrl } from '~/utils/media'
import { formatBytes, type CalendarMediaItem } from '~/utils/calendar'

const props = defineProps<{ items: CalendarMediaItem[]; startIndex?: number }>()
const emit = defineEmits<{ close: [] }>()
const apiBase = getApiBase(useRuntimeConfig())
const viewerItems = computed<OmniMediaViewerItem[]>(() =>
  props.items.map((item) => ({
    id: item.id,
    name: item.name,
    url: resolveMediaUrl(item.url, apiBase),
    type: item.type,
    sizeLabel: formatBytes(item.sizeBytes),
    posterUrl: resolveMediaUrl(item.posterUrl || '', apiBase) || undefined,
  })),
)
</script>

<template>
  <OmniMediaViewer :items="viewerItems" :start-index="startIndex" @close="emit('close')" />
</template>
