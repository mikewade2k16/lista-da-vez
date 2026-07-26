<script setup lang="ts">
import { computed, ref } from 'vue'
import type { Component } from 'vue'
import {
  BadgeDollarSign,
  Database,
  Image as ImageIcon,
  ListTree,
  PackageSearch,
  ReceiptText,
  ShieldCheck,
  Warehouse,
} from 'lucide-vue-next'

import BiApiEntityDetail from '~/components/bi/BiApiEntityDetail.vue'
import { BI_API_ENTITIES, biApiEntityFieldCount } from '~/domain/bi/api-catalog'
import type { BiApiEntity } from '~/domain/bi/api-catalog'

const iconMap: Record<BiApiEntity['icon'], Component> = {
  package: PackageSearch,
  image: ImageIcon,
  coins: BadgeDollarSign,
  receipt: ReceiptText,
  list: ListTree,
  warehouse: Warehouse,
}

const activeEntityId = ref(BI_API_ENTITIES[0]?.id || '')
const activeEntity = computed(
  () => BI_API_ENTITIES.find((entity) => entity.id === activeEntityId.value) || BI_API_ENTITIES[0],
)
const totalFields = computed(() =>
  BI_API_ENTITIES.reduce((total, entity) => total + biApiEntityFieldCount(entity), 0),
)
</script>

<template>
  <section class="bi-catalog" data-testid="bi-api-catalog">
    <header class="bi-catalog__intro omni-glass">
      <div class="bi-catalog__intro-copy">
        <span class="bi-catalog__eyebrow">
          <Database :size="14" aria-hidden="true" />
          Mapa da API Pérola BI
        </span>
        <h3>O que conseguimos consultar</h3>
        <p>
          Catálogo construído a partir de uma amostra controlada de um registro por entidade. Mostra
          a estrutura disponível sem carregar a base inteira e sem expor dados reais.
        </p>
      </div>

      <div class="bi-catalog__stats" aria-label="Resumo do catálogo">
        <article>
          <strong>{{ BI_API_ENTITIES.length }}</strong>
          <span>entidades mapeadas</span>
        </article>
        <article>
          <strong>{{ totalFields }}</strong>
          <span>campos observados</span>
        </article>
        <article>
          <strong>1</strong>
          <span>registro por amostra</span>
        </article>
      </div>
    </header>

    <div class="bi-catalog__notice">
      <ShieldCheck :size="17" aria-hidden="true" />
      <div>
        <strong>Catálogo local · nenhuma chamada de API</strong>
        <span>
          As seis entidades e seus campos vêm do contrato já mapeado. As tabelas de registros,
          filtros e paginação ficam na aba Consultas.
        </span>
      </div>
    </div>

    <nav class="bi-catalog__entity-tabs" role="tablist" aria-label="Entidades da API Pérola BI">
      <button
        v-for="entity in BI_API_ENTITIES"
        :id="`bi-entity-tab-${entity.id}`"
        :key="entity.id"
        type="button"
        role="tab"
        class="bi-entity-tab"
        :class="{ 'is-active': activeEntityId === entity.id }"
        :data-tone="entity.tone"
        :aria-selected="activeEntityId === entity.id"
        :aria-controls="`bi-entity-panel-${entity.id}`"
        @click="activeEntityId = entity.id"
      >
        <span class="bi-entity-tab__icon">
          <component :is="iconMap[entity.icon]" :size="18" aria-hidden="true" />
        </span>
        <span class="bi-entity-tab__copy">
          <strong>{{ entity.label }}</strong>
          <small>{{ biApiEntityFieldCount(entity) }} campos</small>
        </span>
      </button>
    </nav>

    <BiApiEntityDetail v-if="activeEntity" :entity="activeEntity" />

    <footer class="bi-catalog__footnote">
      Este catálogo documenta a estrutura observada em 23/07/2026. Ele não representa uma
      importação, sincronização ou garantia de preenchimento de todos os campos.
    </footer>
  </section>
</template>

<style scoped>
.bi-catalog {
  display: grid;
  gap: 16px;
}

.bi-catalog__intro {
  display: grid;
  grid-template-columns: minmax(0, 1.35fr) minmax(360px, 0.65fr);
  gap: 24px;
  align-items: center;
  padding: 24px;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-lg);
  background:
    radial-gradient(
      circle at 92% 8%,
      color-mix(in srgb, var(--accent-info) 15%, transparent),
      transparent 38%
    ),
    var(--bg-panel);
  box-shadow: var(--shadow-card);
}

