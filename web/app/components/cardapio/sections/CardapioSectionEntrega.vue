<script setup lang="ts">
import { computed, ref } from 'vue'

import CardapioMoneyInput from '~/components/cardapio/CardapioMoneyInput.vue'
import { useSortableList } from '~/composables/useSortableList'
import { useCardapioStore } from '~/stores/cardapio'
import { useUiStore } from '~/stores/ui'
import { getApiErrorMessage } from '~/utils/api-client'
import type { DeliveryZone } from '~/domain/cardapio/types'

// WS-A — Zonas de entrega (bairro + valor do frete). Grid de cards em 2 colunas,
// drag-n-drop (useSortableList, HTML5 nativo) e badge de ordem (#1, #2...).
// PATCH de zona e PARCIAL (pointer-based no back): toggle manda so {isActive};
// edicao manda so {name,feeCents}; reorder manda so {sortOrder} por zona alterada.

const store = useCardapioStore()
const ui = useUiStore()

const newName = ref('')
const newFeeCents = ref(0)
const creating = ref(false)
const busyId = ref('')
const reordering = ref(false)
const editingId = ref('')
const editName = ref('')
const editFeeCents = ref(0)

const ordered = computed(() =>
  [...store.zones].sort((a, b) => a.sortOrder - b.sortOrder || a.name.localeCompare(b.name)),
)

function feeLabel(feeCents: number) {
  return `R$ ${(feeCents / 100).toFixed(2).replace('.', ',')}`
}

async function onCreate() {
  const name = newName.value.trim()
  if (!name || creating.value || !store.restaurantId) {
    return
  }
  creating.value = true
  try {
    await store.createZone(store.restaurantId, {
      name,
      feeCents: newFeeCents.value,
      isActive: true,
      sortOrder: store.zones.length,
    })
    newName.value = ''
    newFeeCents.value = 0
    ui.success('Bairro adicionado.')
  } catch (caught) {
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel adicionar o bairro.'))
  } finally {
    creating.value = false
  }
}

function startEdit(zone: DeliveryZone) {
  editingId.value = zone.id
  editName.value = zone.name
  editFeeCents.value = zone.feeCents
}

function cancelEdit() {
  editingId.value = ''
}

async function saveEdit(zone: DeliveryZone) {
  const name = editName.value.trim()
  if (!name) {
    return
  }
  busyId.value = zone.id
  try {
    // PATCH parcial: so os campos editaveis aqui.
    await store.patchZone(zone.id, { name, feeCents: editFeeCents.value })
    editingId.value = ''
    ui.success('Bairro atualizado.')
  } catch (caught) {
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel atualizar o bairro.'))
  } finally {
    busyId.value = ''
  }
}

async function toggleActive(zone: DeliveryZone) {
  busyId.value = zone.id
  try {
    await store.patchZone(zone.id, { isActive: !zone.isActive })
  } catch (caught) {
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel alterar o status.'))
  } finally {
    busyId.value = ''
  }
}

async function remove(zone: DeliveryZone) {
  const { confirmed } = (await ui.confirm({
    title: 'Remover bairro',
    message: `Remover a zona de entrega "${zone.name}"?`,
    confirmLabel: 'Remover',
  })) as { confirmed: boolean }
  if (!confirmed) {
    return
  }
  busyId.value = zone.id
  try {
    await store.deleteZone(zone.id)
    ui.success('Bairro removido.')
  } catch (caught) {
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel remover o bairro.'))
  } finally {
    busyId.value = ''
  }
}

