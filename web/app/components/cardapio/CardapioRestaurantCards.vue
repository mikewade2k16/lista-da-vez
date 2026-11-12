<script setup lang="ts">
import { computed, ref } from 'vue'

import { resolveMediaUrl } from '~/utils/media'
import type { RestaurantListItem } from '~/domain/cardapio/types'

// Lista de estabelecimentos em cards (visao padrao): so o basico — capa, nome,
// dominio e cliente. Clique abre o editor. Detalhes, filtros e acoes de gestao
// ficam na aba "Detalhes" (tabela).
const props = defineProps<{
  restaurants: RestaurantListItem[]
  loading: boolean
  showClient: boolean
}>()
const emit = defineEmits<{ open: [restaurant: RestaurantListItem] }>()

const config = useRuntimeConfig()
const apiBase = computed(() => String(config.public.apiBase || ''))
const cover = (item: RestaurantListItem) => resolveMediaUrl(item.coverUrl, apiBase.value)

// Logos (imagens quase quadradas) aparecem inteiros; banners largos preenchem a capa.
const logoIds = ref(new Set<string>())
function onCoverLoad(id: string, event: Event) {
  const img = event.target as HTMLImageElement
  if (img.naturalHeight && img.naturalWidth / img.naturalHeight < 1.4) {
    logoIds.value = new Set([...logoIds.value, id])
  }
}
</script>

<template>
  <div class="cardapio-cards-grid">
    <p v-if="loading && !props.restaurants.length" class="cardapio-cards-grid__empty">
      Carregando estabelecimentos...
    </p>
    <p v-else-if="!props.restaurants.length" class="cardapio-cards-grid__empty">
      Nenhum estabelecimento por aqui ainda.
    </p>
    <article v-for="item in props.restaurants" :key="item.id" class="cardapio-rcard">
      <button
        type="button"
        class="cardapio-rcard__open"
        :aria-label="`Abrir ${item.name}`"
        @click="emit('open', item)"
      >
        <span class="cardapio-rcard__cover">
          <img
            v-if="cover(item)"
            :src="cover(item)"
            :class="{ 'is-logo': logoIds.has(item.id) }"
            alt=""
            loading="lazy"
            decoding="async"
            @load="onCoverLoad(item.id, $event)"
          />
          <UIcon v-else name="i-lucide-store" class="cardapio-rcard__placeholder" />
          <span v-if="!item.isActive" class="cardapio-rcard__off">Fora do ar</span>
        </span>
        <span class="cardapio-rcard__body">
          <strong class="cardapio-rcard__name">{{ item.name }}</strong>
          <span v-if="showClient && item.accountName" class="cardapio-rcard__client">
            {{ item.accountName }}
          </span>
        </span>
      </button>
      <a
        v-if="item.primaryDomain"
        :href="`https://${item.primaryDomain}`"
        target="_blank"
        rel="noopener"
        class="cardapio-rcard__link"
        :title="`Ver o cardapio online de ${item.name}`"
      >
        <UIcon name="i-lucide-external-link" />
        <span>Ver cardapio</span>
        <span class="cardapio-rcard__domain">{{ item.primaryDomain }}</span>
      </a>
    </article>
  </div>
</template>

<style scoped>
.cardapio-cards-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(22rem, 1fr));
  gap: 1.1rem;
  align-content: start;
  /* Respiro para o card que sobe no hover nao ser cortado pela area com rolagem. */
  padding: 0.35rem 0.2rem 0.75rem;
}

.cardapio-cards-grid__empty {
  grid-column: 1 / -1;
  padding: 2rem 0;
  color: var(--text-muted);
  text-align: center;
}

.cardapio-rcard {
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-lg);
  background: rgb(var(--surface-2) / 0.6);
  transition:
    border-color 0.15s ease,
    transform 0.15s ease;
}

.cardapio-rcard:hover,
.cardapio-rcard:focus-within {
  border-color: rgb(var(--primary));
  transform: translateY(-2px);
}

.cardapio-rcard__open {
  display: flex;
  flex-direction: column;
  padding: 0;
  border: none;
  background: transparent;
  color: inherit;
  text-align: left;
  cursor: pointer;
}

.cardapio-rcard__cover {
  position: relative;
  display: grid;
  place-items: center;
  /* Capa grande e sempre na mesma proporcao (imagem absoluta nao estica o card). */
  aspect-ratio: 16 / 10;
  overflow: hidden;
  background: rgb(var(--primary) / 0.08);
}

.cardapio-rcard__cover img {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
  transition: transform 0.3s ease;
}

.cardapio-rcard__cover img.is-logo {
  padding: 0.9rem;
  object-fit: contain;
}

.cardapio-rcard:hover .cardapio-rcard__cover img {
  transform: scale(1.03);
}

.cardapio-rcard__placeholder {
  width: 2.5rem;
  height: 2.5rem;
  color: rgb(var(--primary));
}

.cardapio-rcard__off {
  position: absolute;
  top: 0.65rem;
  left: 0.65rem;
  padding: 0.2rem 0.65rem;
  border-radius: 999px;
  background: rgb(var(--danger));
  color: white;
  font-size: 0.75rem;
  font-weight: 600;
}

.cardapio-rcard__body {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
  padding: 0.9rem 1rem 0.35rem;
}

.cardapio-rcard__name {
  color: var(--text-main);
  font-size: 1.12rem;
}

.cardapio-rcard__client {
  color: var(--text-muted);
  font-size: 0.85rem;
}

.cardapio-rcard__link {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  min-width: 0;
  padding: 0.35rem 1rem 0.9rem;
  color: rgb(var(--primary));
  font-size: 0.86rem;
  font-weight: 600;
  text-decoration: none;
}

.cardapio-rcard__link:hover span:first-of-type {
  text-decoration: underline;
}

.cardapio-rcard__domain {
  overflow: hidden;
  color: var(--text-muted);
  font-weight: 400;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (max-width: 720px) {
  .cardapio-cards-grid {
    grid-template-columns: 1fr;
    gap: 0.75rem;
  }
}
</style>
