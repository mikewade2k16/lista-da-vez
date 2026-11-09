<script setup lang="ts">
import AppSelectField from '~/components/ui/AppSelectField.vue'
import ConfigAiRoleModelSelect from '~/components/omnichannel/config/ConfigAiRoleModelSelect.vue'
import { ASSISTANT_AI_CREDENTIALS_PATH } from '~/domain/omnichannel/config-api'
import type { PlusExplanationConfig } from '../../types/plus-explanation'
const model = defineModel<PlusExplanationConfig>({ required: true })
defineProps<{ accountId: string; options: Array<{ value: string; label: string }> }>()
function pricing(event: Event) {
  model.value.pricing = (event.target as HTMLInputElement).checked
    ? {
        version: '',
        source: '',
        validFrom: '',
        validTo: '',
        inputUsdPerMillion: '',
        outputUsdPerMillion: '',
      }
    : null
}
</script>
<template>
  <details class="plus-explanation-config">
    <summary>
      Limites de IA e explicações com fontes · {{ model.enabled ? 'Ativas' : 'Desativadas' }}
    </summary>
    <p>
      Resumo informativo de fontes autorizadas. Sem cálculo ou conclusão tributária. Modelo
      opcional; consultas determinísticas continuam disponíveis. Nenhum fallback de modelo ou chave
      global.
    </p>
    <p>
      Os limites abaixo também controlam as respostas da conversa principal. A credencial da
      conversa é selecionada na configuração do chat. A tabela de preços só é reutilizada se
      credencial e modelo coincidirem; com teto monetário e preço desconhecido, a IA não é chamada.
    </p>
    <UCheckbox v-model="model.enabled" label="Ativar explicações Plus" />
    <AppSelectField
      v-model="model.credentialId"
      label="Credencial do cofre para Plus (OpenAI, Gemini ou GLM)"
      :options="options"
      @change="model.model = ''"
    />
    <ConfigAiRoleModelSelect
      v-model="model.model"
      :credential-id="model.credentialId"
      capability="response"
      :account-id="accountId"
      :credential-base-path="ASSISTANT_AI_CREDENTIALS_PATH"
    />
    <AppSelectField
      v-model="model.taskClass"
      label="Classe de tarefa autorizada"
      :options="[{ value: 'source_summary', label: 'Resumo informativo de fontes' }]"
    />
    <div class="plus-explanation-config__fields">
      <label>
        Contexto máximo (bytes)
        <input
          v-model.number="model.maxInputBytes"
          type="number"
          min="1000"
          max="16000"
          class="omni-input"
        />
      </label>
      <label>
        Tokens máximos de saída
        <input
          v-model.number="model.maxOutputTokens"
          type="number"
          min="128"
          max="2048"
          class="omni-input"
        />
      </label>
      <label>
        Latência máxima (ms)
        <input
          v-model.number="model.timeoutMs"
          type="number"
          min="1000"
          max="15000"
          class="omni-input"
        />
      </label>
      <label>
        Chamadas por dia (UTC, conta inteira)
        <input
          v-model.number="model.dailyRequests"
          type="number"
          min="1"
          max="1000"
          class="omni-input"
        />
      </label>
      <label>
        Teto por chamada (USD; vazio sem teto monetário)
        <input
          v-model="model.maxRunCostUsd"
          class="omni-input"
          inputmode="decimal"
          placeholder="Ex.: 0.01"
        />
      </label>
    </div>
    <p>
      Sem tabela vigente, custo aparece como desconhecido. Teto monetário exige preço vigente;
      limite diário e tokens continuam obrigatórios. Custos calculados são estimativas pela tabela
      declarada, não fatura do provedor.
    </p>
    <label>
      <input type="checkbox" :checked="!!model.pricing" @change="pricing" />
      Informar tabela de preços conferida
    </label>
    <div v-if="model.pricing" class="plus-explanation-config__fields">
      <label>
        Versão da tabela
        <input v-model="model.pricing.version" class="omni-input" maxlength="100" />
      </label>
      <label>
        Fonte verificável
        <input v-model="model.pricing.source" class="omni-input" maxlength="1000" />
      </label>
      <label>
        Válida desde (UTC)
        <input v-model="model.pricing.validFrom" type="date" class="omni-input" />
      </label>
      <label>
        Válida até (exclusivo)
        <input v-model="model.pricing.validTo" type="date" class="omni-input" />
      </label>
      <label>
        USD por milhão de tokens de entrada
        <input v-model="model.pricing.inputUsdPerMillion" class="omni-input" inputmode="decimal" />
      </label>
      <label>
        USD por milhão de tokens de saída
        <input v-model="model.pricing.outputUsdPerMillion" class="omni-input" inputmode="decimal" />
      </label>
    </div>
  </details>
</template>
<style scoped>
.plus-explanation-config {
  min-width: 0;
}
.plus-explanation-config__fields {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
  gap: 0.5rem;
}
.plus-explanation-config__fields label {
  display: grid;
  gap: 0.25rem;
}
</style>
