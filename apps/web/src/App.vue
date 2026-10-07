<script setup>
import { computed } from "vue";
import { useRoute } from "vue-router";

const route = useRoute();
const display = computed(() => route.path.startsWith("/display/"));
</script>

<template>
  <div v-if="display">
    <router-view />
  </div>
  <div v-else class="shell">
    <header class="top">
      <h1>食堂叫号</h1>
    </header>
    <router-view />
  </div>
</template>
