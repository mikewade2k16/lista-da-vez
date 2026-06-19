<script setup lang="ts">
import { computed, ref, watch } from 'vue'

import { slugify } from '~/domain/cardapio/types'

interface TenantOption {
  id: string
  name: string
}

const props = defineProps<{
  open: boolean
  saving: boolean
  isAdmin: boolean
  tenants: TenantOption[]
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'submit', payload: { name: string; slug: string; accountId: string }): void
}>()

// Sentinela "agencia": admin cria sob a propria account ativa (accountId vazio
// no backend) em vez de escolher um cliente. Espelha o modal da bio.
const AGENCY_SENTINEL = 'agency'

const name = ref('')
const slug = ref('')
const slugTouched = ref(false)
const accountId = ref('')

const canSubmit = computed(() => {
  if (!name.value.trim() || !slug.value.trim()) {
    return false
  }
  if (props.isAdmin && !accountId.value) {
    return false
  }
  return true
})

function reset() {
  name.value = ''
  slug.value = ''
  slugTouched.value = false
  accountId.value = props.isAdmin ? AGENCY_SENTINEL : ''
}

watch(
  () => props.open,
  (open) => {
    if (open) {
      reset()
    }
  },
)

watch(name, (value) => {
  if (!slugTouched.value) {
    slug.value = slugify(value)
  }
})

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
    accountId: accountId.value === AGENCY_SENTINEL ? '' : accountId.value,
  })
}
</script>

<template>
  <div v-if="open" class="cardapio-create" role="dialog" aria-modal="true">
    <div class="cardapio-create__backdrop" @click="emit('close')"></div>
    <div class="cardapio-create__panel">
      <header class="cardapio-create__header">
        <h2 class="cardapio-create__title">Novo estabelecimento</h2>
        <button
          type="button"
          class="cardapio-create__close"
          aria-label="Fechar"
          @click="emit('close')"
        >
          &times;
        </button>
      </header>

      <form class="cardapio-create__form" @submit.prevent="onSubmit">
        <label class="cardapio-create__field">
          <span class="cardapio-create__label">Nome do restaurante</span>
          <input
            v-model="name"
            type="text"
            class="cardapio-create__input"
            placeholder="Ex.: Cantina da Nona"
            autofocus
          />
        </label>

        <label class="cardapio-create__field">
          <span class="cardapio-create__label">Slug (identificador na URL)</span>
          <input
            :value="slug"
            type="text"
            class="cardapio-create__input"
            placeholder="cantina-da-nona"
            @input="onSlugInput"
          />
          <span class="cardapio-create__hint">Apenas letras minusculas, numeros e hifens.</span>
        </label>

        <label v-if="isAdmin" class="cardapio-create__field">
          <span class="cardapio-create__label">Cliente (account)</span>
          <select v-model="accountId" class="cardapio-create__input">
            <option :value="AGENCY_SENTINEL">Agencia (propria account)</option>
            <option v-for="tenant in tenants" :key="tenant.id" :value="tenant.id">
              {{ tenant.name }}
            </option>
          </select>
        </label>

        <footer class="cardapio-create__footer">
          <button
            type="button"
            class="cardapio-create__btn"
            :disabled="saving"
            @click="emit('close')"
          >
            Cancelar
          </button>
          <button
            type="submit"
            class="cardapio-create__btn cardapio-create__btn--primary"
            :disabled="!canSubmit || saving"
          >
            <span v-if="saving" class="cardapio-create__spinner" aria-hidden="true"></span>
            {{ saving ? 'Criando...' : 'Criar estabelecimento' }}
          </button>
        </footer>
      </form>
    </div>
  </div>
</template>

<style scoped>
.cardapio-create {
  position: fixed;
  inset: 0;
  z-index: 60;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 1.5rem;
}

.cardapio-create__backdrop {
  position: absolute;
  inset: 0;
  background: rgb(var(--text) / 0.4);
  backdrop-filter: blur(2px);
}

.cardapio-create__panel {
  position: relative;
  width: 100%;
  max-width: 440px;
  background: rgb(var(--surface));
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-md);
  overflow: hidden;
}

.cardapio-create__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1.1rem 1.25rem;
  border-bottom: 1px solid var(--line-soft);
}

.cardapio-create__title {
  font-size: 1.05rem;
  font-weight: 700;
  color: var(--text-main);
}

.cardapio-create__close {
  border: none;
  background: transparent;
  font-size: 1.5rem;
  line-height: 1;
  color: var(--text-muted);
  cursor: pointer;
}

.cardapio-create__form {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  padding: 1.25rem;
}

.cardapio-create__field {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.cardapio-create__label {
  font-size: 0.82rem;
  font-weight: 600;
  color: var(--text-main);
}

.cardapio-create__input {
  width: 100%;
  padding: 0.6rem 0.75rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2) / 0.6);
  color: var(--text-main);
  font-size: 0.92rem;
}

.cardapio-create__input:focus {
  outline: none;
  border-color: rgb(var(--ring));
  box-shadow: 0 0 0 3px rgb(var(--ring) / 0.18);
}

.cardapio-create__hint {
  font-size: 0.78rem;
  color: var(--text-muted);
}

.cardapio-create__footer {
  display: flex;
  justify-content: flex-end;
  gap: 0.6rem;
  margin-top: 0.4rem;
}

.cardapio-create__btn {
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

.cardapio-create__btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.cardapio-create__btn--primary {
  border: none;
  color: rgb(var(--surface));
  background: linear-gradient(135deg, rgb(var(--primary)), rgb(var(--primary-600)));
}

.cardapio-create__spinner {
  width: 0.9rem;
  height: 0.9rem;
  border-radius: 999px;
  border: 2px solid rgb(var(--surface) / 0.5);
  border-top-color: rgb(var(--surface));
  animation: cardapio-create-spin 0.7s linear infinite;
}

@keyframes cardapio-create-spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
