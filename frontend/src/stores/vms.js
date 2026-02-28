import { defineStore } from 'pinia';
import { ref } from 'vue';
import { listVMs, vmAction } from '../api/vms.js';

export const useVMStore = defineStore('vms', () => {
  const vms = ref([]);
  const loading = ref(false);
  const error = ref('');

  async function fetchVMs() {
    loading.value = true;
    error.value = '';
    try {
      const { data } = await listVMs();
      vms.value = data.vms || [];
    } catch (err) {
      error.value = err.response?.data?.message || 'Failed to load VMs';
    } finally {
      loading.value = false;
    }
  }

  async function doAction(vmid, action, node) {
    try {
      await vmAction(vmid, action, node);
      await fetchVMs();
    } catch (err) {
      error.value = err.response?.data?.message || `Action ${action} failed`;
    }
  }

  return { vms, loading, error, fetchVMs, doAction };
});
