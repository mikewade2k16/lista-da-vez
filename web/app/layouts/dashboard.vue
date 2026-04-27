<script setup>
import { ref } from "vue";
import DashboardHeader from "~/components/dashboard/DashboardHeader.vue";
import DashboardWorkspaceNav from "~/components/dashboard/DashboardWorkspaceNav.vue";
import AppToastStack from "~/components/ui/AppToastStack.vue";
import FeedbackFormModal from "~/components/feedback/FeedbackFormModal.vue";
import { useContextRealtime } from "~/composables/useContextRealtime";
import { useDashboardShell } from "~/composables/useDashboardShell";

const { state, activeWorkspaceId, allowedWorkspaces, setActiveProfile, setActiveStore } = useDashboardShell();
useContextRealtime();

const feedbackModalOpen = ref(false);

function handleProfileChange(profileId) {
  void setActiveProfile(profileId);
}

function handleStoreChange(storeId) {
  void setActiveStore(storeId);
}
</script>

<template>
  <main class="shell">
    <AppToastStack />
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

    <button
      class="dashboard-feedback-btn"
      @click="feedbackModalOpen = true"
      title="Enviar feedback para o time"
      aria-label="Enviar feedback"
    >
      <span class="dashboard-feedback-btn__icon">💬</span>
    </button>

    <FeedbackFormModal v-model="feedbackModalOpen" />
  </main>
</template>

<style scoped>
.dashboard-feedback-btn {
  position: fixed;
  bottom: 2rem;
  right: 2rem;
  width: 3rem;
  height: 3rem;
  border-radius: 50%;
  background-color: #3b82f6;
  border: none;
  cursor: pointer;
  box-shadow: 0 4px 12px rgba(59, 130, 246, 0.3);
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s;
  z-index: 500;
}

.dashboard-feedback-btn:hover {
  background-color: #2563eb;
  box-shadow: 0 6px 16px rgba(59, 130, 246, 0.4);
  transform: scale(1.1);
}

.dashboard-feedback-btn:active {
  transform: scale(0.95);
}

.dashboard-feedback-btn__icon {
  font-size: 1.5rem;
  line-height: 1;
}
</style>
