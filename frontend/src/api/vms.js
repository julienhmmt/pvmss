import api from './client.js';

export function listVMs() {
  return api.get('/vms');
}

export function getVM(id) {
  return api.get(`/vms/${id}`);
}

export function vmAction(id, action, node) {
  return api.post(`/vms/${id}/action`, { action, node });
}
