<script setup lang="ts">
import { useFiscalCustomers } from '../../composables/useFinanceRecords'
import { customerVersionSummary, formatRecordDate, maskDocument } from '../../utils/finance-records'

// Clientes (tomadores) salvos pelo chat. Edição manual cria nova versão imutável;
// o CPF/CNPJ identifica o cliente e não muda (documento diferente = outro cliente).
const customersState = useFiscalCustomers()
const { customers, versions, editingId, draft, saving, loading, error, status } = customersState
function money(value: number | null): string {
  return typeof value === 'number'
    ? `R$ ${value.toLocaleString('pt-BR', { minimumFractionDigits: 2 })}`
    : 'sem valor'
}
</script>

<template>
  <details class="finance-records">
    <summary>
      Clientes salvos
      <span class="finance-records__summary-meta">
        {{ loading ? 'carregando' : `${customers.length} clientes` }}
      </span>
    </summary>
    <p class="finance-records__intro">
      Clientes guardados quando você completa uma nota pelo chat. Na próxima nota para o mesmo
      cliente, CPF/CNPJ, serviço e valor são reaproveitados. Toda alteração fica no histórico.
    </p>
    <p v-if="error" class="finance-records__status finance-records__status--error" role="alert">
      {{ error }}
    </p>
    <p v-if="status" class="finance-records__status" role="status">{{ status }}</p>
    <p v-if="!loading && !error && customers.length === 0" class="finance-records__status">
      Nenhum cliente salvo ainda. Peça no chat: “faz uma nota de 500 pro João”.
    </p>
    <ul v-if="customers.length" class="finance-records__list">
      <li v-for="customer in customers" :key="customer.id" class="finance-records__item">
        <div class="finance-records__item-head">
          <span class="finance-records__title">{{ customer.name }}</span>
          <span class="finance-records__meta">
            CPF/CNPJ {{ maskDocument(customer.document) }} · v{{ customer.version }} · usado em
            {{ formatRecordDate(customer.lastUsedAt) }}
          </span>
        </div>
        <template v-if="editingId !== customer.id">
          <span class="finance-records__line">
            {{ customer.serviceDescription || 'Sem descrição de serviço' }} ·
            {{ money(customer.amount) }}
          </span>
          <div class="finance-records__actions">
            <button
              type="button"
              class="finance-records__button"
              @click="customersState.edit(customer)"
            >
              Editar
            </button>
            <button
              type="button"
              class="finance-records__button"
              @click="customersState.toggleVersions(customer.id)"
            >
              {{ versions[customer.id] ? 'Ocultar histórico' : 'Histórico' }}
            </button>
          </div>
        </template>
        <form v-else @submit.prevent="customersState.save(customer)">
          <fieldset class="finance-records__form" :disabled="saving">
            <label>
              Nome
              <input v-model.trim="draft.name" maxlength="300" required />
            </label>
            <label>
              Valor habitual (R$)
              <input
                v-model.number="draft.amount"
                type="number"
                min="0.01"
                step="0.01"
                inputmode="decimal"
              />
            </label>
            <label class="finance-records__field--wide">
              Serviço
              <textarea v-model="draft.serviceDescription" maxlength="4000" rows="2"></textarea>
            </label>
            <div class="finance-records__form-actions">
              <button type="submit" class="finance-records__button">
                {{ saving ? 'Salvando...' : 'Salvar nova versão' }}
              </button>
              <button type="button" class="finance-records__button" @click="customersState.cancel">
                Cancelar
              </button>
            </div>
          </fieldset>
        </form>
        <ul v-if="versions[customer.id]" class="finance-records__history">
          <li v-for="version in versions[customer.id]" :key="version.version">
            {{ customerVersionSummary(version) }}
          </li>
        </ul>
      </li>
    </ul>
  </details>
</template>

<style src="../../assets/finance-records.css"></style>
