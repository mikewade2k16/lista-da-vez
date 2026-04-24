<script setup>
import { useAuthStore } from "~/stores/auth";

definePageMeta({
  layout: false
});

const auth = useAuthStore();
await auth.ensureSession();
await navigateTo(auth.isAuthenticated ? auth.homePath : "/auth/login");
</script>
