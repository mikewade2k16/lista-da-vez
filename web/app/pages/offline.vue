<script setup lang="ts">
// Fallback offline do PWA (PWA-01). Precisa funcionar sem hidratacao,
// portanto nao usa stores, API ou fontes externas.
definePageMeta({
  layout: false,
})
</script>

<template>
  <main class="offline-page">
    <p class="offline-page__brand">Omni</p>
    <h1 class="offline-page__title">Você está offline</h1>
    <p class="offline-page__text">
      Não foi possível falar com o servidor. Verifique sua conexão com a internet e tente novamente.
    </p>
    <a class="offline-page__retry" href="/">Tentar novamente</a>
  </main>
</template>

<style scoped>
.offline-page {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.8rem;
  padding: 2rem;
  text-align: center;
  background: #060a12;
  color: #e5e9f2;
}

.offline-page__brand {
  margin: 0;
  color: #6366f1;
  font-size: 0.9rem;
  letter-spacing: 0.3em;
  text-transform: uppercase;
}

.offline-page__title {
  margin: 0;
  font-size: 1.6rem;
}

.offline-page__text {
  max-width: 28rem;
  margin: 0;
  color: #9aa3b5;
}

.offline-page__retry {
  margin-top: 0.6rem;
  padding: 0.6rem 1.4rem;
  border-radius: 999px;
  background: #6366f1;
  color: #fff;
  font-weight: 600;
  text-decoration: none;
}
</style>
