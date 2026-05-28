<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Download, Plus } from 'lucide-vue-next'
import RoadmapRuleCard from '~/components/roadmap/RoadmapRuleCard.vue'
import RoadmapRuleForm from '~/components/roadmap/RoadmapRuleForm.vue'
import {
  ROADMAP_RULE_CATEGORY_LABEL,
  type RoadmapRule,
  type RuleCategory,
} from '~/components/roadmap/roadmap-data'
import { useRoadmapStore, type RoadmapRuleRow } from '~/stores/roadmap'

const CATEGORY_ORDER: RuleCategory[] = [
  'frontend',
  'backend',
  'banco',
  'linguagens',
  'deploy',
  'padroes-gerais',
]

const activeCategory = ref<RuleCategory | 'all'>('all')
const store = useRoadmapStore()
const copied = ref(false)
const showForm = ref(false)

onMounted(() => {
  if (!store.rules.length) {
    void store.fetchAll()
  }
})

const filteredRules = computed<RoadmapRuleRow[]>(() => {
  const list = store.rules
  if (activeCategory.value === 'all') return list
  return list.filter((r) => r.category === activeCategory.value)
})

const groupedByCategory = computed(() => {
  return CATEGORY_ORDER.map((cat) => ({
    category: cat,
    label: ROADMAP_RULE_CATEGORY_LABEL[cat],
    rules: filteredRules.value.filter((r) => r.category === cat),
  })).filter((g) => g.rules.length > 0)
})

function buildMarkdown(rules: RoadmapRule[]): string {
  const lines: string[] = []
  lines.push('# AGENT_RULES.md')
  lines.push('')
  lines.push(
    'Regras canonicas que todo agente/IA deve ler antes de iniciar qualquer tarefa neste projeto.',
  )
  lines.push('')
  lines.push('Gerado em ' + new Date().toISOString() + ' a partir do /roadmap > Regras.')
  lines.push('')

  for (const cat of CATEGORY_ORDER) {
    const items = rules.filter((r) => r.category === cat)
    if (!items.length) continue
    lines.push('## ' + ROADMAP_RULE_CATEGORY_LABEL[cat])
    lines.push('')
    for (const r of items) {
      lines.push('### ' + r.title)
      lines.push(r.body)
      lines.push('')
      if (r.why) lines.push('- **Por que:** ' + r.why)
      if (r.appliesWhen) lines.push('- **Aplica quando:** ' + r.appliesWhen)
      lines.push('')
    }
    lines.push('---')
    lines.push('')
  }

  return lines.join('\n')
}

