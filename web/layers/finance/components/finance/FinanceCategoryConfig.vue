<script setup lang="ts">
import { InputsOmniSelectMenuInput as OmniSelectMenuInput } from '#components'
import { FINANCE_CONFIG_KEY } from '../../composables/useFinanceConfigEditor'
import { KIND_OPTIONS } from '../../utils/finance-helpers'
const config = inject(FINANCE_CONFIG_KEY)!
const {
  configDraft,
  newCategory,
  categoryEditDraft,
  editingCategoryId,
  addCategory,
  startEditCategory,
  finishEditCategory,
  cancelEditCategory,
  removeCategory,
} = config
</script>
<template>
  <UCollapsible :default-open="false" class="finances-config__collapsible">
    <template #default="{ open }">
      <div class="finances-config__collapsible-trigger">
        <div class="min-w-0">
          <p class="text-sm font-semibold">Categorias</p>
          <p class="text-xs text-[rgb(var(--muted))]">Defina categorias para entradas e saidas.</p>
        </div>
        <div class="flex items-center gap-2">
          <UBadge class="finances-config__counter-badge" color="neutral" variant="soft">
            {{ configDraft.categories.length }} itens
          </UBadge>
          <UIcon :name="open ? 'i-lucide-chevron-up' : 'i-lucide-chevron-down'" class="size-4" />
        </div>
      </div>
    </template>

    <template #content>
      <div class="finances-config__collapsible-content space-y-2">
        <div class="grid gap-2 md:grid-cols-[minmax(0,1fr)_160px]">
          <UInput
            v-model="newCategory.name"
            class="finances-config__input finances-config__input--category-name"
            placeholder="Nome da categoria"
          />
          <OmniSelectMenuInput
            v-model="newCategory.kind"
            class="finances-config__input finances-config__input--category-kind"
            :items="KIND_OPTIONS"
            item-display-mode="text"
            :searchable="false"
            :creatable="false"
            :clear="false"
            :badge-mode="true"
            option-edit-mode="none"
            color="neutral"
            variant="none"
            :highlight="false"
          />
        </div>
        <UTextarea
          v-model="newCategory.description"
          class="finances-config__input finances-config__input--category-description"
          :rows="2"
          placeholder="Descricao (opcional)"
        />
        <div class="flex justify-end">
          <UButton
            class="finances-config__action finances-config__action--add-category"
            icon="i-lucide-plus"
            size="sm"
            @click="addCategory"
          >
            Adicionar categoria
          </UButton>
        </div>

        <div class="mt-3 space-y-2">
          <div
            v-for="category in configDraft.categories"
            :key="category.id"
            class="finances-config__item-row"
          >
            <div class="min-w-0 w-full">
              <template v-if="editingCategoryId === category.id">
                <div class="grid gap-2 md:grid-cols-[minmax(0,1fr)_140px]">
                  <UInput
                    v-model="categoryEditDraft.name"
                    class="finances-config__input finances-config__input--category-edit-name"
                    placeholder="Nome da categoria"
                  />
                  <OmniSelectMenuInput
                    v-model="categoryEditDraft.kind"
                    class="finances-config__input finances-config__input--category-edit-kind"
                    :items="KIND_OPTIONS"
                    item-display-mode="text"
                    :searchable="false"
                    :creatable="false"
                    :clear="false"
                    :badge-mode="true"
                    option-edit-mode="none"
                    color="neutral"
                    variant="none"
                    :highlight="false"
                  />
                </div>
                <div class="mt-2 flex justify-end gap-1">
                  <UButton
                    size="xs"
                    color="neutral"
                    variant="soft"
                    class="finances-config__action finances-config__action--save-category"
                    @click="finishEditCategory"
                  >
                    Salvar
                  </UButton>
                  <UButton
                    size="xs"
                    color="neutral"
                    variant="ghost"
                    class="finances-config__action finances-config__action--cancel-category"
                    @click="cancelEditCategory"
                  >
                    Cancelar
                  </UButton>
                </div>
              </template>
              <template v-else>
                <p class="truncate text-sm font-medium">{{ category.name }}</p>
                <p class="text-xs text-[rgb(var(--muted))]">
                  {{ category.kind }} | {{ category.description || 'Sem descricao' }}
                </p>
              </template>
            </div>
            <div class="flex items-center gap-1">
              <UButton
                icon="i-lucide-pencil"
                color="neutral"
                variant="ghost"
                size="xs"
                class="finances-config__action finances-config__action--edit-category"
                @click="startEditCategory(category.id)"
              />
              <UButton
                icon="i-lucide-trash-2"
                color="error"
                variant="ghost"
                size="xs"
                class="finances-config__action finances-config__action--remove-category"
                @click="removeCategory(category.id)"
              />
            </div>
          </div>
        </div>
      </div>
    </template>
  </UCollapsible>
</template>
