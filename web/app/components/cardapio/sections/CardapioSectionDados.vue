<script setup lang="ts">
import CardapioMoneyInput from '~/components/cardapio/CardapioMoneyInput.vue'
import { useCardapioEditor } from '~/composables/useCardapioEditor'

const { form, dirty, savingDados, uploading, saveDados, uploadAndApply, addHour, removeHour } =
  useCardapioEditor()

function onUpload(event: Event, target: 'logoUrl' | 'bannerUrl') {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) {
    return
  }
  void uploadAndApply(file, (url) => {
    form.value[target] = url
  })
  input.value = ''
}
</script>

<template>
  <div class="cardapio-dados">
    <section class="cardapio-dados__card">
      <h3 class="cardapio-dados__heading">Identidade</h3>
      <div class="cardapio-dados__grid">
        <label class="cardapio-dados__field">
          <span class="cardapio-dados__label">Nome</span>
          <input v-model="form.name" type="text" class="cardapio-dados__input" />
        </label>
        <label class="cardapio-dados__field">
          <span class="cardapio-dados__label">Slug</span>
          <input :value="form.slug" type="text" class="cardapio-dados__input" readonly disabled />
          <span class="cardapio-dados__hint">
            O slug e o identificador na URL e nao pode ser alterado depois de criado.
          </span>
        </label>
        <label class="cardapio-dados__field cardapio-dados__field--full">
          <span class="cardapio-dados__label">Chamada (tagline)</span>
          <input v-model="form.tagline" type="text" class="cardapio-dados__input" />
        </label>
        <label class="cardapio-dados__field cardapio-dados__field--full">
          <span class="cardapio-dados__label">Descricao</span>
          <textarea v-model="form.description" rows="3" class="cardapio-dados__input"></textarea>
        </label>
        <div class="cardapio-dados__field">
          <span class="cardapio-dados__label">Logo</span>
          <div class="cardapio-dados__media">
            <img v-if="form.logoUrl" :src="form.logoUrl" alt="Logo" class="cardapio-dados__thumb" />
            <input
              v-model="form.logoUrl"
              type="text"
              class="cardapio-dados__input"
              placeholder="URL"
            />
            <label class="cardapio-dados__upload">
              <input type="file" accept="image/*" hidden @change="onUpload($event, 'logoUrl')" />
              {{ uploading ? '...' : 'Enviar' }}
            </label>
          </div>
        </div>
        <div class="cardapio-dados__field">
          <span class="cardapio-dados__label">Banner</span>
          <div class="cardapio-dados__media">
            <img
              v-if="form.bannerUrl"
              :src="form.bannerUrl"
              alt="Banner"
              class="cardapio-dados__thumb"
            />
            <input
              v-model="form.bannerUrl"
              type="text"
              class="cardapio-dados__input"
              placeholder="URL"
            />
            <label class="cardapio-dados__upload">
              <input type="file" accept="image/*" hidden @change="onUpload($event, 'bannerUrl')" />
              {{ uploading ? '...' : 'Enviar' }}
            </label>
          </div>
        </div>
      </div>
    </section>

    <section class="cardapio-dados__card">
      <h3 class="cardapio-dados__heading">Contato</h3>
      <div class="cardapio-dados__grid">
        <label class="cardapio-dados__field">
          <span class="cardapio-dados__label">WhatsApp</span>
          <input v-model="form.whatsapp" type="text" class="cardapio-dados__input" />
        </label>
        <label class="cardapio-dados__field">
          <span class="cardapio-dados__label">Telefone</span>
          <input v-model="form.phone" type="text" class="cardapio-dados__input" />
        </label>
        <label class="cardapio-dados__field">
          <span class="cardapio-dados__label">E-mail</span>
          <input v-model="form.email" type="email" class="cardapio-dados__input" />
        </label>
        <label class="cardapio-dados__field">
          <span class="cardapio-dados__label">Instagram</span>
          <input v-model="form.instagram" type="text" class="cardapio-dados__input" />
        </label>
      </div>
    </section>

    <section class="cardapio-dados__card">
      <h3 class="cardapio-dados__heading">Endereco</h3>
      <div class="cardapio-dados__grid">
        <label class="cardapio-dados__field cardapio-dados__field--full">
          <span class="cardapio-dados__label">Rua</span>
          <input v-model="form.address.street" type="text" class="cardapio-dados__input" />
        </label>
        <label class="cardapio-dados__field">
          <span class="cardapio-dados__label">Bairro</span>
          <input v-model="form.address.neighborhood" type="text" class="cardapio-dados__input" />
        </label>
        <label class="cardapio-dados__field">
          <span class="cardapio-dados__label">Cidade</span>
          <input v-model="form.address.city" type="text" class="cardapio-dados__input" />
        </label>
        <label class="cardapio-dados__field">
          <span class="cardapio-dados__label">Estado (UF)</span>
          <input
            v-model="form.address.state"
            type="text"
            maxlength="2"
            class="cardapio-dados__input"
          />
        </label>
        <label class="cardapio-dados__field">
          <span class="cardapio-dados__label">CEP</span>
          <input v-model="form.address.zip" type="text" class="cardapio-dados__input" />
        </label>
      </div>
    </section>

    <section class="cardapio-dados__card">
      <div class="cardapio-dados__row-head">
        <h3 class="cardapio-dados__heading">Horarios</h3>
        <button type="button" class="cardapio-dados__small" @click="addHour">Adicionar</button>
      </div>
      <p v-if="!form.hours.length" class="cardapio-dados__hint">Nenhum horario configurado.</p>
      <div v-for="(hour, index) in form.hours" :key="index" class="cardapio-dados__hour">
        <input
          v-model="hour.days"
          type="text"
          class="cardapio-dados__input"
          placeholder="Seg a Sex"
        />
        <input
          v-model="hour.hours"
          type="text"
          class="cardapio-dados__input"
          placeholder="11h as 23h"
        />
        <button type="button" class="cardapio-dados__remove" @click="removeHour(index)">
          Remover
        </button>
      </div>
    </section>

    <section class="cardapio-dados__card">
      <h3 class="cardapio-dados__heading">Entrega e retirada</h3>
      <div class="cardapio-dados__toggles">
        <label class="cardapio-dados__toggle">
          <input v-model="form.settings.deliveryEnabled" type="checkbox" />
          <span>Entrega habilitada</span>
        </label>
        <label class="cardapio-dados__toggle">
          <input v-model="form.settings.pickupEnabled" type="checkbox" />
          <span>Retirada habilitada</span>
        </label>
        <label class="cardapio-dados__toggle">
          <input v-model="form.settings.dineInEnabled" type="checkbox" />
          <span>Consumo no local</span>
        </label>
      </div>
      <div class="cardapio-dados__grid">
        <label class="cardapio-dados__field">
          <span class="cardapio-dados__label">Taxa de entrega</span>
          <CardapioMoneyInput v-model="form.settings.deliveryFeeCents" />
        </label>
        <label class="cardapio-dados__field">
          <span class="cardapio-dados__label">Pedido minimo</span>
          <CardapioMoneyInput v-model="form.settings.minOrderCents" />
        </label>
        <label class="cardapio-dados__field">
          <span class="cardapio-dados__label">Frete gratis acima de</span>
          <CardapioMoneyInput v-model="form.settings.freeDeliveryAboveCents" />
        </label>
      </div>
    </section>

    <section class="cardapio-dados__card">
      <h3 class="cardapio-dados__heading">Tema (JSON)</h3>
      <p class="cardapio-dados__hint">
        Tokens livres usados pelo front publico. Deve ser um objeto JSON.
      </p>
      <textarea
        v-model="form.theme"
        rows="6"
        class="cardapio-dados__input cardapio-dados__mono"
      ></textarea>
    </section>

    <footer class="cardapio-dados__footer">
      <span v-if="dirty" class="cardapio-dados__dirty">Alteracoes nao salvas</span>
      <button
        type="button"
        class="cardapio-dados__save"
        :disabled="savingDados || !dirty"
        @click="saveDados"
      >
        <span v-if="savingDados" class="cardapio-dados__spinner" aria-hidden="true"></span>
        {{ savingDados ? 'Salvando...' : 'Salvar dados' }}
      </button>
    </footer>
  </div>
