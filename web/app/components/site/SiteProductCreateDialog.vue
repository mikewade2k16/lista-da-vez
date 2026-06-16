<script setup lang="ts">
import { reactive, watch } from 'vue'
import type { ProductCreateInput } from '~/types/products'

const props = defineProps<{
  open: boolean
  creating: boolean
}>()

const emit = defineEmits<{
  'update:open': [value: boolean]
  submit: [payload: ProductCreateInput]
}>()

function emptyForm(): ProductCreateInput {
  return {
    name: '',
    code: '',
    description: '',
    image: '',
    categories: [],
    campaigns: [],
    price: 0,
    fator: 1,
    tipo: '',
    stock: 0,
  }
}

const form = reactive<ProductCreateInput>(emptyForm())

watch(
  () => props.open,
  (open) => {
    if (!open) return
    Object.assign(form, emptyForm())
  },
)

function close() {
  emit('update:open', false)
}

function submit() {
  emit('submit', { ...form })
}
</script>

<template>
  <UModal :open="props.open" @update:open="emit('update:open', $event)">
    <template #content>
      <UCard class="site-product-create">
        <template #header>
          <h3 class="text-base font-semibold">Novo produto</h3>
        </template>

        <div class="space-y-3">
          <div>
            <label class="block text-xs text-[rgb(var(--muted))] mb-1">Nome</label>
            <UInput
              :model-value="form.name"
              @update:model-value="form.name = String($event ?? '')"
            />
          </div>
          <div>
            <label class="block text-xs text-[rgb(var(--muted))] mb-1">Codigo</label>
            <UInput
              :model-value="form.code"
              @update:model-value="form.code = String($event ?? '')"
            />
          </div>
          <div>
            <label class="block text-xs text-[rgb(var(--muted))] mb-1">Descricao</label>
            <UInput
              :model-value="form.description"
              @update:model-value="form.description = String($event ?? '')"
            />
          </div>
          <div>
            <label class="block text-xs text-[rgb(var(--muted))] mb-1">Image URL</label>
            <UInput
              :model-value="form.image"
              placeholder="https://..."
              @update:model-value="form.image = String($event ?? '')"
            />
          </div>
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="block text-xs text-[rgb(var(--muted))] mb-1">Preco</label>
              <UInput
                type="number"
                :model-value="form.price"
                @update:model-value="form.price = Number($event ?? 0)"
              />
            </div>
            <div>
              <label class="block text-xs text-[rgb(var(--muted))] mb-1">Fator</label>
              <UInput
                type="number"
                :model-value="form.fator"
                @update:model-value="form.fator = Number($event ?? 1)"
              />
            </div>
          </div>
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="block text-xs text-[rgb(var(--muted))] mb-1">Estoque</label>
              <UInput
                type="number"
                :model-value="form.stock"
                @update:model-value="form.stock = Number($event ?? 0)"
              />
            </div>
            <div>
              <label class="block text-xs text-[rgb(var(--muted))] mb-1">Tipo</label>
              <UInput
                :model-value="form.tipo"
                @update:model-value="form.tipo = String($event ?? '')"
              />
            </div>
          </div>
        </div>

        <template #footer>
          <div class="flex justify-end gap-2">
            <UButton label="Cancelar" color="neutral" variant="ghost" @click="close" />
            <UButton label="Criar" color="primary" :loading="props.creating" @click="submit" />
          </div>
        </template>
      </UCard>
    </template>
  </UModal>
</template>
