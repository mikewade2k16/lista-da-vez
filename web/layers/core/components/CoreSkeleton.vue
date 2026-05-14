<script setup lang="ts">
import { computed } from "vue";

type Variant = "card" | "table-row" | "text" | "avatar" | "block";

const props = withDefaults(
  defineProps<{
    variant?: Variant;
    width?: string;
    height?: string;
    rounded?: boolean;
    count?: number;
  }>(),
  {
    variant: "block",
    width: "",
    height: "",
    rounded: true,
    count: 1
  }
);

const items = computed(() => Array.from({ length: Math.max(1, props.count) }));

const inlineStyle = computed(() => {
  const style: Record<string, string> = {};
  if (props.width) style.width = props.width;
  if (props.height) style.height = props.height;
  return style;
});

const wrapperClass = computed(() => ({
  "core-skeleton": true,
  [`core-skeleton--${props.variant}`]: true,
  "core-skeleton--rounded": props.rounded
}));
</script>

<template>
  <div class="core-skeleton-group" :class="{ 'is-row': variant === 'table-row' }">
    <div
      v-for="(_, idx) in items"
      :key="idx"
      :class="wrapperClass"
      :style="inlineStyle"
      aria-hidden="true"
    >
      <template v-if="variant === 'card'">
        <div class="core-skeleton__line core-skeleton__line--lg" />
        <div class="core-skeleton__line core-skeleton__line--md" />
        <div class="core-skeleton__line core-skeleton__line--sm" />
      </template>
      <template v-else-if="variant === 'table-row'">
        <div class="core-skeleton__cell" />
        <div class="core-skeleton__cell core-skeleton__cell--wide" />
        <div class="core-skeleton__cell" />
        <div class="core-skeleton__cell core-skeleton__cell--narrow" />
      </template>
    </div>
  </div>
</template>

<style scoped>
.core-skeleton-group {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
  width: 100%;
}

.core-skeleton {
  position: relative;
  background: rgba(148, 163, 184, 0.10);
  overflow: hidden;
}

.core-skeleton::after {
  content: "";
  position: absolute;
  inset: 0;
  transform: translateX(-100%);
  background: linear-gradient(
    90deg,
    rgba(255, 255, 255, 0) 0%,
    rgba(255, 255, 255, 0.06) 50%,
    rgba(255, 255, 255, 0) 100%
  );
  animation: core-skeleton-shimmer 1.4s ease-in-out infinite;
}

.core-skeleton--rounded {
  border-radius: 8px;
}

.core-skeleton--block {
  height: 1.2rem;
}

.core-skeleton--text {
  height: 0.85rem;
  width: 70%;
}

.core-skeleton--avatar {
  width: 2.5rem;
  height: 2.5rem;
  border-radius: 999px;
}

.core-skeleton--card {
  padding: 1rem;
  min-height: 7rem;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  border: 1px solid rgba(148, 163, 184, 0.08);
  border-radius: 12px;
}

.core-skeleton--card::after {
  border-radius: 12px;
}

.core-skeleton--table-row {
  display: grid;
  grid-template-columns: 1fr 2fr 1fr 0.6fr;
  gap: 0.7rem;
  align-items: center;
  padding: 0.75rem 0.6rem;
  border-bottom: 1px solid rgba(148, 163, 184, 0.05);
  background: transparent;
}

.core-skeleton--table-row::after {
  display: none;
}

.core-skeleton__line {
  background: rgba(148, 163, 184, 0.18);
  border-radius: 4px;
  height: 0.7rem;
  position: relative;
  overflow: hidden;
}

.core-skeleton__line::after {
  content: "";
  position: absolute;
  inset: 0;
  transform: translateX(-100%);
  background: linear-gradient(
    90deg,
    rgba(255, 255, 255, 0) 0%,
    rgba(255, 255, 255, 0.07) 50%,
    rgba(255, 255, 255, 0) 100%
  );
  animation: core-skeleton-shimmer 1.4s ease-in-out infinite;
}

.core-skeleton__line--lg {
  width: 60%;
  height: 1rem;
}

.core-skeleton__line--md {
  width: 85%;
}

.core-skeleton__line--sm {
  width: 40%;
}

.core-skeleton__cell {
  height: 0.85rem;
  background: rgba(148, 163, 184, 0.16);
  border-radius: 4px;
  position: relative;
  overflow: hidden;
}

.core-skeleton__cell::after {
  content: "";
  position: absolute;
  inset: 0;
  transform: translateX(-100%);
  background: linear-gradient(
    90deg,
    rgba(255, 255, 255, 0) 0%,
    rgba(255, 255, 255, 0.07) 50%,
    rgba(255, 255, 255, 0) 100%
  );
  animation: core-skeleton-shimmer 1.4s ease-in-out infinite;
}

.core-skeleton__cell--wide {
  height: 1rem;
}

.core-skeleton__cell--narrow {
  height: 0.85rem;
  width: 60%;
}

@keyframes core-skeleton-shimmer {
  0% {
    transform: translateX(-100%);
  }
  100% {
    transform: translateX(100%);
  }
}
</style>
