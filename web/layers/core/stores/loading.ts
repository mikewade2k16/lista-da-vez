import { defineStore } from "pinia";
import { computed, ref } from "vue";

// Store de loading global. Conta requisicoes/operacoes em andamento via push/pop
// e expoe um `isLoading` reativo. Pensado para alimentar `CoreLoadingOverlay`
// (barra fina no topo + leve fade) sem precisar instalar pinia/vuex extra.
//
// O api-client (`web/app/utils/api-client.ts`) faz push quando uma requisicao
// passa de ~200ms e pop quando ela termina. Tambem pode ser usado manualmente
// em paginas para fluxos longos (ex: bootstrap de account, save de formulario).
export const useCoreLoadingStore = defineStore("core/loading", () => {
  const counter = ref(0);
  const labelStack = ref<string[]>([]);

  const isLoading = computed(() => counter.value > 0);
  const activeLabel = computed(() => labelStack.value[labelStack.value.length - 1] ?? "");

  function push(label = "") {
    counter.value += 1;
    if (label) {
      labelStack.value.push(label);
    }
  }

  function pop(label = "") {
    if (counter.value > 0) {
      counter.value -= 1;
    }
    if (label) {
      const index = labelStack.value.lastIndexOf(label);
      if (index >= 0) {
        labelStack.value.splice(index, 1);
      }
    } else if (labelStack.value.length > 0) {
      labelStack.value.pop();
    }
  }

  function reset() {
    counter.value = 0;
    labelStack.value = [];
  }

  return {
    counter,
    isLoading,
    activeLabel,
    push,
    pop,
    reset
  };
});
