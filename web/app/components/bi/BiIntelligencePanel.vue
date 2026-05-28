<script setup lang="ts">
import { computed } from 'vue'
import { Circle, Gem, PackageSearch, Sparkles, Watch } from 'lucide-vue-next'

import type { BiDataTable, BiIntelligenceSection } from '~/stores/bi'

const props = withDefaults(
  defineProps<{
    sections: BiIntelligenceSection[]
    itemsTable?: BiDataTable | null
    inventoryLoading?: boolean
  }>(),
  {
    itemsTable: null,
    inventoryLoading: false,
  },
)

const previewRows = computed(() => {
  const rows = Array.isArray(props.itemsTable?.rows) ? props.itemsTable.rows : []
  return rows.slice(0, 8)
})

function typeLabel(row: Record<string, unknown>) {
  return String(row.tipo || row.subtipo || 'Mix').trim() || 'Mix'
}

function iconForRow(row: Record<string, unknown>) {
  const value = `${row.tipo || ''} ${row.subtipo || ''}`.toLowerCase()
  if (value.includes('relog')) return Watch
  if (value.includes('alianc') || value.includes('anel')) return Gem
  if (value.includes('colar') || value.includes('pulse')) return Sparkles
  return PackageSearch
}

function swatchForRow(row: Record<string, unknown>) {
  const value = String(row.cor || row.material || row.marca || '').toLowerCase()
  if (value.includes('amarel') || value.includes('ouro')) return '#d5a739'
  if (value.includes('prata') || value.includes('steel') || value.includes('aco')) return '#aab4c8'
  if (value.includes('rose') || value.includes('rosa')) return '#cc8c8b'
  if (value.includes('azul')) return '#4977d6'
  if (value.includes('verde')) return '#4f9f6d'
  if (value.includes('vermel')) return '#c65b58'
  if (value.includes('preto')) return '#434a5c'
  if (value.includes('branco')) return '#e7ecf5'
  return '#7f89a7'
}

function compactValue(row: Record<string, unknown>, key: string, fallback = '-') {
  const value = String(row[key] || '').trim()
  return value || fallback
}
</script>

<template>
  <section class="bi-intelligence">
    <div class="bi-intelligence__sections">
      <article
        v-for="section in sections"
        :key="section.key"
        class="bi-intelligence__section"
        :data-tone="section.tone || 'default'"
      >
        <header class="bi-intelligence__section-head">
          <div>
            <h3>{{ section.title }}</h3>
            <p>{{ section.summary }}</p>
          </div>
          <span v-if="section.key === 'estoque' && inventoryLoading" class="bi-intelligence__pill">
            carregando
          </span>
        </header>

        <div class="bi-intelligence__items">
          <article v-for="item in section.items" :key="`${section.key}-${item.label}`" class="bi-intelligence__item">
            <span>{{ item.label }}</span>
            <strong>{{ item.value }}</strong>
            <small>{{ item.detail || ' ' }}</small>
          </article>
        </div>
      </article>
    </div>

    <aside class="bi-mix-preview">
      <header class="bi-mix-preview__head">
        <div>
          <h3>Preview do mix</h3>
          <p>
            A API atual nao traz foto real do produto. Aqui usamos tipo, marca, colecao,
            material e cor para uma leitura visual rapida.
          </p>
        </div>
      </header>

      <div class="bi-mix-preview__grid">
        <article v-for="(row, index) in previewRows" :key="row.__rowId || index" class="bi-mix-card">
          <div class="bi-mix-card__visual" :style="{ '--swatch': swatchForRow(row) }">
            <component :is="iconForRow(row)" :size="26" aria-hidden="true" />
            <span class="bi-mix-card__swatch">
              <Circle :size="10" :fill="swatchForRow(row)" :color="swatchForRow(row)" aria-hidden="true" />
              {{ compactValue(row, 'cor', compactValue(row, 'material')) }}
            </span>
          </div>

          <div class="bi-mix-card__body">
            <strong>{{ typeLabel(row) }}</strong>
            <span>{{ compactValue(row, 'marca') }}</span>
            <small>
              {{ compactValue(row, 'colecao', compactValue(row, 'referencia')) }}
            </small>
          </div>
        </article>

        <article v-if="!previewRows.length" class="bi-mix-card bi-mix-card--empty">
          <div class="bi-mix-card__visual">
            <PackageSearch :size="26" aria-hidden="true" />
          </div>
          <div class="bi-mix-card__body">
            <strong>Aguardando itens</strong>
            <small>Assim que a Perola responder com o cadastro, a vitrine do mix aparece aqui.</small>
          </div>
        </article>
      </div>
    </aside>
  </section>
