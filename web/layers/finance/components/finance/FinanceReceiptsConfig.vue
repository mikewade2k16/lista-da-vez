<script setup lang="ts">
import { useUiStore } from '~/stores/ui'
import { useFinanceReceipts } from '../../composables/useFinanceRecords'
import { formatRecordDate, originalSummary, receiptLinkSummary } from '../../utils/finance-records'
import type { FinanceReceiptItem } from '../../types/records'

// Comprovantes enviados pelo chat/anexo: vínculo com a planilha (revalidado no back)
// e arquivo original. Desfazer o vínculo não apaga o lançamento da planilha.
const props = defineProps<{ canManage: boolean }>()
const receipts = useFinanceReceipts()
const { items, more, busyId, loading, error, status } = receipts
const ui = useUiStore()

async function confirm(message: string): Promise<boolean> {
  const result = (await ui.confirm({ title: 'Comprovantes', message })) as { confirmed: boolean }
  return result.confirmed
}
async function unlink(item: FinanceReceiptItem) {
  if (
    await confirm(
      'Desfazer o vínculo deste comprovante? O lançamento continua na planilha para você editar.',
    )
  )
    await receipts.unlink(item.document.id)
}
async function removeOriginal(item: FinanceReceiptItem) {
  if (
    await confirm(
      'Apagar agora o arquivo original? Os dados extraídos e o vínculo continuam salvos. Não dá para desfazer.',
    )
  )
    await receipts.removeOriginal(item.document.id)
}
function title(item: FinanceReceiptItem): string {
  const fields = item.document.fields
  const who = fields.payeeName || fields.customerName || item.document.sourceName
  const amount =
    typeof fields.amount === 'number'
      ? ` · R$ ${fields.amount.toLocaleString('pt-BR', { minimumFractionDigits: 2 })}`
      : ''
  return `${who}${amount}`
}
</script>

<template>
  <details class="finance-records">
    <summary>
      Comprovantes enviados
      <span class="finance-records__summary-meta">
        {{ loading ? 'carregando' : `${items.length}${more ? '+' : ''} documentos` }}
      </span>
    </summary>
    <p class="finance-records__intro">
      Documentos enviados pelo chat. Mostra se cada um foi lançado na planilha e se o arquivo
      original ainda está guardado. Para lançar um comprovante, peça no chat: “lança esse
      comprovante na planilha”.
    </p>
    <p v-if="error" class="finance-records__status finance-records__status--error" role="alert">
      {{ error }}
    </p>
    <p v-if="status" class="finance-records__status" role="status">{{ status }}</p>
    <p v-if="!loading && !error && items.length === 0" class="finance-records__status">
      Nenhum documento enviado ainda. Anexe um comprovante no chat do Finance.
    </p>
    <ul v-if="items.length" class="finance-records__list">
      <li v-for="item in items" :key="item.document.id" class="finance-records__item">
        <div class="finance-records__item-head">
          <span class="finance-records__title">{{ title(item) }}</span>
          <span class="finance-records__meta">
            {{
              item.document.fields.paymentDate
                ? formatRecordDate(item.document.fields.paymentDate)
                : 'sem data'
            }}
            · {{ item.document.sourceName }}
          </span>
        </div>
        <span
          :class="`finance-records__line finance-records__line--${receiptLinkSummary(item).tone}`"
        >
          {{ receiptLinkSummary(item).text }}
        </span>
        <span
          :class="`finance-records__line finance-records__line--${originalSummary(item.original).tone}`"
        >
          {{ originalSummary(item.original).text }}
        </span>
        <div class="finance-records__actions">
          <button
            v-if="item.original?.status === 'stored'"
            type="button"
            class="finance-records__button"
            :disabled="busyId === item.document.id"
            @click="receipts.download(item)"
          >
            Baixar original
          </button>
          <button
            v-if="item.link && props.canManage"
            type="button"
            class="finance-records__button"
            :disabled="busyId === item.document.id"
            @click="unlink(item)"
          >
            Desfazer vínculo
          </button>
          <button
            v-if="item.original?.status === 'stored' && props.canManage"
            type="button"
            class="finance-records__button finance-records__button--danger"
            :disabled="busyId === item.document.id"
            @click="removeOriginal(item)"
          >
            Apagar original agora
          </button>
        </div>
      </li>
    </ul>
    <div class="finance-records__actions">
      <button v-if="more" type="button" class="finance-records__button" @click="receipts.loadMore">
        Carregar mais
      </button>
      <button
        type="button"
        class="finance-records__button"
        :disabled="loading"
        @click="receipts.reload"
      >
        Atualizar
      </button>
    </div>
  </details>
</template>

<style src="../../assets/finance-records.css"></style>
