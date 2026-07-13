<script setup>
import { useAuthStore } from '~/stores/auth'

// '/' e so um distribuidor: manda pra home do papel (ou login). O redirect vive
// num MIDDLEWARE (roda ANTES de montar a pagina), nao mais como `await
// navigateTo` no corpo do setup. O await no setup roda DENTRO do Suspense do
// NuxtPage e prendia a tela numa transicao vazia (tela preta) ate resolver — o
// mesmo limbo que aparecia ao abrir o painel com o chunk ainda frio. O
// auth.global ja rodou ensureSession + hidratou accounts nesta mesma navegacao;
// aqui so lemos o estado resolvido e roteamos.
definePageMeta({
  layout: false,
  middleware() {
    if (import.meta.server) return
    const auth = useAuthStore()
    if (!auth.isAuthenticated) return navigateTo('/auth/login', { replace: true })
    if (auth.mustChangePassword) return navigateTo('/perfil', { replace: true })
    return navigateTo(auth.homePath, { replace: true })
  },
})
</script>

<template>
  <!--
    Fase 9 (apply-dashboard): esqueleto dos cards iniciais do dashboard.
    A pagina '/' so redireciona para auth.homePath, mas entre o clique e a
    resolucao do redirect a tela ficava em branco. O skeleton e aditivo:
    aparece imediatamente (< 100ms) e e descartado quando o redirect resolve
    e a rota de destino monta. Nenhum comportamento de redirect foi alterado.
  -->
  <main class="dashboard-skeleton" aria-busy="true" aria-live="polite">
    <span class="dashboard-skeleton__sr">Carregando o painel...</span>
    <header class="dashboard-skeleton__header">
      <CoreSkeleton variant="text" width="14rem" height="1.4rem" />
      <CoreSkeleton variant="block" width="9rem" height="2.4rem" />
    </header>
    <section class="dashboard-skeleton__cards">
      <CoreSkeleton v-for="card in 6" :key="`card-${card}`" variant="card" />
    </section>
  </main>
</template>

<style scoped>
.dashboard-skeleton {
  display: flex;
  flex-direction: column;
  gap: 1.2rem;
  width: 100%;
  max-width: 80rem;
  margin: 0 auto;
  padding: 1.6rem;
}

.dashboard-skeleton__sr {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}

.dashboard-skeleton__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  flex-wrap: wrap;
}

.dashboard-skeleton__cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(15rem, 1fr));
  gap: 1rem;
}
</style>
