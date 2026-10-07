<script setup>
import { ref } from "vue";
import { useRouter } from "vue-router";
import { api, setToken } from "../api";

const router = useRouter();
const canteenName = ref("");
const windowName = ref("");
const windowCode = ref("");
const canteenId = ref("");
const message = ref("");

async function createCanteen() {
  message.value = "";
  const result = await api("/api/v1/admin/canteens", {
    method: "POST",
    body: { name: canteenName.value, sort: 10 },
  });
  if (!result.ok) {
    message.value = result.error?.message || "创建失败";
    if (result.error?.code === "UNAUTHENTICATED") {
      setToken("");
      router.replace("/");
    }
    return;
  }
  canteenId.value = result.data.id;
  message.value = "食堂已创建";
}

async function createWindow() {
  message.value = "";
  const result = await api("/api/v1/admin/windows", {
    method: "POST",
    body: {
      canteen_id: canteenId.value,
      name: windowName.value,
      code: windowCode.value,
      blurb: "",
      sort: 1,
    },
  });
  message.value = result.ok ? "窗口已创建，用餐者刷新后可见" : result.error?.message || "创建失败";
}
</script>

<template>
  <section class="card">
    <button type="button" class="ghost" @click="router.push('/home')">返回</button>
    <h2>新建食堂</h2>
    <input v-model="canteenName" placeholder="食堂名称" />
    <button type="button" @click="createCanteen">创建食堂</button>
    <h2>新建窗口</h2>
    <input v-model="canteenId" placeholder="食堂 ID" />
    <input v-model="windowName" placeholder="窗口名称" />
    <input v-model="windowCode" placeholder="窗口编号" />
    <button type="button" @click="createWindow">创建窗口</button>
    <p v-if="message" class="muted">{{ message }}</p>
  </section>
</template>
