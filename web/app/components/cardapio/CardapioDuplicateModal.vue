<script setup lang="ts">
import { computed, ref, watch } from 'vue'

import { slugify } from '~/domain/cardapio/types'

const props = defineProps<{
  open: boolean
  saving: boolean
  sourceName: string
  sourceSlug: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'submit', payload: { name: string; slug: string }): void
}>()

const name = ref('')
const slug = ref('')
const slugTouched = ref(false)

const canSubmit = computed(() => Boolean(name.value.trim() && slug.value.trim()))

// Pre-preenche Nome = "Copia de <nome>" e Slug = "<slug-atual>-copia".
// O slug do source ja e normalizado; em caso de conflito global o back devolve
// 409 (ErrSlugConflict) e o submit trata como erro.
function reset() {
  name.value = props.sourceName.trim() ? `Copia de ${props.sourceName.trim()}` : ''
  const baseSlug = slugify(props.sourceSlug || props.sourceName)
  slug.value = baseSlug ? `${baseSlug}-copia` : ''
  slugTouched.value = false
}

watch(
  () => props.open,
  (open) => {
    if (open) {
      reset()
    }
  },
)

function onSlugInput(event: Event) {
  slugTouched.value = true
  slug.value = slugify((event.target as HTMLInputElement).value)
}

function onSubmit() {
  if (!canSubmit.value || props.saving) {
    return
  }
  emit('submit', {
    name: name.value.trim(),
    slug: slug.value.trim(),
  })
}
</script>

<template>
  <div v-if="open" class="cardapio-duplicate" role="dialog" aria-modal="true">
    <div class="cardapio-duplicate__backdrop" @click="emit('close')"></div>
    <div class="cardapio-duplicate__panel">
      <header class="cardapio-duplicate__header">
        <h2 class="cardapio-duplicate__title">Duplicar estabelecimento</h2>
        <button
          type="button"
          class="cardapio-duplicate__close"
          aria-label="Fechar"
          @click="emit('close')"
        >
          &times;
        </button>
      </header>

      <form class="cardapio-duplicate__form" @submit.prevent="onSubmit">
        <p class="cardapio-duplicate__intro">
          Cria uma copia de
          <strong>{{ sourceName || 'este estabelecimento' }}</strong>
          (catalogo, zonas de entrega e layout) no mesmo cliente. A copia nasce inativa.
        </p>

        <label class="cardapio-duplicate__field">
          <span class="cardapio-duplicate__label">Nome da copia</span>
          <input
            v-model="name"
            type="text"
            class="cardapio-duplicate__input"
            placeholder="Ex.: Copia de Cantina da Nona"
            autofocus
          />
        </label>

        <label class="cardapio-duplicate__field">
          <span class="cardapio-duplicate__label">Slug (identificador na URL)</span>
          <input
            :value="slug"
            type="text"
            class="cardapio-duplicate__input"
            placeholder="cantina-da-nona-copia"
            @input="onSlugInput"
          />
          <span class="cardapio-duplicate__hint">Apenas letras minusculas, numeros e hifens.</span>
        </label>

        <footer class="cardapio-duplicate__footer">
          <button
            type="button"
            class="cardapio-duplicate__btn"
            :disabled="saving"
            @click="emit('close')"
          >
            Cancelar
          </button>
          <button
            type="submit"
            class="cardapio-duplicate__btn cardapio-duplicate__btn--primary"
            :disabled="!canSubmit || saving"
          >
            <span v-if="saving" class="cardapio-duplicate__spinner" aria-hidden="true"></span>
            {{ saving ? 'Duplicando...' : 'Duplicar' }}
          </button>
        </footer>
      </form>
    </div>
  </div>
</template>

<style scoped>
.cardapio-duplicate {
  position: fixed;
  inset: 0;
  z-index: 60;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 1.5rem;
}

.cardapio-duplicate__backdrop {
  position: absolute;
  inset: 0;
  background: rgb(var(--text) / 0.4);
  backdrop-filter: blur(2px);
}

.cardapio-duplicate__panel {
  position: relative;
  width: 100%;
  max-width: 440px;
  background: rgb(var(--surface));
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-md);
  overflow: hidden;
}

.cardapio-duplicate__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1.1rem 1.25rem;
  border-bottom: 1px solid var(--line-soft);
}

.cardapio-duplicate__title {
  font-size: 1.05rem;
  font-weight: 700;
  color: var(--text-main);
}

.cardapio-duplicate__close {
  border: none;
  background: transparent;
  font-size: 1.5rem;
  line-height: 1;
  color: var(--text-muted);
  cursor: pointer;
}

.cardapio-duplicate__form {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  padding: 1.25rem;
}

.cardapio-duplicate__intro {
  font-size: 0.85rem;
  color: var(--text-muted);
  line-height: 1.4;
}

.cardapio-duplicate__field {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.cardapio-duplicate__label {
  font-size: 0.82rem;
  font-weight: 600;
  color: var(--text-main);
}

.cardapio-duplicate__input {
  width: 100%;
  padding: 0.6rem 0.75rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2) / 0.6);
  color: var(--text-main);
  font-size: 0.92rem;
}

.cardapio-duplicate__input:focus {
  outline: none;
  border-color: rgb(var(--ring));
  box-shadow: 0 0 0 3px rgb(var(--ring) / 0.18);
}

.cardapio-duplicate__hint {
  font-size: 0.78rem;
  color: var(--text-muted);
}

.cardapio-duplicate__footer {
  display: flex;
  justify-content: flex-end;
  gap: 0.6rem;
  margin-top: 0.4rem;
}

.cardapio-duplicate__btn {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  padding: 0.6rem 1rem;
  border-radius: var(--radius-sm);
  border: 1px solid var(--line-soft);
  background: rgb(var(--surface-2) / 0.8);
  color: var(--text-main);
  font-weight: 600;
  font-size: 0.9rem;
  cursor: pointer;
}

.cardapio-duplicate__btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.cardapio-duplicate__btn--primary {
  border: none;
  color: rgb(var(--surface));
  background: linear-gradient(135deg, rgb(var(--primary)), rgb(var(--primary-600)));
}

.cardapio-duplicate__spinner {
  width: 0.9rem;
  height: 0.9rem;
  border-radius: 999px;
  border: 2px solid rgb(var(--surface) / 0.5);
  border-top-color: rgb(var(--surface));
  animation: cardapio-duplicate-spin 0.7s linear infinite;
}

@keyframes cardapio-duplicate-spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
