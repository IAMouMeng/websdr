import axios from 'axios';

const request = axios.create({
  baseURL: '',
  timeout: 60_000,
  headers: {
    'Content-Type': 'application/json',
  },
});

request.interceptors.request.use(
  (config) => config,
  (error) => Promise.reject(error),
);

request.interceptors.response.use(
  (response) => response.data,
  (error) => {
    const data = error.response?.data;
    let message = '请求失败';
    if (typeof data === 'string' && data) {
      message = data;
    } else if (data?.error) {
      message = data.error;
    } else if (data?.message) {
      message = data.message;
    } else if (error.message) {
      message = error.message;
    }
    console.error('[API]', error.config?.method?.toUpperCase(), error.config?.url, message);
    return Promise.reject(new Error(message));
  },
);

export default request;