.bi-catalog__eyebrow {
  display: inline-flex;
  gap: 7px;
  align-items: center;
  margin-bottom: 8px;
  color: var(--accent-info);
  font-size: 0.75rem;
  font-weight: 800;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.bi-catalog__intro h3 {
  margin: 0;
  color: var(--text-main);
  font-size: clamp(1.35rem, 2vw, 1.85rem);
  line-height: 1.15;
}

.bi-catalog__intro p {
  max-width: 720px;
  margin: 9px 0 0;
  color: var(--text-muted);
  line-height: 1.65;
}

.bi-catalog__stats {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
}

.bi-catalog__stats article {
  display: grid;
  gap: 4px;
  min-width: 0;
  padding: 14px 10px;
  text-align: center;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-md);
  background: color-mix(in srgb, var(--bg-panel) 82%, transparent);
}

.bi-catalog__stats strong {
  color: var(--text-main);
  font-size: 1.35rem;
}

.bi-catalog__stats span {
  color: var(--text-muted);
  font-size: 0.72rem;
  line-height: 1.25;
}

.bi-catalog__notice {
  display: flex;
  gap: 10px;
  align-items: flex-start;
  padding: 12px 14px;
  color: var(--accent-success);
  border: 1px solid color-mix(in srgb, var(--accent-success) 30%, var(--line-soft));
  border-radius: var(--radius-md);
  background: color-mix(in srgb, var(--accent-success) 7%, var(--bg-panel));
}

.bi-catalog__notice div {
  display: grid;
  gap: 2px;
}

.bi-catalog__notice strong {
  color: var(--text-main);
  font-size: 0.82rem;
}

.bi-catalog__notice span {
  color: var(--text-muted);
  font-size: 0.78rem;
  line-height: 1.45;
}

.bi-catalog__entity-tabs {
  display: grid;
  grid-template-columns: repeat(6, minmax(0, 1fr));
  gap: 8px;
}

.bi-entity-tab {
  display: flex;
  gap: 10px;
  align-items: center;
  min-width: 0;
  min-height: 58px;
  padding: 9px 11px;
  color: var(--text-muted);
  text-align: left;
  cursor: pointer;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-md);
  background: var(--bg-panel);
  transition:
    border-color 160ms ease,
    background 160ms ease,
    transform 160ms ease;
}

.bi-entity-tab:hover {
  transform: translateY(-1px);
  border-color: color-mix(in srgb, var(--accent-info) 45%, var(--line-soft));
}

.bi-entity-tab.is-active {
  color: var(--accent-info);
  border-color: color-mix(in srgb, var(--accent-info) 52%, var(--line-soft));
  background: color-mix(in srgb, var(--accent-info) 8%, var(--bg-panel));
}

.bi-entity-tab[data-tone='attention'].is-active {
  color: var(--accent-warning);
  border-color: color-mix(in srgb, var(--accent-warning) 52%, var(--line-soft));
  background: color-mix(in srgb, var(--accent-warning) 8%, var(--bg-panel));
}

.bi-entity-tab__icon {
  display: inline-grid;
  flex: 0 0 auto;
  width: 34px;
  height: 34px;
  place-items: center;
  border-radius: var(--radius-sm);
  background: color-mix(in srgb, currentColor 10%, transparent);
}

.bi-entity-tab__copy {
  display: grid;
  gap: 2px;
  min-width: 0;
}

.bi-entity-tab__copy strong,
.bi-entity-tab__copy small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.bi-entity-tab__copy strong {
  color: var(--text-main);
  font-size: 0.78rem;
}

.bi-entity-tab__copy small {
  color: var(--text-muted);
  font-size: 0.69rem;
}

.bi-catalog__footnote {
  padding: 0 3px;
  color: var(--text-muted);
  font-size: 0.72rem;
}

@media (max-width: 1180px) {
  .bi-catalog__intro {
    grid-template-columns: 1fr;
  }

  .bi-catalog__entity-tabs {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 620px) {
  .bi-catalog__intro {
    padding: 18px;
  }

  .bi-catalog__stats,
  .bi-catalog__entity-tabs {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
