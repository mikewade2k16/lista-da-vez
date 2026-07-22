<script setup lang="ts">
import OmnichannelAutomationMvp from "~/components/omnichannel/automation/OmnichannelAutomationMvp.vue";

definePageMeta({
  layout: "dashboard",
  workspaceId: "omnichannel",
});
</script>

<template>
  <div class="page-workspace">
    <OmnichannelAutomationMvp />
  </div>
</template>
