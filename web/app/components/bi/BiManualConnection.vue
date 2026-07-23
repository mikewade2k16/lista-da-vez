<script setup lang="ts">
import { ref } from 'vue'
import { storeToRefs } from 'pinia'
import { LogIn, RefreshCw, ShieldCheck, Trash2 } from 'lucide-vue-next'

import { useBiStore } from '~/stores/bi'
import { useUiStore } from '~/stores/ui'

const biStore = useBiStore()
const ui = useUiStore()
const { loading, loggingIn, loginError, manualConfig, manualToken, hasManualToken } =
  storeToRefs(biStore)

const showSecrets = ref(false)

function inputValue(event: Event) {
  return String((event.target as HTMLInputElement | null)?.value || '')
}

async function generateToken() {
  const response = await biStore.loginPerola()
  if (!response.ok) {
    ui.error(response.message || 'Falha ao gerar token Pérola.')
    return
  }

  ui.success('Token manual da Pérola gerado para esta sessão.')
}

async function loadWithToken() {
  const response = await biStore.refreshOverview()
  if (!response.ok) {
    ui.error(response.message || 'Não foi possível carregar o BI com o token manual.')
    return
  }

  ui.success('BI Pérola atualizado com o token manual.')
}

function clearSession() {
  biStore.clearManualSession()
  ui.success('Token manual removido. A autenticação automática voltou a ser usada.')
}
</script>

<template>
  <section class="bi-manual" data-testid="bi-manual-connection">
    <header>
      <div>
        <span class="bi-manual__eyebrow">
          <ShieldCheck :size="14" aria-hidden="true" />
          Uso excepcional
        </span>
        <h3>Diagnóstico de conexão manual</h3>
        <p>
          A operação normal não exige estes dados. Use-os apenas para testar credenciais ou um token
          específico durante um diagnóstico.
        </p>
      </div>

      <span class="bi-manual__status" :data-active="hasManualToken">
        {{ hasManualToken ? 'Token manual ativo' : 'Modo automático ativo' }}
      </span>
    </header>

    <div class="bi-manual__grid">
      <label class="bi-manual__field">
        <span>dsCompanyKey</span>
        <input
          :type="showSecrets ? 'text' : 'password'"
          :value="manualConfig.companyKey"
          autocomplete="off"
          spellcheck="false"
          placeholder="Usa o backend se vazio"
          @input="biStore.updateManualConfig({ companyKey: inputValue($event) })"
        />
      </label>

      <label class="bi-manual__field">
        <span>dsCnpjEmpresa</span>
        <input
          :value="manualConfig.cnpjEmpresa"
          inputmode="numeric"
          autocomplete="off"
          spellcheck="false"
          placeholder="Usa o backend se vazio"
          @input="biStore.updateManualConfig({ cnpjEmpresa: inputValue($event) })"
        />
      </label>

      <label class="bi-manual__field">
        <span>Login</span>
        <input
          :value="manualConfig.login"
          autocomplete="username"
          spellcheck="false"
          placeholder="Opcional se houver token"
          @input="biStore.updateManualConfig({ login: inputValue($event) })"
        />
      </label>

      <label class="bi-manual__field">
        <span>Pass</span>
        <input
          :type="showSecrets ? 'text' : 'password'"
          :value="manualConfig.pass"
          autocomplete="current-password"
          placeholder="Opcional se houver token"
          @input="biStore.updateManualConfig({ pass: inputValue($event) })"
        />
      </label>

      <label class="bi-manual__field bi-manual__field--wide">
        <span>Bearer Token</span>
        <input
          :type="showSecrets ? 'text' : 'password'"
          :value="manualToken"
          autocomplete="off"
          spellcheck="false"
          placeholder="Cole um JWT ou gere pelas credenciais acima"
          @input="biStore.setManualToken(inputValue($event))"
        />
      </label>
    </div>

    <p v-if="loginError" class="bi-manual__error">{{ loginError }}</p>

    <footer>
      <button
        class="bi-manual__button bi-manual__button--primary"
        type="button"
        :disabled="loggingIn"
        @click="generateToken"
      >
        <LogIn :size="14" aria-hidden="true" />
        {{ loggingIn ? 'Gerando...' : 'Gerar token' }}
      </button>
      <button
        class="bi-manual__button"
        type="button"
        :disabled="!hasManualToken || loading"
        @click="loadWithToken"
      >
        <RefreshCw :size="14" aria-hidden="true" />
        {{ loading ? 'Carregando...' : 'Carregar com token' }}
      </button>
      <button
        class="bi-manual__button bi-manual__button--ghost"
        type="button"
        :disabled="!hasManualToken"
        @click="clearSession"
      >
        <Trash2 :size="14" aria-hidden="true" />
        Limpar sessão
      </button>
      <label class="bi-manual__secret">
        <input v-model="showSecrets" type="checkbox" />
        Mostrar segredos
      </label>
    </footer>
  </section>
