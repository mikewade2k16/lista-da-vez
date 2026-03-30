import { computed, onMounted, watch } from "vue";
import { storeToRefs } from "pinia";
import { getAllowedWorkspaces } from "@core/utils/permissions";
import { useDashboardStore } from "~/stores/dashboard";
import { getWorkspaceLabel, getWorkspacePath } from "~/utils/workspaces";

export function useDashboardState() {
  const dashboard = useDashboardStore();
  const { state } = storeToRefs(dashboard);

  return {
    dashboard,
    state
  };
}

export function useDashboardShell() {
  const route = useRoute();
  const { dashboard, state } = useDashboardState();

  const activeWorkspaceId = computed(() =>
    String(route.meta.workspaceId || state.value?.activeWorkspace || "operacao")
  );
  const activeProfile = computed(() =>
    state.value.profiles.find((profile) => profile.id === state.value.activeProfileId) || state.value.profiles[0] || null
  );
  const activeRole = computed(() => activeProfile.value?.role || "consultant");
  const allowedWorkspaces = computed(() => getAllowedWorkspaces(activeRole.value));
  const pageLabel = computed(() => getWorkspaceLabel(activeWorkspaceId.value) || "Painel");

  useHead(() => ({
    title: `${pageLabel.value} | ${state.value.brandName}`
  }));

  async function syncWorkspaceState() {
    await dashboard.ensure();
    const allowed = allowedWorkspaces.value;
    const fallbackWorkspace = allowed[0] || "operacao";
    const nextWorkspace = allowed.includes(activeWorkspaceId.value) ? activeWorkspaceId.value : fallbackWorkspace;

    if (nextWorkspace !== activeWorkspaceId.value) {
      await navigateTo(getWorkspacePath(nextWorkspace), { replace: true });
      return;
    }

    if (state.value.activeWorkspace !== nextWorkspace) {
      await dashboard.setWorkspace(nextWorkspace);
    }
  }

  onMounted(async () => {
    await dashboard.ensure();
    await syncWorkspaceState();
  });

  watch([activeWorkspaceId, activeRole], () => {
    if (import.meta.client) {
      void syncWorkspaceState();
    }
  });

  return {
    dashboard,
    state,
    activeWorkspaceId,
    allowedWorkspaces,
    pageLabel,
    setActiveProfile(profileId) {
      return dashboard.setActiveProfile(profileId);
    },
    setActiveStore(storeId) {
      return dashboard.setActiveStore(storeId);
    }
  };
}
