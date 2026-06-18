<script setup lang="ts">
import { computed } from 'vue'
import type { MenuLayout, MenuPlacement } from '~/stores/menuLayout'
import type { MenuLayoutEditorItem, MenuLayoutEditorSection } from './useMenuLayoutEditor'

const props = defineProps<{
  sections: MenuLayoutEditorSection[]
  layout: MenuLayout
}>()

interface PreviewNode {
  id: string
  label: string
  isGroup: boolean
}

function placementOf(id: string): MenuPlacement {
  return props.layout.items[id]?.placement || 'both'
}

function childInHeader(item: MenuLayoutEditorItem) {
  const placement = placementOf(item.id)
  return placement === 'header' || placement === 'both'
}

function childInSidebar(item: MenuLayoutEditorItem) {
  const placement = placementOf(item.id)
  return placement === 'sidebar' || placement === 'both'
}

// Itens top-level que aparecem no header: placement header/both. Grupos entram
// se forem header/both E tiverem ao menos um filho elegivel.
const headerNodes = computed<PreviewNode[]>(() => {
  const nodes: PreviewNode[] = []
  for (const section of props.sections) {
    for (const item of section.items) {
      const placement = placementOf(item.id)
      if (placement !== 'header' && placement !== 'both') continue
      if (item.children.length) {
        if (item.children.some(childInHeader)) {
          nodes.push({ id: item.id, label: item.label, isGroup: true })
        }
        continue
      }
      nodes.push({ id: item.id, label: item.label, isGroup: false })
    }
  }
  return nodes
})

// Secoes da sidebar: itens com placement sidebar/both; grupos com ao menos um
// filho elegivel.
const sidebarSections = computed(() =>
  props.sections
    .map((section) => {
      const items: PreviewNode[] = []
      for (const item of section.items) {
        if (item.children.length) {
          if (item.children.some(childInSidebar)) {
            items.push({ id: item.id, label: item.label, isGroup: true })
          }
          continue
        }
        if (childInSidebar(item)) {
          items.push({ id: item.id, label: item.label, isGroup: false })
        }
      }
      return { id: section.id, label: section.label, items }
    })
    .filter((section) => section.items.length > 0),
)
</script>

<template>
  <div class="menu-layout-preview">
    <div class="menu-layout-preview__block">
      <span class="menu-layout-preview__caption">Header</span>
      <div class="menu-layout-preview__header">
        <span v-if="!headerNodes.length" class="menu-layout-preview__empty">
          Nenhum item no header.
        </span>
        <span
          v-for="node in headerNodes"
          :key="node.id"
          class="menu-layout-preview__pill"
          :class="{ 'menu-layout-preview__pill--group': node.isGroup }"
        >
          {{ node.label }}
        </span>
      </div>
    </div>

    <div class="menu-layout-preview__block">
      <span class="menu-layout-preview__caption">Sidebar</span>
      <div class="menu-layout-preview__sidebar">
        <span v-if="!sidebarSections.length" class="menu-layout-preview__empty">
          Nenhum item na sidebar.
        </span>
        <div
          v-for="section in sidebarSections"
          :key="section.id"
          class="menu-layout-preview__sidebar-group"
        >
          <span class="menu-layout-preview__sidebar-label">{{ section.label }}</span>
          <span
            v-for="node in section.items"
            :key="node.id"
            class="menu-layout-preview__sidebar-item"
            :class="{ 'menu-layout-preview__sidebar-item--group': node.isGroup }"
          >
            {{ node.label }}
          </span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.menu-layout-preview {
  display: grid;
  gap: 1rem;
}

.menu-layout-preview__block {
  display: grid;
  gap: 0.45rem;
}

.menu-layout-preview__caption {
  color: var(--text-muted);
  font-size: 0.64rem;
  font-weight: 800;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.menu-layout-preview__header {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
  padding: 0.6rem 0.7rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-md);
  background: var(--bg-muted);
}

.menu-layout-preview__pill {
  padding: 0.25rem 0.6rem;
  border-radius: 999px;
  border: 1px solid var(--line-soft);
  background: var(--surface);
  color: var(--text-main);
  font-size: 0.76rem;
  font-weight: 750;
}

.menu-layout-preview__pill--group {
  border-color: rgb(var(--primary) / 0.4);
  color: rgb(var(--primary));
}

.menu-layout-preview__sidebar {
  display: grid;
  gap: 0.7rem;
  padding: 0.6rem 0.7rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-md);
  background: var(--bg-muted);
}

.menu-layout-preview__sidebar-group {
  display: grid;
  gap: 0.25rem;
}

.menu-layout-preview__sidebar-label {
  color: var(--text-muted);
  font-size: 0.62rem;
  font-weight: 800;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.menu-layout-preview__sidebar-item {
  padding: 0.3rem 0.55rem;
  border-radius: var(--radius-sm);
  border: 1px solid transparent;
  background: var(--surface);
  color: var(--text-main);
  font-size: 0.78rem;
  font-weight: 700;
}

.menu-layout-preview__sidebar-item--group {
  border-color: rgb(var(--primary) / 0.4);
  color: rgb(var(--primary));
}

.menu-layout-preview__empty {
  color: var(--text-muted);
  font-size: 0.78rem;
  font-style: italic;
}
</style>
