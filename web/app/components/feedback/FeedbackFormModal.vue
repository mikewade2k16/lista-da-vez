<script setup lang="ts">
import { ref, watch, onMounted, onBeforeUnmount } from 'vue'
import { ImagePlus, X } from 'lucide-vue-next'
import { useFeedbackStore } from '~/stores/feedback'
import { useUiStore } from '~/stores/ui'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import { compressFeedbackImage, formatFeedbackImageSize } from '~/utils/feedback-image'

const props = defineProps({
  modelValue: {
    type: Boolean,
    default: false,
  },
})

const emit = defineEmits(['update:modelValue'])

const feedbackStore = useFeedbackStore()
const ui = useUiStore()

const kind = ref('')
const subject = ref('')
const body = ref('')
const selectedImage = ref<File | null>(null)
const selectedImagePreviewUrl = ref('')
const submitting = ref(false)

let previousBodyOverflow = ''

const kindOptions = [
  { value: 'suggestion', label: 'Sugestão' },
  { value: 'question', label: 'Dúvida' },
  { value: 'problem', label: 'Problema' },
]

function closeModal() {
  emit('update:modelValue', false)
}

function resetForm() {
  kind.value = ''
  subject.value = ''
  body.value = ''
  clearSelectedImage()
}

function setSelectedImage(file: File | null) {
  if (import.meta.client && selectedImagePreviewUrl.value) {
    URL.revokeObjectURL(selectedImagePreviewUrl.value)
  }

  selectedImage.value = file
  selectedImagePreviewUrl.value = file && import.meta.client ? URL.createObjectURL(file) : ''
}

function clearSelectedImage() {
  setSelectedImage(null)
}

async function handleImageChange(event: Event) {
  const target = event.target as HTMLInputElement
  const file = target.files?.[0] || null
  if (!file) {
    return
  }

  try {
    const compressedImage = await compressFeedbackImage(file)
    setSelectedImage(compressedImage)
  } catch (err) {
    ui.error(err instanceof Error ? err.message : 'Nao foi possivel preparar a imagem.')
  } finally {
    target.value = ''
  }
}

function syncBodyScrollLock(isOpen) {
  if (!import.meta.client) {
    return
  }

  if (isOpen) {
    previousBodyOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return
  }

  document.body.style.overflow = previousBodyOverflow
}

function handleEscape(event) {
  if (event.key === 'Escape' && props.modelValue) {
    closeModal()
  }
}

onMounted(() => {
  document.addEventListener('keydown', handleEscape)
})

watch(
  () => props.modelValue,
  (isOpen) => {
    if (!isOpen) {
      resetForm()
    }
    syncBodyScrollLock(isOpen)
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  document.removeEventListener('keydown', handleEscape)
  syncBodyScrollLock(false)
  clearSelectedImage()
})

async function handleSubmit() {
  if (!kind.value || !subject.value.trim() || !body.value.trim()) {
    ui.error('Preencha todos os campos obrigatórios')
    return
  }

  submitting.value = true
  try {
    const result = await feedbackStore.submitFeedback({
      kind: kind.value,
      subject: subject.value.trim(),
      body: body.value.trim(),
      image: selectedImage.value,
    })

    if (result.ok) {
      ui.success('Feedback enviado com sucesso!')
      closeModal()
    } else {
      ui.error(result.message || 'Erro ao enviar feedback')
    }
  } finally {
    submitting.value = false
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
              <button
                class="feedback-form-modal__close-btn"
                aria-label="Fechar"
                @click="closeModal"
              >
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

              <div class="feedback-form-modal__field">
                <label class="feedback-form-modal__label">Imagem do problema</label>
                <div class="feedback-form-modal__upload-card">
                  <label class="feedback-form-modal__upload-trigger">
                    <input
                      type="file"
                      accept="image/png,image/jpeg,image/webp"
                      hidden
                      :disabled="submitting"
                      @change="handleImageChange"
                    />
                    <ImagePlus :size="16" :stroke-width="2.1" />
                    <span>{{ selectedImage ? 'Trocar imagem' : 'Adicionar imagem' }}</span>
                  </label>
                  <small class="feedback-form-modal__upload-hint">
                    Opcional. A imagem e compactada antes do envio e apagada 7 dias apos o
                    encerramento do chamado.
                  </small>

                  <div v-if="selectedImagePreviewUrl" class="feedback-form-modal__upload-preview">
                    <img
                      :src="selectedImagePreviewUrl"
                      alt="Preview da imagem anexada"
                      class="feedback-form-modal__upload-image"
                    />
                    <div class="feedback-form-modal__upload-copy">
                      <strong>{{ selectedImage?.name }}</strong>
                      <span>{{ formatFeedbackImageSize(selectedImage?.size || 0) }}</span>
                    </div>
                    <button
                      type="button"
                      class="feedback-form-modal__upload-remove"
                      :disabled="submitting"
                      @click="clearSelectedImage"
                    >
                      <X :size="14" :stroke-width="2.2" />
                    </button>
                  </div>
                </div>
              </div>

              <div class="feedback-form-modal__actions">
                <button
                  type="button"
                  class="feedback-form-modal__btn feedback-form-modal__btn--secondary"
                  :disabled="submitting"
                  @click="closeModal"
                >
                  Cancelar
                </button>
                <button
                  type="submit"
                  class="feedback-form-modal__btn feedback-form-modal__btn--primary"
                  :disabled="submitting || !kind || !subject.trim() || !body.trim()"
                >
                  {{ submitting ? 'Enviando...' : 'Enviar' }}
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
  background: rgb(3 6 12 / 0.76);
  backdrop-filter: blur(4px);
}

.feedback-form-modal__dialog {
  position: relative;
  z-index: 1;
  max-height: calc(100vh - 2rem);
  overflow: auto;
  overscroll-behavior: contain;
  border-radius: 1.2rem;
  border: 1px solid rgb(var(--border) / 0.74);
  background: linear-gradient(180deg, rgb(var(--surface)), rgb(var(--surface-2)));
  box-shadow: var(--shadow-dropdown, 0 30px 80px rgb(15 23 42 / 0.22));
  width: min(42rem, calc(100vw - 2rem));
}

.feedback-form-modal__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  padding: 1.2rem 1.2rem 0.9rem;
  border-bottom: 1px solid rgb(var(--border) / 0.74);
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
  color: rgb(var(--muted) / 0.82);
}

