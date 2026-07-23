<script setup lang="ts">
import { computed, ref } from 'vue'
import { AlertTriangle, CheckCircle2, FileWarning, Search, Send } from 'lucide-vue-next'

import AppSelectField from '~/components/ui/AppSelectField.vue'
import {
  BI_ERP_API_GAPS,
  BI_ERP_VOLUME_EVIDENCE,
  BI_GAP_AUDIT_DATE,
  BI_GAP_DOMAIN_LABELS,
  BI_GAP_PRIORITY_LABELS,
  BI_GAP_STATUS_LABELS,
  filterBiGaps,
} from '~/domain/bi/gap-catalog'

const search = ref('')
const priority = ref('')
const domain = ref('')

const priorityOptions = [
  { value: '', label: 'Todas as prioridades' },
  ...Object.entries(BI_GAP_PRIORITY_LABELS).map(([value, label]) => ({ value, label })),
]
const domainOptions = [
  { value: '', label: 'Todos os domínios' },
  ...Object.entries(BI_GAP_DOMAIN_LABELS).map(([value, label]) => ({ value, label })),
]

const filteredGaps = computed(() =>
  filterBiGaps(BI_ERP_API_GAPS, {
    search: search.value,
    priority: priority.value,
    domain: domain.value,
  }),
)
const blockingCount = BI_ERP_API_GAPS.filter((gap) => gap.priority === 'P0').length
const activeFilterCount = computed(
  () =>
    Number(Boolean(search.value.trim())) +
    Number(Boolean(priority.value)) +
    Number(Boolean(domain.value)),
)

function clearFilters() {
  search.value = ''
  priority.value = ''
  domain.value = ''
}
</script>

<template>
  <section class="bi-gaps" data-testid="bi-gap-analysis">
    <header class="bi-gaps__hero omni-glass">
      <div class="bi-gaps__hero-copy">
        <span class="bi-gaps__eyebrow">
          <FileWarning :size="15" aria-hidden="true" />
          Auditoria ERP × API Pérola
        </span>
        <h3>O que falta para a API substituir o ERP</h3>
        <p>
          Lista objetiva para encaminhar à fornecedora. A auditoria diferencia ausência confirmada,
          chave ainda não validada e contrato técnico insuficiente.
        </p>
        <span class="bi-gaps__audit-date">Levantamento atualizado em {{ BI_GAP_AUDIT_DATE }}</span>
      </div>

      <div class="bi-gaps__verdict">
        <AlertTriangle :size="22" aria-hidden="true" />
        <div>
          <strong>A substituição ainda está bloqueada</strong>
          <span>{{ blockingCount }} solicitações P0 precisam ser atendidas primeiro.</span>
        </div>
      </div>
    </header>

    <section class="bi-gaps__evidence" aria-label="Dimensão observada no ERP">
      <article v-for="item in BI_ERP_VOLUME_EVIDENCE" :key="item.label" class="omni-glass">
        <strong>{{ item.value.toLocaleString('pt-BR') }}</strong>
        <span>{{ item.label }}</span>
      </article>
    </section>

    <section class="bi-gaps__toolbar omni-glass" aria-label="Filtros das lacunas">
      <label class="bi-gaps__search">
        <span>Buscar</span>
        <span class="bi-gaps__search-control">
          <Search :size="15" aria-hidden="true" />
          <input
            v-model="search"
            type="search"
            placeholder="Campo, chave ou solicitação..."
            data-testid="bi-gap-search"
          />
        </span>
      </label>

      <AppSelectField
        v-model="priority"
        label="Prioridade"
        :options="priorityOptions"
        placeholder="Todas as prioridades"
        compact
      />
      <AppSelectField
        v-model="domain"
        label="Domínio"
        :options="domainOptions"
        placeholder="Todos os domínios"
        compact
      />

      <button
        class="bi-gaps__clear"
        type="button"
        :disabled="activeFilterCount === 0"
        @click="clearFilters"
      >
        Limpar filtros
      </button>
    </section>

    <div class="bi-gaps__result-summary">
      <strong>{{ filteredGaps.length }}</strong>
      <span>de {{ BI_ERP_API_GAPS.length }} solicitações</span>
    </div>

    <div v-if="filteredGaps.length" class="bi-gaps__grid">
      <article
        v-for="gap in filteredGaps"
        :key="gap.id"
        class="bi-gap-card omni-glass"
        :data-priority="gap.priority"
      >
        <header>
          <span class="bi-gap-card__priority">{{ gap.priority }}</span>
          <span class="bi-gap-card__domain">{{ BI_GAP_DOMAIN_LABELS[gap.domain] }}</span>
          <span class="bi-gap-card__status">{{ BI_GAP_STATUS_LABELS[gap.status] }}</span>
        </header>

        <h4>{{ gap.title }}</h4>

        <dl>
          <div>
            <dt>
              <CheckCircle2 :size="15" aria-hidden="true" />
              O ERP possui
            </dt>
            <dd>{{ gap.erpEvidence }}</dd>
          </div>
          <div>
            <dt>
              <AlertTriangle :size="15" aria-hidden="true" />
              Lacuna na API
            </dt>
            <dd>{{ gap.apiGap }}</dd>
          </div>
        </dl>

        <footer>
          <Send :size="15" aria-hidden="true" />
          <div>
            <strong>Solicitar à fornecedora</strong>
            <span>{{ gap.supplierRequest }}</span>
          </div>
        </footer>
      </article>
    </div>

    <div v-else class="bi-gaps__empty">
      <Search :size="24" aria-hidden="true" />
      <strong>Nenhuma lacuna encontrada</strong>
      <span>Ajuste a busca ou limpe os filtros.</span>
    </div>
  </section>
