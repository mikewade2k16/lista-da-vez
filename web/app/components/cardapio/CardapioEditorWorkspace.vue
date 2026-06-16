<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'

import CardapioSectionDados from '~/components/cardapio/sections/CardapioSectionDados.vue'
import CardapioSectionCategorias from '~/components/cardapio/sections/CardapioSectionCategorias.vue'
import CardapioSectionProdutos from '~/components/cardapio/sections/CardapioSectionProdutos.vue'
import CardapioSectionAvaliacoes from '~/components/cardapio/sections/CardapioSectionAvaliacoes.vue'
import CardapioSectionPedidos from '~/components/cardapio/sections/CardapioSectionPedidos.vue'
import CardapioSectionDominios from '~/components/cardapio/sections/CardapioSectionDominios.vue'
import { useCardapioStore } from '~/stores/cardapio'
import { useUiStore } from '~/stores/ui'
import { getApiErrorMessage } from '~/utils/api-client'

const props = defineProps<{ restaurantId: string }>()

const store = useCardapioStore()
const ui = useUiStore()

type SectionId = 'dados' | 'categorias' | 'produtos' | 'avaliacoes' | 'pedidos' | 'dominios'

interface SectionTab {
  id: SectionId
  label: string
}

const SECTIONS: SectionTab[] = [
  { id: 'dados', label: 'Dados' },
  { id: 'categorias', label: 'Categorias' },
  { id: 'produtos', label: 'Produtos' },
  { id: 'avaliacoes', label: 'Avaliacoes' },
  { id: 'pedidos', label: 'Pedidos' },
  { id: 'dominios', label: 'Dominios' },
]

const active = ref<SectionId>('dados')
const togglingActive = ref(false)

const isActive = computed(() => store.restaurant?.isActive ?? false)
const publicUrl = computed(() => (store.primaryDomain ? `https://${store.primaryDomain}` : ''))

async function onToggleActive() {
  if (togglingActive.value || !store.restaurantId) {
    return
  }
  togglingActive.value = true
  try {
    await store.patchRestaurant(store.restaurantId, { isActive: !isActive.value })
    ui.success(isActive.value ? 'Cardapio publicado.' : 'Cardapio despublicado.')
  } catch (caught) {
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel alterar o status.'))
  } finally {
    togglingActive.value = false
  }
}

function loadActive() {
  if (props.restaurantId) {
    void store.loadRestaurant(props.restaurantId)
  }
}

watch(() => props.restaurantId, loadActive)
onMounted(loadActive)
</script>

<template>
  <section class="cardapio-editor">
    <header class="cardapio-editor__top">
      <div class="cardapio-editor__crumbs">
        <NuxtLink to="/cardapio" class="cardapio-editor__back">Cardapios</NuxtLink>
        <span class="cardapio-editor__sep">/</span>
        <span class="cardapio-editor__current">
          {{ store.restaurant?.name || 'Carregando...' }}
        </span>
      </div>

      <div class="cardapio-editor__status">
        <a
          v-if="publicUrl"
          :href="publicUrl"
          target="_blank"
          rel="noopener"
          class="cardapio-editor__link"
        >
          {{ store.primaryDomain }}
        </a>
        <span class="cardapio-editor__pill" :class="isActive ? 'is-on' : 'is-off'">
          {{ isActive ? 'Ativo' : 'Inativo' }}
        </span>
        <button
          type="button"
          class="cardapio-editor__toggle"
          :disabled="togglingActive || !store.restaurant"
          @click="onToggleActive"
        >
          <span v-if="togglingActive" class="cardapio-editor__spinner" aria-hidden="true"></span>
          {{ isActive ? 'Despublicar' : 'Publicar' }}
        </button>
      </div>
    </header>

    <p v-if="store.detailError" class="cardapio-editor__error">{{ store.detailError }}</p>

    <div v-if="store.detailPending && !store.restaurant" class="cardapio-editor__loading">
      Carregando cardapio...
    </div>

    <div v-else class="cardapio-editor__body">
      <nav class="cardapio-editor__nav" aria-label="Secoes do cardapio">
        <button
          v-for="section in SECTIONS"
          :key="section.id"
          type="button"
          class="cardapio-editor__nav-item"
          :class="{ 'cardapio-editor__nav-item--active': active === section.id }"
          @click="active = section.id"
        >
          {{ section.label }}
        </button>
      </nav>

      <div class="cardapio-editor__panel">
        <CardapioSectionDados v-if="active === 'dados'" />
        <CardapioSectionCategorias v-else-if="active === 'categorias'" />
        <CardapioSectionProdutos v-else-if="active === 'produtos'" />
        <CardapioSectionAvaliacoes v-else-if="active === 'avaliacoes'" />
        <CardapioSectionPedidos v-else-if="active === 'pedidos'" />
        <CardapioSectionDominios v-else-if="active === 'dominios'" />
      </div>
    </div>
  </section>
