<script setup lang="ts">
import { computed } from "vue";

const props = withDefaults(
  defineProps<{
    title?: string;
    description?: string;
    cardWidth?: string;
  }>(),
  {
    title: "",
    description: "",
    cardWidth: "28rem"
  }
);

const showHeader = computed(() => Boolean(props.title || props.description));
</script>

<template>
  <div class="admin-auth-page">
    <div class="admin-auth-glow admin-auth-glow--left" />
    <div class="admin-auth-glow admin-auth-glow--right" />

    <section class="admin-auth-card" :style="{ '--admin-auth-card-width': props.cardWidth }">
      <div class="admin-auth-brand">
        <picture>
          <source srcset="/logo.avif" type="image/avif">
          <source srcset="/logo.webp" type="image/webp">
          <img src="/logo.png" alt="Logo da plataforma" class="admin-auth-brand__logo">
        </picture>

        <div class="admin-auth-brand__copy" aria-hidden="true" hidden>
          <span class="admin-auth-brand__name">Plataforma</span>
          <span class="admin-auth-brand__eyebrow">acesso administrativo</span>
        </div>
      </div>

      <header v-if="showHeader" class="admin-auth-header">
        <h1 v-if="props.title" class="admin-auth-title">{{ props.title }}</h1>
        <p v-if="props.description" class="admin-auth-description">{{ props.description }}</p>
      </header>

      <slot />
    </section>
  </div>
</template>