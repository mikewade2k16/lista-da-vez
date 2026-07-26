<script setup lang="ts">
import { computed, defineAsyncComponent, ref, watch } from "vue";
import OmnichannelInboxLoading from "~/components/omnichannel/OmnichannelInboxLoading.vue";
import OmnichannelConfigDrawer from "~/components/omnichannel/config/OmnichannelConfigDrawer.vue";
import { useAuthStore } from "~/stores/auth";

const AsyncOmnichannelInboxModule = defineAsyncComponent({
  loader: () => import("~/components/omnichannel/OmnichannelInboxModule.vue"),
  loadingComponent: OmnichannelInboxLoading,
  delay: 120,
  suspensible: false,
  timeout: 20_000,
});

definePageMeta({
  layout: "dashboard",
  workspaceId: "omnichannel",
  pageLabel: "Omnichannel",
});

const auth = useAuthStore();
const isPlatformAdmin = computed(() => auth.role === "platform_admin");
const route = useRoute();
const configOpen = ref(false);
const configTab = computed(() => String(route.query.config || ""));
const canConfigure = computed(
  () =>
    isPlatformAdmin.value ||
    [
      "omnichannel.instances.manage",
      "omnichannel.settings.manage",
      "omnichannel.agents.manage",
    ].some((key) => auth.effectivePermissionKeys.includes(key)),
);

watch(
  () => route.query.config,
  (value) => {
    if (value && canConfigure.value) configOpen.value = true;
  },
  { immediate: true },
);
</script>

<template>
  <section class="omnichannel-inbox-page">
    <OmnichannelConfigDrawer v-model:open="configOpen" :initial-tab="configTab" />

    <ClientOnly>
      <AsyncOmnichannelInboxModule @configure="configOpen = true" />
      <template #fallback>
        <OmnichannelInboxLoading />
      </template>
    </ClientOnly>
  </section>
</template>

<style scoped>
.omnichannel-inbox-page {
  display: flex;
  flex-direction: column;
  min-height: 0;
  height: calc(100vh - 6.5rem);
}
</style>
