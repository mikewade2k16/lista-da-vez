<script setup lang="ts">
import { computed } from 'vue'

import BioCollapsibleItem from '~/components/bio/BioCollapsibleItem.vue'
import BioLinkListEditor from '~/components/bio/BioLinkListEditor.vue'
import BioSectionCard from '~/components/bio/BioSectionCard.vue'
import type { BioData, BioLayout, BioLink, BioMenuItem } from '~/domain/bio/types'

// Secao links: links[] (botoes da pagina) + headerMenu[] (menu do topo) + o
// toggle "Header mobile ativo" (layout.headerMobileActive — vive aqui, nao na
// secao de fundo). Os dois listas usam o BioLinkListEditor reutilizavel — a
// ORDEM DO ARRAY define a ordem de exibicao (botoes subir/descer).

const props = defineProps<{ draft: BioData }>()
const emit = defineEmits<{ (e: 'update:draft', value: BioData): void }>()

type ListItem = Record<string, unknown>

const LINK_FIELDS = [
  { key: 'label', label: 'Texto', placeholder: 'Texto do link' },
  { key: 'href', label: 'Link (href)', placeholder: 'https://...' },
  { key: 'action', label: 'Acao', placeholder: 'openStoreLocator (opcional)' },
]

const MENU_FIELDS = [
  { key: 'label', label: 'Texto', placeholder: 'Texto do item' },
  { key: 'href', label: 'Link (href)', placeholder: 'https://...' },
  { key: 'action', label: 'Acao', placeholder: 'openStoreLocator (opcional)' },
]

const layout = computed<BioLayout>(() => props.draft.layout || {})
const links = computed<ListItem[]>(() => (props.draft.links || []) as unknown as ListItem[])
const headerMenu = computed<ListItem[]>(
  () => (props.draft.headerMenu || []) as unknown as ListItem[],
)

function countSummary(total: number, singular: string, plural: string): string {
  if (!total) {
    return `nenhum ${singular}`
  }
  return `${total} ${total === 1 ? singular : plural}`
}

const menuSummary = computed(() => countSummary(headerMenu.value.length, 'item', 'itens'))
const linksSummary = computed(() => countSummary(links.value.length, 'link', 'links'))

function setHeaderMobileActive(value: boolean) {
  emit('update:draft', {
    ...props.draft,
    layout: { ...(props.draft.layout || {}), headerMobileActive: value },
  })
}

function updateLinks(next: ListItem[]) {
  emit('update:draft', { ...props.draft, links: next as unknown as BioLink[] })
}

function updateMenu(next: ListItem[]) {
  emit('update:draft', { ...props.draft, headerMenu: next as unknown as BioMenuItem[] })
}
</script>

<template>
  <BioSectionCard
    title="Links e menu"
    description="Botoes de link da pagina e itens do menu do topo. A ordem da lista e a ordem de exibicao."
  >
    <BioCollapsibleItem title="Menu do topo (header)" :summary="menuSummary">
      <template #actions>
        <div class="bio-field bio-field--switch" @click.stop>
          <label class="bio-field__label">Header mobile</label>
          <USwitch
            :model-value="layout.headerMobileActive ?? false"
            @update:model-value="setHeaderMobileActive(Boolean($event))"
          />
        </div>
      </template>
      <BioLinkListEditor
        :model-value="headerMenu"
        :fields="MENU_FIELDS"
        item-label="item de menu"
        add-label="Adicionar item de menu"
        empty-hint="Nenhum item de menu ainda."
        @update:model-value="updateMenu"
      />
    </BioCollapsibleItem>

    <BioCollapsibleItem title="Links" :summary="linksSummary">
      <BioLinkListEditor
        :model-value="links"
        :fields="LINK_FIELDS"
        item-label="link"
        add-label="Adicionar link"
        empty-hint="Nenhum link ainda. Os links aparecem como botoes na bio."
        @update:model-value="updateLinks"
      />
    </BioCollapsibleItem>
  </BioSectionCard>
</template>

<style scoped>
.bio-field {
  display: grid;
  gap: 0.3rem;
}

.bio-field--switch {
  display: flex;
  align-items: center;
  gap: 0.4rem;
}

.bio-field__label {
  font-size: 0.72rem;
  font-weight: 700;
  color: var(--text-muted);
  white-space: nowrap;
}
</style>
