<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import { useCardapioStore } from '~/stores/cardapio'
import { useUiStore } from '~/stores/ui'
import { getApiErrorMessage } from '~/utils/api-client'
import { openWhatsapp } from '~/utils/whatsapp'
import {
  ORDER_STATUS_LABELS,
  ORDER_STATUS_ORDER,
  ORDER_TYPE_LABELS,
  formatCurrency,
} from '~/domain/cardapio/types'
import type { Order, OrderStatus } from '~/domain/cardapio/types'

const store = useCardapioStore()
const ui = useUiStore()

const statusFilter = ref<'' | OrderStatus>('')
const updatingId = ref('')
const expandedId = ref('')

const statusOptions = ORDER_STATUS_ORDER
const totalPages = computed(() => Math.max(1, Math.ceil(store.orders.total / store.orders.perPage)))

function dateLabel(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return '—'
  }
  return date.toLocaleString('pt-BR', {
    day: '2-digit',
    month: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

async function reload(page = 1) {
  if (!store.restaurantId) {
    return
  }
  await store.loadOrders(store.restaurantId, { status: statusFilter.value, page })
}

function onFilterChange() {
  void reload(1)
}

function changePage(delta: number) {
  const next = store.orders.page + delta
  if (next < 1 || next > totalPages.value) {
    return
  }
  void reload(next)
}

async function onStatusChange(order: Order, event: Event) {
  const value = (event.target as HTMLSelectElement).value as OrderStatus
  updatingId.value = order.id
  try {
    await store.updateOrderStatus(order.id, value)
    ui.success(`Pedido #${order.orderNumber} agora esta "${ORDER_STATUS_LABELS[value]}".`)
  } catch (caught) {
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel atualizar o status.'))
  } finally {
    updatingId.value = ''
  }
}

function toggleExpand(orderId: string) {
  expandedId.value = expandedId.value === orderId ? '' : orderId
}

function openWhatsappForOrder(order: Order) {
  const text = order.code ? `Ola! Sobre seu pedido ${order.code}` : undefined
  openWhatsapp(order.customerPhone, text)
}

onMounted(() => {
  void reload(1)
})
</script>

<template>
  <div class="cardapio-orders">
    <div class="cardapio-orders__toolbar">
      <label class="cardapio-orders__filter">
        <span class="cardapio-orders__filter-label">Status</span>
        <select v-model="statusFilter" class="cardapio-orders__select" @change="onFilterChange">
          <option value="">Todos</option>
          <option v-for="status in statusOptions" :key="status" :value="status">
            {{ ORDER_STATUS_LABELS[status] }}
          </option>
        </select>
      </label>
    </div>

    <p v-if="store.ordersError" class="cardapio-orders__error">{{ store.ordersError }}</p>
    <div v-if="store.ordersPending" class="cardapio-orders__state">Carregando pedidos...</div>

    <p v-else-if="!store.orders.items.length" class="cardapio-orders__empty">
      Nenhum pedido neste filtro.
    </p>

    <ul v-else class="cardapio-orders__list">
      <li v-for="order in store.orders.items" :key="order.id" class="cardapio-orders__item">
        <div class="cardapio-orders__head" @click="toggleExpand(order.id)">
          <div class="cardapio-orders__id">
            <span class="cardapio-orders__number">{{ order.code || `#${order.orderNumber}` }}</span>
            <span class="cardapio-orders__type">
              #{{ order.orderNumber }} · {{ ORDER_TYPE_LABELS[order.type] }}
            </span>
          </div>
          <div class="cardapio-orders__meta">
            <span class="cardapio-orders__customer">{{ order.customerName || 'Cliente' }}</span>
            <span class="cardapio-orders__date">{{ dateLabel(order.createdAt) }}</span>
          </div>
          <span class="cardapio-orders__total">{{ formatCurrency(order.totalCents) }}</span>
          <select
            class="cardapio-orders__status"
            :value="order.status"
            :disabled="updatingId === order.id"
            @click.stop
            @change="onStatusChange(order, $event)"
          >
            <option v-for="status in statusOptions" :key="status" :value="status">
              {{ ORDER_STATUS_LABELS[status] }}
            </option>
          </select>
        </div>

        <div v-if="expandedId === order.id" class="cardapio-orders__details">
          <p v-if="order.customerPhone" class="cardapio-orders__line">
            <strong>Telefone:</strong>
            <button
              type="button"
              class="cardapio-orders__whatsapp"
              aria-label="Abrir conversa no WhatsApp"
              :title="`Abrir conversa no WhatsApp: ${order.customerPhone}`"
              @click="openWhatsappForOrder(order)"
            >
              <svg
                class="cardapio-orders__whatsapp-icon"
                viewBox="0 0 24 24"
                width="16"
                height="16"
                fill="currentColor"
                aria-hidden="true"
              >
                <path
                  d="M12.04 2.5c-5.25 0-9.5 4.25-9.5 9.5 0 1.68.44 3.3 1.28 4.74L2.5 21.5l4.9-1.28a9.46 9.46 0 0 0 4.64 1.2h.01c5.24 0 9.5-4.26 9.5-9.5s-4.26-9.5-9.51-9.5Zm0 17.2h-.01a7.9 7.9 0 0 1-4.02-1.1l-.29-.17-2.9.76.77-2.83-.19-.29a7.86 7.86 0 0 1-1.21-4.18 7.94 7.94 0 0 1 13.56-5.6 7.86 7.86 0 0 1 2.33 5.61c0 4.36-3.55 7.9-7.84 7.9Zm4.34-5.92c-.24-.12-1.4-.69-1.62-.77-.22-.08-.38-.12-.53.12-.16.24-.61.77-.75.93-.14.16-.28.18-.51.06-.24-.12-1-.37-1.91-1.18-.71-.63-1.18-1.41-1.32-1.65-.14-.24-.02-.37.1-.49.11-.11.24-.28.36-.42.12-.14.16-.24.24-.4.08-.16.04-.3-.02-.42-.06-.12-.53-1.28-.73-1.76-.19-.46-.39-.4-.53-.4l-.45-.01c-.16 0-.42.06-.63.3-.22.24-.83.81-.83 1.97 0 1.16.85 2.29.97 2.45.12.16 1.67 2.55 4.04 3.58.57.24 1 .39 1.35.5.57.18 1.08.16 1.49.1.46-.07 1.4-.57 1.6-1.12.2-.55.2-1.02.14-1.12-.06-.1-.22-.16-.46-.28Z"
                />
              </svg>
              {{ order.customerPhone }}
            </button>
          </p>
          <p v-if="order.deliveryAddress" class="cardapio-orders__line">
            <strong>Entrega:</strong>
            {{ order.deliveryAddress }}
          </p>
          <p v-if="order.notes" class="cardapio-orders__line">
            <strong>Observacao:</strong>
            {{ order.notes }}
          </p>
          <ul class="cardapio-orders__items">
            <li v-for="item in order.items" :key="item.id" class="cardapio-orders__product">
              <span>{{ item.quantity }}x {{ item.productName }}</span>
              <span v-if="item.variationName" class="cardapio-orders__variation">
                ({{ item.variationName }})
              </span>
              <span class="cardapio-orders__product-total">
                {{ formatCurrency(item.totalCents) }}
              </span>
            </li>
          </ul>
          <div class="cardapio-orders__summary">
            <span>Subtotal {{ formatCurrency(order.subtotalCents) }}</span>
            <span>Entrega {{ formatCurrency(order.deliveryFeeCents) }}</span>
            <span v-if="order.discountCents">
              Desconto -{{ formatCurrency(order.discountCents) }}
            </span>
          </div>
        </div>
      </li>
    </ul>

    <div v-if="store.orders.items.length" class="cardapio-orders__pager">
      <button
        type="button"
        class="cardapio-orders__page-btn"
        :disabled="store.orders.page <= 1 || store.ordersPending"
        @click="changePage(-1)"
      >
        Anterior
      </button>
      <span class="cardapio-orders__page-label">
        Pagina {{ store.orders.page }} de {{ totalPages }}
      </span>
      <button
        type="button"
        class="cardapio-orders__page-btn"
        :disabled="store.orders.page >= totalPages || store.ordersPending"
        @click="changePage(1)"
      >
        Proxima
      </button>
    </div>
  </div>
</template>

<style scoped>
.cardapio-orders {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.cardapio-orders__toolbar {
  display: flex;
  gap: 1rem;
}

.cardapio-orders__filter {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
}

.cardapio-orders__filter-label {
  font-size: 0.78rem;
  font-weight: 600;
  color: var(--text-muted);
}

.cardapio-orders__select,
.cardapio-orders__status {
  padding: 0.5rem 0.65rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2) / 0.6);
  color: var(--text-main);
  font-size: 0.86rem;
}

.cardapio-orders__select:focus,
.cardapio-orders__status:focus {
  outline: none;
  border-color: rgb(var(--ring));
  box-shadow: 0 0 0 3px rgb(var(--ring) / 0.18);
}

.cardapio-orders__error {
  color: rgb(var(--danger));
  background: rgb(var(--danger) / 0.14);
  padding: 0.6rem 0.85rem;
  border-radius: var(--radius-sm);
  font-size: 0.9rem;
}

.cardapio-orders__state,
.cardapio-orders__empty {
  color: var(--text-muted);
  font-size: 0.9rem;
  padding: 1rem 0;
}

.cardapio-orders__list {
  display: flex;
  flex-direction: column;
  gap: 0.55rem;
  list-style: none;
}

.cardapio-orders__item {
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.6);
  overflow: hidden;
}

