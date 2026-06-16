<script setup lang="ts">
import { ref } from 'vue'

import { useCardapioStore } from '~/stores/cardapio'
import { useUiStore } from '~/stores/ui'
import { getApiErrorMessage } from '~/utils/api-client'
import type { RestaurantDomain } from '~/domain/cardapio/types'

const store = useCardapioStore()
const ui = useUiStore()

const newHost = ref('')
const newPrimary = ref(false)
const creating = ref(false)
const busyHost = ref('')

function normalizeHost(value: string): string {
  return String(value || '')
    .trim()
    .toLowerCase()
    .replace(/^https?:\/\//, '')
    .replace(/^www\./, '')
    .replace(/\/.*$/, '')
    .replace(/:\d+$/, '')
}

async function onCreate() {
  const host = normalizeHost(newHost.value)
  if (!host || creating.value || !store.restaurantId) {
    return
  }
  creating.value = true
  try {
    await store.createDomain(store.restaurantId, host, newPrimary.value)
    newHost.value = ''
    newPrimary.value = false
    ui.success('Dominio adicionado.')
  } catch (caught) {
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel adicionar o dominio.'))
  } finally {
    creating.value = false
  }
}

async function remove(domain: RestaurantDomain) {
  const { confirmed } = (await ui.confirm({
    title: 'Remover dominio',
    message: `Remover o dominio "${domain.host}"?`,
    confirmLabel: 'Remover',
  })) as { confirmed: boolean }
  if (!confirmed) {
    return
  }
  busyHost.value = domain.host
  try {
    await store.deleteDomain(domain.host)
    ui.success('Dominio removido.')
  } catch (caught) {
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel remover o dominio.'))
  } finally {
    busyHost.value = ''
  }
}
</script>

<template>
  <div class="cardapio-dom">
    <div class="cardapio-dom__note">
      <strong class="cardapio-dom__note-title">Como o front publico encontra este cardapio</strong>
      <p class="cardapio-dom__note-text">
        O site estatico resolve o restaurante pelo host de acesso. Ha duas formas:
      </p>
      <ul class="cardapio-dom__note-list">
        <li>
          <strong>Subdominio por convencao:</strong>
          o slug vira automaticamente
          <code>{{ store.restaurant?.slug || 'slug' }}.&lt;dominio-base&gt;</code>
          (configurado no servidor). Nao precisa cadastrar nada aqui.
        </li>
        <li>
          <strong>Dominio proprio:</strong>
          cadastre abaixo o host que o cliente vai usar (ex.:
          <code>cardapio.restaurante.com.br</code>
          ). Marque um como principal para gerar o link publico no topo do editor.
        </li>
      </ul>
    </div>

    <form class="cardapio-dom__create" @submit.prevent="onCreate">
      <input
        v-model="newHost"
        type="text"
        class="cardapio-dom__input"
        placeholder="cardapio.restaurante.com.br"
      />
      <label class="cardapio-dom__primary">
        <input v-model="newPrimary" type="checkbox" />
        <span>Principal</span>
      </label>
      <button type="submit" class="cardapio-dom__add" :disabled="creating || !newHost.trim()">
        {{ creating ? 'Adicionando...' : 'Adicionar' }}
      </button>
    </form>

    <p v-if="!store.domains.length" class="cardapio-dom__empty">
      Nenhum dominio proprio cadastrado. O acesso por subdominio segue funcionando.
    </p>

    <ul v-else class="cardapio-dom__list">
      <li v-for="domain in store.domains" :key="domain.host" class="cardapio-dom__item">
        <span class="cardapio-dom__host">{{ domain.host }}</span>
        <span v-if="domain.isPrimary" class="cardapio-dom__badge">Principal</span>
        <button
          type="button"
          class="cardapio-dom__remove"
          :disabled="busyHost === domain.host"
          @click="remove(domain)"
        >
          Remover
        </button>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.cardapio-dom {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.cardapio-dom__note {
  padding: 1rem 1.1rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--primary) / 0.06);
}

.cardapio-dom__note-title {
  display: block;
  font-size: 0.92rem;
  color: var(--text-main);
  margin-bottom: 0.4rem;
}

.cardapio-dom__note-text {
  font-size: 0.86rem;
  color: var(--text-muted);
  margin-bottom: 0.4rem;
}

.cardapio-dom__note-list {
  margin-left: 1.1rem;
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  font-size: 0.86rem;
  color: var(--text-main);
}

.cardapio-dom__note-list code {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.82rem;
  background: rgb(var(--surface-2) / 0.8);
  padding: 0.05rem 0.35rem;
  border-radius: 0.35rem;
}

.cardapio-dom__create {
  display: flex;
  gap: 0.6rem;
  align-items: center;
  flex-wrap: wrap;
}

.cardapio-dom__input {
  flex: 1;
  min-width: 200px;
  padding: 0.55rem 0.7rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2) / 0.6);
  color: var(--text-main);
  font-size: 0.9rem;
}

.cardapio-dom__input:focus {
  outline: none;
  border-color: rgb(var(--ring));
  box-shadow: 0 0 0 3px rgb(var(--ring) / 0.18);
}

.cardapio-dom__primary {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  font-size: 0.86rem;
  color: var(--text-main);
}

.cardapio-dom__add {
  border: none;
  color: rgb(var(--surface));
  background: linear-gradient(135deg, rgb(var(--primary)), rgb(var(--primary-600)));
  padding: 0.55rem 1.1rem;
  border-radius: var(--radius-sm);
  font-weight: 600;
  font-size: 0.88rem;
  cursor: pointer;
}

.cardapio-dom__add:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.cardapio-dom__empty {
  color: var(--text-muted);
  font-size: 0.88rem;
}

.cardapio-dom__list {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  list-style: none;
}

.cardapio-dom__item {
  display: flex;
  align-items: center;
  gap: 0.7rem;
  padding: 0.6rem 0.85rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.6);
}

.cardapio-dom__host {
  flex: 1;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.88rem;
  color: var(--text-main);
}

.cardapio-dom__badge {
  font-size: 0.72rem;
  font-weight: 700;
  padding: 0.15rem 0.5rem;
  border-radius: 999px;
  background: rgb(var(--primary) / 0.16);
  color: rgb(var(--primary));
}

.cardapio-dom__remove {
  border: 1px solid rgb(var(--danger) / 0.4);
  background: transparent;
  color: rgb(var(--danger));
  padding: 0.4rem 0.7rem;
  border-radius: var(--radius-sm);
  font-size: 0.82rem;
  font-weight: 600;
  cursor: pointer;
}

.cardapio-dom__remove:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}
</style>
