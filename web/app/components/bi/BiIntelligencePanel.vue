<script setup lang="ts">
import { computed, ref } from 'vue'
import { DatabaseZap, GitCompareArrows, Lightbulb, Network, ShieldCheck } from 'lucide-vue-next'

import BiIntelligenceOpportunityCard from '~/components/bi/BiIntelligenceOpportunityCard.vue'
import BiIntelligenceSourceMatrix from '~/components/bi/BiIntelligenceSourceMatrix.vue'
import BiLiveIntelligenceSnapshot from '~/components/bi/BiLiveIntelligenceSnapshot.vue'
import {
  BI_INTELLIGENCE_OPPORTUNITIES,
  BI_INTELLIGENCE_READINESS,
  BI_INTELLIGENCE_SOURCES,
} from '~/domain/bi/intelligence-catalog'
import type { BiIntelligenceSourceId } from '~/domain/bi/intelligence-catalog'
import type { BiIntelligenceSection } from '~/stores/bi'

const props = withDefaults(
  defineProps<{
    sections: BiIntelligenceSection[]
    inventoryLoading?: boolean
  }>(),
  {
    inventoryLoading: false,
  },
)

type IntelligenceView = 'opportunities' | 'sources'
type SourceFilter = 'all' | 'cross' | BiIntelligenceSourceId

const activeView = ref<IntelligenceView>('opportunities')
const sourceFilter = ref<SourceFilter>('all')

const sourceFilters: Array<{ id: SourceFilter; label: string }> = [
  { id: 'all', label: 'Todas' },
  ...BI_INTELLIGENCE_SOURCES.map((source) => ({ id: source.id, label: source.label })),
  { id: 'cross', label: 'Cruzamentos' },
]

const filteredOpportunities = computed(() => {
  if (sourceFilter.value === 'all') return BI_INTELLIGENCE_OPPORTUNITIES
  if (sourceFilter.value === 'cross') {
    return BI_INTELLIGENCE_OPPORTUNITIES.filter((opportunity) => opportunity.sources.length > 1)
  }
  return BI_INTELLIGENCE_OPPORTUNITIES.filter((opportunity) =>
    opportunity.sources.includes(sourceFilter.value as BiIntelligenceSourceId),
  )
})

const dataReadyCount = BI_INTELLIGENCE_OPPORTUNITIES.filter(
  (opportunity) => opportunity.readiness === 'data-ready',
).length
const mappingGapCount = BI_INTELLIGENCE_OPPORTUNITIES.filter(
  (opportunity) => opportunity.readiness === 'mapping-gap',
).length
</script>

<template>
  <section class="bi-intelligence" data-testid="bi-intelligence-panel">
    <header class="bi-intelligence__hero">
      <div>
        <span class="bi-intelligence__eyebrow">
          <Lightbulb :size="15" aria-hidden="true" />
          Inteligência orientada a decisões
        </span>
        <h3>O que podemos descobrir com BI, ERP e Fila</h3>
        <p>
          Este mapa separa dados disponíveis de cálculos ainda não implementados. Nenhum card
          representa um indicador ao vivo sem uma consulta explícita e segura.
        </p>
      </div>

      <div class="bi-intelligence__summary" aria-label="Resumo das oportunidades">
        <article>
          <strong>{{ BI_INTELLIGENCE_OPPORTUNITIES.length }}</strong>
          <span>leituras possíveis</span>
        </article>
        <article>
          <strong>{{ dataReadyCount }}</strong>
          <span>com dados internos</span>
        </article>
        <article>
          <strong>{{ mappingGapCount }}</strong>
          <span>dependem de ligação</span>
        </article>
      </div>
    </header>

    <nav class="bi-intelligence__views" aria-label="Visões da inteligência">
      <button
        type="button"
        :class="{ 'is-active': activeView === 'opportunities' }"
        @click="activeView = 'opportunities'"
      >
        <DatabaseZap :size="16" aria-hidden="true" />
        Leituras possíveis
      </button>
      <button
        type="button"
        :class="{ 'is-active': activeView === 'sources' }"
        @click="activeView = 'sources'"
      >
        <GitCompareArrows :size="16" aria-hidden="true" />
        BI × ERP × Fila
      </button>
    </nav>

    <template v-if="activeView === 'opportunities'">
      <section class="bi-intelligence__toolbar">
        <div class="bi-intelligence__filters" aria-label="Filtrar por fonte">
          <button
            v-for="filter in sourceFilters"
            :key="filter.id"
            type="button"
            :class="{ 'is-active': sourceFilter === filter.id }"
            @click="sourceFilter = filter.id"
          >
            {{ filter.label }}
          </button>
        </div>

        <div class="bi-intelligence__legend">
          <span v-for="(readiness, readinessId) in BI_INTELLIGENCE_READINESS" :key="readinessId">
            <i :data-readiness="readinessId" aria-hidden="true"></i>
            {{ readiness.label }}
          </span>
        </div>
      </section>

      <div class="bi-intelligence__grid">
        <BiIntelligenceOpportunityCard
          v-for="opportunity in filteredOpportunities"
          :key="opportunity.id"
          :opportunity="opportunity"
        />
      </div>
    </template>

    <BiIntelligenceSourceMatrix v-else />

    <aside class="bi-intelligence__next">
      <Network :size="18" aria-hidden="true" />
      <div>
        <strong>Próximo passo técnico seguro</strong>
        <span>
          Confirmar as chaves Documento ERP ↔ Nota e itemSaldoId ↔ Item/SKU. Depois disso,
          implementar agregações por período no backend, sem varrer as seis entidades no navegador.
        </span>
      </div>
      <ShieldCheck :size="18" aria-hidden="true" />
    </aside>

    <BiLiveIntelligenceSnapshot
      :sections="props.sections"
      :inventory-loading="props.inventoryLoading"
    />
  </section>
