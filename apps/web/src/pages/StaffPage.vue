<script setup>
import { onMounted, onUnmounted, ref } from "vue";
import { useRouter } from "vue-router";
import { api, getToken, setToken } from "../api";

const router = useRouter();
const windows = ref([]);
const message = ref("");
const links = ref({});
let socket = null;

async function load() {
  const list = await api("/api/v1/staff/windows");
  if (!list.ok) {
    message.value = list.error?.message || "无法加载授权窗口";
    if (list.error?.code === "UNAUTHENTICATED") {
      setToken("");
      router.replace("/");
    }
    return;
  }
  windows.value = list.data.items || [];
}

async function issueLink(id) {
  message.value = "";
  const result = await api(`/api/v1/windows/${id}/display-token`, { method: "POST" });
  if (!result.ok) {
    message.value = result.error?.message || "无法生成大屏链接";
    return;
  }
  const url = `${location.origin}${result.data.path}`;
  links.value = { ...links.value, [id]: url };
  try {
    await navigator.clipboard.writeText(url);
    message.value = "大屏链接已复制";
  } catch {
    message.value = "大屏链接已生成，请手动复制";
  }
}

async function revokeLink(id) {
  const result = await api(`/api/v1/windows/${id}/display-token`, { method: "DELETE" });
  if (!result.ok) {
    message.value = result.error?.message || "吊销失败";
    return;
  }
  const next = { ...links.value };
  delete next[id];
  links.value = next;
  message.value = "展示链接已吊销";
}

async function act(path) {
  message.value = "";
  const result = await api(path, { method: "POST" });
  if (!result.ok) {
    message.value = result.error?.message || "操作失败";
  }
  await load();
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
  };
  socket.onopen = () => {
    socket.send(JSON.stringify({ type: "subscribe", topics: windows.value.map((w) => `window:${w.id}`) }));
  };
}

onMounted(async () => {
  await load();
  connectRealtime();
});

onUnmounted(() => {
  if (socket) socket.close();
});
</script>

<template>
  <section class="card">
    <button type="button" class="ghost" @click="router.push('/home')">返回</button>
    <article v-for="w in windows" :key="w.id" class="window">
      <h2>{{ w.name }} <span class="muted">{{ w.code }} / {{ w.status }}</span></h2>
      <p>当前号 {{ w.current_number || "—" }} · 等待 {{ w.waiting_count }} 人</p>
      <div class="chips">
        <button type="button" @click="act(`/api/v1/windows/${w.id}/call-next`)">叫下一号</button>
        <button type="button" class="ghost" @click="act(`/api/v1/windows/${w.id}/complete`)">完成</button>
        <button type="button" class="ghost" @click="act(`/api/v1/windows/${w.id}/skip`)">过号</button>
        <button type="button" class="ghost" @click="act(`/api/v1/windows/${w.id}/pause-take`)">暂停取号</button>
        <button type="button" class="ghost" @click="act(`/api/v1/windows/${w.id}/resume-take`)">恢复取号</button>
        <button type="button" class="ghost" @click="act(`/api/v1/windows/${w.id}/close`)">打烊</button>
        <button type="button" class="ghost" @click="act(`/api/v1/windows/${w.id}/open`)">开始营业</button>
        <button type="button" class="ghost" @click="issueLink(w.id)">大屏链接</button>
        <button type="button" class="ghost" @click="revokeLink(w.id)">吊销大屏</button>
      </div>
      <p v-if="links[w.id]" class="muted">{{ links[w.id] }}</p>
    </article>
    <p v-if="!windows.length" class="muted">没有授权窗口</p>
    <p v-if="message" class="error">{{ message }}</p>
  </section>
</template>
