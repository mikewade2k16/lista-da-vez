<script setup lang="ts">
import { inject } from 'vue'
import { TASKS_PAGE_CONTEXT_KEY } from '../composables/useTasksPageContext'
import OmniSelectMenuInput from './inputs/OmniSelectMenuInput.vue'
import type { TaskProjectItem } from '../types/tasks'

const ctx = inject(TASKS_PAGE_CONTEXT_KEY)!
const {
  projectSettingsOpen,
  projectSettingsDraft,
  statusOptions,
  responsibleOptions,
  typeOptions,
  BOARD_GROUP_OPTIONS,
  FIELD_DEFS,
  filterSwitchDefs,
  cardFieldSwitchDefs,
  fieldSwitchValue,
  setFieldSwitch,
  viewerUserType,
  projectCount,
  deleteProject,
  settingsSaving,
  saveProjectSettings,
} = ctx
</script>

<template>
  <USlideover v-model:open="projectSettingsOpen" title="Configurar pagina"
    description="Defina base, colunas, filtros e campos do card." :ui="{ content: 'max-w-2xl' }">
    <template #body>
      <div class="tasks-page__settings space-y-4">
        <div class="tasks-page__settings-field space-y-1">
          <p
            class="tasks-page__settings-label text-xs font-semibold uppercase tracking-wide text-[rgb(var(--muted))]">
            Nome da pagina</p>
          <UInput v-model="projectSettingsDraft.name" class="tasks-page__settings-input"
            placeholder="Nome da pagina" />
        </div>

        <div class="tasks-page__settings-field space-y-1">
          <p
            class="tasks-page__settings-label text-xs font-semibold uppercase tracking-wide text-[rgb(var(--muted))]">
            Descricao</p>
          <UTextarea v-model="projectSettingsDraft.description" class="tasks-page__settings-input" :rows="2"
            placeholder="Descreva o uso desta pagina" />
        </div>

        <div class="tasks-page__settings-field space-y-1">
          <p
            class="tasks-page__settings-label text-xs font-semibold uppercase tracking-wide text-[rgb(var(--muted))]">
            Colunas do board</p>
          <OmniSelectMenuInput v-model="projectSettingsDraft.statuses" :items="statusOptions"
            placeholder="Criar ou selecionar coluna" :multiple="true"
            :creatable="{ when: 'always', position: 'bottom' }" :searchable="true" :full-content-width="true"
            item-display-mode="text" color="neutral" variant="none" :highlight="false" :badge-mode="true" clear
            option-edit-mode="full" />
        </div>

        <div class="tasks-page__settings-grid grid gap-4 md:grid-cols-2">
          <div class="tasks-page__settings-field space-y-1">
            <p
              class="tasks-page__settings-label text-xs font-semibold uppercase tracking-wide text-[rgb(var(--muted))]">
              Agrupar board por</p>
            <OmniSelectMenuInput v-model="projectSettingsDraft.boardGroupBy" :items="BOARD_GROUP_OPTIONS"
              placeholder="Agrupar por" :searchable="false" :full-content-width="true" item-display-mode="text"
              color="neutral" variant="none" :highlight="false" :badge-mode="true" option-edit-mode="color" />
          </div>

          <label
            class="tasks-page__settings-switch-row mt-5 flex items-center justify-between gap-3 rounded-[var(--radius-sm)] border border-[rgb(var(--border))] px-3 py-2">
            <span class="text-sm">Mostrar contagem nas colunas</span>
            <USwitch v-model="projectSettingsDraft.showAggregation" />
          </label>
        </div>

        <div class="tasks-page__settings-group rounded-[var(--radius-sm)] border border-[rgb(var(--border))] p-3">
          <p class="tasks-page__settings-group-title mb-2 text-sm font-semibold">Padroes de criacao</p>
          <div class="tasks-page__settings-switch-grid grid gap-2 sm:grid-cols-2">
            <label
              class="tasks-page__settings-switch-row flex items-center justify-between gap-3 rounded-[var(--radius-sm)] border border-[rgb(var(--border))] px-3 py-2">
              <span class="text-sm">Responsavel = criador</span>
              <USwitch v-model="projectSettingsDraft.defaults.responsibleFromCreator" />
            </label>
            <label
              class="tasks-page__settings-switch-row flex items-center justify-between gap-3 rounded-[var(--radius-sm)] border border-[rgb(var(--border))] px-3 py-2">
              <span class="text-sm">Cliente da sessao</span>
              <USwitch v-model="projectSettingsDraft.defaults.clientFromSession" />
            </label>
            <label
              class="tasks-page__settings-switch-row flex items-center justify-between gap-3 rounded-[var(--radius-sm)] border border-[rgb(var(--border))] px-3 py-2">
              <span class="text-sm">Mostrar criacao no card</span>
              <USwitch v-model="projectSettingsDraft.defaults.showCreatedAt" />
            </label>
          </div>
        </div>

        <div class="tasks-page__settings-grid grid gap-4 md:grid-cols-2">
          <div class="tasks-page__settings-field space-y-1">
            <p
              class="tasks-page__settings-label text-xs font-semibold uppercase tracking-wide text-[rgb(var(--muted))]">
              Responsaveis</p>
            <OmniSelectMenuInput v-model="projectSettingsDraft.responsibles" :items="responsibleOptions"
              placeholder="Adicionar responsavel" :multiple="true" :creatable="{ when: 'always', position: 'bottom' }"
              :searchable="true" :full-content-width="true" item-display-mode="text" color="neutral" variant="none"
              :highlight="false" :badge-mode="true" clear option-edit-mode="full" />
          </div>

          <div class="tasks-page__settings-field space-y-1">
            <p
              class="tasks-page__settings-label text-xs font-semibold uppercase tracking-wide text-[rgb(var(--muted))]">
              Tipos</p>
            <OmniSelectMenuInput v-model="projectSettingsDraft.types" :items="typeOptions"
              placeholder="Adicionar tipo" :multiple="true" :creatable="{ when: 'always', position: 'bottom' }"
              :searchable="true" :full-content-width="true" item-display-mode="text" color="neutral" variant="none"
              :highlight="false" :badge-mode="true" clear option-edit-mode="full" />
          </div>
        </div>

        <div class="tasks-page__settings-group rounded-[var(--radius-sm)] border border-[rgb(var(--border))] p-3">
          <p class="tasks-page__settings-group-title mb-2 text-sm font-semibold">Filtros ativos</p>
          <div class="tasks-page__settings-switch-grid grid gap-2 sm:grid-cols-2">
            <label v-for="item in filterSwitchDefs" :key="item.key"
              class="tasks-page__settings-switch-row flex items-center justify-between gap-3 rounded-[var(--radius-sm)] border border-[rgb(var(--border))] px-3 py-2">
              <span class="text-sm">{{ item.label }}</span>
              <USwitch v-model="projectSettingsDraft.filters[item.key as keyof TaskProjectItem['filters']]"
                :disabled="item.key === 'client' && viewerUserType === 'client'" />
            </label>
          </div>
        </div>

        <div class="tasks-page__settings-group rounded-[var(--radius-sm)] border border-[rgb(var(--border))] p-3">
          <p class="tasks-page__settings-group-title mb-2 text-sm font-semibold">Campos do card</p>
          <div class="tasks-page__settings-switch-grid grid gap-2 sm:grid-cols-2">
            <label v-for="item in cardFieldSwitchDefs" :key="item.key"
              class="tasks-page__settings-switch-row flex items-center justify-between gap-3 rounded-[var(--radius-sm)] border border-[rgb(var(--border))] px-3 py-2">
              <span class="text-sm">{{ item.label }}</span>
              <USwitch v-model="projectSettingsDraft.cardFields[item.key as keyof TaskProjectItem['cardFields']]"
                :disabled="item.key === 'client' && viewerUserType === 'client'" />
            </label>
          </div>
        </div>

        <div class="tasks-page__settings-group rounded-[var(--radius-sm)] border border-[rgb(var(--border))] p-3">
          <p class="tasks-page__settings-group-title mb-2 text-sm font-semibold">Campos visiveis no board</p>
          <div class="tasks-page__settings-switch-grid grid gap-2 sm:grid-cols-2">
            <label v-for="item in FIELD_DEFS.filter(field => field.key !== 'title' && field.key !== 'archived')"
              :key="`board-${item.key}`"
              class="tasks-page__settings-switch-row flex items-center justify-between gap-3 rounded-[var(--radius-sm)] border border-[rgb(var(--border))] px-3 py-2">
              <span class="text-sm">{{ item.label }}</span>
              <USwitch :model-value="fieldSwitchValue(projectSettingsDraft.boardVisibleFieldKeys, item.key)"
                :disabled="item.key === 'clientId' && viewerUserType === 'client'"
                @update:model-value="setFieldSwitch(projectSettingsDraft.boardVisibleFieldKeys, item.key, Boolean($event))" />
            </label>
          </div>
        </div>

        <div class="tasks-page__settings-group rounded-[var(--radius-sm)] border border-[rgb(var(--border))] p-3">
          <p class="tasks-page__settings-group-title mb-2 text-sm font-semibold">Campos visiveis na tabela</p>
          <div class="tasks-page__settings-switch-grid grid gap-2 sm:grid-cols-2">
            <label v-for="item in FIELD_DEFS" :key="`table-${item.key}`"
              class="tasks-page__settings-switch-row flex items-center justify-between gap-3 rounded-[var(--radius-sm)] border border-[rgb(var(--border))] px-3 py-2">
              <span class="text-sm">{{ item.label }}</span>
              <USwitch
                :model-value="item.key === 'title' || fieldSwitchValue(projectSettingsDraft.tableVisibleFieldKeys, item.key)"
                :disabled="item.key === 'title' || (item.key === 'clientId' && viewerUserType === 'client')"
                @update:model-value="setFieldSwitch(projectSettingsDraft.tableVisibleFieldKeys, item.key, Boolean($event))" />
            </label>
          </div>
        </div>

        <div class="tasks-page__settings-group rounded-[var(--radius-sm)] border border-[rgb(var(--border))] p-3">
          <p class="tasks-page__settings-group-title mb-2 text-sm font-semibold">Campos visiveis no modal</p>
          <div class="tasks-page__settings-switch-grid grid gap-2 sm:grid-cols-2">
            <label v-for="item in FIELD_DEFS.filter(field => field.key !== 'title')" :key="`modal-${item.key}`"
              class="tasks-page__settings-switch-row flex items-center justify-between gap-3 rounded-[var(--radius-sm)] border border-[rgb(var(--border))] px-3 py-2">
              <span class="text-sm">{{ item.label }}</span>
              <USwitch :model-value="fieldSwitchValue(projectSettingsDraft.modalVisibleFieldKeys, item.key)"
                :disabled="item.key === 'clientId' && viewerUserType === 'client'"
                @update:model-value="setFieldSwitch(projectSettingsDraft.modalVisibleFieldKeys, item.key, Boolean($event))" />
            </label>
          </div>
        </div>
      </div>
    </template>

    <template #footer>
      <div class="tasks-page__settings-footer flex w-full items-center justify-between gap-2">
        <UButton icon="i-lucide-trash-2" label="Excluir pagina" color="error" variant="ghost"
          :disabled="projectCount <= 1" @click="deleteProject" />
        <div class="tasks-page__settings-footer-actions flex items-center gap-2">
          <UButton label="Cancelar" color="neutral" variant="ghost" @click="projectSettingsOpen = false" />
          <UButton label="Salvar" color="primary" :loading="settingsSaving" @click="saveProjectSettings" />
        </div>
      </div>
    </template>
  </USlideover>
</template>
