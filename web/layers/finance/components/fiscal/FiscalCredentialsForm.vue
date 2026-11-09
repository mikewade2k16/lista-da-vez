<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import { confirmFiscalAction } from '../../utils/fiscal-confirm'
import type { FiscalProfileEditor } from '../../composables/useFiscalProfile'
const props = defineProps<{ editor: FiscalProfileEditor; expanded?: boolean }>()
const emit = defineEmits<{ dirty: [value: boolean] }>()
const { view, profileDirty, saving } = props.editor
const portalAccessUrl = computed(() => {
  const profile = view.value?.profile
  if (!profile?.portalUrl) return ''
  if (profile.portalLoginMethod === 'govbr') {
    const host = new URL(profile.portalUrl).hostname.toLowerCase()
    if (host === 'www.nfse.gov.br' || host === 'nfse.gov.br')
      return 'https://www.nfse.gov.br/EmissorNacional/AcessoGovBR/AcessarGovBR'
  }
  return profile.portalUrl
})
const username = ref('')
const password = ref('')
const canSave = computed(
  () =>
    Boolean(view.value?.updatedAt && view.value?.profile.portalUrl) &&
    !profileDirty.value &&
    view.value?.profile.portalLoginMethod !== 'govbr',
)
watch([username, password], () => emit('dirty', Boolean(username.value || password.value)))
async function save() {
  if (await props.editor.saveCredentials(username.value, password.value)) {
    username.value = ''
    password.value = ''
  }
}
async function remove() {
  if (
    await confirmFiscalAction(
      'Remover somente o acesso ao portal salvo neste perfil?',
      () => props.editor.scopeKey.value,
    )
  )
    await props.editor.deleteCredentials()
}
onBeforeUnmount(() => emit('dirty', false))
</script>
<template>
  <details class="fiscal-profile__section" :open="expanded">
    <summary>
      Credenciais protegidas
      <span>
        {{
          view?.profile.portalLoginMethod === 'govbr'
            ? 'gov.br — entrada no navegador'
            : view?.credentialsSet
              ? 'Acesso salvo'
              : 'Não cadastrado'
        }}
      </span>
    </summary>
    <p v-if="view?.profile.portalLoginMethod === 'govbr'" class="fiscal-profile__notice">
      Seu acesso usa gov.br. Entre diretamente no navegador e conclua as confirmações de segurança.
      Não informe a senha gov.br neste formulário.
    </p>
    <a
      v-if="view?.profile.portalUrl"
      :href="portalAccessUrl"
      target="_blank"
      rel="noopener noreferrer"
    >
      Abrir portal para entrar
    </a>
    <p v-if="view?.profile.portalLoginMethod !== 'govbr'" class="fiscal-profile__hint">
      O acesso fica protegido no seu perfil. A senha não será exibida novamente. Salvar não realiza
      login no portal.
    </p>
    <p
      v-if="!canSave && view?.profile.portalLoginMethod !== 'govbr'"
      class="fiscal-profile__notice"
    >
      Salve o endereço do portal nos dados do emitente antes de cadastrar ou alterar o acesso.
    </p>
    <form
      v-if="view?.profile.portalLoginMethod !== 'govbr'"
      class="fiscal-profile__form"
      @submit.prevent="save"
    >
      <fieldset :disabled="!canSave || saving" class="fiscal-profile__fieldset">
        <div class="fiscal-profile__grid">
          <label class="fiscal-profile__field">
            <span>CPF/CNPJ ou usuário do portal</span>
            <input v-model="username" autocomplete="off" maxlength="320" required />
          </label>
          <label class="fiscal-profile__field">
            <span>Senha do portal</span>
            <input
              v-model="password"
              type="password"
              autocomplete="new-password"
              maxlength="4096"
              required
            />
          </label>
        </div>
        <p v-if="view?.credentialsSet" class="fiscal-profile__hint">
          Preencha usuário e senha apenas para substituir o acesso já salvo.
        </p>
        <div class="fiscal-profile__actions">
          <AppPanelButton type="submit">
            {{ saving ? 'Salvando...' : 'Salvar acesso' }}
          </AppPanelButton>
          <AppPanelButton v-if="view?.credentialsSet" variant="secondary" @click="remove">
            Remover acesso salvo
          </AppPanelButton>
        </div>
      </fieldset>
    </form>
  </details>
</template>