.cardapio-orders__head {
  display: flex;
  align-items: center;
  gap: 0.85rem;
  padding: 0.7rem 0.9rem;
  cursor: pointer;
  flex-wrap: wrap;
}

.cardapio-orders__id {
  display: flex;
  flex-direction: column;
}

.cardapio-orders__number {
  font-weight: 700;
  color: var(--text-main);
}

.cardapio-orders__type {
  font-size: 0.74rem;
  color: var(--text-muted);
}

.cardapio-orders__meta {
  flex: 1;
  min-width: 120px;
  display: flex;
  flex-direction: column;
}

.cardapio-orders__customer {
  font-size: 0.88rem;
  color: var(--text-main);
}

.cardapio-orders__date {
  font-size: 0.76rem;
  color: var(--text-muted);
}

.cardapio-orders__total {
  font-weight: 700;
  color: var(--text-main);
}

.cardapio-orders__details {
  padding: 0.85rem 0.9rem;
  border-top: 1px solid var(--line-soft);
  background: rgb(var(--surface-2) / 0.4);
}

.cardapio-orders__line {
  font-size: 0.86rem;
  color: var(--text-main);
  margin-bottom: 0.3rem;
}

.cardapio-orders__whatsapp {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  padding: 0.2rem 0.55rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2) / 0.6);
  color: var(--text-main);
  font-size: 0.86rem;
  font-weight: 600;
  cursor: pointer;
  vertical-align: middle;
}