.feedback-form-modal__title {
  margin: 0;
  color: rgb(var(--text));
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
  border: 1px solid rgb(var(--border) / 0.78);
  background: rgb(var(--surface-2) / 0.74);
  color: rgb(var(--muted));
  cursor: pointer;
  transition: all 0.2s ease;
  flex-shrink: 0;
}

.feedback-form-modal__close-btn:hover {
  background: rgb(var(--primary) / 0.12);
  color: rgb(var(--primary));
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
  color: rgb(var(--text));
  letter-spacing: 0.02em;
}

.feedback-form-modal__input,
.feedback-form-modal__textarea {
  width: 100%;
  padding: 0.75rem 0.9rem;
  border: 1px solid rgb(var(--border) / 0.84);
  border-radius: 0.7rem;
  background: rgb(var(--surface-2) / 0.76);
  font-family: inherit;
  font-size: 0.875rem;
  color: rgb(var(--text));
  transition: all 0.2s ease;
}

.feedback-form-modal__input::placeholder,
.feedback-form-modal__textarea::placeholder {
  color: rgb(var(--muted) / 0.5);
}

.feedback-form-modal__input:focus,
.feedback-form-modal__textarea:focus {
  outline: none;
  border-color: rgb(var(--ring) / 0.4);
  background: rgb(var(--surface-2) / 0.92);
  box-shadow: 0 0 0 3px rgb(var(--ring) / 0.15);
}

.feedback-form-modal__textarea {
  resize: vertical;
  min-height: 110px;
  font-family: inherit;
}

.feedback-form-modal__upload-card {
  display: grid;
  gap: 0.65rem;
  padding: 0.85rem;
  border: 1px dashed rgb(var(--border) / 0.82);
  border-radius: 0.8rem;
  background: rgb(var(--surface-2) / 0.64);
}

.feedback-form-modal__upload-trigger {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  width: fit-content;
  padding: 0.6rem 0.8rem;
  border-radius: 0.7rem;
  border: 1px solid rgb(var(--ring) / 0.22);
  background: rgb(var(--primary) / 0.12);
  color: rgb(var(--primary));
  font-size: 0.82rem;
  font-weight: 600;
  cursor: pointer;
}

.feedback-form-modal__upload-hint {
  color: rgb(var(--muted) / 0.74);
  font-size: 0.76rem;
}

.feedback-form-modal__upload-preview {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.75rem;
  padding: 0.6rem;
  border-radius: 0.75rem;
  background: rgb(var(--surface-2) / 0.76);
}

.feedback-form-modal__upload-image {
  width: 4rem;
  height: 4rem;
  object-fit: cover;
  border-radius: 0.65rem;
  border: 1px solid rgb(var(--border) / 0.78);
}

.feedback-form-modal__upload-copy {
  min-width: 0;
  display: grid;
  gap: 0.2rem;
}

.feedback-form-modal__upload-copy strong,
.feedback-form-modal__upload-copy span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.feedback-form-modal__upload-copy strong {
  color: rgb(var(--text));
  font-size: 0.82rem;
}

.feedback-form-modal__upload-copy span {
  color: rgb(var(--muted) / 0.72);
  font-size: 0.74rem;
}

.feedback-form-modal__upload-remove {
  width: 2rem;
  height: 2rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid rgb(var(--border) / 0.78);
  border-radius: 999px;
  background: rgb(var(--surface-2) / 0.7);
  color: rgb(var(--muted));
  cursor: pointer;
}

.feedback-form-modal__upload-remove:disabled {
  opacity: 0.45;
  cursor: not-allowed;
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
  background: linear-gradient(135deg, rgb(var(--primary)) 0%, rgb(var(--primary-600)) 100%);
  color: rgb(255 255 255);
  border-color: rgb(var(--ring) / 0.3);
}

.feedback-form-modal__btn--primary:hover:not(:disabled) {
  background: linear-gradient(135deg, rgb(var(--primary-600)) 0%, rgb(var(--primary)) 100%);
  box-shadow: 0 8px 20px rgb(var(--primary) / 0.25);
}

.feedback-form-modal__btn--primary:active:not(:disabled) {
  transform: scale(0.98);
}

.feedback-form-modal__btn--secondary {
  background: rgb(var(--surface-2) / 0.74);
  color: rgb(var(--text) / 0.9);
  border-color: rgb(var(--border) / 0.84);
}

.feedback-form-modal__btn--secondary:hover:not(:disabled) {
  background: rgb(var(--primary) / 0.12);
  border-color: rgb(var(--ring) / 0.28);
}

.feedback-form-modal__btn--secondary:active:not(:disabled) {
  transform: scale(0.98);
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
