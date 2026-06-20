// WebSocket transport + binary frame parsing.

export const MSG_SPECTRUM = 0x01;
export const MSG_AUDIO = 0x02;
export const MSG_IQ = 0x03;

// connect opens a reconnecting WebSocket and dispatches decoded messages to
// the supplied handlers. Returns { send, close }.
export function connect(handlers) {
  let ws = null;
  let closed = false;

  const open = () => {
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    ws = new WebSocket(`${proto}//${location.host}/ws`);
    ws.binaryType = 'arraybuffer';
    ws.onopen = () => handlers.onOpen?.();
    ws.onclose = () => {
      handlers.onClose?.();
      if (!closed) setTimeout(open, 2000);
    };
    ws.onmessage = (ev) => {
      if (typeof ev.data === 'string') {
        const msg = JSON.parse(ev.data);
        if (msg.type === 'status') handlers.onStatus?.(msg);
        else handlers.onMessage?.(msg);
        return;
      }
      const dv = new DataView(ev.data);
      switch (dv.getUint8(0)) {
        case MSG_SPECTRUM: {
          const centerFreq = dv.getUint32(1, true);
          const bins = dv.getUint16(5, true);
          handlers.onSpectrum?.(centerFreq, new Uint8Array(ev.data, 7, bins));
          break;
        }
        case MSG_AUDIO: {
          const n = (ev.data.byteLength - 3) / 2;
          const pcm = new Float32Array(n);
          for (let i = 0; i < n; i++) pcm[i] = dv.getInt16(3 + i * 2, true) / 32768;
          handlers.onAudio?.(pcm);
          break;
        }
        case MSG_IQ: {
          if (ev.data.byteLength < 14) break;
          const centerHz = dv.getUint32(1, true);
          const rate = dv.getUint32(5, true);
          const channels = dv.getUint8(9);
          const n = dv.getUint32(10, true);
          handlers.onIQ?.({
            centerHz,
            rate,
            channels,
            data: new Int8Array(ev.data, 14, n),
          });
          break;
        }
      }
    };
  };

  open();

  return {
    send(obj) {
      if (ws?.readyState === WebSocket.OPEN) ws.send(JSON.stringify(obj));
    },
    close() {
      closed = true;
      ws?.close();
    },
  };
}
