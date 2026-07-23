<script setup lang="ts">
import { Check, Database, Sparkles } from 'lucide-vue-next'

import {
  BI_INTELLIGENCE_SOURCES,
  BI_SOURCE_COMPARISON,
  ERP_DATA_NOT_OBSERVED_IN_PEROLA,
} from '~/domain/bi/intelligence-catalog'
</script>

<template>
  <section class="bi-source-map">
    <header class="bi-source-map__header">
      <div>
        <span>
          <Database :size="14" aria-hidden="true" />
          Inventário comparativo
        </span>
        <h3>O papel de cada fonte</h3>
        <p>
          As três fontes se complementam. O ERP não substitui a Pérola fiscal, e a Pérola não
          registra toda a jornada operacional ou os contatos comerciais.
        </p>
      </div>
    </header>

    <div class="bi-source-map__sources">
      <article v-for="source in BI_INTELLIGENCE_SOURCES" :key="source.id" :data-source="source.id">
        <strong>{{ source.label }}</strong>
        <p>{{ source.description }}</p>
      </article>
    </div>

    <div class="bi-source-map__table-wrap">
      <table>
        <thead>
          <tr>
            <th>Domínio</th>
            <th>BI Pérola</th>
            <th>ERP</th>
            <th>Fila</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in BI_SOURCE_COMPARISON" :key="row.domain">
            <th>{{ row.domain }}</th>
            <td>{{ row.perola }}</td>
            <td>{{ row.erp }}</td>
            <td>{{ row.queue }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <aside class="bi-source-map__erp-gap">
      <header>
        <span>
          <Sparkles :size="15" aria-hidden="true" />
          Exclusivo do contrato ERP atual
        </span>
        <h4>O que temos no ERP e não observamos no BI Pérola</h4>
      </header>

      <ul>
        <li v-for="item in ERP_DATA_NOT_OBSERVED_IN_PEROLA" :key="item">
          <Check :size="14" aria-hidden="true" />
          {{ item }}
        </li>
      </ul>
    </aside>
  </section>
</template>

<style scoped>
.bi-source-map {
  display: grid;
  gap: 15px;
}

.bi-source-map__header,
.bi-source-map__erp-gap {
  padding: 18px;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-lg);
  background: var(--bg-panel);
  box-shadow: var(--shadow-card);
}

.bi-source-map__header span,
.bi-source-map__erp-gap header > span {
  display: inline-flex;
  gap: 6px;
  align-items: center;
  color: var(--accent-info);
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.bi-source-map h3,
.bi-source-map h4 {
  margin: 5px 0;
  color: var(--text-main);
}

.bi-source-map h3 {
  font-size: 1.1rem;
}

.bi-source-map h4 {
  font-size: 0.95rem;
}

.bi-source-map__header p,
.bi-source-map__sources p {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.8rem;
  line-height: 1.5;
}

.bi-source-map__sources {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
}

.bi-source-map__sources article {
  display: grid;
  gap: 5px;
  padding: 14px;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-md);
  background: var(--bg-muted);
}

.bi-source-map__sources article[data-source='perola'] {
  border-top-color: var(--accent-info);
}

.bi-source-map__sources article[data-source='erp'] {
  border-top-color: var(--accent-success);
}

.bi-source-map__sources article[data-source='queue'] {
  border-top-color: var(--accent-warning);
}

.bi-source-map__sources strong {
  color: var(--text-main);
  font-size: 0.8rem;
}

.bi-source-map__table-wrap {
  overflow-x: auto;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-lg);
  background: var(--bg-panel);
}

.bi-source-map table {
  width: 100%;
  min-width: 800px;
  border-collapse: collapse;
}

.bi-source-map th,
.bi-source-map td {
  padding: 12px 13px;
  text-align: left;
  vertical-align: top;
  border-bottom: 1px solid var(--line-soft);
}

.bi-source-map thead th {
  color: var(--text-muted);
  font-size: 0.68rem;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  background: var(--bg-muted);
}

.bi-source-map tbody th {
  width: 105px;
  color: var(--text-main);
  font-size: 0.78rem;
}

.bi-source-map td {
  color: var(--text-muted);
  font-size: 0.76rem;
  line-height: 1.45;
}

.bi-source-map tbody tr:last-child th,
.bi-source-map tbody tr:last-child td {
  border-bottom: 0;
}

.bi-source-map__erp-gap {
  display: grid;
  grid-template-columns: minmax(220px, 0.8fr) minmax(0, 1.2fr);
  gap: 18px;
  border-color: color-mix(in srgb, var(--accent-success) 32%, var(--line-soft));
  background:
    radial-gradient(
      circle at 0 0,
      color-mix(in srgb, var(--accent-success) 9%, transparent),
      transparent 34%
    ),
    var(--bg-panel);
}

.bi-source-map__erp-gap header > span {
  color: var(--accent-success);
}

.bi-source-map__erp-gap ul {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px 12px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.bi-source-map__erp-gap li {
  display: flex;
  gap: 7px;
  align-items: flex-start;
  color: var(--text-muted);
  font-size: 0.75rem;
  line-height: 1.4;
}

.bi-source-map__erp-gap li svg {
  flex: 0 0 auto;
  margin-top: 2px;
  color: var(--accent-success);
}

@media (max-width: 820px) {
  .bi-source-map__sources,
  .bi-source-map__erp-gap {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 620px) {
  .bi-source-map__erp-gap ul {
    grid-template-columns: 1fr;
  }
}
</style>