</template>

<style scoped>
.bi-gaps {
  display: grid;
  gap: 1rem;
}

.bi-gaps__hero {
  display: grid;
  grid-template-columns: minmax(0, 1.35fr) minmax(280px, 0.65fr);
  gap: 1rem;
  padding: 1.15rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-lg);
}

.bi-gaps__eyebrow,
.bi-gaps__verdict,
.bi-gap-card header,
.bi-gap-card dt,
.bi-gap-card footer,
.bi-gaps__result-summary {
  display: flex;
  align-items: center;
  gap: 0.45rem;
}

.bi-gaps__eyebrow {
  color: var(--accent-warning);
  font-size: 0.75rem;
  font-weight: 800;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.bi-gaps__hero h3 {
  margin: 0.45rem 0 0;
  color: var(--text-main);
  font-size: clamp(1.1rem, 2vw, 1.45rem);
}

.bi-gaps__hero p,
.bi-gaps__verdict span,
.bi-gap-card dd,
.bi-gap-card footer span,
.bi-gaps__empty span {
  color: var(--text-muted);
  line-height: 1.5;
}

.bi-gaps__hero p {
  margin: 0.45rem 0;
  max-width: 72ch;
}

.bi-gaps__audit-date {
  color: var(--text-muted);
  font-size: 0.75rem;
}

.bi-gaps__verdict {
  align-self: stretch;
  padding: 1rem;
  color: var(--accent-warning);
  border: 1px solid color-mix(in srgb, var(--accent-warning) 30%, var(--line-soft));
  border-radius: var(--radius-md);
  background: color-mix(in srgb, var(--accent-warning) 8%, transparent);
}

.bi-gaps__verdict div {
  display: grid;
  gap: 0.25rem;
}

.bi-gaps__verdict strong {
  color: var(--text-main);
}

.bi-gaps__evidence {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 0.75rem;
}

.bi-gaps__evidence article {
  display: grid;
  gap: 0.2rem;
  padding: 0.9rem 1rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-md);
}

.bi-gaps__evidence strong {
  color: var(--text-main);
  font-size: 1.25rem;
}

.bi-gaps__evidence span,
.bi-gaps__result-summary span {
  color: var(--text-muted);
  font-size: 0.78rem;
}

