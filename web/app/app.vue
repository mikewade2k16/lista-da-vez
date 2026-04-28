<script setup>
import AppDialogHost from "~/components/ui/AppDialogHost.vue";
import AppToastStack from "~/components/ui/AppToastStack.vue";
</script>

<template>
  <NuxtLayout>
    <NuxtPage />
  </NuxtLayout>
  <ClientOnly>
    <AppDialogHost />
    <AppToastStack />
  </ClientOnly>
</template>
