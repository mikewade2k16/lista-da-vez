<script setup>
import { computed, ref, watch } from "vue";
import { storeToRefs } from "pinia";
import { useUiStore } from "~/stores/ui";

const ui = useUiStore();
const { dialog } = storeToRefs(ui);
const promptValue = ref("");

const isPrompt = computed(() => dialog.value?.kind === "prompt");
const isAlert = computed(() => dialog.value?.kind === "alert");
const isConfirmDisabled = computed(() =>
  isPrompt.value && dialog.value?.required ? !promptValue.value.trim() : false
);

watch(
  dialog,
  (nextDialog) => {
    promptValue.value = nextDialog?.initialValue || "";
  },
  { immediate: true }
);

function closeDialog() {
  ui.cancelDialog();
}

function submitDialog() {
  if (isConfirmDisabled.value) {
    return;
  }

  ui.submitDialog(isPrompt.value ? promptValue.value.trim() : "true");
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="dialog"
      class="ui-dialog-backdrop"
      data-testid="ui-dialog-backdrop"
      @click.self="closeDialog"
    >
      <div
        class="ui-dialog"
        role="dialog"
        aria-modal="true"
        :aria-labelledby="dialog.id"
        data-testid="ui-dialog"
      >
        <header class="ui-dialog__header">
          <h2 :id="dialog.id" class="ui-dialog__title">{{ dialog.title }}</h2>
          <button
            class="ui-dialog__close"
            type="button"
            aria-label="Fechar"
            data-testid="ui-dialog-close"
            @click="closeDialog"
          >
            X
          </button>
        </header>

        <form class="ui-dialog__body" @submit.prevent="submitDialog">
          <p class="ui-dialog__message">{{ dialog.message }}</p>

          <label v-if="isPrompt" class="ui-dialog__field">
            <span v-if="dialog.inputLabel" class="ui-dialog__label">{{ dialog.inputLabel }}</span>
            <input
              v-model="promptValue"
              class="finish-form__input"
              type="text"
              :placeholder="dialog.inputPlaceholder"
              data-testid="ui-dialog-input"
              autofocus
            >
          </label>

          <div class="ui-dialog__actions">
            <button
              v-if="!isAlert"
              class="column-action column-action--secondary"
              type="button"
              data-testid="ui-dialog-cancel"
              @click="closeDialog"
            >
              {{ dialog.cancelLabel }}
            </button>
            <button
              class="column-action column-action--primary"
              type="submit"
              :disabled="isConfirmDisabled"
              data-testid="ui-dialog-confirm"
            >
              {{ dialog.confirmLabel }}
            </button>
          </div>
        </form>
      </div>
    </div>
  </Teleport>
</template>
