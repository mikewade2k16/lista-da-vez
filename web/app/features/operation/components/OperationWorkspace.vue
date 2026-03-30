<script setup>
import OperationFinishModal from "~/features/operation/components/OperationFinishModal.vue";
import OperationConsultantStrip from "~/features/operation/components/OperationConsultantStrip.vue";
import OperationQueueColumns from "~/features/operation/components/OperationQueueColumns.vue";

defineProps({
  state: {
    type: Object,
    required: true
  }
});
</script>

<template>
  <OperationQueueColumns :state="state" />
  <OperationConsultantStrip :state="state" />
  <OperationFinishModal :state="state" />
</template>
