import { ref } from "vue";
import { defineStore } from "pinia";
import { mockQueueState } from "~/domain/data/mock-queue";
import { cloneValue } from "~/domain/utils/object";
import { createAppStore } from "~/stores/dashboard/runtime/create-dashboard-runtime";

export const useAppRuntimeStore = defineStore("app-runtime", () => {
  const state = ref(cloneValue(mockQueueState));
  let runtimeStore = null;
  let unsubscribe = null;
  let initialized = false;

  function getSeedState() {
    return cloneValue(state.value || mockQueueState);
  }

  function replaceState(nextState) {
    state.value = cloneValue(nextState || mockQueueState);
  }

  function getRuntimeStore() {
    if (!runtimeStore) {
      runtimeStore = createAppStore(getSeedState());
    }

    return runtimeStore;
  }

  function bindRuntimeStore(store) {
    if (unsubscribe) {
      return;
    }

    unsubscribe = store.subscribe((nextState) => {
      replaceState(nextState);
    });
  }

  async function ensure() {
    const store = getRuntimeStore();

    if (!initialized) {
      bindRuntimeStore(store);
      store.hydrate(getSeedState());
      initialized = true;
    }

    replaceState(store.getState());

    return store;
  }

  async function withStore(handler) {
    const store = await ensure();

    if (!store) {
      return null;
    }

    const result = await handler(store);
    replaceState(store.getState());
    return result;
  }

  async function run(actionName, ...args) {
    return withStore((store) => {
      const action = store?.[actionName];

      if (typeof action !== "function") {
        return null;
      }

      return action(...args);
    });
  }

  function hydrate(nextState) {
    const store = getRuntimeStore();
    bindRuntimeStore(store);
    store.hydrate(nextState);

    initialized = true;
    replaceState(store.getState());
    return store.getState();
  }

  return {
    state,
    ensure,
    hydrate,
    run,
    withStore
  };
});
