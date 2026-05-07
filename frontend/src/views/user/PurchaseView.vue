<template>
  <AppLayout v-if="loading">
    <div class="flex min-h-[50vh] items-center justify-center">
      <div
        class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"
      ></div>
    </div>
  </AppLayout>
  <PurchaseSubscriptionView v-else-if="useExternalPurchase || !useInternalPayment" />
  <PaymentView v-else />
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useAppStore } from "@/stores";
import AppLayout from "@/components/layout/AppLayout.vue";
import PaymentView from "./PaymentView.vue";
import PurchaseSubscriptionView from "./PurchaseSubscriptionView.vue";

const appStore = useAppStore();
const loading = ref(false);

const useExternalPurchase = computed(
  () => appStore.cachedPublicSettings?.purchase_subscription_enabled === true,
);

const useInternalPayment = computed(
  () => appStore.cachedPublicSettings?.payment_enabled === true,
);

onMounted(async () => {
  if (appStore.publicSettingsLoaded) return;
  loading.value = true;
  try {
    await appStore.fetchPublicSettings();
  } finally {
    loading.value = false;
  }
});
</script>
