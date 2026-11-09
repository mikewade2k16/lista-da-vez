<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import { confirmFiscalAction } from '../../utils/fiscal-confirm'
import { useFiscalProfile } from '../../composables/useFiscalProfile'
import type { FiscalProfile } from '../../types/fiscal'
import FiscalAccessPanel from './FiscalAccessPanel.vue'
import FiscalTemplatesEditor from './FiscalTemplatesEditor.vue'
import FiscalCustomersPanel from './FiscalCustomersPanel.vue'

const emit = defineEmits<{ dirty: [value: boolean] }>()
const editor = useFiscalProfile()
const {
  enabled,
  account,
  loading,
  saving,
  error,
  message,
  view,
  draft,
  profileDirty,
  templateDirty,
} = editor
const issuerOptions = [
  { value: '', label: 'Não informado' },
  { value: 'mei', label: 'MEI' },
  { value: 'company', label: 'Empresa (não MEI)' },
  { value: 'individual', label: 'Pessoa física / autônomo' },
]
const credentialsDirty = ref(false)
const unsaved = computed(() => profileDirty.value || templateDirty.value || credentialsDirty.value)
watch(unsaved, (value) => emit('dirty', value), { immediate: true })
const fields: {
  key: Exclude<keyof FiscalProfile, 'issuerType' | 'portalLoginMethod' | 'portalUrl'>
  label: string
  type?: string
  max?: number
}[] = [
  { key: 'legalName', label: 'Nome / razão social' },
  { key: 'document', label: 'CPF / CNPJ', max: 18 },
  { key: 'taxRegime', label: 'Regime tributário informado' },
  { key: 'municipalRegistration', label: 'Inscrição municipal (se aplicável)' },
  { key: 'municipality', label: 'Município' },
  { key: 'municipalityCode', label: 'Código IBGE do município (opcional)', max: 7 },
  { key: 'state', label: 'UF', max: 2 },
  { key: 'email', label: 'E-mail fiscal', type: 'email', max: 254 },
  { key: 'postalCode', label: 'CEP', max: 9 },
  { key: 'street', label: 'Logradouro' },
  { key: 'number', label: 'Número' },
  { key: 'complement', label: 'Complemento (opcional)' },
  { key: 'district', label: 'Bairro' },
]
async function mayDiscard() {
  return (
    !unsaved.value ||
    (await confirmFiscalAction(
      'Descartar as alterações fiscais ainda não salvas?',
      () => editor.scopeKey.value,
    ))
  )
}
async function reload() {
  if (await mayDiscard()) await editor.load()
}
function beforeUnload(event: BeforeUnloadEvent) {
  if (!unsaved.value) return
  event.preventDefault()
  event.returnValue = ''
}
onBeforeRouteLeave(() => mayDiscard())
onMounted(() => window.addEventListener('beforeunload', beforeUnload))
onBeforeUnmount(() => {
  window.removeEventListener('beforeunload', beforeUnload)
  emit('dirty', false)
})
</script>

<template>
  <article v-if="enabled" id="fiscal" class="settings-card fiscal-profile">
    <header class="settings-card__header">
      <h3 class="settings-card__title">Dados fiscais e NFS-e</h3>
      <p class="settings-card__text">
        Seu cadastro na conta {{ account.activeAccount?.name }}. Preencha uma vez e reutilize nos
        próximos serviços.
      </p>
    </header>
    <p class="fiscal-profile__notice">
      Cadastro e templates disponíveis. A emissão de notas será habilitada em uma próxima etapa.
    </p>
    <p v-if="loading" role="status">Carregando seu cadastro fiscal...</p>
    <p v-if="error" role="alert" class="fiscal-profile__error">{{ error }}</p>
    <p v-if="message" role="status" class="fiscal-profile__success">{{ message }}</p>
    <div v-if="error" class="fiscal-profile__actions">
      <AppPanelButton :disabled="loading || saving" variant="secondary" @click="reload">
        Recarregar dados
      </AppPanelButton>
    </div>
    <fieldset :disabled="loading || saving || !view" class="fiscal-profile__fieldset">
      <details class="fiscal-profile__section">
        <summary>
          Dados do emitente
          <span>
            {{
              view?.updatedAt
                ? view.missingFields.length
                  ? 'Cadastro incompleto'
                  : 'Cadastrado'
                : 'Não cadastrado'
            }}
          </span>
        </summary>
        <p v-if="view?.missingFields.length" class="fiscal-profile__hint">
          Complete quando disponíveis: {{ view.missingFields.join(', ') }}.
        </p>
        <form class="fiscal-profile__form" @submit.prevent="editor.saveProfile">
          <div class="fiscal-profile__grid">
            <label v-for="field in fields" :key="field.key" class="fiscal-profile__field">
              <span>{{ field.label }}</span>
              <input
                v-model="draft[field.key]"
                :type="field.type || 'text'"
                :maxlength="field.max || 240"
                @input="profileDirty = true"
              />
            </label>
            <AppSelectField
              v-model="draft.issuerType"
              label="Tipo de emitente"
              :options="issuerOptions"
              :disabled="loading || saving"
              @change="profileDirty = true"
            />
          </div>
          <p class="fiscal-profile__hint">
            Informe os dados do responsável fiscal. O cadastro não verifica enquadramento ou
            tributação.
          </p>
          <p v-if="view?.credentialsSet" class="fiscal-profile__hint">
            Alterar o endereço do portal remove o acesso salvo para o endereço anterior.
          </p>
          <div class="fiscal-profile__actions">
            <AppPanelButton type="submit">
              {{ saving ? 'Salvando...' : 'Salvar dados fiscais' }}
            </AppPanelButton>
            <span v-if="profileDirty" role="status">Alterações não salvas</span>
          </div>
        </form>
      </details>
      <FiscalAccessPanel
        :key="`${editor.scopeKey.value}:${editor.reloadKey.value}`"
        :editor="editor"
        @dirty="credentialsDirty = $event"
      />
      <FiscalTemplatesEditor :key="`templates:${editor.scopeKey.value}`" :editor="editor" />
      <FiscalCustomersPanel :key="`customers:${editor.scopeKey.value}`" />
    </fieldset>
  </article>
</template>

<style src="../../assets/fiscal-profile.css"></style>
