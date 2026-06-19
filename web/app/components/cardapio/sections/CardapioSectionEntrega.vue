<script setup lang="ts">
import { computed, ref } from 'vue'

import CardapioMoneyInput from '~/components/cardapio/CardapioMoneyInput.vue'
import { useCardapioStore } from '~/stores/cardapio'
import { useUiStore } from '~/stores/ui'
import { getApiErrorMessage } from '~/utils/api-client'
import type { DeliveryZone } from '~/domain/cardapio/types'

// WS-A — Zonas de entrega (bairro + valor do frete). PATCH de zona e PARCIAL
// (pointer-based no back): o toggle de ativo manda so {isActive}; o save da
// edicao manda so os campos alterados (name/feeCents).

const store = useCardapioStore()
const ui = useUiStore()

const newName = ref('')
const newFeeCents = ref(0)
const creating = ref(false)
const busyId = ref('')
const editingId = ref('')
const editName = ref('')
const editFeeCents = ref(0)

const ordered = computed(() =>
  [...store.zones].sort((a, b) => a.sortOrder - b.sortOrder || a.name.localeCompare(b.name)),
)

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
</script>

<template>
  <div class="cardapio-zones">
    <p class="cardapio-zones__note">
      Cada bairro tem um valor de frete proprio. No site, o cliente escolhe o bairro na entrega e o
      frete e calculado pela zona. Bairros inativos nao aparecem no site.
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

    <table v-else class="cardapio-zones__table">
      <thead>
        <tr>
          <th class="cardapio-zones__th">Bairro</th>
          <th class="cardapio-zones__th cardapio-zones__th--fee">Valor</th>
          <th class="cardapio-zones__th cardapio-zones__th--actions">Acoes</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="zone in ordered" :key="zone.id" class="cardapio-zones__row">
          <td class="cardapio-zones__cell">
            <input
              v-if="editingId === zone.id"
              v-model="editName"
              type="text"
              class="cardapio-zones__input"
              @keydown.enter="saveEdit(zone)"
            />
            <span v-else class="cardapio-zones__name">{{ zone.name }}</span>
          </td>
          <td class="cardapio-zones__cell cardapio-zones__cell--fee">
            <div v-if="editingId === zone.id" class="cardapio-zones__fee">
              <CardapioMoneyInput v-model="editFeeCents" />
            </div>
            <span v-else>R$ {{ (zone.feeCents / 100).toFixed(2).replace('.', ',') }}</span>
          </td>
          <td class="cardapio-zones__cell cardapio-zones__cell--actions">
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
              <button type="button" class="cardapio-zones__btn" @click="cancelEdit">
                Cancelar
              </button>
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
          </td>
        </tr>
      </tbody>
    </table>
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

.cardapio-zones__table {
  width: 100%;
  border-collapse: collapse;
}

.cardapio-zones__th {
  text-align: left;
  font-size: 0.78rem;
  font-weight: 600;
  color: var(--text-muted);
  padding: 0.5rem 0.7rem;
  border-bottom: 1px solid var(--line-soft);
}

.cardapio-zones__th--fee {
  width: 9rem;
}

.cardapio-zones__th--actions {
  width: 1%;
  white-space: nowrap;
}

.cardapio-zones__row {
  border-bottom: 1px solid var(--line-soft);
}

.cardapio-zones__cell {
  padding: 0.5rem 0.7rem;
  color: var(--text-main);
  font-size: 0.9rem;
  vertical-align: middle;
}

.cardapio-zones__cell--fee {
  white-space: nowrap;
}

.cardapio-zones__cell--actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  justify-content: flex-end;
}

.cardapio-zones__name {
  font-weight: 600;
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

@media (max-width: 640px) {
  .cardapio-zones__fee {
    width: 100%;
  }
}
</style>
