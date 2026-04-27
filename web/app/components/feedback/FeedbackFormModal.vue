<script setup lang="ts">
import { ref, watch, onMounted, onBeforeUnmount } from "vue";
import { X } from "lucide-vue-next";
import { useFeedbackStore } from "~/stores/feedback";
import { useUiStore } from "~/stores/ui";
import AppSelectField from "~/components/ui/AppSelectField.vue";

const props = defineProps({
  modelValue: {
    type: Boolean,
    default: false
  }
});

const emit = defineEmits(["update:modelValue"]);

const feedbackStore = useFeedbackStore();
const ui = useUiStore();

const kind = ref("");
const subject = ref("");
const body = ref("");
const submitting = ref(false);

let previousBodyOverflow = "";

const kindOptions = [
  { value: "suggestion", label: "Sugestão" },
  { value: "question", label: "Dúvida" },
  { value: "problem", label: "Problema" }
];

function closeModal() {
  emit("update:modelValue", false);
}

function resetForm() {
  kind.value = "";
  subject.value = "";
  body.value = "";
}

function syncBodyScrollLock(isOpen) {
  if (!import.meta.client) {
    return;
  }

  if (isOpen) {
    previousBodyOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return;
  }

  document.body.style.overflow = previousBodyOverflow;
}

function handleEscape(event) {
  if (event.key === "Escape" && props.modelValue) {
    closeModal();
  }
}

onMounted(() => {
  document.addEventListener("keydown", handleEscape);
});

watch(
  () => props.modelValue,
  (isOpen) => {
    if (!isOpen) {
      resetForm();
    }
    syncBodyScrollLock(isOpen);
  },
  { immediate: true }
);

onBeforeUnmount(() => {
  document.removeEventListener("keydown", handleEscape);
  syncBodyScrollLock(false);
});

async function handleSubmit() {
  if (!kind.value || !subject.value.trim() || !body.value.trim()) {
    ui.error("Preencha todos os campos obrigatórios");
    return;
  }

  submitting.value = true;
  try {
    const result = await feedbackStore.submitFeedback({
      kind: kind.value,
      subject: subject.value.trim(),
      body: body.value.trim()
    });

    if (result.ok) {
      ui.success("Feedback enviado com sucesso!");
      closeModal();
    } else {
      ui.error(result.message || "Erro ao enviar feedback");
    }
  } finally {
    submitting.value = false;
  }
}
</script>

<template>
  <Teleport to="body">
    <Transition name="feedback-modal-fade">
      <div v-if="modelValue" class="feedback-form-modal__overlay" @click.self="closeModal">
        <Transition name="feedback-modal-slide">
          <div v-if="modelValue" class="feedback-form-modal__dialog">
            <div class="feedback-form-modal__header">
              <div class="feedback-form-modal__copy">
                <p class="feedback-form-modal__eyebrow">Comunicação</p>
                <h2 class="feedback-form-modal__title">Enviar Feedback</h2>
              </div>
              <button class="feedback-form-modal__close-btn" @click="closeModal" aria-label="Fechar">
                <X :size="18" :stroke-width="2.1" />
              </button>
            </div>

            <form class="feedback-form-modal__form" @submit.prevent="handleSubmit">
              <div class="feedback-form-modal__field">
                <label class="feedback-form-modal__label">Tipo</label>
                <AppSelectField
                  v-model="kind"
                  :options="kindOptions"
                  placeholder="Selecione o tipo"
                  required
                />
              </div>

              <div class="feedback-form-modal__field">
                <label class="feedback-form-modal__label">Assunto</label>
                <input
                  v-model="subject"
                  type="text"
                  class="feedback-form-modal__input"
                  placeholder="Descreva brevemente o assunto"
                  required
                />
              </div>

              <div class="feedback-form-modal__field">
                <label class="feedback-form-modal__label">Mensagem</label>
                <textarea
                  v-model="body"
                  class="feedback-form-modal__textarea"
                  placeholder="Descreva sua sugestão, dúvida ou problema em detalhes"
                  rows="5"
                  required
                ></textarea>
              </div>

              <div class="feedback-form-modal__actions">
                <button
                  type="button"
                  class="feedback-form-modal__btn feedback-form-modal__btn--secondary"
                  @click="closeModal"
                  :disabled="submitting"
                >
                  Cancelar
                </button>
                <button
                  type="submit"
                  class="feedback-form-modal__btn feedback-form-modal__btn--primary"
                  :disabled="submitting || !kind || !subject.trim() || !body.trim()"
                >
                  {{ submitting ? "Enviando..." : "Enviar" }}
                </button>
              </div>
            </form>
          </div>
        </Transition>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.feedback-form-modal__overlay {
  position: fixed;
  inset: 0;
  z-index: 80;
  display: grid;
  place-items: center;
  padding: 1rem;
  background: rgba(3, 6, 12, 0.76);
  backdrop-filter: blur(4px);
}

.feedback-form-modal__dialog {
  position: relative;
  z-index: 1;
  max-height: calc(100vh - 2rem);
  overflow: auto;
  overscroll-behavior: contain;
  border-radius: 1.2rem;
  border: 1px solid rgba(226, 232, 240, 0.1);
  background: linear-gradient(180deg, rgba(13, 18, 29, 0.98), rgba(8, 12, 19, 0.98));
  box-shadow: 0 30px 80px rgba(0, 0, 0, 0.42);
  width: min(42rem, calc(100vw - 2rem));
}

.feedback-form-modal__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  padding: 1.2rem 1.2rem 0.9rem;
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

