<script setup>
import { onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { api, setToken } from "../api";

const router = useRouter();
const user = ref(null);
const adminPing = ref("");

onMounted(async () => {
  const cached = sessionStorage.getItem("jiaohao.user");
  if (cached) user.value = JSON.parse(cached);
  const me = await api("/api/v1/me");
  if (!me.ok) {
    setToken("");
    router.replace("/");
    return;
  }
  user.value = me.data;
  const ping = await api("/api/v1/admin/ping");
  adminPing.value = ping.ok
    ? "管理员探测通过"
    : `${ping.error?.code || ping.status} ${ping.error?.message || ""}`;
});

async function logout() {
  await api("/api/v1/auth/logout", { method: "POST" });
  setToken("");
  sessionStorage.removeItem("jiaohao.user");
  router.replace("/");
}
</script>

<template>
  <section class="card" v-if="user">
    <p>已登录：{{ user.student_id }}（{{ user.role }}）</p>
    <p class="muted">访问 /api/v1/admin/ping：{{ adminPing }}</p>
    <p class="muted">窗口列表与取号将在后续阶段接入。</p>
    <button type="button" @click="logout">退出</button>
  </section>
</template>
