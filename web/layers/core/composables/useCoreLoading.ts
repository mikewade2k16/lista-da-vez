import { computed } from "vue";
import { useCoreLoadingStore } from "../stores/loading";

// Composable de loading global. Acesso ergonomico ao `useCoreLoadingStore`
// para uso em paginas/components. Exemplo:
//
//   const loading = useCoreLoading();
//   loading.push("loading-context");
//   try { await fetchContext(); } finally { loading.pop("loading-context"); }
//
// Tambem usado pelo api-client para envolver requests > 200ms.
export function useCoreLoading() {
  const store = useCoreLoadingStore();

  function withLoading<T>(label: string, fn: () => Promise<T>): Promise<T> {
    store.push(label);
    return fn().finally(() => {
      store.pop(label);
    });
  }

  return {
    push: store.push,
    pop: store.pop,
    reset: store.reset,
    isLoading: computed(() => store.isLoading),
    activeLabel: computed(() => store.activeLabel),
    withLoading
  };
}
