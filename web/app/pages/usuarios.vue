<script setup>
import { onMounted } from "vue";
import UsersWorkspace from "~/components/users/UsersWorkspace.vue";
import { useUsersStore } from "~/stores/users";

definePageMeta({
  layout: "dashboard",
  workspaceId: "usuarios"
});

const usersStore = useUsersStore();

onMounted(() => {
  void usersStore.ensureLoaded();
});
</script>

<template>
  <div class="workspace-host">
    <UsersWorkspace />
  </div>
</template>
