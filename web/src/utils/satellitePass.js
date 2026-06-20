/**
 * Satellite pass prediction and Doppler using satellite.js + backend TLE.
 */
import {
  propagate,
  twoline2satrec,
  gstime,
  eciToEcf,
  ecfToLookAngles,
  radiansLat,
  radiansLong,
} from 'satellite.js';

const C = 299_792_458; // m/s
const OBS_KEY = 'websdr_observer';
export const PASS_DAYS = 7;
export const PASS_MIN_EL = 10;

export function loadObserver() {
  try {
    const raw = localStorage.getItem(OBS_KEY);
    if (raw) {
      const o = JSON.parse(raw);
      if (Number.isFinite(o.lat) && Number.isFinite(o.lon)) return o;
    }
  } catch { /* ignore */ }
  return { lat: 31.23, lon: 121.47, alt: 0, name: '默认' };
}

export function saveObserver(obs) {
  localStorage.setItem(OBS_KEY, JSON.stringify(obs));
}

export function observerGeodetic(obs) {
  return {
    latitude: radiansLat(obs.lat),
    longitude: radiansLong(obs.lon),
    height: (obs.alt || 0) / 1000,
  };
}


function lookAt(satrec, obs, date) {
  const pv = propagate(satrec, date);
  if (!pv?.position) return null;
  const gmst = gstime(date);
  const ecf = eciToEcf(pv.position, gmst);
  return ecfToLookAngles(obs, ecf);
}

function elevationDeg(satrec, obs, date) {
  const la = lookAt(satrec, obs, date);
  if (!la) return -90;
  return la.elevation * (180 / Math.PI);
}

function rangeM(satrec, obs, date) {
  const la = lookAt(satrec, obs, date);
  return la ? la.range : 0;
}

/** Doppler shift in Hz (positive = higher received freq). */
export function dopplerHz(satrec, obs, freqHz, date = new Date()) {
  const dt = 0.5;
  const t1 = new Date(date.getTime());
  const t2 = new Date(date.getTime() + dt * 1000);
  const r1 = rangeM(satrec, obs, t1);
  const r2 = rangeM(satrec, obs, t2);
  const rangeRate = (r2 - r1) / dt;
  return -(freqHz * rangeRate) / C;
}

/** Live geometry for map/sky display. */
export function geometryNow(satrec, obs, date = new Date()) {
  const la = lookAt(satrec, obs, date);
  if (!la) return { elevation: 0, azimuth: 0, rangeKm: 0 };
  return {
    elevation: la.elevation * (180 / Math.PI),
    azimuth: la.azimuth * (180 / Math.PI),
    rangeKm: la.range,
  };
}

/** GEO satellites: no LEO-style passes; return current sky position. */
export function geoStatus(satrec, obs, minEl = 5) {
  const geo = geometryNow(satrec, obs);
  return {
    isGeo: true,
    visible: geo.elevation >= minEl,
    elevation: geo.elevation,
    azimuth: geo.azimuth,
    rangeKm: geo.rangeKm,
  };
}

function refineBoundary(satrec, obs, tLo, tHi, minEl, findRising) {
  let lo = tLo;
  let hi = tHi;
  while (hi - lo > 1000) {
    const mid = Math.floor((lo + hi) / 2);
    const above = elevationDeg(satrec, obs, new Date(mid)) >= minEl;
    if (findRising) {
      if (above) hi = mid;
      else lo = mid;
    } else if (above) {
      lo = mid;
    } else {
      hi = mid;
    }
  }
  return new Date(findRising ? hi : lo);
}

function refineMaxEl(satrec, obs, tLo, tHi) {
  let lo = tLo;
  let hi = tHi;
  let bestT = lo;
  let bestEl = -90;
  while (hi - lo > 2000) {
    const third = Math.floor((hi - lo) / 3);
    const t1 = lo + third;
    const t2 = hi - third;
    const el1 = elevationDeg(satrec, obs, new Date(t1));
    const el2 = elevationDeg(satrec, obs, new Date(t2));
    if (el1 < el2) {
      lo = t1;
      if (el2 > bestEl) {
        bestEl = el2;
        bestT = t2;
      }
    } else {
      hi = t2;
      if (el1 > bestEl) {
        bestEl = el1;
        bestT = t1;
      }
    }
  }
  const mid = Math.floor((lo + hi) / 2);
  const elMid = elevationDeg(satrec, obs, new Date(mid));
  if (elMid > bestEl) {
    bestEl = elMid;
    bestT = mid;
  }
  return { maxEl: bestEl, maxElTime: new Date(bestT) };
}

/**
 * Find passes above minEl degrees in the next `days` days.
 * Coarse scan (30s) + binary refinement for AOS/LOS.
 */
