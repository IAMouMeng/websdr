// Build WAV from recorded int8 IQ chunks.

function writeStr(view, offset, s) {
  for (let i = 0; i < s.length; i++) view.setUint8(offset + i, s.charCodeAt(i));
}

export function buildIQWav(chunks, channels) {
  if (!chunks.length) return null;
  const rate = chunks[0].rate;
  let total = 0;
  for (const c of chunks) total += c.data.length;
  const isStereo = channels === 2;
  const pcmBytes = isStereo ? total * 2 : total * 2;
  const buf = new ArrayBuffer(44 + pcmBytes);
  const view = new DataView(buf);
  writeStr(view, 0, 'RIFF');
  view.setUint32(4, 36 + pcmBytes, true);
  writeStr(view, 8, 'WAVE');
  writeStr(view, 12, 'fmt ');
  view.setUint32(16, 16, true);
  view.setUint16(20, 1, true);
  view.setUint16(22, isStereo ? 2 : 1, true);
  view.setUint32(24, rate, true);
  view.setUint32(28, rate * (isStereo ? 4 : 2), true);
  view.setUint16(32, isStereo ? 4 : 2, true);
  view.setUint16(34, 16, true);
  writeStr(view, 36, 'data');
  view.setUint32(40, pcmBytes, true);

  let o = 44;
  for (const chunk of chunks) {
    const ch = chunk.channels || channels;
    if (ch === 2 && isStereo) {
      for (let i = 0; i + 1 < chunk.data.length; i += 2) {
        view.setInt16(o, chunk.data[i] << 8, true);
        view.setInt16(o + 2, chunk.data[i + 1] << 8, true);
        o += 4;
      }
    } else if (ch === 1 && isStereo) {
      for (let i = 0; i < chunk.data.length; i++) {
        view.setInt16(o, chunk.data[i] << 8, true);
        view.setInt16(o + 2, 0, true);
        o += 4;
      }
    } else {
      for (let i = 0; i < chunk.data.length; i++) {
        view.setInt16(o, chunk.data[i] << 8, true);
        o += 2;
      }
    }
  }
  return new Blob([buf], { type: 'audio/wav' });
}

export function fmtBytes(n) {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / (1024 * 1024)).toFixed(2)} MB`;
}

export function downloadBlob(blob, filename) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}