// Reordena local, recalcula sortOrder contiguo (0..n-1) e PATCH parcial {sortOrder}
// so nas zonas cujo indice mudou. Em erro, re-hidrata do banco (reloadZones) + toast.
async function onReorder(from: number, to: number) {
  if (reordering.value || from === to) {
    return
  }
  const previous = ordered.value
  const next = [...previous]
  const [moved] = next.splice(from, 1)
  if (!moved) {
    return
  }
  next.splice(to, 0, moved)

  const changed = next
    .map((zone, index) => ({ zone, sortOrder: index }))
    .filter(({ zone, sortOrder }) => zone.sortOrder !== sortOrder)
  if (!changed.length) {
    return
  }

  reordering.value = true
  try {
    await Promise.all(changed.map(({ zone, sortOrder }) => store.patchZone(zone.id, { sortOrder })))
    ui.success('Ordem das zonas atualizada.')
  } catch (caught) {
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel reordenar as zonas.'))
    if (store.restaurantId) {
      await store.reloadZones(store.restaurantId)
    }
  } finally {
    reordering.value = false
  }
}

const { draggingIndex, overIndex, itemHandlers } = useSortableList({ onReorder })
</script>

<template>
  <div class="cardapio-zones">
    <p class="cardapio-zones__note">
      Cada bairro tem um valor de frete proprio. No site, o cliente escolhe o bairro na entrega e o
      frete e calculado pela zona. Bairros inativos nao aparecem no site. Arraste os cards para
      reordenar a exibicao.
    </p>

    <form class="cardapio-zones__create" @submit.prevent="onCreate">
      <input
        v-model="newName"
        type="text"
        class="cardapio-zones__input"
        placeholder="Nome do bairro"
      />
      <div class="cardapio-zones__fee">
        <CardapioMoneyInput v-model="newFeeCents" />
      </div>
      <button type="submit" class="cardapio-zones__add" :disabled="creating || !newName.trim()">
        {{ creating ? 'Adicionando...' : 'Adicionar' }}
      </button>
    </form>

    <p v-if="!ordered.length" class="cardapio-zones__empty">
      Nenhuma zona de entrega cadastrada. Adicione os bairros que voce atende.
    </p>

    <ul v-else class="cardapio-zones__grid">
      <li
        v-for="(zone, index) in ordered"
        :key="zone.id"
        class="cardapio-zones__card"
        :class="{
          'is-dragging': draggingIndex === index,
          'is-over': overIndex === index && draggingIndex !== index,
        }"
        v-bind="itemHandlers(index)"
      >
        <span class="cardapio-zones__handle" aria-hidden="true" title="Arrastar para reordenar">
          &#x2630;
        </span>
        <span class="cardapio-zones__badge">#{{ index + 1 }}</span>

        <div class="cardapio-zones__body">
          <template v-if="editingId === zone.id">
            <input
              v-model="editName"
              type="text"
              class="cardapio-zones__input"
              placeholder="Nome do bairro"
              @keydown.enter="saveEdit(zone)"
            />
            <div class="cardapio-zones__fee">
              <CardapioMoneyInput v-model="editFeeCents" />
            </div>
          </template>
          <template v-else>
            <span class="cardapio-zones__name">{{ zone.name }}</span>
            <span class="cardapio-zones__feeval">{{ feeLabel(zone.feeCents) }}</span>
          </template>
        </div>

        <div class="cardapio-zones__actions">
          <span
            class="cardapio-zones__pill"
            :class="zone.isActive ? 'is-on' : 'is-off'"
            @click="toggleActive(zone)"
          >
            {{ zone.isActive ? 'Ativo' : 'Inativo' }}
          </span>
          <template v-if="editingId === zone.id">
            <button
              type="button"
              class="cardapio-zones__btn"
              :disabled="busyId === zone.id"
              @click="saveEdit(zone)"
            >
              Salvar
            </button>
            <button type="button" class="cardapio-zones__btn" @click="cancelEdit">Cancelar</button>
          </template>
          <template v-else>
            <button type="button" class="cardapio-zones__btn" @click="startEdit(zone)">
              Editar
            </button>
            <button
              type="button"
              class="cardapio-zones__btn cardapio-zones__btn--danger"
              :disabled="busyId === zone.id"
              @click="remove(zone)"
            >
              Remover
            </button>
          </template>
        </div>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.cardapio-zones {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.cardapio-zones__note {
  font-size: 0.86rem;
  color: var(--text-muted);
}

.cardapio-zones__create {
  display: flex;
  gap: 0.6rem;
  align-items: center;
  flex-wrap: wrap;
}

.cardapio-zones__input {
  flex: 1;
  min-width: 160px;
  padding: 0.55rem 0.7rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2) / 0.6);
  color: var(--text-main);
  font-size: 0.9rem;
}

