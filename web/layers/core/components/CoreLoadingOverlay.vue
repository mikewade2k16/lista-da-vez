<script setup lang="ts">
import { computed } from "vue";
import { useCoreLoadingStore } from "../stores/loading";

const store = useCoreLoadingStore();

const visible = computed(() => store.isLoading);
const label = computed(() => store.activeLabel);
</script>

<template>
  <transition name="core-loading-fade">
    <div v-if="visible" class="core-loading-overlay" role="status" :aria-label="label || 'Carregando'">
      <div class="core-loading-overlay__bar">
        <div class="core-loading-overlay__bar-fill" />
      </div>
      <span v-if="label" class="core-loading-overlay__label">{{ label }}</span>
    </div>
  </transition>
</template>

<style scoped>
.core-loading-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  z-index: 9999;
  pointer-events: none;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.4rem;
  padding-top: 0;
}

.core-loading-overlay__bar {
  width: 100%;
  height: 3px;
  background: rgba(99, 102, 241, 0.12);
  overflow: hidden;
}

.core-loading-overlay__bar-fill {
  display: block;
  height: 100%;
  width: 30%;
  background: linear-gradient(
    90deg,
    rgba(99, 102, 241, 0) 0%,
    rgba(99, 102, 241, 0.95) 50%,
    rgba(129, 140, 248, 0) 100%
  );
  animation: core-loading-slide 1.1s ease-in-out infinite;
  border-radius: 2px;
}

.core-loading-overlay__label {
  margin-top: 0.5rem;
  padding: 0.25rem 0.7rem;
  font-size: 0.72rem;
  color: rgba(226, 232, 240, 0.85);
  background: rgba(13, 18, 29, 0.82);
  border: 1px solid rgba(99, 102, 241, 0.25);
  border-radius: 999px;
  box-shadow: 0 4px 14px rgba(2, 6, 23, 0.35);
}

@keyframes core-loading-slide {
  0% {
    transform: translateX(-100%);
  }
  100% {
    transform: translateX(400%);
  }
}

.core-loading-fade-enter-active,
.core-loading-fade-leave-active {
  transition: opacity 0.18s ease;
}

.core-loading-fade-enter-from,
.core-loading-fade-leave-to {
  opacity: 0;
}
</style>
