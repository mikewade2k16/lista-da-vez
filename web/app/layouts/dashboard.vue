<script setup>
import DashboardHeader from "~/components/dashboard/DashboardHeader.vue";
import DashboardWorkspaceNav from "~/components/dashboard/DashboardWorkspaceNav.vue";
import { useDashboardShell } from "~/composables/useDashboardShell";

const { state, activeWorkspaceId, allowedWorkspaces, setActiveProfile, setActiveStore } = useDashboardShell();

function handleProfileChange(profileId) {
  void setActiveProfile(profileId);
}

function handleStoreChange(storeId) {
  void setActiveStore(storeId);
}
</script>

<template>
  <main class="shell">
    <section class="app-surface">
      <DashboardHeader
        :state="state"
        @profile-change="handleProfileChange"
        @store-change="handleStoreChange"
      />
      <div class="workspace">
        <DashboardWorkspaceNav
          :active-workspace="activeWorkspaceId"
          :allowed-workspaces="allowedWorkspaces"
        />
        <slot />
      </div>
    </section>
  </main>
</template>
