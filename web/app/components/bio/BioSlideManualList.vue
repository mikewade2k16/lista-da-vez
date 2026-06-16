<script setup lang="ts">
// Lista de slides manuais (B7): grid de cards lado a lado, cada um com imagem
// obrigatoria + textos + acoes (mover/remover). A ORDEM da lista e a ordem de
// exibicao. So edita a lista de slides; a fonte/modo/botao ficam na secao pai
// (BioSectionSlides). Extraido para manter a secao abaixo do limite de linhas.
import BioMediaField from '~/components/bio/BioMediaField.vue'
import type { BioMediaKind, BioSlide } from '~/domain/bio/types'

const props = defineProps<{
  slides: BioSlide[]
  uploadMedia: (kind: BioMediaKind, file: File) => Promise<string | null>
  isUploading: (kind: BioMediaKind) => boolean
}>()

const emit = defineEmits<{ (e: 'update:slides', value: BioSlide[]): void }>()

function updateSlide<K extends keyof BioSlide>(index: number, key: K, value: BioSlide[K]) {
  emit(
    'update:slides',
    props.slides.map((slide, position) =>
      position === index ? { ...slide, [key]: value } : slide,
    ),
  )
}

function addSlide() {
  emit('update:slides', [...props.slides, { src: '' }])
}

function removeSlide(index: number) {
  emit(
    'update:slides',
    props.slides.filter((_, position) => position !== index),
  )
}

function moveSlide(index: number, direction: -1 | 1) {
  const target = index + direction
  if (target < 0 || target >= props.slides.length) {
    return
  }
  const next = [...props.slides]
  const [moved] = next.splice(index, 1)
  next.splice(target, 0, moved)
  emit('update:slides', next)
}
</script>

<template>
  <div class="bio-cards">
    <p v-if="!slides.length" class="bio-cards__empty">
      Nenhum slide ainda. Adicione o primeiro abaixo.
    </p>

    <div class="bio-cards__grid">
      <div v-for="(slide, index) in slides" :key="index" class="bio-card">
        <div class="bio-card__head">
          <span class="bio-card__index">Slide {{ index + 1 }}</span>
          <div class="bio-card__actions">
            <button
              type="button"
              class="bio-card__btn"
              :disabled="index === 0"
              aria-label="Mover slide para cima"
              @click="moveSlide(index, -1)"
            >
              <UIcon name="i-lucide-chevron-up" />
            </button>
            <button
              type="button"
              class="bio-card__btn"
              :disabled="index === slides.length - 1"
              aria-label="Mover slide para baixo"
              @click="moveSlide(index, 1)"
            >
              <UIcon name="i-lucide-chevron-down" />
            </button>
            <button
              type="button"
              class="bio-card__btn bio-card__btn--danger"
              aria-label="Remover slide"
              @click="removeSlide(index)"
            >
              <UIcon name="i-lucide-trash-2" />
            </button>
          </div>
        </div>

        <div class="bio-card__body">
          <BioMediaField
            label="Imagem (obrigatorio)"
            kind="slide"
            accept="image/*"
            :model-value="slide.src"
            :uploading="isUploading('slide')"
            :on-upload="uploadMedia"
            @update:model-value="updateSlide(index, 'src', String($event ?? ''))"
          />

          <div class="bio-field">
            <label class="bio-field__label">Titulo</label>
            <UInput
              :model-value="slide.title || ''"
              @update:model-value="updateSlide(index, 'title', String($event ?? ''))"
            />
          </div>
          <div class="bio-field">
            <label class="bio-field__label">Descricao</label>
            <UInput
              :model-value="slide.desc || ''"
              @update:model-value="updateSlide(index, 'desc', String($event ?? ''))"
            />
          </div>
          <div class="bio-section-grid bio-section-grid--tight">
            <div class="bio-field">
              <label class="bio-field__label">Preco</label>
              <UInput
                :model-value="slide.price || ''"
                @update:model-value="updateSlide(index, 'price', String($event ?? ''))"
              />
            </div>
            <div class="bio-field">
              <label class="bio-field__label">WhatsApp</label>
              <UInput
                :model-value="slide.whatsapp || ''"
                placeholder="5511999999999"
                @update:model-value="updateSlide(index, 'whatsapp', String($event ?? ''))"
              />
            </div>
          </div>
        </div>
      </div>
    </div>

    <UButton
      icon="i-lucide-plus"
      color="neutral"
      variant="soft"
      label="Adicionar slide"
      @click="addSlide"
    />
  </div>
</template>

<style scoped>
.bio-section-grid--tight {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(110px, 1fr));
  gap: 0.6rem 0.85rem;
}

.bio-field {
  display: grid;
  gap: 0.3rem;
}

.bio-field__label {
  font-size: 0.72rem;
  font-weight: 700;
  color: var(--text-muted);
}

.bio-cards {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.bio-cards__empty {
  margin: 0;
  padding: 0.85rem 1rem;
  border: 1px dashed var(--line-soft);
  border-radius: var(--radius-soft);
  color: var(--text-muted);
  font-size: 0.85rem;
  background: rgb(var(--surface-2) / 0.4);
}

.bio-cards__grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 0.75rem;
}

.bio-card {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
  padding: 0.75rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-soft);
  background: rgb(var(--surface) / 0.7);
}

.bio-card__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.bio-card__index {
  font-size: 0.78rem;
  font-weight: 700;
  color: var(--text-main);
}

.bio-card__body {
  display: flex;
  flex-direction: column;
  gap: 0.55rem;
}

.bio-card__actions {
  display: flex;
  gap: 0.3rem;
}

.bio-card__btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 1.7rem;
  height: 1.7rem;
  border: 1px solid var(--line-soft);
  border-radius: 0.4rem;
  background: rgb(var(--surface-2) / 0.6);
  color: var(--text-muted);
  cursor: pointer;
}

.bio-card__btn:hover:not(:disabled) {
  color: rgb(var(--primary));
  border-color: rgb(var(--primary) / 0.4);
}

.bio-card__btn--danger:hover:not(:disabled) {
  color: rgb(var(--danger));
  border-color: rgb(var(--danger) / 0.4);
}

.bio-card__btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}
</style>
