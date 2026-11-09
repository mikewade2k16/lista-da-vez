<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import FiscalCredentialsForm from './FiscalCredentialsForm.vue'
import type { FiscalProfileEditor } from '../../composables/useFiscalProfile'
const props = defineProps<{ editor: FiscalProfileEditor; expanded?: boolean }>()
const emit = defineEmits<{ dirty: [value: boolean] }>()
const credentialsDirty = ref(false)
const { draft, view, loading, saving, profileDirty, portalResult, sessionResult } = props.editor
const cannotConnect = computed(
  () =>
    loading.value ||
    saving.value ||
    credentialsDirty.value ||
    profileDirty.value ||
    !view.value?.updatedAt ||
    (!view.value?.credentialsSet && view.value?.profile.portalLoginMethod !== 'govbr'),
)
const methods = [
  { value: '', label: 'Escolha a forma de acesso' },
  { value: 'portal', label: 'CPF/CNPJ e senha do Emissor Nacional' },
  { value: 'govbr', label: 'Conta gov.br' },
]
function setLoginMethod(value: unknown) {
  if (value !== '' && value !== 'portal' && value !== 'govbr') return
  draft.portalLoginMethod = value
  profileDirty.value = true
}
function setCredentialsDirty(value: boolean) {
  credentialsDirty.value = value
  emit('dirty', value)
}
function nationalPortal() {
  draft.portalUrl = 'https://www.nfse.gov.br/EmissorNacional/Login'
  draft.portalLoginMethod = 'portal'
  profileDirty.value = true
}
watch(
  () => view.value?.updatedAt,
  () => {
    void props.editor.loadPortalStatus()
  },
  { immediate: true },
)
</script>
<template>
  <details class="fiscal-profile__section" :open="expanded">
    <summary>
      Acesso ao Emissor Nacional
      <span>{{ view?.credentialsSet ? 'Acesso salvo' : 'Configurar' }}</span>
    </summary>
    <form class="fiscal-profile__form" @submit.prevent="editor.saveProfile">
      <div class="fiscal-profile__actions">
        <AppPanelButton variant="secondary" :disabled="loading || saving" @click="nationalPortal">
          Usar Emissor Nacional
        </AppPanelButton>
      </div>
      <div class="fiscal-profile__grid">
        <label class="fiscal-profile__field">
          Portal HTTPS
          <input
            v-model="draft.portalUrl"
            type="url"
            maxlength="1000"
            :disabled="loading || saving"
            @input="profileDirty = true"
          />
        </label>
        <AppSelectField
          :model-value="draft.portalLoginMethod"
          label="Forma de acesso"
          :options="methods"
          :disabled="loading || saving"
          @update:model-value="setLoginMethod"
        />
      </div>
      <div class="fiscal-profile__actions">
        <AppPanelButton v-if="profileDirty" type="submit" :disabled="loading || saving">
          Salvar forma de acesso
        </AppPanelButton>
      </div>
    </form>
    <FiscalCredentialsForm
      :key="`${editor.scopeKey.value}:${editor.reloadKey.value}`"
      :editor="editor"
      :expanded="expanded"
      @dirty="setCredentialsDirty"
    />
    <div class="fiscal-profile__actions">
      <AppPanelButton :disabled="cannotConnect" @click="editor.testPortalAccess">
        {{ saving ? 'Aguarde...' : 'Testar acesso' }}
      </AppPanelButton>
      <AppPanelButton
        v-if="view?.profile.portalLoginMethod !== 'govbr'"
        variant="secondary"
        :disabled="cannotConnect"
        @click="editor.connectPortalSession"
      >
        Manter sessão
      </AppPanelButton>
    </div>
    <p
      v-if="sessionResult && !credentialsDirty && !profileDirty"
      role="status"
      class="fiscal-profile__notice"
    >
      {{ sessionResult.message }}
      <span v-if="sessionResult.expiresAt">
        · Validade: {{ new Date(sessionResult.expiresAt).toLocaleString('pt-BR') }}
      </span>
    </p>
    <p
      v-if="portalResult && !credentialsDirty && !profileDirty"
      role="status"
      class="fiscal-profile__notice"
    >
      {{ portalResult.message }}
      <span v-if="portalResult.checkedAt">
        · {{ new Date(portalResult.checkedAt).toLocaleString('pt-BR') }}
      </span>
    </p>
  </details>
</template>
<style src="../../assets/fiscal-profile.css"></style>
