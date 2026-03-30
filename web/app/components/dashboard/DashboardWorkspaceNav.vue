<script setup>
import { computed } from "vue";
import { WORKSPACES } from "~/utils/workspaces";

const props = defineProps({
  activeWorkspace: {
    type: String,
    required: true
  },
  allowedWorkspaces: {
    type: Array,
    required: true
  }
});

const visibleWorkspaces = computed(() =>
  WORKSPACES.filter((workspace) => props.allowedWorkspaces.includes(workspace.id))
);
</script>

<template>
  <nav class="workspace-nav" aria-label="Areas do sistema">
    <NuxtLink
      v-for="workspace in visibleWorkspaces"
      :key="workspace.id"
      :to="workspace.path"
      class="workspace-nav__button"
      :class="{ 'workspace-nav__button--active': workspace.id === activeWorkspace }"
      :title="workspace.label"
    >
      <span class="material-icons-round workspace-nav__icon">{{ workspace.icon }}</span>
      <span class="workspace-nav__label">{{ workspace.label }}</span>
    </NuxtLink>
  </nav>
</template>
