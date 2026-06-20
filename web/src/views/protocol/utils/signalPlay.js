/** Route target for opening a dedicated listen/decode page from a protocol row. */

const APT_DOWNLINKS_HZ = [
  137_100_000,
  137_500_000,
  137_620_000,
  137_912_500,
];
const APT_SNAP_MAX_HZ = 80_000;

export function snapAptDownlink(freqHz) {
  if (freqHz < 136_000_000 || freqHz > 138_500_000) return freqHz;
  let best = freqHz;
  let bestDist = APT_SNAP_MAX_HZ + 1;
  for (const ch of APT_DOWNLINKS_HZ) {
    const dist = Math.abs(freqHz - ch);
    if (dist < bestDist) {
      bestDist = dist;
      best = ch;
    }
  }
  return bestDist <= APT_SNAP_MAX_HZ ? best : freqHz;
}

function satelliteSvc(item) {
  return String(item?.cols?.svc || item?.decode?.service || item?.label || '').toLowerCase();
}

export function signalPlayRoute(item) {
  if (!item?.type) return null;

  switch (item.type) {
    case 'adsb':
      return { name: 'ads-b' };
    case 'ais':
      return { name: 'ais' };
    case 'broadcast':
      if (!item.freqHz) return null;
      return {
        name: 'radio',
        query: { tune: String(item.freqHz), mode: 'wfm' },
      };
    case 'cw':
      if (!item.freqHz) return null;
      return {
        name: 'radio',
        query: { tune: String(item.freqHz), mode: 'cw' },
      };
    case 'satellite':
      if (!item.freqHz) return null;
      const svc = satelliteSvc(item);
      if (item.freqHz >= 1_690_000_000 || svc.includes('hrpt')) {
        return {
          name: 'radio',
          query: { tune: String(item.freqHz), mode: 'usb' },
        };
      }
      if (svc.includes('lrpt') || svc.includes('meteor') || svc.includes('satellite')) {
        return { name: 'satellite', query: { freqHz: String(item.freqHz) } };
      }
      if (svc.includes('dsb') || svc.includes('tip')) {
        return {
          name: 'radio',
          query: { tune: String(item.freqHz), mode: 'usb' },
        };
      }
      if (item.freqHz >= 136_000_000 && item.freqHz <= 138_500_000) {
        const snapped = snapAptDownlink(item.freqHz);
        return { name: 'apt', query: { freqHz: String(snapped) } };
      }
      if (
        (item.freqHz >= 144_000_000 && item.freqHz <= 147_000_000) ||
        (item.freqHz >= 435_000_000 && item.freqHz <= 438_000_000)
      ) {
        if (svc.includes('业余') || svc.includes('fm') || svc.includes('so-') || svc.includes('iss')) {
          return {
            name: 'radio',
            query: { tune: String(item.freqHz), mode: 'nfm' },
          };
        }
      }
      return null;
    default:
      return null;
  }
}

export function canSignalPlay(item) {
  return signalPlayRoute(item) != null;
}
