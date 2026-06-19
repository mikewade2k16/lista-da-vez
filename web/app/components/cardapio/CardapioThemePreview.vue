<script setup lang="ts">
import { computed } from 'vue'

import { themeToPreviewVars } from '~/domain/cardapio/types'
import type { RestaurantTheme } from '~/domain/cardapio/types'

// Previa ao vivo da APARENCIA (WS-D), autossuficiente no painel: SEM iframe/cross-
// origin. Renderiza um mock FIEL dos blocos do site publico (TAVOLA) e aplica o
// tema rico em TEMPO REAL via CSS custom properties (--prev-*) no container raiz.
// Cada mock le essas vars, entao tudo muda ao vivo quando o editor altera o tema.
// As cores aqui sao DADO do usuario (vindas do tema), nao tokens do painel.

const props = defineProps<{
  theme: RestaurantTheme
  restaurantName?: string
}>()

const rootStyle = computed(() => themeToPreviewVars(props.theme))

const displayName = computed(() => String(props.restaurantName || '').trim() || 'Seu Restaurante')

const initial = computed(() => displayName.value.charAt(0).toUpperCase())

interface PreviewDish {
  name: string
  desc: string
  price: string
}

const dishes: PreviewDish[] = [
  {
    name: 'Risoto de funghi',
    desc: 'Arroz arboreo, mix de cogumelos, parmesao',
    price: 'R$ 68,00',
  },
  {
    name: 'Bife ancho 350g',
    desc: 'Maturado, manteiga de ervas, batata rustica',
    price: 'R$ 94,00',
  },
]
</script>

<template>
  <div class="theme-preview">
    <div class="theme-preview__bar">
      <UIcon name="i-lucide-radio" class="theme-preview__live" />
      <span>Previa ao vivo</span>
    </div>

    <div class="theme-preview__frame" :style="rootStyle">
      <header class="theme-preview__header">
        <div class="theme-preview__brand">
          <span class="theme-preview__logo">{{ initial }}</span>
          <span class="theme-preview__name">{{ displayName }}</span>
        </div>
        <nav class="theme-preview__nav">
          <span>Menu</span>
          <span>Sobre</span>
          <span class="theme-preview__nav-cta">Pedir</span>
        </nav>
      </header>

      <section class="theme-preview__hero">
        <span class="theme-preview__eyebrow">Cozinha de autor</span>
        <h2 class="theme-preview__title">Sabores que contam uma historia</h2>
        <p class="theme-preview__lead">
          Ingredientes selecionados e pratos preparados na hora, do entrada a sobremesa.
        </p>
        <button type="button" class="theme-preview__cta">Ver cardapio</button>
      </section>

      <section class="theme-preview__menu">
        <article v-for="dish in dishes" :key="dish.name" class="theme-preview__dish">
          <div class="theme-preview__thumb" aria-hidden="true">
            <UIcon name="i-lucide-utensils" />
          </div>
          <div class="theme-preview__dish-body">
            <h3 class="theme-preview__dish-name">{{ dish.name }}</h3>
            <p class="theme-preview__dish-desc">{{ dish.desc }}</p>
            <div class="theme-preview__dish-foot">
              <span class="theme-preview__price">{{ dish.price }}</span>
              <button type="button" class="theme-preview__add">Adicionar</button>
            </div>
          </div>
        </article>
      </section>
    </div>
  </div>
</template>

<style scoped>
/* Casca do preview: usa tokens do PAINEL (a moldura nao faz parte do tema). */
.theme-preview {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  position: sticky;
  top: 0;
}

.theme-preview__bar {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  font-size: 0.78rem;
  font-weight: 600;
  color: var(--text-muted);
}

.theme-preview__live {
  color: rgb(var(--success));
}

/* Tudo daqui pra baixo e dirigido pelas vars do TEMA (--prev-*). */
.theme-preview__frame {
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  overflow: hidden;
  box-shadow: var(--shadow-card);
  background: var(--prev-bg);
  color: var(--prev-text);
  font-family: var(--prev-body);
}

.theme-preview__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.85rem 1.1rem;
  border-bottom: 1px solid var(--prev-border);
  background: var(--prev-surface);
}

.theme-preview__brand {
  display: inline-flex;
  align-items: center;
  gap: 0.55rem;
  min-width: 0;
}

.theme-preview__logo {
  display: grid;
  place-items: center;
  width: 2rem;
  height: 2rem;
  border-radius: var(--prev-radius);
  background: var(--prev-accent);
  color: var(--prev-bg);
  font-family: var(--prev-display);
  font-weight: 700;
  font-size: 1.05rem;
  flex-shrink: 0;
}

.theme-preview__name {
  font-family: var(--prev-display);
  font-weight: 700;
  font-size: 1.1rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.theme-preview__nav {
  display: inline-flex;
  align-items: center;
  gap: 0.85rem;
  font-size: 0.78rem;
  opacity: 0.92;
}

.theme-preview__nav-cta {
  padding: 0.28rem 0.7rem;
  border-radius: var(--prev-radius);
  border: 1px solid var(--prev-accent);
  color: var(--prev-accent);
  font-weight: 600;
}

.theme-preview__hero {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  padding: 1.6rem 1.25rem;
  text-align: center;
}

.theme-preview__eyebrow {
  font-size: 0.72rem;
  letter-spacing: 0.16em;
  text-transform: uppercase;
  color: var(--prev-accent);
  font-weight: 700;
}

.theme-preview__title {
  margin: 0;
  font-family: var(--prev-display);
  font-weight: 700;
  font-size: 1.7rem;
  line-height: 1.15;
}

.theme-preview__lead {
  margin: 0 auto;
  max-width: 30ch;
  font-size: 0.86rem;
  opacity: 0.8;
}

.theme-preview__cta {
  align-self: center;
  margin-top: 0.4rem;
  padding: 0.6rem 1.4rem;
  border: none;
  border-radius: var(--prev-radius);
  background: var(--prev-accent);
  color: var(--prev-bg);
  font-weight: 700;
  font-size: 0.86rem;
  cursor: pointer;
}

.theme-preview__menu {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 0.85rem;
  padding: 0 1.1rem 1.3rem;
}

.theme-preview__dish {
  display: flex;
  flex-direction: column;
  border: 1px solid var(--prev-border);
  border-radius: var(--prev-radius);
  overflow: hidden;
  background: var(--prev-surface);
}

.theme-preview__thumb {
  display: grid;
  place-items: center;
  height: 5.5rem;
  font-size: 1.6rem;
  color: var(--prev-accent);
  background: color-mix(in srgb, var(--prev-accent) 14%, var(--prev-surface));
  border-bottom: 1px solid var(--prev-border);
}

.theme-preview__dish-body {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
  padding: 0.7rem 0.8rem 0.8rem;
}

.theme-preview__dish-name {
  margin: 0;
  font-family: var(--prev-display);
  font-weight: 700;
  font-size: 1rem;
}

.theme-preview__dish-desc {
  margin: 0;
  font-size: 0.76rem;
  opacity: 0.72;
}

.theme-preview__dish-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  margin-top: 0.25rem;
}

.theme-preview__price {
  font-family: var(--prev-display);
  font-weight: 700;
  font-size: 1.05rem;
  color: var(--prev-accent);
}

.theme-preview__add {
  padding: 0.35rem 0.7rem;
  border: 1px solid var(--prev-border);
  border-radius: var(--prev-radius);
  background: transparent;
  color: var(--prev-text);
  font-size: 0.74rem;
  font-weight: 600;
  cursor: pointer;
}
</style>
