// Shared connection logic for the digital decode pages (ADS-B, AIS). It opens
// the websocket, asks the backend to switch the single tuner to the given
// service, and exposes the decoded item list as a reactive ref. The backend
// pushes a full snapshot ~1 Hz, so we just replace the list each message.

import { ref, onMounted, onUnmounted } from 'vue';
import { connect } from '@/utils/protocol.js';
import { hasValidPos } from '@/utils/decodeItem.js';
import { playNewTargetSound, playPositionSound } from '@/utils/alertSound.js';

export function useDecodeService({
  service,
  messageType,
  field,
  idKey,
  /** ADS-B: beep when first snapshot has tracks without position yet */
  soundInitialWithoutPos = false,
}) {
  const items = ref([]);
  const statusText = ref('连接中...');
  const connected = ref(false);
  const receiverEnabled = ref(true);
  let conn = null;
  const known = new Map();
  let seeded = false;

  function itemId(item) {
    if (idKey) return item[idKey];
    return item.icao ?? item.mmsi;
  }

  onMounted(() => {
    conn = connect({
      onOpen: () => {
        connected.value = true;
        statusText.value = '已连接';
        conn.send({ cmd: 'service', service });
      },
      onClose: () => {
        connected.value = false;
        statusText.value = '已断开，重连中...';
      },
      onStatus: (msg) => {
        if (msg.enabled !== undefined) {
          receiverEnabled.value = msg.enabled;
          statusText.value = msg.enabled ? '已连接' : 'SDR 已关闭';
        }
      },
      onMessage: (msg) => {
        if (msg.type !== messageType) return;
        const list = msg[field] || [];

        if (!seeded) {
          for (const item of list) {
            const id = itemId(item);
            if (id == null || id === '') continue;
            const pos = hasValidPos(item);
            known.set(id, pos);
            if (soundInitialWithoutPos && !pos) {
              playNewTargetSound();
            }
          }
          seeded = true;
          items.value = list;
          return;
        }

        for (const item of list) {
          const id = itemId(item);
          if (id == null || id === '') continue;
          const pos = hasValidPos(item);
          const prev = known.get(id);
          if (prev === undefined) {
            playNewTargetSound();
          } else if (!prev && pos) {
            playPositionSound();
          }
          known.set(id, pos);
        }

        items.value = list;
      },
    });
  });

  onUnmounted(() => {
    const c = conn;
    conn = null;
    if (!c) return;
    c.send({ cmd: 'service', service: 'radio' });
    // Let the service command flush before closing — immediate close can drop it.
    setTimeout(() => c.close(), 200);
  });

  return { items, statusText, connected, receiverEnabled };
}
