import { computed } from "vue";
import { defineStore, storeToRefs } from "pinia";
import { getAllowedWorkspaces } from "~/domain/utils/permissions";
import { useAuthStore } from "~/stores/auth";
import { useAppRuntimeStore } from "~/stores/app-runtime";

export const useWorkspaceStore = defineStore("workspace", () => {
  const runtime = useAppRuntimeStore();
  const auth = useAuthStore();
  const { state } = storeToRefs(runtime);

  const activeProfile = computed(() =>
    state.value.profiles.find((profile) => profile.id === state.value.activeProfileId) ||
    state.value.profiles[0] ||
    null
  );
  const activeRole = computed(() => auth.role || activeProfile.value?.role || "consultant");
  const allowedWorkspaces = computed(() => getAllowedWorkspaces(activeRole.value));

  return {
    state,
    activeProfile,
    activeRole,
    allowedWorkspaces,
    ensure: runtime.ensure,
    setWorkspace(workspaceId) {
      return runtime.run("setWorkspace", workspaceId);
    },
    setActiveProfile(profileId) {
      return runtime.run("setActiveProfile", profileId);
    },
    setActiveStore(storeId) {
      if (auth.isAuthenticated) {
        return auth.setActiveStore(storeId);
      }

      return runtime.run("setActiveStore", storeId);
    }
  };
});
