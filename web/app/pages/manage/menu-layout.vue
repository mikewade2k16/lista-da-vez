<script setup lang="ts">
import AdminPageHeader from '../../../layers/core/components/admin/AdminPageHeader.vue'
import MenuLayoutEditor from '~/components/menu-layout/MenuLayoutEditor.vue'
import MenuLayoutPreview from '~/components/menu-layout/MenuLayoutPreview.vue'
import { useMenuLayoutEditor } from '~/components/menu-layout/useMenuLayoutEditor'

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'menu_layout',
})

const {
  sections,
  orderedSections,
  draftLayout,
  saving,
  placementFor,
  setPlacement,
  reorderItems,
  reorderSections,
  suggestLeanLayout,
  save,
} = useMenuLayoutEditor()

async function handleSave() {
  await save()
}
</script>

<template>
  <div class="menu-layout-page">
    <AdminPageHeader
      eyebrow="Plataforma"
      title="Config do menu"
      description="Defina a posicao de cada item entre header e sidebar. A configuracao e global e vale para todos os usuarios."
    />

    <div class="menu-layout-page__actions">
      <UButton
        to="/manage/experimental-features"
        icon="i-lucide-flask-conical"
        color="neutral"
        variant="soft"
        label="Recursos experimentais"
      />
      <UButton
        icon="i-lucide-wand-2"
        color="neutral"
        variant="soft"
        label="Sugerir layout enxuto"
        @click="suggestLeanLayout"
      />
      <UButton
        icon="i-lucide-save"
        color="primary"
        :loading="saving"
        label="Salvar"
        @click="handleSave"
      />
    </div>

    <div class="menu-layout-page__grid">
      <div class="menu-layout-page__editor">
        <MenuLayoutEditor
          :sections="orderedSections"
          :placement-resolver="placementFor"
          @set-placement="setPlacement"
          @reorder-items="reorderItems"
          @reorder-sections="reorderSections"
        />
      </div>

      <aside class="menu-layout-page__preview">
        <MenuLayoutPreview :sections="sections" :layout="draftLayout" />
      </aside>
    </div>
  </div>
</template>

<style scoped>
.menu-layout-page {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  display: grid;
  gap: 1.1rem;
  padding: 1rem 1.2rem 2rem;
  align-content: start;
}

.menu-layout-page__actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 0.6rem;
}

.menu-layout-page__grid {
  display: grid;
  grid-template-columns: minmax(0, 1.5fr) minmax(0, 1fr);
  gap: 1.1rem;
  align-items: start;
}

.menu-layout-page__preview {
  position: sticky;
  top: 0.5rem;
  padding: 0.9rem 1rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-md);
  background: var(--surface);
  box-shadow: var(--shadow-card);
}

@media (max-width: 980px) {
  .menu-layout-page__grid {
    grid-template-columns: minmax(0, 1fr);
  }

  .menu-layout-page__preview {
    position: static;
  }
}
</style>
