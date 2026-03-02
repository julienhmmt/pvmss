import { defineStore } from 'pinia';
import { ref } from 'vue';
import api from '../api/client.js';

export const useSearchStore = defineStore('search', () => {
  const results = ref([]);
  const loading = ref(false);
  const error = ref('');
  const hasSearched = ref(false);

  async function search({ vmid = '', name = '', tags = '', limit = 25 } = {}) {
    loading.value = true;
    error.value = '';
    hasSearched.value = true;
    try {
      const { data } = await api.get('/search/vms', { params: { vmid, name, tags, limit } });
      results.value = data.results || [];
    } catch (err) {
      error.value = err.response?.data?.message || 'Search failed';
      results.value = [];
    } finally {
      loading.value = false;
    }
  }

  function clear() {
    results.value = [];
    hasSearched.value = false;
    error.value = '';
  }

  return { results, loading, error, hasSearched, search, clear };
});
