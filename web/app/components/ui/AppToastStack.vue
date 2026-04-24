<script setup>
import { storeToRefs } from "pinia";
import { useUiStore } from "~/stores/ui";

const ui = useUiStore();
const { toasts } = storeToRefs(ui);
</script>

<template>
  <Teleport to="body">
    <div class="ui-toast-stack" aria-live="polite" aria-atomic="true" data-testid="ui-toast-stack">
      <article
        v-for="toast in toasts"
        :key="toast.id"
        class="ui-toast"
        :class="`ui-toast--${toast.type}`"
        :data-testid="`ui-toast-${toast.type}`"
      >
        <div class="ui-toast__content">
          <strong v-if="toast.title" class="ui-toast__title">{{ toast.title }}</strong>
          <p class="ui-toast__message">{{ toast.message }}</p>
        </div>
        <button
          class="ui-toast__close"
          type="button"
          aria-label="Fechar"
          :data-testid="`ui-toast-close-${toast.id}`"
          @click="ui.dismissToast(toast.id)"
        >
          X
        </button>
      </article>
    </div>
  </Teleport>
</template>