.bi-gaps__toolbar {
  display: grid;
  grid-template-columns: minmax(240px, 1fr) minmax(190px, 0.45fr) minmax(180px, 0.4fr) auto;
  gap: 0.75rem;
  align-items: end;
  padding: 0.85rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-md);
}

.bi-gaps__search {
  display: grid;
  gap: 0.3rem;
  color: var(--text-muted);
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.bi-gaps__search-control {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  min-height: 2.25rem;
  padding: 0 0.7rem;
  border: 1px solid var(--line-soft);
  border-radius: 999px;
  background: var(--bg-panel);
}

.bi-gaps__search-control input {
  width: 100%;
  border: 0;
  outline: 0;
  background: transparent;
  color: var(--text-main);
}

.bi-gaps__clear {
  min-height: 2.25rem;
  padding: 0 0.85rem;
  border: 1px solid var(--line-soft);
  border-radius: 999px;
  background: var(--bg-panel);
  color: var(--text-main);
  font-weight: 750;
  cursor: pointer;
}

.bi-gaps__clear:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.bi-gaps__result-summary strong {
  color: var(--text-main);
}

.bi-gaps__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.85rem;
}

.bi-gap-card {
  display: grid;
  gap: 0.8rem;
  padding: 1rem;
  border: 1px solid var(--line-soft);
  border-left: 3px solid var(--line-soft);
  border-radius: var(--radius-md);
}

.bi-gap-card[data-priority='P0'] {
  border-left-color: var(--accent-danger);
}

.bi-gap-card[data-priority='P1'] {
  border-left-color: var(--accent-warning);
}

.bi-gap-card[data-priority='P2'] {
  border-left-color: var(--accent-info);
}

.bi-gap-card header {
  flex-wrap: wrap;
}

.bi-gap-card__priority,
.bi-gap-card__domain,
.bi-gap-card__status {
  padding: 0.22rem 0.48rem;
  border: 1px solid var(--line-soft);
  border-radius: 999px;
  color: var(--text-muted);
  font-size: 0.68rem;
  font-weight: 800;
}

.bi-gap-card__priority {
  color: var(--text-main);
}

.bi-gap-card h4 {
  margin: 0;
  color: var(--text-main);
  font-size: 1rem;
}

.bi-gap-card dl {
  display: grid;
  gap: 0.65rem;
  margin: 0;
}

.bi-gap-card dl > div {
  display: grid;
  gap: 0.25rem;
}

.bi-gap-card dt {
  color: var(--text-main);
  font-size: 0.76rem;
  font-weight: 800;
}

.bi-gap-card dd {
  margin: 0;
  font-size: 0.82rem;
}

.bi-gap-card footer {
  align-items: flex-start;
  padding-top: 0.75rem;
  border-top: 1px solid var(--line-soft);
  color: var(--accent-success);
}

.bi-gap-card footer div {
  display: grid;
  gap: 0.2rem;
}

.bi-gap-card footer strong {
  color: var(--text-main);
  font-size: 0.75rem;
}

.bi-gap-card footer span {
  font-size: 0.8rem;
}

.bi-gaps__empty {
  display: grid;
  justify-items: center;
  gap: 0.35rem;
  padding: 2.5rem;
  border: 1px dashed var(--line-soft);
  border-radius: var(--radius-md);
  color: var(--text-muted);
}

.bi-gaps__empty strong {
  color: var(--text-main);
}

@media (max-width: 900px) {
  .bi-gaps__hero,
  .bi-gaps__grid {
    grid-template-columns: 1fr;
  }

  .bi-gaps__evidence {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .bi-gaps__toolbar {
    grid-template-columns: 1fr 1fr;
  }
}

@media (max-width: 620px) {
  .bi-gaps__evidence,
  .bi-gaps__toolbar {
    grid-template-columns: 1fr;
  }

  .bi-gaps__clear {
    width: 100%;
  }
}
</style>