</template>

<style scoped>
.bi-intelligence {
  display: grid;
  gap: 16px;
}

.bi-intelligence__hero {
  display: grid;
  grid-template-columns: minmax(0, 1.25fr) minmax(330px, 0.75fr);
  gap: 22px;
  align-items: center;
  padding: 22px;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-lg);
  background:
    radial-gradient(
      circle at 88% 10%,
      color-mix(in srgb, var(--accent-info) 13%, transparent),
      transparent 37%
    ),
    var(--bg-panel);
  box-shadow: var(--shadow-card);
}

.bi-intelligence__eyebrow {
  display: inline-flex;
  gap: 7px;
  align-items: center;
  color: var(--accent-info);
  font-size: 0.72rem;
  font-weight: 800;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.bi-intelligence__hero h3 {
  margin: 6px 0;
  color: var(--text-main);
  font-size: clamp(1.3rem, 2vw, 1.75rem);
}

.bi-intelligence__hero p {
  max-width: 700px;
  margin: 0;
  color: var(--text-muted);
  line-height: 1.6;
}

.bi-intelligence__summary {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
}

.bi-intelligence__summary article {
  display: grid;
  gap: 3px;
  padding: 13px 9px;
  text-align: center;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-md);
  background: color-mix(in srgb, var(--bg-panel) 84%, transparent);
}

.bi-intelligence__summary strong {
  color: var(--text-main);
  font-size: 1.3rem;
}

.bi-intelligence__summary span {
  color: var(--text-muted);
  font-size: 0.68rem;
  line-height: 1.3;
}

.bi-intelligence__views,
.bi-intelligence__filters,
.bi-intelligence__legend {
  display: flex;
  gap: 7px;
  align-items: center;
  flex-wrap: wrap;
}

.bi-intelligence__views {
  padding-bottom: 11px;
  border-bottom: 1px solid var(--line-soft);
}

.bi-intelligence__views button,
.bi-intelligence__filters button {
  display: inline-flex;
  gap: 6px;
  align-items: center;
  justify-content: center;
  min-height: 35px;
  padding: 0 11px;
  color: var(--text-muted);
  font-size: 0.75rem;
  font-weight: 750;
  cursor: pointer;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: var(--bg-panel);
}

.bi-intelligence__views button.is-active,
.bi-intelligence__filters button.is-active {
  color: var(--accent-info);
  border-color: color-mix(in srgb, var(--accent-info) 42%, var(--line-soft));
  background: color-mix(in srgb, var(--accent-info) 8%, var(--bg-panel));
}

.bi-intelligence__toolbar {
  display: flex;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
}

.bi-intelligence__legend {
  justify-content: flex-end;
}

.bi-intelligence__legend span {
  display: inline-flex;
  gap: 5px;
  align-items: center;
  color: var(--text-muted);
  font-size: 0.66rem;
}

.bi-intelligence__legend i {
  width: 7px;
  height: 7px;
  border-radius: 999px;
  background: var(--accent-success);
}

.bi-intelligence__legend i[data-readiness='safe-query'] {
  background: var(--accent-warning);
}

.bi-intelligence__legend i[data-readiness='mapping-gap'] {
  background: var(--accent-info);
}

.bi-intelligence__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.bi-intelligence__next {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  gap: 10px;
  align-items: flex-start;
  padding: 13px 15px;
  color: var(--accent-info);
  border: 1px solid color-mix(in srgb, var(--accent-info) 30%, var(--line-soft));
  border-radius: var(--radius-md);
  background: color-mix(in srgb, var(--accent-info) 6%, var(--bg-panel));
}

.bi-intelligence__next div {
  display: grid;
  gap: 3px;
}

.bi-intelligence__next strong {
  color: var(--text-main);
  font-size: 0.8rem;
}

.bi-intelligence__next span {
  color: var(--text-muted);
  font-size: 0.75rem;
  line-height: 1.45;
}

@media (max-width: 1040px) {
  .bi-intelligence__hero,
  .bi-intelligence__grid {
    grid-template-columns: 1fr;
  }

  .bi-intelligence__toolbar {
    align-items: flex-start;
    flex-direction: column;
  }

  .bi-intelligence__legend {
    justify-content: flex-start;
  }
}

@media (max-width: 620px) {
  .bi-intelligence__hero {
    padding: 17px;
  }

  .bi-intelligence__summary {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .bi-intelligence__views button,
  .bi-intelligence__filters button {
    flex: 1 1 auto;
  }
}
</style>
