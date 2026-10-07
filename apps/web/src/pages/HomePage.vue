<script setup>
import { onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { api, setToken } from "../api";

const router = useRouter();
const user = ref(null);
const canteens = ref([]);
const message = ref("");

onMounted(async () => {
  const me = await api("/api/v1/me");
  if (!me.ok) {
    setToken("");
    router.replace("/");
    return;
  }
  user.value = me.data;
  const list = await api("/api/v1/canteens");
  if (!list.ok) {
    message.value = list.error?.message || "无法加载食堂";
    return;
  }
  canteens.value = list.data.items || [];
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
    <p class="muted">选择食堂查看窗口排队</p>
    <button v-if="user.role === 'staff'" type="button" class="list-btn" @click="router.push('/staff')">员工叫号台</button>
    <button
      v-for="c in canteens"
      :key="c.id"
      type="button"
      class="list-btn"
      @click="router.push(`/canteens/${c.id}`)"
    >
      {{ c.name }}
    </button>
    <p v-if="message" class="error">{{ message }}</p>
    <button type="button" class="ghost" @click="logout">退出</button>
  </section>
</template>
