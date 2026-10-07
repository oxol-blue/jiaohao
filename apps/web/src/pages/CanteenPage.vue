<script setup>
import { onMounted, onUnmounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { api, getToken } from "../api";

const route = useRoute();
const router = useRouter();
const floors = ref([]);
const windows = ref([]);
const selectedFloor = ref("");
const ticket = ref(null);
const message = ref("");
let socket = null;

async function loadMine() {
  const mine = await api("/api/v1/me/ticket");
  ticket.value = mine.ok ? mine.data : null;
}

async function loadWindows() {
  const params = new URLSearchParams({ canteen_id: route.params.id });
  if (selectedFloor.value) params.set("floor_id", selectedFloor.value);
  const list = await api(`/api/v1/windows?${params.toString()}`);
  if (!list.ok) {
    message.value = list.error?.message || "无法加载窗口";
    return;
  }
  windows.value = list.data.items || [];
}

async function take(windowId) {
  message.value = "";
  const result = await api("/api/v1/tickets", {
    method: "POST",
    body: { window_id: windowId },
  });
  if (!result.ok) {
    message.value = result.error?.message || "取号失败";
    return;
  }
  ticket.value = result.data;
  await loadWindows();
}

async function cancel() {
  if (!ticket.value?.id) return;
  const result = await api(`/api/v1/tickets/${ticket.value.id}/cancel`, { method: "POST" });
  if (!result.ok) {
    message.value = result.error?.message || "取消失败";
    return;
  }
  ticket.value = null;
  await loadWindows();
}

function connectRealtime() {
  const token = getToken();
  if (!token) return;
  const proto = location.protocol === "https:" ? "wss" : "ws";
  socket = new WebSocket(`${proto}://${location.host}/api/v1/ws?token=${encodeURIComponent(token)}`);
  socket.onmessage = (ev) => {
    const msg = JSON.parse(ev.data);
    if (msg.event === "queue.window.updated") {
      windows.value = windows.value.map((w) =>
        w.id === msg.window_id
          ? { ...w, current_number: msg.current_number, waiting_count: msg.waiting_count, status: msg.status }
          : w
      );
    }
    if (msg.event === "queue.ticket.updated" && msg.ticket) {
      ticket.value = msg.ticket.status === "waiting" || msg.ticket.status === "called" ? msg.ticket : null;
    }
  };
  socket.onopen = () => {
    const topics = ["ticket:mine", ...windows.value.map((w) => `window:${w.id}`)];
    socket.send(JSON.stringify({ type: "subscribe", topics }));
  };
}

onMounted(async () => {
  const floorRes = await api(`/api/v1/canteens/${route.params.id}/floors`);
  if (!floorRes.ok) {
    message.value = floorRes.error?.message || "无法加载楼层";
    return;
  }
  floors.value = floorRes.data.items || [];
  await loadWindows();
  await loadMine();
  connectRealtime();
});

onUnmounted(() => {
  if (socket) socket.close();
});
</script>

<template>
  <section class="card">
    <button type="button" class="ghost" @click="router.push('/home')">返回食堂</button>
    <p v-if="ticket" class="mine">
      我的号：{{ ticket.window_name }} {{ ticket.number }}（{{ ticket.status }}，前面 {{ ticket.people_ahead }} 人）
      <button v-if="ticket.status === 'waiting'" type="button" class="ghost" @click="cancel">取消</button>
    </p>
    <div v-if="floors.length" class="chips">
      <button type="button" class="chip" :class="{ on: !selectedFloor }" @click="selectedFloor = ''; loadWindows()">
        全部楼层
      </button>
      <button
        v-for="f in floors"
        :key="f.id"
        type="button"
        class="chip"
        :class="{ on: selectedFloor === f.id }"
        @click="selectedFloor = f.id; loadWindows()"
      >
        {{ f.name }}
      </button>
    </div>
    <article v-for="w in windows" :key="w.id" class="window">
      <h2>{{ w.name }} <span class="muted">{{ w.code }}</span></h2>
      <p>{{ w.blurb || "暂无说明" }}</p>
      <p>当前号 {{ w.current_number || "—" }} · 等待 {{ w.waiting_count }} 人 · {{ w.status }}</p>
      <button type="button" @click="take(w.id)">取号</button>
    </article>
    <p v-if="!windows.length" class="muted">该范围暂无窗口</p>
    <p v-if="message" class="error">{{ message }}</p>
  </section>
</template>
