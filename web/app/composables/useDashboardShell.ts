import { computed, onMounted, watch } from "vue";
import { storeToRefs } from "pinia";
import { useAuthStore } from "~/stores/auth";
import { useWorkspaceStore } from "~/stores/workspace";
import { getWorkspaceLabel, getWorkspacePath } from "~/utils/workspaces";

export function useDashboardState() {
  const workspace = useWorkspaceStore();
  const { state } = storeToRefs(workspace);

  return {
    state
  };
}

export function useDashboardShell() {
  const route = useRoute();
  const auth = useAuthStore();
  const workspace = useWorkspaceStore();
  const { activeRole, allowedWorkspaces } = storeToRefs(workspace);
  const { state } = useDashboardState();

  const activeWorkspaceId = computed(() =>
    String(route.meta.workspaceId || state.value?.activeWorkspace || "operacao")
  );
  const pageLabel = computed(() => getWorkspaceLabel(activeWorkspaceId.value) || "Painel");

  useHead(() => ({
    title: `${pageLabel.value} | ${state.value?.brandName || "Fila Atendimento"}`
  }));

  async function syncWorkspaceState() {
    await auth.ensureSession();

    if (!auth.isAuthenticated) {
      return;
    }

    await workspace.ensure();
    const allowed = allowedWorkspaces.value;
    const fallbackWorkspace = allowed[0] || "operacao";
    const nextWorkspace = allowed.includes(activeWorkspaceId.value) ? activeWorkspaceId.value : fallbackWorkspace;

    if (nextWorkspace !== activeWorkspaceId.value) {
      await navigateTo(getWorkspacePath(nextWorkspace), { replace: true });
      return;
    }

    if (state.value.activeWorkspace !== nextWorkspace) {
      await workspace.setWorkspace(nextWorkspace);
    }
  }

  onMounted(async () => {
    await syncWorkspaceState();
  });

  watch([activeWorkspaceId, activeRole, () => auth.isAuthenticated], ([, , isAuthenticated]) => {
    if (import.meta.client) {
      if (!isAuthenticated) {
        return;
      }

      void syncWorkspaceState();
    }
  });

  return {
    state,
    activeWorkspaceId,
    allowedWorkspaces,
    pageLabel,
    setActiveProfile(profileId) {
      return workspace.setActiveProfile(profileId);
    },
    setActiveStore(storeId) {
      return workspace.setActiveStore(storeId);
    }
  };
}
