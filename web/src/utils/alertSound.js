let ctx = null;

function audioCtx() {
  if (!ctx) {
    ctx = new (window.AudioContext || window.webkitAudioContext)();
  }
  return ctx;
}

function tone({ freq = 880, duration = 0.08, type = 'sine', gain = 0.14, attack = 0.015 }) {
  try {
    const ac = audioCtx();
    if (ac.state === 'suspended') {
      ac.resume();
    }
    const osc = ac.createOscillator();
    const amp = ac.createGain();
    const t0 = ac.currentTime;
    osc.type = type;
    osc.frequency.value = freq;
    amp.gain.setValueAtTime(0, t0);
    amp.gain.linearRampToValueAtTime(gain, t0 + attack);
    amp.gain.exponentialRampToValueAtTime(0.001, t0 + duration);
    osc.connect(amp);
    amp.connect(ac.destination);
    osc.start(t0);
    osc.stop(t0 + duration + 0.04);
  } catch {
    // autoplay policy or missing Web Audio
  }
}

/** Short single beep when a new aircraft/vessel appears in the list. */
export function playNewTargetSound() {
  tone({ freq: 620, duration: 0.09, gain: 0.12 });
}

/** Two-tone chime when position is first resolved for a target. */
export function playPositionSound() {
  tone({ freq: 880, duration: 0.07, gain: 0.1 });
  setTimeout(() => tone({ freq: 1175, duration: 0.11, gain: 0.13 }), 95);
}