</template>

<style scoped>
.bi-intelligence {
  display: grid;
  grid-template-columns: minmax(0, 1.5fr) minmax(19rem, 0.9fr);
  gap: 1rem;
}

.bi-intelligence__sections,
.bi-intelligence__items,
.bi-mix-preview,
.bi-mix-preview__grid {
  display: grid;
  gap: 0.8rem;
}

.bi-intelligence__section,
.bi-mix-preview,
.bi-mix-card {
  border: 1px solid var(--line-soft);
  border-radius: 0.95rem;
  background: rgba(13, 18, 29, 0.9);
  box-shadow: var(--shadow-card);
}

.bi-intelligence__section {
  padding: 0.95rem;
}

.bi-intelligence__section[data-tone='success'] {
  border-color: rgba(83, 198, 160, 0.28);
}

.bi-intelligence__section[data-tone='warning'] {
  border-color: rgba(255, 184, 108, 0.22);
}

.bi-intelligence__section-head {
  display: flex;
  justify-content: space-between;
  gap: 0.8rem;
  margin-bottom: 0.85rem;
}

.bi-intelligence__section-head h3,
.bi-mix-preview__head h3 {
  margin: 0;
  color: var(--text-main);
  font-size: 1rem;
}

.bi-intelligence__section-head p,
.bi-mix-preview__head p,
.bi-intelligence__item span,
.bi-intelligence__item small,
.bi-mix-card__body span,
.bi-mix-card__body small {
  color: var(--text-muted);
}

.bi-intelligence__section-head p,
.bi-mix-preview__head p {
  margin: 0.28rem 0 0;
  line-height: 1.5;
}

.bi-intelligence__items {
  grid-template-columns: repeat(auto-fit, minmax(10.5rem, 1fr));
}

.bi-intelligence__item {
  display: grid;
  gap: 0.2rem;
  padding: 0.85rem;
  border-radius: 0.85rem;
  background: rgba(17, 24, 39, 0.88);
}

.bi-intelligence__item span {
  font-size: 0.72rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.bi-intelligence__item strong {
  color: var(--text-main);
  font-size: 1rem;
}

.bi-intelligence__pill {
  display: inline-flex;
  align-items: center;
  min-height: 1.8rem;
  padding: 0 0.6rem;
  border-radius: 999px;
  border: 1px solid rgba(129, 140, 248, 0.25);
  color: #c8d2ff;
  font-size: 0.74rem;
  font-weight: 700;
}

.bi-mix-preview {
  padding: 0.95rem;
  align-content: start;
}

.bi-mix-preview__grid {
  grid-template-columns: repeat(auto-fit, minmax(11.5rem, 1fr));
}

.bi-mix-card {
  overflow: hidden;
}

.bi-mix-card__visual {
  display: grid;
  gap: 0.55rem;
  justify-items: start;
  min-height: 6.5rem;
  padding: 1rem;
  background:
    radial-gradient(circle at top right, color-mix(in srgb, var(--swatch) 55%, transparent), transparent 58%),
    linear-gradient(145deg, rgba(22, 29, 44, 0.98), rgba(12, 17, 27, 0.95));
  color: #f6f7fb;
}

.bi-mix-card__swatch {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  min-height: 1.85rem;
  padding: 0 0.55rem;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.08);
  font-size: 0.72rem;
  font-weight: 700;
}

.bi-mix-card__body {
  display: grid;
  gap: 0.18rem;
  padding: 0.85rem 0.9rem 0.95rem;
}

.bi-mix-card__body strong {
  color: var(--text-main);
}

.bi-mix-card--empty {
  border-style: dashed;
}

@media (max-width: 1080px) {
  .bi-intelligence {
    grid-template-columns: 1fr;
  }
}
</style>
