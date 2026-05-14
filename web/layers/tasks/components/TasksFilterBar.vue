<script setup lang="ts">
import { inject } from 'vue'
import { TASKS_PAGE_CONTEXT_KEY } from '../composables/useTasksPageContext'
import OmniSelectMenuInput from './inputs/OmniSelectMenuInput.vue'

const ctx = inject(TASKS_PAGE_CONTEXT_KEY)!
const {
  projectModel,
  projectOptions,
  onCreateProject,
  activeProject,
  activeFilterChips,
  filters,
  searchOpen,
  closeSearch,
  toggleSearch,
  responsibleOpen,
  responsibleInnerOpen,
  responsibleOptions,
  clientOpen,
  clientInnerOpen,
  clientOptions,
  typeOpen,
  typeInnerOpen,
  typeOptions,
  hasAnyActiveFilter,
  clearFilters,
  viewerUserType,
  projectTasks,
  filteredTasks,
  viewMode,
  boardGroupBy,
  boardView,
  fieldLabel,
  createColumn,
  projectSettingsOpen,
  showAllColumns,
  createTableTask,
  beginCreateTaskInFirstColumn,
} = ctx
</script>

<template>
  <div class="tasks-toolbar">
    <div class="tasks-toolbar__project">
      <OmniSelectMenuInput v-model="projectModel" :items="projectOptions" placeholder="Selecionar pagina"
        :creatable="{ when: 'always', position: 'bottom' }" :searchable="true" :full-content-width="true"
        item-display-mode="text" color="neutral" variant="outline" :highlight="false" :badge-mode="false"
        option-edit-mode="none" @create="onCreateProject" />
    </div>

    <div v-if="activeProject && activeFilterChips.length" class="tasks-toolbar__chips">
      <span v-for="chip in activeFilterChips" :key="chip.key" class="tasks-toolbar__chip"
        :title="`${chip.label}: ${chip.value}`">
        <span class="tasks-toolbar__chip-key">{{ chip.label }}</span>
        <span class="tasks-toolbar__chip-value">{{ chip.value }}</span>
        <button type="button" class="tasks-toolbar__chip-x" :aria-label="`Remover filtro ${chip.label}`"
          @click="chip.onRemove">
          <UIcon name="i-lucide-x" />
        </button>
      </span>
    </div>

    <span class="tasks-toolbar__spacer"></span>

    <div v-if="activeProject" class="tasks-toolbar__filters">
      <div v-if="activeProject.filters.search" class="tasks-toolbar__search-group">
        <div class="tasks-toolbar__search" :class="{ 'is-open': searchOpen || !!filters.search }">
          <UInput v-model="filters.search" placeholder="Buscar por titulo, descricao e tags..." size="sm"
            @keydown.esc="closeSearch" @blur="closeSearch" />
        </div>
        <UButton class="tasks-toolbar__filter-btn" icon="i-lucide-search"
          :color="filters.search || searchOpen ? 'primary' : 'neutral'"
          :variant="filters.search || searchOpen ? 'soft' : 'ghost'" size="sm" title="Buscar"
          :data-active="!!filters.search" @click="toggleSearch" />
      </div>

      <UPopover v-if="activeProject.filters.responsible" v-model:open="responsibleOpen"
        :content="{ side: 'bottom', align: 'end' }">
        <UButton class="tasks-toolbar__filter-btn" icon="i-lucide-user"
          :color="filters.responsible ? 'primary' : 'neutral'" :variant="filters.responsible ? 'soft' : 'ghost'"
          size="sm" title="Responsavel" :data-active="!!filters.responsible" />
        <template #content>
          <div class="tasks-toolbar__filter-popover">
            <OmniSelectMenuInput v-model:open="responsibleInnerOpen" v-model="filters.responsible"
              :items="responsibleOptions" placeholder="Responsavel" :searchable="true" :full-content-width="true"
              item-display-mode="text" color="neutral" variant="none" :highlight="false" :badge-mode="true" clear
              option-edit-mode="color" />
          </div>
        </template>
      </UPopover>

      <UPopover v-if="viewerUserType === 'admin' && activeProject.filters.client" v-model:open="clientOpen"
        :content="{ side: 'bottom', align: 'end' }">
        <UButton class="tasks-toolbar__filter-btn" icon="i-lucide-building-2"
          :color="filters.clientId ? 'primary' : 'neutral'" :variant="filters.clientId ? 'soft' : 'ghost'" size="sm"
          title="Cliente" :data-active="!!filters.clientId" />
        <template #content>
          <div class="tasks-toolbar__filter-popover">
            <OmniSelectMenuInput v-model:open="clientInnerOpen" v-model="filters.clientId" :items="clientOptions"
              placeholder="Cliente" :searchable="true" :full-content-width="true" item-display-mode="text"
              color="neutral" variant="none" :highlight="false" :badge-mode="true" clear option-edit-mode="color" />
          </div>
        </template>
      </UPopover>

      <UPopover v-if="activeProject.filters.type" v-model:open="typeOpen"
        :content="{ side: 'bottom', align: 'end' }">
        <UButton class="tasks-toolbar__filter-btn" icon="i-lucide-tag" :color="filters.type ? 'primary' : 'neutral'"
          :variant="filters.type ? 'soft' : 'ghost'" size="sm" title="Tipo" :data-active="!!filters.type" />
        <template #content>
          <div class="tasks-toolbar__filter-popover">
            <OmniSelectMenuInput v-model:open="typeInnerOpen" v-model="filters.type" :items="typeOptions"
              placeholder="Tipo" :searchable="true" :full-content-width="true" item-display-mode="text"
              color="neutral" variant="none" :highlight="false" :badge-mode="true" clear option-edit-mode="color" />
          </div>
        </template>
      </UPopover>

      <UButton v-if="activeProject.filters.hideArchived" class="tasks-toolbar__filter-btn" icon="i-lucide-archive"
        :color="filters.hideArchived ? 'primary' : 'neutral'" :variant="filters.hideArchived ? 'soft' : 'ghost'"
        size="sm" :title="filters.hideArchived ? 'Mostrar arquivadas' : 'Ocultar arquivadas'"
        @click="filters.hideArchived = !filters.hideArchived" />

      <UButton v-if="hasAnyActiveFilter" class="tasks-toolbar__filter-btn" icon="i-lucide-filter-x" color="neutral"
        variant="ghost" size="sm" title="Limpar filtros" @click="clearFilters" />
    </div>

    <div v-if="activeProject" class="tasks-toolbar__stats">
      <span class="tasks-toolbar__stat tasks-toolbar__stat--neutral" :title="`Total: ${projectTasks.length}`">{{
        projectTasks.length }}</span>
      <span class="tasks-toolbar__stat tasks-toolbar__stat--primary"
        :title="`Filtradas: ${filteredTasks.length}`">{{
          filteredTasks.length }}</span>
      <span class="tasks-toolbar__stat tasks-toolbar__stat--warning"
        :title="`Arquivadas: ${projectTasks.filter(t => t.archived).length}`">{{projectTasks.filter(t =>
          t.archived).length
        }}</span>
    </div>

    <div v-if="activeProject" class="tasks-toolbar__view-seg" role="tablist">
      <button class="tasks-toolbar__view-btn" :class="{ 'is-active': viewMode === 'board' }" type="button"
        title="Board" role="tab" :aria-selected="viewMode === 'board'" @click="viewMode = 'board'">
        <UIcon name="i-lucide-kanban" />
      </button>
      <button class="tasks-toolbar__view-btn" :class="{ 'is-active': viewMode === 'table' }" type="button"
        title="Tabela" role="tab" :aria-selected="viewMode === 'table'" @click="viewMode = 'table'">
        <UIcon name="i-lucide-table" />
      </button>
    </div>

    <UButton class="tasks-toolbar__primary" icon="i-lucide-plus"
      :label="viewMode === 'table' ? 'Nova linha' : 'Nova task'" color="primary" variant="soft" size="sm"
      @click="viewMode === 'table' ? createTableTask() : beginCreateTaskInFirstColumn()" />

    <UPopover :content="{ side: 'bottom', align: 'end' }">
      <UButton class="tasks-toolbar__filter-btn" icon="i-lucide-more-horizontal" color="neutral" variant="ghost"
        size="sm" title="Mais acoes" />
      <template #content>
        <div class="tasks-toolbar__menu">
          <button class="tasks-toolbar__menu-item" type="button"
            :disabled="!activeProject || boardGroupBy !== 'status'" @click="createColumn">
            <UIcon name="i-lucide-columns-3" />
            <span>Nova coluna</span>
          </button>
          <button class="tasks-toolbar__menu-item" type="button" :disabled="!activeProject"
            @click="projectSettingsOpen = true">
            <UIcon name="i-lucide-settings-2" />
            <span>Configurar pagina</span>
          </button>
          <button v-if="activeProject && boardView.hiddenColumnIds.length" class="tasks-toolbar__menu-item"
            type="button" @click="showAllColumns">
            <UIcon name="i-lucide-eye" />
            <span>Mostrar {{ boardView.hiddenColumnIds.length }} grupos</span>
          </button>
          <div v-if="activeProject" class="tasks-toolbar__menu-info">
            Pagina: <strong>{{ activeProject.name }}</strong>
            <span class="tasks-toolbar__menu-info-sep">·</span>
            Agrupar: <strong>{{ fieldLabel(boardGroupBy) }}</strong>
          </div>
        </div>
      </template>
    </UPopover>
  </div>
</template>
