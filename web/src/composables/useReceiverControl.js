import { ref, onMounted, onUnmounted } from 'vue';
import { connect } from '@/utils/protocol.js';

export function useReceiverControl() {
  const enabled = ref(true);
  const connected = ref(false);
  const statusText = ref('连接中...');
  const busy = ref(false);
  let conn = null;

  function applyStatus(msg) {
    if (msg.enabled !== undefined) enabled.value = msg.enabled;
  }

  function setEnabled(on) {
    if (!conn || busy.value || on === enabled.value) return;
    busy.value = true;
    conn.send({ cmd: 'receiver', enabled: on });
    setTimeout(() => { busy.value = false; }, 800);
  }

  onMounted(() => {
    conn = connect({
      onOpen: () => {
        connected.value = true;
        statusText.value = '已连接';
      },
      onClose: () => {
        connected.value = false;
        statusText.value = '已断开，重连中...';
      },
      onStatus: applyStatus,
    });
  });

  onUnmounted(() => conn?.close());

  return { enabled, connected, statusText, busy, setEnabled };
}
