<script setup lang="ts">
import { computed, ref, watch } from 'vue'

import CardapioColorField from '~/components/cardapio/CardapioColorField.vue'
import CardapioThemePreview from '~/components/cardapio/CardapioThemePreview.vue'
import { useCardapioStore } from '~/stores/cardapio'
import { useUiStore } from '~/stores/ui'
import { getApiErrorMessage } from '~/utils/api-client'
import {
  CARDAPIO_THEME_COLOR_FIELDS,
  CARDAPIO_THEME_FONTS,
  CARDAPIO_THEME_MODES,
  CARDAPIO_THEME_PRESETS,
  CARDAPIO_THEME_RADII,
  normalizeTheme,
} from '~/domain/cardapio/types'
import type { RestaurantTheme, ThemeColors, ThemeMode, ThemeRadius } from '~/domain/cardapio/types'

// WS-D — Aparencia RICA. Dona do restaurant.theme (jsonb): salva o shape rico
// { base, mode, colors{5}, fonts{2}, radius } via PATCH parcial {theme}. As cores
// sao HEX escolhidos pelo usuario (DADO, nao token do design system do painel) —
// unica excecao a regra "nunca hex". Presets preenchem um conjunto inicial.

const store = useCardapioStore()
const ui = useUiStore()

const form = ref<RestaurantTheme>(normalizeTheme(store.restaurant?.theme))
const baseline = ref<string>(JSON.stringify(form.value))
const saving = ref(false)

const dirty = computed(() => JSON.stringify(form.value) !== baseline.value)

function syncFromStore() {
  form.value = normalizeTheme(store.restaurant?.theme)
  baseline.value = JSON.stringify(form.value)
}

watch(() => store.restaurant, syncFromStore, { immediate: true })

// Aplica um preset: copia cores/fontes/modo/cantos e marca a base escolhida. O
// usuario pode ajustar tudo depois (continua sendo dado livre).
function applyPreset(value: string) {
  const preset = CARDAPIO_THEME_PRESETS.find((item) => item.value === value)
  if (!preset) {
    return
  }
  form.value = {
    base: preset.value,
    mode: preset.theme.mode,
    colors: { ...preset.theme.colors },
    fonts: { ...preset.theme.fonts },
    radius: preset.theme.radius,
  }
}

function setColor(key: keyof ThemeColors, value: string) {
  form.value = { ...form.value, colors: { ...form.value.colors, [key]: value } }
}

function setMode(value: ThemeMode) {
  form.value = { ...form.value, mode: value }
}

function setRadius(value: ThemeRadius) {
  form.value = { ...form.value, radius: value }
}