.feedback-form-modal__copy {
  display: grid;
  gap: 0.3rem;
}

.feedback-form-modal__eyebrow {
  margin: 0;
  font-size: 0.72rem;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: rgba(148, 163, 184, 0.82);
}

.feedback-form-modal__title {
  margin: 0;
  color: #ffffff;
  font-size: 1.15rem;
  font-weight: 600;
}

.feedback-form-modal__close-btn {
  width: 2.45rem;
  height: 2.45rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  border: 1px solid rgba(255, 255, 255, 0.08);
  background: rgba(255, 255, 255, 0.04);
  color: rgba(226, 232, 240, 0.7);
  cursor: pointer;
  transition: all 0.2s ease;
  flex-shrink: 0;
}

.feedback-form-modal__close-btn:hover {
  background: rgba(255, 255, 255, 0.08);
  color: #ffffff;
}

.feedback-form-modal__close-btn:active {
  transform: scale(0.95);
}

.feedback-form-modal__form {
  display: grid;
  gap: 1rem;
  padding: 1.2rem;
}

.feedback-form-modal__field {
  display: grid;
  gap: 0.5rem;
}

.feedback-form-modal__label {
  font-size: 0.85rem;
  font-weight: 500;
  color: #ffffff;
  letter-spacing: 0.02em;
}

.feedback-form-modal__input,
.feedback-form-modal__textarea {
  width: 100%;
  padding: 0.75rem 0.9rem;
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 0.7rem;
  background: rgba(18, 25, 38, 0.6);
  font-family: inherit;
  font-size: 0.875rem;
  color: #ffffff;
  transition: all 0.2s ease;
}

.feedback-form-modal__input::placeholder,
.feedback-form-modal__textarea::placeholder {
  color: rgba(203, 213, 225, 0.5);
}

.feedback-form-modal__input:focus,
.feedback-form-modal__textarea:focus {
  outline: none;
  border-color: rgba(148, 163, 184, 0.4);
  background: rgba(18, 25, 38, 0.9);
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.15);
}

.feedback-form-modal__textarea {
  resize: vertical;
  min-height: 110px;
  font-family: inherit;
}

.feedback-form-modal__actions {
  display: flex;
  gap: 0.75rem;
  justify-content: flex-end;
  padding-top: 0.5rem;
}

.feedback-form-modal__btn {
  padding: 0.65rem 1.2rem;
  border-radius: 0.7rem;
  font-size: 0.85rem;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
  border: 1px solid transparent;
}

.feedback-form-modal__btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.feedback-form-modal__btn--primary {
  background: linear-gradient(135deg, #3b82f6 0%, #2563eb 100%);
  color: #ffffff;
  border-color: rgba(59, 130, 246, 0.3);
}

.feedback-form-modal__btn--primary:hover:not(:disabled) {
  background: linear-gradient(135deg, #2563eb 0%, #1d4ed8 100%);
  box-shadow: 0 8px 20px rgba(59, 130, 246, 0.25);
}

.feedback-form-modal__btn--primary:active:not(:disabled) {
  transform: scale(0.98);
}

.feedback-form-modal__btn--secondary {
  background: rgba(255, 255, 255, 0.08);
  color: rgba(226, 232, 240, 0.9);
  border-color: rgba(255, 255, 255, 0.1);
}

.feedback-form-modal__btn--secondary:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.12);
  border-color: rgba(255, 255, 255, 0.2);
}

.feedback-form-modal__btn--secondary:active:not(:disabled) {
  transform: scale(0.98);
}

@keyframes feedback-modal-fade-enter-active {
  animation: fadeIn 0.3s cubic-bezier(0.34, 1.56, 0.64, 1);
}

@keyframes feedback-modal-fade-leave-active {
  animation: fadeOut 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}

@keyframes feedback-modal-slide-enter-active {
  animation: slideUp 0.4s cubic-bezier(0.34, 1.56, 0.64, 1) 0.1s both;
}

@keyframes feedback-modal-slide-leave-active {
  animation: slideDown 0.3s cubic-bezier(0.4, 0, 0.2, 1) both;
}

@keyframes fadeIn {
  from {
    opacity: 0;
  }
  to {
    opacity: 1;
  }
}

@keyframes fadeOut {
  from {
    opacity: 1;
  }
  to {
    opacity: 0;
  }
}

@keyframes slideUp {
  from {
    opacity: 0;
    transform: translateY(20px) scale(0.95);
  }
  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

@keyframes slideDown {
  from {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
  to {
    opacity: 0;
    transform: translateY(20px) scale(0.95);
  }
}

.feedback-modal-fade-enter-active,
.feedback-modal-fade-leave-active {
  transition: opacity 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.feedback-modal-fade-enter-from,
.feedback-modal-fade-leave-to {
  opacity: 0;
}

.feedback-modal-slide-enter-active {
  animation: slideUp 0.4s cubic-bezier(0.34, 1.56, 0.64, 1) 0.1s;
}

.feedback-modal-slide-leave-active {
  animation: slideDown 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

@media (max-width: 640px) {
  .feedback-form-modal__overlay {
    padding: 1rem;
  }

  .feedback-form-modal__dialog {
    width: calc(100vw - 1rem) !important;
  }

  .feedback-form-modal__form {
    padding: 1rem;
  }

  .feedback-form-modal__textarea {
    min-height: 90px;
  }

  .feedback-form-modal__actions {
    gap: 0.5rem;
  }

  .feedback-form-modal__btn {
    padding: 0.6rem 1rem;
    font-size: 0.8rem;
  }
}
</style>
