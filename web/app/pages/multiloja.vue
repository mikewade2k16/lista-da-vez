<script setup>
import { onMounted } from "vue";
import MultiStoreWorkspace from "~/components/multistore/MultiStoreWorkspace.vue";
import { storeToRefs } from "pinia";
import { useMultiStoreStore } from "~/stores/multistore";

definePageMeta({
  layout: "dashboard",
  workspaceId: "multiloja"
});

const multiStore = useMultiStoreStore();
const { state } = storeToRefs(multiStore);

onMounted(() => {
  void multiStore.ensureLoaded();
});
</script>

<template>
  <div class="workspace-host">
    <MultiStoreWorkspace :state="state" />
  </div>
</template>
