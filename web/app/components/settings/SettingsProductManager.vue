<script setup>
import { reactive, ref, watch } from "vue";

const props = defineProps({
  products: {
    type: Array,
    default: () => []
  }
});

const emit = defineEmits(["add", "update", "remove"]);
const drafts = ref({});
const newProduct = reactive({
  name: "",
  code: "",
  category: "",
  basePrice: ""
});
const addError = ref("");

watch(
  () => props.products,
  (products) => {
    drafts.value = Object.fromEntries(
      (products || []).map((product) => [
        product.id,
        {
          name: product.name,
          code: product.code || "",
          category: product.category || "",
          basePrice: Number(product.basePrice || 0)
        }
      ])
    );
  },
  { immediate: true, deep: true }
);

function normalize(str) {
  return String(str || "").trim().toLowerCase();
}

function isDuplicateName(name, excludeId = null) {
  const normalized = normalize(name);
  if (!normalized) return false;
  return (props.products || []).some(
    (p) => p.id !== excludeId && normalize(p.name) === normalized
  );
}

function submitAdd() {
  const trimmed = newProduct.name.trim();
  if (!trimmed) return;
  if (isDuplicateName(trimmed)) {
    addError.value = "Já existe um produto com esse nome.";
    return;
  }
  addError.value = "";
  emit("add", { ...newProduct });
  newProduct.name = "";
  newProduct.code = "";
  newProduct.category = "";
  newProduct.basePrice = "";
}
</script>

<template>
  <article class="settings-card">
    <header class="settings-card__header">
      <h3 class="settings-card__title">Catalogo de produtos</h3>
      <p class="settings-card__text">
        Usado no search do modal. Depois voce pode trocar por API sem mudar o fluxo do fechamento.
      </p>
    </header>

    <div class="product-head">
      <span>Produto</span>
      <span>Codigo</span>
      <span>Categoria</span>
      <span>Preco base</span>
    </div>

    <div class="product-list">
      <span v-if="!products.length" class="insight-empty">Sem produtos no catalogo.</span>
      <div
        v-for="product in products"
        :key="product.id"
        class="product-row"
      >
        <input v-if="drafts[product.id]" v-model="drafts[product.id].name" class="product-row__input" type="text" @change="$emit('update', product.id, drafts[product.id])">
        <input v-if="drafts[product.id]" v-model="drafts[product.id].code" class="product-row__input" type="text" @change="$emit('update', product.id, drafts[product.id])">
        <input v-if="drafts[product.id]" v-model="drafts[product.id].category" class="product-row__input" type="text" @change="$emit('update', product.id, drafts[product.id])">
        <input v-if="drafts[product.id]" v-model="drafts[product.id].basePrice" class="product-row__input" type="number" min="0" step="0.01" @change="$emit('update', product.id, drafts[product.id])">
        <button class="product-row__remove" type="button" @click="$emit('remove', product.id)">Excluir</button>
      </div>
    </div>

    <form class="product-add" @submit.prevent="submitAdd">
      <input v-model="newProduct.name" class="product-add__input" type="text" placeholder="Nome do produto" @input="addError = ''">
      <input v-model="newProduct.code" class="product-add__input" type="text" placeholder="Codigo do produto">
      <input v-model="newProduct.category" class="product-add__input" type="text" placeholder="Categoria">
      <input v-model="newProduct.basePrice" class="product-add__input" type="number" min="0" step="0.01" placeholder="Preco base">
      <button class="product-add__button" type="submit">Adicionar produto</button>
    </form>
    <span v-if="addError" class="option-add__error">{{ addError }}</span>
  </article>
</template>
