// Whether a decode snapshot row has a usable map position.
export function hasValidPos(item) {
  if (!item) return false;
  if (item.hasPos === true) return true;
  const { lat, lon } = item;
  if (!Number.isFinite(lat) || !Number.isFinite(lon)) return false;
  if (lat === 0 && lon === 0) return false;
  return Math.abs(lat) <= 90 && Math.abs(lon) <= 180;
}
