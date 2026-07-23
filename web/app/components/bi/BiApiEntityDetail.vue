<script setup lang="ts">
import type { Component } from 'vue'
import {
  BadgeDollarSign,
  Image as ImageIcon,
  Layers3,
  Link2,
  ListTree,
  PackageSearch,
  ReceiptText,
  TriangleAlert,
  Warehouse,
} from 'lucide-vue-next'

import {
  biApiEntityFieldCount,
  biApiFieldLabel,
  biApiFieldTypeLabel,
} from '~/domain/bi/api-catalog'
import type { BiApiEntity } from '~/domain/bi/api-catalog'

defineProps<{
  entity: BiApiEntity
}>()

const iconMap: Record<BiApiEntity['icon'], Component> = {
  package: PackageSearch,
  image: ImageIcon,
  coins: BadgeDollarSign,
  receipt: ReceiptText,
  list: ListTree,
  warehouse: Warehouse,
}
</script>

<template>
  <article
    :id="`bi-entity-panel-${entity.id}`"
    class="bi-entity-detail omni-glass"
    role="tabpanel"
    :aria-labelledby="`bi-entity-tab-${entity.id}`"
    :data-tone="entity.tone"
  >
    <header class="bi-entity-detail__header">
      <div class="bi-entity-detail__identity">
        <span class="bi-entity-detail__icon">
          <component :is="iconMap[entity.icon]" :size="23" aria-hidden="true" />
        </span>
        <div>
          <span class="bi-entity-detail__endpoint">POST {{ entity.endpoint }}</span>
          <h3>{{ entity.label }}</h3>
          <p>{{ entity.description }}</p>
        </div>
      </div>

      <span class="bi-entity-detail__count">
        {{ biApiEntityFieldCount(entity) }} campos observados
      </span>
    </header>

    <div class="bi-entity-detail__summary">
      <section>
        <h4>Informações disponíveis</h4>
        <ul>
          <li v-for="item in entity.availableInformation" :key="item">
            <span aria-hidden="true"></span>
            {{ item }}
          </li>
        </ul>
      </section>

      <aside class="bi-entity-detail__rules">
        <div>
          <Layers3 :size="16" aria-hidden="true" />
          <p>
            <strong>Regra de consulta</strong>
            <span>{{ entity.queryRule }}</span>
          </p>
        </div>
        <div v-if="entity.relation">
          <Link2 :size="16" aria-hidden="true" />
          <p>
            <strong>Relacionamento</strong>
            <span>{{ entity.relation }}</span>
          </p>
        </div>
        <div :data-alert="entity.tone === 'attention'">
          <TriangleAlert :size="16" aria-hidden="true" />
          <p>
            <strong>Comportamento observado</strong>
            <span>{{ entity.performance }}</span>
          </p>
        </div>
      </aside>
    </div>

    <section class="bi-entity-detail__fields">
      <header>
        <div>
          <span>Contrato observado</span>
          <h4>Campos retornados pela API</h4>
        </div>
        <p>
          “Opcional” significa que a amostra veio nula; o tipo definitivo ainda precisa ser
          confirmado com outros registros.
        </p>
      </header>

      <div class="bi-field-groups">
        <section
          v-for="fieldGroup in entity.fieldGroups"
          :key="fieldGroup.id"
          class="bi-field-group"
        >
          <header>
            <h5>{{ fieldGroup.label }}</h5>
            <span>{{ fieldGroup.fields.length }}</span>
          </header>

          <dl>
            <div v-for="apiField in fieldGroup.fields" :key="apiField.key" class="bi-api-field">
              <dt>{{ biApiFieldLabel(apiField.key) }}</dt>
              <dd>
                <code>{{ apiField.key }}</code>
                <span :data-type="apiField.type">
                  {{ biApiFieldTypeLabel(apiField.type) }}
                </span>
              </dd>
            </div>
          </dl>
        </section>
      </div>
    </section>
  </article>
</template>

<style scoped>
.bi-entity-detail {
  display: grid;
  gap: 1rem;
  padding: 1.15rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-shell);
  background: rgb(var(--surface) / 0.9);
  box-shadow: var(--shadow-card);
}

.bi-entity-detail[data-tone='attention'] {
  border-color: color-mix(in srgb, var(--accent-warning) 42%, var(--line-soft));
}

.bi-entity-detail[data-tone='sensitive'] {
  border-color: rgb(var(--danger) / 0.32);
}

.bi-entity-detail__header,
.bi-entity-detail__identity,
.bi-entity-detail__fields > header,
.bi-field-group > header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
}

.bi-entity-detail__identity {
  justify-content: flex-start;
}

.bi-entity-detail__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 auto;
  width: 3rem;
  height: 3rem;
  border-radius: var(--radius-soft);
  background: rgb(var(--primary) / 0.12);
  color: var(--accent-info);
}