async function save() {
  if (saving.value || !store.restaurantId) {
    return
  }
  saving.value = true
  try {
    // PATCH parcial: so o theme RICO completo. O back mantem o resto do restaurante.
    await store.patchRestaurant(store.restaurantId, {
      theme: {
        base: form.value.base,
        mode: form.value.mode,
        colors: { ...form.value.colors },
        fonts: { ...form.value.fonts },
        radius: form.value.radius,
      },
    })
    syncFromStore()
    ui.success('Aparencia salva.')
  } catch (caught) {
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel salvar a aparencia.'))
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="cardapio-look">
    <div class="cardapio-look__layout">
      <div class="cardapio-look__editor">
        <p class="cardapio-look__note">
          Defina a paleta e a tipografia do site publico deste estabelecimento. As cores sao
          aplicadas em tempo real na previa ao lado.
        </p>

        <section class="cardapio-look__card">
          <header class="cardapio-look__head">
            <h3 class="cardapio-look__title">Presets</h3>
            <span class="cardapio-look__subtitle">Ponto de partida (ajustavel)</span>
          </header>
          <div class="cardapio-look__presets">
            <button
              v-for="preset in CARDAPIO_THEME_PRESETS"
              :key="preset.value"
              type="button"
              class="cardapio-look__preset"
              :class="{ 'cardapio-look__preset--active': form.base === preset.value }"
              @click="applyPreset(preset.value)"
            >
              <span class="cardapio-look__swatches" aria-hidden="true">
                <span
                  v-for="(hex, key) in preset.theme.colors"
                  :key="key"
                  class="cardapio-look__chip"
                  :style="{ background: hex }"
                ></span>
              </span>
              <span class="cardapio-look__preset-name">{{ preset.label }}</span>
              <span class="cardapio-look__preset-desc">{{ preset.description }}</span>
            </button>
          </div>
        </section>

        <section class="cardapio-look__card">
          <header class="cardapio-look__head">
            <h3 class="cardapio-look__title">Cores</h3>
            <span class="cardapio-look__subtitle">5 cores semanticas da marca</span>
          </header>
          <div class="cardapio-look__colors">
            <CardapioColorField
              v-for="field in CARDAPIO_THEME_COLOR_FIELDS"
              :key="field.key"
              :label="field.label"
              :hint="field.hint"
              :model-value="form.colors[field.key]"
              @update:model-value="setColor(field.key, $event)"
            />
          </div>
        </section>

        <section class="cardapio-look__card">
          <header class="cardapio-look__head">
            <h3 class="cardapio-look__title">Tipografia e cantos</h3>
          </header>
          <div class="cardapio-look__grid">
            <label class="cardapio-look__field">
              <span class="cardapio-look__label">Fonte dos titulos</span>
              <select v-model="form.fonts.display" class="cardapio-look__input">
                <option v-for="font in CARDAPIO_THEME_FONTS" :key="font.value" :value="font.value">
                  {{ font.label }}
                </option>
              </select>
            </label>

            <label class="cardapio-look__field">
              <span class="cardapio-look__label">Fonte do corpo</span>
              <select v-model="form.fonts.body" class="cardapio-look__input">
                <option v-for="font in CARDAPIO_THEME_FONTS" :key="font.value" :value="font.value">
                  {{ font.label }}
                </option>
              </select>
            </label>
          </div>

          <div class="cardapio-look__toggles">
            <div class="cardapio-look__toggle">
              <span class="cardapio-look__label">Modo</span>
              <div class="cardapio-look__seg" role="group" aria-label="Modo de cor">
                <button
                  v-for="mode in CARDAPIO_THEME_MODES"
                  :key="mode.value"
                  type="button"
                  class="cardapio-look__seg-btn"
                  :class="{ 'cardapio-look__seg-btn--on': form.mode === mode.value }"
                  @click="setMode(mode.value)"
                >
                  {{ mode.label }}
                </button>
              </div>
            </div>

            <div class="cardapio-look__toggle">
              <span class="cardapio-look__label">Cantos</span>
              <div class="cardapio-look__seg" role="group" aria-label="Raio dos cantos">
                <button
                  v-for="radius in CARDAPIO_THEME_RADII"
                  :key="radius.value"
                  type="button"
                  class="cardapio-look__seg-btn"
                  :class="{ 'cardapio-look__seg-btn--on': form.radius === radius.value }"
                  @click="setRadius(radius.value)"
                >
                  {{ radius.label }}
                </button>
              </div>
            </div>
          </div>
        </section>
      </div>

      <aside class="cardapio-look__aside">
        <CardapioThemePreview :theme="form" :restaurant-name="store.restaurant?.name" />
      </aside>
    </div>

    <footer class="cardapio-look__footer">
      <span v-if="dirty" class="cardapio-look__dirty">Alteracoes nao salvas</span>
      <button type="button" class="cardapio-look__save" :disabled="saving || !dirty" @click="save">
        <span v-if="saving" class="cardapio-look__spinner" aria-hidden="true"></span>
        {{ saving ? 'Salvando...' : 'Salvar aparencia' }}
      </button>
    </footer>
  </div>
</template>

<style scoped>
.cardapio-look {
  display: flex;
  flex-direction: column;
  gap: 1.1rem;
}

.cardapio-look__layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 0.85fr);
  gap: 1.25rem;
  align-items: start;
}

.cardapio-look__editor {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  min-width: 0;
}

.cardapio-look__aside {
  min-width: 0;
}

.cardapio-look__note {
  font-size: 0.86rem;
  color: var(--text-muted);
  margin: 0;
}

