<script setup lang="ts">
import SegmentConditionGroup from '~/components/customer-intelligence/segments/SegmentConditionGroup.vue'
import type { SegmentFieldCatalog, SegmentFilterAst } from '~/domain/customer-data/segment-types'

const props = defineProps<{
  ast: SegmentFilterAst
  catalog: SegmentFieldCatalog
  editable: boolean
  dirty: boolean
  busy: boolean
}>()

const emit = defineEmits<{
  update: [ast: SegmentFilterAst]
  save: []
  discard: []
}>()

const predicateCount = computed(() => {
  function count(node: SegmentFilterAst['root']): number {
    return node.children.reduce(
      (total, child) => total + (child.kind === 'group' ? count(child) : 1),
      0,
    )
  }
  return count(props.ast.root)
})
</script>

<template>
  <section class="segment-panel">
    <header>
      <div>
        <h3>Builder deterministico</h3>
        <p>
          Campos e operadores sao definidos pelo backend. Nao ha SQL, JSON, regex ou prompt livre.
        </p>
      </div>
      <small>
        {{ predicateCount }}/{{ catalog.caps.maxPredicates }} condicoes · catalogo
        {{ catalog.version }}
      </small>
    </header>
    <SegmentConditionGroup
      :group="ast.root"
      :catalog="catalog"
      :editable="editable && predicateCount < catalog.caps.maxPredicates"
      @update="emit('update', { ...ast, root: $event })"
    />
    <footer v-if="editable">
      <span v-if="dirty">Alteracoes ainda nao salvas</span>
      <button type="button" :disabled="!dirty || busy" @click="emit('discard')">Descartar</button>
      <button type="button" :disabled="!dirty || busy" @click="emit('save')">Salvar draft</button>
    </footer>
  </section>
</template>

<style scoped>
.segment-panel,
.segment-panel header {
  display: grid;
  gap: 0.75rem;
}

.segment-panel {
  padding: 1rem;
  border: 1px solid rgb(var(--border) / 0.8);
  border-radius: 0.9rem;
}

.segment-panel header {
  grid-template-columns: 1fr auto;
}

.segment-panel h3,
.segment-panel p {
  margin: 0;
}

.segment-panel p,
.segment-panel small,
.segment-panel footer span {
  color: rgb(var(--muted));
  font-size: 0.72rem;
}

.segment-panel footer {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 0.5rem;
}
</style>
