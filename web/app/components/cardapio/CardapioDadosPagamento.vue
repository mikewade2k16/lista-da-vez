<script setup lang="ts">
import { computed } from 'vue'

import CardapioMoneyInput from '~/components/cardapio/CardapioMoneyInput.vue'
import { formatBrands, parseBrands } from '~/domain/cardapio/types'
import type { RestaurantSettings } from '~/domain/cardapio/types'

// WS-B — Pagamento informativo. Edita settings.payment (sub-objeto do form de
// Dados). Salvo junto no body COMPLETO do saveDados (settings inteiro). As
// bandeiras sao digitadas por virgula e convertidas em array no modelo.
// `settings`/`payment` apontam para o MESMO objeto reativo do form do editor
// (passado por prop): mutar suas propriedades aninhadas propaga para o dirty-
// check do composable. Variaveis locais evitam o lint de mutacao de prop.
const props = defineProps<{ settings: RestaurantSettings }>()

const settings = props.settings
const payment = computed(() => settings.payment)

// v-model de bandeiras: string na UI <-> array no modelo.
const debitBrands = computed({
  get: () => formatBrands(payment.value?.debit.brands),
  set: (value: string) => {
    if (payment.value) {
      payment.value.debit.brands = parseBrands(value)
    }
  },
})

const creditBrands = computed({
  get: () => formatBrands(payment.value?.credit.brands),
  set: (value: string) => {
    if (payment.value) {
      payment.value.credit.brands = parseBrands(value)
    }
  },
})
</script>

<template>
  <section v-if="payment" class="cardapio-pay">
    <h3 class="cardapio-pay__heading">Formas de pagamento</h3>
    <p class="cardapio-pay__hint">
      Informativo: as formas escolhidas aparecem no site para o cliente. Nao alteram o checkout.
    </p>

    <div class="cardapio-pay__toggles">
      <label class="cardapio-pay__toggle">
        <input v-model="payment.cash" type="checkbox" />
        <span>Dinheiro</span>
      </label>
      <label class="cardapio-pay__toggle">
        <input v-model="payment.pix" type="checkbox" />
        <span>PIX</span>
      </label>
      <label class="cardapio-pay__toggle">
        <input v-model="payment.ticket" type="checkbox" />
        <span>Vale / ticket</span>
      </label>
      <label class="cardapio-pay__toggle">
        <input v-model="payment.debit.accepted" type="checkbox" />
        <span>Cartao de debito</span>
      </label>
      <label class="cardapio-pay__toggle">
        <input v-model="payment.credit.accepted" type="checkbox" />
        <span>Cartao de credito</span>
      </label>
    </div>

    <div class="cardapio-pay__grid">
      <label v-if="payment.debit.accepted" class="cardapio-pay__field">
        <span class="cardapio-pay__label">Bandeiras do debito</span>
        <input
          v-model="debitBrands"
          type="text"
          class="cardapio-pay__input"
          placeholder="Visa, Mastercard, Elo"
        />
        <span class="cardapio-pay__sub">Separe por virgula.</span>
      </label>
      <label v-if="payment.credit.accepted" class="cardapio-pay__field">
        <span class="cardapio-pay__label">Bandeiras do credito</span>
        <input
          v-model="creditBrands"
          type="text"
          class="cardapio-pay__input"
          placeholder="Visa, Mastercard, Elo, Amex"
        />
        <span class="cardapio-pay__sub">Separe por virgula.</span>
      </label>
      <label class="cardapio-pay__field cardapio-pay__field--full">
        <span class="cardapio-pay__label">Outras formas</span>
        <input
          v-model="payment.other"
          type="text"
          class="cardapio-pay__input"
          placeholder="Ex.: PicPay, transferencia"
        />
      </label>
    </div>

    <p class="cardapio-pay__hint">
      O pedido minimo configurado em "Entrega e retirada" tambem aparece no site.
      <span class="cardapio-pay__min">
        Atual:
        <CardapioMoneyInput v-model="settings.minOrderCents" disabled />
      </span>
    </p>
  </section>
</template>

<style scoped>
.cardapio-pay {
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.6);
  padding: 1.1rem 1.25rem;
}

.cardapio-pay__heading {
  font-size: 0.95rem;
  font-weight: 700;
  color: var(--text-main);
  margin-bottom: 0.5rem;
}

.cardapio-pay__hint {
  font-size: 0.83rem;
  color: var(--text-muted);
  margin-bottom: 0.85rem;
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.cardapio-pay__min {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  width: 9rem;
}

.cardapio-pay__toggles {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
  margin-bottom: 0.85rem;
}

.cardapio-pay__toggle {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  font-size: 0.88rem;
  color: var(--text-main);
}

.cardapio-pay__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.85rem;
}

.cardapio-pay__field {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
  min-width: 0;
}

.cardapio-pay__field--full {
  grid-column: 1 / -1;
}

.cardapio-pay__label {
  font-size: 0.8rem;
  font-weight: 600;
  color: var(--text-main);
}

.cardapio-pay__sub {
  font-size: 0.76rem;
  color: var(--text-muted);
}

.cardapio-pay__input {
  width: 100%;
  padding: 0.55rem 0.7rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2) / 0.6);
  color: var(--text-main);
  font-size: 0.9rem;
  font-family: inherit;
}

.cardapio-pay__input:focus {
  outline: none;
  border-color: rgb(var(--ring));
  box-shadow: 0 0 0 3px rgb(var(--ring) / 0.18);
}

@media (max-width: 720px) {
  .cardapio-pay__grid {
    grid-template-columns: 1fr;
  }
}
</style>
