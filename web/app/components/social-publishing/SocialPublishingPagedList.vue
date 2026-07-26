<script setup lang="ts">
import type { SocialPublishingPost } from '~/domain/social-publishing/model'

defineProps<{
  posts: SocialPublishingPost[]
  mode: 'queue' | 'content'
  pageIndex: number
  pageSize: number
  hasNext: boolean
  loading: boolean
  refreshing: boolean
  canManage: boolean
  busyPostIds: string[]
}>()

const emit = defineEmits<{
  page: [pageIndex: number]
  refresh: []
  edit: [post: SocialPublishingPost]
  cancel: [post: SocialPublishingPost]
  retry: [post: SocialPublishingPost]
}>()
</script>

<template>
  <div>
    <SocialPublishingPagination
      :page-index="pageIndex"
      :page-size="pageSize"
      :item-count="posts.length"
      :has-next="hasNext"
      :loading="loading"
      :refreshing="refreshing"
      @previous="emit('page', Math.max(0, pageIndex - 1))"
      @next="emit('page', pageIndex + 1)"
      @refresh="emit('refresh')"
    />
    <SocialPublishingPostList
      :posts="posts"
      :mode="mode"
      :can-manage="canManage"
      :busy-post-ids="busyPostIds"
      @edit="emit('edit', $event)"
      @cancel="emit('cancel', $event)"
      @retry="emit('retry', $event)"
    />
  </div>
</template>