.bi-entity-detail__endpoint,
.bi-entity-detail__fields > header span {
  display: inline-flex;
  color: var(--accent-info);
  font-size: 0.72rem;
  font-weight: 800;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.bi-entity-detail__header h3,
.bi-entity-detail__summary h4,
.bi-entity-detail__fields h4,
.bi-field-group h5 {
  margin: 0;
  color: var(--text-main);
}

.bi-entity-detail__header h3 {
  margin-top: 0.2rem;
  font-size: 1.25rem;
}

.bi-entity-detail__header p,
.bi-entity-detail__fields > header p {
  color: var(--text-muted);
  line-height: 1.55;
}

.bi-entity-detail__header p {
  margin: 0.25rem 0 0;
}

.bi-entity-detail__count {
  flex: 0 0 auto;
  padding: 0.45rem 0.65rem;
  border-radius: 999px;
  background: rgb(var(--primary) / 0.1);
  color: var(--accent-info);
  font-size: 0.75rem;
  font-weight: 800;
}

.bi-entity-detail__summary {
  display: grid;
  grid-template-columns: minmax(0, 1.25fr) minmax(19rem, 0.75fr);
  gap: 1rem;
}

.bi-entity-detail__summary > section,
.bi-entity-detail__rules {
  padding: 0.9rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface-2) / 0.55);
}

.bi-entity-detail__summary h4 {
  font-size: 0.92rem;
}

.bi-entity-detail__summary ul {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.55rem 1rem;
  margin: 0.75rem 0 0;
  padding: 0;
  list-style: none;
}

.bi-entity-detail__summary li {
  display: flex;
  align-items: flex-start;
  gap: 0.45rem;
  color: var(--text-muted);
  font-size: 0.82rem;
  line-height: 1.4;
}

.bi-entity-detail__summary li span {
  width: 0.42rem;
  height: 0.42rem;
  margin-top: 0.36rem;
  flex: 0 0 auto;
  border-radius: 999px;
  background: var(--accent-success);
}

.bi-entity-detail__rules {
  display: grid;
  gap: 0.7rem;
}

.bi-entity-detail__rules > div {
  display: flex;
  align-items: flex-start;
  gap: 0.6rem;
  color: var(--accent-info);
}

.bi-entity-detail__rules > div[data-alert='true'] {
  color: var(--accent-warning);
}

.bi-entity-detail__rules p {
  display: grid;
  gap: 0.15rem;
  margin: 0;
}

.bi-entity-detail__rules strong {
  color: var(--text-main);
  font-size: 0.78rem;
}

.bi-entity-detail__rules span {
  color: var(--text-muted);
  font-size: 0.78rem;
  line-height: 1.4;
}

.bi-entity-detail__fields {
  display: grid;
  gap: 0.8rem;
}

.bi-entity-detail__fields > header {
  align-items: flex-end;
}

.bi-entity-detail__fields > header h4 {
  margin-top: 0.2rem;
  font-size: 1rem;
}

.bi-entity-detail__fields > header p {
  max-width: 36rem;
  margin: 0;
  font-size: 0.75rem;
  text-align: right;
}

.bi-field-groups {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem;
}

.bi-field-group {
  min-width: 0;
  overflow: hidden;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface-2) / 0.42);
}

.bi-field-group > header {
  padding: 0.7rem 0.8rem;
  border-bottom: 1px solid var(--line-soft);
}

.bi-field-group h5 {
  font-size: 0.82rem;
}

.bi-field-group > header span {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 1.6rem;
  height: 1.6rem;
  border-radius: 999px;
  background: rgb(var(--primary) / 0.1);
  color: var(--accent-info);
  font-size: 0.7rem;
  font-weight: 800;
}

.bi-field-group dl {
  margin: 0;
}

.bi-api-field {
  display: grid;
  grid-template-columns: minmax(9rem, 0.85fr) minmax(0, 1.15fr);
  gap: 0.75rem;
  padding: 0.58rem 0.8rem;
  border-bottom: 1px solid var(--line-soft);
}

.bi-api-field:last-child {
  border-bottom: 0;
}

.bi-api-field dt {
  color: var(--text-main);
  font-size: 0.77rem;
  font-weight: 700;
}

.bi-api-field dd {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  min-width: 0;
  margin: 0;
}

.bi-api-field code {
  overflow: hidden;
  color: var(--text-muted);
  font-size: 0.7rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.bi-api-field dd span {
  flex: 0 0 auto;
  padding: 0.2rem 0.4rem;
  border-radius: 999px;
  background: rgb(var(--primary) / 0.09);
  color: var(--accent-info);
  font-size: 0.62rem;
  font-weight: 750;
}

.bi-api-field dd span[data-type='null-observed'] {
  background: rgb(var(--muted) / 0.1);
  color: var(--text-muted);
}

@media (max-width: 820px) {
  .bi-entity-detail__summary,
  .bi-field-groups {
    grid-template-columns: 1fr;
  }

  .bi-entity-detail__header,
  .bi-entity-detail__fields > header {
    align-items: flex-start;
    flex-direction: column;
  }

  .bi-entity-detail__fields > header p {
    text-align: left;
  }
}

@media (max-width: 620px) {
  .bi-entity-detail__summary ul {
    grid-template-columns: 1fr;
  }

  .bi-api-field {
    grid-template-columns: 1fr;
    gap: 0.3rem;
  }

  .bi-api-field dd {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