export function findPasses(satrec, obs, days = PASS_DAYS, minEl = PASS_MIN_EL) {
  const start = new Date();
  const endMs = start.getTime() + days * 24 * 3600 * 1000;
  const coarseStep = 30;
  const passes = [];
  let inPass = false;
  let aosApprox = 0;
  let losApprox = 0;
  let maxEl = -90;
  let maxElTime = start;

  for (let t = start.getTime(); t <= endMs; t += coarseStep * 1000) {
    const el = elevationDeg(satrec, obs, new Date(t));
    if (!inPass && el >= minEl) {
      inPass = true;
      aosApprox = Math.max(start.getTime(), t - coarseStep * 1000);
      maxEl = el;
      maxElTime = new Date(t);
      losApprox = t;
    } else if (inPass && el >= minEl) {
      losApprox = t;
      if (el > maxEl) {
        maxEl = el;
        maxElTime = new Date(t);
      }
    } else if (inPass && el < minEl) {
      inPass = false;
      const exitT = t;
      const aos = refineBoundary(satrec, obs, aosApprox, exitT, minEl, true);
      const los = refineBoundary(satrec, obs, losApprox, exitT, minEl, false);
      const peak = refineMaxEl(satrec, obs, aos.getTime(), los.getTime());
      if (peak.maxEl >= minEl) {
        passes.push({ aos, los, maxEl: peak.maxEl, maxElTime: peak.maxElTime });
      }
    }
  }
  if (inPass) {
    const aos = refineBoundary(satrec, obs, aosApprox, endMs, minEl, true);
    const peak = refineMaxEl(satrec, obs, aos.getTime(), endMs);
    if (peak.maxEl >= minEl) {
      passes.push({
        aos,
        los: new Date(endMs),
        maxEl: Math.max(maxEl, peak.maxEl),
        maxElTime: peak.maxElTime,
      });
    }
  }
  return passes;
}

/** Elevation curve for one pass (for chart). */
export function passElevationCurve(satrec, obs, aos, los, points = 80) {
  const span = los.getTime() - aos.getTime();
  const out = [];
  for (let i = 0; i <= points; i++) {
    const t = new Date(aos.getTime() + (span * i) / points);
    const la = lookAt(satrec, obs, t);
    const el = la ? la.elevation * (180 / Math.PI) : 0;
    out.push({ t: t.getTime(), el, minFromStart: (t.getTime() - aos.getTime()) / 60000 });
  }
  return out;
}

/** Flat elevation curve for GEO (constant elevation). */
export function geoElevationCurve(satrec, obs, hours = 24) {
  const start = Date.now();
  const el = geometryNow(satrec, obs).elevation;
  return [
    { t: start, el, minFromStart: 0 },
    { t: start + hours * 3600 * 1000, el, minFromStart: hours * 60 },
  ];
}

/** Whether a catalog entry or NORAD id is geostationary. */
export function isGeoSatellite(entry, norad = 0) {
  if (entry?.orbit === 'geo') return true;
  return norad === 43823;
}

export function satrecFromTLE(line1, line2) {
  return twoline2satrec(line1, line2);
}

export function fmtTime(d) {
  if (!d) return '—';
  return d.toLocaleString(undefined, { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' });
}

export function fmtDur(aos, los) {
  const sec = Math.round((los - aos) / 1000);
  const m = Math.floor(sec / 60);
  const s = sec % 60;
  return `${m}:${String(s).padStart(2, '0')}`;
}

export function fmtFreqMHz(hz) {
  if (!hz) return '—';
  return `${(hz / 1e6).toFixed(3)} MHz`;
}

export function fmtClockUTC(d = new Date()) {
  return d.toISOString().slice(11, 19);
}

export function fmtClockBeijing(d = new Date()) {
  return d.toLocaleTimeString('zh-CN', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
    timeZone: 'Asia/Shanghai',
  });
}

export function fmtCountdown(ms) {
  if (!Number.isFinite(ms) || ms < 0) return '00:00:00';
  const sec = Math.floor(ms / 1000);
  const h = Math.floor(sec / 3600);
  const m = Math.floor((sec % 3600) / 60);
  const s = sec % 60;
  return `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`;
}

export function currentPass(passes, now = Date.now()) {
  for (const p of passes) {
    if (now >= p.aos.getTime() && now <= p.los.getTime()) return p;
  }
  return null;
}

export function nextPass(passes, now = Date.now()) {
  for (const p of passes) {
    if (p.aos.getTime() > now) return p;
  }
  return null;
}

/** Ascending / descending leg during a pass (升轨 / 降轨). */
export function passLeg(satrec, obs, date = new Date()) {
  if (!satrec) return 'level';
  const el0 = geometryNow(satrec, obs, date).elevation;
  const el1 = geometryNow(satrec, obs, new Date(date.getTime() + 20_000)).elevation;
  if (el1 > el0 + 0.08) return 'asc';
  if (el1 < el0 - 0.08) return 'desc';
  return 'level';
}

export function passLegLabel(leg) {
  if (leg === 'asc') return '升轨';
  if (leg === 'desc') return '降轨';
  return '平轨';
}
