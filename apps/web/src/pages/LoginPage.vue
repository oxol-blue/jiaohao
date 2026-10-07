<script setup>
import { ref } from "vue";
import { useRouter } from "vue-router";
import { api, setToken } from "../api";

const router = useRouter();
const studentId = ref("20260001");
const password = ref("diner123");
const message = ref("");

async function submit() {
  message.value = "";
  const result = await api("/api/v1/auth/login", {
    method: "POST",
    body: { student_id: studentId.value, password: password.value },
  });
  if (!result.ok) {
    message.value = result.error?.message || "登录失败";
    return;
  }
  setToken(result.data.token);
  sessionStorage.setItem("jiaohao.user", JSON.stringify(result.data.user));
  router.push("/home");
}
</script>

<template>
  <form class="card" @submit.prevent="submit">
    <label>
      学号
      <input v-model="studentId" autocomplete="username" />
    </label>
    <label>
      密码
      <input v-model="password" type="password" autocomplete="current-password" />
    </label>
    <button type="submit">登录</button>
    <p v-if="message" class="error">{{ message }}</p>
  </form>
</template>
