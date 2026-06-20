import request from './request.js';

/** @returns {Promise<Record<string, string[]>>} */
export function fetchTLE() {
  return request.get('/api/satellite/tle').then((data) => data.tle || {});
}

/** @returns {Promise<{ satellites: object[], channels: object, msu: object[] }>} */
export function fetchCatalog() {
  return request.get('/api/satellite/catalog');
}
