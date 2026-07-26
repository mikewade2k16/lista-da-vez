<script setup lang="ts">
import { ref } from 'vue'
import AppPasswordInput from '~/components/ui/AppPasswordInput.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import {
  INTELLIGENCE_AI_PROVIDER_OPTIONS,
  type IntelligenceAIProvider,
  type IntelligenceCredential,
  type IntelligenceCredentialWriteInput,
} from '~/domain/customer-intelligence/agent-admin-types'

const props = defineProps<{
  credentials: IntelligenceCredential[]
  busyKey: string
  saveCredential: (input: IntelligenceCredentialWriteInput) => Promise<boolean>
  revokeCredential: (credentialId: string) => Promise<boolean>
}>()

const provider = ref<IntelligenceAIProvider>('openai')
const label = ref('')
const apiKey = ref('')
const rotationKeys = ref<Record<string, string>>({})
const validationError = ref('')

function setProvider(value: string): void {
  if (value === 'openai' || value === 'gemini' || value === 'glm') provider.value = value
}

function validate(name: string, secret: string): string {
  const normalizedName = name.trim()
  const normalizedSecret = secret.trim()
  if (!normalizedName || normalizedName.length > 160) {
    return 'Informe um nome com ate 160 caracteres.'
  }
  if (normalizedSecret.length < 8 || secret.length > 20_000) {
    return 'A chave deve ter entre 8 e 20.000 caracteres.'
  }
  return ''
}

async function createCredential(): Promise<void> {
  validationError.value = validate(label.value, apiKey.value)
  if (validationError.value) return
  const input: IntelligenceCredentialWriteInput = {
    provider: provider.value,
    label: label.value.trim(),
    apiKey: apiKey.value,
  }
  try {
    const saved = await props.saveCredential(input)
    if (saved) label.value = ''
  } finally {
    apiKey.value = ''
    input.apiKey = ''
  }
}

async function rotateCredential(item: IntelligenceCredential): Promise<void> {
  const secret = rotationKeys.value[item.id] ?? ''
  validationError.value = validate(item.label, secret)
  if (validationError.value) return
  const input: IntelligenceCredentialWriteInput = {
    provider: item.provider,
    label: item.label,
    apiKey: secret,
  }
  try {
    await props.saveCredential(input)
  } finally {
    rotationKeys.value[item.id] = ''
    input.apiKey = ''
  }
}

async function revoke(item: IntelligenceCredential): Promise<void> {
  if (!window.confirm(`Revogar a credencial "${item.label}"?`)) return
  await props.revokeCredential(item.id)
}
</script>

