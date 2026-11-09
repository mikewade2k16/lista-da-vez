<script setup lang="ts">
import { computed, toRef } from 'vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import type { FiscalTemplateFields } from '../../types/fiscal'
const props = defineProps<{ service: FiscalTemplateFields }>()
const service = toRef(props, 'service')
const retention = computed({
  get: () => (service.value.issWithheld == null ? '' : service.value.issWithheld ? 'yes' : 'no'),
  set: (value: string) => {
    service.value.issWithheld = value === '' ? null : value === 'yes'
  },
})
const rate = computed({
  get: () => service.value.issRate ?? '',
  set: (value: string | number) => {
    service.value.issRate = value === '' ? null : Number(value)
  },
})
</script>
<template>
  <details class="finance-service-fields">
    <summary>Códigos e tributação informados</summary>
    <p>
      Preencha os dados aplicáveis ao serviço ou use um template. Valores não informados permanecem
      vazios.
    </p>
    <div class="finance-service-fields__grid">
      <label>
        Código do serviço
        <input v-model="service.serviceCode" maxlength="60" />
      </label>
      <label>
        CNAE
        <input v-model="service.cnae" maxlength="60" />
      </label>
      <label>
        NBS
        <input v-model="service.nbs" maxlength="60" />
      </label>
      <label>
        Alíquota ISS (%)
        <input v-model="rate" type="number" min="0" max="100" step="0.01" />
      </label>
      <AppSelectField
        v-model="retention"
        label="ISS retido"
        :options="[
          { value: '', label: 'Não informado' },
          { value: 'yes', label: 'Sim' },
          { value: 'no', label: 'Não' },
        ]"
      />
    </div>
  </details>
</template>
<style scoped>
.finance-service-fields {
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  padding: 10px;
}
.finance-service-fields summary {
  cursor: pointer;
  font-weight: 600;
}
.finance-service-fields p {
  font-size: 0.8rem;
  margin: 8px 0;
  color: rgb(var(--muted));
}
.finance-service-fields__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 10px;
}
.finance-service-fields__grid label {
  display: grid;
  gap: 5px;
  font-size: 0.8rem;
}
.finance-service-fields__grid input {
  width: 100%;
  min-width: 0;
  box-sizing: border-box;
  padding: 7px;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  background: rgb(var(--surface));
  color: rgb(var(--text));
}
</style>
