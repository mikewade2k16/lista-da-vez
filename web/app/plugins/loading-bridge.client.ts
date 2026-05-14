import { setApiLoadingHooks } from "~/utils/api-client";
import { useCoreLoadingStore } from "../../layers/core/stores/loading";

// Plugin client-only que conecta o store global de loading ao api-client.
// Sem ele, requisicoes nao acionam o CoreLoadingOverlay (barra fina no topo)
// quando passam de 200ms. Isolar a ligacao em plugin evita import direto do
// store dentro do api-client e a dependencia circular que isso causaria
// (stores usam o api-client durante o setup do pinia).
//
// Import relativo para o layer (auto-import via `~/` aponta para app/srcDir).
export default defineNuxtPlugin(() => {
  const store = useCoreLoadingStore();

  setApiLoadingHooks({
    push: () => store.push(),
    pop: () => store.pop()
  });
});
