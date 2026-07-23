<script setup lang="ts">
import { ArrowRight, CheckCircle2, KeyRound, ShieldAlert } from 'lucide-vue-next'

import { BI_INTELLIGENCE_READINESS, biIntelligenceSource } from '~/domain/bi/intelligence-catalog'
import type { BiIntelligenceOpportunity } from '~/domain/bi/intelligence-catalog'

defineProps<{
  opportunity: BiIntelligenceOpportunity
}>()

const readinessIcon = {
  'data-ready': CheckCircle2,
  'safe-query': ShieldAlert,
  'mapping-gap': KeyRound,
}
</script>

<template>
  <article class="bi-opportunity" :data-readiness="opportunity.readiness">
    <header>
      <div class="bi-opportunity__sources">
        <span v-for="sourceId in opportunity.sources" :key="sourceId" :data-source="sourceId">
          {{ biIntelligenceSource(sourceId)?.shortLabel }}
        </span>
      </div>

      <span class="bi-opportunity__readiness">
        <component :is="readinessIcon[opportunity.readiness]" :size="14" aria-hidden="true" />
        {{ BI_INTELLIGENCE_READINESS[opportunity.readiness].label }}
      </span>
    </header>

    <div class="bi-opportunity__title">
      <span>{{ opportunity.question }}</span>
      <h4>{{ opportunity.title }}</h4>
      <p>{{ opportunity.outcome }}</p>
    </div>

    <div class="bi-opportunity__body">
      <section>
        <h5>Ingredientes</h5>
        <ul>
          <li v-for="ingredient in opportunity.ingredients" :key="ingredient">
            <ArrowRight :size="13" aria-hidden="true" />
            {{ ingredient }}
          </li>
        </ul>
      </section>

      <section>
        <h5>Recortes possíveis</h5>
        <div class="bi-opportunity__dimensions">
          <span v-for="dimension in opportunity.dimensions" :key="dimension">
            {{ dimension }}
          </span>
        </div>
      </section>
    </div>

    <footer v-if="opportunity.guardrail">
      <ShieldAlert :size="15" aria-hidden="true" />
      <span>{{ opportunity.guardrail }}</span>
    </footer>
  </article>
</template>

<style scoped>
.bi-opportunity {
  display: grid;
  gap: 15px;
  min-width: 0;
  padding: 17px;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-lg);
  background: var(--bg-panel);
  box-shadow: var(--shadow-card);
}

.bi-opportunity[data-readiness='safe-query'] {
  border-color: color-mix(in srgb, var(--accent-warning) 32%, var(--line-soft));
}

.bi-opportunity[data-readiness='mapping-gap'] {
  border-color: color-mix(in srgb, var(--accent-info) 35%, var(--line-soft));
}

.bi-opportunity > header,
.bi-opportunity__sources,
.bi-opportunity__dimensions {
  display: flex;
  gap: 7px;
  align-items: center;
  flex-wrap: wrap;
}

.bi-opportunity > header {
  justify-content: space-between;
}

.bi-opportunity__sources span,
.bi-opportunity__dimensions span {
  border: 1px solid var(--line-soft);
  border-radius: 999px;
  font-size: 0.68rem;
  font-weight: 800;
}

.bi-opportunity__sources span {
  padding: 4px 7px;
  color: var(--text-muted);
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.bi-opportunity__sources span[data-source='perola'] {
  color: var(--accent-info);
  border-color: color-mix(in srgb, var(--accent-info) 35%, var(--line-soft));
  background: color-mix(in srgb, var(--accent-info) 7%, var(--bg-panel));
}

.bi-opportunity__sources span[data-source='erp'] {
  color: var(--accent-success);
  border-color: color-mix(in srgb, var(--accent-success) 35%, var(--line-soft));
  background: color-mix(in srgb, var(--accent-success) 7%, var(--bg-panel));
}

.bi-opportunity__sources span[data-source='queue'] {
  color: var(--accent-warning);
  border-color: color-mix(in srgb, var(--accent-warning) 35%, var(--line-soft));
  background: color-mix(in srgb, var(--accent-warning) 7%, var(--bg-panel));
}

.bi-opportunity__readiness {
  display: inline-flex;
  gap: 5px;
  align-items: center;
  color: var(--text-muted);
  font-size: 0.7rem;
  font-weight: 750;
}

.bi-opportunity__title {
  display: grid;
  gap: 4px;
}

.bi-opportunity__title > span {
  color: var(--text-muted);
  font-size: 0.72rem;
}

.bi-opportunity h4,
.bi-opportunity h5 {
  margin: 0;
  color: var(--text-main);
}

.bi-opportunity h4 {
  font-size: 1.03rem;
}

.bi-opportunity h5 {
  font-size: 0.74rem;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.bi-opportunity p {
  margin: 2px 0 0;
  color: var(--text-muted);
  font-size: 0.82rem;
  line-height: 1.55;
}

.bi-opportunity__body {
  display: grid;
  grid-template-columns: minmax(0, 1.35fr) minmax(150px, 0.65fr);
  gap: 14px;
  padding-top: 13px;
  border-top: 1px solid var(--line-soft);
}

.bi-opportunity__body section {
  display: grid;
  gap: 9px;
  align-content: start;
}

.bi-opportunity ul {
  display: grid;
  gap: 7px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.bi-opportunity li {
  display: flex;
  gap: 6px;
  align-items: flex-start;
  color: var(--text-muted);
  font-size: 0.76rem;
  line-height: 1.4;
}

.bi-opportunity li svg {
  flex: 0 0 auto;
  margin-top: 2px;
  color: var(--accent-success);
}

.bi-opportunity__dimensions span {
  padding: 4px 7px;
  color: var(--text-muted);
  background: var(--bg-muted);
}

.bi-opportunity > footer {
  display: flex;
  gap: 7px;
  align-items: flex-start;
  padding: 9px 10px;
  color: var(--accent-warning);
  border-radius: var(--radius-sm);
  background: color-mix(in srgb, var(--accent-warning) 7%, var(--bg-muted));
}

.bi-opportunity > footer span {
  color: var(--text-muted);
  font-size: 0.72rem;
  line-height: 1.4;
}

@media (max-width: 620px) {
  .bi-opportunity__body {
    grid-template-columns: 1fr;
  }

  .bi-opportunity > header {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