.cardapio-orders__whatsapp:hover {
  border-color: rgb(var(--ring));
  background: rgb(var(--surface-2) / 0.9);
}

.cardapio-orders__whatsapp:focus-visible {
  outline: none;
  border-color: rgb(var(--ring));
  box-shadow: 0 0 0 3px rgb(var(--ring) / 0.18);
}

.cardapio-orders__whatsapp-icon {
  color: rgb(var(--success));
  flex-shrink: 0;
}

.cardapio-orders__items {
  list-style: none;
  margin: 0.5rem 0;
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.cardapio-orders__product {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  font-size: 0.86rem;
  color: var(--text-main);
}

.cardapio-orders__variation {
  color: var(--text-muted);
  font-size: 0.8rem;
}

.cardapio-orders__product-total {
  margin-left: auto;
  color: var(--text-muted);
}

.cardapio-orders__summary {
  display: flex;
  gap: 1rem;
  flex-wrap: wrap;
  font-size: 0.82rem;
  color: var(--text-muted);
  margin-top: 0.4rem;
}

.cardapio-orders__pager {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 1rem;
}

.cardapio-orders__page-btn {
  border: 1px solid var(--line-soft);
  background: rgb(var(--surface-2) / 0.8);
  color: var(--text-main);
  padding: 0.45rem 0.9rem;
  border-radius: var(--radius-sm);
  font-size: 0.84rem;
  font-weight: 600;
  cursor: pointer;
}

.cardapio-orders__page-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.cardapio-orders__page-label {
  font-size: 0.84rem;
  color: var(--text-muted);
}
</style>
