<script setup lang="ts">
import type { ProductItem } from '~/types/products'

// Conteudo do popover "Detalhes do produto" da tabela de produtos do site.
// Extraido do workspace para manter o arquivo principal enxuto.
defineProps<{ product: ProductItem }>()
</script>

<template>
  <div class="site-product-info space-y-1 text-xs">
    <p>
      <strong>Nome:</strong>
      {{ product.name }}
    </p>
    <p>
      <strong>Codigo:</strong>
      {{ product.code || '-' }}
    </p>
    <p>
      <strong>Status:</strong>
      {{ product.status }}
    </p>
    <p>
      <strong>Preco:</strong>
      R$ {{ product.price.toFixed(2) }}
    </p>
    <p>
      <strong>Fator:</strong>
      {{ product.fator }}
    </p>
    <p>
      <strong>Estoque:</strong>
      {{ product.stock }}
    </p>
    <p>
      <strong>Tipo:</strong>
      {{ product.tipo || '-' }}
    </p>
    <p>
      <strong>Categorias:</strong>
      {{ product.categories.join(', ') || '-' }}
    </p>
    <p>
      <strong>Campanhas:</strong>
      {{ product.campaigns.join(', ') || '-' }}
    </p>
    <p>
      <strong>Fonte:</strong>
      {{ product.sourceLabel || '-' }}
    </p>
    <p v-if="product.description">
      <strong>Descricao:</strong>
      {{ product.description }}
    </p>

    <div class="site-product-info__erp">
      <p class="site-product-info__erp-head">
        <UBadge :color="product.erpSynced ? 'success' : 'neutral'" variant="soft" size="xs">
          {{ product.erpSynced ? 'Cruzado com ERP' : 'Sem ERP' }}
        </UBadge>
      </p>
      <p v-if="product.erpName">
        <strong>ERP:</strong>
        {{ product.erpName }}
      </p>
      <p v-if="product.erpDescription">
        <strong>ERP (descricao):</strong>
        {{ product.erpDescription }}
      </p>
    </div>
  </div>
</template>

<style scoped>
.site-product-info__erp {
  margin-top: 0.5rem;
  padding-top: 0.5rem;
  border-top: 1px solid var(--line-soft);
}

.site-product-info__erp-head {
  margin-bottom: 0.25rem;
}
</style>
