import { createConsultantActions } from "~/stores/dashboard/runtime/actions/consultant-actions";
import { createOperationActions } from "~/stores/dashboard/runtime/actions/operation-actions";
import { createSettingsActions } from "~/stores/dashboard/runtime/actions/settings-actions";
import { createWorkspaceActions } from "~/stores/dashboard/runtime/actions/workspace-actions";
import { createEmptyState, hydrateState, syncStoreSnapshots } from "~/stores/dashboard/runtime/state";

export function createAppStore(initialState = createEmptyState()) {
  let state = initialState;
  const listeners = new Set();

  function emitChange() {
    listeners.forEach((listener) => listener(state));
  }

  function updateState(nextState) {
    state = syncStoreSnapshots(nextState);
    emitChange();
  }

  const runtimeContext = {
    getState: () => state,
    updateState
  };

  return {
    getState() {
      return state;
    },

    subscribe(listener) {
      listeners.add(listener);

      return () => {
        listeners.delete(listener);
      };
    },

    hydrate(nextState) {
      updateState(hydrateState(nextState));
    },

    ...createWorkspaceActions(runtimeContext),
    ...createSettingsActions(runtimeContext),
    ...createConsultantActions(runtimeContext),
    ...createOperationActions(runtimeContext)
  };
}