</template>

<style scoped>
.cardapio-dados {
  display: flex;
  flex-direction: column;
  gap: 1.1rem;
}

.cardapio-dados__card {
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.6);
  padding: 1.1rem 1.25rem;
}

.cardapio-dados__heading {
  font-size: 0.95rem;
  font-weight: 700;
  color: var(--text-main);
  margin-bottom: 0.85rem;
}

.cardapio-dados__row-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 0.6rem;
}

.cardapio-dados__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.85rem;
}

.cardapio-dados__field {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
  min-width: 0;
}

.cardapio-dados__field--full {
  grid-column: 1 / -1;
}

.cardapio-dados__label {
  font-size: 0.8rem;
  font-weight: 600;
  color: var(--text-main);
}

.cardapio-dados__input {
  width: 100%;
  padding: 0.55rem 0.7rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2) / 0.6);
  color: var(--text-main);
  font-size: 0.9rem;
  font-family: inherit;
}

.cardapio-dados__input:focus {
  outline: none;
  border-color: rgb(var(--ring));
  box-shadow: 0 0 0 3px rgb(var(--ring) / 0.18);
}

.cardapio-dados__mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.82rem;
}

.cardapio-dados__media {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.cardapio-dados__thumb {
  width: 2.4rem;
  height: 2.4rem;
  border-radius: var(--radius-sm);
  object-fit: cover;
  border: 1px solid var(--line-soft);
}

.cardapio-dados__upload {
  flex-shrink: 0;
  padding: 0.5rem 0.7rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2) / 0.8);
  color: var(--text-main);
  font-size: 0.82rem;
  font-weight: 600;
  cursor: pointer;
}

