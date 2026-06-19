<script setup lang="ts">
import { computed } from 'vue'

import LegacyMarker from '~/components/admin/LegacyMarker.vue'
import { useAuthStore } from '~/stores/auth'
import type { DadosForm } from '~/composables/useCardapioEditor'

// WS-C — Estatisticas e scripts. GA ID e Pixel ID sao so identificadores
// (renderizados por template conhecido no site) e seguros para o lojista.
// customHeadHtml e HTML livre injetado no <head> do site publico = risco de XSS:
// editavel SOMENTE por platform_admin (gate aqui) e marcado como legado/risco.
// `form` aponta para o MESMO objeto reativo do editor (passado por prop): mutar
// seus campos propaga para o dirty-check do composable. A variavel local evita
// o lint de mutacao de prop (vue/no-mutating-props).
const props = defineProps<{ form: DadosForm }>()

const form = props.form
const auth = useAuthStore()
const canEditHeadHtml = computed(() => auth.role === 'platform_admin')
</script>

<template>
  <section class="cardapio-stats">
    <h3 class="cardapio-stats__heading">Estatisticas e scripts</h3>
    <p class="cardapio-stats__hint">
      IDs de acompanhamento injetados no site publico por modelo conhecido (sem HTML livre).
    </p>

    <div class="cardapio-stats__grid">
      <label class="cardapio-stats__field">
        <span class="cardapio-stats__label">Google Analytics ID</span>
        <input
          v-model="form.googleAnalyticsId"
          type="text"
          class="cardapio-stats__input"
          placeholder="G-XXXXXXXXXX"
        />
      </label>
      <label class="cardapio-stats__field">
        <span class="cardapio-stats__label">Facebook Pixel ID</span>
        <input
          v-model="form.facebookPixelId"
          type="text"
          class="cardapio-stats__input"
          placeholder="000000000000000"
        />
      </label>
    </div>

    <div class="cardapio-stats__head">
      <LegacyMarker
        kind="legacy"
        label="HTML adicional injetado no site publico"
        detail="HTML livre no <head> = risco de XSS; so platform_admin edita. Ver docs/LEGADO.md."
      />
      <label class="cardapio-stats__field cardapio-stats__field--full">
        <span class="cardapio-stats__label">HTML adicional (head)</span>
        <textarea
          v-model="form.customHeadHtml"
          rows="5"
          class="cardapio-stats__input cardapio-stats__mono"
          :readonly="!canEditHeadHtml"
          :disabled="!canEditHeadHtml"
          :placeholder="
            canEditHeadHtml
              ? 'Tags de script ou meta para o head'
              : 'Somente administradores podem editar.'
          "
        ></textarea>
        <span class="cardapio-stats__warn">
          Atencao: este HTML e injetado direto no site publico. Use apenas codigo confiavel.
        </span>
      </label>
    </div>
  </section>
</template>

<style scoped>
.cardapio-stats {
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.6);
  padding: 1.1rem 1.25rem;
}

.cardapio-stats__heading {
  font-size: 0.95rem;
  font-weight: 700;
  color: var(--text-main);
  margin-bottom: 0.5rem;
}

.cardapio-stats__hint {
  font-size: 0.83rem;
  color: var(--text-muted);
  margin-bottom: 0.85rem;
}

.cardapio-stats__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.85rem;
}

.cardapio-stats__head {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
  margin-top: 0.9rem;
}

.cardapio-stats__field {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
  min-width: 0;
}

.cardapio-stats__field--full {
  grid-column: 1 / -1;
}

.cardapio-stats__label {
  font-size: 0.8rem;
  font-weight: 600;
  color: var(--text-main);
}

.cardapio-stats__warn {
  font-size: 0.76rem;
  color: rgb(var(--danger));
}

.cardapio-stats__input {
  width: 100%;
  padding: 0.55rem 0.7rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2) / 0.6);
  color: var(--text-main);
  font-size: 0.9rem;
  font-family: inherit;
}

.cardapio-stats__input:focus {
  outline: none;
  border-color: rgb(var(--ring));
  box-shadow: 0 0 0 3px rgb(var(--ring) / 0.18);
}

.cardapio-stats__input:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.cardapio-stats__mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.82rem;
}

@media (max-width: 720px) {
  .cardapio-stats__grid {
    grid-template-columns: 1fr;
  }
}
</style>