<template>
  <div class="credentials-manager">
    <form class="credential-form" @submit.prevent="createCredential">
      <header>
        <div>
          <strong>Nova credencial</strong>
          <span>A chave e enviada uma vez e nunca e reidratada pelo painel.</span>
        </div>
        <button type="submit" :disabled="Boolean(busyKey)">
          {{ busyKey.startsWith('credential:') ? 'Gravando...' : 'Gravar credencial' }}
        </button>
      </header>
      <div class="credential-form__grid">
        <AppSelectField
          :model-value="provider"
          label="Provider"
          :options="[...INTELLIGENCE_AI_PROVIDER_OPTIONS]"
          :disabled="Boolean(busyKey)"
          @update:model-value="setProvider"
        />
        <label>
          <span>Nome de referencia</span>
          <input
            v-model="label"
            maxlength="160"
            autocomplete="off"
            placeholder="Ex.: OpenAI producao"
            :disabled="Boolean(busyKey)"
          />
        </label>
        <label class="credential-form__secret">
          <span>Chave secreta</span>
          <AppPasswordInput
            v-model="apiKey"
            autocomplete="new-password"
            placeholder="Cole a chave uma unica vez"
            :disabled="Boolean(busyKey)"
          />
        </label>
      </div>
      <p v-if="validationError" class="credential-form__error">{{ validationError }}</p>
    </form>

    <CustomerIntelligenceStatus
      v-if="credentials.length === 0"
      title="Nenhuma credencial gravada"
      empty
      empty-text="Cadastre uma chave write-only para os providers que exigirem autenticacao."
    />
    <div v-else class="credential-list">
      <article v-for="item in credentials" :key="item.id">
        <header>
          <div>
            <strong>{{ item.label }}</strong>
            <span>{{ item.provider }}</span>
          </div>
          <span class="credential-list__mask">
            {{ item.secret.set ? `•••• ${item.secret.last4}` : 'Sem segredo ativo' }}
          </span>
        </header>
        <label>
          <span>Nova chave para rotacao</span>
          <AppPasswordInput
            :model-value="rotationKeys[item.id] ?? ''"
            autocomplete="new-password"
            placeholder="Nunca preenchido automaticamente"
            :disabled="Boolean(busyKey)"
            @update:model-value="rotationKeys[item.id] = $event"
          />
        </label>
        <footer>
          <button type="button" :disabled="Boolean(busyKey)" @click="rotateCredential(item)">
            {{ busyKey === `credential:${item.label}` ? 'Rotacionando...' : 'Rotacionar' }}
          </button>
          <button
            class="credential-list__revoke"
            type="button"
            :disabled="Boolean(busyKey)"
            @click="revoke(item)"
          >
            {{ busyKey === `credential:${item.id}` ? 'Revogando...' : 'Revogar' }}
          </button>
        </footer>
      </article>
    </div>

    <p class="credentials-manager__notice">
      Nome e provider identificam a credencial no contrato atual. Renomear criaria outro registro;
      por isso esta tela permite apenas criar, rotacionar pelo mesmo nome ou revogar.
    </p>
  </div>
</template>

<style scoped>
.credentials-manager,
.credential-form,
.credential-list,
.credential-list article {
  display: grid;
  gap: 0.8rem;
}

.credential-form,
.credential-list article {
  padding: 0.85rem;
  border: 1px solid rgb(var(--border) / 0.75);
  border-radius: 0.8rem;
}

.credential-form > header,
.credential-list article > header,
.credential-list footer {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.7rem;
  flex-wrap: wrap;
}

.credential-form header div,
.credential-list header div,
.credential-form label,
.credential-list label {
  display: grid;
  gap: 0.3rem;
}

.credential-form__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.65rem;
}

.credential-form__secret {
  grid-column: 1 / -1;
}

.credential-form label > span,
.credential-list label > span,
.credential-form header span,
.credential-list header span,
.credentials-manager__notice {
  color: rgb(var(--muted));
  font-size: 0.72rem;
}

.credential-form input {
  min-height: 2.5rem;
  padding: 0 0.7rem;
  border: 1px solid rgb(var(--border));
  border-radius: 0.7rem;
  background: rgb(var(--surface));
  color: inherit;
}

.credential-form button,
.credential-list button {
  min-height: 2.35rem;
  padding: 0 0.85rem;
  border: 0;
  border-radius: 999px;
  background: rgb(var(--primary));
  color: white;
  font-weight: 700;
}

.credential-form button:disabled,
.credential-list button:disabled {
  opacity: 0.55;
}

.credential-list__mask {
  padding: 0.3rem 0.55rem;
  border-radius: 999px;
  background: rgb(var(--success) / 0.1);
  color: rgb(var(--success));
  font-family: monospace;
}

.credential-list .credential-list__revoke {
  background: rgb(var(--danger) / 0.12);
  color: rgb(var(--danger));
}

.credential-form__error {
  margin: 0;
  color: rgb(var(--danger));
  font-size: 0.74rem;
}

.credentials-manager__notice {
  margin: 0;
  padding: 0.7rem;
  border: 1px solid rgb(var(--warning) / 0.3);
  border-radius: 0.7rem;
}

@media (max-width: 760px) {
  .credential-form__grid {
    grid-template-columns: 1fr;
  }
}
</style>