.cardapio-dados__toggles {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
  margin-bottom: 0.85rem;
}

.cardapio-dados__toggle {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  font-size: 0.88rem;
  color: var(--text-main);
}

.cardapio-dados__hour {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) auto;
  gap: 0.6rem;
  align-items: center;
  margin-bottom: 0.55rem;
}

.cardapio-dados__remove,
.cardapio-dados__small {
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

.cardapio-dados__hint {
  font-size: 0.83rem;
  color: var(--text-muted);
  margin-bottom: 0.5rem;
}

.cardapio-dados__footer {
  position: sticky;
  bottom: 0;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.85rem;
  padding-top: 0.4rem;
}

.cardapio-dados__dirty {
  font-size: 0.84rem;
  color: var(--accent-warning, rgb(var(--primary)));
}

.cardapio-dados__save {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  border: none;
  color: rgb(var(--surface));
  background: linear-gradient(135deg, rgb(var(--primary)), rgb(var(--primary-600)));
  padding: 0.6rem 1.2rem;
  border-radius: var(--radius-sm);
  font-weight: 600;
  font-size: 0.9rem;
  cursor: pointer;
}

.cardapio-dados__save:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.cardapio-dados__spinner {
  width: 0.85rem;
  height: 0.85rem;
  border-radius: 999px;
  border: 2px solid rgb(var(--surface) / 0.5);
  border-top-color: rgb(var(--surface));
  animation: cardapio-dados-spin 0.7s linear infinite;
}

@keyframes cardapio-dados-spin {
  to {
    transform: rotate(360deg);
  }
}

@media (max-width: 720px) {
  .cardapio-dados__grid {
    grid-template-columns: 1fr;
  }
}
</style>