function exportMarkdown() {
  const content = buildMarkdown(store.rules)
  const blob = new Blob([content], { type: 'text/markdown;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = 'AGENT_RULES.md'
  document.body.appendChild(anchor)
  anchor.click()
  document.body.removeChild(anchor)
  URL.revokeObjectURL(url)
}

async function copyMarkdown() {
  const content = buildMarkdown(store.rules)
  if (!navigator?.clipboard?.writeText) return
  try {
    await navigator.clipboard.writeText(content)
    copied.value = true
    setTimeout(() => (copied.value = false), 1800)
  } catch {
    // ignore
  }
}

async function handleUpdate(
  r: RoadmapRuleRow,
  payload: { title: string; body: string; why: string; appliesWhen: string },
) {
  try {
    await store.updateRule(r.id, payload)
  } catch (err) {
    console.error('roadmap.rules.update failed', err)
  }
}

async function handleCreate(payload: {
  sourceId: string
  category: RuleCategory
  title: string
  body: string
  why: string
  appliesWhen: string
}) {
  try {
    await store.createRule(payload)
    showForm.value = false
  } catch (err) {
    console.error('roadmap.rules.create failed', err)
  }
}

async function handleDelete(r: RoadmapRuleRow) {
  if (!window.confirm(`Apagar a regra "${r.title}"? Vira o seed global.`)) return
  try {
    await store.deleteRule(r.id)
  } catch (err) {
    console.error('roadmap.rules.delete failed', err)
  }
}
</script>

<template>
  <div class="roadmap-rules-board">
    <header class="roadmap-rules-board__header">
      <div class="roadmap-rules-board__intro">
        <h3 class="roadmap-rules-board__title">Regras para agentes</h3>
        <p class="roadmap-rules-board__text">
          Padroes obrigatorios de front, back, banco, linguagens e deploy. Versao canonica vive em
          <code>AGENT_RULES.md</code>
          na raiz do projeto.
          <span v-if="!store.backendAvailable" class="roadmap-rules-board__badge-ro">
            Modo leitura
          </span>
          <span v-else class="roadmap-rules-board__badge-live">Persistido no banco</span>
        </p>
      </div>

      <div class="roadmap-rules-board__actions">
        <button
          v-if="store.backendAvailable"
          type="button"
          class="roadmap-rules-board__btn roadmap-rules-board__btn--accent"
          @click="showForm = !showForm"
        >
          <Plus :size="15" :stroke-width="2.2" aria-hidden="true" />
          {{ showForm ? 'Fechar' : 'Nova regra' }}
        </button>
        <button
          type="button"
          class="roadmap-rules-board__btn roadmap-rules-board__btn--secondary"
          @click="copyMarkdown"
        >
          {{ copied ? 'Copiado!' : 'Copiar .md' }}
        </button>
        <button
          type="button"
          class="roadmap-rules-board__btn roadmap-rules-board__btn--primary"
          @click="exportMarkdown"
        >
          <Download :size="15" :stroke-width="2.2" aria-hidden="true" />
          Exportar .md
        </button>
      </div>
    </header>

    <RoadmapRuleForm v-if="showForm" @submit="handleCreate" @cancel="showForm = false" />

    <p v-if="store.error" class="roadmap-rules-board__error">{{ store.error }}</p>

    <nav class="roadmap-rules-board__filters" aria-label="Filtros por categoria">
      <button
        type="button"
        class="roadmap-rules-board__filter"
        :class="{ 'is-active': activeCategory === 'all' }"
        @click="activeCategory = 'all'"
      >
        Todas
      </button>
      <button
        v-for="cat in CATEGORY_ORDER"
        :key="cat"
        type="button"
        class="roadmap-rules-board__filter"
        :class="{ 'is-active': activeCategory === cat }"
        @click="activeCategory = cat"
      >
        {{ ROADMAP_RULE_CATEGORY_LABEL[cat] }}
      </button>
    </nav>

    <p v-if="store.loading && !filteredRules.length" class="roadmap-rules-board__empty">
      Carregando...
    </p>

    <section
      v-for="group in groupedByCategory"
      :key="group.category"
      class="roadmap-rules-board__group"
    >
      <header class="roadmap-rules-board__group-head">
        <h4 class="roadmap-rules-board__group-title">{{ group.label }}</h4>
        <span class="roadmap-rules-board__group-count">{{ group.rules.length }}</span>
      </header>
      <div class="roadmap-rules-board__grid">
        <RoadmapRuleCard
          v-for="r in group.rules"
          :key="r.id ?? r.sourceId"
          :rule="r"
          :editable="store.backendAvailable"
          @update="(payload) => handleUpdate(r, payload)"
          @delete="handleDelete(r)"
        />
      </div>
    </section>

    <p v-if="!groupedByCategory.length && !store.loading" class="roadmap-rules-board__empty">
      Nenhuma regra nessa categoria.
    </p>
  </div>
</template>

<style scoped>
.roadmap-rules-board {
  display: grid;
  gap: 1.2rem;
}

.roadmap-rules-board__header {
  display: grid;
  gap: 1rem;
}

@media (min-width: 760px) {
  .roadmap-rules-board__header {
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: center;
  }
}

.roadmap-rules-board__intro {
  display: grid;
  gap: 0.35rem;
}

.roadmap-rules-board__title {
  margin: 0;
  font-size: 1.25rem;
  color: var(--text-main);
}

.roadmap-rules-board__text {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.88rem;
  line-height: 1.45;
  max-width: 60ch;
}

.roadmap-rules-board__text code {
  font-family: ui-monospace, SFMono-Regular, monospace;
  font-size: 0.85em;
  padding: 0.05rem 0.3rem;
  border-radius: 4px;
  background: rgb(var(--muted) / 0.4);
}

.roadmap-rules-board__badge-ro,
.roadmap-rules-board__badge-live {
  display: inline-block;
  margin-left: 0.4rem;
  padding: 0.05rem 0.5rem;
  border-radius: 999px;
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.roadmap-rules-board__badge-ro {
  background: rgb(var(--muted) / 0.4);
  color: var(--text-muted);
}

.roadmap-rules-board__badge-live {
  background: rgb(var(--success) / 0.18);
  color: rgb(var(--success));
}

.roadmap-rules-board__error {
  margin: 0;
  padding: 0.7rem 0.9rem;
  border-radius: 10px;
  background: rgb(var(--danger) / 0.14);
  color: rgb(var(--danger));
  font-size: 0.85rem;
}

.roadmap-rules-board__actions {
  display: inline-flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.roadmap-rules-board__btn {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.5rem 0.95rem;
  border: 1px solid var(--admin-header-border);
  border-radius: 10px;
  background: transparent;
  color: var(--text-main);
  font-size: 0.82rem;
  font-weight: 700;
  cursor: pointer;
  transition:
    border-color 0.16s ease,
    background 0.16s ease,
    color 0.16s ease;
}

.roadmap-rules-board__btn:hover {
  border-color: rgb(var(--ring) / 0.32);
  background: var(--admin-header-hover-bg);
}

.roadmap-rules-board__btn--primary {
  background: rgb(var(--primary) / 0.16);
  border-color: rgb(var(--primary) / 0.4);
  color: rgb(var(--primary));
}

.roadmap-rules-board__btn--primary:hover {
  background: rgb(var(--primary) / 0.22);
}

.roadmap-rules-board__btn--accent {
  background: rgb(var(--success) / 0.14);
  border-color: rgb(var(--success) / 0.4);
  color: rgb(var(--success));
}

.roadmap-rules-board__btn--accent:hover {
  background: rgb(var(--success) / 0.22);
}

.roadmap-rules-board__filters {
  display: inline-flex;
  flex-wrap: wrap;
  gap: 0.4rem;
}

.roadmap-rules-board__filter {
  padding: 0.42rem 0.85rem;
  border: 1px solid var(--admin-header-border);
  border-radius: 999px;
  background: transparent;
  color: var(--text-muted);
  font-size: 0.8rem;
  font-weight: 700;
  cursor: pointer;
  transition:
    border-color 0.16s ease,
    color 0.16s ease,
    background 0.16s ease;
}

.roadmap-rules-board__filter:hover {
  border-color: rgb(var(--ring) / 0.32);
  color: var(--text-main);
}

.roadmap-rules-board__filter.is-active {
  border-color: rgb(var(--primary) / 0.4);
  background: rgb(var(--primary) / 0.12);
  color: rgb(var(--primary));
}

.roadmap-rules-board__group {
  display: grid;
  gap: 0.7rem;
}

.roadmap-rules-board__group-head {
  display: inline-flex;
  align-items: center;
  gap: 0.6rem;
}

.roadmap-rules-board__group-title {
  margin: 0;
  font-size: 0.85rem;
  font-weight: 800;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.roadmap-rules-board__group-count {
  padding: 0.05rem 0.5rem;
  border-radius: 999px;
  background: rgb(var(--muted) / 0.4);
  color: var(--text-muted);
  font-size: 0.72rem;
  font-weight: 700;
}

.roadmap-rules-board__grid {
  display: grid;
  gap: 0.8rem;
  grid-template-columns: repeat(auto-fill, minmax(min(320px, 100%), 1fr));
}

.roadmap-rules-board__empty {
  padding: 2rem;
  text-align: center;
  color: var(--text-muted);
  border: 1px dashed var(--admin-header-border);
  border-radius: 12px;
}
</style>
