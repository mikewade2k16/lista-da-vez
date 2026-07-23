<script setup lang="ts">
import { Activity, RefreshCw } from 'lucide-vue-next'

import type { BiIntelligenceSection } from '~/stores/bi'

defineProps<{
  sections: BiIntelligenceSection[]
  inventoryLoading?: boolean
}>()
</script>

<template>
  <section class="bi-live">
    <header>
      <div>
        <span>
          <Activity :size="14" aria-hidden="true" />
          Leitura carregada
        </span>
        <h3>Indicadores da amostra atual</h3>
        <p>
          Estes números só aparecem depois de uma atualização intencional do BI e representam a
          janela limitada retornada pelo overview atual.
        </p>
      </div>
    </header>

    <div v-if="sections.length" class="bi-live__sections">
      <article
        v-for="section in sections"
        :key="section.key"
        class="bi-live__section"
        :data-tone="section.tone || 'default'"
      >
        <header>
          <div>
            <h4>{{ section.title }}</h4>
            <p>{{ section.summary }}</p>
          </div>
          <span v-if="section.key === 'estoque' && inventoryLoading">carregando</span>
        </header>

        <div>
          <article v-for="item in section.items" :key="`${section.key}-${item.label}`">
            <span>{{ item.label }}</span>
            <strong>{{ item.value }}</strong>
            <small>{{ item.detail || ' ' }}</small>
          </article>
        </div>
      </article>
    </div>

    <div v-else class="bi-live__empty">
      <RefreshCw :size="20" aria-hidden="true" />
      <div>
        <strong>Nenhuma amostra carregada nesta sessão</strong>
        <span>
          Use “Atualizar dados” nesta aba quando quiser consultar o overview. O mapa de inteligência
          acima não dispara consultas automaticamente.
        </span>
      </div>
    </div>
  </section>
</template>

<style scoped>
.bi-live {
  display: grid;
  gap: 13px;
  padding-top: 3px;
}

.bi-live > header span {
  display: inline-flex;
  gap: 6px;
  align-items: center;
  color: var(--accent-info);
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.bi-live h3,
.bi-live h4 {
  margin: 4px 0;
  color: var(--text-main);
}

.bi-live h3 {
  font-size: 1.08rem;
}

.bi-live h4 {
  font-size: 0.9rem;
}

.bi-live > header p,
.bi-live__section header p {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.78rem;
  line-height: 1.5;
}

.bi-live__sections {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 11px;
}

.bi-live__section {
  display: grid;
  gap: 11px;
  padding: 14px;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-md);
  background: var(--bg-panel);
  box-shadow: var(--shadow-card);
}

.bi-live__section > header {
  display: flex;
  gap: 10px;
  align-items: flex-start;
  justify-content: space-between;
}

.bi-live__section > header > span {
  padding: 4px 7px;
  color: var(--accent-warning);
  font-size: 0.68rem;
  border: 1px solid color-mix(in srgb, var(--accent-warning) 32%, var(--line-soft));
  border-radius: 999px;
}

.bi-live__section > div {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(130px, 1fr));
  gap: 8px;
}

.bi-live__section > div article {
  display: grid;
  gap: 3px;
  padding: 11px;
  border-radius: var(--radius-sm);
  background: var(--bg-muted);
}

.bi-live__section article span,
.bi-live__section article small {
  color: var(--text-muted);
  font-size: 0.7rem;
}

.bi-live__section article strong {
  color: var(--text-main);
}

.bi-live__empty {
  display: flex;
  gap: 11px;
  align-items: flex-start;
  padding: 15px;
  color: var(--accent-info);
  border: 1px dashed var(--line-strong);
  border-radius: var(--radius-md);
  background: var(--bg-muted);
}

.bi-live__empty div {
  display: grid;
  gap: 3px;
}

.bi-live__empty strong {
  color: var(--text-main);
  font-size: 0.82rem;
}

.bi-live__empty span {
  color: var(--text-muted);
  font-size: 0.75rem;
  line-height: 1.45;
}

@media (max-width: 820px) {
  .bi-live__sections {
    grid-template-columns: 1fr;
  }
}
</style>
