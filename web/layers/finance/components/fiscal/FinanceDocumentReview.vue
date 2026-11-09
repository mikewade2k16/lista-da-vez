<script setup lang="ts">
import { computed } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import { useFinanceDocumentReview } from '../../composables/useFinanceDocumentReview'
const props = defineProps<{ id: string; version: string; dirty: boolean; expanded?: boolean }>()
const editor = useFinanceDocumentReview(
  () => props.id,
  () => props.version,
  () => props.dirty,
)
const { review, busy, error } = editor
const amount = computed(() =>
  review.value?.fields.amount == null
    ? 'Não informado'
    : new Intl.NumberFormat('pt-BR', { style: 'currency', currency: 'BRL' }).format(
        review.value.fields.amount,
      ),
)
function toggle(event: Event) {
  void editor.toggle((event.target as HTMLDetailsElement).open)
}
</script>
<template>
  <details class="finance-review" :open="expanded" @toggle="toggle">
    <summary>Revisar dados para a nota</summary>
    <div class="finance-review__body">
      <p v-if="dirty" role="status">
        Salve as alterações do rascunho antes de confirmar a revisão.
      </p>
      <p v-if="busy" role="status">Carregando...</p>
      <p v-if="error" role="alert">{{ error }}</p>
      <template v-if="review">
        <p v-if="review.approvalId" role="status">
          Revisão confirmada para esta versão. Nenhuma nota foi emitida.
        </p>
        <p v-else>
          Confira os dados abaixo. Alterações no rascunho ou no perfil exigem uma nova revisão.
        </p>
        <dl class="finance-review__grid">
          <div>
            <dt>Emitente</dt>
            <dd>{{ review.issuer.legalName || 'Não informado' }}</dd>
          </div>
          <div>
            <dt>CPF/CNPJ do emitente</dt>
            <dd>{{ review.issuer.document || 'Não informado' }}</dd>
          </div>
          <div>
            <dt>Tipo e regime informado</dt>
            <dd>
              {{ review.issuer.issuerType || 'Não informado' }} ·
              {{ review.issuer.taxRegime || 'Não informado' }}
            </dd>
          </div>
          <div>
            <dt>Município do emitente</dt>
            <dd>
              {{ review.issuer.municipality || 'Não informado' }} /
              {{ review.issuer.state || 'Não informado' }}
            </dd>
          </div>
          <div>
            <dt>Endereço do emitente</dt>
            <dd>
              {{ review.issuer.street }} {{ review.issuer.number }} {{ review.issuer.complement }} ·
              {{ review.issuer.district }} · {{ review.issuer.postalCode }}
            </dd>
          </div>
          <div>
            <dt>Tomador</dt>
            <dd>{{ review.fields.customerName || 'Não informado' }}</dd>
          </div>
          <div>
            <dt>CPF/CNPJ do tomador</dt>
            <dd>{{ review.fields.customerDocument || 'Não informado' }}</dd>
          </div>
          <div>
            <dt>Valor do serviço</dt>
            <dd>{{ amount }}</dd>
          </div>
          <div>
            <dt>Competência</dt>
            <dd>{{ review.fields.competenceDate || 'Não informada' }}</dd>
          </div>
          <div>
            <dt>Serviço</dt>
            <dd>{{ review.fields.service.serviceDescription || 'Não informado' }}</dd>
          </div>
          <div>
            <dt>Código / CNAE / NBS</dt>
            <dd>
              {{ review.fields.service.serviceCode || 'Não informado' }} /
              {{ review.fields.service.cnae || 'Não informado' }} /
              {{ review.fields.service.nbs || 'Não informado' }}
            </dd>
          </div>
          <div>
            <dt>ISS informado</dt>
            <dd>
              {{
                review.fields.service.issRate == null
                  ? 'Não informado'
                  : `${review.fields.service.issRate}%`
              }}
              · Retenção:
              {{
                review.fields.service.issWithheld == null
                  ? 'Não informada'
                  : review.fields.service.issWithheld
                    ? 'Sim'
                    : 'Não'
              }}
            </dd>
          </div>
          <div>
            <dt>Observações</dt>
            <dd>{{ review.fields.service.notes || 'Sem observações' }}</dd>
          </div>
        </dl>
        <p v-if="review.missingFields.length" role="status">
          Pendências: {{ review.missingFields.join(', ') }}.
        </p>
        <NuxtLink to="/perfil#fiscal">Completar ou corrigir emitente no perfil</NuxtLink>
        <p>
          Esta confirmação registra a revisão dos dados preparados. O preenchimento e a emissão no
          portal ainda dependem da integração; não autoriza emissão automática.
        </p>
        <div class="finance-review__actions">
          <AppPanelButton
            v-if="!review.approvalId"
            :disabled="busy || dirty || !review.ready"
            @click="editor.approve(true)"
          >
            Confirmar revisão dos dados
          </AppPanelButton>
          <AppPanelButton
            v-else
            variant="secondary"
            :disabled="busy || dirty"
            @click="editor.approve(false)"
          >
            Cancelar confirmação
          </AppPanelButton>
          <AppPanelButton
            variant="secondary"
            :disabled="busy || dirty"
            @click="editor.downloadPreparation"
          >
            Baixar dados do rascunho (JSON)
          </AppPanelButton>
          <AppPanelButton variant="secondary" :disabled="busy" @click="editor.load">
            Recarregar revisão
          </AppPanelButton>
        </div>
      </template>
    </div>
  </details>
</template>
<style scoped>
.finance-review {
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  padding: 10px;
  margin-top: 10px;
}
.finance-review summary {
  cursor: pointer;
  font-weight: 600;
}
.finance-review__body {
  display: grid;
  gap: 10px;
  padding-top: 10px;
  font-size: 0.8rem;
}
.finance-review__body p {
  margin: 0;
}
.finance-review__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 10px;
  margin: 0;
}
.finance-review__grid dt {
  color: rgb(var(--muted));
}
.finance-review__grid dd {
  margin: 3px 0 0;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.finance-review__actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
</style>
