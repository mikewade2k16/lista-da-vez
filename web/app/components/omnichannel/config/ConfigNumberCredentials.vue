<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import { setInstanceCredentials } from '~/domain/omnichannel/config-api'

// Credencial do número — WRITE-ONLY (espelho de ConfigAiKeys). SEGURANCA: a chave crua
// NUNCA volta do back, nunca entra em log nem em localStorage; só o status {set,last4}.
// O back cifra via secretbox. Vazio NAO limpa (o endpoint rejeita apiKey vazio —
// needsWiring): por isso a tela só oferece "Salvar", nunca um "Limpar" que falharia mudo.
const props = defineProps<{
  instanceName: string
  initialSet: boolean
  disabled?: boolean
}>()

const emit = defineEmits<{ saved: [] }>()

const auth = useAuthStore()
const ui = useUiStore()
const runtimeConfig = useRuntimeConfig()
const api = createApiRequest(runtimeConfig, () => auth.accessToken)

// Status mascarado: o boolean vem da instancia (hasEvolutionApiKey); o last4 só existe
// após gravar nesta sessão (o GET da instancia não devolve last4).
const isSet = ref(props.initialSet)
const last4 = ref('')
const draft = ref('')
const busy = ref(false)

const canSave = computed(() => !props.disabled && !busy.value && draft.value.trim().length > 0)

// Trocar de número DESCARTA o rascunho não salvo (senão a chave do número A vai parar no
// B) e re-hidrata o status do novo número (precedente ConfigAiKeys).
watch(
  () => props.instanceName,
  () => {
    draft.value = ''
    isSet.value = props.initialSet
    last4.value = ''
  },
)
watch(
  () => props.initialSet,
  (value) => {
    if (!draft.value) isSet.value = value
  },
)

async function save(): Promise<void> {
  if (!canSave.value) return
  busy.value = true
  try {
    const status = await setInstanceCredentials(api, props.instanceName, draft.value.trim())
    isSet.value = status.set
    last4.value = status.last4
    draft.value = ''
    ui.success('Credencial do número salva.')
    emit('saved')
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível salvar a credencial.'))
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="cfg-cred">
    <div class="cfg-cred__head">
      <span class="cfg-cred__title">Credencial (API key do provider)</span>
      <span class="cfg-cred__status" :class="isSet ? 'is-set' : 'is-unset'">
        <template v-if="isSet">
          configurada
          <template v-if="last4">&bull;&bull;&bull;&bull;{{ last4 }}</template>
        </template>
        <template v-else>não configurada</template>
      </span>
    </div>
    <p class="cfg-cred__hint">
      A chave fica cifrada no servidor — nunca no navegador. Aqui só aparece o status mascarado;
      para trocar, digite uma nova chave.
    </p>
    <div class="cfg-cred__row">
      <input
        v-model="draft"
        class="cfg-input"
        type="password"
        autocomplete="off"
        :placeholder="isSet ? 'Digite para trocar a chave' : 'Cole a chave da API'"
        :disabled="disabled || busy"
      />
      <AppPanelButton variant="ghost" :disabled="!canSave" @click="save">Salvar</AppPanelButton>
    </div>
  </div>
</template>

<style scoped>
.cfg-cred {
  display: grid;
  gap: 0.4rem;
}

.cfg-cred__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}

.cfg-cred__title {
  font-size: 0.8rem;
  font-weight: 700;
  color: rgb(var(--text));
}

.cfg-cred__status {
  font-size: 0.74rem;
  font-weight: 700;
}

.cfg-cred__status.is-set {
  color: rgb(var(--success));
}

.cfg-cred__status.is-unset {
  color: rgb(var(--muted));
}

.cfg-cred__hint {
  margin: 0;
  color: rgb(var(--muted));
  font-size: 0.76rem;
  line-height: 1.35;
}

.cfg-cred__row {
  display: flex;
  gap: 0.5rem;
  align-items: center;
}

.cfg-input {
  flex: 1;
  min-width: 0;
  min-height: 36px;
  padding: 0 0.75rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface));
  color: rgb(var(--text));
  font-size: 0.82rem;
}

.cfg-input:focus {
  outline: none;
  border-color: rgb(var(--primary) / 0.6);
}

.cfg-input:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}
</style>
