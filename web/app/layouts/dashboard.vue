<script setup>
import { computed, ref } from "vue";
import DashboardHeader from "~/components/dashboard/DashboardHeader.vue";
import DashboardSidebarNav from "~/components/dashboard/DashboardSidebarNav.vue";
import DashboardWorkspaceNav from "~/components/dashboard/DashboardWorkspaceNav.vue";
import FeedbackFormModal from "~/components/feedback/FeedbackFormModal.vue";
import { useContextRealtime } from "~/composables/useContextRealtime";
import { useDashboardShell } from "~/composables/useDashboardShell";
import { useAuthStore } from "~/stores/auth";

const { state, activeWorkspaceId, allowedWorkspaces, setActiveProfile, setActiveStore } = useDashboardShell();
const auth = useAuthStore();
useContextRealtime();

const feedbackModalOpen = ref(false);
const runtimeSettingsNotice = computed(() => String(auth.runtimeSettingsNotice || "").trim());
const canShowExperimentalSidebar = computed(() => auth.role === "platform_admin");

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
      <div v-if="runtimeSettingsNotice" class="runtime-settings-banner" role="status" aria-live="polite">
        <span class="material-icons-round runtime-settings-banner__icon" aria-hidden="true">warning</span>
        <div class="runtime-settings-banner__body">
          <strong>Modo degradado de configuracoes</strong>
          <p>{{ runtimeSettingsNotice }}</p>
        </div>
      </div>
      <DashboardSidebarNav
        v-if="canShowExperimentalSidebar"
        class="dashboard-sidebar-shell"
        :active-workspace="activeWorkspaceId"
        :allowed-workspaces="allowedWorkspaces"
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
.runtime-settings-banner {
  display: flex;
  align-items: flex-start;
  gap: 0.85rem;
  margin: 1rem 1.2rem 0;
  padding: 0.95rem 1rem;
  border-radius: 18px;
  border: 1px solid rgba(245, 158, 11, 0.35);
  background: linear-gradient(135deg, rgba(245, 158, 11, 0.16), rgba(15, 23, 42, 0.92));
  color: #fef3c7;
  box-shadow: 0 16px 40px rgba(15, 23, 42, 0.24);
}

.runtime-settings-banner__icon {
  font-size: 1.15rem;
  line-height: 1;
  margin-top: 0.1rem;
  color: #fbbf24;
}

.runtime-settings-banner__body {
  display: grid;
  gap: 0.2rem;
}

.runtime-settings-banner__body strong {
  font-size: 0.92rem;
  letter-spacing: 0.01em;
}

.runtime-settings-banner__body p {
  margin: 0;
  color: rgba(255, 247, 237, 0.88);
  font-size: 0.88rem;
  line-height: 1.45;
}

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

.dashboard-sidebar-shell {
  position: fixed;
  top: 5.1rem;
  bottom: 1rem;
  left: max(0.75rem, calc((100vw - 1400px) / 2 - 17rem));
  width: 16rem;
  z-index: 45;
}

@media (max-width: 980px) {
  .dashboard-sidebar-shell {
    top: auto;
    left: 0.75rem;
    bottom: 1rem;
    width: min(16rem, calc(100vw - 1.5rem));
  }
}
</style>
