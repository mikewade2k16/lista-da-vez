<script setup>
import AppDialogHost from '~/components/ui/AppDialogHost.vue'
import PwaReloadPrompt from '~/components/pwa/PwaReloadPrompt.vue'
import AppToastStack from '~/components/ui/AppToastStack.vue'
import CoreLoadingOverlay from '../layers/core/components/CoreLoadingOverlay.vue'
import { useCoreLoadingStore } from '../layers/core/stores/loading'

// Fase 9A — feedback visual: a barra fina (CoreLoadingOverlay) aparece em
// qualquer layout sempre que o api-client detecta requisicao acima de 200ms
// ou que uma pagina chama useCoreLoading().push(). Tambem ativa em mudancas
// de rota via hooks page:start / page:finish.
//
// page:loading:start/end cobrem a janela que page:start NAO cobre: o fetch do
// chunk lazy da rota (no dev, o Vite compila a pagina no 1o clique — segundos
// sem nenhum feedback; em prod, chunk em rede lenta). Sem esse par, o clique
// no menu parecia "travado" ate o chunk chegar. O flag navLoading impede pop
// duplo (page:loading:end + afterEach com failure) de roubar um push alheio.
//
// Imports relativos para o layer (auto-import via `~/` aponta para app/srcDir).
const loading = useCoreLoadingStore()
const nuxtApp = useNuxtApp()
const router = useRouter()

const NAV_LABEL = 'Carregando página…'
let navLoading = false

nuxtApp.hook('page:loading:start', () => {
  if (!navLoading) {
    navLoading = true
    loading.push(NAV_LABEL)
  }
})

nuxtApp.hook('page:loading:end', () => {
  if (navLoading) {
    navLoading = false
    loading.pop(NAV_LABEL)
  }
})

router.afterEach((_to, _from, failure) => {
  if (failure && navLoading) {
    navLoading = false
    loading.pop(NAV_LABEL)
  }
})

nuxtApp.hook('page:start', () => {
  loading.push()
})

nuxtApp.hook('page:finish', () => {
  loading.pop()
})

nuxtApp.hook('vue:error', () => {
  loading.reset()
})
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
      <PwaReloadPrompt />
    </ClientOnly>
  </UApp>
</template>
