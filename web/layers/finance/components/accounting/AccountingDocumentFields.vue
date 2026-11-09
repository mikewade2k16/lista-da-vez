<script setup lang="ts">
import type { AccountingFields } from '../../types/accounting'
const props = defineProps<{ fields: AccountingFields; disabled: boolean }>()
const emit = defineEmits<{ change: [value: AccountingFields] }>()
function update(path: string, event: Event) {
  const value = (event.target as HTMLInputElement).value
  const fields = JSON.parse(JSON.stringify(props.fields)) as AccountingFields
  const [group, key] = path.split('.')
  if (group === 'service' && key && fields.service)
    Object.assign(fields.service, { [key]: value || null })
  else if (group === 'customer' && key && fields.customer)
    Object.assign(fields.customer, { [key]: value || null })
  else if (group === 'recipient' && key && fields.recipient)
    Object.assign(fields.recipient, { [key]: value || null })
  else if (group === 'operation' && key && fields.operation)
    Object.assign(fields.operation, { [key]: value })
  else if (group === 'item' && key && fields.items?.[0])
    Object.assign(fields.items[0], { [key]: value || null })
  else Object.assign(fields, { [path]: value || null })
  emit('change', fields)
}
</script>
<template>
  <fieldset class="accounting-form__grid" :disabled="disabled">
    <label class="accounting-form__field">
      <span>Competência do documento</span>
      <input
        class="omni-input"
        type="date"
        :value="fields.competenceDate || ''"
        @input="update('competenceDate', $event)"
      />
    </label>
    <label class="accounting-form__field">
      <span>Valor confirmado (decimal, ex.: 125.00)</span>
      <input
        class="omni-input"
        inputmode="decimal"
        pattern="[0-9]+\.[0-9]{2}"
        :value="fields.amount || ''"
        placeholder="Não informado"
        @input="update('amount', $event)"
      />
    </label>
    <template v-if="fields.service && fields.customer">
      <label class="accounting-form__field">
        <span>Serviço prestado</span>
        <input
          class="omni-input"
          :value="fields.service.description || ''"
          @input="update('service.description', $event)"
        />
      </label>
      <label class="accounting-form__field">
        <span>Local da prestação</span>
        <input
          class="omni-input"
          :value="fields.service.location || ''"
          @input="update('service.location', $event)"
        />
      </label>
      <label class="accounting-form__field">
        <span>Nome do tomador</span>
        <input
          class="omni-input"
          :value="fields.customer.name || ''"
          @input="update('customer.name', $event)"
        />
      </label>
      <label class="accounting-form__field">
        <span>CPF/CNPJ do tomador</span>
        <input
          class="omni-input"
          :value="fields.customer.document || ''"
          @input="update('customer.document', $event)"
        />
      </label>
      <label
        v-for="field in ['serviceCode', 'cnae', 'nbs'] as const"
        :key="field"
        class="accounting-form__field"
      >
        <span>{{ field }}</span>
        <input
          class="omni-input"
          :value="fields.service[field] || ''"
          placeholder="Sem classificação confirmada"
          @input="update(`service.${field}`, $event)"
        />
      </label>
    </template>
    <template v-if="fields.items?.[0] && fields.operation && fields.recipient">
      <label
        v-for="field in [
          'description',
          'unit',
          'quantity',
          'unitPrice',
          'ncm',
          'cfop',
          'cst',
          'csosn',
          'origin',
        ] as const"
        :key="field"
        class="accounting-form__field"
      >
        <span>
          {{
            {
              description: 'Produto',
              unit: 'Unidade',
              quantity: 'Quantidade',
              unitPrice: 'Valor unitário',
              ncm: 'NCM',
              cfop: 'CFOP',
              cst: 'CST',
              csosn: 'CSOSN',
              origin: 'Origem',
            }[field]
          }}
        </span>
        <input
          class="omni-input"
          :value="fields.items[0][field] || ''"
          placeholder="Não informado"
          @input="update(`item.${field}`, $event)"
        />
      </label>
      <label
        v-for="field in ['nature', 'purpose', 'originState', 'destinationState'] as const"
        :key="field"
        class="accounting-form__field"
      >
        <span>
          {{
            {
              nature: 'Natureza da operação',
              purpose: 'Finalidade',
              originState: 'UF de origem',
              destinationState: 'UF de destino',
            }[field]
          }}
        </span>
        <input
          class="omni-input"
          :value="fields.operation[field]"
          @input="update(`operation.${field}`, $event)"
        />
      </label>
      <label class="accounting-form__field">
        <span>Destinatário</span>
        <input
          class="omni-input"
          :value="fields.recipient.name || ''"
          @input="update('recipient.name', $event)"
        />
      </label>
      <label class="accounting-form__field">
        <span>CPF/CNPJ do destinatário</span>
        <input
          class="omni-input"
          :value="fields.recipient.document || ''"
          @input="update('recipient.document', $event)"
        />
      </label>
      <p>Primeiro item do rascunho. Outros itens existentes são preservados ao salvar.</p>
    </template>
  </fieldset>
</template>
