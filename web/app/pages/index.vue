<script setup>
import { useAuthStore } from '~/stores/auth'

definePageMeta({
  layout: false,
})

const auth = useAuthStore()
await auth.ensureSession()
await navigateTo(auth.isAuthenticated ? auth.homePath : '/auth/login')
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
