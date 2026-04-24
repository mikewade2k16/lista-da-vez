<script setup>
import ConsultantWorkspace from "~/components/consultant/ConsultantWorkspace.vue";
import { storeToRefs } from "pinia";
import { useConsultantsStore } from "~/stores/consultants";

definePageMeta({
  layout: "dashboard",
  workspaceId: "consultor"
});

const consultantsStore = useConsultantsStore();
const { state } = storeToRefs(consultantsStore);
</script>

<template>
  <div class="workspace-host">
    <ConsultantWorkspace :state="state" />
  </div>
</template>
