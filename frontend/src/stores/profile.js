import { defineStore } from 'pinia';
import { ref } from 'vue';
import api from '../api/client.js';

export const useProfileStore = defineStore('profile', () => {
  const vms = ref([]);

  async function fetchMyVMs() {
    const { data } = await api.get('/profile/vms');
    vms.value = data.vms || [];
  }

  async function doAction(vmid, action, node) {
    await api.post(`/vms/${vmid}/action`, { action, node });
    await fetchMyVMs();
  }

  return { vms, fetchMyVMs, doAction };
});
