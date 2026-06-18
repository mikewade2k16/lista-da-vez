<script setup lang="ts">
defineProps<{
  avatarUrl?: string
  initial?: string
}>()
</script>

<template>
  <span class="dashboard-header-avatar" aria-hidden="true">
    <img v-if="avatarUrl" :src="avatarUrl" alt="" />
    <span v-else>{{ initial || 'U' }}</span>
  </span>
</template>

<style scoped>
.dashboard-header-avatar {
  display: grid;
  place-items: center;
  width: 2.3rem;
  height: 2.3rem;
  border-radius: 999px;
  overflow: hidden;
  background: linear-gradient(135deg, rgb(var(--primary) / 0.92), rgb(var(--success) / 0.9));
  color: rgb(255 255 255);
  font-size: 0.86rem;
  font-weight: 800;
  text-transform: uppercase;
  box-shadow: inset 0 0 0 1px rgb(255 255 255 / 0.18);
}

.dashboard-header-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
</style>
