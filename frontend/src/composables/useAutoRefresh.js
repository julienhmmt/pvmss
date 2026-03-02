import { ref, onMounted, onUnmounted } from 'vue';

export function useAutoRefresh(fetchFn, intervalMs = 30000) {
  const loading = ref(false);
  const error = ref('');
  let timer = null;

  async function refresh() {
    if (document.hidden) return;
    loading.value = true;
    error.value = '';
    try {
      await fetchFn();
    } catch (err) {
      error.value = err.message || 'Refresh failed';
    } finally {
      loading.value = false;
    }
  }

  function handleVisibility() {
    if (document.hidden) {
      clearInterval(timer);
    } else {
      refresh();
      timer = setInterval(refresh, intervalMs);
    }
  }

  function start() {
    refresh();
    timer = setInterval(refresh, intervalMs);
    document.addEventListener('visibilitychange', handleVisibility);
  }

  function stop() {
    clearInterval(timer);
    document.removeEventListener('visibilitychange', handleVisibility);
  }

  onMounted(start);
  onUnmounted(stop);

  return { loading, error, refresh };
}
