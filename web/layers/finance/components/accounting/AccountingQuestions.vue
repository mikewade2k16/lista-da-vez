<script setup lang="ts">
import type { AccountingAnswer, Diagnosis } from '../../types/accounting'
const props = defineProps<{
  answers: Record<string, AccountingAnswer>
  intent: Diagnosis['intent']
  disabled: boolean
}>()
const emit = defineEmits<{ change: [value: Record<string, AccountingAnswer>] }>()
const questions = [
  {
    key: 'taxRegime',
    label: 'Qual regime você declara para essa competência?',
    purpose: 'Precisamos conferir o enquadramento e a vigência antes de calcular.',
  },
  {
    key: 'service',
    label: 'Qual serviço foi prestado e onde?',
    purpose: 'A descrição ajuda a identificar os dados e classificações necessários.',
    kind: 'service',
  },
  {
    key: 'customer',
    label: 'Quem foi o tomador do serviço?',
    purpose: 'O pagador do comprovante não é automaticamente o tomador.',
    kind: 'service',
  },
  {
    key: 'items',
    label: 'Quais produtos e quantidades compõem a operação?',
    purpose: 'Os itens e códigos fiscais serão conferidos no rascunho.',
    kind: 'product',
  },
  {
    key: 'recipient',
    label: 'Quem recebe os produtos e em qual localidade?',
    purpose: 'O destino é necessário para identificar a cobertura da operação.',
    kind: 'product',
  },
  {
    key: 'amount',
    label: 'Qual valor você confirma?',
    purpose: 'Depois informe o valor exato no rascunho; não usamos o comprovante por suposição.',
  },
]
function answer(key: string, event: Event) {
  const value = (event.target as HTMLInputElement).value
  emit('change', {
    ...props.answers,
    [key]: value
      ? { value, confirmation: 'declared', origin: 'declaration', evidenceRef: null }
      : { value: null, confirmation: 'unknown', origin: null, evidenceRef: null },
  })
}
</script>
<template>
  <fieldset class="accounting-form__grid" :disabled="disabled">
    <label
      v-for="question in questions.filter(
        (item) => !item.kind || intent === 'mixed' || item.kind === intent,
      )"
      :key="question.key"
      class="accounting-form__field"
    >
      <span>{{ question.label }}</span>
      <input
        class="omni-input"
        maxlength="4000"
        :value="answers[question.key]?.value || ''"
        placeholder="Não sei / informar depois"
        @input="answer(question.key, $event)"
      />
      <small>
        {{ question.purpose }} Origem:
        {{
          answers[question.key]?.confirmation === 'confirmed'
            ? 'evidência revisada no servidor'
            : answers[question.key]?.value
              ? 'declarada'
              : 'desconhecida'
        }}.
      </small>
    </label>
    <p>
      Sem dados suficientes, a orientação fica pendente. Não informe senhas, certificados ou
      documentos de identidade neste formulário de perguntas.
    </p>
  </fieldset>
</template>