</template>

<style scoped>
.bi-manual {
  display: grid;
  gap: 16px;
  padding: 18px;
  border: 1px solid color-mix(in srgb, var(--accent-warning) 28%, var(--line-soft));
  border-radius: var(--radius-lg);
  background: color-mix(in srgb, var(--accent-warning) 4%, var(--bg-panel));
}

.bi-manual > header,
.bi-manual > footer {
  display: flex;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
}

.bi-manual__eyebrow {
  display: inline-flex;
  gap: 6px;
  align-items: center;
  color: var(--accent-warning);
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.bi-manual h3 {
  margin: 4px 0;
  color: var(--text-main);
  font-size: 1rem;
}

.bi-manual header p {
  max-width: 720px;
  margin: 0;
  color: var(--text-muted);
  font-size: 0.8rem;
  line-height: 1.5;
}

.bi-manual__status {
  padding: 6px 9px;
  color: var(--text-muted);
  font-size: 0.72rem;
  font-weight: 750;
  border: 1px solid var(--line-soft);
  border-radius: 999px;
}

.bi-manual__status[data-active='true'] {
  color: var(--accent-success);
  border-color: color-mix(in srgb, var(--accent-success) 42%, var(--line-soft));
}

.bi-manual__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 11px;
}

.bi-manual__field {
  display: grid;
  gap: 5px;
  min-width: 0;
}

.bi-manual__field--wide {
  grid-column: 1 / -1;
}

.bi-manual__field span {
  color: var(--text-muted);
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 0.05em;
  text-transform: uppercase;
}

.bi-manual__field input {
  width: 100%;
  min-height: 38px;
  padding: 0 10px;
  color: var(--text-main);
  font: inherit;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: var(--bg-panel);
}

.bi-manual__field input:focus {
  outline: none;
  border-color: color-mix(in srgb, var(--accent-info) 55%, var(--line-soft));
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent-info) 14%, transparent);
}

.bi-manual__button {
  display: inline-flex;
  gap: 6px;
  align-items: center;
  justify-content: center;
  min-height: 36px;
  padding: 0 12px;
  color: var(--text-main);
  font-weight: 750;
  cursor: pointer;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: var(--bg-panel);
}

.bi-manual__button--primary {
  border-color: color-mix(in srgb, var(--accent-info) 40%, var(--line-soft));
  background: color-mix(in srgb, var(--accent-info) 12%, var(--bg-panel));
}

.bi-manual__button--ghost {
  background: transparent;
}

.bi-manual__button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.bi-manual__secret {
  display: inline-flex;
  gap: 6px;
  align-items: center;
  margin-left: auto;
  color: var(--text-muted);
  font-size: 0.76rem;
}

.bi-manual__error {
  margin: 0;
  padding: 10px;
  color: rgb(var(--danger));
  border: 1px solid color-mix(in srgb, rgb(var(--danger)) 30%, var(--line-soft));
  border-radius: var(--radius-sm);
  background: color-mix(in srgb, rgb(var(--danger)) 7%, var(--bg-panel));
}

@media (max-width: 700px) {
  .bi-manual__grid {
    grid-template-columns: 1fr;
  }

  .bi-manual__field--wide {
    grid-column: auto;
  }

  .bi-manual__button {
    flex: 1 1 10rem;
  }

  .bi-manual__secret {
    flex-basis: 100%;
    margin-left: 0;
  }
}
</style>
