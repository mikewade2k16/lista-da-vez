<script setup lang="ts">
import { ref } from 'vue'
import { ChevronDown, GripVertical } from 'lucide-vue-next'
import type { MenuPlacement } from '~/stores/menuLayout'
import type { MenuLayoutEditorItem, MenuLayoutEditorSection } from './useMenuLayoutEditor'
import MenuLayoutItemRow from './MenuLayoutItemRow.vue'

const props = defineProps<{
  sections: MenuLayoutEditorSection[]
  placementResolver: (id: string) => MenuPlacement
}>()

const emit = defineEmits<{
  'set-placement': [id: string, placement: MenuPlacement]
  'reorder-items': [scopeId: string, sourceId: string, targetId: string]
  'reorder-sections': [sourceId: string, targetId: string]
}>()

const collapsed = ref<Record<string, boolean>>({})
// Escopo (secao/grupo) do drag de item em andamento; restringe o drop ao mesmo
// escopo, evitando mover item entre secoes diferentes.
const dragScopeId = ref('')
const dragItemId = ref('')
const dragSectionId = ref('')

function isCollapsed(sectionId: string) {
  return Boolean(collapsed.value[sectionId])
}

function toggleSection(sectionId: string) {
  collapsed.value = { ...collapsed.value, [sectionId]: !collapsed.value[sectionId] }
}

function onItemDragStart(scopeId: string, itemId: string) {
  dragScopeId.value = scopeId
  dragItemId.value = itemId
}

function onItemDragOver(event: DragEvent, scopeId: string) {
  if (!dragItemId.value || dragScopeId.value !== scopeId) return
  event.preventDefault()
  if (event.dataTransfer) event.dataTransfer.dropEffect = 'move'
}

function onItemDrop(scopeId: string, targetId: string) {
  if (!dragItemId.value || dragScopeId.value !== scopeId) return
  emit('reorder-items', scopeId, dragItemId.value, targetId)
  dragItemId.value = ''
  dragScopeId.value = ''
}

function onItemDragEnd() {
  dragItemId.value = ''
  dragScopeId.value = ''
}

function onSectionDragStart(event: DragEvent, sectionId: string) {
  dragSectionId.value = sectionId
  if (event.dataTransfer) event.dataTransfer.effectAllowed = 'move'
}

function onSectionDragOver(event: DragEvent) {
  if (!dragSectionId.value) return
  event.preventDefault()
  if (event.dataTransfer) event.dataTransfer.dropEffect = 'move'
}

function onSectionDrop(targetSectionId: string) {
  if (!dragSectionId.value || dragSectionId.value === targetSectionId) return
  emit('reorder-sections', dragSectionId.value, targetSectionId)
  dragSectionId.value = ''
}

function onSectionDragEnd() {
  dragSectionId.value = ''
}

function placementOf(item: MenuLayoutEditorItem) {
  return props.placementResolver(item.id)
}
</script>

<template>
  <div class="menu-layout-editor">
    <section
      v-for="section in props.sections"
      :key="section.id"
      class="menu-layout-editor__section"
      :class="{ 'menu-layout-editor__section--dragging': dragSectionId === section.id }"
      @dragover="onSectionDragOver($event)"
      @drop="onSectionDrop(section.id)"
    >
      <header class="menu-layout-editor__section-head">
        <span
          class="menu-layout-editor__section-handle"
          draggable="true"
          aria-label="Reordenar secao"
          @dragstart="onSectionDragStart($event, section.id)"
          @dragend="onSectionDragEnd()"
        >
          <GripVertical :size="16" :stroke-width="2.1" aria-hidden="true" />
        </span>
        <button
          class="menu-layout-editor__section-toggle"
          type="button"
          :aria-expanded="isCollapsed(section.id) ? 'false' : 'true'"
          @click="toggleSection(section.id)"
        >
          <span class="menu-layout-editor__section-label">{{ section.label }}</span>
          <ChevronDown
            class="menu-layout-editor__section-chevron"
            :class="{ 'is-collapsed': isCollapsed(section.id) }"
            :size="16"
            :stroke-width="2.2"
            aria-hidden="true"
          />
        </button>
      </header>

      <div v-if="!isCollapsed(section.id)" class="menu-layout-editor__items">
        <template v-for="item in section.items" :key="item.id">
          <MenuLayoutItemRow
            :item="item"
            :placement="placementOf(item)"
            :scope-id="section.id"
            :dragging="dragScopeId === section.id && dragItemId === item.id"
            @update:placement="emit('set-placement', item.id, $event)"
            @dragstart="onItemDragStart(section.id, item.id)"
            @dragover="onItemDragOver($event, section.id)"
            @drop="onItemDrop(section.id, item.id)"
            @dragend="onItemDragEnd()"
          />

          <div v-if="item.children.length" class="menu-layout-editor__children">
            <MenuLayoutItemRow
              v-for="child in item.children"
              :key="child.id"
              :item="child"
              :placement="placementOf(child)"
              :scope-id="item.id"
              :dragging="dragScopeId === item.id && dragItemId === child.id"
              @update:placement="emit('set-placement', child.id, $event)"
              @dragstart="onItemDragStart(item.id, child.id)"
              @dragover="onItemDragOver($event, item.id)"
              @drop="onItemDrop(item.id, child.id)"
              @dragend="onItemDragEnd()"
            />
          </div>
        </template>
      </div>
    </section>
  </div>
</template>

<style scoped>
.menu-layout-editor {
  display: grid;
  gap: 0.85rem;
}

.menu-layout-editor__section {
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-md);
  background: var(--surface);
  box-shadow: var(--shadow-card);
}

.menu-layout-editor__section--dragging {
  border-color: rgb(var(--primary) / 0.4);
}

.menu-layout-editor__section-head {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  padding: 0.55rem 0.7rem;
  border-bottom: 1px solid var(--line-soft);
}

.menu-layout-editor__section-handle {
  display: inline-grid;
  place-items: center;
  flex-shrink: 0;
  color: var(--text-muted);
  cursor: grab;
}

.menu-layout-editor__section-toggle {
  appearance: none;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  flex: 1;
  border: 0;
  background: transparent;
  color: var(--text-main);
  cursor: pointer;
}

.menu-layout-editor__section-label {
  font-size: 0.78rem;
  font-weight: 800;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.menu-layout-editor__section-chevron {
  flex-shrink: 0;
  color: var(--text-muted);
  transition: transform 0.16s ease;
}

.menu-layout-editor__section-chevron.is-collapsed {
  transform: rotate(-90deg);
}

.menu-layout-editor__items {
  display: grid;
  gap: 0.4rem;
  padding: 0.6rem 0.7rem;
}

.menu-layout-editor__children {
  display: grid;
  gap: 0.4rem;
  margin-left: 1.1rem;
  padding-left: 0.7rem;
  border-left: 1px solid var(--line-soft);
}
</style>
