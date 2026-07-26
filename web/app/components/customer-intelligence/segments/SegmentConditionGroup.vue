<script setup lang="ts">
import type {
  SegmentFieldCatalog,
  SegmentFieldDescriptor,
  SegmentFilterNode,
  SegmentGroupNode,
  SegmentPredicateNode,
  SegmentValue,
} from '~/domain/customer-data/segment-types'

const props = withDefaults(
  defineProps<{
    group: SegmentGroupNode
    catalog: SegmentFieldCatalog
    editable: boolean
    depth?: number
  }>(),
  { depth: 0 },
)

const emit = defineEmits<{
  update: [group: SegmentGroupNode]
  remove: []
}>()

function nodeId(): string {
  return globalThis.crypto?.randomUUID?.() ?? `node-${Date.now()}`
}

function field(fieldKey: string): SegmentFieldDescriptor | undefined {
  return props.catalog.fields.find((item) => item.fieldKey === fieldKey)
}

function cloneGroup(children = props.group.children): SegmentGroupNode {
  return { ...props.group, children: structuredClone(children) }
}

function updateCombinator(value: string): void {
  if (value !== 'and' && value !== 'or') return
  emit('update', { ...cloneGroup(), combinator: value })
}

function updateChild(index: number, child: SegmentFilterNode): void {
  const children = [...props.group.children]
  children[index] = child
  emit('update', cloneGroup(children))
}

function removeChild(index: number): void {
  emit('update', cloneGroup(props.group.children.filter((_, childIndex) => childIndex !== index)))
}

function addPredicate(): void {
  const descriptor = props.catalog.fields.find((item) => item.availability !== 'unavailable')
  if (!descriptor) return
  const predicate: SegmentPredicateNode = {
    kind: 'predicate',
    nodeId: nodeId(),
    fieldKey: descriptor.fieldKey,
    operator: descriptor.operators[0] ?? 'equals',
    value: descriptor.valueType === 'boolean' ? false : '',
  }
  emit('update', cloneGroup([...props.group.children, predicate]))
}

function addGroup(): void {
  if (props.depth + 1 >= props.catalog.caps.maxDepth) return
  const group: SegmentGroupNode = {
    kind: 'group',
    nodeId: nodeId(),
    combinator: 'and',
    children: [],
  }
  emit('update', cloneGroup([...props.group.children, group]))
}

function changeField(predicate: SegmentPredicateNode, fieldKey: string): SegmentPredicateNode {
  const descriptor = field(fieldKey)
  return {
    ...predicate,
    fieldKey,
    operator: descriptor?.operators[0] ?? '',
    value: descriptor?.valueType === 'boolean' ? false : '',
  }
}

function changeOperator(predicate: SegmentPredicateNode, operator: string): SegmentPredicateNode {
  return { ...predicate, operator }
}

function changeValue(
  predicate: SegmentPredicateNode,
  rawValue: string | boolean,
): SegmentPredicateNode {
  const descriptor = field(predicate.fieldKey)
  let value: SegmentValue = rawValue
  if (descriptor?.valueType === 'number') value = Number(rawValue)
  if (descriptor?.valueType === 'boolean') value = rawValue === true || rawValue === 'true'
  return { ...predicate, value }
}
</script>

<template>
  <fieldset class="condition-group">
    <legend>
      Grupo
      <select
        :value="group.combinator"
        :disabled="!editable"
        aria-label="Combinador do grupo"
        @change="updateCombinator(($event.target as HTMLSelectElement).value)"
      >
        <option value="and">TODAS (E)</option>
        <option value="or">QUALQUER (OU)</option>
      </select>
      <button v-if="depth > 0 && editable" type="button" @click="emit('remove')">
        Remover grupo
      </button>
    </legend>

    <div
      v-for="(child, index) in group.children"
      :key="child.nodeId"
      class="condition-group__child"
    >
      <SegmentConditionGroup
        v-if="child.kind === 'group'"
        :group="child"
        :catalog="catalog"
        :editable="editable"
        :depth="depth + 1"
        @update="updateChild(index, $event)"
        @remove="removeChild(index)"
      />
      <div v-else class="predicate">
        <select
          :value="child.fieldKey"
          :disabled="!editable"
          aria-label="Campo"
          @change="
            updateChild(index, changeField(child, ($event.target as HTMLSelectElement).value))
          "
        >
          <option
            v-for="descriptor in catalog.fields"
            :key="descriptor.fieldKey"
            :value="descriptor.fieldKey"
            :disabled="descriptor.availability === 'unavailable'"
          >
            {{ descriptor.label }}
          </option>
        </select>
        <select
          :value="child.operator"
          :disabled="!editable"
          aria-label="Operador"
          @change="
            updateChild(index, changeOperator(child, ($event.target as HTMLSelectElement).value))
          "
        >
          <option
            v-for="operator in field(child.fieldKey)?.operators ?? []"
            :key="operator"
            :value="operator"
          >
            {{ operator }}
          </option>
        </select>
        <select
          v-if="field(child.fieldKey)?.valueType === 'enum'"
          :value="String(child.value ?? '')"
          :disabled="!editable"
          aria-label="Valor"
          @change="
            updateChild(index, changeValue(child, ($event.target as HTMLSelectElement).value))
          "
        >
          <option
            v-for="option in field(child.fieldKey)?.options ?? []"
            :key="option.value"
            :value="option.value"
          >
            {{ option.label }}
          </option>
        </select>
        <select
          v-else-if="field(child.fieldKey)?.valueType === 'boolean'"
          :value="String(child.value ?? false)"
          :disabled="!editable"
          aria-label="Valor"
          @change="
            updateChild(index, changeValue(child, ($event.target as HTMLSelectElement).value))
          "
        >
          <option value="true">Sim</option>
          <option value="false">Nao</option>
        </select>
        <input
          v-else
          :type="
            field(child.fieldKey)?.valueType === 'number'
              ? 'number'
              : field(child.fieldKey)?.valueType === 'date'
                ? 'date'
                : 'text'
          "
          :value="String(child.value ?? '')"
          :maxlength="field(child.fieldKey)?.maxLength"
          :disabled="!editable"
          aria-label="Valor"
          @input="updateChild(index, changeValue(child, ($event.target as HTMLInputElement).value))"
        />
        <button v-if="editable" type="button" @click="removeChild(index)">Remover</button>
      </div>
    </div>

    <div v-if="editable" class="condition-group__actions">
      <button type="button" @click="addPredicate">Adicionar condicao</button>
      <button type="button" :disabled="depth + 1 >= catalog.caps.maxDepth" @click="addGroup">
        Adicionar grupo
      </button>
    </div>
  </fieldset>
</template>

<style scoped>
.condition-group {
  display: grid;
  gap: 0.65rem;
  padding: 0.75rem;
  border: 1px solid rgb(var(--border) / 0.85);
  border-radius: 0.75rem;
}

.condition-group legend,
.condition-group__actions,
.predicate {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.condition-group__child {
  min-width: 0;
}

.predicate select,
.predicate input {
  min-height: 2.2rem;
  min-width: 9rem;
  border: 1px solid rgb(var(--border));
  border-radius: 0.55rem;
  background: rgb(var(--surface));
  color: inherit;
}

button {
  cursor: pointer;
}
</style>