.cardapio-zones__input:focus {
  outline: none;
  border-color: rgb(var(--ring));
  box-shadow: 0 0 0 3px rgb(var(--ring) / 0.18);
}

.cardapio-zones__fee {
  width: 9rem;
  flex-shrink: 0;
}

.cardapio-zones__add {
  border: none;
  color: rgb(var(--surface));
  background: linear-gradient(135deg, rgb(var(--primary)), rgb(var(--primary-600)));
  padding: 0.55rem 1.1rem;
  border-radius: var(--radius-sm);
  font-weight: 600;
  font-size: 0.88rem;
  cursor: pointer;
  white-space: nowrap;
}

.cardapio-zones__add:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.cardapio-zones__empty {
  color: var(--text-muted);
  font-size: 0.9rem;
  padding: 1.5rem 0;
  text-align: center;
}

.cardapio-zones__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.6rem;
  list-style: none;
  padding: 0;
  margin: 0;
}

.cardapio-zones__card {
  display: flex;
  align-items: center;
  gap: 0.7rem;
  padding: 0.65rem 0.85rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.6);
  cursor: grab;
}

.cardapio-zones__card.is-dragging {
  opacity: 0.5;
  cursor: grabbing;
}

.cardapio-zones__card.is-over {
  border-color: rgb(var(--ring));
  box-shadow: 0 0 0 2px rgb(var(--ring) / 0.3);
}

.cardapio-zones__handle {
  color: var(--text-muted);
  font-size: 0.95rem;
  line-height: 1;
  cursor: grab;
  flex-shrink: 0;
}

.cardapio-zones__badge {
  min-width: 2rem;
  padding: 0.18rem 0.4rem;
  border-radius: 999px;
  text-align: center;
  font-size: 0.74rem;
  font-weight: 700;
  color: var(--text-muted);
  background: rgb(var(--surface-2) / 0.8);
  border: 1px solid var(--line-soft);
  flex-shrink: 0;
}

.cardapio-zones__body {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 0.6rem;
  flex-wrap: wrap;
}

.cardapio-zones__name {
  font-weight: 600;
  color: var(--text-main);
}

.cardapio-zones__feeval {
  font-size: 0.86rem;
  color: var(--text-muted);
  white-space: nowrap;
}

.cardapio-zones__actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-shrink: 0;
}

.cardapio-zones__pill {
  padding: 0.18rem 0.55rem;
  border-radius: 999px;
  font-size: 0.74rem;
  font-weight: 600;
  cursor: pointer;
}

.cardapio-zones__pill.is-on {
  background: rgb(var(--success) / 0.16);
  color: rgb(var(--success));
}

.cardapio-zones__pill.is-off {
  background: rgb(var(--muted) / 0.16);
  color: var(--text-muted);
}

.cardapio-zones__btn {
  border: 1px solid var(--line-soft);
  background: rgb(var(--surface-2) / 0.8);
  color: var(--text-main);
  padding: 0.4rem 0.7rem;
  border-radius: var(--radius-sm);
  font-size: 0.82rem;
  font-weight: 600;
  cursor: pointer;
  white-space: nowrap;
}

.cardapio-zones__btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.cardapio-zones__btn--danger {
  color: rgb(var(--danger));
  border-color: rgb(var(--danger) / 0.4);
}

@media (max-width: 720px) {
  .cardapio-zones__grid {
    grid-template-columns: minmax(0, 1fr);
  }
}

@media (max-width: 640px) {
  .cardapio-zones__fee {
    width: 100%;
  }

  .cardapio-zones__card {
    flex-wrap: wrap;
  }
}
</style>
