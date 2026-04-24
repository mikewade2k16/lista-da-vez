import { cloneValue } from "~/domain/utils/object";
import { getAllowedWorkspaces } from "~/domain/utils/permissions";
import { createOptionId, getCurrentRole } from "~/stores/dashboard/runtime/shared";
import {
  createEmptyStoreScopedState,
  extractStoreScopedState,
  normalizeStoreScopedState
} from "~/stores/dashboard/runtime/state";

export function createWorkspaceActions({ getState, updateState }) {
  return {
    setActiveProfile(profileId) {
      const state = getState();
      const nextProfile = state.profiles.find((profile) => profile.id === profileId);

      if (!nextProfile) {
        return;
      }

      const allowedWorkspaces = getAllowedWorkspaces(nextProfile.role);
      const activeWorkspace = allowedWorkspaces.includes(state.activeWorkspace)
        ? state.activeWorkspace
        : allowedWorkspaces[0] || "operacao";

      updateState({
        ...state,
        activeProfileId: profileId,
        activeWorkspace,
        finishModalPersonId: null,
        finishModalDraft: null
      });
    },

    setActiveStore(storeId) {
      const state = getState();
      const nextStore = state.stores.find((store) => store.id === storeId);

      if (!nextStore || storeId === state.activeStoreId) {
        return;
      }

      const now = Date.now();
      const currentStoreId = state.activeStoreId;
      const currentSnapshot = extractStoreScopedState(state);
      const targetSnapshot = normalizeStoreScopedState(
        state.storeSnapshots?.[storeId],
        createEmptyStoreScopedState(cloneValue(state.roster)),
        nextStore,
        now
      );

      updateState({
        ...state,
        activeStoreId: storeId,
        storeSnapshots: {
          ...(state.storeSnapshots || {}),
          [currentStoreId]: currentSnapshot,
          [storeId]: targetSnapshot
        },
        ...targetSnapshot,
        finishModalPersonId: null,
        finishModalDraft: null
      });
    },

    createStore({
      name,
      city,
      code,
      cloneActiveRoster = true,
      defaultTemplateId = "",
      monthlyGoal = 0,
      weeklyGoal = 0,
      avgTicketGoal = 0,
      conversionGoal = 0,
      paGoal = 0
    }) {
      const state = getState();
      const normalizedName = String(name || "").trim();

      if (!normalizedName) {
        return { ok: false, message: "Nome da loja e obrigatorio." };
      }

      const storeId = createOptionId("loja", normalizedName, state.stores);
      const nextStore = {
        id: storeId,
        name: normalizedName,
        city: String(city || "").trim(),
        code: String(code || "").trim(),
        defaultTemplateId: String(defaultTemplateId || "").trim(),
        monthlyGoal: Math.max(0, Number(monthlyGoal || 0)),
        weeklyGoal: Math.max(0, Number(weeklyGoal || 0)),
        avgTicketGoal: Math.max(0, Number(avgTicketGoal || 0)),
        conversionGoal: Math.max(0, Math.min(100, Number(conversionGoal || 0))),
        paGoal: Math.max(0, Number(paGoal || 0))
      };
      const baseRoster = cloneActiveRoster ? cloneValue(state.roster) : [];
      const snapshot = normalizeStoreScopedState(
        createEmptyStoreScopedState(baseRoster),
        createEmptyStoreScopedState(baseRoster),
        nextStore,
        Date.now()
      );

      updateState({
        ...state,
        stores: [...state.stores, nextStore],
        storeSnapshots: {
          ...(state.storeSnapshots || {}),
          [storeId]: snapshot
        }
      });

      return { ok: true, storeId };
    },

    updateStore(storeId, patch) {
      const state = getState();
      const existingStore = state.stores.find((store) => store.id === storeId);

      if (!existingStore) {
        return { ok: false, message: "Loja nao encontrada." };
      }

      const name = String((patch?.name ?? existingStore.name) || "").trim();

      if (!name) {
        return { ok: false, message: "Nome da loja e obrigatorio." };
      }

      const updatedStore = {
        ...existingStore,
        name,
        city: String((patch?.city ?? existingStore.city) || "").trim(),
        code: String((patch?.code ?? existingStore.code) || "").trim(),
        defaultTemplateId: String(patch?.defaultTemplateId ?? existingStore.defaultTemplateId ?? "").trim(),
        monthlyGoal: Math.max(0, Number(patch?.monthlyGoal ?? existingStore.monthlyGoal ?? 0)),
        weeklyGoal: Math.max(0, Number(patch?.weeklyGoal ?? existingStore.weeklyGoal ?? 0)),
        avgTicketGoal: Math.max(0, Number(patch?.avgTicketGoal ?? existingStore.avgTicketGoal ?? 0)),
        conversionGoal: Math.max(0, Math.min(100, Number(patch?.conversionGoal ?? existingStore.conversionGoal ?? 0))),
        paGoal: Math.max(0, Number(patch?.paGoal ?? existingStore.paGoal ?? 0))
      };

      updateState({
        ...state,
        stores: state.stores.map((store) => (store.id === storeId ? updatedStore : store))
      });

      return { ok: true };
    },

    archiveStore(storeId) {
      const state = getState();
      const existingStore = state.stores.find((store) => store.id === storeId);

      if (!existingStore) {
        return { ok: false, message: "Loja nao encontrada." };
      }

      if (state.stores.length <= 1) {
        return { ok: false, message: "Mantenha pelo menos uma loja ativa no sistema." };
      }

      const nextStores = state.stores.filter((store) => store.id !== storeId);
      const nextStoreSnapshots = { ...(state.storeSnapshots || {}) };

      delete nextStoreSnapshots[storeId];

      if (storeId !== state.activeStoreId) {
        updateState({
          ...state,
          stores: nextStores,
          storeSnapshots: nextStoreSnapshots
        });

        return { ok: true };
      }

      const nextActiveStoreId = nextStores[0]?.id;
      const nextActiveStoreDescriptor = nextStores.find((store) => store.id === nextActiveStoreId) || null;
      const nextSnapshot = normalizeStoreScopedState(
        nextStoreSnapshots[nextActiveStoreId],
        createEmptyStoreScopedState(cloneValue(state.roster)),
        nextActiveStoreDescriptor,
        Date.now()
      );

      updateState({
        ...state,
        stores: nextStores,
        activeStoreId: nextActiveStoreId,
        storeSnapshots: {
          ...nextStoreSnapshots,
          [nextActiveStoreId]: nextSnapshot
        },
        ...nextSnapshot,
        finishModalPersonId: null,
        finishModalDraft: null
      });

      return { ok: true };
    },

    setWorkspace(workspaceId) {
      const state = getState();
      const allowedWorkspaces = getAllowedWorkspaces(getCurrentRole(state));

      if (!allowedWorkspaces.includes(workspaceId)) {
        return;
      }

      updateState({
        ...state,
        activeWorkspace: workspaceId
      });
    }
  };
}
