<script setup lang="ts">
import type { IssuerProfile } from '../../types/accounting'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
const props = defineProps<{
  profile: IssuerProfile
  busy: boolean
  writable: boolean
  dirty: boolean
}>()
const emit = defineEmits<{ change: [profile: IssuerProfile]; save: [] }>()
const fields: Array<{ key: keyof IssuerProfile; label: string }> = [
  { key: 'legalName', label: 'Nome do emitente' },
  { key: 'document', label: 'CPF/CNPJ' },
  { key: 'issuerType', label: 'Tipo de emitente' },
  { key: 'taxRegime', label: 'Regime declarado' },
  { key: 'municipality', label: 'Município' },
  { key: 'municipalityCode', label: 'Código IBGE' },
  { key: 'state', label: 'UF' },
  { key: 'municipalRegistration', label: 'Inscrição municipal' },
  { key: 'stateRegistration', label: 'Inscrição estadual' },
]
function update(key: keyof IssuerProfile, event: Event) {
  const value = (event.target as HTMLInputElement).value
  emit('change', { ...props.profile, [key]: key === 'legalName' ? value : value || null })
}
</script>
<template>
  <form class="accounting-form" @submit.prevent="emit('save')">
    <p>
      Dados declarados. Deixe vazio o que ainda não sabe; o cadastro não confirma enquadramento
      fiscal.
    </p>
    <fieldset class="accounting-form__grid" :disabled="busy || !writable">
      <label v-for="field in fields" :key="field.key" class="accounting-form__field">
        <span>{{ field.label }}</span>
        <input
          class="omni-input"
          :value="profile[field.key] || ''"
          :required="field.key === 'legalName'"
          :maxlength="field.key === 'legalName' ? 200 : 120"
          placeholder="Não informado"
          @input="update(field.key, $event)"
        />
      </label>
    </fieldset>
    <p v-if="dirty" role="status">Edições ainda não salvas.</p>
    <AppPanelButton
      v-if="writable"
      type="submit"
      :disabled="busy || !dirty || !profile.legalName.trim()"
    >
      Salvar cadastro
    </AppPanelButton>
  </form>
</template>
