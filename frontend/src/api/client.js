import axios from 'axios';

const api = axios.create({
  baseURL: '/api/v1',
  withCredentials: true,
  headers: { 'Content-Type': 'application/json' },
});

// Redirect to the login page on 401 (unauthenticated), except for auth endpoints
// themselves which handle their own error state.
api.interceptors.response.use(
  res => res,
  err => {
    if (err.response?.status === 401) {
      const url = err.config?.url || '';
      const isAuthEndpoint = url.includes('/auth/');
      if (!isAuthEndpoint) {
        window.location.href = '/login';
        return new Promise(() => {});  // prevent further error handling
      }
    }
    return Promise.reject(err);
  }
);

export default api;