.cardapio-look__card {
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.6);
  padding: 1rem 1.15rem;
  display: flex;
  flex-direction: column;
  gap: 0.85rem;
}

.cardapio-look__head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.cardapio-look__title {
  margin: 0;
  font-size: 0.92rem;
  font-weight: 700;
  color: var(--text-main);
}

.cardapio-look__subtitle {
  font-size: 0.74rem;
  color: var(--text-muted);
}

.cardapio-look__presets {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 0.7rem;
}

.cardapio-look__preset {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
  text-align: left;
  padding: 0.7rem 0.8rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-soft);
  background: rgb(var(--surface-2) / 0.5);
  cursor: pointer;
  transition: border-color 0.12s ease;
}

.cardapio-look__preset:hover {
  border-color: rgb(var(--ring) / 0.6);
}

.cardapio-look__preset--active {
  border-color: rgb(var(--ring));
  box-shadow: 0 0 0 2px rgb(var(--ring) / 0.18);
}

.cardapio-look__swatches {
  display: inline-flex;
  gap: 0.2rem;
}

.cardapio-look__chip {
  width: 1.1rem;
  height: 1.1rem;
  border-radius: 999px;
  border: 1px solid rgb(var(--border) / 0.5);
}

.cardapio-look__preset-name {
  font-size: 0.86rem;
  font-weight: 700;
  color: var(--text-main);
}

.cardapio-look__preset-desc {
  font-size: 0.74rem;
  color: var(--text-muted);
}

.cardapio-look__colors {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 0.75rem 0.85rem;
}

.cardapio-look__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 0.85rem;
}

.cardapio-look__field {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
  min-width: 0;
}

.cardapio-look__label {
  font-size: 0.78rem;
  font-weight: 700;
  color: var(--text-main);
}

.cardapio-look__input {
  width: 100%;
  padding: 0.55rem 0.7rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2) / 0.6);
  color: var(--text-main);
  font-size: 0.88rem;
  font-family: inherit;
}

.cardapio-look__input:focus {
  outline: none;
  border-color: rgb(var(--ring));
  box-shadow: 0 0 0 3px rgb(var(--ring) / 0.18);
}

.cardapio-look__toggles {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 0.85rem;
}

.cardapio-look__toggle {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.cardapio-look__seg {
  display: inline-flex;
  padding: 0.15rem;
  gap: 0.1rem;
  border: 1px solid var(--line-soft);
  border-radius: 999px;
  background: rgb(var(--surface-2) / 0.5);
}

.cardapio-look__seg-btn {
  flex: 1;
  padding: 0.4rem 0.7rem;
  border: none;
  border-radius: 999px;
  background: transparent;
  color: var(--text-muted);
  font-size: 0.8rem;
  font-weight: 600;
  cursor: pointer;
  white-space: nowrap;
}

.cardapio-look__seg-btn--on {
  background: rgb(var(--primary) / 0.16);
  color: var(--text-main);
}

.cardapio-look__footer {
  position: sticky;
  bottom: 0;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.85rem;
  padding-top: 0.4rem;
}

.cardapio-look__dirty {
  font-size: 0.84rem;
  color: var(--accent-warning, rgb(var(--primary)));
}

.cardapio-look__save {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  border: none;
  color: rgb(var(--surface));
  background: linear-gradient(135deg, rgb(var(--primary)), rgb(var(--primary-600)));
  padding: 0.6rem 1.2rem;
  border-radius: var(--radius-sm);
  font-weight: 600;
  font-size: 0.9rem;
  cursor: pointer;
}

.cardapio-look__save:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.cardapio-look__spinner {
  width: 0.85rem;
  height: 0.85rem;
  border-radius: 999px;
  border: 2px solid rgb(var(--surface) / 0.5);
  border-top-color: rgb(var(--surface));
  animation: cardapio-look-spin 0.7s linear infinite;
}

@keyframes cardapio-look-spin {
  to {
    transform: rotate(360deg);
  }
}

@media (max-width: 980px) {
  .cardapio-look__layout {
    grid-template-columns: 1fr;
  }
}
</style>
