<script setup>
import { onMounted, onUnmounted, ref } from "vue";
import { useRoute } from "vue-router";

const route = useRoute();
const view = ref(null);
const message = ref("");
const ttsNote = ref("");
let socket = null;
let spoken = "";

function token() {
  return String(route.query.token || "");
}

function markSpoken(id) {
  if (!id) return;
  spoken = id;
  sessionStorage.setItem(`jiaohao.spoken.${route.params.id}`, id);
}

function speak(msg) {
  const id = msg.announcement_id;
  if (!id || id === spoken) return;
  markSpoken(id);
  if (!window.speechSynthesis) {
    ttsNote.value = "本机不支持语音，请看屏幕数字";
    return;
  }
  const utter = new SpeechSynthesisUtterance(msg.text || `请 ${msg.number} 号到 ${msg.window_name}`);
  utter.lang = "zh-CN";
  window.speechSynthesis.speak(utter);
}

async function load() {
  const q = new URLSearchParams({ token: token() });
  const res = await fetch(`/api/v1/display/windows/${route.params.id}?${q.toString()}`);
  const json = await res.json();
  if (!json.ok) {
    view.value = null;
    message.value = json.error?.message || "展示链接无效";
    return;
  }
  view.value = json.data;
  message.value = "";
  markSpoken(json.data.announcement_id);
}

function connect() {
  if (!token()) return;
  const proto = location.protocol === "https:" ? "wss" : "ws";
  socket = new WebSocket(
    `${proto}://${location.host}/api/v1/ws?token=${encodeURIComponent(token())}`,
  );
  socket.onopen = () => {
    socket.send(JSON.stringify({ type: "subscribe", topics: [`display:${route.params.id}`] }));
  };
  socket.onmessage = (ev) => {
    const msg = JSON.parse(ev.data);
    if (msg.event === "queue.window.updated" && msg.window_id === route.params.id) {
      view.value = {
        ...(view.value || {}),
        current_number: msg.current_number,
        waiting_count: msg.waiting_count,
        status: msg.status,
      };
    }
    if (msg.event === "queue.announcement") speak(msg);
  };
}

onMounted(async () => {
  if (!token()) {
    message.value = "缺少展示令牌";
    return;
  }
  await load();
  if (view.value) connect();
});

onUnmounted(() => {
  if (socket) socket.close();
});
</script>

<template>
  <section class="display">
    <p v-if="message" class="error">{{ message }}</p>
    <template v-else-if="view">
      <p class="display-name">{{ view.window_name }} <span>{{ view.code }}</span></p>
      <p class="display-number">{{ view.current_number || "—" }}</p>
      <p class="display-wait">等待 {{ view.waiting_count }} 人 · {{ view.status }}</p>
      <p v-if="ttsNote" class="muted">{{ ttsNote }}</p>
    </template>
  </section>
</template>
