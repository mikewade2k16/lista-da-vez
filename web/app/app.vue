<script setup>
import AppDialogHost from "~/components/ui/AppDialogHost.vue";
import AppToastStack from "~/components/ui/AppToastStack.vue";
import CoreLoadingOverlay from "../layers/core/components/CoreLoadingOverlay.vue";
import { useCoreLoadingStore } from "../layers/core/stores/loading";

// Fase 9A — feedback visual: a barra fina (CoreLoadingOverlay) aparece em
// qualquer layout sempre que o api-client detecta requisicao acima de 200ms
// ou que uma pagina chama useCoreLoading().push(). Tambem ativa em mudancas
// de rota via hooks page:start / page:finish.
//
// Imports relativos para o layer (auto-import via `~/` aponta para app/srcDir).
const loading = useCoreLoadingStore();
const nuxtApp = useNuxtApp();

nuxtApp.hook("page:start", () => {
  loading.push();
});

nuxtApp.hook("page:finish", () => {
  loading.pop();
});

nuxtApp.hook("vue:error", () => {
  loading.reset();
});
</script>

<template>
  <UApp>
    <NuxtLayout>
      <NuxtPage />
    </NuxtLayout>
    <ClientOnly>
      <CoreLoadingOverlay />
      <AppDialogHost />
      <AppToastStack />
    </ClientOnly>
  </UApp>
</template>