</template>

<style scoped>
.cardapio-editor {
  display: flex;
  flex-direction: column;
  gap: 1.1rem;
  padding: 1.5rem;
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}

.cardapio-editor__top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  flex-wrap: wrap;
}

.cardapio-editor__crumbs {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.95rem;
}

.cardapio-editor__back {
  color: rgb(var(--primary));
  font-weight: 600;
  text-decoration: none;
}

.cardapio-editor__sep {
  color: var(--text-muted);
}

.cardapio-editor__current {
  font-weight: 700;
  color: var(--text-main);
}

.cardapio-editor__status {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.cardapio-editor__link {
  font-size: 0.85rem;
  color: rgb(var(--primary));
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  text-decoration: none;
}

.cardapio-editor__pill {
  padding: 0.2rem 0.65rem;
  border-radius: 999px;
  font-size: 0.78rem;
  font-weight: 600;
}

.cardapio-editor__pill.is-on {
  background: rgb(var(--success) / 0.16);
  color: rgb(var(--success));
}

.cardapio-editor__pill.is-off {
  background: rgb(var(--muted) / 0.16);
  color: var(--text-muted);
}

.cardapio-editor__toggle {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  padding: 0.5rem 0.95rem;
  border-radius: var(--radius-sm);
  border: 1px solid var(--line-soft);
  background: rgb(var(--surface-2) / 0.8);
  color: var(--text-main);
  font-weight: 600;
  font-size: 0.87rem;
  cursor: pointer;
}

.cardapio-editor__toggle:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.cardapio-editor__error {
  color: rgb(var(--danger));
  background: rgb(var(--danger) / 0.14);
  padding: 0.6rem 0.85rem;
  border-radius: var(--radius-sm);
  font-size: 0.9rem;
}

.cardapio-editor__loading {
  color: var(--text-muted);
  padding: 1rem 0;
}

.cardapio-editor__body {
  display: grid;
  grid-template-columns: 188px minmax(0, 1fr);
  gap: 1.5rem;
  align-items: start;
  flex: 1;
  min-height: 0;
}

.cardapio-editor__nav {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
  position: sticky;
  top: 0;
}

.cardapio-editor__nav-item {
  width: 100%;
  padding: 0.6rem 0.85rem;
  border-radius: var(--radius-sm);
  border: none;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 0.92rem;
  font-weight: 500;
  text-align: left;
  transition:
    background 0.12s ease,
    color 0.12s ease;
}

.cardapio-editor__nav-item:hover {
  background: rgb(var(--surface-2) / 0.6);
  color: var(--text-main);
}

.cardapio-editor__nav-item--active {
  background: rgb(var(--primary) / 0.15);
  color: var(--text-main);
  font-weight: 600;
}

.cardapio-editor__panel {
  min-width: 0;
}

.cardapio-editor__spinner {
  width: 0.85rem;
  height: 0.85rem;
  border-radius: 999px;
  border: 2px solid rgb(var(--primary) / 0.35);
  border-top-color: rgb(var(--primary));
  animation: cardapio-editor-spin 0.7s linear infinite;
}

@keyframes cardapio-editor-spin {
  to {
    transform: rotate(360deg);
  }
}

@media (max-width: 880px) {
  .cardapio-editor__body {
    grid-template-columns: 1fr;
  }

  .cardapio-editor__nav {
    position: static;
    flex-direction: row;
    flex-wrap: nowrap;
    overflow-x: auto;
    scrollbar-width: none;
  }

  .cardapio-editor__nav-item {
    width: auto;
    white-space: nowrap;
  }
}
</style>
