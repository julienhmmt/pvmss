import { defineStore } from 'pinia';
import { ref } from 'vue';
import api from '../api/client.js';

export const useVmDetailsStore = defineStore('vmDetails', () => {
  const vm      = ref(null);
  const metrics = ref(null);
  const loading = ref(false);
  const error   = ref('');

  async function fetchVM(vmid) {
    loading.value = true;
    error.value = '';
    try {
      const { data } = await api.get(`/vms/${vmid}`);
      vm.value = data;
    } catch (err) {
      error.value = err.response?.data?.message || 'Failed to load VM';
    } finally {
      loading.value = false;
    }
  }

  async function fetchMetrics(vmid) {
    try {
      const { data } = await api.get(`/vms/${vmid}/metrics`);
      metrics.value = data;
    } catch (_) { /* metrics are non-critical */ }
  }

  async function doAction(vmid, action, node) {
    await api.post(`/vms/${vmid}/action`, { action, node });
    await fetchVM(vmid);
  }

  return { vm, metrics, loading, error, fetchVM, fetchMetrics, doAction };
});
